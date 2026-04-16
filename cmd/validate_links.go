package cmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/agent-ecosystem/skill-validator/links"
	"github.com/agent-ecosystem/skill-validator/orchestrate"
	"github.com/agent-ecosystem/skill-validator/types"
)

var validateLinksIgnore []string

var validateLinksCmd = &cobra.Command{
	Use:   "links <path>",
	Short: "Check external link validity (HTTP/HTTPS)",
	Long:  "Validates external (HTTP/HTTPS) links in SKILL.md. Internal (relative) links are checked by validate structure.",
	Args:  cobra.ExactArgs(1),
	RunE:  runValidateLinks,
}

func init() {
	validateLinksCmd.Flags().StringSliceVar(&validateLinksIgnore, "ignore-link", nil,
		"URL patterns to skip (case-insensitive substring match; repeatable or comma-separated, e.g. --ignore-link=github.com/myorg,localhost)")
	validateCmd.AddCommand(validateLinksCmd)
}

func runValidateLinks(cmd *cobra.Command, args []string) error {
	_, mode, dirs, err := detectAndResolve(args)
	if err != nil {
		return err
	}

	ctx := context.Background()
	lopts := links.Options{IgnorePatterns: validateLinksIgnore}

	switch mode {
	case types.SingleSkill:
		r := orchestrate.RunLinkChecks(ctx, dirs[0], lopts)
		return outputReport(r)
	case types.MultiSkill:
		mr := &types.MultiReport{}
		for _, dir := range dirs {
			r := orchestrate.RunLinkChecks(ctx, dir, lopts)
			mr.Skills = append(mr.Skills, r)
			mr.Errors += r.Errors
			mr.Warnings += r.Warnings
		}
		return outputMultiReport(mr)
	}
	return nil
}
