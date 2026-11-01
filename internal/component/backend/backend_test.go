package backend_test

import (
	"bytes"
	"go/format"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/workflowaiorg/workflow/internal/base"
	"github.com/workflowaiorg/workflow/internal/component/backend"
)

// written renders the backend's skeleton for the module example.com/shop into a new directory.
func written(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if _, err := base.ComponentSkeleton(dir, backend.Component, map[string]any{"module": "example.com/shop"}); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestTheSkeletonIsGoInTheProjectsModule(t *testing.T) {
	dir := written(t)
	goMod, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil || !strings.HasPrefix(string(goMod), "module example.com/shop\n") {
		t.Fatalf("go.mod = %q, %v; want module example.com/shop", goMod, err)
	}

	files := 0
	err = filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		if strings.HasSuffix(rel, ".tmpl") || bytes.Contains(content, []byte("[[")) {
			t.Errorf("%s was not rendered", rel)
		}
		if strings.HasSuffix(rel, ".go") {
			files++
			formatted, err := format.Source(content)
			switch {
			case err != nil:
				t.Errorf("%s does not parse: %v", rel, err)
			case !bytes.Equal(formatted, content):
				t.Errorf("%s is not gofmt-formatted", rel)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if files == 0 {
		t.Error("the skeleton holds no Go files")
	}
}

func TestTheFirstServiceIsInTheStack(t *testing.T) {
	compose, err := os.ReadFile(filepath.Join(written(t), "docker-compose.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(compose), `command: ["sh", "-c", "go build -o /tmp/api ./api/cmd/http && exec /tmp/api"]`) {
		t.Errorf("docker-compose.yaml does not run api:\n%s", compose)
	}
}

func TestTheModuleIsChecked(t *testing.T) {
	tests := []struct {
		name   string
		module string
		goMod  string // "" for none
		want   string // "" for accepted
	}{
		{name: "a module path", module: "github.com/acme/shop"},
		{name: "a space", module: "my shop", want: "is not a Go module path"},
		{name: "a quote", module: `acme"shop`, want: "is not a Go module path"},
		{name: "an empty element", module: "acme//shop", want: "is not a Go module path"},
		{name: "a parent element", module: "acme/../shop", want: "is not a Go module path"},
		{name: "a current-directory element", module: "./shop", want: "is not a Go module path"},
		{name: "one element", module: "shop"},
		{name: "go.mod's own module", module: "example.com/shop", goMod: "module example.com/shop\n\ngo 1.25\n"},
		{name: "another module than go.mod's", module: "example.com/other", goMod: "module example.com/shop\n", want: "go.mod names the module example.com/shop"},
		{name: "go.mod's module with a comment after it", module: "example.com/shop", goMod: "module example.com/shop // the shop\n"},
		{name: "go.mod's module quoted", module: "example.com/shop", goMod: "module \"example.com/shop\"\n"},
		{name: "a go.mod with no module", module: "example.com/shop", goMod: "go 1.25\n", want: "go.mod names no module"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			if test.goMod != "" {
				if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(test.goMod), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			err := backend.Component.Check(dir, map[string]any{"module": test.module})
			switch {
			case test.want == "" && err != nil:
				t.Errorf("Check(%q) = %v, want it accepted", test.module, err)
			case test.want != "" && (err == nil || !strings.Contains(err.Error(), test.want)):
				t.Errorf("Check(%q) = %v, want it to say %q", test.module, err, test.want)
			}
		})
	}
}
