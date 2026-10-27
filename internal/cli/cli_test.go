package cli_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/workflowaiorg/workflow/internal/cli"
)

func run(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code = cli.New("0.1.0", &out, &errOut).Run(args)
	return code, out.String(), errOut.String()
}

func TestVersion(t *testing.T) {
	code, stdout, stderr := run(t, "version")
	if code != 0 || stdout != "0.1.0\n" || stderr != "" {
		t.Fatalf("wf version = (%d, %q, %q), want (0, %q, \"\")", code, stdout, stderr, "0.1.0\n")
	}
}

func TestHelpJSONListsEveryCommand(t *testing.T) {
	code, stdout, stderr := run(t, "help", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("wf help --json = (%d, stderr %q), want (0, \"\")", code, stderr)
	}

	var help struct {
		Version  string `json:"version"`
		Commands []struct {
			Name    string `json:"name"`
			Usage   string `json:"usage"`
			Summary string `json:"summary"`
		} `json:"commands"`
	}
	if err := json.Unmarshal([]byte(stdout), &help); err != nil {
		t.Fatalf("wf help --json is not JSON: %v\n%s", err, stdout)
	}
	if help.Version != "0.1.0" {
		t.Errorf("version = %q, want 0.1.0", help.Version)
	}

	var names []string
	for _, command := range help.Commands {
		if command.Usage == "" || command.Summary == "" {
			t.Errorf("command %q has no usage or summary", command.Name)
		}
		names = append(names, command.Name)
	}
	want := []string{"version", "help"}
	if len(names) != len(want) || names[0] != want[0] || names[1] != want[1] {
		t.Errorf("commands = %v, want %v", names, want)
	}
}

func TestHelpListsEveryCommand(t *testing.T) {
	code, stdout, stderr := run(t, "help")
	if code != 0 || stderr != "" {
		t.Fatalf("wf help = (%d, stderr %q), want (0, \"\")", code, stderr)
	}
	for _, usage := range []string{"wf version", "wf help [--json]"} {
		if !strings.Contains(stdout, usage) {
			t.Errorf("wf help does not list %q:\n%s", usage, stdout)
		}
	}
}

func TestExitCodes(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want int
	}{
		{name: "no arguments prints help", args: nil, want: 0},
		{name: "--help prints help", args: []string{"--help"}, want: 0},
		{name: "plain help", args: []string{"help"}, want: 0},
		{name: "unknown command", args: []string{"frobnicate"}, want: 2},
		{name: "version with an argument", args: []string{"version", "extra"}, want: 2},
		{name: "help with an unknown flag", args: []string{"help", "--yaml"}, want: 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			code, _, stderr := run(t, test.args...)
			if code != test.want {
				t.Errorf("exit code = %d, want %d (stderr %q)", code, test.want, stderr)
			}
			if test.want != 0 && stderr == "" {
				t.Error("a failing command printed nothing to stderr")
			}
		})
	}
}

func TestResolveVersionPrefersTheInjectedOne(t *testing.T) {
	if got := cli.ResolveVersion("v1.2.3"); got != "1.2.3" {
		t.Errorf("ResolveVersion(v1.2.3) = %q, want 1.2.3", got)
	}
}

func TestModuleVersion(t *testing.T) {
	tests := map[string]string{
		"v0.1.0":      "0.1.0",
		"v0.2.0-rc.1": "0.2.0-rc.1",
		"(devel)":     "devel",
		"":            "devel",
	}
	for recorded, want := range tests {
		if got := cli.ModuleVersion(recorded); got != want {
			t.Errorf("ModuleVersion(%q) = %q, want %q", recorded, got, want)
		}
	}
}
