package cli_test

import (
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeDocker is put first on PATH in place of docker, and records every call: wf runs none.
const fakeDocker = `#!/bin/sh
echo "$*" >> "$FAKE_DOCKER_DIR/calls"
exit 1
`

// doctorProject copies the sample project, as a clean clone would have it, to a new directory,
// with the fake docker on PATH. It returns the project and a func that returns the docker calls.
func doctorProject(t *testing.T) (dir string, calls func() string) {
	t.Helper()
	dir = t.TempDir()
	if err := os.CopyFS(dir, os.DirFS("testdata/doctor")); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "docker"), []byte(fakeDocker), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("FAKE_DOCKER_DIR", bin)
	return dir, func() string {
		content, err := os.ReadFile(filepath.Join(bin, "calls"))
		if err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		return string(content)
	}
}

// snapshot returns every file under dir with its content.
func snapshot(t *testing.T, dir string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		content, err := os.ReadFile(path)
		files[path] = string(content)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func TestDoctorRunsEveryCheckAndChangesNothing(t *testing.T) {
	dir, calls := doctorProject(t)
	before := snapshot(t, dir)

	code, stdout, stderr := runSample(t, dir, "doctor")
	if code != 0 {
		t.Fatalf("wf doctor = (%d, %q), want 0: only wf scaffold's fixes are missing", code, stderr)
	}
	want := "✓ sample tool installed\n" +
		"→ env file: wf scaffold fixes this with cp .env.example .env\n" +
		"\nwf scaffold fixes 1 of these.\n"
	if stdout != want {
		t.Errorf("wf doctor printed:\n%s\nwant:\n%s", stdout, want)
	}
	if after := snapshot(t, dir); !maps.Equal(after, before) {
		t.Errorf("wf doctor changed the project:\nbefore %v\nafter  %v", before, after)
	}
	if got := calls(); got != "" {
		t.Errorf("wf doctor ran docker: %q", got)
	}
}

func TestDoctorFailsOnAProblemToFixByHand(t *testing.T) {
	dir, _ := doctorProject(t)
	appendFile(t, filepath.Join(dir, ".wf.yml"), "  - name: go installed\n    run: echo go is not on PATH; false\n")

	code, stdout, stderr := runSample(t, dir, "doctor")
	if code != 1 || stderr != "wf doctor: 1 problem to fix by hand\n" {
		t.Errorf("wf doctor = (%d, %q), want 1 naming the problem", code, stderr)
	}
	if want := "✗ go installed\n    go is not on PATH\n"; !strings.Contains(stdout, want) {
		t.Errorf("wf doctor printed:\n%s\nwant it to hold %q", stdout, want)
	}
}

func TestScaffoldTakesACleanCloneToAReadyRepo(t *testing.T) {
	dir, calls := doctorProject(t)

	code, stdout, stderr := runSample(t, dir, "scaffold")
	if code != 0 {
		t.Fatalf("wf scaffold = (%d, %q)\n%s", code, stderr, stdout)
	}
	if read(t, dir, ".env") != "GREETING=hello\n" {
		t.Error("wf scaffold did not run the env file's fix")
	}
	if got := calls(); got != "" {
		t.Errorf("wf scaffold ran docker: %q", got)
	}
	want := "✓ sample tool installed\n" +
		"→ env file: cp .env.example .env\n" +
		"✓ env file\n" +
		"Every check passes.\n"
	if stdout != want {
		t.Errorf("wf scaffold printed:\n%s\nwant:\n%s", stdout, want)
	}
}

func TestScaffoldChangesNothingWhenAProblemNeedsAHand(t *testing.T) {
	dir, calls := doctorProject(t)
	appendFile(t, filepath.Join(dir, ".wf.yml"), "  - name: go installed\n    run: \"false\"\n")
	before := snapshot(t, dir)

	code, _, stderr := runSample(t, dir, "scaffold")
	if code != 1 || !strings.Contains(stderr, "1 problem to fix by hand, so nothing was changed") {
		t.Errorf("wf scaffold = (%d, %q), want 1 saying nothing was changed", code, stderr)
	}
	if after := snapshot(t, dir); !maps.Equal(after, before) {
		t.Errorf("wf scaffold changed the project:\nbefore %v\nafter  %v", before, after)
	}
	if got := calls(); got != "" {
		t.Errorf("wf scaffold ran docker: %q", got)
	}
}

func TestScaffoldFailsWhenAFixDoesNotFix(t *testing.T) {
	dir, calls := doctorProject(t)
	appendFile(t, filepath.Join(dir, ".wf.yml"), "  - name: cache\n    run: test -d cache\n    fix: echo made nothing\n")

	code, _, stderr := runSample(t, dir, "scaffold")
	if code != 1 || stderr != "wf scaffold: cache still fails after its fix ran\n" {
		t.Errorf("wf scaffold = (%d, %q), want 1 naming the check", code, stderr)
	}
	if got := calls(); got != "" {
		t.Errorf("wf scaffold ran docker: %q", got)
	}
}

func TestScaffoldWithNoChecksSaysWhereToAddThem(t *testing.T) {
	dir, _ := doctorProject(t)
	if err := os.WriteFile(filepath.Join(dir, ".wf.yml"), []byte("wf: 0.1.0\nforge: gitlab\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, stdout, _ := runSample(t, dir, "scaffold")
	if want := "No checks: add them under doctor: in .wf.yml.\n"; code != 0 || stdout != want {
		t.Errorf("wf scaffold with no checks = (%d, %q), want (0, %q)", code, stdout, want)
	}
}

func TestDoctorAndScaffoldTakeNoArguments(t *testing.T) {
	dir, _ := doctorProject(t)
	for _, command := range []string{"doctor", "scaffold"} {
		if code, _, _ := runSample(t, dir, command, "app"); code != 2 {
			t.Errorf("wf %s app = %d, want 2", command, code)
		}
	}
}

func TestWfDoesNotRunTheApp(t *testing.T) {
	dir, calls := doctorProject(t)
	for _, command := range []string{"up", "down"} {
		code, _, stderr := runSample(t, dir, command)
		if code != 2 || !strings.Contains(stderr, "unknown command") {
			t.Errorf("wf %s = (%d, %q), want 2: wf does not start or stop the app", command, code, stderr)
		}
	}
	if got := calls(); got != "" {
		t.Errorf("wf ran docker: %q", got)
	}
}
