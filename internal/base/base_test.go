package base_test

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/workflowaiorg/workflow/internal/base"
)

func TestFilesRefusesAForgeItHasNoBaseFor(t *testing.T) {
	if _, err := base.Files("sourcehut"); err == nil || !strings.Contains(err.Error(), "sourcehut") {
		t.Errorf("Files(sourcehut) error = %v, want one naming the forge", err)
	}
}

func TestTheWorkflowSkillNamesNoForgeCommand(t *testing.T) {
	files, err := base.Files("gitlab")
	if err != nil {
		t.Fatal(err)
	}
	// Everything forge-specific lives in the forge's own skill, so the workflow holds on any forge.
	for _, name := range []string{"skills/workflow/SKILL.md", "skills/domain-specs/SKILL.md", "GUIDE.md"} {
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

// project returns a directory holding a project's .wf.yml and the base of its forge.
func project(t *testing.T, pin string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".wf.yml"), []byte("wf: "+pin+"\nforge: gitlab\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := base.Install(dir, "gitlab"); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestCheckNamesEveryFileThatIsNotTheBase(t *testing.T) {
	dir := project(t, "1.2.0")
	if differences, err := base.Check(dir, "1.2.0"); err != nil || len(differences) != 0 {
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
	content, err := os.ReadFile(filepath.Join(dir, ".agents/base/GUIDE.md"))
	if err != nil {
		t.Fatal(err)
	}
	edit("GUIDE.md", strings.ReplaceAll(string(content), "\n", "\r\n"))

	differences, err := base.Check(dir, "1.2.0")
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
	dir := project(t, "1.2.0")
	differences, err := base.Check(dir, "1.3.0")
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
