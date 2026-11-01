package cli

import (
	"errors"
	"flag"
	"fmt"
	"slices"

	"github.com/workflowaiorg/workflow/internal/component"
	"github.com/workflowaiorg/workflow/internal/doctor"
	"github.com/workflowaiorg/workflow/internal/project"
)

func (app *App) doctor() doctor.Doctor {
	return doctor.Doctor{Dir: app.dir, Stdout: app.stdout, Stderr: app.stderr}
}

func runDoctor(app *App, args []string) error {
	if err := app.noArgs("doctor", args); err != nil {
		return err
	}
	checks, err := app.checks()
	if err != nil || len(checks) == 0 {
		return err
	}
	results, err := app.doctor().Run(checks)
	if err != nil {
		return err
	}
	doctor.Report(app.stdout, results)
	problems, fixable := doctor.Count(results)
	if fixable > 0 {
		fmt.Fprintf(app.stdout, "\nwf scaffold fixes %d of these.\n", fixable)
	}
	if problems > 0 {
		return errors.New(doctor.ToFixByHand(problems))
	}
	return nil
}

func runScaffold(app *App, args []string) error {
	if err := app.noArgs("scaffold", args); err != nil {
		return err
	}
	checks, err := app.checks()
	if err != nil || len(checks) == 0 {
		return err
	}
	return app.doctor().Scaffold(checks)
}

// noArgs parses the flags of the command name, which takes neither flags nor arguments.
func (app *App) noArgs(name string, args []string) error {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	if err := app.parseFlags(flags, args); err != nil {
		return err
	}
	if flags.NArg() > 0 {
		return fmt.Errorf("%w: takes no arguments", errUsage)
	}
	return nil
}

// checks returns the doctor checks of the project: its components' first, then its own. A project
// with none is told where to add them.
func (app *App) checks() ([]project.Check, error) {
	config, installed, err := app.load()
	if err != nil {
		return nil, err
	}
	checks := slices.Concat(component.Doctor(installed), config.Doctor)
	if len(checks) == 0 {
		fmt.Fprintf(app.stdout, "No checks: add them under doctor: in %s.\n", project.ConfigFile)
	}
	return checks, nil
}
