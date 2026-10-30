package specs

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

// Plan is one directory under .workflow/plans/ that has not been settled into
// archive/.
type Plan struct {
	Name    string
	Started time.Time
}

var planName = regexp.MustCompile(`^(\d{2}-\d{2}-\d{4})-[a-z0-9]+(-[a-z0-9]+)*$`)

// LoadPlans reads the plans in flight. The date a plan was started is the one in
// its directory name, not git's: a move or a rename gives a plan a new first
// commit. A name that does not parse is a problem; age never is.
func LoadPlans(root string) ([]Plan, []string, error) {
	entries, err := os.ReadDir(filepath.Join(root, planDir))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("reading %s: %w", planDir, err)
	}

	var (
		plans    []Plan
		problems []string
	)

	for _, entry := range entries {
		name := entry.Name()
		if name == "archive" {
			continue
		}

		path := filepath.Join(planDir, name)
		if !entry.IsDir() {
			problems = append(problems, fmt.Sprintf(
				"%s: a file loose in %s/ — everything a plan needs sits beside its PLAN.md, in its own directory",
				path, planDir,
			))
			continue
		}

		match := planName.FindStringSubmatch(name)
		var started time.Time
		if match != nil {
			started, err = time.Parse("02-01-2006", match[1])
		}
		if match == nil || err != nil {
			problems = append(problems, fmt.Sprintf(
				"%s: not a plan name — a plan lives at %s/<DD-MM-YYYY>-<short-slug>/, dated the day it was started",
				path, planDir,
			))
			continue
		}

		plans = append(plans, Plan{Name: name, Started: started})
	}

	return plans, problems, nil
}
