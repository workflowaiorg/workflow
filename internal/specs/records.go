package specs

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Record is the frontmatter of one .workflow/decisions/NNNN-<slug>.md.
type Record struct {
	ID         string
	Title      string
	Date       string
	Status     string
	Domain     string
	Spec       string
	Supersedes string
	Issue      string

	path  string // relative to the repository root
	lines map[string]int
}

// where names the line of one frontmatter key, for a problem about it.
func (r Record) where(key string) string {
	return fmt.Sprintf("%s:%d", r.path, r.lines[key])
}

var (
	recordFile = regexp.MustCompile(`^(\d{4})-[a-z0-9-]+\.md$`)
	recordID   = regexp.MustCompile(`^\d{4}$`)
	dateField  = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

	requiredKeys = []string{"id", "title", "date", "status", "domain", "spec", "issue"}
)

// none is how a record's frontmatter says a field is empty: `supersedes: —`. `supersedes:` is also
// the one key a record may leave out, meaning the same: records written before it existed have
// none, and a record is never edited to add one.
const none = "—"

// LoadRecords reads every record's frontmatter and checks each on its own: every
// key present, the id matching the file name and unique, the one status, and a
// supersession whose old record is gone. Problems that need the specs or the
// index are checked elsewhere.
func LoadRecords(root string) ([]Record, []string, error) {
	entries, err := os.ReadDir(filepath.Join(root, decisionDir))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("reading %s: %w", decisionDir, err)
	}

	var (
		records  []Record
		problems []string
	)

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || name == "README.md" {
			continue
		}

		path := filepath.Join(decisionDir, name)
		match := recordFile.FindStringSubmatch(name)
		if match == nil {
			problems = append(problems, fmt.Sprintf(
				"%s: not a record name — a record is NNNN-<kebab-case-slug>.md, and nothing else lives in %s/",
				path, decisionDir,
			))
			continue
		}

		record, recordProblems, err := parseRecord(root, path)
		if err != nil {
			return nil, nil, err
		}
		problems = append(problems, recordProblems...)

		if record.ID != "" && record.ID != match[1] {
			problems = append(problems, fmt.Sprintf(
				"%s: id %s does not match the file name — a record's id is its file's number",
				record.where("id"), record.ID,
			))
		}
		record.ID = match[1]
		records = append(records, record)
	}

	byID := map[string]Record{}
	for _, record := range records {
		if first, ok := byID[record.ID]; ok {
			problems = append(problems, fmt.Sprintf(
				"%s: id %s is taken by %s too — renumber the later one to one above the highest number ever used",
				record.path, record.ID, first.path,
			))
			continue
		}
		byID[record.ID] = record
	}

	for _, record := range records {
		problems = append(problems, checkRecord(root, record, byID)...)
	}

	return records, problems, nil
}

