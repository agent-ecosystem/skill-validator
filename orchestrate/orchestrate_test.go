package orchestrate

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agent-ecosystem/skill-validator/links"
	"github.com/agent-ecosystem/skill-validator/report"
	"github.com/agent-ecosystem/skill-validator/skillcheck"
	"github.com/agent-ecosystem/skill-validator/structure"
	"github.com/agent-ecosystem/skill-validator/types"
)

// fixtureDir returns the absolute path to a testdata fixture.
func fixtureDir(t *testing.T, name string) string {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("..", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("fixture %q not found: %v", name, err)
	}
	return dir
}

func TestRunAllChecks_AllEnabled(t *testing.T) {
	dir := fixtureDir(t, "valid-skill")

	opts := Options{
		Enabled:    AllGroups(),
		StructOpts: structure.Options{},
	}
	r := RunAllChecks(t.Context(), dir, opts)

	if r.Errors != 0 {
		t.Errorf("expected 0 errors, got %d", r.Errors)
		for _, res := range r.Results {
			if res.Level == types.Error {
				t.Logf("  error: %s: %s", res.Category, res.Message)
			}
		}
	}

	categories := map[string]bool{}
	for _, res := range r.Results {
		categories[res.Category] = true
	}
	if !categories["Structure"] {
		t.Error("expected Structure results")
	}
	if !categories["Frontmatter"] {
		t.Error("expected Frontmatter results")
	}

	if r.ContentReport == nil {
		t.Error("expected ContentReport to be set")
	}
	if r.ContaminationReport == nil {
		t.Error("expected ContaminationReport to be set")
	}

	hasAsset := false
	for _, tc := range r.TokenCounts {
		if strings.HasPrefix(tc.File, "assets/") {
			hasAsset = true
			break
		}
	}
	if !hasAsset {
		t.Error("expected asset files in TokenCounts for valid-skill with assets/ directory")
	}

	if r.ReferencesContentReport == nil {
		t.Error("expected ReferencesContentReport to be set for valid-skill")
	}
	if r.ReferencesContaminationReport == nil {
		t.Error("expected ReferencesContaminationReport to be set for valid-skill")
	}
	if len(r.ReferenceReports) == 0 {
		t.Error("expected per-file ReferenceReports to be set for valid-skill")
	}
}

func TestRunAllChecks_OnlyStructure(t *testing.T) {
	dir := fixtureDir(t, "valid-skill")

	opts := Options{
		Enabled: map[CheckGroup]bool{
			GroupStructure:     true,
			GroupLinks:         false,
			GroupContent:       false,
			GroupContamination: false,
		},
		StructOpts: structure.Options{},
	}
	r := RunAllChecks(t.Context(), dir, opts)

	hasMarkdown := false
	for _, res := range r.Results {
		if res.Category == "Markdown" {
			hasMarkdown = true
		}
	}
	if !hasMarkdown {
		t.Error("expected Markdown results from structure validation")
	}

	for _, res := range r.Results {
		if res.Category == "Links" {
			t.Errorf("unexpected Links result: %s: %s", res.Category, res.Message)
		}
	}
	if r.ContentReport != nil {
		t.Error("expected ContentReport to be nil when content is disabled")
	}
	if r.ReferencesContentReport != nil {
		t.Error("expected ReferencesContentReport to be nil when content is disabled")
	}
	if r.ContaminationReport != nil {
		t.Error("expected ContaminationReport to be nil when contamination is disabled")
	}
	if r.ReferencesContaminationReport != nil {
		t.Error("expected ReferencesContaminationReport to be nil when contamination is disabled")
	}
	if len(r.ReferenceReports) != 0 {
		t.Error("expected no ReferenceReports when both content and contamination are disabled")
	}
}

func TestRunAllChecks_OnlyLinks(t *testing.T) {
	dir := fixtureDir(t, "valid-skill")

	opts := Options{
		Enabled: map[CheckGroup]bool{
			GroupStructure:     false,
			GroupLinks:         true,
			GroupContent:       false,
			GroupContamination: false,
		},
	}
	r := RunAllChecks(t.Context(), dir, opts)

	for _, res := range r.Results {
		if res.Category == "Structure" || res.Category == "Frontmatter" || res.Category == "Tokens" {
			t.Errorf("unexpected structure result: %s: %s", res.Category, res.Message)
		}
	}

	for _, res := range r.Results {
		if res.Category == "Markdown" {
			t.Errorf("unexpected Markdown result in links-only check: %s: %s", res.Category, res.Message)
		}
	}
}

