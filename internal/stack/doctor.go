package stack

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/workflowaiorg/workflow/internal/project"
)

// Mark is what became of one check.
type Mark int

const (
	// Passed is a check whose command exited 0.
	Passed Mark = iota
	// Problem is a failed check with no fix: the developer fixes it by hand.
	Problem
	// Fixable is a failed check that wf scaffold fixes.
	Fixable
	// Skipped is a check not run, because it needs the stack up or something above it failed.
	Skipped
)

// Result is what one check came to.
type Result struct {
	Check project.Check
	Mark  Mark
	// Output is what a failed check printed; Reason is why a check was skipped.
	Output string
	Reason string
}

// Doctor runs checks in order and changes nothing. Every check that does not need the stack runs,
// so one run names every problem. A check that needs it runs only while the stack is up and every
// check above it passed: otherwise its failure would only repeat theirs.
func (s Stack) Doctor(checks []project.Check) ([]Result, error) {
	var results []Result
	for _, check := range checks {
		if check.WhenUp {
			continue
		}
		result, err := s.run(check)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}

	up, upKnown := false, false
	for _, check := range checks {
		if !check.WhenUp {
			continue
		}
		if reason := blockedBy(results); reason != "" {
			results = append(results, Result{Check: check, Mark: Skipped, Reason: reason})
			continue
		}
		if !upKnown {
			up, upKnown = s.isUp(), true
		}
		if !up {
			results = append(results, Result{Check: check, Mark: Skipped, Reason: "the stack is not up; wf up starts it"})
			continue
		}
		result, err := s.run(check)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, nil
}

// blockedBy says why a check that needs the stack cannot run after results, or nothing when it can.
func blockedBy(results []Result) string {
	reason := ""
	for _, result := range results {
		switch result.Mark {
		case Problem:
			return "fix the problems above first"
		case Fixable:
			reason = "wf scaffold has not fixed the checks above yet"
		case Skipped:
			if reason == "" {
				reason = "a check above was skipped"
			}
		}
	}
	return reason
}

func (s Stack) run(check project.Check) (Result, error) {
	ok, output, err := s.probe(check)
	switch {
	case err != nil:
		return Result{}, err
	case ok:
		return Result{Check: check, Mark: Passed}, nil
	case check.Fix != "":
		return Result{Check: check, Mark: Fixable, Output: output}, nil
	default:
		return Result{Check: check, Mark: Problem, Output: output}, nil
	}
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
		case Skipped:
			fmt.Fprintf(w, "– %s: skipped, %s\n", result.Check.Name, result.Reason)
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

// Scaffold takes the project from a clean clone to a running stack. It runs the checks that do not
// need the stack first, and changes nothing when one of them is a problem to fix by hand. Then it
// fixes what fails, checking each again, starts the stack, and runs the checks that need it, fixing
// those that fail too.
func (s Stack) Scaffold(checks []project.Check) error {
	var before, after []project.Check
	for _, check := range checks {
		if check.WhenUp {
			after = append(after, check)
		} else {
			before = append(before, check)
		}
	}

	preflight, err := s.Doctor(before)
	if err != nil {
		return err
	}
	// A check scaffold fixes is reported as it fixes it.
	Report(s.Stdout, slices.DeleteFunc(slices.Clone(preflight), func(result Result) bool { return result.Mark == Fixable }))
	if problems, _ := Count(preflight); problems > 0 {
		return fmt.Errorf("%s, so nothing was changed; fix %s and run wf scaffold again", ToFixByHand(problems), pronoun(problems))
	}
	for _, result := range preflight {
		if result.Mark == Fixable {
			if err := s.fix(result.Check); err != nil {
				return err
			}
		}
	}

	fmt.Fprintln(s.Stdout, "→ starting the stack")
	if err := s.Up(); err != nil {
		return err
	}

	for _, check := range after {
		result, err := s.run(check)
		switch {
		case err != nil:
			return err
		case result.Mark == Problem:
			Report(s.Stdout, []Result{result})
			return fmt.Errorf("%s fails with the stack up, and it has no fix: to run; fix it by hand", check.Name)
		case result.Mark == Fixable:
			if err := s.fix(check); err != nil {
				return err
			}
		default:
			Report(s.Stdout, []Result{result})
		}
	}
	if len(checks) == 0 {
		fmt.Fprintln(s.Stdout, "The stack is up.")
		return nil
	}
	fmt.Fprintln(s.Stdout, "The stack is up, and every check passes.")
	return nil
}

// fix runs check's fix, then the check again, which must now pass.
func (s Stack) fix(check project.Check) error {
	fmt.Fprintf(s.Stdout, "→ %s: %s\n", check.Name, check.Fix)
	if err := s.shell(check.Fix); err != nil {
		return fmt.Errorf("fixing %s: %s: %w", check.Name, check.Fix, err)
	}
	ok, output, err := s.probe(check)
	if err != nil {
		return err
	}
	if !ok {
		writeOutput(s.Stdout, output)
		return fmt.Errorf("%s still fails after its fix ran", check.Name)
	}
	fmt.Fprintf(s.Stdout, "✓ %s\n", check.Name)
	return nil
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
