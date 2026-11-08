// Package project turns a starter archive into a new project.
package project

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/opikdev/pik/internal/archive"
	"github.com/opikdev/pik/internal/catalog"
	"github.com/opikdev/pik/internal/manifest"
)

// RecordFile is written into every generated project.
const RecordFile = ".pik.yaml"

// Record is what pik saves in .pik.yaml, so pik update can re-render the same template later.
type Record struct {
	Template string            `yaml:"template" json:"template"`
	Repo     string            `yaml:"repo" json:"repo"`
	Version  string            `yaml:"version" json:"version"`
	Commit   string            `yaml:"commit,omitempty" json:"commit,omitempty"`
	Answers  map[string]string `yaml:"answers" json:"answers"`
	Pik      string            `yaml:"pik" json:"pik"`
}

// Archives opens a starter's archive at a tag.
type Archives interface {
	Archive(ctx context.Context, owner, repo, ref string) (io.ReadCloser, error)
}

// Ask asks the user for a prompt's value, starting from its default.
type Ask func(prompt manifest.Prompt, initial string) (string, error)

// Options controls one generation.
type Options struct {
	Entry      catalog.Entry
	Dir        string
	Answers    map[string]string
	Ask        Ask
	PikVersion string
}

// Result describes the generated project.
type Result struct {
	Dir      string
	Record   Record
	Manifest *manifest.Manifest
}

// Generate downloads the pinned starter and writes the customized project to opts.Dir.
// It writes into a temporary folder first, so a failure never leaves a half-made project.
func Generate(ctx context.Context, archives Archives, opts Options) (*Result, error) {
	dir, err := filepath.Abs(opts.Dir)
	if err != nil {
		return nil, fmt.Errorf("resolve %s: %w", opts.Dir, err)
	}
	if err := checkTarget(dir); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return nil, fmt.Errorf("create %s: %w", filepath.Dir(dir), err)
	}
	staging, err := os.MkdirTemp(filepath.Dir(dir), ".pik-")
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

	values, err := resolve(spec, opts, filepath.Base(dir))
	if err != nil {
		return nil, err
	}
	if err := apply(staging, spec, values); err != nil {
		return nil, err
	}

	answers := maps.Clone(values)
	delete(answers, "dir")
	record := Record{
		Template: opts.Entry.Name,
		Repo:     opts.Entry.Repo,
		Version:  opts.Entry.Version,
		Commit:   commit,
		Answers:  answers,
		Pik:      opts.PikVersion,
	}
	if err := writeRecord(staging, record); err != nil {
		return nil, err
	}

	if err := os.Remove(dir); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("replace %s: %w", dir, err)
	}
	if err := os.Rename(staging, dir); err != nil {
		return nil, fmt.Errorf("move the project into %s: %w", dir, err)
	}
	return &Result{Dir: dir, Record: record, Manifest: spec}, nil
}

func checkTarget(dir string) error {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("%s exists and isn't a readable folder: %w", dir, err)
	}
	if len(entries) > 0 {
		return fmt.Errorf("%s isn't empty. Choose a new folder", dir)
	}
	return nil
}

func resolve(spec *manifest.Manifest, opts Options, dirName string) (map[string]string, error) {
	answers := maps.Clone(opts.Answers)
	if answers == nil {
		answers = map[string]string{}
	}
	if opts.Ask != nil {
		for _, prompt := range spec.Prompts {
			if _, ok := answers[prompt.Name]; ok {
				continue
			}
			value, err := opts.Ask(prompt, prompt.DefaultFor(dirName))
			if err != nil {
				return nil, err
			}
			answers[prompt.Name] = value
		}
	}
	return spec.Resolve(answers, dirName)
}

func apply(root string, spec *manifest.Manifest, values map[string]string) error {
	for _, replacement := range spec.Replace {
		to, complete := manifest.Render(replacement.To, values)
		if !complete {
			continue
		}
		for _, file := range replacement.Files {
			path := filepath.Join(root, file)
			content, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("%s lists %s, which doesn't exist in the starter: %w", manifest.FileName, file, err)
			}
			if !bytes.Contains(content, []byte(replacement.From)) {
				return fmt.Errorf("%s: %q isn't in %s, so the starter and its manifest disagree", manifest.FileName, replacement.From, file)
			}
			info, err := os.Stat(path)
			if err != nil {
				return fmt.Errorf("read %s: %w", file, err)
			}
			updated := strings.ReplaceAll(string(content), replacement.From, to)
			if err := os.WriteFile(path, []byte(updated), info.Mode().Perm()); err != nil {
				return fmt.Errorf("write %s: %w", file, err)
			}
		}
	}
	for _, file := range append([]string{manifest.FileName}, spec.Exclude...) {
		if err := os.RemoveAll(filepath.Join(root, file)); err != nil {
			return fmt.Errorf("remove %s: %w", file, err)
		}
	}
	return nil
}

func writeRecord(root string, record Record) error {
	var data bytes.Buffer
	data.WriteString("# Written by pik. Keep it, because pik update uses it to find the template and answers.\n")
	encoder := yaml.NewEncoder(&data)
	encoder.SetIndent(2)
	if err := encoder.Encode(record); err != nil {
		return fmt.Errorf("encode %s: %w", RecordFile, err)
	}
	if err := encoder.Close(); err != nil {
		return fmt.Errorf("encode %s: %w", RecordFile, err)
	}
	if err := os.WriteFile(filepath.Join(root, RecordFile), data.Bytes(), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", RecordFile, err)
	}
	return nil
}
