package manifest

import (
	"errors"
	"strings"
	"testing"
)

const sample = `schema: 1
prompts:
  - name: name
    message: Project name
    pattern: ^[a-z][a-z0-9-]*$
    default: "{{dir}}"
  - name: owner
    message: GitHub owner
    optional: true
replace:
  - from: opikdev/demo
    to: "{{owner}}/{{name}}"
    files: [README.md]
  - from: demo
    to: "{{name}}"
    files: [package.json]
exclude: [.baseline.sha256]
setup: [echo hi]
check: [echo ok]
`

func parse(t *testing.T) *Manifest {
	t.Helper()
	spec, err := Parse([]byte(sample))
	if err != nil {
		t.Fatal(err)
	}
	return spec
}

func TestResolveUsesDefaults(t *testing.T) {
	values, err := parse(t).Resolve(nil, "my-api")
	if err != nil {
		t.Fatal(err)
	}
	if values["name"] != "my-api" || values["owner"] != "" || values["dir"] != "my-api" {
		t.Fatalf("values = %v", values)
	}
}

func TestResolveRejects(t *testing.T) {
	cases := []struct {
		label   string
		answers map[string]string
		dir     string
		want    string
	}{
		{"a bad pattern", map[string]string{"name": "My App"}, "x", "doesn't match"},
		{"an unknown prompt", map[string]string{"color": "red"}, "x", `no prompt named "color"`},
		{"an invalid default", nil, "My Folder", "doesn't match"},
	}
	for _, tc := range cases {
		if _, err := parse(t).Resolve(tc.answers, tc.dir); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: error = %v, want %q", tc.label, err, tc.want)
		}
	}
}

func TestResolveRequiresValues(t *testing.T) {
	spec := parse(t)
	_, err := spec.Resolve(map[string]string{"name": ""}, "x")
	var missing *MissingError
	if !errors.As(err, &missing) || !strings.Contains(err.Error(), "--set name=") {
		t.Fatalf("error = %v, want a MissingError naming --set name=", err)
	}
}

func TestRender(t *testing.T) {
	values := map[string]string{"name": "my-api", "owner": ""}
	if got, ok := Render("{{name}}", values); !ok || got != "my-api" {
		t.Fatalf("Render = %q, %v", got, ok)
	}
	if _, ok := Render("{{owner}}/{{name}}", values); ok {
		t.Fatal("Render reported complete with an empty owner")
	}
}

func TestParseRejects(t *testing.T) {
	cases := map[string]string{
		"an unknown field":       "schema: 1\ncolour: red\n",
		"a future schema":        "schema: 2\n",
		"a duplicate prompt":     "schema: 1\nprompts:\n  - name: a\n  - name: a\n",
		"a prompt named dir":     "schema: 1\nprompts:\n  - name: dir\n",
		"a bad pattern":          "schema: 1\nprompts:\n  - name: a\n    pattern: '['\n",
		"an unknown placeholder": "schema: 1\nreplace:\n  - from: x\n    to: '{{nope}}'\n    files: [a]\n",
		"an escaping file":       "schema: 1\nreplace:\n  - from: x\n    to: y\n    files: [../a]\n",
		"an absolute exclude":    "schema: 1\nexclude: [/etc/passwd]\n",
		"a replacement no files": "schema: 1\nreplace:\n  - from: x\n    to: y\n",
	}
	for label, input := range cases {
		if _, err := Parse([]byte(input)); err == nil {
			t.Errorf("%s: Parse succeeded, want an error", label)
		}
	}
}
