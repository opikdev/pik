// Package source downloads the catalog, manifests, and starter archives.
package source

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// Default locations, overridable through environment variables for tests and mirrors.
const (
	DefaultCatalog = "https://raw.githubusercontent.com/opikdev/starters/main/catalog.yaml"
	defaultRaw     = "https://raw.githubusercontent.com"
	defaultArchive = "https://codeload.github.com"
)

// Source fetches files from GitHub, or from the overrides it was given.
type Source struct {
	client      *http.Client
	catalog     string
	rawBase     string
	archiveBase string
}

// FromEnv builds a Source, using PIK_CATALOG, PIK_RAW_URL, and PIK_ARCHIVE_URL when set.
func FromEnv(catalog string) *Source {
	if catalog == "" {
		catalog = envOr("PIK_CATALOG", DefaultCatalog)
	}
	return &Source{
		client:      &http.Client{Timeout: 2 * time.Minute},
		catalog:     catalog,
		rawBase:     strings.TrimSuffix(envOr("PIK_RAW_URL", defaultRaw), "/"),
		archiveBase: strings.TrimSuffix(envOr("PIK_ARCHIVE_URL", defaultArchive), "/"),
	}
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

// Catalog returns catalog.yaml from a URL or a local file path.
func (s *Source) Catalog(ctx context.Context) ([]byte, error) {
	if !strings.HasPrefix(s.catalog, "http://") && !strings.HasPrefix(s.catalog, "https://") {
		data, err := os.ReadFile(s.catalog)
		if err != nil {
			return nil, fmt.Errorf("read catalog: %w", err)
		}
		return data, nil
	}
	body, err := s.get(ctx, s.catalog)
	if err != nil {
		return nil, err
	}
	defer func() { _ = body.Close() }()
	return readAll(body, s.catalog)
}

// Manifest returns a starter's pik.yaml at a tag.
func (s *Source) Manifest(ctx context.Context, owner, repo, ref string) ([]byte, error) {
	address := fmt.Sprintf("%s/%s/%s/%s/pik.yaml", s.rawBase, owner, repo, ref)
	body, err := s.get(ctx, address)
	if err != nil {
		return nil, err
	}
	defer func() { _ = body.Close() }()
	return readAll(body, address)
}

// Archive opens the .tar.gz of a starter at a tag. The caller closes it.
func (s *Source) Archive(ctx context.Context, owner, repo, ref string) (io.ReadCloser, error) {
	return s.get(ctx, fmt.Sprintf("%s/%s/%s/tar.gz/refs/tags/%s", s.archiveBase, owner, repo, ref))
}

func (s *Source) get(ctx context.Context, address string) (io.ReadCloser, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, fmt.Errorf("build request for %s: %w", address, err)
	}
	request.Header.Set("User-Agent", "pik")
	response, err := s.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", address, err)
	}
	if response.StatusCode != http.StatusOK {
		_ = response.Body.Close()
		return nil, fmt.Errorf("download %s: HTTP %d", address, response.StatusCode)
	}
	return response.Body, nil
}

func readAll(body io.Reader, address string) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", address, err)
	}
	return data, nil
}
