// Package catalog parses the starter catalog that pik reads from opikdev/starters.
package catalog

import (
	"fmt"
	"net/url"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Entry is one starter or add-on in the catalog.
type Entry struct {
	Name        string `yaml:"name" json:"name"`
	Repo        string `yaml:"repo" json:"repo"`
	Version     string `yaml:"version" json:"version"`
	Type        string `yaml:"type" json:"type"`
	Description string `yaml:"description" json:"description"`
}

// Catalog is the parsed catalog.yaml.
type Catalog struct {
	Starters []Entry `yaml:"starters"`
}

// Parse decodes and validates catalog.yaml content.
func Parse(data []byte) (*Catalog, error) {
	var parsed Catalog
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("parse catalog: %w", err)
	}
	for _, entry := range parsed.Starters {
		if entry.Name == "" || entry.Repo == "" {
			return nil, fmt.Errorf("catalog entry %q is missing a name or repo", entry.Name)
		}
		if entry.Type != "starter" && entry.Type != "addon" {
			return nil, fmt.Errorf("catalog entry %s has unknown type %q", entry.Name, entry.Type)
		}
		if _, _, err := entry.GitHubRepo(); err != nil {
			return nil, err
		}
	}
	return &parsed, nil
}

// Names lists every entry name, in catalog order.
func (c *Catalog) Names() []string {
	names := make([]string, 0, len(c.Starters))
	for _, entry := range c.Starters {
		names = append(names, entry.Name)
	}
	return names
}

// Starter returns a released starter by name, ready for pik new.
func (c *Catalog) Starter(name string) (Entry, error) {
	index := slices.IndexFunc(c.Starters, func(entry Entry) bool { return entry.Name == name })
	if index == -1 {
		return Entry{}, fmt.Errorf("no starter named %q. Run pik list to see the catalog", name)
	}
	entry := c.Starters[index]
	if entry.Type != "starter" {
		return Entry{}, fmt.Errorf("%s is an add-on, not a starter, so pik new can't create a project from it", name)
	}
	if entry.Version == "" {
		return Entry{}, fmt.Errorf("%s has no release yet, so there's no version to pin", name)
	}
	return entry, nil
}

// GitHubRepo splits the repo URL into owner and repository name.
func (e Entry) GitHubRepo() (owner, repo string, err error) {
	parsed, err := url.Parse(e.Repo)
	if err != nil || parsed.Scheme != "https" || parsed.Host != "github.com" {
		return "", "", fmt.Errorf("catalog entry %s: repo %q must be https://github.com/<owner>/<repo>", e.Name, e.Repo)
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("catalog entry %s: repo %q must be https://github.com/<owner>/<repo>", e.Name, e.Repo)
	}
	return parts[0], parts[1], nil
}
