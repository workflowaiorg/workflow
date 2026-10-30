package specs

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Spec is one .workflow/specs/<domain>/SPEC.md and the records it cites.
type Spec struct {
	path      string // relative to the repository root
	citations []Citation
	bare      []Citation // an ADR-NNNN in the prose with no link on it
}

// Citation is one `[ADR-NNNN](../../decisions/NNNN-<slug>.md)` in a spec. The link
// text is what the domain-specs skill fixes, so it is what this greps for.
type Citation struct {
	ID     string
	Target string
	line   int
}

var (
	citationLink = regexp.MustCompile(`\[ADR-(\d{4})\]\(([^)\s]+)\)`)
	bareMention  = regexp.MustCompile(`ADR-(\d{4})`)
)

// LoadSpecs reads every spec's citations.
func LoadSpecs(root string) ([]Spec, error) {
	paths, err := filepath.Glob(filepath.Join(root, specDir, "*", "SPEC.md"))
	if err != nil {
		return nil, fmt.Errorf("finding the specs: %w", err)
	}

	var specs []Spec
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading a spec: %w", err)
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil, fmt.Errorf("reading a spec: %w", err)
		}

		spec := Spec{path: filepath.ToSlash(rel)}
		for n, text := range strings.Split(string(raw), "\n") {
			for _, match := range citationLink.FindAllStringSubmatch(text, -1) {
				spec.citations = append(spec.citations, Citation{ID: match[1], Target: match[2], line: n + 1})
			}
			for _, match := range bareMention.FindAllStringSubmatch(citationLink.ReplaceAllString(text, ""), -1) {
				spec.bare = append(spec.bare, Citation{ID: match[1], line: n + 1})
			}
		}
		specs = append(specs, spec)
	}

	return specs, nil
}

// checkCitations is the check the rest exists for: every record a spec cites is
// there, is the record the link text names, and has not been superseded. A spec
// citing a superseded record describes behaviour that has since changed.
func checkCitations(root string, specs []Spec, records []Record) []string {
	var problems []string

	byPath := map[string]Record{}
	for _, record := range records {
		byPath[record.path] = record
	}

	for _, spec := range specs {
		for _, bare := range spec.bare {
			problems = append(problems, fmt.Sprintf(
				"%s:%d: ADR-%s is mentioned without a link — write it as [ADR-%s](../../decisions/%s-<slug>.md)",
				spec.path, bare.line, bare.ID, bare.ID, bare.ID,
			))
		}

		for _, citation := range spec.citations {
			at := fmt.Sprintf("%s:%d", spec.path, citation.line)
			target, _, _ := strings.Cut(citation.Target, "#")
			resolved := filepath.ToSlash(filepath.Clean(filepath.Join(filepath.Dir(spec.path), target)))

			record, ok := byPath[resolved]
			switch {
			case !ok:
				problems = append(problems, fmt.Sprintf(
					"%s: [ADR-%s] links to %s, which is not a record", at, citation.ID, resolved,
				))
			case record.ID != citation.ID:
				problems = append(problems, fmt.Sprintf(
					"%s: [ADR-%s] links to %s, which is ADR-%s", at, citation.ID, resolved, record.ID,
				))
			case record.Status == "superseded":
				problems = append(problems, fmt.Sprintf(
					"%s: ADR-%s is superseded by ADR-%s — the behaviour changed, so the spec follows it and cites %s",
					at, citation.ID, record.SupersededBy, record.SupersededBy,
				))
			}
		}
	}

	return problems
}

// checkCited checks the other direction: every accepted record is cited by the
// spec its own `spec:` names. A record no spec cites is reasoning nobody reaching
// the behaviour will find. Process-level decisions say `spec: none` and are exempt.
func checkCited(specs []Spec, records []Record) []string {
	var problems []string

	cited := map[string]map[string]bool{} // spec path → record ids it cites
	for _, spec := range specs {
		cited[spec.path] = map[string]bool{}
		for _, citation := range spec.citations {
			cited[spec.path][citation.ID] = true
		}
	}

	for _, record := range records {
		if record.Status != "accepted" || record.Spec == "none" || record.Spec == "" {
			continue
		}
		citations, ok := cited[record.Spec]
		if ok && !citations[record.ID] {
			problems = append(problems, fmt.Sprintf(
				"%s: ADR-%s is cited nowhere in %s — link it at the claim it supports, or say `spec: none`",
				record.where("spec"), record.ID, record.Spec,
			))
		}
	}

	return problems
}
