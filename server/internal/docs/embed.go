// Package docs exposes the generated in-app documentation bundle embedded in
// the API server binary.
package docs

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"path"
	"strings"
)

const contentRoot = "content"

//go:embed content
var contentFS embed.FS

var embeddedBundle = mustLoadBundle(contentFS)

type manifestData struct {
	Title         string          `json:"title"`
	Groups        []manifestGroup `json:"groups"`
	Assets        []string        `json:"assets"`
	ServerVersion string          `json:"serverVersion"`
}

type manifestGroup struct {
	Label string         `json:"label"`
	Items []manifestItem `json:"items"`
}

type manifestItem struct {
	Slug        string `json:"slug"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
}

type pageData struct {
	Slug        string    `json:"slug"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Body        string    `json:"body"`
	TOC         []tocItem `json:"toc"`
}

type tocItem struct {
	Depth int    `json:"depth"`
	Title string `json:"title"`
	ID    string `json:"id"`
}

// Resource is an immutable embedded document resource.
type Resource struct {
	content []byte
	etag    string
}

// Content returns an isolated copy of the resource bytes.
func (r Resource) Content() []byte { return bytes.Clone(r.content) }

// ETag returns a stable, content-derived HTTP entity tag.
func (r Resource) ETag() string { return r.etag }

type bundle struct {
	manifest manifestData
	pages    map[string]Resource
	assets   map[string]Resource
}

// Manifest returns the navigation manifest with the running server version.
func Manifest(serverVersion string) Resource {
	manifest := embeddedBundle.manifest
	manifest.ServerVersion = serverVersion
	data, err := json.Marshal(manifest)
	if err != nil {
		panic(fmt.Sprintf("docs: marshal validated manifest: %v", err))
	}
	return newResource(data)
}

// Page returns a page only when slug appears in the embedded manifest.
func Page(slug string) (Resource, bool) {
	resource, ok := embeddedBundle.pages[slug]
	return resource, ok
}

// Asset returns an asset only when its exact path appears in the manifest.
func Asset(assetPath string) (Resource, bool) {
	resource, ok := embeddedBundle.assets[assetPath]
	return resource, ok
}

func mustLoadBundle(fsys fs.FS) *bundle {
	loaded, err := loadBundle(fsys)
	if err != nil {
		panic(fmt.Sprintf("docs: invalid embedded bundle: %v", err))
	}
	return loaded
}

func loadBundle(fsys fs.FS) (*bundle, error) {
	manifestBytes, err := fs.ReadFile(fsys, path.Join(contentRoot, "manifest.json"))
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	var manifest manifestData
	if err := decodeJSON(manifestBytes, &manifest); err != nil {
		return nil, fmt.Errorf("decode manifest: %w", err)
	}
	if strings.TrimSpace(manifest.Title) == "" || len(manifest.Groups) == 0 {
		return nil, fmt.Errorf("manifest requires a title and groups")
	}

	loaded := &bundle{manifest: manifest, pages: make(map[string]Resource), assets: make(map[string]Resource)}
	for _, group := range manifest.Groups {
		if strings.TrimSpace(group.Label) == "" {
			return nil, fmt.Errorf("manifest group requires a label")
		}
		for _, item := range group.Items {
			if err := loaded.loadPage(fsys, item); err != nil {
				return nil, err
			}
		}
	}
	for _, assetPath := range manifest.Assets {
		if !validManifestPath(assetPath) {
			return nil, fmt.Errorf("invalid asset path %q", assetPath)
		}
		if _, exists := loaded.assets[assetPath]; exists {
			return nil, fmt.Errorf("duplicate asset path %q", assetPath)
		}
		data, err := fs.ReadFile(fsys, path.Join(contentRoot, "assets", assetPath))
		if err != nil {
			return nil, fmt.Errorf("read asset %q: %w", assetPath, err)
		}
		loaded.assets[assetPath] = newResource(data)
	}
	return loaded, nil
}

func (b *bundle) loadPage(fsys fs.FS, item manifestItem) error {
	if !validManifestPath(item.Slug) || strings.TrimSpace(item.Title) == "" {
		return fmt.Errorf("invalid page entry %q", item.Slug)
	}
	if _, exists := b.pages[item.Slug]; exists {
		return fmt.Errorf("duplicate page slug %q", item.Slug)
	}
	data, err := fs.ReadFile(fsys, path.Join(contentRoot, "pages", item.Slug+".json"))
	if err != nil {
		return fmt.Errorf("read page %q: %w", item.Slug, err)
	}
	var page pageData
	if err := decodeJSON(data, &page); err != nil {
		return fmt.Errorf("decode page %q: %w", item.Slug, err)
	}
	if page.Slug != item.Slug || strings.TrimSpace(page.Title) == "" {
		return fmt.Errorf("page %q has invalid identity", item.Slug)
	}
	b.pages[item.Slug] = newResource(data)
	return nil
}

func validManifestPath(value string) bool {
	return value != "." && value == strings.TrimSpace(value) && fs.ValidPath(value) && !strings.Contains(value, `\`)
}

func decodeJSON(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("unexpected data after JSON document")
	}
	return nil
}

func newResource(data []byte) Resource {
	sum := sha256.Sum256(data)
	return Resource{content: data, etag: fmt.Sprintf(`"%x"`, sum)}
}
