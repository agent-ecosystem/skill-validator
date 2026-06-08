// Package content analyzes the textual content of SKILL.md files. It computes
// metrics such as word count, code block ratio, imperative sentence ratio,
// information density, and instruction specificity to assess content quality.
package content

import (
	"regexp"
	"strings"
	"sync"
	"unicode"

	"github.com/agent-ecosystem/skill-validator/types"
	"github.com/agent-ecosystem/skill-validator/util"
)

// strongMarkerRes contains pre-compiled patterns for strong directive language
// markers (must, always, never, etc.) used to measure instruction specificity.
var strongMarkerRes = compilePatterns([]string{
	`\bmust\b`, `\balways\b`, `\bnever\b`, `\bshall\b`,
	`\brequired\b`, `\bdo not\b`, `\bdon't\b`, `\bensure\b`,
	`\bcritical\b`, `\bmandatory\b`,
})

// weakMarkerRes contains pre-compiled patterns for weak/advisory language
// markers (may, consider, could, etc.) used to measure instruction specificity.
var weakMarkerRes = compilePatterns([]string{
	`\bmay\b`, `\bconsider\b`, `\bcould\b`, `\bmight\b`,
	`\boptional\b`, `\bpossibly\b`, `\bsuggested\b`,
	`\bprefer\b`, `\btry to\b`, `\bif possible\b`,
})

func compilePatterns(patterns []string) []*regexp.Regexp {
	res := make([]*regexp.Regexp, len(patterns))
	for i, p := range patterns {
		res[i] = regexp.MustCompile(p)
	}
	return res
}

var (
	codeLangPattern    = regexp.MustCompile("(?:```|~~~)(\\w+)")
	sentenceSplitPat   = regexp.MustCompile(`[.!?]\s+|[.!?]$|\n\n+`)
	chineseSentencePat = regexp.MustCompile(`[。！？；\n]+`)
	leadingFormatPat   = regexp.MustCompile(`^[#*\->\s]+`)
	sectionPattern     = regexp.MustCompile(`(?m)^#{2,}\s+`)
	listItemPattern    = regexp.MustCompile(`(?m)^[\s]*[-*+]\s+|^\s*\d+\.\s+`)
)

// defaultImperativeVerbs is the fallback English verb set used when no config
// is loaded. It preserves backward compatibility with the original hardcoded set.
var defaultImperativeVerbs = map[string]bool{
	"use": true, "run": true, "create": true, "add": true, "set": true,
	"install": true, "configure": true, "write": true, "read": true,
	"check": true, "verify": true, "make": true, "build": true, "test": true,
	"ensure": true, "include": true, "remove": true, "delete": true,
	"update": true, "call": true, "import": true, "export": true,
	"define": true, "implement": true, "return": true, "pass": true,
	"handle": true, "parse": true, "generate": true, "format": true,
	"validate": true, "convert": true, "follow": true, "apply": true,
	"start": true, "stop": true, "avoid": true, "keep": true, "do": true,
	"execute": true, "open": true, "close": true, "save": true, "load": true,
	"send": true, "receive": true,
}

// imperativeDetector handles multilingual imperative sentence detection.
type imperativeDetector struct {
	mu  sync.RWMutex
	cfg *ImperativeConfig

	// Pre-compiled detection data per language.
	verbSets   map[string]map[string]bool  // lang -> verb set
	keywordRes map[string][]*regexp.Regexp // lang -> keyword regexps
	splitRes   map[string]*regexp.Regexp   // lang -> sentence split regexp
}

// globalDetector is the package-level imperative detector instance.
var globalDetector = newImperativeDetector(nil)

// newImperativeDetector creates a new detector from the given config.
// If cfg is nil, the embedded default config is used.
func newImperativeDetector(cfg *ImperativeConfig) *imperativeDetector {
	if cfg == nil {
		cfg = DefaultImperativeConfig()
	}
	d := &imperativeDetector{
		cfg:        cfg,
		verbSets:   make(map[string]map[string]bool),
		keywordRes: make(map[string][]*regexp.Regexp),
		splitRes:   make(map[string]*regexp.Regexp),
	}
	for lang, rules := range cfg.Languages {
		// Build verb set
		vs := make(map[string]bool, len(rules.Verbs))
		for _, v := range rules.Verbs {
			vs[strings.ToLower(v)] = true
		}
		d.verbSets[lang] = vs

		// Build keyword regexps (match as substring anywhere in sentence)
		for _, kw := range rules.Keywords {
			re := regexp.MustCompile(regexp.QuoteMeta(kw))
			d.keywordRes[lang] = append(d.keywordRes[lang], re)
		}

		// Build per-language sentence split pattern
		if rules.SentenceSplitPattern != "" {
			d.splitRes[lang] = regexp.MustCompile(rules.SentenceSplitPattern)
		}
	}
	return d
}

