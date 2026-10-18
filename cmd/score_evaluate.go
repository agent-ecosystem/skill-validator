package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/agent-ecosystem/skill-validator/evaluate"
	"github.com/agent-ecosystem/skill-validator/judge"
	"github.com/agent-ecosystem/skill-validator/report"
	"github.com/agent-ecosystem/skill-validator/types"
)

var (
	evalProvider          string
	evalModel             string
	evalBaseURL           string
	evalRescore           bool
	evalSkillOnly         bool
	evalRefsOnly          bool
	evalDisplay           string
	evalFullContent       bool
	evalMaxTokensStyle    string
	evalMaxResponseTokens int
)

var scoreEvaluateCmd = &cobra.Command{
	Use:   "evaluate <path>",
	Short: "Score a skill using an LLM judge",
	Long: `Score a skill's quality using an LLM-as-judge approach.

The path can be:
  - A skill directory (containing SKILL.md) — scores SKILL.md and references
  - A multi-skill parent directory — scores each skill
  - A specific .md file — scores just that reference file

Requires an API key via environment variable:
  ANTHROPIC_API_KEY  (for --provider anthropic, the default)
  OPENAI_API_KEY     (for --provider openai)

Optional OpenAI environment variables:
  OPENAI_BASE_URL    (API base URL; overridden by --base-url flag)
  OPENAI_ORG_ID      (organization ID; sent as OpenAI-Organization header)
  OPENAI_PROJECT_ID  (project ID; sent as OpenAI-Project header)

The claude-cli provider uses the locally installed "claude" CLI and does not
require an API key. This is useful when the CLI is already authenticated
(e.g. via a company or team subscription).`,
	Args: cobra.ExactArgs(1),
	RunE: runScoreEvaluate,
}

func init() {
	scoreEvaluateCmd.Flags().StringVar(&evalProvider, "provider", "anthropic", "LLM provider: anthropic, openai, or claude-cli")
	scoreEvaluateCmd.Flags().StringVar(&evalModel, "model", "", "model name (default: claude-sonnet-4-5-20250929 for anthropic, gpt-5.2 for openai, sonnet for claude-cli)")
	scoreEvaluateCmd.Flags().StringVar(&evalBaseURL, "base-url", "", "API base URL (for openai-compatible endpoints)")
	scoreEvaluateCmd.Flags().BoolVar(&evalRescore, "rescore", false, "re-score and overwrite cached results")
	scoreEvaluateCmd.Flags().BoolVar(&evalSkillOnly, "skill-only", false, "score only SKILL.md, skip reference files")
	scoreEvaluateCmd.Flags().BoolVar(&evalRefsOnly, "refs-only", false, "score only reference files, skip SKILL.md")
	scoreEvaluateCmd.Flags().StringVar(&evalDisplay, "display", "aggregate", "reference score display: aggregate or files")
	scoreEvaluateCmd.Flags().BoolVar(&evalFullContent, "full-content", false, "send full file content to LLM (default: truncate to 8,000 chars)")
	scoreEvaluateCmd.Flags().StringVar(&evalMaxTokensStyle, "max-tokens-style", "auto", "token parameter style: auto, max_tokens, or max_completion_tokens")
	scoreEvaluateCmd.Flags().IntVar(&evalMaxResponseTokens, "max-response-tokens", judge.DefaultMaxResponseTokens, "response budget in tokens per judge call; reasoning models think inside this budget before answering")
	scoreCmd.AddCommand(scoreEvaluateCmd)
}

