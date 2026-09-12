package changelog

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

const testCommit = "1111111111111111111111111111111111111111"
const testBaseCommit = "0000000000000000000000000000000000000000"

func testDocument() map[string]any {
	return map[string]any{
		"schema_version": 1,
		"generated_at":   "2026-09-13T00:00:00Z",
		"releases": []any{map[string]any{
			"id":           "fork:example/changelog-fixture:v9.9.9",
			"version":      "v9.9.9",
			"title":        "Synthetic release fixture",
			"published_at": "2026-09-13T00:00:00Z",
			"status":       "published",
			"source":       "fork",
			"commit":       testCommit,
			"base_commit":  testBaseCommit,
			"sections": []any{map[string]any{
				"category": "features",
				"items": []any{map[string]any{
					"text":   "A synthetic fixture change.",
					"commit": testCommit,
				}},
			}},
		}},
	}
}

func testRelease(document map[string]any) map[string]any {
	return document["releases"].([]any)[0].(map[string]any)
}

func testSection(document map[string]any) map[string]any {
	return testRelease(document)["sections"].([]any)[0].(map[string]any)
}

func testItem(document map[string]any) map[string]any {
	return testSection(document)["items"].([]any)[0].(map[string]any)
}

func marshalTestDocument(t *testing.T, document map[string]any) []byte {
	t.Helper()
	data, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestChangelogValidateAcceptsSchemaAndHonestSources(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(map[string]any)
	}{
		{"published fork", func(map[string]any) {}},
		{"empty history", func(d map[string]any) { d["releases"] = []any{} }},
		{"attributed upstream", func(d map[string]any) {
			r := testRelease(d)
			r["id"], r["source"] = "upstream:multica-ai/multica:v9.9.9", "upstream"
			r["commit"], r["base_commit"] = nil, nil
		}},
		{"unreleased fork", func(d map[string]any) {
			r := testRelease(d)
			r["id"], r["version"] = "fork:example/changelog-fixture:unreleased", "Unreleased"
			r["status"], r["published_at"] = "unreleased", nil
		}},
		{"prerelease", func(d map[string]any) {
			r := testRelease(d)
			r["id"], r["version"] = "fork:example/changelog-fixture:v9.9.9-test.1", "v9.9.9-test.1"
			r["status"] = "prerelease"
		}},
		{"sha256 commits", func(d map[string]any) {
			r := testRelease(d)
			r["commit"], r["base_commit"] = strings.Repeat("1", 64), strings.Repeat("0", 64)
			testItem(d)["commit"] = strings.Repeat("a", 64)
		}},
		{"additive fields", func(d map[string]any) {
			d["future_metadata"] = map[string]any{"enabled": true}
			testRelease(d)["future_label"] = "new"
		}},
		{"optional item commit", func(d map[string]any) { delete(testItem(d), "commit") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			document := testDocument()
			tc.mutate(document)
			if _, err := Validate(marshalTestDocument(t, document)); err != nil {
				t.Fatalf("valid history rejected: %v", err)
			}
		})
	}
}

