// Package frontend is the frontend component: a Vue app in web/, typed against the backend's API
// contract, with the skill that says how to build its screens.
package frontend

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/workflowaiorg/workflow/internal/component"
	"github.com/workflowaiorg/workflow/internal/project"
)

// files holds base/, the frontend's part of .agents/base, and skeleton/, what a project is given
// once: the app in web/.
//
//go:embed all:base all:skeleton
var files embed.FS

// contract is the API description the app's types are generated from: the backend's first service.
const contract = "api/openapi.yaml"

// Component is the frontend, as wf add frontend installs it.
var Component = component.Component{
	Name:    "frontend",
	Summary: "A Vue app in web/, typed against the backend's API, and the skill to build its screens",
	Part: component.Part{
		Base:     sub("base"),
		Skeleton: sub("skeleton"),
		Gate: []project.GateStep{
			// The contract too: the lint fails when the types were not generated from it again.
			{Name: "web lint", Paths: []string{"web/**", contract}, Run: "cd web && npm run lint"},
			{Name: "web typecheck", Paths: []string{"web/**", contract}, Run: "cd web && npm run typecheck"},
			{Name: "web build", Paths: []string{"web/**"}, Run: "cd web && npm run build"},
		},
		Doctor: []project.Check{
			// Vite refuses an older Node, and the CI templates run 24.
			{Name: "node 24 or newer installed", Run: `node -e "process.exit(Number(process.versions.node.split('.')[0]) >= 24 ? 0 : 1)"`},
		},
	},
	Check: checkContract,
	Edits: func(map[string]any) []string {
		return []string{
			"add include: [web/compose.yaml] to the top of docker-compose.yaml, so the stack starts the app",
			"in web/, run npm install, then npm run api:types, and commit package-lock.json and src/services/api.gen.d.ts",
		}
	},
	Existing: existing,
}

// existing says what a project's own web/package.json lacks of what wf would have written: the gate
// steps and the CI templates run its scripts, and the lint runs openapi-typescript.
func existing(path string, content []byte) string {
	if path != "web/package.json" {
		return ""
	}
	var manifest struct {
		Scripts         map[string]string `json:"scripts"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if err := json.Unmarshal(content, &manifest); err != nil {
		return ""
	}
	var scripts, packages []string
	for _, name := range slices.Sorted(maps.Keys(manifest.Scripts)) {
		scripts = append(scripts, name+": "+manifest.Scripts[name])
	}
	for _, name := range slices.Sorted(maps.Keys(manifest.DevDependencies)) {
		packages = append(packages, name+"@"+manifest.DevDependencies[name])
	}
	return "the gate and CI run its scripts " + strings.Join(scripts, "; ") +
		", and the lint and the type check need " + strings.Join(packages, " ") + "; add those it lacks"
}

// checkContract checks that the project has the API the app is typed against.
func checkContract(dir string, _ map[string]any) error {
	_, err := os.Stat(filepath.Join(dir, contract))
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return project.UserErrorf("the frontend's types come from the backend's %s, and there is none: run wf add backend first", contract)
	case err != nil:
		return fmt.Errorf("reading %s: %w", contract, err)
	}
	return nil
}

func sub(dir string) fs.FS {
	tree, err := fs.Sub(files, dir)
	if err != nil {
		panic(err)
	}
	return tree
}
