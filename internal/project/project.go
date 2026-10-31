// Package project is what wf knows of the project it works on: where its files are, its settings
// in .wf.yml, and how wf tells the developer's mistakes from its own faults.
package project

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
)

// The files and directories of a project that wf reads and writes, relative to its root.
const (
	ConfigFile = ".wf.yml"
	LockFile   = ".wf.lock"
	SumFile    = ".wf.sum"
	BaseDir    = ".agents/base"
	ProjectDir = ".agents/project"
)

// Forges are the forges wf has a base for.
var Forges = []string{"gitlab"}

// UserError is the developer's mistake rather than wf's fault: a setting that is wrong, or
// something they named that is not there. wf exits 2 on one.
type UserError struct {
	message string
	err     error
}

func (err *UserError) Error() string { return err.message }

func (err *UserError) Unwrap() error { return err.err }

// UserErrorf returns a UserError saying what format and args say.
func UserErrorf(format string, args ...any) error {
	return &UserError{message: fmt.Sprintf(format, args...)}
}

// Config is a project's .wf.yml.
type Config struct {
	// WF is the version of wf the project pins.
	WF    string
	Forge string
	// Worktree is what wf worktree copies into a new worktree.
	Worktree Worktree
	// Gate holds the project's own steps for /pre-mr-check.
	Gate []GateStep
	// Components holds the components wf add installed, by name, each with the answers it was
	// given, by option.
	Components map[string]map[string]any
	// Values is the whole of .wf.yml as written: the data wf's base files are rendered with.
	Values map[string]any
}

// GateStep is one step of /pre-mr-check's gate: the command run when a changed path matches.
type GateStep struct {
	Name  string
	Paths []string
	Run   string
}

// Worktree holds the gitignored files a new worktree needs.
type Worktree struct {
	// Copy holds globs, from the root, of the gitignored files a new worktree needs.
	Copy []string
	// Skip holds globs of what Copy matches but must not be copied, such as "*.example". Each is
	// matched against the path from the root and against the file's name.
	Skip []string
}

// Load reads and checks the .wf.yml of the project at dir. Every mistake in it comes back as one
// UserError naming them all, so none is found only later, by the command that happens to read it.
// A project with no .wf.yml gets a UserError that is also fs.ErrNotExist.
func Load(dir string) (Config, error) {
	content, err := ReadText(filepath.Join(dir, ConfigFile))
	if errors.Is(err, fs.ErrNotExist) {
		return Config{}, &UserError{fmt.Sprintf("no %s in %s: run wf in the project's root", ConfigFile, dir), err}
	}
	if err != nil {
		return Config{}, fmt.Errorf("reading %s: %w", ConfigFile, err)
	}
	values := map[string]any{}
	if err := yaml.Unmarshal(content, &values); err != nil {
		return Config{}, UserErrorf("reading %s: %v", ConfigFile, err)
	}

	// The pin and the forge are read as written: as a number, wf: 1.10 would be 1.1.
	var named struct {
		WF    string `yaml:"wf"`
		Forge string `yaml:"forge"`
	}
	if err := yaml.Unmarshal(content, &named); err != nil {
		return Config{}, UserErrorf("reading %s: %v", ConfigFile, err)
	}

	config := Config{WF: named.WF, Values: values}
	var problems []string
	if named.WF == "" {
		problems = append(problems, "pins no wf version; add wf: and the version wf version prints, such as wf: 0.1.0")
	}
	switch {
	case named.Forge == "":
		problems = append(problems, fmt.Sprintf("names no forge; add forge: %s", strings.Join(Forges, " or ")))
	case !slices.Contains(Forges, named.Forge):
		problems = append(problems, fmt.Sprintf("forge: %s is not one wf has a base for; it knows %s", named.Forge, strings.Join(Forges, ", ")))
	default:
		config.Forge = named.Forge
	}
	gate, gateProblems := readGate(values["gate"])
	config.Gate = gate
	problems = append(problems, gateProblems...)
	components, componentProblems := readComponents(values["components"])
	config.Components = components
	problems = append(problems, componentProblems...)
	worktree, worktreeProblems := readWorktree(values["worktree"])
	config.Worktree = worktree
	problems = append(problems, worktreeProblems...)
	if len(problems) > 0 {
		return Config{}, UserErrorf("%s: %s", ConfigFile, strings.Join(problems, "; "))
	}
	return config, nil
}

// readGate reads gate:, the project's own steps for /pre-mr-check, saying what is wrong with it:
// a list of entries, each with a name, a list of paths and the command to run.
func readGate(value any) ([]GateStep, []string) {
	if value == nil {
		return nil, nil
	}
	steps, ok := value.([]any)
	if !ok {
		return nil, []string{"gate: is a list of steps, each with name:, paths: and run:"}
	}
	var gate []GateStep
	var problems []string
	for i, step := range steps {
		fields, _ := step.(map[string]any)
		name, _ := fields["name"].(string)
		where := fmt.Sprintf("gate[%d]", i)
		if name != "" {
			where += " (" + name + ")"
		}
		var missing []string
		if name == "" {
			missing = append(missing, "name: the step's name")
		}
		paths, ok := stringList(fields["paths"])
		if !ok || len(paths) == 0 {
			missing = append(missing, `paths: a list of globs, such as ["**/*.go"]`)
		}
		run, _ := fields["run"].(string)
		if run == "" {
			missing = append(missing, "run: the command")
		}
		if len(missing) > 0 {
			problems = append(problems, where+" needs "+strings.Join(missing, ", "))
			continue
		}
		gate = append(gate, GateStep{Name: name, Paths: paths, Run: run})
	}
	return gate, problems
}

