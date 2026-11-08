// Package manifest parses pik.yaml, which each starter uses to describe how pik customizes it.
package manifest

import (
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
)

// FileName is where a starter or add-on keeps its manifest.
const FileName = "pik.yaml"

// Manifest describes how pik turns a starter into a new project, or adds an add-on to one.
type Manifest struct {
	Schema   int           `yaml:"schema"`
	Requires []Requirement `yaml:"requires"`
	Prompts  []Prompt      `yaml:"prompts"`
	Replace  []Replacement `yaml:"replace"`
	Exclude  []string      `yaml:"exclude"`
	Add      *AddSpec      `yaml:"add"`
	Setup    []string      `yaml:"setup"`
	Next     []string      `yaml:"next"`
	Check    []string      `yaml:"check"`
}

// AddSpec is the part of an add-on's manifest that pik add uses.
type AddSpec struct {
	// From is the folder in the add-on whose files pik add copies into a project.
	From string `yaml:"from"`
}

// Requirement is a tool the project needs, with an optional minimum version.
type Requirement struct {
	Name        string   `yaml:"name" json:"name"`
	Version     string   `yaml:"version" json:"version,omitempty"`
	VersionArgs []string `yaml:"versionArgs" json:"-"`
}

// Prompt is a value pik asks for, or takes from --set.
type Prompt struct {
	Name     string `yaml:"name"`
	Message  string `yaml:"message"`
	Pattern  string `yaml:"pattern"`
	Default  string `yaml:"default"`
	Optional bool   `yaml:"optional"`
}

// Replacement swaps text in listed files, using {{prompt}} values.
type Replacement struct {
	From  string   `yaml:"from"`
	To    string   `yaml:"to"`
	Files []string `yaml:"files"`
}

var (
	promptName  = regexp.MustCompile(`^[a-z][a-zA-Z0-9]*$`)
	placeholder = regexp.MustCompile(`\{\{([a-zA-Z0-9]+)\}\}`)
)

// Parse decodes and validates pik.yaml content.
func Parse(data []byte) (*Manifest, error) {
	var parsed Manifest
	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	decoder.KnownFields(true)
	if err := decoder.Decode(&parsed); err != nil {
		return nil, fmt.Errorf("parse %s: %w", FileName, err)
	}
	if parsed.Schema != 1 {
		return nil, fmt.Errorf("%s schema %d isn't supported. Update pik", FileName, parsed.Schema)
	}

	names := map[string]bool{"dir": true}
	for _, prompt := range parsed.Prompts {
		if !promptName.MatchString(prompt.Name) || names[prompt.Name] {
			return nil, fmt.Errorf("%s: prompt name %q must be camelCase, unique, and not dir", FileName, prompt.Name)
		}
		names[prompt.Name] = true
		if _, err := regexp.Compile(prompt.Pattern); err != nil {
			return nil, fmt.Errorf("%s: prompt %s has an invalid pattern: %w", FileName, prompt.Name, err)
		}
	}

	for _, replacement := range parsed.Replace {
		if replacement.From == "" || len(replacement.Files) == 0 {
			return nil, fmt.Errorf("%s: each replacement needs from and files", FileName)
		}
		for _, match := range placeholder.FindAllStringSubmatch(replacement.To, -1) {
			if !names[match[1]] {
				return nil, fmt.Errorf("%s: replacement uses unknown prompt {{%s}}", FileName, match[1])
			}
		}
		for _, file := range replacement.Files {
			if err := checkRelative(file); err != nil {
				return nil, err
			}
		}
	}
	for _, file := range parsed.Exclude {
		if err := checkRelative(file); err != nil {
			return nil, err
		}
	}
	if parsed.Add != nil {
		if len(parsed.Prompts) > 0 || len(parsed.Replace) > 0 || len(parsed.Exclude) > 0 {
			return nil, fmt.Errorf("%s: an add-on can't use prompts, replace, or exclude", FileName)
		}
		if err := checkRelative(parsed.Add.From); err != nil {
			return nil, err
		}
	}
	return &parsed, nil
}

func checkRelative(path string) error {
	clean := filepath.Clean(path)
	if filepath.IsAbs(path) || clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return fmt.Errorf("%s: path %q must be relative and inside the project", FileName, path)
	}
	return nil
}

// MissingError reports a required prompt with no value.
type MissingError struct {
	Prompt Prompt
}

func (e *MissingError) Error() string {
	return fmt.Sprintf("%s is required. Pass --set %s=<value>", e.Prompt.Name, e.Prompt.Name)
}

// Default returns a prompt's default, with {{dir}} replaced by the target folder name.
func (p Prompt) DefaultFor(dir string) string {
	return strings.ReplaceAll(p.Default, "{{dir}}", dir)
}

// Validate checks a value against the prompt's pattern.
func (p Prompt) Validate(value string) error {
	if value == "" {
		if p.Optional {
			return nil
		}
		return &MissingError{Prompt: p}
	}
	if p.Pattern != "" && !regexp.MustCompile(p.Pattern).MatchString(value) {
		return fmt.Errorf("%s %q doesn't match %s", p.Name, value, p.Pattern)
	}
	return nil
}

// Resolve combines answers with defaults and validates every prompt.
func (m *Manifest) Resolve(answers map[string]string, dir string) (map[string]string, error) {
	for name := range answers {
		if !slices.ContainsFunc(m.Prompts, func(prompt Prompt) bool { return prompt.Name == name }) {
			return nil, fmt.Errorf("this starter has no prompt named %q", name)
		}
	}
	values := map[string]string{"dir": dir}
	for _, prompt := range m.Prompts {
		value, ok := answers[prompt.Name]
		if !ok {
			value = prompt.DefaultFor(dir)
		}
		if err := prompt.Validate(value); err != nil {
			return nil, err
		}
		values[prompt.Name] = value
	}
	return values, nil
}

// Render fills {{name}} placeholders. It reports false when a placeholder has no value.
func Render(template string, values map[string]string) (string, bool) {
	complete := true
	rendered := placeholder.ReplaceAllStringFunc(template, func(match string) string {
		value := values[placeholder.FindStringSubmatch(match)[1]]
		if value == "" {
			complete = false
		}
		return value
	})
	return rendered, complete
}