func runScoreEvaluate(cmd *cobra.Command, args []string) error {
	if evalSkillOnly && evalRefsOnly {
		return fmt.Errorf("--skill-only and --refs-only are mutually exclusive")
	}

	if evalDisplay != "aggregate" && evalDisplay != "files" {
		return fmt.Errorf("--display must be \"aggregate\" or \"files\"")
	}

	// Validate --max-tokens-style
	switch evalMaxTokensStyle {
	case "auto", "max_tokens", "max_completion_tokens":
		// valid
	default:
		return fmt.Errorf("--max-tokens-style must be \"auto\", \"max_tokens\", or \"max_completion_tokens\"")
	}

	if evalMaxResponseTokens < 1 {
		return fmt.Errorf("--max-response-tokens must be at least 1")
	}

	// Resolve API key (not needed for claude-cli)
	var apiKey string
	if strings.ToLower(evalProvider) != "claude-cli" {
		var err error
		apiKey, err = resolveAPIKey(evalProvider)
		if err != nil {
			return err
		}
	}

	// For the openai provider, read optional env vars for base URL, org, and project.
	baseURL := evalBaseURL
	var orgID, projectID string
	if strings.ToLower(evalProvider) == "openai" {
		if baseURL == "" {
			baseURL = os.Getenv("OPENAI_BASE_URL")
		}
		orgID = os.Getenv("OPENAI_ORG_ID")
		projectID = os.Getenv("OPENAI_PROJECT_ID")
	}

	client, err := judge.NewClient(judge.ClientOptions{
		Provider:          evalProvider,
		APIKey:            apiKey,
		BaseURL:           baseURL,
		Model:             evalModel,
		MaxTokensStyle:    evalMaxTokensStyle,
		MaxResponseTokens: evalMaxResponseTokens,
		OrgID:             orgID,
		ProjectID:         projectID,
	})
	if err != nil {
		return err
	}

	opts := evaluate.Options{
		Rescore:   evalRescore,
		SkillOnly: evalSkillOnly,
		RefsOnly:  evalRefsOnly,
		MaxLen:    evalMaxLen(),
		Progress: func(event, detail string) {
			fmt.Fprintf(os.Stderr, "  %s: %s\n", event, detail)
		},
	}

	ctx := context.Background()
	path := args[0]

	// Check if path is a file (single reference scoring)
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolving path: %w", err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return fmt.Errorf("path not found: %s", path)
	}

	if !info.IsDir() {
		result, err := evaluate.EvaluateSingleFile(ctx, absPath, client, opts)
		if err != nil {
			return err
		}
		return report.FormatEvalResults(os.Stdout, []*evaluate.Result{result}, outputFormat, evalDisplay)
	}

	// Directory mode — detect skills
	_, mode, dirs, err := detectAndResolve(args)
	if err != nil {
		return err
	}

	switch mode {
	case types.SingleSkill:
		result, err := evaluate.EvaluateSkill(ctx, dirs[0], client, opts)
		if err != nil {
			return err
		}
		return report.FormatEvalResults(os.Stdout, []*evaluate.Result{result}, outputFormat, evalDisplay)

	case types.MultiSkill:
		var results []*evaluate.Result
		for _, dir := range dirs {
			result, err := evaluate.EvaluateSkill(ctx, dir, client, opts)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error scoring %s: %v\n", filepath.Base(dir), err)
				continue
			}
			results = append(results, result)
		}
		return report.FormatMultiEvalResults(os.Stdout, results, outputFormat, evalDisplay)
	}

	return nil
}

func evalMaxLen() int {
	if evalFullContent {
		return 0
	}
	return judge.DefaultMaxContentLen
}

func resolveAPIKey(provider string) (string, error) {
	switch strings.ToLower(provider) {
	case "anthropic":
		key := os.Getenv("ANTHROPIC_API_KEY")
		if key == "" {
			return "", fmt.Errorf("ANTHROPIC_API_KEY environment variable not set\n  Set it with: export ANTHROPIC_API_KEY=your-key-here")
		}
		return key, nil
	case "openai":
		key := os.Getenv("OPENAI_API_KEY")
		if key == "" {
			return "", fmt.Errorf("OPENAI_API_KEY environment variable not set\n  Set it with: export OPENAI_API_KEY=your-key-here")
		}
		return key, nil
	default:
		return "", fmt.Errorf("unsupported provider: %s", provider)
	}
}
