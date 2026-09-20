package report

import (
	"encoding/json"
	"io"

	"github.com/agent-ecosystem/skill-validator/types"
)

type jsonReport struct {
	SkillDir                        string                     `json:"skill_dir"`
	Passed                          bool                       `json:"passed"`
	Errors                          int                        `json:"errors"`
	Warnings                        int                        `json:"warnings"`
	Results                         []jsonResult               `json:"results"`
	TokenCounts                     *jsonTokenCounts           `json:"token_counts,omitempty"`
	OtherTokenCounts                *jsonTokenCounts           `json:"other_token_counts,omitempty"`
	ContentAnalysis                 *types.ContentReport       `json:"content_analysis,omitempty"`
	ReferencesContentAnalysis       *types.ContentReport       `json:"references_content_analysis,omitempty"`
	ContaminationAnalysis           *types.ContaminationReport `json:"contamination_analysis,omitempty"`
	ReferencesContaminationAnalysis *types.ContaminationReport `json:"references_contamination_analysis,omitempty"`
	ReferenceReports                []jsonReferenceFileReport  `json:"reference_reports,omitempty"`
}

type jsonReferenceFileReport struct {
	File                  string                     `json:"file"`
	ContentAnalysis       *types.ContentReport       `json:"content_analysis,omitempty"`
	ContaminationAnalysis *types.ContaminationReport `json:"contamination_analysis,omitempty"`
}

type jsonResult struct {
	Level    string `json:"level"`
	Category string `json:"category"`
	Message  string `json:"message"`
	File     string `json:"file,omitempty"`
	Line     int    `json:"line,omitempty"`
}

type jsonTokenCounts struct {
	Files []jsonTokenCount `json:"files"`
	Total int              `json:"total"`
}

type jsonTokenCount struct {
	File      string `json:"file"`
	Tokens    int    `json:"tokens"`
	Truncated bool   `json:"truncated,omitempty"`
}

type jsonMultiReport struct {
	Passed   bool         `json:"passed"`
	Errors   int          `json:"errors"`
	Warnings int          `json:"warnings"`
	Skills   []jsonReport `json:"skills"`
}

func buildJSONReport(r *types.Report, perFile bool) jsonReport {
	out := jsonReport{
		SkillDir: r.SkillDir,
		Passed:   r.Errors == 0,
		Errors:   r.Errors,
		Warnings: r.Warnings,
		Results:  make([]jsonResult, len(r.Results)),
	}

	for i, res := range r.Results {
		out.Results[i] = jsonResult{
			Level:    res.Level.String(),
			Category: res.Category,
			Message:  res.Message,
			File:     res.File,
			Line:     res.Line,
		}
	}

	if len(r.TokenCounts) > 0 {
		tc := &jsonTokenCounts{
			Files: make([]jsonTokenCount, len(r.TokenCounts)),
		}
		for i, c := range r.TokenCounts {
			tc.Files[i] = jsonTokenCount{File: c.File, Tokens: c.Tokens, Truncated: c.Truncated}
			tc.Total += c.Tokens
		}
		out.TokenCounts = tc
	}

	if len(r.OtherTokenCounts) > 0 {
		otc := &jsonTokenCounts{
			Files: make([]jsonTokenCount, len(r.OtherTokenCounts)),
		}
		for i, c := range r.OtherTokenCounts {
			otc.Files[i] = jsonTokenCount{File: c.File, Tokens: c.Tokens, Truncated: c.Truncated}
			otc.Total += c.Tokens
		}
		out.OtherTokenCounts = otc
	}

	out.ContentAnalysis = r.ContentReport
	out.ReferencesContentAnalysis = r.ReferencesContentReport
	out.ContaminationAnalysis = r.ContaminationReport
	out.ReferencesContaminationAnalysis = r.ReferencesContaminationReport

	if perFile && len(r.ReferenceReports) > 0 {
		out.ReferenceReports = make([]jsonReferenceFileReport, len(r.ReferenceReports))
		for i, fr := range r.ReferenceReports {
			out.ReferenceReports[i] = jsonReferenceFileReport{
				File:                  fr.File,
				ContentAnalysis:       fr.ContentReport,
				ContaminationAnalysis: fr.ContaminationReport,
			}
		}
	}

	return out
}

// PrintJSON writes the report as JSON to the given writer.
func PrintJSON(w io.Writer, r *types.Report, perFile bool) error {
	out := buildJSONReport(r, perFile)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

// PrintMultiJSON writes the multi-skill report as JSON to the given writer.
func PrintMultiJSON(w io.Writer, mr *types.MultiReport, perFile bool) error {
	out := jsonMultiReport{
		Passed:   mr.Errors == 0,
		Errors:   mr.Errors,
		Warnings: mr.Warnings,
		Skills:   make([]jsonReport, len(mr.Skills)),
	}
	for i, r := range mr.Skills {
		out.Skills[i] = buildJSONReport(r, perFile)
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