func TestChangelogValidateRejectsInvalidHistory(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(map[string]any)
	}{
		{"unsupported schema", func(d map[string]any) { d["schema_version"] = 2 }},
		{"wrong schema type", func(d map[string]any) { d["schema_version"] = "1" }},
		{"missing timestamp", func(d map[string]any) { delete(d, "generated_at") }},
		{"invalid timestamp", func(d map[string]any) { d["generated_at"] = "2026-02-30T00:00:00Z" }},
		{"missing history", func(d map[string]any) { delete(d, "releases") }},
		{"null history", func(d map[string]any) { d["releases"] = nil }},
		{"unknown source", func(d map[string]any) { testRelease(d)["source"] = "unknown" }},
		{"unknown status", func(d map[string]any) { testRelease(d)["status"] = "future" }},
		{"blank title", func(d map[string]any) { testRelease(d)["title"] = " \t" }},
		{"blank version", func(d map[string]any) { testRelease(d)["version"] = " " }},
		{"identity mismatch", func(d map[string]any) { testRelease(d)["id"] = "fork:example/changelog-fixture:v8.8.8" }},
		{"source mismatch", func(d map[string]any) { testRelease(d)["source"] = "upstream" }},
		{"invalid repository identity", func(d map[string]any) { testRelease(d)["id"] = "fork:bad:v9.9.9" }},
		{"published without date", func(d map[string]any) { testRelease(d)["published_at"] = nil }},
		{"missing date field", func(d map[string]any) { delete(testRelease(d), "published_at") }},
		{"invalid release date", func(d map[string]any) { testRelease(d)["published_at"] = "yesterday" }},
		{"unreleased with date", func(d map[string]any) {
			r := testRelease(d)
			r["status"], r["id"] = "unreleased", "fork:example/changelog-fixture:unreleased"
		}},
		{"fork missing commit", func(d map[string]any) { delete(testRelease(d), "commit") }},
		{"fork null commit", func(d map[string]any) { testRelease(d)["commit"] = nil }},
		{"fork null base", func(d map[string]any) { testRelease(d)["base_commit"] = nil }},
		{"short commit", func(d map[string]any) { testRelease(d)["commit"] = "1111111" }},
		{"uppercase commit", func(d map[string]any) { testRelease(d)["commit"] = strings.Repeat("A", 40) }},
		{"upstream missing nullable field", func(d map[string]any) {
			r := testRelease(d)
			r["id"], r["source"] = "upstream:multica-ai/multica:v9.9.9", "upstream"
			r["commit"] = nil
			delete(r, "base_commit")
		}},
		{"missing sections", func(d map[string]any) { delete(testRelease(d), "sections") }},
		{"null sections", func(d map[string]any) { testRelease(d)["sections"] = nil }},
		{"unknown category", func(d map[string]any) { testSection(d)["category"] = "marketing" }},
		{"missing items", func(d map[string]any) { delete(testSection(d), "items") }},
		{"null items", func(d map[string]any) { testSection(d)["items"] = nil }},
		{"blank item text", func(d map[string]any) { testItem(d)["text"] = " " }},
		{"item text wrong type", func(d map[string]any) { testItem(d)["text"] = 123 }},
		{"item null commit", func(d map[string]any) { testItem(d)["commit"] = nil }},
		{"item invalid commit", func(d map[string]any) { testItem(d)["commit"] = "not-a-commit" }},
		{"duplicate identity", func(d map[string]any) { d["releases"] = append(d["releases"].([]any), testRelease(d)) }},
		{"mixed fork history", func(d map[string]any) {
			other := testRelease(testDocument())
			other["id"] = "fork:other/changelog-fixture:v9.9.9"
			d["releases"] = append(d["releases"].([]any), other)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			document := testDocument()
			tc.mutate(document)
			if _, err := Validate(marshalTestDocument(t, document)); err == nil {
				t.Fatal("invalid history accepted")
			}
		})
	}
}

func TestChangelogValidateBoundsWholeDocument(t *testing.T) {
	valid := marshalTestDocument(t, testDocument())
	for name, data := range map[string][]byte{
		"trailing JSON": append(bytes.Clone(valid), []byte(" {}")...),
		"invalid UTF-8": bytes.Replace(valid, []byte("Synthetic"), []byte{0xff}, 1),
		"over 2 MiB":    append(bytes.Clone(valid), bytes.Repeat([]byte(" "), MaxFileSize+1-len(valid))...),
		"null":          []byte("null"),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Validate(data); err == nil {
				t.Fatal("invalid document accepted")
			}
		})
	}
	bounded := append(bytes.Clone(valid), bytes.Repeat([]byte(" "), MaxFileSize-len(valid))...)
	if _, err := Validate(bounded); err != nil {
		t.Fatalf("document exactly at 2 MiB rejected: %v", err)
	}
}

