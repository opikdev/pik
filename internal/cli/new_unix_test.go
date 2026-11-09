//go:build unix

package cli

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestNewGivesTheProjectFolderTheUsualPermissions(t *testing.T) {
	newFixture(t, templateFiles)
	for umask, want := range map[int]os.FileMode{0o022: 0o755, 0o027: 0o750} {
		previous := syscall.Umask(umask)
		dir := filepath.Join(t.TempDir(), "app")
		code, _, stderr := pik(t, "new", "demo-starter", dir, "--yes", "--no-setup", "--no-git")
		syscall.Umask(previous)
		if code != 0 {
			t.Fatalf("exit %d: %s", code, stderr)
		}
		info, err := os.Stat(dir)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != want {
			t.Errorf("umask %o: project folder mode = %o, want %o", umask, got, want)
		}
	}
}
