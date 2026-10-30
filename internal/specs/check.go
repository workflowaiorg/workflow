// Package specs checks that a project's domain specs, its decision records and the records'
// index agree with each other.
//
// A spec is edited in place and a record is never edited, so the two drift in one direction: a
// decision changes, a new record supersedes the old one, and the spec that described the old
// behaviour goes on citing it. A spec citing a superseded record is the highest-value failure
// here — it means behaviour changed and the spec did not follow.
//
// Every check is structural. None of them reads a spec's prose against the code: "a failed order
// is refunded in full" is not greppable, and keeping a spec true is a person's job, done in the MR
// that made it untrue.
package specs

import (
	"fmt"
	"io"
	"slices"
	"time"
)

const (
	specDir     = ".workflow/specs"
	decisionDir = ".workflow/decisions"
	indexPath   = ".workflow/decisions/README.md"
	planDir     = ".workflow/plans"

	// A plan still in flight after this long is printed as debt, never failed on.
	planDebtAfter = 30 * 24 * time.Hour
)

// Report is what Check found: what there is, and every problem with it.
type Report struct {
	Specs    []Spec
	Records  []Record
	Plans    []Plan
	Problems []string
	now      time.Time
}

// Check reads the specs, records, index and plans of the project at root. A project with none of
// them has nothing wrong with it.
func Check(root string, now time.Time) (Report, error) {
	records, problems, err := LoadRecords(root)
	if err != nil {
		return Report{}, err
	}
	specs, err := LoadSpecs(root)
	if err != nil {
		return Report{}, err
	}
	rows, err := LoadIndex(root)
	if err != nil {
		return Report{}, err
	}
	plans, planProblems, err := LoadPlans(root)
	if err != nil {
		return Report{}, err
	}

	problems = append(problems, planProblems...)
	problems = append(problems, checkCitations(root, specs, records)...)
	problems = append(problems, checkCited(specs, records)...)
	problems = append(problems, checkIndex(records, rows)...)
	slices.Sort(problems)

	return Report{Specs: specs, Records: records, Plans: plans, Problems: problems, now: now}, nil
}

// Write prints the report: the counts, the plans in debt, and every problem.
func (report Report) Write(w io.Writer) {
	counts := map[string]int{}
	for _, record := range report.Records {
		switch {
		case record.Status == "superseded":
			counts["superseded"]++
		case record.Spec == "none":
			counts["none"]++
		default:
			counts["cited"]++
		}
	}

	var debt []Plan
	for _, plan := range report.Plans {
		if report.now.Sub(plan.Started) > planDebtAfter {
			debt = append(debt, plan)
		}
	}

	fmt.Fprintf(w, "%d specs, %d records, %d plans in flight\n", len(report.Specs), len(report.Records), len(report.Plans))
	fmt.Fprintf(w, "  cited       %d  accepted, cited by the spec it names\n", counts["cited"])
	fmt.Fprintf(w, "  spec: none  %d  accepted, process-level, cited by no spec\n", counts["none"])
	fmt.Fprintf(w, "  superseded  %d  kept for their reasoning, cited by no spec\n", counts["superseded"])
	fmt.Fprintf(w, "  plan debt   %d  in flight for more than %d days\n", len(debt), int(planDebtAfter.Hours()/24))
	for _, plan := range debt {
		fmt.Fprintf(w, "    %s/%s  %d days — settle it into %s/archive/, or finish it\n",
			planDir, plan.Name, int(report.now.Sub(plan.Started).Hours()/24), planDir)
	}

	if len(report.Problems) == 0 {
		fmt.Fprintln(w, "\nEvery citation resolves to an accepted record, and every record is cited and indexed.")
		return
	}
	noun := "problems"
	if len(report.Problems) == 1 {
		noun = "problem"
	}
	fmt.Fprintf(w, "\n%d %s:\n", len(report.Problems), noun)
	for _, problem := range report.Problems {
		fmt.Fprintln(w, "  "+problem)
	}
}
