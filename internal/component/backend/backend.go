// Package backend is the backend component: a Go backend of services, each built the same way,
// with the shared packages every service uses and the skills that say how to build one.
package backend

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"

	"github.com/workflowaiorg/workflow/internal/component"
	"github.com/workflowaiorg/workflow/internal/project"
)

// files holds base/, the backend's part of .agents/base, and skeleton/, what a project is given
// once: go.mod, pkg/ and a first service; postgres/, queue/ and llmagents/ hold the same two for
// their options.
//
//go:embed all:base all:skeleton all:postgres all:queue all:llmagents
var files embed.FS

// Component is the backend, as wf add backend installs it.
var Component = component.Component{
	Name:    "backend",
	Summary: "A Go backend of services: shared packages, a first service, and the skills to build more",
	Options: []component.Option{
		{Name: "module", Question: "The Go module path, such as github.com/acme/shop"},
		{
			Name: "postgres", Question: "Keep data in PostgreSQL", Bool: true, Default: "false",
			Part: component.Part{Base: sub("postgres/base"), Skeleton: sub("postgres/skeleton")},
		},
		{
			Name: "queue", Question: "Run jobs in the background, on a Redis queue", Bool: true, Default: "false",
			Part: component.Part{Base: sub("queue/base"), Skeleton: sub("queue/skeleton")},
		},
		{
			Name: "llm-agents", Question: "Build AI agents that call a language model", Bool: true, Default: "false",
			Part: component.Part{Base: sub("llmagents/base"), Skeleton: sub("llmagents/skeleton")},
		},
	},
	Part: component.Part{
		Base:     sub("base"),
		Skeleton: sub("skeleton"),
		Gate: []project.GateStep{
			{Name: "go format", Paths: []string{"**/*.go"}, Run: `gofmt -l .; test -z "$(gofmt -l .)"`},
			{Name: "go vet", Paths: []string{"**/*.go", "go.mod", "go.sum"}, Run: "go vet ./..."},
			// openapi.yaml too: go test is what fails on a file edited by hand.
			{Name: "go test", Paths: []string{"**/*.go", "go.mod", "go.sum", "*/openapi.yaml"}, Run: "go test ./..."},
		},
		Doctor: []project.Check{
			{Name: "go installed", Run: "go version"},
			{Name: "docker running", Run: "docker info >/dev/null"},
		},
	},
	Check: checkModule,
	Edits: func(answers map[string]any) []string {
		edits := []string{"run go mod tidy, which downloads the modules the new code imports, then go test ./..."}
		if agents, _ := answers["llm-agents"].(bool); agents {
			edits = append(edits, "add .env to .gitignore: it will hold the model's API key")
		}
		return edits
	},
	Existing: existing,
}

// checkModule checks, with Go's own rules, that the module answer is a module path and, in a project
// that has a go.mod already, that it is the module go.mod names: the files written import the
// project's own packages by it.
func checkModule(dir string, answers map[string]any) error {
	answer, _ := answers["module"].(string)
	if err := module.CheckImportPath(answer); err != nil {
		return project.UserErrorf("--module %q is not a Go module path, such as github.com/acme/shop: %v", answer, err)
	}
	content, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("reading go.mod: %w", err)
	}
	existing := modfile.ModulePath(content)
	switch {
	case existing == "":
		return project.UserErrorf("go.mod names no module; add its module line, or remove go.mod and run wf add backend again")
	case existing != answer:
		return project.UserErrorf("go.mod names the module %s, not %s; run wf add backend --module=%s", existing, answer, existing)
	}
	return nil
}

// existing says what a project's own go.mod or docker-compose.yaml lacks of what wf would have
// written: without the versions the new code was written for, go mod tidy takes the newest, which
// may need a newer Go or write openapi.yaml differently; without the services, nothing runs.
func existing(path string, content []byte) string {
	switch path {
	case "go.mod":
		file, err := modfile.ParseLax(path, content, nil)
		if err != nil {
			return ""
		}
		var modules []string
		for _, require := range file.Require {
			modules = append(modules, require.Mod.Path+"@"+require.Mod.Version)
		}
		return "the new code was written for " + strings.Join(modules, " ") + "; go get those it lacks, then go mod tidy"
	case "docker-compose.yaml":
		var compose struct {
			Services map[string]any `yaml:"services"`
			Volumes  map[string]any `yaml:"volumes"`
		}
		if err := yaml.Unmarshal(content, &compose); err != nil {
			return ""
		}
		return "add the services " + strings.Join(slices.Sorted(maps.Keys(compose.Services)), ", ") +
			" and the volumes " + strings.Join(slices.Sorted(maps.Keys(compose.Volumes)), ", ") + " to it"
	}
	return ""
}

func sub(dir string) fs.FS {
	tree, err := fs.Sub(files, dir)
	if err != nil {
		panic(err)
	}
	return tree
}
