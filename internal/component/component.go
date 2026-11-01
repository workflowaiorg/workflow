// Package component is what a project can add to wf, one capability at a time: each component
// brings its own part of the base, its own gate steps and doctor checks, and records itself in
// .wf.yml when wf add installs it. Each component wf carries is a package below this one.
package component

import (
	"fmt"
	"io/fs"
	"maps"
	"slices"
	"strings"

	"github.com/workflowaiorg/workflow/internal/project"
)

// Part is what a component, or one of its options, brings to a project.
type Part struct {
	// Base holds its part of .agents/base, in the same layout. Its AGENTS.md and workflow/GUIDE.md
	// are added to wf's as a section of their own; any other file is its alone.
	Base fs.FS
	// Skeleton holds files the project is given once, the first time the component is added.
	Skeleton fs.FS
	// Gate holds its steps for /pre-mr-check.
	Gate []project.GateStep
	// Doctor holds its checks for wf doctor, which run before the project's own.
	Doctor []project.Check
}

// Component is one capability wf add installs.
type Component struct {
	// Name is how the project names it: wf add <name>.
	Name    string
	Summary string
	// Options are the questions wf add asks, each also a flag of wf add.
	Options []Option
	// Part is what the component brings whatever the answers.
	Part
	// chosen holds the parts of the options answered yes, set by With.
	chosen []Part
	// Check, when set, checks the answers against the project before wf add writes anything, so a
	// wrong answer is not written into files the project then owns.
	Check func(dir string, answers map[string]any) error
	// Edits says what the developer should change by hand in files the project owns. wf never
	// edits them itself.
	Edits func(answers map[string]any) []string
}

// Option is one question wf add asks, and the flag that answers it instead.
type Option struct {
	// Name is the flag, --<name>, and the key its answer is recorded under.
	Name     string
	Question string
	// Bool makes it a yes-or-no question, answered by the flag alone.
	Bool bool
	// Default is the answer when none is given: "true" or "false" for a Bool option.
	Default string
	// Choices, when set, are the only answers accepted.
	Choices []string
	// Part, for a Bool option, is what the component brings only when the answer is yes: what is
	// not chosen does not exist in the project.
	Part
}

// With returns c as added with answers: its own part, and the part of each option answered yes.
func (c Component) With(answers map[string]any) Component {
	c.chosen = nil
	for _, option := range c.Options {
		if yes, _ := answers[option.Name].(bool); option.Bool && yes {
			c.chosen = append(c.chosen, option.Part)
		}
	}
	return c
}

// Parts returns what c brings: its own part, then the part of each option With chose.
func (c Component) Parts() []Part {
	return append([]Part{c.Part}, c.chosen...)
}

// Installed returns the components config records, sorted by name, from known. A name wf does not
// know is the developer's mistake.
func Installed(config project.Config, known []Component) ([]Component, error) {
	var installed []Component
	for _, name := range slices.Sorted(maps.Keys(config.Components)) {
		index := slices.IndexFunc(known, func(c Component) bool { return c.Name == name })
		if index < 0 {
			return nil, project.UserErrorf("%s: components.%s is not a component wf knows; %s", project.ConfigFile, name, Known(known))
		}
		c := known[index]
		if problems := c.answerProblems(config.Components[name]); len(problems) > 0 {
			return nil, project.UserErrorf("%s: %s", project.ConfigFile, strings.Join(problems, "; "))
		}
		installed = append(installed, c.With(config.Components[name]))
	}
	return installed, nil
}

// answerProblems says what is wrong with answers, recorded in .wf.yml for c, where they may have
// been edited by hand: each of c's options needs an answer it accepts, and nothing else is an answer.
func (c Component) answerProblems(answers map[string]any) []string {
	var problems []string
	for _, option := range c.Options {
		where := fmt.Sprintf("components.%s.%s", c.Name, option.Name)
		answer, ok := answers[option.Name]
		switch {
		case !ok:
			problems = append(problems, where+" has no answer; wf add asks: "+option.Question)
		case option.Bool:
			if _, ok := answer.(bool); !ok {
				problems = append(problems, where+" is true or false")
			}
		default:
			text, ok := answer.(string)
			if _, err := option.Answer(text); !ok || err != nil {
				problems = append(problems, where+" is "+option.takes())
			}
		}
	}
	for _, key := range slices.Sorted(maps.Keys(answers)) {
		if !slices.ContainsFunc(c.Options, func(option Option) bool { return option.Name == key }) {
			problems = append(problems, fmt.Sprintf("components.%s.%s is not an option of %s", c.Name, key, c))
		}
	}
	return problems
}

// takes says what an option's answer may be, for a message.
func (option Option) takes() string {
	if len(option.Choices) > 0 {
		return "one of " + strings.Join(option.Choices, ", ")
	}
	return "a text answer"
}

// Known says which components wf carries, for a message.
func Known(known []Component) string {
	if len(known) == 0 {
		return "wf carries no components yet"
	}
	var names []string
	for _, c := range known {
		names = append(names, c.Name)
	}
	return "it knows " + strings.Join(names, ", ")
}

// Answer checks answer against option and returns it as recorded: a bool for a Bool option, the
// text otherwise.
func (option Option) Answer(answer string) (any, error) {
	if option.Bool {
		switch strings.ToLower(answer) {
		case "true", "yes", "y":
			return true, nil
		case "false", "no", "n":
			return false, nil
		}
		return nil, project.UserErrorf("--%s is yes or no, not %q", option.Name, answer)
	}
	if len(option.Choices) > 0 && !slices.Contains(option.Choices, answer) {
		return nil, project.UserErrorf("--%s is %s, not %q", option.Name, option.takes(), answer)
	}
	if answer == "" {
		return nil, project.UserErrorf("--%s needs an answer", option.Name)
	}
	return answer, nil
}

// Gate returns the gate steps of components, in order.
func Gate(components []Component) []project.GateStep {
	var gate []project.GateStep
	for _, c := range components {
		for _, part := range c.Parts() {
			gate = append(gate, part.Gate...)
		}
	}
	return gate
}

// Doctor returns the doctor checks of components, in order.
func Doctor(components []Component) []project.Check {
	var checks []project.Check
	for _, c := range components {
		for _, part := range c.Parts() {
			checks = append(checks, part.Doctor...)
		}
	}
	return checks
}

// String names the component in messages.
func (c Component) String() string { return fmt.Sprintf("the %s component", c.Name) }
