package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const addonManifest = `schema: 1
requires:
  - name: git
add:
  from: template
setup:
  - echo ran > setup-ran.txt
next:
  - describe your project in AGENTS.md
`

func harnessFiles() map[string]string {
	return map[string]string{
		"pik.yaml":                      addonManifest,
		"README.md":                     "# Harness repo readme, not copied\n",
		"template/AGENTS.md":            "# Agent guide\n",
		"template/.agents/check.sh":     "#!/bin/sh\nexit 0\n",
		"template/.githooks/pre-commit": "#!/bin/sh\nexit 0\n",
		"template/docs/architecture.md": "# Architecture\n",
	}
}

func existingProject(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func gitInit(t *testing.T, dir string) {
	t.Helper()
	if output, err := exec.Command("git", "-C", dir, "init", "--quiet").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
}

func gitConfig(t *testing.T, dir, key string) string {
	t.Helper()
	output, _ := exec.Command("git", "-C", dir, "config", "--get", key).Output()
	return strings.TrimSpace(string(output))
}

func TestAddCopiesNewFilesAndKeepsExistingOnes(t *testing.T) {
	newFixture(t, templateFiles).serveAddon(t, harnessFiles())
	dir := existingProject(t, map[string]string{"AGENTS.md": "# My own guide\n"})

	code, stdout, stderr := pik(t, "add", "agent-harness", dir, "--no-setup")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if got := read(t, filepath.Join(dir, "AGENTS.md")); got != "# My own guide\n" {
		t.Fatalf("an existing file was overwritten: %q", got)
	}
	if got := read(t, filepath.Join(dir, "docs/architecture.md")); got != "# Architecture\n" {
		t.Fatalf("architecture.md = %q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "README.md")); err == nil {
		t.Fatal("copied a file from outside the add folder")
	}
	if _, err := os.Stat(filepath.Join(dir, "pik.yaml")); err == nil {
		t.Fatal("copied the add-on's manifest")
	}
	for _, want := range []string{"added  docs/architecture.md", "kept   AGENTS.md (already exists)", "Added agent-harness v0.1.0"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("output lacks %q:\n%s", want, stdout)
		}
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(filepath.Join(dir, ".agents/check.sh"))
		if err != nil || info.Mode().Perm()&0o100 == 0 {
			t.Fatalf("check.sh isn't executable: %v %v", info, err)
		}
	}
}

func TestAddRecordsTheAddonAndKeepsTheProjectRecord(t *testing.T) {
	newFixture(t, templateFiles).serveAddon(t, harnessFiles())
	dir := existingProject(t, map[string]string{
		".pik.yaml": "# Written by pik.\ntemplate: demo-starter\nversion: v1.0.0\naddons:\n  - name: other-addon\n    version: v2.0.0\n",
	})

	if code, _, stderr := pik(t, "add", "agent-harness", dir, "--no-setup"); code != 0 {
		t.Fatalf("pik add: %s", stderr)
	}
	record := read(t, filepath.Join(dir, ".pik.yaml"))
	for _, want := range []string{"# Written by pik.", "template: demo-starter", "name: other-addon", "name: agent-harness\n    repo: https://github.com/opikdev/agent-harness\n    version: v0.1.0\n    commit: deadbeef"} {
		if !strings.Contains(record, want) {
			t.Errorf(".pik.yaml lacks %q:\n%s", want, record)
		}
	}
}

func TestAddRefusesAnAddonThatIsAlreadyInstalled(t *testing.T) {
	newFixture(t, templateFiles).serveAddon(t, harnessFiles())
	original := "template: demo-starter\naddons:\n  - name: agent-harness\n    version: v0.0.1\n"
	dir := existingProject(t, map[string]string{".pik.yaml": original})

	code, _, stderr := pik(t, "add", "agent-harness", dir, "--no-setup")
	if code != 1 || !strings.Contains(stderr, "agent-harness is already installed at v0.0.1") {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	if got := read(t, filepath.Join(dir, ".pik.yaml")); got != original {
		t.Errorf(".pik.yaml changed:\n%s", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "AGENTS.md")); err == nil {
		t.Error("copied files for an add-on that was already installed")
	}
}

func TestAddStartsARecordInAProjectWithoutOne(t *testing.T) {
	newFixture(t, templateFiles).serveAddon(t, harnessFiles())
	dir := existingProject(t, map[string]string{"README.md": "# Mine\n"})
	if code, _, stderr := pik(t, "add", "agent-harness", dir, "--no-setup"); code != 0 {
		t.Fatalf("pik add: %s", stderr)
	}
	record := read(t, filepath.Join(dir, ".pik.yaml"))
	if !strings.HasPrefix(record, "# Written by pik.") || !strings.Contains(record, "addons:\n  - name: agent-harness") {
		t.Fatalf(".pik.yaml:\n%s", record)
	}
}

func TestAddDryRunWritesNothing(t *testing.T) {
	newFixture(t, templateFiles).serveAddon(t, harnessFiles())
	dir := existingProject(t, map[string]string{"AGENTS.md": "# Mine\n"})

	code, stdout, stderr := pik(t, "--json", "add", "agent-harness", dir, "--dry-run")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	var result struct {
		DryRun bool     `json:"dryRun"`
		Added  []string `json:"added"`
		Kept   []string `json:"kept"`
		Setup  struct {
			Ran bool `json:"ran"`
		} `json:"setup"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, stdout)
	}
	if !result.DryRun || result.Setup.Ran || len(result.Kept) != 1 || len(result.Added) != 3 {
		t.Fatalf("result = %+v", result)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("a dry run wrote files: %v", entries)
	}
}

func TestAddTurnsOnGitHooksOnlyWhenNoOtherPathIsSet(t *testing.T) {
	newFixture(t, templateFiles).serveAddon(t, harnessFiles())
	fresh := existingProject(t, nil)
	gitInit(t, fresh)
	custom := existingProject(t, nil)
	gitInit(t, custom)
	if output, err := exec.Command("git", "-C", custom, "config", "core.hooksPath", ".husky").CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, output)
	}

	_, stdout, _ := pik(t, "add", "agent-harness", fresh, "--no-setup")
	if gitConfig(t, fresh, "core.hooksPath") != ".githooks" || !strings.Contains(stdout, "Git hooks in .githooks are on") {
		t.Fatalf("hooks weren't turned on:\n%s", stdout)
	}
	_, stdout, _ = pik(t, "add", "agent-harness", custom, "--no-setup")
	if gitConfig(t, custom, "core.hooksPath") != ".husky" || !strings.Contains(stdout, "already set") {
		t.Fatalf("an existing hooks path was replaced:\n%s", stdout)
	}
}

func TestAddRunsSetupWithConsentOnly(t *testing.T) {
	newFixture(t, templateFiles).serveAddon(t, harnessFiles())
	asked := existingProject(t, nil)
	code, stdout, _ := pik(t, "add", "agent-harness", asked)
	if code != 0 {
		t.Fatal(stdout)
	}
	if _, err := os.Stat(filepath.Join(asked, "setup-ran.txt")); err == nil {
		t.Fatal("setup ran without consent")
	}
	if !strings.Contains(stdout, "Next:\n  echo ran > setup-ran.txt\n  describe your project in AGENTS.md") {
		t.Fatalf("next steps:\n%s", stdout)
	}

	consented := existingProject(t, nil)
	if code, _, stderr := pik(t, "add", "agent-harness", consented, "--yes"); code != 0 {
		t.Fatal(stderr)
	}
	if read(t, filepath.Join(consented, "setup-ran.txt")) == "" {
		t.Fatal("setup didn't run with --yes")
	}
}

func TestAddRefusesWhatItCantInstall(t *testing.T) {
	cases := map[string]struct {
		files map[string]string
		args  []string
		want  string
	}{
		"a starter":              {harnessFiles(), []string{"add", "demo-starter"}, "is a starter, not an add-on"},
		"an unknown name":        {harnessFiles(), []string{"add", "nope"}, `no add-on named "nope"`},
		"no name, no terminal":   {harnessFiles(), []string{"add"}, "name an add-on"},
		"a missing folder":       {harnessFiles(), []string{"add", "agent-harness", "/does/not/exist"}, "isn't a folder"},
		"a manifest without add": {map[string]string{"pik.yaml": "schema: 1\n", "template/x": "x"}, []string{"add", "agent-harness"}, "has no add section"},
		"add with prompts": {map[string]string{
			"pik.yaml": "schema: 1\nprompts:\n  - name: x\n    message: x\nadd:\n  from: template\n",
		}, []string{"add", "agent-harness"}, "can't use prompts"},
		"a missing add folder":   {map[string]string{"pik.yaml": "schema: 1\nadd:\n  from: nowhere\n"}, []string{"add", "agent-harness"}, "no nowhere folder"},
		"an escaping add folder": {map[string]string{"pik.yaml": "schema: 1\nadd:\n  from: ../up\n"}, []string{"add", "agent-harness"}, "must be relative"},
	}
	for label, c := range cases {
		t.Run(label, func(t *testing.T) {
			newFixture(t, templateFiles).serveAddon(t, c.files)
			args := c.args
			if len(args) == 2 {
				args = append(args, t.TempDir())
			}
			code, _, stderr := pik(t, args...)
			if code == 0 || !strings.Contains(stderr, c.want) {
				t.Fatalf("exit %d, stderr %q, want %q", code, stderr, c.want)
			}
		})
	}
}

func TestDoctorChecksAnAddonsTools(t *testing.T) {
	newFixture(t, templateFiles).serveAddon(t, harnessFiles())
	code, stdout, stderr := pik(t, "doctor", "agent-harness")
	if code != 0 || strings.Count(stdout, "git") != 1 {
		t.Fatalf("exit %d\n%s\n%s", code, stdout, stderr)
	}
}
