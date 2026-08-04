package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/agent-ecosystem/skill-validator/structure"
	"github.com/agent-ecosystem/skill-validator/types"
)

var (
	skipOrphans                 bool
	strictStructure             bool
	structAllowExtraFrontmatter bool
	structAllowFlatLayouts      bool
	structAllowDirs             []string
	structAllowNestedPaths      []string
)

var validateStructureCmd = &cobra.Command{
	Use:   "structure <path>",
	Short: "Validate skill structure (spec compliance, tokens, code fences, internal links)",
	Long:  "Checks that a skill directory conforms to the spec: structure, frontmatter fields, token limits, skill ratio, code fence integrity, and internal link validity.",
	Args:  cobra.ExactArgs(1),
	RunE:  runValidateStructure,
}

func init() {
	validateStructureCmd.Flags().BoolVar(&skipOrphans, "skip-orphans", false,
		"skip orphan file detection (unreferenced files in scripts/, references/, assets/)")
	validateStructureCmd.Flags().BoolVar(&strictStructure, "strict", false, "treat warnings as errors (exit 1 instead of 2)")
	validateStructureCmd.Flags().BoolVar(&structAllowExtraFrontmatter, "allow-extra-frontmatter", false,
		"suppress warnings for non-spec frontmatter fields")
	validateStructureCmd.Flags().BoolVar(&structAllowFlatLayouts, "allow-flat-layouts", false,
		"allow files at the skill root without warnings and treat them as standard content for token counting")
	validateStructureCmd.Flags().StringSliceVar(&structAllowDirs, "allow-dirs", nil,
		"comma-separated list of directory names to accept without warnings (e.g. --allow-dirs=evals,testing)")
	validateStructureCmd.Flags().StringSliceVar(&structAllowNestedPaths, "allow-nested-paths", nil,
		"comma-separated skill-relative paths where deep nesting is allowed (e.g. --allow-nested-paths=assets/components)")
	validateCmd.AddCommand(validateStructureCmd)
}

func runValidateStructure(cmd *cobra.Command, args []string) error {
	allowNestedPaths, err := structure.NormalizeRelativePaths(structAllowNestedPaths)
	if err != nil {
		return fmt.Errorf("invalid --allow-nested-paths: %w", err)
	}

	_, mode, dirs, err := detectAndResolve(args)
	if err != nil {
		return err
	}

	opts := structure.Options{
		SkipOrphans:           skipOrphans,
		AllowExtraFrontmatter: structAllowExtraFrontmatter,
		AllowFlatLayouts:      structAllowFlatLayouts,
		AllowDirs:             structAllowDirs,
		AllowNestedPaths:      allowNestedPaths,
	}
	eopts := exitOpts{strict: strictStructure}

	switch mode {
	case types.SingleSkill:
		r := structure.Validate(dirs[0], opts)
		return outputReportWithExitOpts(r, false, eopts)
	case types.MultiSkill:
		mr := structure.ValidateMulti(dirs, opts)
		return outputMultiReportWithExitOpts(mr, false, eopts)
	}
	return nil
}
