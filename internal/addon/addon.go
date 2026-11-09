// Package addon copies an add-on's files into an existing project.
package addon

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v3"

	"github.com/opikdev/pik/internal/archive"
	"github.com/opikdev/pik/internal/catalog"
	"github.com/opikdev/pik/internal/manifest"
)

// RecordFile is where pik records the template and add-ons a project uses.
const RecordFile = ".pik.yaml"

// Archives opens an add-on's archive at a tag.
type Archives interface {
	Archive(ctx context.Context, owner, repo, ref string) (io.ReadCloser, error)
}

// Options controls one pik add.
type Options struct {
	Entry  catalog.Entry
	Dir    string
	DryRun bool
}

// Record is one add-on entry in .pik.yaml.
type Record struct {
	Name    string `yaml:"name" json:"name"`
	Repo    string `yaml:"repo" json:"repo"`
	Version string `yaml:"version" json:"version"`
	Commit  string `yaml:"commit,omitempty" json:"commit,omitempty"`
}

// Result says what pik add did, or would do with DryRun.
type Result struct {
	Dir      string
	Record   Record
	Added    []string
	Kept     []string
	Manifest *manifest.Manifest
}

// Add copies the add-on's files into opts.Dir. It never overwrites a file that already exists.
func Add(ctx context.Context, archives Archives, opts Options) (*Result, error) {
	dir, err := filepath.Abs(opts.Dir)
	if err != nil {
		return nil, fmt.Errorf("resolve %s: %w", opts.Dir, err)
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return nil, fmt.Errorf("%s isn't a folder. Run pik add inside your project, or pass its path", dir)
	}
	if version, installed, err := installedVersion(dir, opts.Entry.Name); err != nil {
		return nil, err
	} else if installed {
		return nil, fmt.Errorf("%s is already installed at %s, and pik add never changes an installed add-on. To add it again, remove it from addons in %s", opts.Entry.Name, version, RecordFile)
	}

	staging, err := os.MkdirTemp("", "pik-add-")
	if err != nil {
		return nil, fmt.Errorf("create a temporary folder: %w", err)
	}
	defer func() { _ = os.RemoveAll(staging) }()

	owner, repo, err := opts.Entry.GitHubRepo()
	if err != nil {
		return nil, err
	}
	body, err := archives.Archive(ctx, owner, repo, opts.Entry.Version)
	if err != nil {
		return nil, err
	}
	commit, err := archive.Extract(body, staging, archive.DefaultLimits)
	_ = body.Close()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(filepath.Join(staging, manifest.FileName))
	if err != nil {
		return nil, fmt.Errorf("%s %s has no %s: %w", opts.Entry.Name, opts.Entry.Version, manifest.FileName, err)
	}
	spec, err := manifest.Parse(data)
	if err != nil {
		return nil, err
	}
	if spec.Add == nil {
		return nil, fmt.Errorf("%s %s has no add section in %s, so pik add can't install it", opts.Entry.Name, opts.Entry.Version, manifest.FileName)
	}

	source := filepath.Join(staging, spec.Add.From)
	result := &Result{
		Dir:      dir,
		Record:   Record{Name: opts.Entry.Name, Repo: opts.Entry.Repo, Version: opts.Entry.Version, Commit: commit},
		Manifest: spec,
	}
	if err := copyMissing(source, dir, opts.DryRun, result); err != nil {
		return nil, err
	}
	if !opts.DryRun {
		if err := writeRecord(dir, result.Record); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func copyMissing(source, dir string, dryRun bool, result *Result) error {
	if info, err := os.Stat(source); err != nil || !info.IsDir() {
		return fmt.Errorf("the add-on has no %s folder to copy", filepath.Base(source))
	}
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || entry.Name() == ".DS_Store" {
			return nil
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dir, rel)
		name := filepath.ToSlash(rel)
		if _, err := os.Lstat(target); err == nil {
			result.Kept = append(result.Kept, name)
			return nil
		} else if !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("check %s: %w", name, err)
		}
		result.Added = append(result.Added, name)
		if dryRun {
			return nil
		}
		return copyEntry(path, target, entry)
	})
}

func copyEntry(path, target string, entry fs.DirEntry) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(target), err)
	}
	if entry.Type()&fs.ModeSymlink != 0 {
		link, err := os.Readlink(path)
		if err != nil {
			return fmt.Errorf("read link %s: %w", path, err)
		}
		return os.Symlink(link, target)
	}
	info, err := entry.Info()
	if err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if err := os.WriteFile(target, data, info.Mode().Perm()); err != nil {
		return fmt.Errorf("write %s: %w", target, err)
	}
	return nil
}

func installedVersion(dir, name string) (string, bool, error) {
	data, err := os.ReadFile(filepath.Join(dir, RecordFile))
	if errors.Is(err, fs.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("read %s: %w", RecordFile, err)
	}
	var record struct {
		Addons []Record `yaml:"addons"`
	}
	if err := yaml.Unmarshal(data, &record); err != nil {
		return "", false, fmt.Errorf("parse %s: %w", RecordFile, err)
	}
	for _, addon := range record.Addons {
		if addon.Name == name {
			return addon.Version, true, nil
		}
	}
	return "", false, nil
}

const recordHeader = "# Written by pik. Keep it, because pik update uses it to find the template and answers.\n"

// writeRecord adds the add-on under addons in .pik.yaml, keeping everything else.
func writeRecord(dir string, record Record) error {
	path := filepath.Join(dir, RecordFile)
	var document yaml.Node
	data, err := os.ReadFile(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		document = yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.MappingNode}}}
	case err != nil:
		return fmt.Errorf("read %s: %w", RecordFile, err)
	default:
		if err := yaml.Unmarshal(data, &document); err != nil {
			return fmt.Errorf("parse %s: %w", RecordFile, err)
		}
		if len(document.Content) == 0 || document.Content[0].Kind != yaml.MappingNode {
			return fmt.Errorf("%s isn't a mapping, so pik can't record the add-on there", RecordFile)
		}
	}

	var entry yaml.Node
	if err := entry.Encode(record); err != nil {
		return fmt.Errorf("encode %s: %w", RecordFile, err)
	}
	root := document.Content[0]
	addons := valueOf(root, "addons")
	if addons == nil {
		addons = &yaml.Node{Kind: yaml.SequenceNode}
		root.Content = append(root.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: "addons"}, addons)
	}
	addons.Content = append(addons.Content, &entry)

	var out bytes.Buffer
	if len(data) == 0 {
		out.WriteString(recordHeader)
	}
	encoder := yaml.NewEncoder(&out)
	encoder.SetIndent(2)
	if err := encoder.Encode(&document); err != nil {
		return fmt.Errorf("encode %s: %w", RecordFile, err)
	}
	if err := encoder.Close(); err != nil {
		return fmt.Errorf("encode %s: %w", RecordFile, err)
	}
	if err := os.WriteFile(path, out.Bytes(), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", RecordFile, err)
	}
	return nil
}

func valueOf(mapping *yaml.Node, key string) *yaml.Node {
	if mapping.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1]
		}
	}
	return nil
}
