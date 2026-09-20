package structure

import (
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/agent-ecosystem/skill-validator/skill"
	"github.com/agent-ecosystem/skill-validator/types"
)

var namePattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// Field length limits from the spec are in characters, which this package
// counts as Unicode code points (the same unit as the skills-ref reference
// validator). Counting bytes would reject multibyte descriptions well
// below the limit.
const (
	maxNameChars          = 64
	maxDescriptionChars   = 1024
	maxCompatibilityChars = 500
)

// CheckFrontmatter validates the YAML frontmatter of a parsed skill. It checks
// required fields (name, description), enforces format and length constraints,
// validates optional fields, and warns about unrecognized or keyword-stuffed fields.
func CheckFrontmatter(s *skill.Skill, opts Options) []types.Result {
	ctx := types.ResultContext{Category: "Frontmatter", File: "SKILL.md"}
	var results []types.Result

	// Check name
	name := s.Frontmatter.Name
	if name == "" {
		results = append(results, ctx.Error("name is required"))
	} else {
		if n := utf8.RuneCountInString(name); n > maxNameChars {
			results = append(results, ctx.Errorf("name exceeds %d characters (%d)", maxNameChars, n))
		}
		if !namePattern.MatchString(name) {
			results = append(results, ctx.Errorf("name %q must be lowercase alphanumeric with hyphens, no leading/trailing/consecutive hyphens", name))
		}
		// Check that name matches directory name
		dirName := filepath.Base(s.Dir)
		if name != dirName {
			results = append(results, ctx.Errorf("name does not match directory name (expected %q, got %q)", dirName, name))
		}
		if len(results) == 0 || (name != "" && namePattern.MatchString(name)) {
			results = append(results, ctx.Passf("name: %q (valid)", name))
		}
	}

	// Check description
	desc := s.Frontmatter.Description
	if desc == "" {
		results = append(results, ctx.Error("description is required"))
	} else if n := utf8.RuneCountInString(desc); n > maxDescriptionChars {
		results = append(results, ctx.Errorf("description exceeds %d characters (%d)", maxDescriptionChars, n))
	} else if strings.TrimSpace(desc) == "" {
		results = append(results, ctx.Error("description must not be empty/whitespace-only"))
	} else {
		results = append(results, ctx.Passf("description: (%d chars)", n))
		results = append(results, checkDescriptionKeywordStuffing(ctx, desc)...)
	}

	// Check optional license
	if s.Frontmatter.License != "" {
		results = append(results, ctx.Passf("license: %q", s.Frontmatter.License))
	}

	// Check optional compatibility
	if s.Frontmatter.Compatibility != "" {
		if n := utf8.RuneCountInString(s.Frontmatter.Compatibility); n > maxCompatibilityChars {
			results = append(results, ctx.Errorf("compatibility exceeds %d characters (%d)", maxCompatibilityChars, n))
		} else {
			results = append(results, ctx.Passf("compatibility: (%d chars)", n))
		}
	}

	// Check optional metadata
	if s.RawFrontmatter["metadata"] != nil {
		// Verify it's a map[string]string
		if m, ok := s.RawFrontmatter["metadata"].(map[string]any); ok {
			allStrings := true
			for k, v := range m {
				if _, ok := v.(string); !ok {
					results = append(results, ctx.Errorf("metadata[%q] value must be a string", k))
					allStrings = false
				}
			}
			if allStrings {
				results = append(results, ctx.Passf("metadata: (%d entries)", len(m)))
			}
		} else {
			results = append(results, ctx.Error("metadata must be a map of string keys to string values"))
		}
	}

	// Check optional allowed-tools
	if !s.Frontmatter.AllowedTools.IsEmpty() {
		results = append(results, ctx.Passf("allowed-tools: %q", s.Frontmatter.AllowedTools.Value))
		if s.Frontmatter.AllowedTools.WasList {
			results = append(results, ctx.Info("allowed-tools is a YAML list; the spec defines this as a space-delimited string — both are accepted, but a string is more portable across agent implementations"))
		}
	}

	// Warn on unrecognized fields (unless extra frontmatter is allowed)
	if !opts.AllowExtraFrontmatter {
		for _, field := range s.UnrecognizedFields() {
			results = append(results, ctx.Warnf("unrecognized field: %q", field))
		}
	}

	return results
}

var quotedStringPattern = regexp.MustCompile(`"[^"]*"`)

const (
	// minQuotedStrings is the minimum number of quoted strings that triggers
	// the quoted-string keyword stuffing heuristic.
	minQuotedStrings = 5

	// minCommaSegments is the minimum number of comma-separated segments in a
	// single sentence that triggers the comma-list keyword stuffing heuristic.
	minCommaSegments = 8

	// maxShortSegmentPct is the percentage of comma segments that must be
	// "short" (≤3 words) for the comma-list heuristic to fire.
	maxShortSegmentPct = 60

	// minAvgWordsPerSegment is the minimum average words per comma-separated
	// segment. Sentences at or above this density are considered prose with
	// inline lists rather than keyword dumps, even if many segments are short.
	minAvgWordsPerSegment = 3
)