func TestRunAllChecks_SkipContamination(t *testing.T) {
	dir := fixtureDir(t, "valid-skill")

	opts := Options{
		Enabled: map[CheckGroup]bool{
			GroupStructure:     true,
			GroupLinks:         true,
			GroupContent:       true,
			GroupContamination: false,
		},
	}
	r := RunAllChecks(t.Context(), dir, opts)

	if r.ContentReport == nil {
		t.Error("expected ContentReport when content is enabled")
	}
	if r.ContaminationReport != nil {
		t.Error("expected ContaminationReport to be nil when contamination is skipped")
	}
	if r.ReferencesContentReport == nil {
		t.Error("expected ReferencesContentReport when content is enabled")
	}
	if r.ReferencesContaminationReport != nil {
		t.Error("expected ReferencesContaminationReport to be nil when contamination is skipped")
	}
	if len(r.ReferenceReports) == 0 {
		t.Fatal("expected ReferenceReports when content is enabled")
	}
	for _, fr := range r.ReferenceReports {
		if fr.ContentReport == nil {
			t.Error("expected per-file ContentReport when content is enabled")
		}
		if fr.ContaminationReport != nil {
			t.Error("expected nil per-file ContaminationReport when contamination is skipped")
		}
	}
}

func TestRunAllChecks_OnlyContentContamination(t *testing.T) {
	dir := fixtureDir(t, "rich-skill")

	opts := Options{
		Enabled: map[CheckGroup]bool{
			GroupStructure:     false,
			GroupLinks:         false,
			GroupContent:       true,
			GroupContamination: true,
		},
	}
	r := RunAllChecks(t.Context(), dir, opts)

	if r.ContentReport == nil {
		t.Error("expected ContentReport")
	}
	if r.ContaminationReport == nil {
		t.Error("expected ContaminationReport")
	}

	if r.ContentReport.CodeBlockCount != 4 {
		t.Errorf("expected 4 code blocks, got %d", r.ContentReport.CodeBlockCount)
	}

	foundMongo := false
	for _, tool := range r.ContaminationReport.MultiInterfaceTools {
		if tool == "mongodb" {
			foundMongo = true
		}
	}
	if !foundMongo {
		t.Error("expected mongodb multi-interface tool detection")
	}
}

func TestRunAllChecks_BrokenFrontmatter_AllChecks(t *testing.T) {
	dir := fixtureDir(t, "broken-frontmatter")

	opts := Options{Enabled: AllGroups()}
	r := RunAllChecks(t.Context(), dir, opts)

	if r.Errors == 0 {
		t.Error("expected errors for broken frontmatter")
	}
	foundFMError := false
	for _, res := range r.Results {
		if res.Level == types.Error && res.Category == "Frontmatter" {
			foundFMError = true
		}
	}
	if !foundFMError {
		t.Error("expected Frontmatter error result")
	}

	if r.ContentReport == nil {
		t.Fatal("expected ContentReport despite broken frontmatter")
	}
	if r.ContentReport.WordCount == 0 {
		t.Error("expected non-zero word count from content analysis")
	}
	if r.ContentReport.CodeBlockCount != 2 {
		t.Errorf("expected 2 code blocks (bash, python), got %d", r.ContentReport.CodeBlockCount)
	}
	if len(r.ContentReport.CodeLanguages) != 2 {
		t.Errorf("expected 2 code languages, got %d: %v",
			len(r.ContentReport.CodeLanguages), r.ContentReport.CodeLanguages)
	}

	if r.ContaminationReport == nil {
		t.Fatal("expected ContaminationReport despite broken frontmatter")
	}
	if r.ContaminationReport.ContaminationLevel == "" {
		t.Error("expected non-empty contamination level")
	}

	for _, res := range r.Results {
		if res.Category == "Links" {
			t.Errorf("unexpected Links result for broken-frontmatter skill: %s: %s",
				res.Category, res.Message)
		}
	}
}

