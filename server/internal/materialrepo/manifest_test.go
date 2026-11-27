package materialrepo

import (
	"encoding/json"
	"io/fs"
	"strings"
	"testing"
)

func manifestText(t *testing.T) string {
	t.Helper()
	c, e := BuildCatalog(binding)
	if e != nil {
		t.Fatal(e)
	}
	for _, f := range c.Files {
		if f.Path == "material.yaml" {
			return f.Content
		}
	}
	t.Fatal("manifest missing")
	return ""
}
func TestManifestSemanticCompatibility(t *testing.T) {
	original := manifestText(t)
	equivalent := "# User formatting is preserved\n" + strings.ReplaceAll(original, "  ", "    ")
	m, e := ParseManifest(equivalent)
	if e != nil || m.SystemID != binding.SystemID {
		t.Fatalf("equivalent manifest rejected: %v", e)
	}
	r := inspect(t, binding, snapshot(baseSHA, blob("material.yaml", equivalent)))
	if r.Candidate == nil || len(r.Candidate.Files) != 19 {
		t.Fatal("semantic manifest should be preserved")
	}
	if _, e := VerifyMerged(*r.Candidate, mergeSHA, applied(*r.Candidate, mergeSHA)); e != nil {
		t.Fatal(e)
	}
	r = inspect(t, Binding{SystemID: "different"}, snapshot(baseSHA, blob("material.yaml", original)))
	if r.Candidate != nil {
		t.Fatal("wrong system identity accepted")
	}
}
func TestManifestStrictParsing(t *testing.T) {
	good := manifestText(t)
	cases := map[string]string{
		"duplicate":             good + "system_id: another\n",
		"unknown key":           good + "approved: true\n",
		"multiple documents":    good + "---\n" + good,
		"empty second document": good + "---\n",
		"alias":                 strings.Replace(good, "system_id: \"system-1\"", "system_id: &identity system-1", 1),
		"merge key":             good + "<<: {system_id: other}\n",
		"wrong root type":       "[1,2,3]",
		"unknown schema":        strings.Replace(good, "schema_version: 1", "schema_version: 2", 1),
		"unknown bundle":        strings.Replace(good, BundleVersion, "1.99.0", 1),
		"layout redirect":       strings.Replace(good, "releases: releases", "releases: ../releases", 1),
		"non-string id":         strings.Replace(good, "\"system-1\"", "123", 1),
		"custom tag":            strings.Replace(good, "\"system-1\"", "!custom system-1", 1),
		"tagged map":            strings.Replace(good, "layout:", "layout: !custom", 1),
		"non-string key":        good + "123: x\n",
		"deep nesting":          good + "extra: " + strings.Repeat("[", 20) + "x" + strings.Repeat("]", 20),
		"NUL":                   good + "\x00",
		"invalid UTF8":          good + string([]byte{255}),
		"oversized":             strings.Repeat(" ", 16385) + good,
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			if _, e := ParseManifest(input); e == nil {
				t.Fatal("malformed or unrecognized manifest accepted")
			}
		})
	}
}
func TestSchemasAreLocalAndTemplatesAreNotBusinessRecords(t *testing.T) {
	schemaCount, templateCount := 0, 0
	var walk func(any)
	walk = func(v any) {
		switch n := v.(type) {
		case map[string]any:
			if ref, ok := n["$ref"].(string); ok {
				parts := strings.SplitN(ref, "#", 2)
				if len(parts) != 2 || parts[0] != "material.schema.json" || !strings.HasPrefix(parts[1], "/$defs/") {
					t.Fatalf("nonlocal ref %s", ref)
				}
				b, e := assets.ReadFile("assets/.material/schemas/" + parts[0])
				if e != nil {
					t.Fatal(e)
				}
				var target map[string]any
				if json.Unmarshal(b, &target) != nil {
					t.Fatal("invalid reference target")
				}
				defs := target["$defs"].(map[string]any)
				if _, ok := defs[strings.TrimPrefix(parts[1], "/$defs/")]; !ok {
					t.Fatalf("missing ref %s", ref)
				}
			}
			for _, x := range n {
				walk(x)
			}
		case []any:
			for _, x := range n {
				walk(x)
			}
		}
	}
	err := fs.WalkDir(assets, "assets", func(name string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() {
			return nil
		}
		b, e := assets.ReadFile(name)
		if e != nil {
			return e
		}
		if strings.Contains(name, "/schemas/") {
			schemaCount++
			var v map[string]any
			if json.Unmarshal(b, &v) != nil {
				t.Fatalf("invalid schema %s", name)
			}
			if v["additionalProperties"] != false || v["$schema"] != "https://json-schema.org/draft/2020-12/schema" {
				t.Fatalf("missing strict schema envelope %s", name)
			}
			walk(v)
		}
		if strings.Contains(name, "/templates/") {
			templateCount++
			if (!strings.Contains(string(b), "模板") && !strings.Contains(string(b), "TEMPLATE")) || !strings.Contains(string(b), "<") {
				t.Fatalf("template pretends to be a completed record: %s", name)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if schemaCount != 6 || templateCount != 11 {
		t.Fatalf("got schemas=%d templates=%d", schemaCount, templateCount)
	}
}
