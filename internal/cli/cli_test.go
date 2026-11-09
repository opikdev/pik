package cli

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

const manifestFile = `schema: 1
requires:
  - name: sh
prompts:
  - name: name
    message: Project name
    pattern: ^[a-z][a-z0-9-]*$
    default: "{{dir}}"
  - name: owner
    message: GitHub owner
    optional: true
replace:
  - from: opikdev/demo-starter
    to: "{{owner}}/{{name}}"
    files: [README.md]
  - from: demo-starter
    to: "{{name}}"
    files: [package.json, README.md]
exclude: [.baseline.sha256, .github/CODEOWNERS]
setup:
  - echo ran > setup-ran.txt
next:
  - docker compose up
`

var templateFiles = map[string]string{
	"pik.yaml":             manifestFile,
	"README.md":            "# demo-starter\n\n[CI](https://github.com/opikdev/demo-starter/actions)\n",
	"package.json":         `{ "name": "demo-starter" }` + "\n",
	".baseline.sha256":     "abc  LICENSE\n",
	".github/CODEOWNERS":   "* @opikdev\n",
	".githooks/pre-commit": "#!/bin/sh\nexit 0\n",
}

func tarball(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	compressed := gzip.NewWriter(&buffer)
	writer := tar.NewWriter(compressed)
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	must(writer.WriteHeader(&tar.Header{Typeflag: tar.TypeXGlobalHeader, Name: "pax_global_header", PAXRecords: map[string]string{"comment": "deadbeef"}}))
	for name, body := range files {
		mode := int64(0o644)
		if strings.Contains(name, ".githooks/") || strings.HasSuffix(name, ".sh") {
			mode = 0o755
		}
		must(writer.WriteHeader(&tar.Header{Name: "demo-starter-1.0.0/" + name, Mode: mode, Size: int64(len(body)), Typeflag: tar.TypeReg}))
		_, err := writer.Write([]byte(body))
		must(err)
	}
	must(writer.Close())
	must(compressed.Close())
	return buffer.Bytes()
}

type fixture struct {
	server   *httptest.Server
	mux      *http.ServeMux
	archives int
}

// serveAddon publishes an agent-harness v0.1.0 archive with these files.
func (f *fixture) serveAddon(t *testing.T, files map[string]string) {
	t.Helper()
	archive := tarball(t, files)
	f.mux.HandleFunc("/archive/opikdev/agent-harness/tar.gz/refs/tags/v0.1.0", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(archive)
	})
	f.mux.HandleFunc("/raw/opikdev/agent-harness/v0.1.0/pik.yaml", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, files["pik.yaml"])
	})
}

func newFixture(t *testing.T, files map[string]string) *fixture {
	t.Helper()
	f := &fixture{}
	archive := tarball(t, files)
	catalogYAML := `starters:
  - name: demo-starter
    repo: https://github.com/opikdev/demo-starter
    version: v1.0.0
    type: starter
    description: A demo starter for tests
  - name: agent-harness
    repo: https://github.com/opikdev/agent-harness
    version: v0.1.0
    type: addon
    description: Harness
  - name: later-starter
    repo: https://github.com/opikdev/later-starter
    version: null
    type: starter
    description: Not released
`
	mux := http.NewServeMux()
	mux.HandleFunc("/catalog.yaml", func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, catalogYAML) })
	mux.HandleFunc("/raw/opikdev/demo-starter/v1.0.0/pik.yaml", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, files["pik.yaml"])
	})
	mux.HandleFunc("/archive/opikdev/demo-starter/tar.gz/refs/tags/v1.0.0", func(w http.ResponseWriter, _ *http.Request) {
		f.archives++
		_, _ = w.Write(archive)
	})
	f.mux = mux
	f.server = httptest.NewServer(mux)
	t.Cleanup(f.server.Close)
	t.Setenv("PIK_CATALOG", f.server.URL+"/catalog.yaml")
	t.Setenv("PIK_RAW_URL", f.server.URL+"/raw")
	t.Setenv("PIK_ARCHIVE_URL", f.server.URL+"/archive")
	return f
}