// SetImperativeConfig replaces the global detector's configuration.
// This is safe to call from any goroutine.
func SetImperativeConfig(cfg *ImperativeConfig) {
	globalDetector.mu.Lock()
	defer globalDetector.mu.Unlock()
	newD := newImperativeDetector(cfg)
	globalDetector.verbSets = newD.verbSets
	globalDetector.keywordRes = newD.keywordRes
	globalDetector.splitRes = newD.splitRes
	globalDetector.cfg = newD.cfg
}

// ReloadImperativeConfig reloads the imperative config from the given path.
// If path is empty, the standard lookup order is used.
func ReloadImperativeConfig(path string) {
	SetImperativeConfig(LoadImperativeConfig(path))
}

// hasChinese returns true if the string contains any Chinese characters.
func hasChinese(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

// splitSentences splits text into sentences using language-appropriate rules.
// It handles mixed Chinese/English content by applying both splitters.
func (d *imperativeDetector) splitSentences(text string) []string {
	// Remove code blocks first
	text = util.CodeBlockStrip.ReplaceAllString(text, "")
	// Remove inline code
	text = util.InlineCodeStrip.ReplaceAllString(text, "")

	if hasChinese(text) {
		return splitMixed(text)
	}
	return splitEnglish(text)
}

// splitMixed handles text containing both Chinese and English.
// It first splits on Chinese punctuation, then further splits any
// English-style segments on English sentence boundaries.
func splitMixed(text string) []string {
	// First split on Chinese punctuation
	parts := chineseSentencePat.Split(text, -1)
	var sentences []string
	for _, s := range parts {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		// Pure English segments: use English splitter
		if !hasChinese(s) {
			subParts := splitEnglish(s)
			sentences = append(sentences, subParts...)
			continue
		}
		// Mixed segment: also split on English boundaries if present
		if strings.ContainsAny(s, ".!?") {
			subParts := sentenceSplitPat.Split(s, -1)
			for _, sp := range subParts {
				sp = strings.TrimSpace(sp)
				if sp != "" && len(sp) > 5 {
					sentences = append(sentences, sp)
				}
			}
		} else if len(s) > 5 {
			sentences = append(sentences, s)
		}
	}
	return sentences
}

// splitEnglish splits on English sentence boundaries.
func splitEnglish(text string) []string {
	parts := sentenceSplitPat.Split(text, -1)
	var sentences []string
	for _, s := range parts {
		s = strings.TrimSpace(s)
		if s != "" && len(s) > 5 {
			sentences = append(sentences, s)
		}
	}
	return sentences
}

// isImperative checks if a sentence is imperative using all configured languages.
func (d *imperativeDetector) isImperative(sentence string) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()

	// Clean markdown formatting
	cleaned := leadingFormatPat.ReplaceAllString(sentence, "")

	// Try Chinese detection first if the sentence contains Chinese chars
	if hasChinese(cleaned) {
		if d.isImperativeLang(cleaned, "zh") {
			return true
		}
	}

	// Try English (verb-first) detection
	words := strings.Fields(cleaned)
	if len(words) == 0 {
		return false
	}
	firstWord := strings.ToLower(words[0])

	// Check English verbs from config
	if vs, ok := d.verbSets["en"]; ok && vs[firstWord] {
		return true
	}

	// Fallback to hardcoded default for backward compatibility
	return defaultImperativeVerbs[firstWord]
}