func TestRunAllChecks_BrokenFrontmatter_OnlyContent(t *testing.T) {
	dir := fixtureDir(t, "broken-frontmatter")

	opts := Options{
		Enabled: map[CheckGroup]bool{
			GroupStructure:     false,
			GroupLinks:         false,
			GroupContent:       true,
			GroupContamination: false,
		},
	}
	r := RunAllChecks(t.Context(), dir, opts)

	if r.ContentReport == nil {
		t.Fatal("expected ContentReport for content-only check")
	}
	if r.ContentReport.WordCount == 0 {
		t.Error("expected non-zero word count")
	}
	if r.ContentReport.StrongMarkers == 0 {
		t.Error("expected strong markers (must, always, never)")
	}
}

func TestRunAllChecks_BrokenFrontmatter_OnlyContamination(t *testing.T) {
	dir := fixtureDir(t, "broken-frontmatter")

	opts := Options{
		Enabled: map[CheckGroup]bool{
			GroupStructure:     false,
			GroupLinks:         false,
			GroupContent:       false,
			GroupContamination: true,
		},
	}
	r := RunAllChecks(t.Context(), dir, opts)

	if r.ContaminationReport == nil {
		t.Fatal("expected ContaminationReport for contamination-only check")
	}
	if len(r.ContaminationReport.CodeLanguages) != 2 {
		t.Errorf("expected 2 code languages, got %d: %v",
			len(r.ContaminationReport.CodeLanguages), r.ContaminationReport.CodeLanguages)
	}
}

func TestRunAllChecks_OnlyContent_ReferencesHaveContentOnly(t *testing.T) {
	dir := fixtureDir(t, "valid-skill")

	opts := Options{
		Enabled: map[CheckGroup]bool{
			GroupStructure:     false,
			GroupLinks:         false,
			GroupContent:       true,
			GroupContamination: false,
		},
	}
	r := RunAllChecks(t.Context(), dir, opts)

	if r.ReferencesContentReport == nil {
		t.Error("expected ReferencesContentReport when content is enabled")
	}
	if r.ReferencesContaminationReport != nil {
		t.Error("expected nil ReferencesContaminationReport when contamination is disabled")
	}
	for _, fr := range r.ReferenceReports {
		if fr.ContentReport == nil {
			t.Error("expected per-file ContentReport when content is enabled")
		}
		if fr.ContaminationReport != nil {
			t.Error("expected nil per-file ContaminationReport when contamination is disabled")
		}
	}
}

func TestRunAllChecks_MultiSkill(t *testing.T) {
	dir := fixtureDir(t, "multi-skill")
	_, dirs := skillcheck.DetectSkills(dir)

	opts := Options{Enabled: AllGroups()}

	mr := &types.MultiReport{}
	for _, d := range dirs {
		r := RunAllChecks(t.Context(), d, opts)
		mr.Skills = append(mr.Skills, r)
		mr.Errors += r.Errors
		mr.Warnings += r.Warnings
	}

	if len(mr.Skills) != 3 {
		t.Fatalf("expected 3 skills, got %d", len(mr.Skills))
	}

	for i, r := range mr.Skills {
		if r.ContentReport == nil {
			t.Errorf("skill %d: expected ContentReport", i)
		}
		if r.ContaminationReport == nil {
			t.Errorf("skill %d: expected ContaminationReport", i)
		}
	}
}

// --- RunContaminationAnalysis tests ---

func TestRunContaminationAnalysis_ValidSkill(t *testing.T) {
	dir := fixtureDir(t, "valid-skill")
	r := RunContaminationAnalysis(dir)
	if r.ContaminationReport == nil {
		t.Fatal("expected ContaminationReport to be set")
	}
	if r.Errors != 0 {
		t.Errorf("expected 0 errors, got %d", r.Errors)
	}
	hasPass := false
	for _, res := range r.Results {
		if res.Level == types.Pass && res.Category == "Contamination" {
			hasPass = true
		}
	}
	if !hasPass {
		t.Error("expected pass result with Contamination category")
	}
	if r.ReferencesContentReport == nil {
		t.Error("expected ReferencesContentReport to be set for valid-skill")
	}
	if r.ReferencesContaminationReport == nil {
		t.Error("expected ReferencesContaminationReport to be set for valid-skill")
	}
	if len(r.ReferenceReports) == 0 {
		t.Fatal("expected per-file ReferenceReports for valid-skill")
	}
	if r.ReferenceReports[0].File != "guide.md" {
		t.Errorf("expected first reference file to be guide.md, got %s", r.ReferenceReports[0].File)
	}
}

