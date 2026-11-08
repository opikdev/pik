package catalog

import (
	"strings"
	"testing"
)

const sample = `starters:
  - name: agent-harness
    repo: https://github.com/opikdev/agent-harness
    version: v0.1.0
    type: addon
    description: Harness
  - name: api-hono-starter
    repo: https://github.com/opikdev/api-hono-starter
    version: v0.1.1
    type: starter
    description: Hono API
  - name: api-go-chi-starter
    repo: https://github.com/opikdev/api-go-chi-starter
    version: null
    type: starter
    description: Go API
`

func TestStarter(t *testing.T) {
	parsed, err := Parse([]byte(sample))
	if err != nil {
		t.Fatal(err)
	}
	entry, err := parsed.Starter("api-hono-starter")
	if err != nil {
		t.Fatal(err)
	}
	if entry.Version != "v0.1.1" {
		t.Fatalf("version = %q, want v0.1.1", entry.Version)
	}
	owner, repo, err := entry.GitHubRepo()
	if err != nil || owner != "opikdev" || repo != "api-hono-starter" {
		t.Fatalf("GitHubRepo() = %q, %q, %v", owner, repo, err)
	}
}

func TestStarterRejects(t *testing.T) {
	parsed, err := Parse([]byte(sample))
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		"nope":               "no starter named",
		"agent-harness":      "is an add-on",
		"api-go-chi-starter": "no release yet",
	}
	for name, want := range cases {
		if _, err := parsed.Starter(name); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("Starter(%q) error = %v, want %q", name, err, want)
		}
	}
}

func TestParseRejects(t *testing.T) {
	cases := map[string]string{
		"bad yaml":     "starters: [",
		"missing repo": "starters:\n  - name: x\n    type: starter\n",
		"unknown type": "starters:\n  - name: x\n    repo: https://github.com/a/x\n    type: thing\n",
		"not github":   "starters:\n  - name: x\n    repo: https://gitlab.com/a/x\n    type: starter\n",
		"no repo name": "starters:\n  - name: x\n    repo: https://github.com/a\n    type: starter\n",
	}
	for label, input := range cases {
		if _, err := Parse([]byte(input)); err == nil {
			t.Errorf("%s: Parse succeeded, want an error", label)
		}
	}
}

func TestNames(t *testing.T) {
	parsed, err := Parse([]byte(sample))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(parsed.Names(), ","); got != "agent-harness,api-hono-starter,api-go-chi-starter" {
		t.Fatalf("Names() = %s", got)
	}
}