func pik(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	stdin, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = stdin.Close() }()
	var stdout, stderr bytes.Buffer
	code := run(&app{version: "0.1.0-test", stdout: &stdout, stderr: &stderr, stdin: stdin}, args)
	return code, stdout.String(), stderr.String()
}

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestNewCreatesACustomizedProject(t *testing.T) {
	newFixture(t, templateFiles)
	dir := filepath.Join(t.TempDir(), "my-api")

	code, stdout, stderr := pik(t, "new", "demo-starter", dir, "--set", "owner=acme", "--yes", "--json")
	if code != 0 {
		t.Fatalf("exit %d: %s %s", code, stdout, stderr)
	}
	var result struct {
		OK    bool
		Dir   string
		Git   bool
		Setup struct{ Ran bool }
		Pik   struct {
			Version string
			Commit  string
			Answers map[string]string
		}
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("invalid JSON %q: %v", stdout, err)
	}
	if !result.OK || !result.Git || !result.Setup.Ran || result.Pik.Commit != "deadbeef" {
		t.Fatalf("result = %+v", result)
	}

	if got := read(t, filepath.Join(dir, "README.md")); got != "# my-api\n\n[CI](https://github.com/acme/my-api/actions)\n" {
		t.Errorf("README.md = %q", got)
	}
	if got := read(t, filepath.Join(dir, "package.json")); !strings.Contains(got, `"name": "my-api"`) {
		t.Errorf("package.json = %q", got)
	}
	for _, removed := range []string{"pik.yaml", ".baseline.sha256", ".github/CODEOWNERS"} {
		if _, err := os.Stat(filepath.Join(dir, removed)); err == nil {
			t.Errorf("%s should be excluded", removed)
		}
	}
	if read(t, filepath.Join(dir, "setup-ran.txt")) != "ran\n" {
		t.Error("setup command didn't run")
	}

	var record struct {
		Template, Repo, Version, Commit, Pik string
		Answers                              map[string]string
	}
	if err := yaml.Unmarshal([]byte(read(t, filepath.Join(dir, ".pik.yaml"))), &record); err != nil {
		t.Fatal(err)
	}
	if record.Template != "demo-starter" || record.Version != "v1.0.0" || record.Commit != "deadbeef" ||
		record.Pik != "0.1.0-test" || record.Answers["name"] != "my-api" || record.Answers["owner"] != "acme" {
		t.Errorf(".pik.yaml = %+v", record)
	}
	if !strings.Contains(read(t, filepath.Join(dir, ".pik.yaml")), "\nanswers:\n  name: my-api\n") {
		t.Errorf(".pik.yaml should use 2-space indentation, like the starters' formatter")
	}
	if _, ok := record.Answers["dir"]; ok {
		t.Error(".pik.yaml should not record dir")
	}

	hooks, err := exec.Command("git", "-C", dir, "config", "core.hooksPath").Output()
	if err != nil || strings.TrimSpace(string(hooks)) != ".githooks" {
		t.Errorf("core.hooksPath = %q, %v", hooks, err)
	}
	if info, err := os.Stat(filepath.Join(dir, ".githooks/pre-commit")); err != nil || info.Mode().Perm()&0o100 == 0 {
		t.Errorf("pre-commit should stay executable: %v", err)
	}
}

func TestNewSkipsReplacementsWithMissingOptionalValues(t *testing.T) {
	newFixture(t, templateFiles)
	dir := filepath.Join(t.TempDir(), "plain")
	if code, _, stderr := pik(t, "new", "demo-starter", dir, "--yes", "--no-setup", "--no-git"); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if got := read(t, filepath.Join(dir, "README.md")); got != "# plain\n\n[CI](https://github.com/opikdev/plain/actions)\n" {
		t.Errorf("README.md = %q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
		t.Error("--no-git still created a repo")
	}
	if _, err := os.Stat(filepath.Join(dir, "setup-ran.txt")); err == nil {
		t.Error("--no-setup still ran setup")
	}
}

func TestNewNeedsConsentForSetup(t *testing.T) {
	newFixture(t, templateFiles)
	dir := filepath.Join(t.TempDir(), "careful")
	code, stdout, _ := pik(t, "new", "demo-starter", dir, "--json")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if _, err := os.Stat(filepath.Join(dir, "setup-ran.txt")); err == nil {
		t.Fatal("setup ran without --yes")
	}
	if !strings.Contains(stdout, `"ran": false`) || !strings.Contains(stdout, "echo ran > setup-ran.txt") {
		t.Errorf("JSON should list the skipped setup commands: %s", stdout)
	}
}

func TestNewRejects(t *testing.T) {
	newFixture(t, templateFiles)
	occupied := t.TempDir()
	if err := os.WriteFile(filepath.Join(occupied, "keep.txt"), []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		label string
		args  []string
		want  string
	}{
		{"no arguments without a terminal", []string{"new"}, "name a starter"},
		{"no folder without a terminal", []string{"new", "demo-starter"}, "name a folder"},
		{"an unknown starter", []string{"new", "nope", "x"}, "no starter named"},
		{"an add-on", []string{"new", "agent-harness", "x"}, "is an add-on"},
		{"an unreleased starter", []string{"new", "later-starter", "x"}, "no release yet"},
		{"a bad value", []string{"new", "demo-starter", "x", "--set", "name=Bad Name"}, "doesn't match"},
		{"an unknown prompt", []string{"new", "demo-starter", "x", "--set", "color=red"}, `no prompt named "color"`},
		{"a malformed --set", []string{"new", "demo-starter", "x", "--set", "name"}, "must be name=value"},
		{"a folder that isn't empty", []string{"new", "demo-starter", occupied}, "isn't empty"},
	}
	for _, tc := range cases {
		code, stdout, _ := pik(t, append(tc.args, "--json")...)
		var result struct {
			OK    bool
			Error string
		}
		if err := json.Unmarshal([]byte(stdout), &result); err != nil {
			t.Fatalf("%s: invalid JSON %q", tc.label, stdout)
		}
		if code != 1 || result.OK || !strings.Contains(result.Error, tc.want) {
			t.Errorf("%s: exit %d, error %q, want %q", tc.label, code, result.Error, tc.want)
		}
	}
	if read(t, filepath.Join(occupied, "keep.txt")) != "mine" {
		t.Error("an existing folder was changed")
	}
}

func emptyFolder(t *testing.T, name string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), name)
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestNewFillsTheCurrentFolderInPlace(t *testing.T) {
	newFixture(t, templateFiles)
	dir := emptyFolder(t, "my-app")
	before, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	if code, _, stderr := pik(t, "new", "demo-starter", ".", "--yes", "--no-setup", "--no-git"); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	after, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(before, after) {
		t.Fatal("the current folder was replaced instead of filled")
	}
	if got := read(t, filepath.Join(dir, "package.json")); !strings.Contains(got, `"name": "my-app"`) {
		t.Errorf("package.json = %q", got)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".pik-") {
			t.Errorf("left the staging folder %s behind", entry.Name())
		}
	}
}

