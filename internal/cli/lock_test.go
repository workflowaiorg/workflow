package cli_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/workflowaiorg/workflow/internal/cli"
	"github.com/workflowaiorg/workflow/internal/launcher"
)

func TestLock(t *testing.T) {
	sum := sha256.Sum256([]byte("archive"))
	var checksums strings.Builder
	for _, platform := range launcher.Platforms {
		fmt.Fprintf(&checksums, "%s  %s\n", hex.EncodeToString(sum[:]), launcher.Archive("0.2.0", platform))
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v0.2.0/checksums.txt" {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(checksums.String()))
	}))
	defer server.Close()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".wf.yml"), []byte("wf: 0.2.0\nforge: gitlab\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	lock := func() (int, string, string) {
		app := cli.New("0.1.0", dir, strings.NewReader(""), new(bytes.Buffer), new(bytes.Buffer))
		app.SetReleases(server.URL)
		return runWith(t, app, "lock")
	}
	if code, stdout, stderr := lock(); code != 0 || stdout != "wrote    .wf.sum (wf 0.2.0's fingerprints: commit it)\n" {
		t.Fatalf("wf lock = (%d, %q, %q), want .wf.sum written", code, stdout, stderr)
	}
	if content, _ := os.ReadFile(filepath.Join(dir, ".wf.sum")); strings.Count(string(content), "\n") != len(launcher.Platforms) {
		t.Errorf(".wf.sum =\n%s", content)
	}
	if code, stdout, _ := lock(); code != 0 || stdout != ".wf.sum already holds wf 0.2.0's fingerprints.\n" {
		t.Errorf("a second wf lock = (%d, %q), want nothing to do", code, stdout)
	}

	if code, _, stderr := runIn(t, t.TempDir(), "lock"); code != 2 || !strings.Contains(stderr, "no .wf.yml") {
		t.Errorf("wf lock with no .wf.yml = (%d, %q), want exit 2", code, stderr)
	}
}
