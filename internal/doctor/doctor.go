// Package doctor runs a project's doctor checks, which say whether the machine and the repo are
// ready, and the fixes wf scaffold runs for those that fail. It never starts or stops the app: the
// agent does that, the way the skills say.
package doctor

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"slices"
	"strings"

	"github.com/workflowaiorg/workflow/internal/project"
)

// Doctor runs the checks of the project at Dir, writing what a fix prints to Stdout and Stderr.
type Doctor struct {
	Dir    string
	Stdout io.Writer
	Stderr io.Writer
}

// Mark is what became of one check.
type Mark int

const (
	// Passed is a check whose command exited 0.
	Passed Mark = iota
	// Problem is a failed check with no fix: the developer fixes it by hand.
	Problem
	// Fixable is a failed check that wf scaffold fixes.
	Fixable
)

// Result is what one check came to.
type Result struct {
	Check project.Check
	Mark  Mark
	// Output is what a failed check printed.
	Output string
}

// Run runs every check in order and changes nothing, so one run names every problem.
func (d Doctor) Run(checks []project.Check) ([]Result, error) {
	var results []Result
	for _, check := range checks {
		ok, output, err := d.probe(check)
		switch {
		case err != nil:
			return nil, err
		case ok:
			results = append(results, Result{Check: check, Mark: Passed})
		case check.Fix != "":
			results = append(results, Result{Check: check, Mark: Fixable, Output: output})
		default:
			results = append(results, Result{Check: check, Mark: Problem, Output: output})
		}
	}
	return results, nil
}

// Report writes results to w, one line each, with what a failed check printed under it.
func Report(w io.Writer, results []Result) {
	for _, result := range results {
		switch result.Mark {
		case Passed:
			fmt.Fprintf(w, "✓ %s\n", result.Check.Name)
		case Problem:
			fmt.Fprintf(w, "✗ %s\n", result.Check.Name)
			writeOutput(w, result.Output)
		case Fixable:
			fmt.Fprintf(w, "→ %s: wf scaffold fixes this with %s\n", result.Check.Name, result.Check.Fix)
			writeOutput(w, result.Output)
		}
	}
}

// Count returns how many results are problems to fix by hand, and how many wf scaffold fixes.
func Count(results []Result) (problems, fixable int) {
	for _, result := range results {
		switch result.Mark {
		case Problem:
			problems++
		case Fixable:
			fixable++
		}
	}
	return problems, fixable
}

// writeOutput writes what a check printed, indented under its line.
func writeOutput(w io.Writer, output string) {
	for line := range strings.Lines(strings.TrimRight(output, "\n")) {
		fmt.Fprintf(w, "    %s", line)
	}
	if strings.TrimSpace(output) != "" {
		fmt.Fprintln(w)
	}
}

// Scaffold takes a clean clone to a repo whose every check passes. It changes nothing when a check
// is a problem to fix by hand; otherwise it runs the fix of each check that fails, checking it
// again.
func (d Doctor) Scaffold(checks []project.Check) error {
	results, err := d.Run(checks)
	if err != nil {
		return err
	}
	// A check scaffold fixes is reported as it fixes it.
	Report(d.Stdout, slices.DeleteFunc(slices.Clone(results), func(result Result) bool { return result.Mark == Fixable }))
	if problems, _ := Count(results); problems > 0 {
		return fmt.Errorf("%s, so nothing was changed; fix %s and run wf scaffold again", ToFixByHand(problems), pronoun(problems))
	}
	for _, result := range results {
		if result.Mark == Fixable {
			if err := d.fix(result.Check); err != nil {
				return err
			}
		}
	}
	fmt.Fprintln(d.Stdout, "Every check passes.")
	return nil
}

// fix runs check's fix, then the check again, which must now pass.
func (d Doctor) fix(check project.Check) error {
	fmt.Fprintf(d.Stdout, "→ %s: %s\n", check.Name, check.Fix)
	command := exec.Command("sh", "-c", check.Fix)
	command.Dir = d.Dir
	command.Stdout, command.Stderr = d.Stdout, d.Stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("fixing %s: %s: %w", check.Name, check.Fix, err)
	}
	ok, output, err := d.probe(check)
	if err != nil {
		return err
	}
	if !ok {
		writeOutput(d.Stdout, output)
		return fmt.Errorf("%s still fails after its fix ran", check.Name)
	}
	fmt.Fprintf(d.Stdout, "✓ %s\n", check.Name)
	return nil
}

// probe runs a check's command with sh in the project, and returns whether it passed and what it
// printed. A command that cannot be started at all is an error, not a failed check.
func (d Doctor) probe(check project.Check) (bool, string, error) {
	command := exec.Command("sh", "-c", check.Run)
	command.Dir = d.Dir
	var out bytes.Buffer
	command.Stdout, command.Stderr = &out, &out
	err := command.Run()
	var exit *exec.ExitError
	switch {
	case err == nil:
		return true, out.String(), nil
	case errors.As(err, &exit):
		return false, out.String(), nil
	default:
		return false, "", fmt.Errorf("running the check %q: %w", check.Name, err)
	}
}

// ToFixByHand says how many problems there are to fix by hand.
func ToFixByHand(problems int) string {
	if problems == 1 {
		return "1 problem to fix by hand"
	}
	return fmt.Sprintf("%d problems to fix by hand", problems)
}

func pronoun(n int) string {
	if n == 1 {
		return "it"
	}
	return "them"
}
