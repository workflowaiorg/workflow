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

func TestLoadWithNoSettings(t *testing.T) {
	_, err := project.Load(t.TempDir())
	var mistake *project.UserError
	if !errors.As(err, &mistake) || !errors.Is(err, fs.ErrNotExist) || !strings.Contains(err.Error(), "run wf in the project's root") {
		t.Errorf("Load with no %s = %v, want a UserError that is fs.ErrNotExist", project.ConfigFile, err)
	}
}
