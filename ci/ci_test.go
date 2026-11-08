package ci_test

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/workflowaiorg/workflow/internal/component"
	"github.com/workflowaiorg/workflow/internal/component/backend"
	"github.com/workflowaiorg/workflow/internal/project"
)

// componentJob is a job a CI template runs for a gate step of a component.
type componentJob struct {
	name   string
	marker string // the file only a project with the component has
	step   project.GateStep
}

// markers are the components the templates run jobs for, each with the file only a project with it
// has.
var markers = []struct {
	component component.Component
	file      string
}{{backend.Component, "go.mod"}}

// componentJobs returns the job each template must have for each gate step of each component,
// every option on so that a gate step an option brings is there too, named <component><sep><step>.
func componentJobs(sep string) []componentJob {
	var jobs []componentJob
	for _, m := range markers {
		all := map[string]any{}
		for _, option := range m.component.Options {
			if option.Bool {
				all[option.Name] = true
			}
		}
		for _, step := range component.Gate([]component.Component{m.component.With(all)}) {
			name := m.component.Name + sep + strings.ReplaceAll(step.Name, " ", "-")
			jobs = append(jobs, componentJob{name, m.file, step})
		}
	}
	return jobs
}

// extra returns the names in have that look like a component's job but are none of want.
func extra[T any](have map[string]T, want []componentJob, sep string) []string {
	var names []string
	for name := range have {
		known := slices.ContainsFunc(want, func(j componentJob) bool { return j.name == name })
		for _, m := range markers {
			if strings.HasPrefix(name, m.component.Name+sep) && !known {
				names = append(names, name)
			}
		}
	}
	return names
}

func readYAML(t *testing.T, file string, into any) {
	t.Helper()
	content, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal(content, into); err != nil {
		t.Fatal(err)
	}
}

// TestGitLabJobsAreTheGateSteps checks that each component's jobs in the GitLab template run its
// gate steps, on the same paths, in a project that has it, and that the template runs nothing else
// of it.
func TestGitLabJobsAreTheGateSteps(t *testing.T) {
	var jobs map[string]struct {
		Extends string   `yaml:"extends"`
		Script  []string `yaml:"script"`
		Rules   []struct {
			Exists  []string `yaml:"exists"`
			Changes []string `yaml:"changes"`
			When    string   `yaml:"when"`
		} `yaml:"rules"`
	}
	readYAML(t, "gitlab-ci.yml", &jobs)
	// With no rules at all, GitLab would leave wf's own jobs out of merge request pipelines.
	if rules := jobs[".wf:install"].Rules; len(rules) != 1 || rules[0].When != "on_success" {
		t.Errorf(".wf:install has the rules %+v, want one: when on_success", rules)
	}

	want := componentJobs(":")
	for _, w := range want {
		j, ok := jobs[w.name]
		if !ok {
			t.Errorf("no job %s for the gate step %q", w.name, w.step.Name)
			continue
		}
		component, _, _ := strings.Cut(w.name, ":")
		if j.Extends != ".wf:"+component || !slices.Equal(j.Script, []string{w.step.Run}) {
			t.Errorf("%s extends %q and runs %q, want .wf:%s and %q", w.name, j.Extends, j.Script, component, w.step.Run)
		}
		if len(j.Rules) != 1 || !slices.Equal(j.Rules[0].Exists, []string{w.marker}) || !slices.Equal(j.Rules[0].Changes, w.step.Paths) {
			t.Errorf("%s has the rules %+v, want one: exists [%s], changes %q", w.name, j.Rules, w.marker, w.step.Paths)
		}
	}
	for _, name := range extra(jobs, want, ":") {
		t.Errorf("%s is no gate step of a component", name)
	}
}