func TestRunContaminationAnalysis_RichSkill(t *testing.T) {
	dir := fixtureDir(t, "rich-skill")
	r := RunContaminationAnalysis(dir)
	if r.ContaminationReport == nil {
		t.Fatal("expected ContaminationReport to be set")
	}
	if r.ContaminationReport.ContaminationScore <= 0 {
		t.Error("expected positive contamination score for rich-skill")
	}
	if r.ReferencesContentReport != nil {
		t.Error("expected nil ReferencesContentReport for skill without references")
	}
	if r.ReferencesContaminationReport != nil {
		t.Error("expected nil ReferencesContaminationReport for skill without references")
	}
	if len(r.ReferenceReports) != 0 {
		t.Error("expected no ReferenceReports for skill without references")
	}
}

func TestRunContaminationAnalysis_RichSkill_AppMismatch(t *testing.T) {
	dir := fixtureDir(t, "rich-skill")
	r := RunContaminationAnalysis(dir)
	cr := r.ContaminationReport
	if cr == nil {
		t.Fatal("expected ContaminationReport")
	}
	// rich-skill has bash+javascript+python+yaml: javascript↔python is app↔app mismatch
	if !cr.LanguageMismatch {
		t.Error("expected language mismatch for javascript↔python")
	}
	// Auxiliary categories (shell, config) should NOT appear in mismatches
	for _, cat := range cr.MismatchedCategories {
		if cat == "shell" || cat == "config" {
			t.Errorf("auxiliary category %q should not be in mismatched categories", cat)
		}
	}

	var buf bytes.Buffer
	report.Print(&buf, r, false)
	output := buf.String()
	if !strings.Contains(output, "Language mismatch") {
		t.Error("expected Language mismatch warning in output for app↔app mismatch")
	}
}

func TestRunContaminationAnalysis_AuxiliaryOnly_NoMismatch(t *testing.T) {
	dir := fixtureDir(t, "auxiliary-only-skill")
	r := RunContaminationAnalysis(dir)
	cr := r.ContaminationReport
	if cr == nil {
		t.Fatal("expected ContaminationReport")
	}
	if cr.LanguageMismatch {
		t.Error("expected no language mismatch for auxiliary-only languages")
	}
	if len(cr.MismatchedCategories) != 0 {
		t.Errorf("expected no mismatched categories, got %v", cr.MismatchedCategories)
	}

	var buf bytes.Buffer
	report.Print(&buf, r, false)
	output := buf.String()
	if strings.Contains(output, "Language mismatch") {
		t.Error("Language mismatch warning should not appear for auxiliary-only languages")
	}
}

func TestRunContaminationAnalysis_BrokenDir(t *testing.T) {
	dir := t.TempDir()
	r := RunContaminationAnalysis(dir)
	if r.Errors != 1 {
		t.Errorf("expected 1 error, got %d", r.Errors)
	}
	if r.ContaminationReport != nil {
		t.Error("expected nil ContaminationReport for broken dir")
	}
	if r.ReferencesContaminationReport != nil {
		t.Error("expected nil ReferencesContaminationReport for broken dir")
	}
	if len(r.ReferenceReports) != 0 {
		t.Error("expected no ReferenceReports for broken dir")
	}
}

func TestRunContaminationAnalysis_ReferencesValidSkill(t *testing.T) {
	dir := fixtureDir(t, "valid-skill")
	r := RunContaminationAnalysis(dir)
	if r.ReferencesContaminationReport == nil {
		t.Fatal("expected ReferencesContaminationReport for valid-skill")
	}
	if len(r.ReferenceReports) == 0 {
		t.Fatal("expected per-file ReferenceReports for valid-skill")
	}
}