func readTestResponse(t *testing.T, reader *Reader) (Response, Resource, error) {
	t.Helper()
	resource, readErr := reader.Read()
	var response Response
	if err := json.Unmarshal(resource.Content(), &response); err != nil {
		t.Fatalf("invalid response resource: %v", err)
	}
	return response, resource, readErr
}

func replaceTestFile(t *testing.T, path string, data []byte) {
	t.Helper()
	staged := path + ".next"
	if err := os.WriteFile(staged, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(staged, path); err != nil {
		t.Fatal(err)
	}
}

func TestChangelogReaderEmbeddedBaseline(t *testing.T) {
	response, resource, err := readTestResponse(t, New("", "v1.2.3"))
	if err != nil || response.FeedSource != "embedded" || response.IsStale || response.Warning != nil {
		t.Fatalf("embedded state = %+v, error = %v", response, err)
	}
	if response.SchemaVersion != 1 || response.Releases == nil || response.ServerVersion == nil || *response.ServerVersion != "v1.2.3" {
		t.Fatalf("embedded envelope = %+v", response)
	}
	if !strings.HasPrefix(resource.ETag(), `"`) || strings.HasPrefix(resource.ETag(), "W/") {
		t.Fatalf("strong ETag missing: %q", resource.ETag())
	}
	unknown, other, err := readTestResponse(t, New("", ""))
	if err != nil || unknown.ServerVersion != nil || other.ETag() == resource.ETag() {
		t.Fatalf("unavailable server version must be null and change ETag: %+v, %v", unknown, err)
	}
	content := resource.Content()
	content[0] = '!'
	if resource.Content()[0] != '{' {
		t.Fatal("caller mutated immutable resource")
	}
}

func TestChangelogReaderHotReplacementAndFailureRetention(t *testing.T) {
	path := filepath.Join(t.TempDir(), "feed.json")
	document := testDocument()
	replaceTestFile(t, path, marshalTestDocument(t, document))
	reader := New(path, "v1.2.3")
	first, firstResource, err := readTestResponse(t, reader)
	if err != nil || first.FeedSource != "file" || first.IsStale || first.Warning != nil {
		t.Fatalf("configured file state = %+v, error = %v", first, err)
	}
	testRelease(document)["title"] = "An atomically published revision"
	replaceTestFile(t, path, marshalTestDocument(t, document))
	updated, updatedResource, err := readTestResponse(t, reader)
	if err != nil || len(updated.Releases) != 1 || updated.Releases[0].Title != "An atomically published revision" || firstResource.ETag() == updatedResource.ETag() {
		t.Fatalf("same reader did not observe file replacement: %+v, %v", updated, err)
	}
	for _, tc := range []struct {
		name, warning string
		fail          func()
	}{
		{"malformed", WarningFileInvalid, func() { replaceTestFile(t, path, []byte(`{"releases":`)) }},
		{"oversized", WarningFileInvalid, func() { replaceTestFile(t, path, bytes.Repeat([]byte(" "), MaxFileSize+1)) }},
		{"missing", WarningFileUnavailable, func() {
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.fail()
			stale, staleResource, err := readTestResponse(t, reader)
			if err == nil || !stale.IsStale || stale.Warning == nil || *stale.Warning != tc.warning || stale.FeedSource != "file" {
				t.Fatalf("failed refresh not represented honestly: %+v, %v", stale, err)
			}
			if len(stale.Releases) != 1 || stale.Releases[0].Title != updated.Releases[0].Title || staleResource.ETag() == updatedResource.ETag() {
				t.Fatal("failure erased last valid history or reused healthy ETag")
			}
			if strings.Contains(string(staleResource.Content()), path) {
				t.Fatal("public response exposes filesystem path")
			}
		})
	}
	replaceTestFile(t, path, marshalTestDocument(t, document))
	recovered, recoveredResource, err := readTestResponse(t, reader)
	if err != nil || recovered.IsStale || recovered.Warning != nil || recoveredResource.ETag() != updatedResource.ETag() {
		t.Fatalf("restoring valid source did not recover: %+v, %v", recovered, err)
	}
}

func TestChangelogReaderInitialFailureUsesEmbeddedAndInstancesDoNotShareCache(t *testing.T) {
	path := filepath.Join(t.TempDir(), "feed.json")
	replaceTestFile(t, path, marshalTestDocument(t, testDocument()))
	firstReader := New(path, "one")
	first, _, err := readTestResponse(t, firstReader)
	if err != nil || first.FeedSource != "file" {
		t.Fatalf("initial read: %+v, %v", first, err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	retained, _, err := readTestResponse(t, firstReader)
	if err == nil || retained.FeedSource != "file" || len(retained.Releases) != 1 {
		t.Fatalf("first instance lost its own file snapshot: %+v, %v", retained, err)
	}
	secondReader := New(path, "two")
	fallback, _, err := readTestResponse(t, secondReader)
	if err == nil || fallback.FeedSource != "embedded" || !fallback.IsStale || fallback.Warning == nil || *fallback.Warning != WarningFileUnavailable {
		t.Fatalf("never-valid instance did not use explicit stale embedded fallback: %+v, %v", fallback, err)
	}
	if fallback.ServerVersion == nil || *fallback.ServerVersion != "two" {
		t.Fatal("server instance version leaked")
	}
	replaceTestFile(t, path, []byte("not JSON"))
	fallback, _, err = readTestResponse(t, secondReader)
	if err == nil || fallback.FeedSource != "embedded" || fallback.Warning == nil || *fallback.Warning != WarningFileInvalid {
		t.Fatalf("initial invalid file fallback = %+v, %v", fallback, err)
	}
	otherPath := filepath.Join(t.TempDir(), "other.json")
	otherDocument := testDocument()
	testRelease(otherDocument)["title"] = "A separate deployment"
	replaceTestFile(t, otherPath, marshalTestDocument(t, otherDocument))
	other, _, err := readTestResponse(t, New(otherPath, "three"))
	if err != nil || other.Releases[0].Title != "A separate deployment" {
		t.Fatalf("separate instance feed = %+v, %v", other, err)
	}
}

func TestChangelogReaderConcurrentReadsAndAtomicReplacement(t *testing.T) {
	path := filepath.Join(t.TempDir(), "feed.json")
	document := testDocument()
	replaceTestFile(t, path, marshalTestDocument(t, document))
	reader := New(path, "race-fixture")
	start := make(chan struct{})
	var readers sync.WaitGroup
	for range 12 {
		readers.Go(func() {
			<-start
			for range 30 {
				response, _, err := readTestResponse(t, reader)
				if err != nil || response.IsStale || response.FeedSource != "file" || len(response.Releases) != 1 {
					t.Errorf("partial document during atomic replacement: %+v, %v", response, err)
					return
				}
			}
		})
	}
	close(start)
	for revision := range 20 {
		testRelease(document)["title"] = fmt.Sprintf("Synthetic revision %d", revision)
		replaceTestFile(t, path, marshalTestDocument(t, document))
	}
	readers.Wait()
	last, _, err := readTestResponse(t, reader)
	if err != nil || last.Releases[0].Title != "Synthetic revision 19" {
		t.Fatalf("latest committed snapshot lost: %+v, %v", last, err)
	}
	replaceTestFile(t, path, []byte("invalid"))
	stale, _, err := readTestResponse(t, reader)
	if err == nil || !stale.IsStale || stale.Releases[0].Title != last.Releases[0].Title {
		t.Fatalf("failed read lost the final serialized snapshot: %+v, %v", stale, err)
	}
}
