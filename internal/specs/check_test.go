package specs_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/workflowaiorg/workflow/internal/specs"
)

var now = time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC)

// sample copies the valid sample into a new directory and returns it.
func sample(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.CopyFS(dir, os.DirFS("testdata/valid")); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestTheSampleHasNoProblems(t *testing.T) {
	report, err := specs.Check(sample(t), now)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Problems) != 0 || len(report.Specs) != 2 || len(report.Records) != 4 || len(report.Plans) != 1 {
		t.Errorf("Check = %d specs, %d records, %d plans, problems %q; want 2, 4, 1 and none",
			len(report.Specs), len(report.Records), len(report.Plans), report.Problems)
	}
}

func TestAProjectWithoutWorkflowHasNoProblems(t *testing.T) {
	report, err := specs.Check(t.TempDir(), now)
	if err != nil || len(report.Problems) != 0 {
		t.Errorf("Check on an empty project = (%q, %v), want no problems", report.Problems, err)
	}
}

func TestPlanDebtIsReportedNeverFailed(t *testing.T) {
	report, err := specs.Check(sample(t), now.AddDate(0, 2, 0))
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	report.Write(&out)
	if len(report.Problems) != 0 || !strings.Contains(out.String(), "02-03-2026-order-history  79 days") {
		t.Errorf("a plan two months old: problems %q, report\n%s\nwant no problems and the plan as debt", report.Problems, out.String())
	}
}

func TestEachRuleBrokenIsAProblem(t *testing.T) {
	const (
		ordersSpec = ".workflow/specs/orders/SPEC.md"
		record1    = ".workflow/decisions/0001-cancellation-costs-a-fee.md"
		record2    = ".workflow/decisions/0002-late-message-is-never-applied.md"
		record3    = ".workflow/decisions/0003-cancellation-refunds-in-full.md"
		index      = ".workflow/decisions/README.md"
	)
	tests := []struct {
		name    string
		breakIt func(t *testing.T, dir string)
		want    string
	}{
		{"a mention with no link", appendTo(ordersSpec, "\nSee ADR-0002.\n"), "ADR-0002 is mentioned without a link"},
		{"a link to no record", replace(ordersSpec, "0002-late-message-is-never-applied.md", "0002-gone.md"), "which is not a record"},
		{"a link to another record", replace(ordersSpec, "[ADR-0002]", "[ADR-0004]"), "[ADR-0004] links to .workflow/decisions/0002-late-message-is-never-applied.md, which is ADR-0002"},
		{"a superseded record cited", appendTo(ordersSpec, "\n→ [ADR-0001](../../decisions/0001-cancellation-costs-a-fee.md)\n"), "ADR-0001 is superseded by ADR-0003"},
		{"an accepted record cited nowhere", replace(ordersSpec, "→ [ADR-0002](../../decisions/0002-late-message-is-never-applied.md#decision)", ""), "ADR-0002 is cited nowhere"},
		{"a missing key", replace(record2, "issue: 7\n", ""), "`issue:` is missing or empty"},
		{"no frontmatter", write(".workflow/decisions/0005-no-frontmatter.md", "# Nothing here\n"), "no frontmatter"},
		{"an id that is not the file's number", replace(record2, "id: 0002", "id: 0009"), "id 0009 does not match the file name"},
		{"an id taken twice", copyFile(record2, ".workflow/decisions/0002-a-second-one.md"), "id 0002 is taken by"},
		{"a file that is not a record", write(".workflow/decisions/notes.md", "loose\n"), "not a record name"},
		{"a status there is not", replace(record2, "status: accepted", "status: proposed"), `status "proposed"`},
		{"a date that is not a date", replace(record2, "date: 2026-01-12", "date: 12.01.2026"), "is not YYYY-MM-DD"},
		{"an issue that is not a number", replace(record2, "issue: 7", "issue: #7"), "is not a number"},
		{"a spec of another domain", replace(record2, "spec: .workflow/specs/orders/SPEC.md", "spec: .workflow/specs/billing/SPEC.md"), "is not the spec of domain"},
		{"a spec that does not exist", replace(record2, "domain: orders\nspec: .workflow/specs/orders/SPEC.md", "domain: shipping\nspec: .workflow/specs/shipping/SPEC.md"), "does not exist"},
		{"supersedes a record there is not", replace(record3, "supersedes: 0001", "supersedes: 0008"), "there is no record 0008"},
		{"supersedes one that does not say so", replace(record1, "superseded_by: 0003", "superseded_by: 0002"), "does not say `superseded_by: 0003`"},
		{"superseded by an earlier one", replace(record2, "status: accepted\n", "status: superseded\n"), "status is superseded, so superseded_by names the record"},
		{"accepted but superseded", replace(record2, "superseded_by: —", "superseded_by: 0003"), "but the status is accepted"},
		{"a record with no row", replace(index, "| [0004](0004-squash-on-merge.md) | Merge requests are squashed | none | accepted | #15 | — | — |\n", ""), "no row in .workflow/decisions/README.md"},
		{"a row that disagrees", replace(index, "| Merge requests are squashed |", "| Merges are squashed |"), "0004 title is \"Merges are squashed\" here"},
		{"a row with no record", appendTo(index, "| [0009](0009-gone.md) | Gone | none | accepted | #20 | — | — |\n"), "0009 is indexed but"},
		{"a second row", appendTo(index, "| [0004](0004-squash-on-merge.md) | Merge requests are squashed | none | accepted | #15 | — | — |\n"), "0004 has a second row"},
		{"rows that do not name each other", replace(index, "| #12 | 0001 | — |", "| #12 | — | — |"), "0001 is superseded by 0003, but 0003's row does not say it supersedes 0001"},
		{"a plan not named by its date", mkdir(".workflow/plans/order-history"), "not a plan name"},
		{"a file loose in plans", write(".workflow/plans/notes.md", "loose\n"), "a file loose in .workflow/plans/"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := sample(t)
			test.breakIt(t, dir)
			report, err := specs.Check(dir, now)
			if err != nil {
				t.Fatal(err)
			}
			if !containsAny(report.Problems, test.want) {
				t.Errorf("problems = %q, want one holding %q", report.Problems, test.want)
			}
		})
	}
}

func containsAny(problems []string, want string) bool {
	for _, problem := range problems {
		if strings.Contains(problem, want) {
			return true
		}
	}
	return false
}

func replace(name, old, new string) func(*testing.T, string) {
	return func(t *testing.T, dir string) {
		t.Helper()
		content, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(content), old) {
			t.Fatalf("%s holds no %q to replace", name, old)
		}
		write(name, strings.Replace(string(content), old, new, 1))(t, dir)
	}
}

func appendTo(name, text string) func(*testing.T, string) {
	return func(t *testing.T, dir string) {
		t.Helper()
		content, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		write(name, string(content)+text)(t, dir)
	}
}

func copyFile(from, to string) func(*testing.T, string) {
	return func(t *testing.T, dir string) {
		t.Helper()
		content, err := os.ReadFile(filepath.Join(dir, from))
		if err != nil {
			t.Fatal(err)
		}
		write(to, string(content))(t, dir)
	}
}

func write(name, content string) func(*testing.T, string) {
	return func(t *testing.T, dir string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func mkdir(name string) func(*testing.T, string) {
	return func(t *testing.T, dir string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Join(dir, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
}