func TestRunContaminationAnalysis_NoReferences(t *testing.T) {
	dir := fixtureDir(t, "rich-skill")
	r := RunContaminationAnalysis(dir)
	if r.ReferencesContaminationReport != nil {
		t.Error("expected nil ReferencesContaminationReport for skill without references")
	}
	if len(r.ReferenceReports) != 0 {
		t.Error("expected no ReferenceReports for skill without references")
	}
}

// --- RunContentAnalysis tests ---

func TestRunContentAnalysis_ValidSkill(t *testing.T) {
	dir := fixtureDir(t, "valid-skill")
	r := RunContentAnalysis(dir)
	if r.ContentReport == nil {
		t.Fatal("expected ContentReport to be set")
	}
	if r.ContentReport.WordCount == 0 {
		t.Error("expected non-zero word count")
	}
	if r.Errors != 0 {
		t.Errorf("expected 0 errors, got %d", r.Errors)
	}
	if r.ReferencesContentReport == nil {
		t.Fatal("expected ReferencesContentReport to be set for valid-skill")
	}
	if r.ReferencesContentReport.WordCount == 0 {
		t.Error("expected non-zero word count in references content report")
	}
	if r.ReferencesContaminationReport == nil {
		t.Fatal("expected ReferencesContaminationReport to be set for valid-skill")
	}
	if len(r.ReferenceReports) == 0 {
		t.Fatal("expected per-file ReferenceReports for valid-skill")
	}
	if r.ReferenceReports[0].File != "guide.md" {
		t.Errorf("expected first reference file to be guide.md, got %s", r.ReferenceReports[0].File)
	}
	if r.ReferenceReports[0].ContentReport == nil {
		t.Error("expected per-file ContentReport to be set")
	}
	if r.ReferenceReports[0].ContaminationReport == nil {
		t.Error("expected per-file ContaminationReport to be set")
	}
}

func TestRunContentAnalysis_BrokenDir(t *testing.T) {
	dir := t.TempDir()
	r := RunContentAnalysis(dir)
	if r.Errors != 1 {
		t.Errorf("expected 1 error, got %d", r.Errors)
	}
	if r.ContentReport != nil {
		t.Error("expected nil ContentReport for broken dir")
	}
	if r.ReferencesContentReport != nil {
		t.Error("expected nil ReferencesContentReport for broken dir")
	}
	if r.ReferencesContaminationReport != nil {
		t.Error("expected nil ReferencesContaminationReport for broken dir")
	}
	if len(r.ReferenceReports) != 0 {
		t.Error("expected no ReferenceReports for broken dir")
	}
}

func TestRunContentAnalysis_NoReferences(t *testing.T) {
	dir := fixtureDir(t, "rich-skill")
	r := RunContentAnalysis(dir)
	if r.ContentReport == nil {
		t.Fatal("expected ContentReport to be set")
	}
	if r.ReferencesContentReport != nil {
		t.Error("expected nil ReferencesContentReport for skill without references")
	}
}

func TestRunContentAnalysis_NoReferencesContamination(t *testing.T) {
	dir := fixtureDir(t, "rich-skill")
	r := RunContentAnalysis(dir)
	if r.ReferencesContaminationReport != nil {
		t.Error("expected nil ReferencesContaminationReport for skill without references")
	}
	if len(r.ReferenceReports) != 0 {
		t.Error("expected no ReferenceReports for skill without references")
	}
}

// --- RunLinkChecks tests ---

func TestRunLinkChecks_ValidSkill(t *testing.T) {
	dir := fixtureDir(t, "valid-skill")
	r := RunLinkChecks(t.Context(), dir, links.Options{})
	if r.Errors != 0 {
		t.Errorf("expected 0 errors, got %d", r.Errors)
		for _, res := range r.Results {
			if res.Level == types.Error {
				t.Logf("  error: %s: %s", res.Category, res.Message)
			}
		}
	}
	hasLinks := false
	for _, res := range r.Results {
		if res.Category == "Links" {
			hasLinks = true
		}
	}
	if !hasLinks {
		t.Error("expected Links results from link checks")
	}
}

func TestRunLinkChecks_InvalidSkill(t *testing.T) {
	dir := fixtureDir(t, "invalid-skill")
	r := RunLinkChecks(t.Context(), dir, links.Options{})
	if r.Errors == 0 {
		t.Error("expected errors for invalid skill with broken links")
	}
}

