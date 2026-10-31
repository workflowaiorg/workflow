package project_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/workflowaiorg/workflow/internal/project"
)

func load(t *testing.T, content string) (project.Config, error) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, project.ConfigFile), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return project.Load(dir)
}

func TestLoadReadsTheSettings(t *testing.T) {
	config, err := load(t, "wf: 1.10\r\nforge: gitlab\r\nname: sample\r\nworktree:\r\n  copy: [\".env*\"]\r\n  skip: [\"*.example\"]\r\n")
	if err != nil {
		t.Fatal(err)
	}
	if config.WF != "1.10" || config.Forge != "gitlab" {
		t.Errorf("Load = wf %q, forge %q; want 1.10 and gitlab", config.WF, config.Forge)
	}
	if !slices.Equal(config.Worktree.Copy, []string{".env*"}) || !slices.Equal(config.Worktree.Skip, []string{"*.example"}) {
		t.Errorf("Load worktree = %+v, want copy .env* and skip *.example", config.Worktree)
	}
	// Every key reaches the base's templates, including the ones wf itself never reads.
	if config.Values["name"] != "sample" {
		t.Errorf("Load values = %v, want name: sample among them", config.Values)
	}
}

func TestLoadNamesEveryMistakeAtOnce(t *testing.T) {
	_, err := load(t, "forge: github\ngate:\n  - name: lint\nworktree:\n  skip: [\"[\"]\n")
	var mistake *project.UserError
	if !errors.As(err, &mistake) {
		t.Fatalf("Load = %v, want a UserError", err)
	}
	for _, want := range []string{"pins no wf version", "forge: github", "gate[0] (lint) needs paths:", `worktree.skip has a bad pattern "["`} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("Load = %q, want it to say %q", err, want)
		}
	}
}

func TestLoadReadsTheDoctorChecks(t *testing.T) {
	config, err := load(t, "wf: 0.1.0\nforge: gitlab\ndoctor:\n  - name: env\n    run: test -f .env\n    fix: cp .env.example .env\n  - name: api answers\n    run: curl -sf localhost:8000\n    when: up\n")
	if err != nil {
		t.Fatal(err)
	}
	want := []project.Check{
		{Name: "env", Run: "test -f .env", Fix: "cp .env.example .env"},
		{Name: "api answers", Run: "curl -sf localhost:8000", WhenUp: true},
	}
	if !slices.Equal(config.Doctor, want) {
		t.Errorf("Load doctor = %+v, want %+v", config.Doctor, want)
	}
}

func TestLoadNamesEveryMistakeInDoctor(t *testing.T) {
	_, err := load(t, "wf: 0.1.0\nforge: gitlab\ndoctor:\n  - name: env\n    fix: [cp]\n    when: down\n    then: x\n  - name: off\n    run: false\n  - run: x\n")
	var mistake *project.UserError
	if !errors.As(err, &mistake) {
		t.Fatalf("Load = %v, want a UserError", err)
	}
	for _, want := range []string{"doctor[0] (env) needs run:", "has a fix: that is not a command", "has a when: other than up", "has then:", "doctor[1] (off) has a run: that YAML does not read as text", "doctor[2] needs name:"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("Load = %q, want it to say %q", err, want)
		}
	}
	if _, err := load(t, "wf: 0.1.0\nforge: gitlab\ndoctor: true\n"); err == nil || !strings.Contains(err.Error(), "doctor: is a list of checks") {
		t.Errorf("Load with doctor: true = %v, want it to say doctor: is a list", err)
	}
}

func TestLoadWithNoSettings(t *testing.T) {
	_, err := project.Load(t.TempDir())
	var mistake *project.UserError
	if !errors.As(err, &mistake) || !errors.Is(err, fs.ErrNotExist) || !strings.Contains(err.Error(), "run wf in the project's root") {
		t.Errorf("Load with no %s = %v, want a UserError that is fs.ErrNotExist", project.ConfigFile, err)
	}
}

func TestWithComponentKeepsWhatIsThere(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, project.ConfigFile)
	original := "# settings\nwf: 1.2.0\nforge: gitlab # the forge\ncomponents:\n"
	if err := os.WriteFile(file, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	addComponent(t, dir, "backend", map[string]any{"grpc": true})
	addComponent(t, dir, "frontend", map[string]any{})
	content, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"# settings\n", "forge: gitlab # the forge\n", "  backend:\n    grpc: true\n", "  frontend: {}\n"} {
		if !strings.Contains(string(content), want) {
			t.Errorf(".wf.yml after WithComponent lacks %q:\n%s", want, content)
		}
	}
	config, err := project.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if config.Components["backend"]["grpc"] != true || config.Components["frontend"] == nil {
		t.Errorf("Load after WithComponent = %v, want both components", config.Components)
	}
}

func TestWithComponentRefusesComponentsItCannotAddTo(t *testing.T) {
	for name, original := range map[string]string{
		"alias":     "wf: 1.2.0\nforge: gitlab\nsaved: &installed\n  first: {}\ncomponents: *installed\n",
		"merge key": "wf: 1.2.0\nforge: gitlab\nsaved: &settings\n  components:\n    first: {}\n<<: *settings\n",
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, project.ConfigFile), []byte(original), 0o644); err != nil {
				t.Fatal(err)
			}
			if config, err := project.Load(dir); err != nil || config.Components["first"] == nil {
				t.Fatalf("Load = (%v, %v), want first installed", config.Components, err)
			}
			_, err := project.WithComponent(dir, "second", map[string]any{})
			if userErr := (*project.UserError)(nil); !errors.As(err, &userErr) || !strings.Contains(err.Error(), "written out in full") {
				t.Errorf("WithComponent = %v, want a UserError asking for components: in full", err)
			}
		})
	}
}

// addComponent records name in dir's .wf.yml, as wf add does.
func addComponent(t *testing.T, dir, name string, answers map[string]any) {
	t.Helper()
	content, err := project.WithComponent(dir, name, answers)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, project.ConfigFile), content, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadRefusesComponentsThatAreNotAMapping(t *testing.T) {
	for _, components := range []string{"components: [backend]\n", "components:\n  backend: yes\n"} {
		if _, err := load(t, "wf: 1.2.0\nforge: gitlab\n"+components); err == nil || !strings.Contains(err.Error(), "components") {
			t.Errorf("Load with %q = %v, want an error naming components", components, err)
		}
	}
}