// isImperativeLang checks imperative detection for a specific language.
func (d *imperativeDetector) isImperativeLang(cleaned, lang string) bool {
	if _, ok := d.cfg.Languages[lang]; !ok {
		return false
	}

	// Check verb-first match
	// For space-separated languages (e.g. English), match the first word.
	// For non-space-separated languages (e.g. Chinese), match the sentence prefix.
	if vs, ok := d.verbSets[lang]; ok {
		if hasChinese(cleaned) {
			// Prefix match: check if sentence starts with any verb
			for verb := range vs {
				if strings.HasPrefix(cleaned, verb) {
					return true
				}
			}
		} else {
			words := strings.Fields(cleaned)
			if len(words) > 0 {
				first := strings.ToLower(words[0])
				if vs[first] {
					return true
				}
			}
		}
	}

	// Check keyword match anywhere in the sentence
	for _, re := range d.keywordRes[lang] {
		if re.MatchString(cleaned) {
			return true
		}
	}

	return false
}

// Analyze computes content metrics for SKILL.md content.
func Analyze(content string) *types.ContentReport {
	return AnalyzeWithConfig(content, nil)
}

// AnalyzeWithConfig computes content metrics using the provided imperative
// config. If cfg is nil, the current global detector is used.
func AnalyzeWithConfig(content string, cfg *ImperativeConfig) *types.ContentReport {
	if strings.TrimSpace(content) == "" {
		return &types.ContentReport{}
	}

	var detector *imperativeDetector
	if cfg != nil {
		detector = newImperativeDetector(cfg)
	} else {
		detector = globalDetector
	}

	words := strings.Fields(content)
	wordCount := len(words)

	// Code block analysis
	codeBlocks := util.CodeBlockPattern.FindAllStringSubmatch(content, -1)
	codeBlockCount := len(codeBlocks)
	codeBlockWords := 0
	for _, match := range codeBlocks {
		codeBlockWords += len(strings.Fields(match[1]))
	}
	codeBlockRatio := 0.0
	if wordCount > 0 {
		codeBlockRatio = float64(codeBlockWords) / float64(wordCount)
	}

	// Code languages
	langMatches := codeLangPattern.FindAllStringSubmatch(content, -1)
	codeLanguages := make([]string, 0, len(langMatches))
	for _, m := range langMatches {
		codeLanguages = append(codeLanguages, m[1])
	}

	// Sentence analysis (multilingual)
	sentences := detector.splitSentences(content)
	sentenceCount := len(sentences)
	imperativeCount := countImperativeSentencesWithDetector(sentences, detector)
	imperativeRatio := 0.0
	if sentenceCount > 0 {
		imperativeRatio = float64(imperativeCount) / float64(sentenceCount)
	}

	// Information density: when code blocks are present, factor in the
	// code-to-prose ratio; otherwise score purely on imperative ratio so
	// prose-only skills aren't penalized for lacking code.
	informationDensity := imperativeRatio
	if codeBlockCount > 0 {
		informationDensity = (codeBlockRatio * 0.5) + (imperativeRatio * 0.5)
	}

	// Language marker analysis
	strongCount := countMarkerMatches(content, strongMarkerRes)
	weakCount := countMarkerMatches(content, weakMarkerRes)
	totalMarkers := strongCount + weakCount
	instructionSpecificity := 0.0
	if totalMarkers > 0 {
		instructionSpecificity = float64(strongCount) / float64(totalMarkers)
	}

	// Section count (H2+ headers)
	sectionCount := len(sectionPattern.FindAllString(content, -1))

	// List item count
	listItemCount := len(listItemPattern.FindAllString(content, -1))

	return &types.ContentReport{
		WordCount:              wordCount,
		CodeBlockCount:         codeBlockCount,
		CodeBlockRatio:         util.RoundTo(codeBlockRatio, 4),
		CodeLanguages:          codeLanguages,
		SentenceCount:          sentenceCount,
		ImperativeCount:        imperativeCount,
		ImperativeRatio:        util.RoundTo(imperativeRatio, 4),
		InformationDensity:     util.RoundTo(informationDensity, 4),
		StrongMarkers:          strongCount,
		WeakMarkers:            weakCount,
		InstructionSpecificity: util.RoundTo(instructionSpecificity, 4),
		SectionCount:           sectionCount,
		ListItemCount:          listItemCount,
	}
}

func countImperativeSentencesWithDetector(sentences []string, d *imperativeDetector) int {
	count := 0
	for _, sentence := range sentences {
		if d.isImperative(sentence) {
			count++
		}
	}
	return count
}

func countMarkerMatches(text string, patterns []*regexp.Regexp) int {
	total := 0
	textLower := strings.ToLower(text)
	for _, re := range patterns {
		total += len(re.FindAllString(textLower, -1))
	}
	return total
}
