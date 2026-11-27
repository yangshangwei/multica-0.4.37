package materialrepo

import (
	"embed"
	"io/fs"
	"path"
	"sort"
	"strings"

	"github.com/multica-ai/multica/server/internal/material"
)

//go:embed all:assets
var assets embed.FS

// BuildCatalog creates only the twenty base files. It never creates product
// versions, material records, module examples or empty directory placeholders.
func BuildCatalog(binding Binding) (Catalog, error) {
	if err := validateBinding(binding); err != nil {
		return Catalog{}, err
	}
	out := Catalog{Binding: binding, SchemaVersion: SchemaVersion, BundleVersion: BundleVersion, Files: []File{}}
	err := fs.WalkDir(assets, "assets", func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		body, err := assets.ReadFile(name)
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(name, "assets/")
		if rel == "material.yaml" {
			body = []byte(strings.ReplaceAll(string(body), "{{SYSTEM_ID}}", binding.SystemID))
		}
		nameInRepository := path.Join(binding.RootPath, rel)
		if !validPath(nameInRepository, false) {
			return invalid("invalid_binding", nameInRepository)
		}
		out.Files = append(out.Files, fingerprint(nameInRepository, "100644", string(body)))
		return nil
	})
	if err != nil {
		return Catalog{}, err
	}
	sort.Slice(out.Files, func(i, j int) bool { return out.Files[i].Path < out.Files[j].Path })
	out.Digest = material.JSONDigest(struct {
		Binding Binding
		Schema  int
		Bundle  string
		Files   []Entry
	}{binding, SchemaVersion, BundleVersion, fileEntries(out.Files)})
	return out, nil
}
func fileEntries(files []File) []Entry {
	out := make([]Entry, 0, len(files))
	for _, f := range files {
		out = append(out, Entry{Path: f.Path, Mode: f.Mode, SHA256: f.SHA256})
	}
	return out
}
