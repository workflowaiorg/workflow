package cli_test

import (
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeDocker is put first on PATH in place of docker. It records every call, and keeps whether the
// stack is up in a file, so the tests need no Docker.
const fakeDocker = `#!/bin/sh
echo "$*" >> "$FAKE_DOCKER_DIR/calls"
case "$*" in
  "compose up -d") touch "$FAKE_DOCKER_DIR/up" ;;
  "compose down") rm -f "$FAKE_DOCKER_DIR/up" ;;
  "compose ps --status running --quiet") if [ -f "$FAKE_DOCKER_DIR/up" ]; then echo 1f2e3d; fi ;;
  "compose up stop-me") kill -INT $PPID; sleep 1 ;;
  "compose up "*) echo "attached to ${*#compose up }" ;;
  *) echo "fake docker cannot $*" >&2; exit 1 ;;
esac
`

// stackProject copies the sample stack project, as a clean clone would have it, to a new directory,
// with the fake docker on PATH. It returns the project and a func that returns the docker calls.
func stackProject(t *testing.T) (dir string, calls func() string) {
	t.Helper()
	dir = t.TempDir()
	if err := os.CopyFS(dir, os.DirFS("testdata/stack")); err != nil {
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
	dir, calls := stackProject(t)
	before := snapshot(t, dir)

	code, stdout, stderr := runSample(t, dir, "doctor")
	if code != 0 {
		t.Fatalf("wf doctor = (%d, %q), want 0: only wf scaffold's fixes are missing", code, stderr)
	}
	want := "✓ sample tool installed\n" +
		"→ env file: wf scaffold fixes this with cp .env.example .env\n" +
		"– app answers: skipped, wf scaffold has not fixed the checks above yet\n" +
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
	dir, _ := stackProject(t)
	appendFile(t, filepath.Join(dir, ".wf.yml"), "  - name: go installed\n    run: echo go is not on PATH; false\n")

	code, stdout, stderr := runSample(t, dir, "doctor")
	if code != 1 || stderr != "wf doctor: 1 problem to fix by hand\n" {
		t.Errorf("wf doctor = (%d, %q), want 1 naming the problem", code, stderr)
	}
	for _, want := range []string{"✗ go installed\n    go is not on PATH\n", "– app answers: skipped, fix the problems above first\n"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("wf doctor printed:\n%s\nwant it to hold %q", stdout, want)
		}
	}
}

func TestDoctorRunsTheStackChecksWhileItIsUp(t *testing.T) {
	dir, _ := stackProject(t)
	if code, _, stderr := runSample(t, dir, "scaffold"); code != 0 {
		t.Fatalf("wf scaffold = (%d, %q)", code, stderr)
	}
	code, stdout, _ := runSample(t, dir, "doctor")
	want := "✓ sample tool installed\n✓ env file\n✓ app answers\n"
	if code != 0 || stdout != want {
		t.Errorf("wf doctor on a running stack = (%d)\n%s\nwant 0 and:\n%s", code, stdout, want)
	}

	if code, _, stderr := runSample(t, dir, "down"); code != 0 {
		t.Fatalf("wf down = (%d, %q)", code, stderr)
	}
	_, stdout, _ = runSample(t, dir, "doctor")
	if !strings.Contains(stdout, "– app answers: skipped, the stack is not up; wf up starts it\n") {
		t.Errorf("wf doctor on a stopped stack printed:\n%s", stdout)
	}
}

func TestScaffoldTakesACleanCloneToARunningStack(t *testing.T) {
	dir, calls := stackProject(t)

	code, stdout, stderr := runSample(t, dir, "scaffold")
	if code != 0 {
		t.Fatalf("wf scaffold = (%d, %q)\n%s", code, stderr, stdout)
	}
	if read(t, dir, ".env") != "GREETING=hello\n" {
		t.Error("wf scaffold did not run the env file's fix")
	}
	if !strings.Contains(calls(), "compose up -d\n") {
		t.Errorf("wf scaffold did not start the stack; docker calls:\n%s", calls())
	}
	want := "✓ sample tool installed\n" +
		"→ env file: cp .env.example .env\n" +
		"✓ env file\n" +
		"→ starting the stack\n" +
		"✓ app answers\n" +
		"The stack is up, and every check passes.\n"
	if stdout != want {
		t.Errorf("wf scaffold printed:\n%s\nwant:\n%s", stdout, want)
	}
}

func TestScaffoldChangesNothingWhenAProblemNeedsAHand(t *testing.T) {
	dir, calls := stackProject(t)
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
	dir, calls := stackProject(t)
	appendFile(t, filepath.Join(dir, ".wf.yml"), "  - name: cache\n    run: test -d cache\n    fix: echo made nothing\n")

	code, _, stderr := runSample(t, dir, "scaffold")
	if code != 1 || stderr != "wf scaffold: cache still fails after its fix ran\n" {
		t.Errorf("wf scaffold = (%d, %q), want 1 naming the check", code, stderr)
	}
	if strings.Contains(calls(), "compose up") {
		t.Error("wf scaffold started the stack after a fix failed")
	}
}

func TestUpAndDown(t *testing.T) {
	dir, calls := stackProject(t)
	tests := []struct {
		args []string
		code int
		call string
	}{
		{args: []string{"up"}, call: "compose up -d"},
		{args: []string{"up", "app", "db"}, call: "compose up app db"},
		{args: []string{"down"}, call: "compose down"},
		{args: []string{"up", "app", "-d"}, code: 2},
		{args: []string{"down", "app"}, code: 2},
		{args: []string{"doctor", "app"}, code: 2},
	}
	for _, test := range tests {
		before := calls()
		code, _, stderr := runSample(t, dir, test.args...)
		if code != test.code {
			t.Errorf("wf %s = (%d, %q), want %d", strings.Join(test.args, " "), code, stderr, test.code)
		}
		if got := strings.TrimPrefix(calls(), before); test.call != "" && got != test.call+"\n" {
			t.Errorf("wf %s ran docker %q, want %q", strings.Join(test.args, " "), got, test.call)
		}
	}
}

func TestUpAndDownRunOnlyInTheProjectsRoot(t *testing.T) {
	_, calls := stackProject(t)
	for _, command := range []string{"up", "down"} {
		code, _, stderr := runSample(t, t.TempDir(), command)
		if code != 2 || !strings.Contains(stderr, "run wf in the project's root") {
			t.Errorf("wf %s outside a project = (%d, %q), want 2", command, code, stderr)
		}
	}
	if got := calls(); got != "" {
		t.Errorf("wf ran docker outside a project: %q", got)
	}
}

func TestCtrlCStopsUpQuietly(t *testing.T) {
	dir, _ := stackProject(t)
	code, _, stderr := runSample(t, dir, "up", "stop-me")
	if code != 130 || stderr != "" {
		t.Errorf("wf up interrupted = (%d, %q), want (130, \"\")", code, stderr)
	}
}

func TestScaffoldFixesTheChecksThatNeedTheStack(t *testing.T) {
	dir, _ := stackProject(t)
	appendFile(t, filepath.Join(dir, ".wf.yml"), "  - name: migrations\n    run: test -f migrated\n    fix: touch migrated\n    when: up\n")

	code, stdout, stderr := runSample(t, dir, "scaffold")
	if code != 0 {
		t.Fatalf("wf scaffold = (%d, %q)\n%s", code, stderr, stdout)
	}
	if !strings.HasSuffix(stdout, "✓ app answers\n→ migrations: touch migrated\n✓ migrations\nThe stack is up, and every check passes.\n") {
		t.Errorf("wf scaffold printed:\n%s\nwant it to fix migrations once the stack is up", stdout)
	}
}

func TestScaffoldFailsOnACheckThatNeedsTheStackAndHasNoFix(t *testing.T) {
	dir, _ := stackProject(t)
	appendFile(t, filepath.Join(dir, ".wf.yml"), "  - name: api answers\n    run: echo connection refused; false\n    when: up\n")

	code, stdout, stderr := runSample(t, dir, "scaffold")
	if code != 1 || stderr != "wf scaffold: api answers fails with the stack up, and it has no fix: to run; fix it by hand\n" {
		t.Errorf("wf scaffold = (%d, %q), want 1 naming the check", code, stderr)
	}
	if !strings.HasSuffix(stdout, "✗ api answers\n    connection refused\n") {
		t.Errorf("wf scaffold printed:\n%s\nwant the check's output last", stdout)
	}
}

func TestScaffoldWithNoChecksStartsTheStack(t *testing.T) {
	dir, calls := stackProject(t)
	config := "wf: 0.1.0\nforge: gitlab\n"
	if err := os.WriteFile(filepath.Join(dir, ".wf.yml"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	code, stdout, _ := runSample(t, dir, "scaffold")
	want := "No checks: add them under doctor: in .wf.yml.\n→ starting the stack\nThe stack is up.\n"
	if code != 0 || stdout != want || calls() != "compose up -d\n" {
		t.Errorf("wf scaffold with no checks = (%d)\n%s\nwant 0 and:\n%s", code, stdout, want)
	}
}