// TestGitHubJobsAreTheGateSteps checks the same of the GitHub Actions workflow: each component's
// job runs its gate step, when the changes job says the project has the component and one of the
// step's paths changed.
func TestGitHubJobsAreTheGateSteps(t *testing.T) {
	var workflow struct {
		Jobs map[string]struct {
			Needs   string            `yaml:"needs"`
			If      string            `yaml:"if"`
			Outputs map[string]string `yaml:"outputs"`
			Steps   []struct {
				ID  string `yaml:"id"`
				Run string `yaml:"run"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	readYAML(t, "../.github/workflows/wf.yml", &workflow)
	changes := workflow.Jobs["changes"]
	var script string
	for _, step := range changes.Steps {
		if step.ID == "changes" {
			script = step.Run
		}
	}

	want := componentJobs("-")
	for _, w := range want {
		quoted := make([]string, len(w.step.Paths))
		for i, path := range w.step.Paths {
			quoted[i] = "'" + path + "'"
		}
		line := fmt.Sprintf(`echo "%s=$(changed %s %s)"`, w.name, w.marker, strings.Join(quoted, " "))
		if !strings.Contains(script, line+"\n") {
			t.Errorf("the changes job does not say whether %s runs with\n%s", w.name, line)
		}
		if got, want := changes.Outputs[w.name], "${{ steps.changes.outputs."+w.name+" }}"; got != want {
			t.Errorf("the changes job's output %s is %q, want %q", w.name, got, want)
		}
		j, ok := workflow.Jobs[w.name]
		if !ok {
			t.Errorf("no job %s for the gate step %q", w.name, w.step.Name)
			continue
		}
		if j.Needs != "changes" || j.If != "needs.changes.outputs."+w.name+" == 'true'" {
			t.Errorf("%s needs %q if %q, want it to need changes and run if its output is true", w.name, j.Needs, j.If)
		}
		if len(j.Steps) == 0 || j.Steps[len(j.Steps)-1].Run != w.step.Run {
			t.Errorf("%s does not end by running %q", w.name, w.step.Run)
		}
	}
	for _, name := range extra(workflow.Jobs, want, "-") {
		t.Errorf("%s is no gate step of a component", name)
	}
}

// TestBothTemplatesCheckBreakingChanges checks that the GitLab template and the GitHub workflow
// both have the breaking-change job, with the same oasdiff, and both let api-passthrough through.
// The job is named for the contract, not the backend, as it is no gate step: it needs the labels
// only CI knows.
func TestBothTemplatesCheckBreakingChanges(t *testing.T) {
	var gitlab map[string]struct {
		// A string for most jobs; {name, entrypoint} for an image whose entrypoint is not a shell.
		Image  any      `yaml:"image"`
		Script []string `yaml:"script"`
	}
	readYAML(t, "gitlab-ci.yml", &gitlab)
	var github struct {
		Jobs map[string]struct {
			Steps []struct {
				Run string `yaml:"run"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	readYAML(t, "../.github/workflows/wf.yml", &github)

	job, ok := gitlab["openapi:breaking"]
	if !ok {
		t.Fatal("the GitLab template has no openapi:breaking job")
	}
	fields, _ := job.Image.(map[string]any)
	image, _ := fields["name"].(string)
	if !strings.HasPrefix(image, "tufin/oasdiff:v") {
		t.Fatalf("openapi:breaking runs %q, want a pinned tufin/oasdiff", image)
	}
	var githubScript string
	for _, step := range github.Jobs["openapi-breaking"].Steps {
		githubScript += step.Run
	}
	for name, script := range map[string]string{"GitLab's openapi:breaking": strings.Join(job.Script, "\n"), "GitHub's openapi-breaking": githubScript} {
		for _, want := range []string{"api-passthrough", "*/openapi.yaml", "--fail-on ERR"} {
			if !strings.Contains(script, want) {
				t.Errorf("%s does not hold %q", name, want)
			}
		}
	}
	if !strings.Contains(githubScript, image) {
		t.Errorf("GitHub's openapi-breaking does not run %s, as GitLab's does", image)
	}
}
