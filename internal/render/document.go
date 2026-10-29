package render

import (
	"bytes"
	"fmt"
	"strings"

	"go.yaml.in/yaml/v3"
)

// projectHeading opens the project part of a generated file that has both a base and a project part.
const projectHeading = "## This project\n\nProject rules below override the base rules above."

// document is a Markdown file split into its YAML frontmatter and its body.
type document struct {
	// frontmatter is the text between the --- lines, kept as written so a file nothing overrides
	// comes out unchanged. Empty when the file has none.
	frontmatter string
	body        string
}

// parseDocument splits data into frontmatter and body. path is only for the error.
func parseDocument(path string, data []byte) (document, error) {
	text := string(data)
	rest, found := strings.CutPrefix(text, "---\n")
	if !found {
		return document{body: text}, nil
	}
	// Empty frontmatter: the closing --- comes straight after the opening one.
	if body, found := strings.CutPrefix(rest, "---\n"); found || rest == "---" {
		return document{body: body}, nil
	}
	frontmatter, body, found := strings.Cut(rest, "\n---\n")
	if !found {
		frontmatter, found = strings.CutSuffix(rest, "\n---")
		if !found {
			return document{}, fmt.Errorf("%s: frontmatter opened with --- is never closed", path)
		}
	}
	doc := document{frontmatter: frontmatter + "\n", body: body}
	if _, err := doc.fields(path); err != nil {
		return document{}, err
	}
	return doc, nil
}

// fields returns the frontmatter as a YAML mapping, or nil when there is none.
func (doc document) fields(path string) (*yaml.Node, error) {
	if doc.frontmatter == "" {
		return nil, nil
	}
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(doc.frontmatter), &root); err != nil {
		return nil, fmt.Errorf("%s: reading frontmatter: %w", path, err)
	}
	if len(root.Content) == 0 {
		return nil, nil
	}
	mapping := root.Content[0]
	if mapping.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%s: frontmatter is not a set of key: value pairs", path)
	}
	// YAML keeps the last of two equal keys without a word, so one of them would be lost unseen.
	for i := 0; i < len(mapping.Content); i += 2 {
		for j := i + 2; j < len(mapping.Content); j += 2 {
			if mapping.Content[i].Value == mapping.Content[j].Value {
				return nil, fmt.Errorf("%s: frontmatter has %q twice", path, mapping.Content[i].Value)
			}
		}
	}
	return mapping, nil
}

// field returns the string value of key in the frontmatter, and whether it is there.
func (doc document) field(path, key string) (string, bool, error) {
	mapping, err := doc.fields(path)
	if err != nil || mapping == nil {
		return "", false, err
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1].Value, true, nil
		}
	}
	return "", false, nil
}

// merge joins a base and a project part: the project's frontmatter keys replace the base's, and
// its body follows the base's under projectHeading. Either part may be missing, never both.
func merge(path string, base, project *document) (document, error) {
	switch {
	case base == nil:
		return *project, nil
	case project == nil:
		return *base, nil
	}

	frontmatter := base.frontmatter
	if project.frontmatter != "" {
		var err error
		if frontmatter, err = mergeFrontmatter(path, *base, *project); err != nil {
			return document{}, err
		}
	}
	body := base.body
	if strings.TrimSpace(project.body) != "" {
		// Only blank lines are trimmed, so a part opening with an indented code block keeps it.
		rules := strings.TrimRight(strings.TrimLeft(project.body, "\n"), " \t\n")
		body = strings.TrimRight(base.body, "\n") + "\n\n" + projectHeading + "\n\n" + rules + "\n"
	}
	return document{frontmatter: frontmatter, body: body}, nil
}

func mergeFrontmatter(path string, base, project document) (string, error) {
	merged, err := base.fields(path)
	if err != nil {
		return "", err
	}
	overrides, err := project.fields(path)
	if err != nil {
		return "", err
	}
	if overrides == nil {
		// Frontmatter holding only comments overrides nothing.
		return base.frontmatter, nil
	}
	if merged == nil {
		merged = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	}
	for i := 0; i+1 < len(overrides.Content); i += 2 {
		key, value := overrides.Content[i], overrides.Content[i+1]
		replaced := false
		for j := 0; j+1 < len(merged.Content); j += 2 {
			if merged.Content[j].Value == key.Value {
				merged.Content[j+1] = value
				replaced = true
			}
		}
		if !replaced {
			merged.Content = append(merged.Content, key, value)
		}
	}

	var out bytes.Buffer
	encoder := yaml.NewEncoder(&out)
	encoder.SetIndent(2)
	if err := encoder.Encode(merged); err != nil {
		return "", fmt.Errorf("%s: writing frontmatter: %w", path, err)
	}
	if err := encoder.Close(); err != nil {
		return "", fmt.Errorf("%s: writing frontmatter: %w", path, err)
	}
	return out.String(), nil
}

// bytes writes doc as a generated file: frontmatter, then header, then the body.
func (doc document) bytes(header string) []byte {
	var out strings.Builder
	if doc.frontmatter != "" {
		out.WriteString("---\n" + doc.frontmatter + "---\n")
	}
	out.WriteString(header + "\n\n")
	out.WriteString(strings.TrimLeft(doc.body, "\n"))
	if !strings.HasSuffix(doc.body, "\n") {
		out.WriteString("\n")
	}
	return []byte(out.String())
}
