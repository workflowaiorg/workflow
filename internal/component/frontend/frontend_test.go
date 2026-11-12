package frontend_test

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/workflowaiorg/workflow/internal/base"
	"github.com/workflowaiorg/workflow/internal/component/backend"
	"github.com/workflowaiorg/workflow/internal/component/frontend"
)

// skeleton returns, by path, the files the frontend gives a project.
func skeleton(t *testing.T) map[string][]byte {
	t.Helper()
	files, err := base.RenderComponentSkeleton(frontend.Component, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// packageJSON returns the skeleton's web/package.json.
func packageJSON(t *testing.T) (manifest struct {
	Engines map[string]string `json:"engines"`
	Scripts map[string]string `json:"scripts"`
}) {
	t.Helper()
	if err := json.Unmarshal(skeleton(t)["web/package.json"], &manifest); err != nil {
		t.Fatal(err)
	}
	return manifest
}

func TestTheFrontendNeedsTheBackend(t *testing.T) {
	dir := t.TempDir()
	err := frontend.Component.Check(dir, map[string]any{})
	if err == nil || !strings.Contains(err.Error(), "run wf add backend first") {
		t.Errorf("Check without api/openapi.yaml = %v, want it to say to add the backend first", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "api"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "api", "openapi.yaml"), []byte("openapi: 3.0.3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := frontend.Component.Check(dir, map[string]any{}); err != nil {
		t.Errorf("Check with api/openapi.yaml = %v, want it accepted", err)
	}
}

// TestTheTypesComeFromTheBackendsContract checks that the app generates its types from the file the
// backend writes, and that the gate steps that read the types run when it changes.
func TestTheTypesComeFromTheBackendsContract(t *testing.T) {
	if _, err := fs.Stat(backend.Component.Skeleton, "api/openapi.yaml"); err != nil {
		t.Fatalf("the backend writes no api/openapi.yaml: %v", err)
	}
	if script := packageJSON(t).Scripts["api:types"]; !strings.Contains(script, "../api/openapi.yaml") {
		t.Errorf("npm run api:types runs %q, which does not read ../api/openapi.yaml", script)
	}
	for _, step := range frontend.Component.Gate {
		reads := step.Name != "web build"
		if got := strings.Contains(strings.Join(step.Paths, " "), "api/openapi.yaml"); got != reads {
			t.Errorf("the gate step %q runs on %q; want api/openapi.yaml among them: %v", step.Name, step.Paths, reads)
		}
	}
}

func TestTheGateRunsTheAppsOwnScripts(t *testing.T) {
	scripts := packageJSON(t).Scripts
	for _, step := range frontend.Component.Gate {
		script, ok := strings.CutPrefix(step.Run, "cd web && npm run ")
		if !ok || scripts[script] == "" {
			t.Errorf("the gate step %q runs %q, not one of web/package.json's scripts", step.Name, step.Run)
		}
	}
}

func TestTheStackRunsTheNodeTheAppNeeds(t *testing.T) {
	var compose struct {
		Services map[string]struct {
			Image string `yaml:"image"`
		} `yaml:"services"`
	}
	if err := yaml.Unmarshal(skeleton(t)["web/compose.yaml"], &compose); err != nil {
		t.Fatal(err)
	}
	major := strings.TrimPrefix(packageJSON(t).Engines["node"], "^")
	if image := compose.Services["web"].Image; !strings.HasPrefix(image, "node:"+major+"-") {
		t.Errorf("web/compose.yaml runs %q, want node %s, the major web/package.json's engines names", image, major)
	}
}

func TestAnExistingPackageJSONGetsWhatTheGateRuns(t *testing.T) {
	hint := frontend.Component.Existing("web/package.json", skeleton(t)["web/package.json"])
	for _, want := range []string{"lint: node scripts/lint.mjs", "typecheck: vue-tsc --build", "api:types: ", "openapi-typescript@^", "vue-tsc@^"} {
		if !strings.Contains(hint, want) {
			t.Errorf("the hint for an existing web/package.json does not say %q:\n%s", want, hint)
		}
	}
	if hint := frontend.Component.Existing("web/src/main.ts", nil); hint != "" {
		t.Errorf("Existing(web/src/main.ts) = %q, want nothing", hint)
	}
}
