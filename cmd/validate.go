package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/agent-ecosystem/skill-validator/report"
	"github.com/agent-ecosystem/skill-validator/types"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate skill structure or links",
	Long:  "Parent command for structure and link validation subcommands.",
}

func init() {
	rootCmd.AddCommand(validateCmd)
}

func outputReport(r *types.Report) error {
	return outputReportWithExitOpts(r, false, exitOpts{})
}

func outputReportWithPerFile(r *types.Report, perFile bool) error {
	return outputReportWithExitOpts(r, perFile, exitOpts{})
}

func outputReportWithExitOpts(r *types.Report, perFile bool, opts exitOpts) error {
	switch outputFormat {
	case "json":
		if err := report.PrintJSON(os.Stdout, r, perFile); err != nil {
			return fmt.Errorf("writing JSON: %w", err)
		}
	case "markdown":
		if err := report.PrintMarkdown(os.Stdout, r, perFile); err != nil {
			return fmt.Errorf("writing markdown: %w", err)
		}
	case "compact":
		report.PrintCompact(os.Stdout, r)
	default:
		report.Print(os.Stdout, r, perFile)
	}
	if emitAnnotations {
		wd, _ := os.Getwd()
		report.PrintAnnotations(os.Stdout, r, wd)
	}
	if code := opts.resolve(r.Errors, r.Warnings); code != 0 {
		return exitCodeError{code: code}
	}
	return nil
}

func outputMultiReport(mr *types.MultiReport) error {
	return outputMultiReportWithExitOpts(mr, false, exitOpts{})
}

func outputMultiReportWithPerFile(mr *types.MultiReport, perFile bool) error {
	return outputMultiReportWithExitOpts(mr, perFile, exitOpts{})
}

func outputMultiReportWithExitOpts(mr *types.MultiReport, perFile bool, opts exitOpts) error {
	switch outputFormat {
	case "json":
		if err := report.PrintMultiJSON(os.Stdout, mr, perFile); err != nil {
			return fmt.Errorf("writing JSON: %w", err)
		}
	case "markdown":
		if err := report.PrintMultiMarkdown(os.Stdout, mr, perFile); err != nil {
			return fmt.Errorf("writing markdown: %w", err)
		}
	case "compact":
		report.PrintMultiCompact(os.Stdout, mr)
	default:
		report.PrintMulti(os.Stdout, mr, perFile)
	}
	if emitAnnotations {
		wd, _ := os.Getwd()
		report.PrintMultiAnnotations(os.Stdout, mr, wd)
	}
	if code := opts.resolve(mr.Errors, mr.Warnings); code != 0 {
		return exitCodeError{code: code}
	}
	return nil
}
