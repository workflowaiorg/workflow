package base_test

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/workflowaiorg/workflow/internal/base"
	"github.com/workflowaiorg/workflow/internal/component"
	"github.com/workflowaiorg/workflow/internal/project"
)

func TestFilesRefusesAForgeItHasNoBaseFor(t *testing.T) {
	if _, err := base.Files("sourcehut", nil); err == nil || !strings.Contains(err.Error(), "sourcehut") {
		t.Errorf("Files(sourcehut) error = %v, want one naming the forge", err)
	}
}

func TestTheWorkflowSkillNamesNoForgeCommand(t *testing.T) {
	files, err := base.Files("gitlab", nil)
	if err != nil {
		t.Fatal(err)
	}
	// Everything forge-specific lives in the forge's own skill, so the workflow holds on any forge.
	for _, name := range []string{"skills/workflow/SKILL.md", "skills/domain-specs/SKILL.md", "workflow/GUIDE.md"} {
		for _, word := range []*regexp.Regexp{regexp.MustCompile(`\bglab\b`), regexp.MustCompile(`\bgh\b`), regexp.MustCompile(`\w::\w`)} {
			if found := word.FindString(string(files[name])); found != "" {
				t.Errorf("%s names %q, which belongs to a forge skill", name, found)
			}
		}
	}
	if !strings.Contains(string(files["skills/forge-gitlab/SKILL.md"]), "glab") {
		t.Error("the forge-gitlab skill never names glab")
	}
}

// installed returns a directory holding a project's .wf.yml and the base of its forge.
func installed(t *testing.T, pin string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".wf.yml"), []byte("wf: "+pin+"\nforge: gitlab\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := base.Install(dir, "gitlab", nil); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestCheckNamesEveryFileThatIsNotTheBase(t *testing.T) {
	dir := installed(t, "1.2.0")
	if differences, err := check(t, dir, "1.2.0"); err != nil || len(differences) != 0 {
		t.Fatalf("Check right after Install = (%v, %v), want no differences", differences, err)
	}

	edit := func(name, content string) {
		t.Helper()
		full := filepath.Join(dir, ".agents/base", filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	edit("skills/workflow/SKILL.md", "edited by hand\n")
	edit("skills/ours/SKILL.md", "a project's own skill, in the wrong place\n")
	if err := os.Remove(filepath.Join(dir, ".agents/base/commands/plan-check.md")); err != nil {
		t.Fatal(err)
	}
	// Windows line endings are not a difference.
	content, err := os.ReadFile(filepath.Join(dir, ".agents/base/workflow/GUIDE.md"))
	if err != nil {
		t.Fatal(err)
	}
	edit("workflow/GUIDE.md", strings.ReplaceAll(string(content), "\n", "\r\n"))

	differences, err := check(t, dir, "1.2.0")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		".agents/base/commands/plan-check.md: missing",
		".agents/base/skills/workflow/SKILL.md: differs",
		".agents/base/skills/ours/SKILL.md: not part of",
	}
	if len(differences) != len(want) {
		t.Fatalf("Check = %q, want %d differences", differences, len(want))
	}
	for _, prefix := range want {
		if !slices.ContainsFunc(differences, func(line string) bool { return strings.HasPrefix(line, prefix) }) {
			t.Errorf("Check = %q, want a line starting %q", differences, prefix)
		}
	}
}

func TestCheckNeedsThePinnedVersion(t *testing.T) {
	dir := installed(t, "1.2.0")
	differences, err := check(t, dir, "1.3.0")
	if err != nil || len(differences) != 1 || !strings.Contains(differences[0], "pins wf 1.2.0, but this is wf 1.3.0") {
		t.Errorf("Check by another version = (%q, %v), want one line naming both versions", differences, err)
	}
}

func TestSkeletonLeavesWhatIsThere(t *testing.T) {
	dir := t.TempDir()
	index := filepath.Join(dir, ".workflow/decisions/README.md")
	written, err := base.Skeleton(dir)
	if err != nil || !slices.Contains(written, ".workflow/decisions/README.md") {
		t.Fatalf("Skeleton = (%v, %v), want the decisions index written", written, err)
	}
	if err := os.WriteFile(index, []byte("the project's own index\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if written, err := base.Skeleton(dir); err != nil || len(written) != 0 {
		t.Errorf("Skeleton over an existing index = (%v, %v), want nothing written", written, err)
	}
	if content, _ := os.ReadFile(index); string(content) != "the project's own index\n" {
		t.Errorf("the project's index was overwritten: %q", content)
	}
}

// check compares the base of the project at dir, read with its own settings, as version would.
func check(t *testing.T, dir, version string) ([]string, error) {
	t.Helper()
	config, err := project.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	return base.Check(dir, config, nil, version)
}

func TestFilesAddsEachComponentsPart(t *testing.T) {
	core, err := base.Files("gitlab", nil)
	if err != nil {
		t.Fatal(err)
	}
	part := func(name string, files map[string]string) component.Component {
		fsys := fstest.MapFS{}
		for path, content := range files {
			fsys[path] = &fstest.MapFile{Data: []byte(content)}
		}
		return component.Component{Name: name, Base: fsys}
	}
	a := part("a", map[string]string{"AGENTS.md": "## A\n", "workflow/GUIDE.md": "## A's guide\n", "skills/a/SKILL.md": "a"})
	b := part("b", map[string]string{"AGENTS.md": "## B\n"})

	files, err := base.Files("gitlab", []component.Component{a, b})
	if err != nil {
		t.Fatal(err)
	}
	trimmed := func(name string) string { return strings.TrimRight(string(core[name]), "\n") }
	if got, want := string(files["AGENTS.md"]), trimmed("AGENTS.md")+"\n\n## A\n\n## B\n"; got != want {
		t.Errorf("AGENTS.md ends %q, want wf's, then A's section, then B's", got[max(0, len(got)-40):])
	}
	if got, want := string(files["workflow/GUIDE.md"]), trimmed("workflow/GUIDE.md")+"\n\n## A's guide\n"; got != want {
		t.Errorf("workflow/GUIDE.md does not end with A's section")
	}
	if string(files["skills/a/SKILL.md"]) != "a" {
		t.Error("A's own skill is not in the base")
	}

	clash := part("clash", map[string]string{"skills/a/SKILL.md": "mine"})
	if _, err := base.Files("gitlab", []component.Component{a, clash}); err == nil || !strings.Contains(err.Error(), "skills/a/SKILL.md") {
		t.Errorf("Files with two components owning one file = %v, want an error naming it", err)
	}
}