func TestRunLinkChecks_BrokenDir(t *testing.T) {
	dir := t.TempDir()
	r := RunLinkChecks(t.Context(), dir, links.Options{})
	if r.Errors != 1 {
		t.Errorf("expected 1 error, got %d", r.Errors)
	}
}

// --- JSON output tests ---

func TestRunAllChecks_JSONOutput(t *testing.T) {
	dir := fixtureDir(t, "rich-skill")

	opts := Options{Enabled: AllGroups()}
	r := RunAllChecks(t.Context(), dir, opts)

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")

	type jsonCheck struct {
		ContentAnalysis       interface{} `json:"content_analysis,omitempty"`
		ContaminationAnalysis interface{} `json:"contamination_analysis,omitempty"`
	}

	out := jsonCheck{
		ContentAnalysis:       r.ContentReport,
		ContaminationAnalysis: r.ContaminationReport,
	}

	if err := enc.Encode(out); err != nil {
		t.Fatal(err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatal(err)
	}

	ca, ok := parsed["content_analysis"].(map[string]any)
	if !ok {
		t.Fatal("expected content_analysis object in JSON")
	}
	if ca["word_count"].(float64) == 0 {
		t.Error("expected non-zero word_count in JSON")
	}
	if ca["code_block_count"].(float64) != 4 {
		t.Errorf("expected 4 code_block_count, got %v", ca["code_block_count"])
	}

	ra, ok := parsed["contamination_analysis"].(map[string]any)
	if !ok {
		t.Fatal("expected contamination_analysis object in JSON")
	}
	if ra["contamination_level"].(string) == "" {
		t.Error("expected non-empty contamination_level in JSON")
	}
	if ra["contamination_score"].(float64) <= 0 {
		t.Error("expected positive contamination_score in JSON")
	}
}

func TestOutputJSON_FullCheck_ValidSkill(t *testing.T) {
	dir := fixtureDir(t, "valid-skill")
	opts := Options{Enabled: AllGroups()}
	r := RunAllChecks(t.Context(), dir, opts)

	var buf bytes.Buffer
	if err := report.PrintJSON(&buf, r, false); err != nil {
		t.Fatalf("PrintJSON error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if parsed["passed"] != true {
		t.Error("expected passed=true")
	}
	if _, ok := parsed["content_analysis"]; !ok {
		t.Error("expected content_analysis in JSON")
	}
	if _, ok := parsed["references_content_analysis"]; !ok {
		t.Error("expected references_content_analysis in JSON for valid-skill")
	}
	if _, ok := parsed["contamination_analysis"]; !ok {
		t.Error("expected contamination_analysis in JSON")
	}
	if _, ok := parsed["references_contamination_analysis"]; !ok {
		t.Error("expected references_contamination_analysis in JSON for valid-skill")
	}
	if _, ok := parsed["token_counts"]; !ok {
		t.Error("expected token_counts in JSON")
	}
	if _, ok := parsed["reference_reports"]; ok {
		t.Error("expected no reference_reports in JSON without --per-file")
	}
}

func TestOutputJSON_FullCheck_RichSkill(t *testing.T) {
	dir := fixtureDir(t, "rich-skill")
	opts := Options{Enabled: AllGroups()}
	r := RunAllChecks(t.Context(), dir, opts)

	var buf bytes.Buffer
	if err := report.PrintJSON(&buf, r, false); err != nil {
		t.Fatalf("PrintJSON error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	ca := parsed["contamination_analysis"].(map[string]any)
	if ca["contamination_level"].(string) == "" {
		t.Error("expected non-empty contamination_level")
	}
	if ca["contamination_score"].(float64) <= 0 {
		t.Error("expected positive contamination_score")
	}
	if ca["language_mismatch"] != true {
		t.Error("expected language_mismatch=true")
	}

	tools := ca["multi_interface_tools"].([]any)
	foundMongo := false
	for _, tool := range tools {
		if tool.(string) == "mongodb" {
			foundMongo = true
		}
	}
	if !foundMongo {
		t.Error("expected mongodb in multi_interface_tools")
	}

	co := parsed["content_analysis"].(map[string]any)
	if co["word_count"].(float64) == 0 {
		t.Error("expected non-zero word_count")
	}
	if co["code_block_count"].(float64) != 4 {
		t.Errorf("expected 4 code_block_count, got %v", co["code_block_count"])
	}
}

func TestOutputJSON_MultiSkill(t *testing.T) {
	dir := fixtureDir(t, "multi-skill")
	_, dirs := skillcheck.DetectSkills(dir)

	opts := Options{Enabled: AllGroups()}

	mr := &types.MultiReport{}
	for _, d := range dirs {
		r := RunAllChecks(t.Context(), d, opts)
		mr.Skills = append(mr.Skills, r)
		mr.Errors += r.Errors
		mr.Warnings += r.Warnings
	}

	var buf bytes.Buffer
	if err := report.PrintMultiJSON(&buf, mr, false); err != nil {
		t.Fatalf("PrintMultiJSON error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	skills := parsed["skills"].([]any)
	if len(skills) != 3 {
		t.Fatalf("expected 3 skills in JSON, got %d", len(skills))
	}

	for i, s := range skills {
		sk := s.(map[string]any)
		if _, ok := sk["contamination_analysis"]; !ok {
			t.Errorf("skill %d: expected contamination_analysis in JSON", i)
		}
		if _, ok := sk["content_analysis"]; !ok {
			t.Errorf("skill %d: expected content_analysis in JSON", i)
		}
	}
}

func TestOutputJSON_PerFile_ValidSkill(t *testing.T) {
	dir := fixtureDir(t, "valid-skill")
	opts := Options{Enabled: AllGroups()}
	r := RunAllChecks(t.Context(), dir, opts)

	var buf bytes.Buffer
	if err := report.PrintJSON(&buf, r, true); err != nil {
		t.Fatalf("PrintJSON error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	rr, ok := parsed["reference_reports"].([]any)
	if !ok {
		t.Fatal("expected reference_reports array in JSON with --per-file")
	}
	if len(rr) == 0 {
		t.Fatal("expected at least one reference report")
	}

	first := rr[0].(map[string]any)
	if first["file"].(string) != "guide.md" {
		t.Errorf("expected file=guide.md, got %s", first["file"])
	}
	if _, ok := first["content_analysis"]; !ok {
		t.Error("expected content_analysis in per-file report")
	}
	if _, ok := first["contamination_analysis"]; !ok {
		t.Error("expected contamination_analysis in per-file report")
	}
}

func TestRunLinkChecks_WithIgnorePatterns(t *testing.T) {
	// invalid-skill has https://httpstat.us/404 as its only HTTP link.
	dir := fixtureDir(t, "invalid-skill")

	t.Run("ignored URL produces no errors", func(t *testing.T) {
		opts := links.Options{IgnorePatterns: []string{"httpstat.us"}}
		r := RunLinkChecks(t.Context(), dir, opts)
		if r.Errors != 0 {
			t.Errorf("expected 0 errors when matching URL is ignored, got %d", r.Errors)
			for _, res := range r.Results {
				if res.Level == types.Error {
					t.Logf("  error: %s: %s", res.Category, res.Message)
				}
			}
		}
	})

	t.Run("non-matching pattern leaves link checked", func(t *testing.T) {
		opts := links.Options{IgnorePatterns: []string{"some-other-host.example.com"}}
		r := RunLinkChecks(t.Context(), dir, opts)
		if r.Errors == 0 {
			t.Error("expected errors when ignore pattern does not match the broken link")
		}
	})
}

func TestRunAllChecks_WithIgnoreLinks(t *testing.T) {
	// invalid-skill has https://httpstat.us/404 as its only HTTP link.
	dir := fixtureDir(t, "invalid-skill")

	t.Run("ignored link produces no Links errors", func(t *testing.T) {
		opts := Options{
			Enabled: map[CheckGroup]bool{
				GroupStructure:     false,
				GroupLinks:         true,
				GroupContent:       false,
				GroupContamination: false,
			},
			LinksOpts: links.Options{IgnorePatterns: []string{"httpstat.us"}},
		}
		r := RunAllChecks(t.Context(), dir, opts)
		for _, res := range r.Results {
			if res.Category == "Links" && res.Level == types.Error {
				t.Errorf("unexpected Links error (should have been ignored): %s", res.Message)
			}
		}
	})
}
