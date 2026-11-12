package cli

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
	"text/tabwriter"

	"github.com/workflowaiorg/workflow/internal/base"
	"github.com/workflowaiorg/workflow/internal/component"
	"github.com/workflowaiorg/workflow/internal/project"
	"github.com/workflowaiorg/workflow/internal/render"
)

func runAdd(app *App, args []string) error {
	if len(args) == 0 {
		return app.listComponents()
	}
	name := args[0]
	if name == "-h" || name == "--help" {
		return app.parseFlags(flag.NewFlagSet("add", flag.ContinueOnError), args)
	}
	index := slices.IndexFunc(app.components, func(c component.Component) bool { return c.Name == name })
	if index < 0 {
		return fmt.Errorf("%w: %q is not a component wf knows; %s", errUsage, name, component.Known(app.components))
	}
	c := app.components[index]

	// Each option is a flag; one left out is asked.
	flags := flag.NewFlagSet("add", flag.ContinueOnError)
	given := map[string]string{}
	for _, option := range c.Options {
		set := func(value string) error { given[option.Name] = value; return nil }
		if option.Bool {
			flags.BoolFunc(option.Name, option.Question, set)
		} else {
			flags.Func(option.Name, option.Question, set)
		}
	}
	force := flags.Bool("force", false, "replace files at generated paths that wf did not write")
	if slices.Contains(args[1:], "-h") || slices.Contains(args[1:], "--help") {
		return app.componentUsage(c)
	}
	if err := app.parseFlags(flags, args[1:]); err != nil {
		return err
	}
	if flags.NArg() > 0 {
		return fmt.Errorf("%w: takes the component's name and its options, not %q", errUsage, flags.Arg(0))
	}

	config, installed, err := app.load()
	if err != nil {
		return err
	}
	// The base belongs to the version the project pins; another version's would fail every check.
	if config.WF != app.version {
		return project.UserErrorf("%s: pins wf %s, but this is wf %s; run the version it pins", project.ConfigFile, config.WF, app.version)
	}
	if _, ok := config.Components[name]; ok {
		return fmt.Errorf("%w: %s is installed already; its answers are under components: in %s", errUsage, c, project.ConfigFile)
	}

	answers := map[string]any{}
	for _, option := range c.Options {
		text, ok := given[option.Name]
		if !ok {
			question, flags := option.Question, "--"+option.Name
			if option.Bool {
				question += " (yes or no)"
				flags += " for yes or --" + option.Name + "=false for no"
			} else if len(option.Choices) > 0 {
				question += " (" + strings.Join(option.Choices, ", ") + ")"
			}
			if text, err = app.ask(question, option.Default, flags); err != nil {
				return err
			}
		}
		if answers[option.Name], err = option.Answer(text); err != nil {
			return err
		}
	}

	c = c.With(answers)
	if c.Check != nil {
		if err := c.Check(app.dir, answers); err != nil {
			return err
		}
	}
	configFile, err := project.WithComponent(app.dir, name, answers)
	if err != nil {
		return err
	}
	// Rendered before anything is written: a template that fails would otherwise leave the base
	// installed for a component .wf.yml does not list.
	skeleton, err := base.RenderComponentSkeleton(c, answers)
	if err != nil {
		return err
	}
	installed = append(installed, c)
	slices.SortFunc(installed, func(a, b component.Component) int { return strings.Compare(a.Name, b.Name) })
	if err := base.Install(app.dir, config.Forge, installed); err != nil {
		return err
	}
	fmt.Fprintln(app.stdout, "wrote    "+project.BaseDir+"/")
	written, existing, err := base.WriteSkeleton(app.dir, skeleton)
	if err != nil {
		return err
	}
	for _, path := range written {
		fmt.Fprintln(app.stdout, "wrote    "+path+" (the project's from now on)")
	}
	// .wf.yml is written after the base, so an add that failed before it can be run again.
	if err := project.WriteAtomically(filepath.Join(app.dir, project.ConfigFile), configFile); err != nil {
		return fmt.Errorf("writing %s: %w", project.ConfigFile, err)
	}
	fmt.Fprintln(app.stdout, "wrote    "+project.ConfigFile)
	if config, err = project.Load(app.dir); err != nil {
		return err
	}

	report, syncErr := render.Sync(app.dir, config, installed, app.version, *force)
	err = app.reportSync(report, syncErr)
	// What the project owns is the developer's to change, so wf says what to change instead.
	for _, path := range existing {
		line := path + " was there already, so wf left it alone"
		if c.Existing != nil {
			if hint := c.Existing(path, skeleton[path]); hint != "" {
				line += ": " + hint
			}
		}
		fmt.Fprintln(app.stderr, "by hand  "+line)
	}
	if c.Edits != nil {
		for _, edit := range c.Edits(answers) {
			fmt.Fprintln(app.stderr, "by hand  "+edit)
		}
	}
	return err
}

// componentUsage prints how to add c: each option, the flag that answers it, and what it takes.
func (app *App) componentUsage(c component.Component) error {
	fmt.Fprintf(app.stdout, "usage: wf add %s [--<option> …] [--force]\n%s\n", c.Name, c.Summary)
	if len(c.Options) == 0 {
		return errHelpShown
	}
	fmt.Fprintln(app.stdout, "\nOptions, each asked when its flag is not given:")
	table := tabwriter.NewWriter(app.stdout, 0, 0, 3, ' ', 0)
	for _, option := range c.Options {
		flag, takes := "--"+option.Name+"=<answer>", ""
		switch {
		case option.Bool:
			flag, takes = "--"+option.Name+"[=false]", "yes or no; "
		case len(option.Choices) > 0:
			takes = strings.Join(option.Choices, ", ") + "; "
		}
		fallback := "no default"
		if option.Default != "" {
			fallback = "default " + option.Default
		}
		fmt.Fprintf(table, "  %s\t%s (%s%s)\n", flag, option.Question, takes, fallback)
	}
	if err := table.Flush(); err != nil {
		return err
	}
	return errHelpShown
}

// listComponents prints the components wf add offers, and which the project has.
func (app *App) listComponents() error {
	if len(app.components) == 0 {
		_, err := fmt.Fprintln(app.stdout, "wf carries no components yet.")
		return err
	}
	// Before wf init there is no .wf.yml and nothing is installed; a broken one is reported, not
	// read as nothing installed.
	_, installed, err := app.load()
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	table := tabwriter.NewWriter(app.stdout, 0, 0, 3, ' ', 0)
	for _, c := range app.components {
		state := ""
		if slices.ContainsFunc(installed, func(i component.Component) bool { return i.Name == c.Name }) {
			state = "(installed)"
		}
		fmt.Fprintf(table, "  %s\t%s\t%s\n", c.Name, c.Summary, state)
	}
	return table.Flush()
}
