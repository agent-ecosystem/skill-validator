// Package types defines the core data types used throughout the
// skill-validator: validation results, severity levels, token counts,
// skill modes, and aggregated reports.
package types

// Level represents the severity of a validation result.
type Level int

const (
	// Pass indicates a check passed successfully.
	Pass Level = iota
	// Info indicates an informational finding that requires no action.
	Info
	// Warning indicates a non-blocking issue that should be reviewed.
	Warning
	// Error indicates a blocking issue that must be fixed.
	Error
)

// String returns the lowercase name of the level.
func (l Level) String() string {
	switch l {
	case Pass:
		return "pass"
	case Info:
		return "info"
	case Warning:
		return "warning"
	case Error:
		return "error"
	default:
		return "unknown"
	}
}

// Result represents a single validation finding.
type Result struct {
	Level    Level
	Category string
	Message  string
	File     string // path relative to skill dir, e.g. "SKILL.md", "references/guide.md"
	Line     int    // 0 = no line info
}

// TokenCount holds the token count for a single file. Truncated marks a
// file larger than the read limit, whose count covers only the prefix that
// was read.
type TokenCount struct {
	File      string
	Tokens    int
	Truncated bool
}

// ContentReport holds content quality metrics computed by the content analyzer.
type ContentReport struct {
	WordCount              int      `json:"word_count"`
	CodeBlockCount         int      `json:"code_block_count"`
	CodeBlockRatio         float64  `json:"code_block_ratio"`
	CodeLanguages          []string `json:"code_languages"`
	SentenceCount          int      `json:"sentence_count"`
	ImperativeCount        int      `json:"imperative_count"`
	ImperativeRatio        float64  `json:"imperative_ratio"`
	InformationDensity     float64  `json:"information_density"`
	StrongMarkers          int      `json:"strong_markers"`
	WeakMarkers            int      `json:"weak_markers"`
	InstructionSpecificity float64  `json:"instruction_specificity"`
	SectionCount           int      `json:"section_count"`
	ListItemCount          int      `json:"list_item_count"`
}

// ContaminationReport holds cross-language contamination metrics.
type ContaminationReport struct {
	MultiInterfaceTools  []string           `json:"multi_interface_tools"`
	CodeLanguages        []string           `json:"code_languages"`
	LanguageCategories   []string           `json:"language_categories"`
	PrimaryCategory      string             `json:"primary_category"`
	MismatchedCategories []string           `json:"mismatched_categories"`
	MismatchWeights      map[string]float64 `json:"mismatch_weights"`
	LanguageMismatch     bool               `json:"language_mismatch"`
	TechReferences       []string           `json:"tech_references"`
	ScopeBreadth         int                `json:"scope_breadth"`
	ContaminationScore   float64            `json:"contamination_score"`
	ContaminationLevel   string             `json:"contamination_level"`
}

// ReferenceFileReport holds per-file content and contamination analysis for a single reference file.
type ReferenceFileReport struct {
	File                string
	ContentReport       *ContentReport
	ContaminationReport *ContaminationReport
}

// Report holds all validation results and token counts.
type Report struct {
	SkillDir                      string
	Results                       []Result
	TokenCounts                   []TokenCount
	OtherTokenCounts              []TokenCount
	ContentReport                 *ContentReport
	ReferencesContentReport       *ContentReport
	ContaminationReport           *ContaminationReport
	ReferencesContaminationReport *ContaminationReport
	ReferenceReports              []ReferenceFileReport
	Errors                        int
	Warnings                      int
}

// Tally counts errors and warnings in the report.
func (r *Report) Tally() {
	r.Errors = 0
	r.Warnings = 0
	for _, result := range r.Results {
		switch result.Level {
		case Error:
			r.Errors++
		case Warning:
			r.Warnings++
		}
	}
}

// SkillMode indicates what kind of skill directory was detected.
type SkillMode int

const (
	// NoSkill means no SKILL.md was found in the directory.
	NoSkill SkillMode = iota
	// SingleSkill means the directory itself contains a SKILL.md.
	SingleSkill
	// MultiSkill means the directory contains subdirectories with SKILL.md files.
	MultiSkill
)

// MultiReport holds aggregated results from validating multiple skills.
type MultiReport struct {
	Skills   []*Report
	Errors   int
	Warnings int
}

// DimensionScore holds a single scoring dimension's display name and value.
type DimensionScore struct {
	Label string // Display name, e.g., "Token Efficiency"
	Value int    // Score value, typically 1-5
}

// Scored is the interface implemented by both SkillScores and RefScores.
// It allows formatting code to iterate dimensions generically.
type Scored interface {
	DimensionScores() []DimensionScore
	OverallScore() float64
	Assessment() string
	NovelDetails() string
}
