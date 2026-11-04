package ci_test

import (
	"os"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/workflowaiorg/workflow/internal/component"
	"github.com/workflowaiorg/workflow/internal/component/backend"
)

type job struct {
	Extends string   `yaml:"extends"`
	Script  []string `yaml:"script"`
	Rules   []struct {
		Exists  []string `yaml:"exists"`
		Changes []string `yaml:"changes"`
		When    string   `yaml:"when"`
	} `yaml:"rules"`
}

// TestGitLabJobsAreTheGateSteps checks that each component's jobs in the GitLab template run its
// gate steps, on the same paths, in a project that has it, and that the template runs nothing else
// of it.
func TestGitLabJobsAreTheGateSteps(t *testing.T) {
	content, err := os.ReadFile("gitlab-ci.yml")
	if err != nil {
		t.Fatal(err)
	}
	var jobs map[string]job
	if err := yaml.Unmarshal(content, &jobs); err != nil {
		t.Fatal(err)
	}
	// With no rules at all, GitLab would leave wf's own jobs out of merge request pipelines.
	if rules := jobs[".wf:install"].Rules; len(rules) != 1 || rules[0].When != "on_success" {
		t.Errorf(".wf:install has the rules %+v, want one: when on_success", rules)
	}

	// The file only a project with the component has.
	markers := []struct {
		component component.Component
		file      string
	}{{backend.Component, "go.mod"}}
	for _, m := range markers {
		// Every option on, so a gate step an option brings is checked too.
		all := map[string]any{}
		for _, option := range m.component.Options {
			if option.Bool {
				all[option.Name] = true
			}
		}
		var want []string
		for _, step := range component.Gate([]component.Component{m.component.With(all)}) {
			name := m.component.Name + ":" + strings.ReplaceAll(step.Name, " ", "-")
			want = append(want, name)
			j, ok := jobs[name]
			if !ok {
				t.Errorf("no job %s for the gate step %q", name, step.Name)
				continue
			}
			if j.Extends != ".wf:"+m.component.Name || !slices.Equal(j.Script, []string{step.Run}) {
				t.Errorf("%s extends %q and runs %q, want .wf:%s and %q", name, j.Extends, j.Script, m.component.Name, step.Run)
			}
			if len(j.Rules) != 1 || !slices.Equal(j.Rules[0].Exists, []string{m.file}) || !slices.Equal(j.Rules[0].Changes, step.Paths) {
				t.Errorf("%s has the rules %+v, want one: exists [%s], changes %q", name, j.Rules, m.file, step.Paths)
			}
		}
		for name := range jobs {
			if strings.HasPrefix(name, m.component.Name+":") && !slices.Contains(want, name) {
				t.Errorf("%s is no gate step of %s", name, m.component)
			}
		}
	}
}