func TestNewLeavesAnEmptyFolderEmptyOnFailure(t *testing.T) {
	broken := map[string]string{}
	for name, body := range templateFiles {
		broken[name] = body
	}
	broken["package.json"] = "{}\n"
	newFixture(t, broken)
	dir := emptyFolder(t, "app")
	if code, stdout, _ := pik(t, "new", "demo-starter", dir, "--yes", "--json"); code != 1 || !strings.Contains(stdout, "disagree") {
		t.Fatalf("exit %d, output %s", code, stdout)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("left behind %v, %v", entries, err)
	}
}

func TestNewLeavesNothingBehindOnFailure(t *testing.T) {
	broken := map[string]string{}
	for name, body := range templateFiles {
		broken[name] = body
	}
	broken["package.json"] = "{}\n"
	newFixture(t, broken)
	parent := t.TempDir()
	code, stdout, _ := pik(t, "new", "demo-starter", filepath.Join(parent, "app"), "--yes", "--json")
	if code != 1 || !strings.Contains(stdout, "disagree") {
		t.Fatalf("exit %d, output %s", code, stdout)
	}
	entries, err := os.ReadDir(parent)
	if err != nil || len(entries) != 0 {
		t.Fatalf("left behind %v, %v", entries, err)
	}
}

func TestList(t *testing.T) {
	newFixture(t, templateFiles)
	code, stdout, _ := pik(t, "list")
	if code != 0 || !strings.Contains(stdout, "demo-starter") || !strings.Contains(stdout, "unreleased") {
		t.Fatalf("exit %d, output %s", code, stdout)
	}
	code, stdout, _ = pik(t, "list", "--json")
	var result struct {
		OK       bool
		Starters []struct{ Name, Type string }
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil || code != 0 || len(result.Starters) != 3 {
		t.Fatalf("exit %d, %v, %s", code, err, stdout)
	}
}

func TestListFromLocalFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "catalog.yaml")
	if err := os.WriteFile(path, []byte("starters:\n  - name: x-starter\n    repo: https://github.com/a/x-starter\n    version: v1.0.0\n    type: starter\n    description: Local\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, stdout, _ := pik(t, "list", "--catalog", path); code != 0 || !strings.Contains(stdout, "x-starter") {
		t.Fatalf("exit %d, output %s", code, stdout)
	}
}

func TestDoctor(t *testing.T) {
	newFixture(t, templateFiles)
	code, stdout, _ := pik(t, "doctor", "demo-starter", "--json")
	if code != 0 || !strings.Contains(stdout, `"name": "sh"`) {
		t.Fatalf("exit %d, output %s", code, stdout)
	}
}

func TestDoctorFailsOnMissingTools(t *testing.T) {
	missing := map[string]string{}
	for name, body := range templateFiles {
		missing[name] = body
	}
	missing["pik.yaml"] = strings.Replace(manifestFile, "  - name: sh\n", "  - name: definitely-not-installed\n", 1)
	newFixture(t, missing)
	code, stdout, _ := pik(t, "doctor", "demo-starter")
	if code != 1 || !strings.Contains(stdout, "✗ definitely-not-installed: not installed") {
		t.Fatalf("exit %d, output %s", code, stdout)
	}
}
