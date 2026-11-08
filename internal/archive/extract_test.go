package archive

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type entry struct {
	name     string
	body     string
	mode     int64
	typeflag byte
	link     string
}

func build(t *testing.T, commit string, entries ...entry) *bytes.Buffer {
	t.Helper()
	var buffer bytes.Buffer
	compressed := gzip.NewWriter(&buffer)
	writer := tar.NewWriter(compressed)
	if commit != "" {
		if err := writer.WriteHeader(&tar.Header{Typeflag: tar.TypeXGlobalHeader, Name: "pax_global_header", PAXRecords: map[string]string{"comment": commit}}); err != nil {
			t.Fatal(err)
		}
	}
	for _, item := range entries {
		header := &tar.Header{Name: item.name, Mode: item.mode, Typeflag: item.typeflag, Linkname: item.link, Size: int64(len(item.body))}
		if header.Typeflag == 0 {
			header.Typeflag = tar.TypeReg
		}
		if header.Mode == 0 {
			header.Mode = 0o644
		}
		if header.Typeflag != tar.TypeReg {
			header.Size = 0
		}
		if err := writer.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if header.Typeflag == tar.TypeReg {
			if _, err := writer.Write([]byte(item.body)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := compressed.Close(); err != nil {
		t.Fatal(err)
	}
	return &buffer
}

func TestExtract(t *testing.T) {
	dest := t.TempDir()
	archive := build(t, "abc123",
		entry{name: "demo-1.0.0/", typeflag: tar.TypeDir},
		entry{name: "demo-1.0.0/README.md", body: "# Demo"},
		entry{name: "demo-1.0.0/bin/run.sh", body: "echo", mode: 0o755},
		entry{name: "demo-1.0.0/.agents/skills/a/SKILL.md", body: "skill"},
		entry{name: "demo-1.0.0/.claude/skills/a", typeflag: tar.TypeSymlink, link: "../../.agents/skills/a"},
	)
	commit, err := Extract(archive, dest, DefaultLimits)
	if err != nil {
		t.Fatal(err)
	}
	if commit != "abc123" {
		t.Errorf("commit = %q, want abc123", commit)
	}
	if body, _ := os.ReadFile(filepath.Join(dest, "README.md")); string(body) != "# Demo" {
		t.Errorf("README.md = %q", body)
	}
	info, err := os.Stat(filepath.Join(dest, "bin/run.sh"))
	if err != nil || info.Mode().Perm()&0o100 == 0 {
		t.Errorf("run.sh mode = %v, %v, want executable", info, err)
	}
	if body, err := os.ReadFile(filepath.Join(dest, ".claude/skills/a/SKILL.md")); err != nil || string(body) != "skill" {
		t.Errorf("symlinked skill = %q, %v", body, err)
	}
}

func TestExtractCopiesLinksWhenAsked(t *testing.T) {
	dest := t.TempDir()
	archive := build(t, "",
		entry{name: "demo/.agents/skills/a/SKILL.md", body: "skill"},
		entry{name: "demo/docs/guide.md", body: "guide"},
		entry{name: "demo/.claude/skills/a", typeflag: tar.TypeSymlink, link: "../../.agents/skills/a"},
		entry{name: "demo/README.md", typeflag: tar.TypeSymlink, link: "docs/guide.md"},
	)
	limits := DefaultLimits
	limits.CopyLinks = true
	if _, err := Extract(archive, dest, limits); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{".claude/skills/a", "README.md"} {
		info, err := os.Lstat(filepath.Join(dest, name))
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			t.Errorf("%s should be a copy, not a link: %v %v", name, info, err)
		}
	}
	if body, _ := os.ReadFile(filepath.Join(dest, ".claude/skills/a/SKILL.md")); string(body) != "skill" {
		t.Errorf("copied skill = %q", body)
	}
	if body, _ := os.ReadFile(filepath.Join(dest, "README.md")); string(body) != "guide" {
		t.Errorf("copied file = %q", body)
	}
}

func TestExtractRejectsLinksToMissingTargetsWhenCopying(t *testing.T) {
	limits := DefaultLimits
	limits.CopyLinks = true
	archive := build(t, "", entry{name: "demo/link", typeflag: tar.TypeSymlink, link: "missing"})
	if _, err := Extract(archive, t.TempDir(), limits); err == nil || !strings.Contains(err.Error(), "isn't in the archive") {
		t.Fatalf("error = %v", err)
	}
}

func TestExtractRejects(t *testing.T) {
	cases := map[string]entry{
		"a parent path":       {name: "demo/../../evil", body: "x"},
		"an absolute path":    {name: "/etc/evil", body: "x"},
		"an escaping symlink": {name: "demo/link", typeflag: tar.TypeSymlink, link: "../../outside"},
		"an absolute symlink": {name: "demo/link", typeflag: tar.TypeSymlink, link: "/etc/passwd"},
		"a hard link":         {name: "demo/hard", typeflag: tar.TypeLink, link: "demo/README.md"},
		"a device":            {name: "demo/dev", typeflag: tar.TypeChar},
	}
	for label, item := range cases {
		dest := t.TempDir()
		if _, err := Extract(build(t, "", item), dest, DefaultLimits); err == nil {
			t.Errorf("%s: Extract succeeded, want an error", label)
		}
		if _, err := os.Stat(filepath.Join(filepath.Dir(dest), "evil")); err == nil {
			t.Fatalf("%s: wrote outside the destination", label)
		}
	}
}

func TestExtractLimits(t *testing.T) {
	big := entry{name: "demo/big", body: strings.Repeat("x", 100)}
	if _, err := Extract(build(t, "", big), t.TempDir(), Limits{Bytes: 99, Files: 10}); err == nil || !strings.Contains(err.Error(), "larger than") {
		t.Errorf("byte limit: error = %v", err)
	}
	many := []entry{{name: "demo/a", body: "1"}, {name: "demo/b", body: "2"}, {name: "demo/c", body: "3"}}
	if _, err := Extract(build(t, "", many...), t.TempDir(), Limits{Bytes: 1 << 20, Files: 2}); err == nil || !strings.Contains(err.Error(), "more than 2 files") {
		t.Errorf("file limit: error = %v", err)
	}
}

func TestExtractRejectsNonGzip(t *testing.T) {
	if _, err := Extract(strings.NewReader("not gzip"), t.TempDir(), DefaultLimits); err == nil {
		t.Fatal("Extract accepted plain text")
	}
}
