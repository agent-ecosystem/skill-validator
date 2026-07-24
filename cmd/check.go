package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/agent-ecosystem/skill-validator/orchestrate"
	"github.com/agent-ecosystem/skill-validator/structure"
	"github.com/agent-ecosystem/skill-validator/types"
)

var (
	checkOnly                  []string
	checkSkip                  []string
	perFileCheck               bool
	checkSkipOrphans           bool
	strictCheck                bool
	checkAllowExtraFrontmatter bool
	checkAllowFlatLayouts      bool
	checkAllowDirs             []string
	checkAllowNestedPaths      []string
)

var checkCmd = &cobra.Command{
	Use:   "check <path>",
	Short: "Run all checks (structure + links + content + contamination)",
	Long:  "Runs all validation and analysis checks. Use --only or --skip to select specific check groups.",
	Args:  cobra.ExactArgs(1),
	RunE:  runCheck,
}

func init() {
	checkCmd.Flags().StringSliceVar(&checkOnly, "only", nil, "check groups to run: structure,links,content,contamination (comma-separated or repeatable)")
	checkCmd.Flags().StringSliceVar(&checkSkip, "skip", nil, "check groups to skip: structure,links,content,contamination (comma-separated or repeatable)")
	checkCmd.Flags().BoolVar(&perFileCheck, "per-file", false, "show per-file reference analysis")
	checkCmd.Flags().BoolVar(&checkSkipOrphans, "skip-orphans", false,
		"skip orphan file detection (unreferenced files in scripts/, references/, assets/)")
	checkCmd.Flags().BoolVar(&strictCheck, "strict", false, "treat warnings as errors (exit 1 instead of 2)")
	checkCmd.Flags().BoolVar(&checkAllowExtraFrontmatter, "allow-extra-frontmatter", false,
		"suppress warnings for non-spec frontmatter fields")
	checkCmd.Flags().BoolVar(&checkAllowFlatLayouts, "allow-flat-layouts", false,
		"allow files at the skill root without warnings and treat them as standard content for token counting")
	checkCmd.Flags().StringSliceVar(&checkAllowDirs, "allow-dirs", nil,
		"comma-separated list of directory names to accept without warnings (e.g. --allow-dirs=evals,testing)")
	checkCmd.Flags().StringSliceVar(&checkAllowNestedPaths, "allow-nested-paths", nil,
		"comma-separated skill-relative paths where deep nesting is allowed (e.g. --allow-nested-paths=assets/components)")
	rootCmd.AddCommand(checkCmd)
}

var validGroups = map[orchestrate.CheckGroup]bool{
	orchestrate.GroupStructure:     true,
	orchestrate.GroupLinks:         true,
	orchestrate.GroupContent:       true,
	orchestrate.GroupContamination: true,
}

func runCheck(cmd *cobra.Command, args []string) error {
	if len(checkOnly) > 0 && len(checkSkip) > 0 {
		return fmt.Errorf("--only and --skip are mutually exclusive")
	}

	allowNestedPaths, err := structure.NormalizeRelativePaths(checkAllowNestedPaths)
	if err != nil {
		return fmt.Errorf("invalid --allow-nested-paths: %w", err)
	}

	enabled, err := resolveCheckGroups(checkOnly, checkSkip)
	if err != nil {
		return err
	}

	_, mode, dirs, err := detectAndResolve(args)
	if err != nil {
		return err
	}

	opts := orchestrate.Options{
		Enabled: enabled,
		StructOpts: structure.Options{
			SkipOrphans:           checkSkipOrphans,
			AllowExtraFrontmatter: checkAllowExtraFrontmatter,
			AllowFlatLayouts:      checkAllowFlatLayouts,
			AllowDirs:             checkAllowDirs,
			AllowNestedPaths:      allowNestedPaths,
		},
	}
	eopts := exitOpts{strict: strictCheck}
	ctx := context.Background()

	switch mode {
	case types.SingleSkill:
		r := orchestrate.RunAllChecks(ctx, dirs[0], opts)
		return outputReportWithExitOpts(r, perFileCheck, eopts)
	case types.MultiSkill:
		mr := &types.MultiReport{}
		for _, dir := range dirs {
			r := orchestrate.RunAllChecks(ctx, dir, opts)
			mr.Skills = append(mr.Skills, r)
			mr.Errors += r.Errors
			mr.Warnings += r.Warnings
		}
		return outputMultiReportWithExitOpts(mr, perFileCheck, eopts)
	}
	return nil
}

func resolveCheckGroups(only, skip []string) (map[orchestrate.CheckGroup]bool, error) {
	enabled := orchestrate.AllGroups()

	if len(only) > 0 {
		// Reset all to false, enable only specified
		for k := range enabled {
			enabled[k] = false
		}
		for _, g := range only {
			g = strings.TrimSpace(g)
			cg := orchestrate.CheckGroup(g)
			if !validGroups[cg] {
				return nil, fmt.Errorf("unknown check group %q (valid: structure, links, content, contamination)", g)
			}
			enabled[cg] = true
		}
	}

	if len(skip) > 0 {
		for _, g := range skip {
			g = strings.TrimSpace(g)
			cg := orchestrate.CheckGroup(g)
			if !validGroups[cg] {
				return nil, fmt.Errorf("unknown check group %q (valid: structure, links, content, contamination)", g)
			}
			enabled[cg] = false
		}
	}

	return enabled, nil
}
