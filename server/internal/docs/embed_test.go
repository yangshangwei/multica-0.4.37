package docs

import (
	"encoding/json"
	"fmt"
	"testing"
	"testing/fstest"
)

func TestEmbeddedBundle(t *testing.T) {
	manifest := Manifest("v1.2.3")
	var decoded manifestData
	if err := json.Unmarshal(manifest.Content(), &decoded); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	if decoded.ServerVersion != "v1.2.3" {
		t.Fatalf("serverVersion = %q, want v1.2.3", decoded.ServerVersion)
	}
	if manifest.ETag() == "" {
		t.Fatal("manifest ETag is empty")
	}
	if got := Manifest("v1.2.3").ETag(); got != manifest.ETag() {
		t.Fatalf("manifest ETag changed: %q != %q", got, manifest.ETag())
	}
	if got := Manifest("v1.2.4").ETag(); got == manifest.ETag() {
		t.Fatal("manifest ETag did not change with serverVersion")
	}

	page, ok := Page("daemon-runtimes")
	if !ok {
		t.Fatal("manifest page daemon-runtimes was not loaded")
	}
	var decodedPage pageData
	if err := json.Unmarshal(page.Content(), &decodedPage); err != nil {
		t.Fatalf("decode page: %v", err)
	}
	if decodedPage.Slug != "daemon-runtimes" || page.ETag() == "" {
		t.Fatalf("unexpected page: slug=%q etag=%q", decodedPage.Slug, page.ETag())
	}

	asset, ok := Asset("images/docs/runtime-machine-detail.webp")
	if !ok || len(asset.Content()) == 0 || asset.ETag() == "" {
		t.Fatalf("unexpected asset: ok=%v size=%d etag=%q", ok, len(asset.Content()), asset.ETag())
	}
}

func TestLookupsRejectAnythingOutsideManifest(t *testing.T) {
	for _, slug := range []string{"", "../manifest", "developers/../../manifest", "manifest"} {
		if _, ok := Page(slug); ok {
			t.Errorf("Page(%q) unexpectedly succeeded", slug)
		}
	}
	for _, assetPath := range []string{"", "../manifest.json", "images/docs/../../manifest.json", `images\docs\runtime-machine-detail.webp`, "manifest.json"} {
		if _, ok := Asset(assetPath); ok {
			t.Errorf("Asset(%q) unexpectedly succeeded", assetPath)
		}
	}
}

func TestResourceContentReturnsCopy(t *testing.T) {
	first := Manifest("v1")
	content := first.Content()
	content[0] = 'x'
	if got := first.Content()[0]; got != '{' {
		t.Fatalf("resource content was mutated: first byte = %q", got)
	}
}

func TestLoadBundleValidatesManifestAndResources(t *testing.T) {
	tests := []struct {
		name     string
		manifest string
		files    fstest.MapFS
	}{
		{
			name:     "malformed manifest",
			manifest: `{`,
		},
		{
			name:     "unknown manifest field",
			manifest: `{"title":"Docs","groups":[],"assets":[],"extra":true}`,
		},
		{
			name:     "traversal page slug",
			manifest: manifestJSON("../secret", nil),
		},
		{
			name:     "backslash page slug",
			manifest: manifestJSON(`developers\secret`, nil),
		},
		{
			name:     "missing page",
			manifest: manifestJSON("missing", nil),
		},
		{
			name:     "page identity mismatch",
			manifest: manifestJSON("welcome", nil),
			files: fstest.MapFS{
				"content/pages/welcome.json": &fstest.MapFile{Data: []byte(`{"slug":"other","title":"Welcome","description":"","body":"","toc":[]}`)},
			},
		},
		{
			name:     "traversal asset path",
			manifest: manifestJSON("", []string{"../secret"}),
		},
		{
			name:     "unlisted asset does not satisfy listed asset",
			manifest: manifestJSON("", []string{"images/docs/listed.webp"}),
			files: fstest.MapFS{
				"content/assets/images/docs/unlisted.webp": &fstest.MapFile{Data: []byte("image")},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files := tt.files
			if files == nil {
				files = fstest.MapFS{}
			}
			files["content/manifest.json"] = &fstest.MapFile{Data: []byte(tt.manifest)}
			if _, err := loadBundle(files); err == nil {
				t.Fatal("loadBundle succeeded, want validation error")
			}
		})
	}
}

func TestLoadBundleUsesManifestAsWhitelist(t *testing.T) {
	files := fstest.MapFS{
		"content/manifest.json":                    &fstest.MapFile{Data: []byte(manifestJSON("welcome", []string{"images/docs/listed.webp"}))},
		"content/pages/welcome.json":               &fstest.MapFile{Data: []byte(`{"slug":"welcome","title":"Welcome","description":"","body":"Hello","toc":[]}`)},
		"content/pages/unlisted.json":              &fstest.MapFile{Data: []byte(`{"slug":"unlisted","title":"Unlisted","description":"","body":"","toc":[]}`)},
		"content/assets/images/docs/listed.webp":   &fstest.MapFile{Data: []byte("listed")},
		"content/assets/images/docs/unlisted.webp": &fstest.MapFile{Data: []byte("unlisted")},
	}
	loaded, err := loadBundle(files)
	if err != nil {
		t.Fatalf("loadBundle: %v", err)
	}
	if _, ok := loaded.pages["unlisted"]; ok {
		t.Fatal("unlisted page was loaded")
	}
	if _, ok := loaded.assets["images/docs/unlisted.webp"]; ok {
		t.Fatal("unlisted asset was loaded")
	}
}

func manifestJSON(slug string, assets []string) string {
	items := "[]"
	if slug != "" {
		items = fmt.Sprintf(`[{"slug":%q,"title":"Welcome"}]`, slug)
	}
	assetJSON, _ := json.Marshal(assets)
	return fmt.Sprintf(`{"title":"Docs","groups":[{"label":"Start","items":%s}],"assets":%s}`, items, assetJSON)
}