func parseRecord(root, path string) (Record, []string, error) {
	file, err := os.Open(filepath.Join(root, path))
	if err != nil {
		return Record{}, nil, fmt.Errorf("reading %s: %w", path, err)
	}
	defer file.Close()

	record := Record{path: path, lines: map[string]int{}}
	fields := map[string]string{}

	scanner := bufio.NewScanner(file)
	line, open, closed := 0, false, false
	for scanner.Scan() {
		line++
		text := strings.TrimSpace(scanner.Text())

		if text == "---" {
			if open {
				closed = true
				break
			}
			if line == 1 {
				open = true
				continue
			}
		}
		if !open {
			break
		}

		key, value, ok := strings.Cut(text, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		fields[key] = strings.TrimSpace(value)
		record.lines[key] = line
	}
	if err := scanner.Err(); err != nil {
		return Record{}, nil, fmt.Errorf("%s: %w", path, err)
	}

	if !closed {
		return record, []string{fmt.Sprintf(
			"%s:1: no frontmatter — a record opens with the block from .workflow/DECISION-TEMPLATE.md", path,
		)}, nil
	}

	var problems []string
	for _, key := range requiredKeys {
		if fields[key] == "" {
			problems = append(problems, fmt.Sprintf(
				"%s:1: `%s:` is missing or empty", path, key,
			))
		}
	}

	record.ID = fields["id"]
	record.Title = fields["title"]
	record.Date = fields["date"]
	record.Status = fields["status"]
	record.Domain = fields["domain"]
	record.Spec = fields["spec"]
	record.Supersedes = fields["supersedes"]
	if record.Supersedes == "" {
		record.Supersedes = none
	}
	record.Issue = fields["issue"]

	return record, problems, nil
}

// checkRecord checks one record's fields against each other and against the other
// records: the checks that need neither a spec nor the index.
func checkRecord(root string, record Record, byID map[string]Record) []string {
	var problems []string

	if record.Date != "" && !dateField.MatchString(record.Date) {
		problems = append(problems, fmt.Sprintf(
			"%s: date %q is not YYYY-MM-DD", record.where("date"), record.Date,
		))
	}

	if record.Issue != "" {
		if _, err := strconv.Atoi(record.Issue); err != nil {
			problems = append(problems, fmt.Sprintf(
				"%s: issue %q is not a number — it is the ADR Issue the record was accepted in, without the #",
				record.where("issue"), record.Issue,
			))
		}
	}

	if record.Spec != "" && record.Spec != "none" {
		want := fmt.Sprintf("%s/%s/SPEC.md", specDir, record.Domain)
		switch {
		case record.Spec != want:
			problems = append(problems, fmt.Sprintf(
				"%s: spec %s is not the spec of domain %q — it is %s, or `none` for a process-level decision",
				record.where("spec"), record.Spec, record.Domain, want,
			))
		case !exists(filepath.Join(root, record.Spec)):
			problems = append(problems, fmt.Sprintf(
				"%s: spec %s does not exist", record.where("spec"), record.Spec,
			))
		}
	}

	if record.Supersedes != none {
		switch older, ok := byID[record.Supersedes]; {
		case !recordID.MatchString(record.Supersedes):
			problems = append(problems, fmt.Sprintf(
				"%s: supersedes names the record this one replaced, as NNNN, or is `—`",
				record.where("supersedes"),
			))
		case record.Supersedes >= record.ID:
			problems = append(problems, fmt.Sprintf(
				"%s: supersedes %s — a record supersedes an earlier one, never a later one",
				record.where("supersedes"), record.Supersedes,
			))
		case ok:
			problems = append(problems, fmt.Sprintf(
				"%s: supersedes %s, but %s is still here — the MR that lands a record deletes the one it supersedes, with its index row",
				record.where("supersedes"), record.Supersedes, older.path,
			))
		}
	}

	if _, ok := record.lines["superseded_by"]; ok {
		problems = append(problems, fmt.Sprintf(
			"%s: `superseded_by:` is gone — a superseded record is deleted, and the new one says `supersedes:`",
			record.where("superseded_by"),
		))
	}

	if record.Status != "" && record.Status != "accepted" {
		problems = append(problems, fmt.Sprintf(
			"%s: status %q — a record is `accepted`; there is no proposed status, the ADR Issue is the proposal, and a superseded record is deleted",
			record.where("status"), record.Status,
		))
	}

	return problems
}

// Row is one row of the table in .workflow/decisions/README.md.
type Row struct {
	ID         string
	Link       string
	Title      string
	Domain     string
	Status     string
	Issue      string
	Supersedes []string

	line int
}

var (
	rowLink = regexp.MustCompile(`^\[(\d{4})\]\(([^)\s]+)\)$`)
	anyID   = regexp.MustCompile(`\d{4}`)
)

// LoadIndex reads the index table's rows. A row is any table line whose first
// cell is a link; the header and the separator are not.
func LoadIndex(root string) ([]Row, error) {
	raw, err := os.ReadFile(filepath.Join(root, indexPath))
	if errors.Is(err, fs.ErrNotExist) {
		// With no index, every record is reported as having no row.
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", indexPath, err)
	}

	var rows []Row
	for n, text := range strings.Split(string(raw), "\n") {
		if !strings.HasPrefix(text, "| [") {
			continue
		}

		cells := strings.Split(strings.Trim(strings.TrimSpace(text), "|"), "|")
		for i := range cells {
			cells[i] = strings.TrimSpace(cells[i])
		}

		row := Row{line: n + 1}
		if match := rowLink.FindStringSubmatch(cells[0]); match != nil {
			row.ID, row.Link = match[1], match[2]
		}
		if len(cells) == 6 {
			row.Title, row.Domain, row.Status, row.Issue = cells[1], cells[2], cells[3], cells[4]
			row.Supersedes = anyID.FindAllString(cells[5], -1)
		}
		rows = append(rows, row)
	}

	return rows, nil
}

// checkIndex checks that every record has exactly one row and that the row says what
// the file says.
func checkIndex(records []Record, rows []Row) []string {
	var problems []string
	at := func(row Row) string { return fmt.Sprintf("%s:%d", indexPath, row.line) }

	byID := map[string]Record{}
	for _, record := range records {
		byID[record.ID] = record
	}

	indexed := map[string]Row{}
	for _, row := range rows {
		if row.ID == "" || row.Status == "" {
			problems = append(problems, fmt.Sprintf(
				"%s: not a row this check can read — | [NNNN](NNNN-slug.md) | title | domain | status | #issue | supersedes |",
				at(row),
			))
			continue
		}
		if _, ok := indexed[row.ID]; ok {
			problems = append(problems, fmt.Sprintf("%s: %s has a second row — delete one", at(row), row.ID))
			continue
		}
		indexed[row.ID] = row

		record, ok := byID[row.ID]
		if !ok {
			problems = append(problems, fmt.Sprintf(
				"%s: %s is indexed but %s/ has no record %s", at(row), row.ID, decisionDir, row.ID,
			))
			continue
		}

		problems = append(problems, checkRow(at(row), row, record)...)
	}

	for _, record := range records {
		if _, ok := indexed[record.ID]; !ok {
			problems = append(problems, fmt.Sprintf(
				"%s: no row in %s — add it in the same commit as the record", record.path, indexPath,
			))
		}
	}

	return problems
}

func checkRow(at string, row Row, record Record) []string {
	var problems []string
	differs := func(column, index, file string) {
		problems = append(problems, fmt.Sprintf(
			"%s: %s %s is %q here and %q in %s", at, row.ID, column, index, file, record.path,
		))
	}

	if want := filepath.Base(record.path); row.Link != want {
		differs("link", row.Link, want)
	}
	if row.Title != record.Title {
		differs("title", row.Title, record.Title)
	}
	if row.Domain != record.Domain {
		differs("domain", row.Domain, record.Domain)
	}
	if row.Status != record.Status {
		differs("status", row.Status, record.Status)
	}
	if row.Issue != "#"+record.Issue {
		differs("issue", row.Issue, "#"+record.Issue)
	}

	supersedes := strings.Join(row.Supersedes, ", ")
	fileSupersedes := record.Supersedes
	if fileSupersedes == none {
		fileSupersedes = ""
	}
	if supersedes != fileSupersedes {
		differs("supersedes", supersedes, fileSupersedes)
	}

	return problems
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
