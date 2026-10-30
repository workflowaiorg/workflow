package render

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"
)

// files returns what src generates for every assistant: Claude Code (.claude), GitHub Copilot
// (.github) and opencode (.opencode). Each reads only its own folder.
func (src source) files(header string) ([]File, error) {
	where := src.where()
	doc, err := merge(where, src.base, src.project)
	if err != nil {
		return nil, err
	}

	switch src.kind {
	case instructions:
		data := doc.bytes(header)
		return []File{
			{Path: "AGENTS.md", Data: data},
			{Path: ".claude/CLAUDE.md", Data: data},
			{Path: ".github/copilot-instructions.md", Data: data},
		}, nil

	case workflowFile:
		return []File{{Path: ".workflow/" + src.name + ".md", Data: doc.bytes(header)}}, nil

	case skill:
		for _, key := range []string{"name", "description"} {
			_, ok, err := doc.field(where, key)
			if err != nil {
				return nil, err
			}
			if !ok {
				return nil, fmt.Errorf("%s: a skill's frontmatter needs %q", where, key)
			}
		}
		if description, _, _ := doc.field(where, "description"); strings.TrimSpace(description) == "" {
			return nil, fmt.Errorf("%s: a skill's description is empty, so no assistant knows when to use it", where)
		}
		// Assistants find a skill by its folder, and refuse or misname one whose name differs.
		if name, _, _ := doc.field(where, "name"); name != src.name {
			return nil, fmt.Errorf("%s: the skill's name is %q, but its folder is %q", where, name, src.name)
		}
		data := doc.bytes(header)
		return []File{
			{Path: ".claude/skills/" + src.name + "/SKILL.md", Data: data},
			{Path: ".github/skills/" + src.name + "/SKILL.md", Data: data},
			{Path: ".opencode/skills/" + src.name + "/SKILL.md", Data: data},
		}, nil

	case command:
		data := doc.bytes(header)
		return []File{
			{Path: ".claude/commands/" + src.name + ".md", Data: data},
			{Path: ".github/prompts/" + src.name + ".prompt.md", Data: data},
			{Path: ".opencode/commands/" + src.name + ".md", Data: data},
		}, nil

	case subagent:
		return src.subagentFiles(where, doc, header)
	}
	return nil, fmt.Errorf("%s: unknown kind %d", where, src.kind)
}

// subagentFiles writes a subagent for each assistant. Its source holds only a description; the
// rest of the frontmatter is written here, once per assistant, because each one draws the line
// around writing differently.
//
// A subagent reports and never edits. It still needs to read whatever a change touches — the diff,
// the history behind a line, an Issue, a library's docs — so it keeps the shell and the web tools.
// Editing and writing are denied where an assistant can deny them; where one cannot, the prompt
// itself carries the rule.
func (src source) subagentFiles(where string, doc document, header string) ([]File, error) {
	mapping, err := doc.fields(where)
	if err != nil {
		return nil, err
	}
	description, ok, _ := doc.field(where, "description")
	if !ok || strings.TrimSpace(description) == "" {
		return nil, fmt.Errorf("%s: a subagent's frontmatter needs a description", where)
	}
	for i := 0; i < len(mapping.Content); i += 2 {
		if key := mapping.Content[i].Value; key != "description" {
			return nil, fmt.Errorf("%s: a subagent's frontmatter holds only a description, not %q", where, key)
		}
	}
	// A double-quoted YAML scalar: a description holding ": " or opening with #, [, {, * or a
	// quote would otherwise give frontmatter no assistant can parse, and none of them says so.
	quoted := `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", " ").Replace(strings.TrimSpace(description)) + `"`

	claude := "name: " + src.name + "\n" +
		"description: " + quoted + "\n" +
		"tools: Read, Grep, Glob, Bash, WebSearch, WebFetch\n" +
		"model: inherit\n"
	opencode := "description: " + quoted + "\n" +
		"mode: subagent\n" +
		"permission:\n" +
		"  edit: deny\n" +
		"  bash:\n" +
		"    \"*\": allow\n" +
		"    \"git commit*\": deny\n" +
		"    \"git push*\": deny\n" +
		"    \"git add*\": deny\n"
	copilot := "name: " + src.name + "\n" +
		"description: " + quoted + "\n" +
		// Copilot ignores tool names it does not know, so only its own aliases are listed.
		`tools: ["read", "search", "execute", "web"]` + "\n"

	return []File{
		{Path: ".claude/agents/" + src.name + ".md", Data: document{frontmatter: claude, body: doc.body}.bytes(header)},
		{Path: ".github/agents/" + src.name + ".agent.md", Data: document{frontmatter: copilot, body: doc.body}.bytes(header)},
		{Path: ".opencode/agents/" + src.name + ".md", Data: document{frontmatter: opencode, body: doc.body}.bytes(header)},
	}, nil
}

// generatable says whether path is one wf could have generated. .wf.lock is trusted only that far,
// since wf removes the files it lists.
func generatable(file string) bool {
	if path.Clean(file) != file || strings.Contains(file, `\`) || !filepath.IsLocal(filepath.FromSlash(file)) {
		return false
	}
	for _, pattern := range []string{
		"AGENTS.md", ".claude/CLAUDE.md", ".github/copilot-instructions.md",
		".workflow/GUIDE.md", ".workflow/SPEC-TEMPLATE.md", ".workflow/DECISION-TEMPLATE.md",
		".claude/skills/*/SKILL.md", ".github/skills/*/SKILL.md", ".opencode/skills/*/SKILL.md",
		".claude/commands/*.md", ".github/prompts/*.prompt.md", ".opencode/commands/*.md",
		".claude/agents/*.md", ".github/agents/*.agent.md", ".opencode/agents/*.md",
	} {
		if matched, _ := path.Match(pattern, file); matched {
			return true
		}
	}
	return false
}

// where names src's source files in errors.
func (src source) where() string {
	switch src.kind {
	case instructions:
		return ".agents/{base,project}/" + src.name + ".md"
	case workflowFile:
		return ".agents/{base,project}/workflow/" + src.name + ".md"
	case skill:
		return ".agents/{base,project}/skills/" + src.name + "/SKILL.md"
	case command:
		return ".agents/{base,project}/commands/" + src.name + ".md"
	default:
		return ".agents/{base,project}/subagents/" + src.name + ".md"
	}
}
