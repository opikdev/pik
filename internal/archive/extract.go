// Package archive extracts GitHub source tarballs safely.
package archive

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
)

// Limits caps what an archive may write.
type Limits struct {
	Bytes int64
	Files int
}

// DefaultLimits is generous for a starter and small enough to stop runaway archives.
var DefaultLimits = Limits{Bytes: 512 << 20, Files: 20_000}

// Extract writes a .tar.gz into dest, dropping the top-level folder GitHub adds.
// It returns the commit SHA that GitHub records in the archive, when present.
func Extract(source io.Reader, dest string, limits Limits) (string, error) {
	compressed, err := gzip.NewReader(source)
	if err != nil {
		return "", fmt.Errorf("read archive: %w", err)
	}
	defer func() { _ = compressed.Close() }()

	root, err := os.OpenRoot(dest)
	if err != nil {
		return "", fmt.Errorf("open %s: %w", dest, err)
	}
	defer func() { _ = root.Close() }()

	reader := tar.NewReader(compressed)
	var commit string
	var written int64
	files := 0
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return commit, nil
		}
		if err != nil {
			return "", fmt.Errorf("read archive: %w", err)
		}
		if header.Typeflag == tar.TypeXGlobalHeader {
			commit = header.PAXRecords["comment"]
			continue
		}

		name, err := stripTopLevel(header.Name)
		if err != nil {
			return "", err
		}
		if name == "" {
			continue
		}

		files++
		if files > limits.Files {
			return "", fmt.Errorf("archive has more than %d files", limits.Files)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := root.MkdirAll(name, 0o755); err != nil {
				return "", fmt.Errorf("create %s: %w", name, err)
			}
		case tar.TypeReg:
			written += header.Size
			if written > limits.Bytes {
				return "", fmt.Errorf("archive is larger than %d bytes", limits.Bytes)
			}
			if err := writeFile(root, name, header, reader); err != nil {
				return "", err
			}
		case tar.TypeSymlink:
			if err := checkLink(name, header.Linkname); err != nil {
				return "", err
			}
			if err := root.MkdirAll(path.Dir(name), 0o755); err != nil {
				return "", fmt.Errorf("create %s: %w", path.Dir(name), err)
			}
			if err := root.Symlink(header.Linkname, name); err != nil {
				return "", fmt.Errorf("link %s: %w", name, err)
			}
		default:
			return "", fmt.Errorf("archive entry %s has unsupported type %q", name, header.Typeflag)
		}
	}
}

func stripTopLevel(name string) (string, error) {
	if strings.HasPrefix(name, "/") || strings.Contains(name, "\\") {
		return "", fmt.Errorf("archive entry %q has an unsafe path", name)
	}
	_, rest, _ := strings.Cut(strings.TrimSuffix(name, "/"), "/")
	if rest == "" {
		return "", nil
	}
	clean := path.Clean(rest)
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("archive entry %q escapes the project folder", name)
	}
	return clean, nil
}

func checkLink(name, target string) error {
	resolved := path.Clean(path.Join(path.Dir(name), target))
	if path.IsAbs(target) || resolved == ".." || strings.HasPrefix(resolved, "../") {
		return fmt.Errorf("archive link %s points outside the project, to %q", name, target)
	}
	return nil
}

func writeFile(root *os.Root, name string, header *tar.Header, content io.Reader) error {
	if err := root.MkdirAll(path.Dir(name), 0o755); err != nil {
		return fmt.Errorf("create %s: %w", path.Dir(name), err)
	}
	mode := os.FileMode(0o644)
	if header.Mode&0o111 != 0 {
		mode = 0o755
	}
	file, err := root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return fmt.Errorf("write %s: %w", name, err)
	}
	if _, err := io.CopyN(file, content, header.Size); err != nil {
		_ = file.Close()
		return fmt.Errorf("write %s: %w", name, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("write %s: %w", name, err)
	}
	return nil
}
