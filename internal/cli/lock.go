package cli

import (
	"flag"
	"fmt"

	"github.com/workflowaiorg/workflow/internal/launcher"
	"github.com/workflowaiorg/workflow/internal/project"
)

func runLock(app *App, args []string) error {
	flags := flag.NewFlagSet("lock", flag.ContinueOnError)
	if err := app.parseFlags(flags, args); err != nil {
		return err
	}
	if flags.NArg() > 0 {
		return fmt.Errorf("%w: takes no arguments", errUsage)
	}
	l := launcher.New(app.version, app.dir)
	l.ReleasesURL = app.releases
	result, err := l.Lock()
	if err != nil {
		return err
	}
	if !result.Changed {
		fmt.Fprintf(app.stdout, "%s already holds wf %s's fingerprints.\n", project.SumFile, result.Version)
		return nil
	}
	fmt.Fprintf(app.stdout, "wrote    %s (wf %s's fingerprints: commit it)\n", project.SumFile, result.Version)
	return nil
}
