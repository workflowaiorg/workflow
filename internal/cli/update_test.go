package cli_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/workflowaiorg/workflow/internal/cli"
	"github.com/workflowaiorg/workflow/internal/launcher"
)

func write(t *testing.T, dir, name, content string) {
	t.Helper()
	file := filepath.Join(dir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateBringsInTheBaseAndKeepsTheProjectsPart(t *testing.T) {
	dir := initialised(t)
	// The base an older wf left: one skill changed since, one gone, one new.
	write(t, dir, ".agents/base/skills/workflow/SKILL.md", read(t, dir, ".agents/base/skills/workflow/SKILL.md")+"\nAn older rule.\n")
	write(t, dir, ".agents/base/skills/gone/SKILL.md", "---\nname: gone\ndescription: \"Removed since\"\n---\n")
	if err := os.Remove(filepath.Join(dir, ".agents/base/commands/plan-check.md")); err != nil {
		t.Fatal(err)
	}
	write(t, dir, ".agents/project/skills/workflow/SKILL.md", "Our own rule.\n")
	write(t, dir, ".agents/project/skills/domain-specs/SKILL.md", "Our specs rule.\n")

	code, stdout, stderr := runSample(t, dir, "update")
	if code != 0 {
		t.Fatalf("wf update exited %d: %s", code, stderr)
	}
	if !strings.Contains(stdout, "wrote    .agents/base/ (wf 0.1.0's)") {
		t.Errorf("wf update does not say it replaced the base:\n%s", stdout)
	}
	if !strings.Contains(stderr, "check    .agents/project/skills/workflow/SKILL.md") || strings.Contains(stderr, "domain-specs") {
		t.Errorf("wf update should name the part whose base changed, and only it:\n%s", stderr)
	}
	if code, _, stderr := runSample(t, dir, "sync", "--check"); code != 0 {
		t.Errorf("wf sync --check after wf update exited %d: %s", code, stderr)
	}
	if got := read(t, dir, ".agents/project/skills/workflow/SKILL.md"); got != "Our own rule.\n" {
		t.Errorf("the project's part became %q", got)
	}
	if skill := read(t, dir, ".claude/skills/workflow/SKILL.md"); !strings.Contains(skill, "Our own rule.") || strings.Contains(skill, "An older rule.") {
		t.Errorf("the rendered skill is not the new base with the project's part:\n%s", skill)
	}
	for _, gone := range []string{".agents/base/skills/gone", ".claude/skills/gone"} {
		if _, err := os.Stat(filepath.Join(dir, gone)); err == nil {
			t.Errorf("%s is still there", gone)
		}
	}

	if _, stdout, stderr := runSample(t, dir, "update"); strings.Contains(stdout, ".agents/base") || strings.Contains(stderr, "check") {
		t.Errorf("a second wf update changed something:\n%s%s", stdout, stderr)
	}
}

func TestUpdateRefusesAPinOfAnotherVersion(t *testing.T) {
	dir := initialised(t)
	write(t, dir, ".wf.yml", strings.Replace(read(t, dir, ".wf.yml"), "wf: 0.1.0", "wf: 0.2.0", 1))
	if code, _, stderr := runSample(t, dir, "update"); code != 2 || !strings.Contains(stderr, "run the version it pins") {
		t.Errorf("wf update as another version than pinned = (%d, %q), want exit 2", code, stderr)
	}
}

func TestUpdateToMovesThePinAndHandsOver(t *testing.T) {
	const platform = "linux_amd64"
	archive := release(t, "#!/bin/sh\necho wf 0.2.0\n")
	sum := sha256.Sum256(archive)
	var checksums strings.Builder
	for _, p := range launcher.Platforms {
		fmt.Fprintf(&checksums, "%s  %s\n", hex.EncodeToString(sum[:]), launcher.Archive("0.2.0", p))
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v0.2.0/checksums.txt":
			w.Write([]byte(checksums.String()))
		case "/v0.2.0/" + launcher.Archive("0.2.0", platform):
			w.Write(archive)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	defer cli.ConfigureLauncher(func(l *launcher.Launcher) {
		l.ReleasesURL, l.Platform, l.CacheDir, l.NoLaunch = server.URL, platform, filepath.Join(t.TempDir(), "cache"), false
	})()

	update := func(version, dir string, args ...string) (int, string, []string) {
		var execed []string
		fakeExec := func(binary string, argv, env []string) error {
			execed = append([]string{binary}, argv[1:]...)
			if !slices.Contains(env, launcher.HandoverEnv+"=0.2.0") {
				t.Errorf("handed over with no %s=0.2.0", launcher.HandoverEnv)
			}
			return errors.New("exec failed")
		}
		var stdout, stderr bytes.Buffer
		code := cli.Main(version, dir, append([]string{"update"}, args...), nil, strings.NewReader(""), &stdout, &stderr, fakeExec)
		return code, stdout.String() + stderr.String(), execed
	}

	const config = "wf: 0.1.0 # the version every checkout runs\nforge: gitlab\n"
	dir := t.TempDir()
	write(t, dir, ".wf.yml", config)
	code, output, execed := update("0.1.0", dir, "--to", "0.2.0")
	if len(execed) != 2 || execed[1] != "update" {
		t.Fatalf("wf update --to 0.2.0 = (%d, %q), handed over as %v; want wf 0.2.0 run with update", code, output, execed)
	}
	if got := read(t, dir, ".wf.yml"); got != "wf: 0.2.0 # the version every checkout runs\nforge: gitlab\n" {
		t.Errorf(".wf.yml became %q", got)
	}
	if got := read(t, dir, ".wf.sum"); strings.Count(got, "wf_0.2.0_") != len(launcher.Platforms) {
		t.Errorf(".wf.sum =\n%s", got)
	}

	for _, to := range []string{"0.3.0", "latest", ""} {
		dir := t.TempDir()
		write(t, dir, ".wf.yml", config)
		if code, output, execed := update("0.1.0", dir, "--to="+to); code != 2 || execed != nil || read(t, dir, ".wf.yml") != config {
			t.Errorf("wf update --to=%s = (%d, %q), handed over as %v, .wf.yml %q; want exit 2 and nothing changed", to, code, output, execed, read(t, dir, ".wf.yml"))
		}
	}

	// The wf installed is the version asked for: it finishes the update itself.
	dir = initialised(t)
	if code, output, execed := update("0.2.0", dir, "--to", "0.2.0"); code != 0 || execed != nil {
		t.Fatalf("wf 0.2.0 update --to 0.2.0 = (%d, %q), handed over as %v; want it done here", code, output, execed)
	}
	if code, stdout, stderr := runWith(t, cli.New("0.2.0", dir, strings.NewReader(""), new(bytes.Buffer), new(bytes.Buffer)), "sync", "--check"); code != 0 {
		t.Errorf("wf sync --check after the update = (%d, %q, %q)", code, stdout, stderr)
	}

	// With launching off, nothing could finish moving to another version, so nothing starts.
	defer cli.ConfigureLauncher(func(l *launcher.Launcher) {
		l.ReleasesURL, l.Platform, l.CacheDir, l.NoLaunch = server.URL, platform, filepath.Join(t.TempDir(), "cache"), true
	})()
	dir = t.TempDir()
	write(t, dir, ".wf.yml", config)
	if code, output, execed := update("0.1.0", dir, "--to", "0.2.0"); code != 2 || execed != nil || read(t, dir, ".wf.yml") != config || !strings.Contains(output, launcher.NoLaunchEnv) {
		t.Errorf("wf update --to 0.2.0 with %s=1 = (%d, %q), handed over as %v; want exit 2 and nothing changed", launcher.NoLaunchEnv, code, output, execed)
	}
}
