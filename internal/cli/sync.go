package cli

import (
	"errors"
	"flag"
	"fmt"

	"github.com/workflowaiorg/workflow/internal/base"
	"github.com/workflowaiorg/workflow/internal/project"
	"github.com/workflowaiorg/workflow/internal/render"
)

func runSync(app *App, args []string) error {
	flags := flag.NewFlagSet("sync", flag.ContinueOnError)
	check := flags.Bool("check", false, "compare only; write nothing")
	force := flags.Bool("force", false, "replace files at generated paths that wf did not write")
	if err := app.parseFlags(flags, args); err != nil {
		return err
	}
	if flags.NArg() > 0 {
		return fmt.Errorf("%w: takes no arguments besides --check or --force", errUsage)
	}
	if *check && *force {
		return fmt.Errorf("%w: --check writes nothing, so it takes no --force", errUsage)
	}

	config, err := project.Load(app.dir)
	if err != nil {
		return err
	}
	if *check {
		return app.check(config)
	}
	report, err := render.Sync(app.dir, config, app.version, *force)
	return app.reportSync(report, err)
}

// check compares the generated files, and .agents/base, with what this version of wf writes.
func (app *App) check(config project.Config) error {
	differences, err := render.Check(app.dir, config, app.version)
	if err != nil {
		return err
	}
	baseDifferences, err := base.Check(app.dir, config, app.version)
	if err != nil {
		return err
	}
	differences = append(differences, baseDifferences...)
	if len(differences) > 0 {
		for _, difference := range differences {
			fmt.Fprintln(app.stderr, "  "+difference)
		}
		if len(differences) == 1 {
			return fmt.Errorf("1 file differs from what wf %s writes", app.version)
		}
		return fmt.Errorf("%d files differ from what wf %s writes", len(differences), app.version)
	}
	fmt.Fprintln(app.stdout, "Every generated file is up to date, and .agents/base is wf's.")
	return nil
}

// reportSync prints what a sync did and returns its error. What was done before a failure is
// still printed, so the developer knows what changed.
func (app *App) reportSync(report render.Report, err error) error {
	for _, path := range report.Written {
		fmt.Fprintln(app.stdout, "wrote    "+path)
	}
	for _, path := range report.Removed {
		fmt.Fprintln(app.stdout, "removed  "+path)
	}
	// What wf overwrote or left behind of someone's own goes to stderr, where it is not missed.
	for _, path := range report.Replaced {
		fmt.Fprintln(app.stderr, "replaced "+path+" (it was edited by hand, or not written by wf)")
	}
	for _, path := range report.Kept {
		fmt.Fprintln(app.stderr, "kept     "+path+" (no longer generated, but edited by hand: remove it yourself)")
	}
	for _, path := range report.Skipped {
		fmt.Fprintln(app.stderr, "skipped  "+path+" (not written by wf)")
	}
	fmt.Fprintf(app.stdout, "%d written, %d replaced, %d removed, %d kept, %d skipped, %d unchanged.\n",
		len(report.Written), len(report.Replaced), len(report.Removed), len(report.Kept), len(report.Skipped), report.Unchanged)
	if err != nil {
		return err
	}
	if len(report.Skipped) > 0 {
		return errors.New("files wf did not write were left alone; wf sync --force replaces them")
	}
	return nil
}

func runShow(app *App, args []string) error {
	flags := flag.NewFlagSet("show", flag.ContinueOnError)
	baseOnly := flags.Bool("base", false, "print wf's part alone")
	if err := app.parseFlags(flags, args); err != nil {
		return err
	}
	// The skill's name may come before the flag as well as after it.
	var names []string
	if flags.NArg() > 0 {
		names = append(names, flags.Arg(0))
		if err := app.parseFlags(flags, flags.Args()[1:]); err != nil {
			return err
		}
		names = append(names, flags.Args()...)
	}
	if len(names) != 1 {
		return fmt.Errorf("%w: names one skill", errUsage)
	}

	config, err := project.Load(app.dir)
	if err != nil {
		return err
	}
	content, err := render.Skill(app.dir, config, app.version, names[0], *baseOnly)
	if err != nil {
		return err
	}
	_, err = app.stdout.Write(content)
	return err
}
