package cli

import (
	"flag"
	"fmt"
	"regexp"

	"github.com/workflowaiorg/workflow/internal/worktree"
)

var (
	issueNumber = regexp.MustCompile(`^[1-9][0-9]*$`)
	kebabSlug   = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
)

func runWorktree(app *App, args []string) error {
	flags := flag.NewFlagSet("worktree", flag.ContinueOnError)
	if err := app.parseFlags(flags, args); err != nil {
		return err
	}
	if flags.NArg() != 2 {
		return fmt.Errorf("%w: names an Issue and a slug, or plan and a slug", errUsage)
	}
	issue, slug := flags.Arg(0), flags.Arg(1)
	if issue != "plan" && !issueNumber.MatchString(issue) {
		return fmt.Errorf("%w: %q is neither an Issue number nor plan", errUsage, issue)
	}
	if !kebabSlug.MatchString(slug) {
		return fmt.Errorf("%w: the slug %q is not short-kebab-case", errUsage, slug)
	}

	result, err := worktree.Open(app.dir, issue, slug)
	if err != nil {
		return err
	}
	for _, note := range result.Notes {
		fmt.Fprintln(app.stderr, note)
	}
	// The path comes last and alone on its line, for the agent that goes there next.
	_, err = fmt.Fprintln(app.stdout, result.Path)
	return err
}
