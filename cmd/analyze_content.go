package cmd

import (
	"github.com/spf13/cobra"

	"github.com/agent-ecosystem/skill-validator/orchestrate"
	"github.com/agent-ecosystem/skill-validator/types"
)

var perFileContent bool

var analyzeContentCmd = &cobra.Command{
	Use:   "content <path>",
	Short: "Analyze content quality metrics",
	Long:  "Computes content metrics: word count, code block ratio, imperative ratio, information density, instruction specificity, and more.",
	Args:  cobra.ExactArgs(1),
	RunE:  runAnalyzeContent,
}

func init() {
	analyzeContentCmd.Flags().BoolVar(&perFileContent, "per-file", false, "show per-file reference analysis")
	analyzeCmd.AddCommand(analyzeContentCmd)
}

func runAnalyzeContent(cmd *cobra.Command, args []string) error {
	_, mode, dirs, err := detectAndResolve(args)
	if err != nil {
		return err
	}

	switch mode {
	case types.SingleSkill:
		r := orchestrate.RunContentAnalysis(dirs[0])
		return outputReportWithPerFile(r, perFileContent)
	case types.MultiSkill:
		mr := &types.MultiReport{}
		for _, dir := range dirs {
			r := orchestrate.RunContentAnalysis(dir)
			mr.Skills = append(mr.Skills, r)
			mr.Errors += r.Errors
			mr.Warnings += r.Warnings
		}
		return outputMultiReportWithPerFile(mr, perFileContent)
	}
	return nil
}
