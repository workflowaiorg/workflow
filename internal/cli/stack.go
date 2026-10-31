package cli

import (
	"errors"
	"flag"
	"fmt"
	"slices"
	"strings"

	"github.com/workflowaiorg/workflow/internal/component"
	"github.com/workflowaiorg/workflow/internal/project"
	"github.com/workflowaiorg/workflow/internal/stack"
)

func (app *App) stack() stack.Stack {
	return stack.Stack{Dir: app.dir, Stdin: app.stdin, Stdout: app.stdout, Stderr: app.stderr}
}

func runUp(app *App, args []string) error {
	services, err := app.stackArgs("up", args)
	if err != nil {
		return err
	}
	if slices.ContainsFunc(services, func(arg string) bool { return strings.HasPrefix(arg, "-") }) {
		return fmt.Errorf("%w: takes only the names of services in the compose file", errUsage)
	}
	// Only in the project's root: from anywhere else, compose would look for a compose file in
	// the folders above, and might find another one.
	if _, _, err := app.load(); err != nil {
		return err
	}
	return app.stack().Up(services...)
}

func runDown(app *App, args []string) error {
	if err := app.noArgs("down", args); err != nil {
		return err
	}
	if _, _, err := app.load(); err != nil {
		return err
	}
	return app.stack().Down()
}

func runDoctor(app *App, args []string) error {
	if err := app.noArgs("doctor", args); err != nil {
		return err
	}
	checks, err := app.checks()
	if err != nil || len(checks) == 0 {
		return err
	}
	results, err := app.stack().Doctor(checks)
	if err != nil {
		return err
	}
	stack.Report(app.stdout, results)
	problems, fixable := stack.Count(results)
	if fixable > 0 {
		fmt.Fprintf(app.stdout, "\nwf scaffold fixes %d of these.\n", fixable)
	}
	if problems > 0 {
		return errors.New(stack.ToFixByHand(problems))
	}
	return nil
}

func runScaffold(app *App, args []string) error {
	if err := app.noArgs("scaffold", args); err != nil {
		return err
	}
	checks, err := app.checks()
	if err != nil {
		return err
	}
	return app.stack().Scaffold(checks)
}

// stackArgs parses the flags of the command name, which has none but -h, and returns the rest.
func (app *App) stackArgs(name string, args []string) ([]string, error) {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	if err := app.parseFlags(flags, args); err != nil {
		return nil, err
	}
	return flags.Args(), nil
}

// noArgs parses the flags of the command name, which takes neither flags nor arguments.
func (app *App) noArgs(name string, args []string) error {
	rest, err := app.stackArgs(name, args)
	if err != nil {
		return err
	}
	if len(rest) > 0 {
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
