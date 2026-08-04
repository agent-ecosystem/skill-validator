package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/agent-ecosystem/skill-validator/skillcheck"
	"github.com/agent-ecosystem/skill-validator/types"
)

const version = "v1.6.0"

var (
	outputFormat    string
	emitAnnotations bool
)

var rootCmd = &cobra.Command{
	Use:   "skill-validator",
	Short: "Validate and analyze agent skills",
	Long:  "A CLI for validating skill directory structure, analyzing content quality, and detecting cross-language contamination.",
	// Once a command starts running (args parsed successfully), don't print
	// usage on error — the error is operational, not a CLI mistake.
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		cmd.SilenceUsage = true
	},
}

func init() {
	rootCmd.Version = version
	rootCmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "text", "output format: text, compact, json, or markdown")
	rootCmd.PersistentFlags().BoolVar(&emitAnnotations, "emit-annotations", false, "emit GitHub Actions workflow command annotations (::error/::warning) alongside normal output")
}

// Execute runs the root command.
func Execute() {
	// We handle error printing ourselves so that exitCodeError (validation
	// failures) doesn't produce cobra's default "Error: exit code N" noise.
	rootCmd.SilenceErrors = true
	if err := rootCmd.Execute(); err != nil {
		if ec, ok := err.(exitCodeError); ok {
			// Validation failure — report was already printed.
			os.Exit(ec.code)
		}
		// CLI/usage error — print and exit.
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(ExitCobra)
	}
}

// resolvePath resolves a path argument to an absolute directory path.
func resolvePath(args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("path argument required")
	}

	dir := args[0]
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("resolving path: %w", err)
	}

	info, err := os.Stat(absDir)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("%s is not a valid directory", dir)
	}

	return absDir, nil
}

// detectAndResolve resolves the path and detects skills.
func detectAndResolve(args []string) (string, types.SkillMode, []string, error) {
	absDir, err := resolvePath(args)
	if err != nil {
		return "", 0, nil, err
	}

	mode, dirs := skillcheck.DetectSkills(absDir)
	if mode == types.NoSkill {
		return "", 0, nil, fmt.Errorf("no skills found in %s (expected SKILL.md or subdirectories containing SKILL.md)", args[0])
	}

	return absDir, mode, dirs, nil
}