func checkDescriptionKeywordStuffing(ctx types.ResultContext, desc string) []types.Result {
	// Heuristic 1: Many quoted strings with insufficient prose context suggest keyword stuffing.
	// Descriptions that have substantial prose alongside quoted trigger lists are fine —
	// the spec encourages keywords, and many good descriptions use a prose sentence
	// followed by a supplementary trigger list.
	quotes := quotedStringPattern.FindAllString(desc, -1)
	if len(quotes) >= minQuotedStrings {
		// Strip all quoted strings to measure the remaining prose
		prose := quotedStringPattern.ReplaceAllString(desc, "")
		proseWordCount := 0
		for w := range strings.FieldsSeq(prose) {
			// Skip punctuation-only tokens (commas, periods, colons, etc.)
			cleaned := strings.TrimFunc(w, func(r rune) bool {
				return (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9')
			})
			if len(cleaned) > 0 {
				proseWordCount++
			}
		}
		// If the prose (outside quotes) has fewer words than quoted strings,
		// the description is dominated by keyword lists
		if proseWordCount < len(quotes) {
			return []types.Result{ctx.Warnf(
				"description contains %d quoted strings with little surrounding prose — "+
					"this looks like keyword stuffing; per the spec, the description should "+
					"concisely describe what the skill does and when to use it, not just list trigger phrases",
				len(quotes),
			)}
		}
	}

	// Heuristic 2: Many comma-separated short segments in a single sentence suggest a
	// bare keyword list. We check per-sentence rather than across the whole description
	// to avoid false positives on multi-sentence prose with inline enumeration lists.
	// Strip quoted strings first so that prose + trigger-list descriptions aren't penalized.
	descWithoutQuotes := quotedStringPattern.ReplaceAllString(desc, "")
	for _, sentence := range splitSentences(descWithoutQuotes) {
		allSegments := strings.Split(sentence, ",")
		var segments []string
		for _, seg := range allSegments {
			if strings.TrimSpace(seg) != "" {
				segments = append(segments, seg)
			}
		}
		if len(segments) >= minCommaSegments {
			shortCount := 0
			totalWords := 0
			for _, seg := range segments {
				words := strings.Fields(strings.TrimSpace(seg))
				totalWords += len(words)
				if len(words) <= 3 {
					shortCount++
				}
			}
			// Sentences with enough prose density are not keyword dumps,
			// even if many individual segments are short.
			if totalWords >= minAvgWordsPerSegment*len(segments) {
				continue
			}
			if shortCount*100/len(segments) >= maxShortSegmentPct {
				return []types.Result{ctx.Warnf(
					"description has %d comma-separated segments, most very short — "+
						"this looks like a keyword list; per the spec, the description should "+
						"concisely describe what the skill does and when to use it",
					len(segments),
				)}
			}
		}
	}

	return nil
}

// periodPlaceholder is a Unicode character (double surface integral) used to
// temporarily protect periods that are not sentence boundaries during splitting.
// Chosen because it is extremely unlikely to appear in skill descriptions.
const periodPlaceholder = "∯"

var (
	// Common abbreviations that should not be treated as sentence boundaries.
	// "etc." is intentionally excluded — when followed by a capital letter it
	// typically IS a sentence boundary ("...diagnostics, etc. Supports Kafka").
	nonBreakingAbbrevs = regexp.MustCompile(
		`(?i)\b(e\.g|i\.e|vs|al|approx|dept|govt|incl|assoc|avg|est|max|min|misc|ref|spec|tech)\.\s`)

	// Periods between digits: version numbers (v18.0), decimals (3.14), IPs.
	digitPeriodPattern = regexp.MustCompile(`(\d)\.(\d)`)

	// A sentence boundary: sentence-ending punctuation, whitespace, then an
	// uppercase letter starting the next sentence.
	sentenceBoundaryPattern = regexp.MustCompile(`[.!?]\s+([A-Z])`)
)

// splitSentences splits text into sentences using a lightweight heuristic.
// It protects periods in common abbreviations and numbers from being treated
// as sentence boundaries. When the heuristic fails to detect a boundary
// (e.g. missing capitalization or spaces), the text remains unsplit, which
// degrades gracefully toward stricter keyword-stuffing detection.
func splitSentences(text string) []string {
	if text == "" {
		return nil
	}

	// Protect non-boundary periods with a placeholder.
	protected := nonBreakingAbbrevs.ReplaceAllStringFunc(text, func(m string) string {
		return strings.Replace(m, ".", periodPlaceholder, 1)
	})
	protected = digitPeriodPattern.ReplaceAllString(protected, "${1}"+periodPlaceholder+"${2}")

	// Find boundary positions and split just before the capital letter.
	indices := sentenceBoundaryPattern.FindAllStringSubmatchIndex(protected, -1)
	var sentences []string
	start := 0
	for _, idx := range indices {
		capStart := idx[2] // start of the ([A-Z]) capture group
		s := strings.TrimSpace(protected[start:capStart])
		if s != "" {
			sentences = append(sentences, strings.ReplaceAll(s, periodPlaceholder, "."))
		}
		start = capStart
	}
	if start < len(protected) {
		s := strings.TrimSpace(protected[start:])
		if s != "" {
			sentences = append(sentences, strings.ReplaceAll(s, periodPlaceholder, "."))
		}
	}
	return sentences
}
