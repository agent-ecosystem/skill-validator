package report

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/agent-ecosystem/skill-validator/types"
)

func escapeAnnotationData(s string) string {
	s = strings.ReplaceAll(s, "%", "%25")
	s = strings.ReplaceAll(s, "\r", "%0D")
	s = strings.ReplaceAll(s, "\n", "%0A")
	return s
}

func escapeAnnotationProperty(s string) string {
	s = escapeAnnotationData(s)
	s = strings.ReplaceAll(s, ":", "%3A")
	s = strings.ReplaceAll(s, ",", "%2C")
	return s
}

// PrintAnnotations writes GitHub Actions workflow command annotations for
// errors and warnings in the report. Pass and Info results are skipped.
// workDir is the working directory used to compute relative file paths;
// in CI this is typically the repository root.
func PrintAnnotations(w io.Writer, r *types.Report, workDir string) {
	for _, res := range r.Results {
		line := formatAnnotation(r.SkillDir, res, workDir)
		if line != "" {
			_, _ = fmt.Fprintln(w, line)
		}
	}
}

// PrintMultiAnnotations writes annotations for all skills in a multi-report.
func PrintMultiAnnotations(w io.Writer, mr *types.MultiReport, workDir string) {
	for _, r := range mr.Skills {
		PrintAnnotations(w, r, workDir)
	}
}

func formatAnnotation(skillDir string, res types.Result, workDir string) string {
	var cmd string
	switch res.Level {
	case types.Error:
		cmd = "error"
	case types.Warning:
		cmd = "warning"
	default:
		return ""
	}

	var params string
	if res.File != "" {
		absPath := filepath.Join(skillDir, res.File)
		relPath, err := filepath.Rel(workDir, absPath)
		if err != nil {
			relPath = absPath
		}
		params = fmt.Sprintf(" file=%s", escapeAnnotationProperty(filepath.ToSlash(relPath)))
		if res.Line > 0 {
			params += fmt.Sprintf(",line=%d", res.Line)
		}
		params += fmt.Sprintf(",title=%s", escapeAnnotationProperty(res.Category))
	} else {
		params = fmt.Sprintf(" title=%s", escapeAnnotationProperty(res.Category))
	}

	return fmt.Sprintf("::%s%s::%s", cmd, params, escapeAnnotationData(res.Message))
}
