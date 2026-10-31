package cli_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/workflowaiorg/workflow/internal/cli"
	"github.com/workflowaiorg/workflow/internal/launcher"
)

// release returns a release archive whose wf is body.
func release(t *testing.T, body string) []byte {
	t.Helper()
	var out bytes.Buffer
	compressed := gzip.NewWriter(&out)
	files := tar.NewWriter(compressed)
	if err := files.WriteHeader(&tar.Header{Name: "wf", Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	files.Write([]byte(body))
	files.Close()
	compressed.Close()
	return out.Bytes()
}

func TestMainRunsTheVersionPinned(t *testing.T) {
	const platform = "linux_amd64"
	archive := release(t, "#!/bin/sh\necho wf 0.2.0\n")
	sum := sha256.Sum256(archive)
	recorded := fmt.Sprintf("%s sha256:%s\n", launcher.Archive("0.2.0", platform), hex.EncodeToString(sum[:]))

	tests := []struct {
		name     string
		config   string // "" for no .wf.yml
		sum      string
		served   []byte // the archive the release server gives
		handover string // WF_HANDOVER_VERSION, "" for unset
		execErr  error
		wantCode int
		wantExec bool
		wantOut  string
	}{
		{name: "no pin runs itself", wantCode: 0, wantOut: "0.1.0\n"},
		{name: "its own pin runs itself", config: "wf: 0.1.0\n", wantCode: 0, wantOut: "0.1.0\n"},
		{name: "another pin hands over", config: "wf: 0.2.0\n", sum: recorded, served: archive, wantExec: true, wantCode: 1},
		{name: "no .wf.sum is the developer's mistake", config: "wf: 0.2.0\n", wantCode: 2},
		{name: "a download that fails its checksum is wf's failure", config: "wf: 0.2.0\n", sum: recorded, served: release(t, "tampered"), wantCode: 1},
		{name: "a wf started as another version is the developer's mistake", config: "wf: 0.2.0\n", handover: "0.9.9", wantCode: 2},
		{name: "a wf a launcher started runs itself", config: "wf: 0.2.0\n", handover: "0.1.0", wantCode: 0, wantOut: "0.1.0\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v0.2.0/"+launcher.Archive("0.2.0", platform) || test.served == nil {
					http.NotFound(w, r)
					return
				}
				w.Write(test.served)
			}))
			defer server.Close()
			defer cli.ConfigureLauncher(func(l *launcher.Launcher) {
				l.ReleasesURL, l.Platform, l.CacheDir, l.NoLaunch = server.URL, platform, filepath.Join(t.TempDir(), "cache"), false
			})()
			if test.handover != "" {
				t.Setenv(launcher.HandoverEnv, test.handover)
			}
			dir := t.TempDir()
			for name, content := range map[string]string{".wf.yml": test.config, ".wf.sum": test.sum} {
				if content != "" {
					if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
						t.Fatal(err)
					}
				}
			}

			var execed []string
			fakeExec := func(binary string, argv, env []string) error {
				execed = append([]string{binary}, argv[1:]...)
				if !slices.Contains(env, launcher.HandoverEnv+"=0.2.0") {
					t.Errorf("handed over with no %s=0.2.0", launcher.HandoverEnv)
				}
				return errors.New("exec failed")
			}
			var stdout, stderr bytes.Buffer
			code := cli.Main("0.1.0", dir, []string{"version"}, []string{"HOME=/home/dev"}, strings.NewReader(""), &stdout, &stderr, fakeExec)

			if code != test.wantCode || stdout.String() != test.wantOut {
				t.Errorf("Main = (%d, %q, stderr %q), want (%d, %q)", code, stdout.String(), stderr.String(), test.wantCode, test.wantOut)
			}
			if handedOver := execed != nil; handedOver != test.wantExec {
				t.Fatalf("handed over = %v (%v), want %v", handedOver, execed, test.wantExec)
			}
			if test.wantExec {
				if out, err := exec.Command(execed[0]).Output(); err != nil || string(out) != "wf 0.2.0\n" || execed[1] != "version" {
					t.Errorf("handed over to %v, which printed %q (%v); want the cached 0.2.0 run with version", execed, out, err)
				}
				if !strings.Contains(stderr.String(), "delete that folder") {
					t.Errorf("an exec that failed says %q, want it to name the fix", stderr.String())
				}
			}
		})
	}
}
