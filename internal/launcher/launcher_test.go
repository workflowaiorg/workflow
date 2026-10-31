package launcher_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/workflowaiorg/workflow/internal/launcher"
	"github.com/workflowaiorg/workflow/internal/project"
)

// platform is the one the tests run releases for: each release's wf is a shell script, which
// runs on any machine the tests do.
const platform = "linux_amd64"

// releases serves wf releases the way GitHub does, under /v<version>/.
type releases struct {
	server   *httptest.Server
	files    map[string][]byte
	requests atomic.Int32
}

// newReleases serves a release of each version, whose wf prints "wf <version>".
func newReleases(t *testing.T, versions ...string) *releases {
	t.Helper()
	r := &releases{files: map[string][]byte{}}
	for _, version := range versions {
		r.add(t, version, archive(t, tarEntry{name: "wf", body: "#!/bin/sh\necho wf " + version + "\n", mode: 0o755}, tarEntry{name: "LICENSE", body: "MIT\n", mode: 0o644}))
	}
	r.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		r.requests.Add(1)
		data, ok := r.files[req.URL.Path]
		if !ok {
			http.NotFound(w, req)
			return
		}
		w.Write(data)
	}))
	t.Cleanup(r.server.Close)
	return r
}

// add serves version with wfArchive as every platform's archive, and a checksums.txt listing them.
func (r *releases) add(t *testing.T, version string, wfArchive []byte) {
	t.Helper()
	var checksums strings.Builder
	for _, p := range launcher.Platforms {
		name := launcher.Archive(version, p)
		r.files["/v"+version+"/"+name] = wfArchive
		fmt.Fprintf(&checksums, "%s  %s\n", hexSum(wfArchive), name)
	}
	// A release's other files are listed too, and left out of .wf.sum.
	fmt.Fprintf(&checksums, "%s  wf_%s_source.zip\n", hexSum([]byte("source")), version)
	r.files["/v"+version+"/checksums.txt"] = []byte(checksums.String())
}

type tarEntry struct {
	name, body, link string
	mode             int64
}

