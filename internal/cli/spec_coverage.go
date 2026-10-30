package cli

import (
	"errors"
	"flag"
	"fmt"

	"github.com/workflowaiorg/workflow/internal/specs"
)

func runSpecCoverage(app *App, args []string) error {
	flags := flag.NewFlagSet("spec-coverage", flag.ContinueOnError)
	reportOnly := flags.Bool("report-only", false, "print the report and exit 0")
	if err := app.parseFlags(flags, args); err != nil {
		return err
	}
	if flags.NArg() > 0 {
		return fmt.Errorf("%w: takes no arguments besides --report-only", errUsage)
	}

	report, err := specs.Check(app.dir, app.now())
	if err != nil {
		return err
	}
	report.Write(app.stdout)
	if len(report.Problems) == 0 {
		return nil
	}
	if *reportOnly {
		fmt.Fprintln(app.stdout, "\nReport-only: exiting 0. The same problems fail the check.")
		return nil
	}
	if len(report.Problems) == 1 {
		return errors.New("1 problem in .workflow")
	}
	return fmt.Errorf("%d problems in .workflow", len(report.Problems))
}