// readComponents reads components:, which wf add writes: each component's name, holding its
// answers by option. Whether wf knows each name is for the caller, which knows the components.
func readComponents(value any) (map[string]map[string]any, []string) {
	if value == nil {
		return nil, nil
	}
	entries, ok := value.(map[string]any)
	if !ok {
		return nil, []string{"components: holds each installed component's name, with its answers under it"}
	}
	components := map[string]map[string]any{}
	var problems []string
	for name, answers := range entries {
		switch answers := answers.(type) {
		case nil:
			components[name] = map[string]any{}
		case map[string]any:
			components[name] = answers
		default:
			problems = append(problems, fmt.Sprintf("components.%s holds the component's answers, each as option: answer", name))
		}
	}
	slices.Sort(problems)
	return components, problems
}

// readWorktree reads worktree:, saying what is wrong with it.
func readWorktree(value any) (Worktree, []string) {
	if value == nil {
		return Worktree{}, nil
	}
	fields, ok := value.(map[string]any)
	if !ok {
		return Worktree{}, []string{"worktree: holds copy: and skip:, each a list of globs"}
	}
	var worktree Worktree
	var problems []string
	for _, key := range []string{"copy", "skip"} {
		if fields[key] == nil {
			continue
		}
		globs, ok := stringList(fields[key])
		if !ok {
			problems = append(problems, fmt.Sprintf(`worktree.%s is a list of globs, such as [".env*"]`, key))
			continue
		}
		for _, glob := range globs {
			if _, err := path.Match(glob, ""); err != nil {
				problems = append(problems, fmt.Sprintf("worktree.%s has a bad pattern %q", key, glob))
			}
		}
		if key == "copy" {
			worktree.Copy = globs
		} else {
			worktree.Skip = globs
		}
	}
	return worktree, problems
}

// stringList returns value as a list of strings, and whether it is one with none of them empty.
func stringList(value any) ([]string, bool) {
	items, ok := value.([]any)
	if !ok {
		return nil, false
	}
	list := make([]string, 0, len(items))
	for _, item := range items {
		text, ok := item.(string)
		if !ok || text == "" {
			return nil, false
		}
		list = append(list, text)
	}
	return list, true
}

// WithComponent returns the .wf.yml of the project at dir with the component name recorded as
// installed with answers, and writes nothing: wf add checks the edit before it writes anything else,
// and writes .wf.yml last. The developer's settings and comments stay; blank lines between keys and
// their own indentation do not, as the file is written back whole.
func WithComponent(dir, name string, answers map[string]any) ([]byte, error) {
	before, err := Load(dir)
	if err != nil {
		return nil, err
	}
	content, err := ReadText(filepath.Join(dir, ConfigFile))
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", ConfigFile, err)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(content, &doc); err != nil {
		return nil, UserErrorf("reading %s: %v", ConfigFile, err)
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, UserErrorf("%s holds no settings", ConfigFile)
	}
	root := doc.Content[0]

	var entry yaml.Node
	if err := entry.Encode(answers); err != nil {
		return nil, fmt.Errorf("writing %s: %w", ConfigFile, err)
	}
	components := mappingValue(root, "components")
	switch {
	case components == nil:
		components = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		root.Content = append(root.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: "components"}, components)
	case components.Kind == yaml.ScalarNode:
		// components: left empty.
		*components = yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	}
	if components.Kind != yaml.MappingNode {
		return nil, notInFull()
	}
	if existing := mappingValue(components, name); existing != nil {
		*existing = entry
	} else {
		components.Content = append(components.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: name}, &entry)
	}

	var out bytes.Buffer
	encoder := yaml.NewEncoder(&out)
	encoder.SetIndent(2)
	if err := encoder.Encode(&doc); err != nil {
		return nil, fmt.Errorf("writing %s: %w", ConfigFile, err)
	}
	if err := encoder.Close(); err != nil {
		return nil, fmt.Errorf("writing %s: %w", ConfigFile, err)
	}

	// An alias or a merge key can hand components: down from elsewhere in the file, where the edit
	// above would replace it rather than add to it: what was installed must still be there.
	var after map[string]any
	if err := yaml.Unmarshal(out.Bytes(), &after); err != nil {
		return nil, fmt.Errorf("writing %s: %w", ConfigFile, err)
	}
	recorded, problems := readComponents(after["components"])
	want := maps.Clone(before.Components)
	if want == nil {
		want = map[string]map[string]any{}
	}
	want[name] = answers
	if len(problems) > 0 || !reflect.DeepEqual(recorded, want) {
		return nil, notInFull()
	}
	return out.Bytes(), nil
}

func notInFull() error {
	return UserErrorf("%s: wf add can only add to components: written out in full; replace its alias or merge key (<<:) with the components themselves", ConfigFile)
}

// mappingValue returns the value under key in mapping, or nil when it has none.
func mappingValue(mapping *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1]
		}
	}
	return nil
}

// ReadText reads a text file as wf compares and renders it: Windows line endings become \n and a
// byte order mark is dropped, so a checkout with core.autocrlf, or a file saved by a Windows
// editor, reads the same as any other.
func ReadText(file string) ([]byte, error) {
	content, err := os.ReadFile(file)
	content = bytes.TrimPrefix(content, []byte("\xef\xbb\xbf"))
	return bytes.ReplaceAll(content, []byte("\r\n"), []byte("\n")), err
}
