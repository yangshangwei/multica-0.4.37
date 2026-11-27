package materialrepo

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestCatalogDeterministicAndScoped(t *testing.T) {
	b := Binding{SystemID: "system-1", RootPath: "docs/materials"}
	first, err := BuildCatalog(b)
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildCatalog(b)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Files) != 20 || !reflect.DeepEqual(first, second) {
		t.Fatal("catalog must contain 20 deterministic files")
	}
	for _, f := range first.Files {
		if !strings.HasPrefix(f.Path, "docs/materials/") || f.Mode != "100644" || f.Content == "" {
			t.Fatalf("invalid file %s", f.Path)
		}
	}
	for _, root := range []string{"../escape", "/absolute", "docs/.git", "docs/%2e", "docs\\x", "docs//x", "docs\nprivate"} {
		if _, e := BuildCatalog(Binding{SystemID: "system-1", RootPath: root}); e == nil {
			t.Errorf("accepted unsafe root %q", root)
		}
	}
}

func TestCatalogValidatesJoinedPathLength(t *testing.T) {
	catalog, err := BuildCatalog(Binding{SystemID: "system-1"})
	if err != nil {
		t.Fatal(err)
	}
	longest := 0
	for _, file := range catalog.Files {
		if len(file.Path) > longest {
			longest = len(file.Path)
		}
	}
	root := strings.Repeat("r", 1024-longest-1)
	boundary, err := BuildCatalog(Binding{SystemID: "system-1", RootPath: root})
	if err != nil {
		t.Fatalf("exact boundary rejected: %v", err)
	}
	for _, file := range boundary.Files {
		if len(file.Path) > 1024 {
			t.Fatal("catalog returned an unusable file path")
		}
	}
	if _, err := BuildCatalog(Binding{SystemID: "system-1", RootPath: root + "r"}); err == nil {
		t.Fatal("catalog accepted a root whose generated suffix exceeds the path limit")
	}
}

func TestSharedSchemaPathCases(t *testing.T) {
	raw, err := os.ReadFile("testdata/path-cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Path  string `json:"path"`
		Valid bool   `json:"valid"`
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	for _, example := range cases {
		if validPath(example.Path, false) != example.Valid {
			t.Errorf("path %q: expected valid=%v", example.Path, example.Valid)
		}
	}
}
