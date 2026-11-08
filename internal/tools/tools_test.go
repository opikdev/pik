package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opikdev/pik/internal/manifest"
)

func fakeTool(t *testing.T, name, output string, exit int) {
	t.Helper()
	dir := t.TempDir()
	script := "#!/bin/sh\necho '" + output + "'\nexit " + string(rune('0'+exit)) + "\n"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
}

func TestCheck(t *testing.T) {
	cases := []struct {
		label   string
		output  string
		exit    int
		needs   string
		ok      bool
		problem string
	}{
		{"a new enough version", "v26.13.0", 0, "26.0.0", true, ""},
		{"a version with words", "Docker version 29.8.2, build abc", 0, "29", true, ""},
		{"an old version", "v20.1.0", 0, "26.0.0", false, "older than 26.0.0"},
		{"an equal two-part version", "go1.27", 0, "1.27", true, ""},
		{"no version needed", "anything", 0, "", true, ""},
		{"an unreadable version", "unknown", 0, "1.0.0", false, "couldn't be read"},
		{"a tool without --version, when no version is needed", "Illegal option --", 2, "", true, ""},
		{"a failing version command, when a version is needed", "Illegal option -- 1.2.3", 2, "1.0.0", false, "couldn't be read"},
	}
	for _, tc := range cases {
		fakeTool(t, "faketool", tc.output, tc.exit)
		result := Check(context.Background(), manifest.Requirement{Name: "faketool", Version: tc.needs})
		if result.OK != tc.ok || !strings.Contains(result.Problem, tc.problem) {
			t.Errorf("%s: got ok=%v problem=%q", tc.label, result.OK, result.Problem)
		}
	}
}

func TestCheckMissing(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	result := Check(context.Background(), manifest.Requirement{Name: "nothing-here"})
	if result.OK || result.Problem != "not installed" {
		t.Fatalf("result = %+v", result)
	}
}
