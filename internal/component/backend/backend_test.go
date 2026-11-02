package backend_test

import (
	"bytes"
	"fmt"
	"go/format"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/workflowaiorg/workflow/internal/base"
	"github.com/workflowaiorg/workflow/internal/component/backend"
)

// options is which of the backend's options are chosen.
type options struct {
	postgres, queue bool
}

// everyChoice is every way the options can be chosen.
var everyChoice = []options{{}, {postgres: true}, {queue: true}, {postgres: true, queue: true}}

// written renders the backend's skeleton for the module example.com/shop into a new directory,
// with the options chosen.
func written(t *testing.T, chosen options) string {
	t.Helper()
	dir := t.TempDir()
	answers := map[string]any{"module": "example.com/shop", "postgres": chosen.postgres, "queue": chosen.queue}
	if _, err := base.ComponentSkeleton(dir, backend.Component, answers); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestTheSkeletonIsGoInTheProjectsModule(t *testing.T) {
	for _, chosen := range everyChoice {
		t.Run(fmt.Sprintf("%+v", chosen), func(t *testing.T) { checkSkeleton(t, written(t, chosen)) })
	}
}

// checkSkeleton checks that the skeleton written into dir is rendered, gofmt-formatted Go in the
// module example.com/shop.
func checkSkeleton(t *testing.T, dir string) {
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
	compose, err := os.ReadFile(filepath.Join(written(t, options{}), "docker-compose.yaml"))
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

func TestPostgresExistsOnlyWhenChosen(t *testing.T) {
	for _, postgres := range []bool{false, true} {
		dir := written(t, options{postgres: postgres})
		_, err := os.Stat(filepath.Join(dir, "pkg/postgres/postgres.go"))
		if (err == nil) != postgres {
			t.Errorf("with postgres %v, pkg/postgres exists = %v", postgres, err == nil)
		}
		// What each file holds of the database, when it is chosen.
		marks := map[string]string{
			"docker-compose.yaml":           "postgres",
			"api/internal/app/app.go":       "postgres.Open",
			"api/internal/config/config.go": "PostgresDSN",
			"api/cmd/http/main.go":          "DB.Close",
		}
		for name, mark := range marks {
			content, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				t.Fatal(err)
			}
			if got := strings.Contains(string(content), mark); got != postgres {
				t.Errorf("with postgres %v, %s holds %s = %v:\n%s", postgres, name, mark, got, content)
			}
		}
	}
}

func TestPostgresComesUpBeforeTheService(t *testing.T) {
	compose, err := os.ReadFile(filepath.Join(written(t, options{postgres: true}), "docker-compose.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"    depends_on:\n      migrate:\n        condition: service_completed_successfully\n\n  postgres:\n",
		"    depends_on:\n      postgres:\n        condition: service_healthy\n",
		"github.com/pressly/goose/v3/cmd/goose@v3.27.0 up",
		"  postgres-data:\n",
	} {
		if !strings.Contains(string(compose), want) {
			t.Errorf("docker-compose.yaml does not hold %q:\n%s", want, compose)
		}
	}
}

func TestTheQueueExistsOnlyWhenChosen(t *testing.T) {
	for _, queue := range []bool{false, true} {
		dir := written(t, options{queue: queue})
		_, err := os.Stat(filepath.Join(dir, "pkg/queue/queue.go"))
		if (err == nil) != queue {
			t.Errorf("with queue %v, pkg/queue exists = %v", queue, err == nil)
		}
		// What each file holds of the queue, when it is chosen.
		marks := map[string]string{
			"docker-compose.yaml":           "redis",
			"api/internal/app/app.go":       "queue.New",
			"api/internal/config/config.go": "RedisAddr",
			"api/cmd/http/main.go":          "Workers.Wait",
		}
		for name, mark := range marks {
			content, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				t.Fatal(err)
			}
			if got := strings.Contains(string(content), mark); got != queue {
				t.Errorf("with queue %v, %s holds %s = %v:\n%s", queue, name, mark, got, content)
			}
		}
	}
}

func TestTheServiceWaitsForEveryStoreItUses(t *testing.T) {
	tests := []struct {
		chosen options
		want   string
	}{
		{options{queue: true}, "    depends_on:\n      redis:\n        condition: service_healthy\n\n"},
		{
			options{postgres: true, queue: true},
			"    depends_on:\n      migrate:\n        condition: service_completed_successfully\n      redis:\n        condition: service_healthy\n\n",
		},
	}
	for _, test := range tests {
		compose, err := os.ReadFile(filepath.Join(written(t, test.chosen), "docker-compose.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{test.want, "  redis-data:\n"} {
			if !strings.Contains(string(compose), want) {
				t.Errorf("with %+v, docker-compose.yaml does not hold %q:\n%s", test.chosen, want, compose)
			}
		}
	}
}