func archive(t *testing.T, entries ...tarEntry) []byte {
	t.Helper()
	var out bytes.Buffer
	compressed := gzip.NewWriter(&out)
	files := tar.NewWriter(compressed)
	for _, entry := range entries {
		header := &tar.Header{Name: entry.name, Mode: entry.mode, Size: int64(len(entry.body)), Typeflag: tar.TypeReg}
		if entry.link != "" {
			header = &tar.Header{Name: entry.name, Mode: entry.mode, Typeflag: tar.TypeSymlink, Linkname: entry.link}
		}
		if err := files.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if entry.link == "" {
			if _, err := files.Write([]byte(entry.body)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := files.Close(); err != nil {
		t.Fatal(err)
	}
	if err := compressed.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func hexSum(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// checkout returns a project whose .wf.yml is config.
func checkout(t *testing.T, config string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, project.ConfigFile), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// newLauncher returns the launcher of wf 0.1.0 in dir, downloading from r into cache.
func newLauncher(dir, cache string, r *releases) *launcher.Launcher {
	l := launcher.New("0.1.0", dir)
	l.CacheDir = cache
	l.ReleasesURL = r.server.URL
	l.Platform = platform
	l.NoLaunch = false
	return l
}

// runTarget runs target's binary and returns what it prints.
func runTarget(t *testing.T, target launcher.Target) string {
	t.Helper()
	out, err := exec.Command(target.Binary).Output()
	if err != nil {
		t.Fatalf("running %s: %v", target.Binary, err)
	}
	return string(out)
}

func lockedCheckout(t *testing.T, pin, cache string, r *releases) *launcher.Launcher {
	t.Helper()
	l := newLauncher(checkout(t, "wf: "+pin+"\nforge: gitlab\n"), cache, r)
	if _, err := l.Lock(); err != nil {
		t.Fatalf("wf lock for %s: %v", pin, err)
	}
	return l
}

func TestTwoCheckoutsEachRunTheVersionTheyPin(t *testing.T) {
	r := newReleases(t, "0.2.0", "0.3.0")
	cache := t.TempDir()
	for _, pin := range []string{"0.2.0", "0.3.0"} {
		target, err := lockedCheckout(t, pin, cache, r).Resolve([]string{"sync"})
		if err != nil {
			t.Fatalf("checkout pinning %s: %v", pin, err)
		}
		if target.Version != pin || runTarget(t, target) != "wf "+pin+"\n" {
			t.Errorf("checkout pinning %s ran %s (%s)", pin, target.Binary, target.Version)
		}
	}
}

func TestACachedVersionRunsWithNoNetwork(t *testing.T) {
	r := newReleases(t, "0.2.0")
	cache := t.TempDir()
	first := lockedCheckout(t, "0.2.0", cache, r)
	second := lockedCheckout(t, "0.2.0", cache, r)
	if _, err := first.Resolve(nil); err != nil {
		t.Fatal(err)
	}
	r.server.Close()

	target, err := second.Resolve([]string{"sync"})
	if err != nil {
		t.Fatalf("with the network gone, a cached version = %v, want it run", err)
	}
	if got := runTarget(t, target); got != "wf 0.2.0\n" {
		t.Errorf("the cached version printed %q", got)
	}
}

func TestADownloadThatFailsItsChecksumIsRefused(t *testing.T) {
	r := newReleases(t, "0.2.0")
	cache := t.TempDir()
	l := lockedCheckout(t, "0.2.0", cache, r)
	// The release changed after the project recorded it.
	r.add(t, "0.2.0", archive(t, tarEntry{name: "wf", body: "#!/bin/sh\necho tampered\n", mode: 0o755}))

	target, err := l.Resolve([]string{"sync"})
	if err == nil || !strings.Contains(err.Error(), "refusing wf 0.2.0") || target.Binary != "" {
		t.Fatalf("Resolve = (%+v, %v), want the download refused", target, err)
	}
	assertNothingCached(t, cache)
}

func TestAnArchiveWithoutOnePlainWfIsRefused(t *testing.T) {
	tests := map[string][]tarEntry{
		"a link":       {{name: "wf", link: "/bin/sh", mode: 0o755}},
		"two wf files": {{name: "wf", body: "one", mode: 0o755}, {name: "./wf", body: "two", mode: 0o755}},
		"no wf":        {{name: "LICENSE", body: "MIT", mode: 0o644}},
	}
	for name, entries := range tests {
		t.Run(name, func(t *testing.T) {
			r := newReleases(t)
			r.add(t, "0.2.0", archive(t, entries...))
			cache := t.TempDir()
			if _, err := lockedCheckout(t, "0.2.0", cache, r).Resolve(nil); err == nil || !strings.Contains(err.Error(), "refusing wf 0.2.0") {
				t.Fatalf("Resolve = %v, want the archive refused", err)
			}
			assertNothingCached(t, cache)
		})
	}
}

func assertNothingCached(t *testing.T, cache string) {
	t.Helper()
	filepath.WalkDir(cache, func(path string, entry fs.DirEntry, err error) error {
		if err == nil && !entry.IsDir() {
			t.Errorf("%s was cached", path)
		}
		return err
	})
}

func TestTheRunningWfRunsItself(t *testing.T) {
	tests := []struct {
		name     string
		config   string // "" for no .wf.yml
		args     []string
		noLaunch bool
	}{
		{name: "no .wf.yml", args: []string{"init"}},
		{name: "no pin", config: "forge: gitlab\n", args: []string{"sync"}},
		{name: "its own version pinned", config: "wf: 0.1.0\n", args: []string{"sync"}},
		{name: "WF_NO_LAUNCH", config: "wf: 0.2.0\n", args: []string{"sync"}, noLaunch: true},
		{name: "wf lock", config: "wf: 0.2.0\n", args: []string{"lock"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r := newReleases(t, "0.2.0")
			dir := t.TempDir()
			if test.config != "" {
				dir = checkout(t, test.config)
			}
			l := newLauncher(dir, t.TempDir(), r)
			l.NoLaunch = test.noLaunch
			target, err := l.Resolve(test.args)
			if err != nil || target.Binary != "" || r.requests.Load() != 0 {
				t.Errorf("Resolve = (%+v, %v) after %d requests, want this wf run with no download", target, err, r.requests.Load())
			}
		})
	}
}

func TestResolveRefuses(t *testing.T) {
	tests := []struct {
		name, config, sum, want string
	}{
		{name: "a malformed .wf.yml", config: "wf: [0.2.0\n", want: "reading .wf.yml"},
		{name: "a pin that is no version", config: "wf: ../0.2.0\n", want: "is not a release version"},
		{name: "no .wf.sum", config: "wf: 0.2.0\n", want: "run wf lock and commit .wf.sum"},
		{name: "a .wf.sum of another version", config: "wf: 0.2.0\n", sum: "wf_0.1.9_linux_amd64.tar.gz sha256:" + strings.Repeat("a", 64) + "\n", want: "run wf lock"},
		{name: "a malformed .wf.sum", config: "wf: 0.2.0\n", sum: "wf_0.2.0_linux_amd64.tar.gz deadbeef\n", want: ".wf.sum: line 1"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r := newReleases(t, "0.2.0")
			dir := checkout(t, test.config)
			if test.sum != "" {
				if err := os.WriteFile(filepath.Join(dir, project.SumFile), []byte(test.sum), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			_, err := newLauncher(dir, t.TempDir(), r).Resolve([]string{"sync"})
			var mistake *project.UserError
			if err == nil || !strings.Contains(err.Error(), test.want) || !errors.As(err, &mistake) {
				t.Errorf("Resolve = %v, want the developer's mistake naming %q", err, test.want)
			}
		})
	}
}

func TestLockRecordsEveryPlatformOfThePin(t *testing.T) {
	r := newReleases(t, "0.2.0", "0.3.0")
	l := newLauncher(checkout(t, "wf: 0.2.0\n"), t.TempDir(), r)
	result, err := l.Lock()
	if err != nil || !result.Changed || result.Version != "0.2.0" {
		t.Fatalf("Lock = (%+v, %v), want .wf.sum written for 0.2.0", result, err)
	}
	sum := hexSum(r.files["/v0.2.0/"+launcher.Archive("0.2.0", platform)])
	var want strings.Builder
	for _, p := range launcher.Platforms {
		fmt.Fprintf(&want, "%s sha256:%s\n", launcher.Archive("0.2.0", p), sum)
	}
	if got := readFile(t, l.Dir, project.SumFile); got != want.String() {
		t.Errorf(".wf.sum =\n%s\nwant\n%s", got, want.String())
	}

	// Recorded already: nothing to do, and no network.
	r.server.Close()
	if result, err := l.Lock(); err != nil || result.Changed {
		t.Errorf("a second Lock = (%+v, %v), want nothing changed with no network", result, err)
	}
}

func TestLockReplacesTheOldPinsFingerprints(t *testing.T) {
	r := newReleases(t, "0.2.0", "0.3.0")
	l := lockedCheckout(t, "0.2.0", t.TempDir(), r)
	if err := os.WriteFile(filepath.Join(l.Dir, project.ConfigFile), []byte("wf: 0.3.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if result, err := l.Lock(); err != nil || !result.Changed {
		t.Fatalf("Lock after a new pin = (%+v, %v), want .wf.sum rewritten", result, err)
	}
	if got := readFile(t, l.Dir, project.SumFile); strings.Contains(got, "0.2.0") || strings.Count(got, "wf_0.3.0_") != len(launcher.Platforms) {
		t.Errorf(".wf.sum after a new pin =\n%s\nwant 0.3.0's lines alone", got)
	}
}

func TestLockNeverReplacesAFingerprintRecorded(t *testing.T) {
	r := newReleases(t, "0.2.0")
	l := lockedCheckout(t, "0.2.0", t.TempDir(), r)
	// One line lost, so wf lock goes to the release, which has changed since it was recorded.
	before := readFile(t, l.Dir, project.SumFile)
	lines := strings.SplitAfter(before, "\n")
	if err := os.WriteFile(filepath.Join(l.Dir, project.SumFile), []byte(strings.Join(lines[1:], "")), 0o644); err != nil {
		t.Fatal(err)
	}
	r.add(t, "0.2.0", archive(t, tarEntry{name: "wf", body: "tampered", mode: 0o755}))

	if _, err := l.Lock(); err == nil || !strings.Contains(err.Error(), "changed after it was recorded") {
		t.Fatalf("Lock = %v, want it refused", err)
	}
	if got := readFile(t, l.Dir, project.SumFile); got != strings.Join(lines[1:], "") {
		t.Errorf(".wf.sum was changed by a refused Lock:\n%s", got)
	}
}

func TestLockRefuses(t *testing.T) {
	tests := []struct {
		name, config, checksums, want string
	}{
		{name: "no pin", config: "forge: gitlab\n", want: "pins no wf version"},
		{name: "a version with no release", config: "wf: 9.9.9\n", want: "which has no release"},
		{name: "a platform missing", config: "wf: 0.2.0\n", checksums: "drop linux_arm64", want: "does not list wf_0.2.0_linux_arm64.tar.gz"},
		{name: "a platform twice", config: "wf: 0.2.0\n", checksums: "twice", want: "lists wf_0.2.0_darwin_amd64.tar.gz twice"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r := newReleases(t, "0.2.0")
			checksums := string(r.files["/v0.2.0/checksums.txt"])
			switch test.checksums {
			case "drop linux_arm64":
				var kept []string
				for line := range strings.Lines(checksums) {
					if !strings.Contains(line, "linux_arm64") {
						kept = append(kept, line)
					}
				}
				checksums = strings.Join(kept, "")
			case "twice":
				checksums += strings.SplitAfter(checksums, "\n")[0]
			}
			r.files["/v0.2.0/checksums.txt"] = []byte(checksums)

			l := newLauncher(checkout(t, test.config), t.TempDir(), r)
			if _, err := l.Lock(); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Lock = %v, want an error naming %q", err, test.want)
			}
			if _, err := os.Stat(filepath.Join(l.Dir, project.SumFile)); !errors.Is(err, fs.ErrNotExist) {
				t.Errorf("a refused Lock wrote .wf.sum")
			}
		})
	}
}

func TestHandover(t *testing.T) {
	t.Setenv(launcher.HandoverEnv, "0.2.0")
	if handedOver, err := launcher.Handover("0.2.0"); !handedOver || err != nil {
		t.Fatalf("Handover = (%v, %v), want (true, nil)", handedOver, err)
	}
	if _, ok := os.LookupEnv(launcher.HandoverEnv); ok {
		t.Errorf("%s is still set, so a wf this one starts would take it for its own", launcher.HandoverEnv)
	}

	t.Setenv(launcher.HandoverEnv, "0.2.0")
	var mistake *project.UserError
	if _, err := launcher.Handover("0.3.0"); !errors.As(err, &mistake) {
		t.Errorf("a wf started as 0.2.0 that is 0.3.0 = %v, want it refused as the developer's mistake", err)
	}

	environ := launcher.Environ([]string{"HOME=/home/dev", launcher.HandoverEnv + "=0.1.0"}, launcher.Target{Binary: "wf", Version: "0.2.0"})
	if got := strings.Join(environ, " "); got != "HOME=/home/dev "+launcher.HandoverEnv+"=0.2.0" {
		t.Errorf("Environ = %q", got)
	}
}

func readFile(t *testing.T, dir, name string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

func TestALineOfAPlatformThisWfDoesNotKnowIsKept(t *testing.T) {
	r := newReleases(t, "0.2.0")
	l := lockedCheckout(t, "0.2.0", t.TempDir(), r)
	// A later wf recorded a platform, and a kind of archive, this one does not know.
	future := "wf_0.2.0_windows_amd64.zip sha256:" + strings.Repeat("b", 64) + "\n"
	sum := readFile(t, l.Dir, project.SumFile) + future
	if err := os.WriteFile(filepath.Join(l.Dir, project.SumFile), []byte(sum), 0o644); err != nil {
		t.Fatal(err)
	}

	if target, err := l.Resolve(nil); err != nil || runTarget(t, target) != "wf 0.2.0\n" {
		t.Fatalf("Resolve with a line it does not know = (%+v, %v), want its own line used", target, err)
	}
	r.server.Close()
	if result, err := l.Lock(); err != nil || result.Changed {
		t.Errorf("Lock = (%+v, %v), want nothing changed with no network", result, err)
	}
	if got := readFile(t, l.Dir, project.SumFile); !strings.Contains(got, future) {
		t.Errorf("Lock dropped the line of a platform it does not know:\n%s", got)
	}
}

func TestAnInstallAnotherWfFinishedFirstSucceeds(t *testing.T) {
	r := newReleases(t, "0.2.0")
	l := lockedCheckout(t, "0.2.0", t.TempDir(), r)
	archive := launcher.Archive("0.2.0", platform)
	want := "sha256:" + hexSum(r.files["/v0.2.0/"+archive])
	binary := filepath.Join(l.CacheDir, "0.2.0", platform, strings.TrimPrefix(want, "sha256:"), "wf")
	// Another wf cached it between this one's look and its own install.
	if err := os.MkdirAll(filepath.Dir(binary), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binary, []byte("#!/bin/sh\necho wf 0.2.0\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := l.Install("0.2.0", archive, want, binary); err != nil {
		t.Errorf("Install over the same binary cached meanwhile = %v, want success", err)
	}
	entries, _ := os.ReadDir(filepath.Dir(filepath.Dir(binary)))
	if len(entries) != 1 {
		t.Errorf("the cache holds %d entries, want the staging directory gone", len(entries))
	}
}
