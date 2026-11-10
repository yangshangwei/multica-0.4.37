package artifact

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func testRequest(t *testing.T) Request {
	t.Helper()
	entries := []Entry{{Path: "z.txt", SizeBytes: 1, SHA256: strings.Repeat("a", 64)}, {Path: "a.txt", SizeBytes: 0, SHA256: strings.Repeat("b", 64)}}
	digest, err := InputDigest(entries)
	if err != nil {
		t.Fatal(err)
	}
	return Request{AgentID: "11111111-1111-4111-8111-111111111111", TaskID: "22222222-2222-4222-8222-222222222222", ResourceKind: "issue", ResourceID: "33333333-3333-4333-8333-333333333333", Action: Action, StagingID: "44444444-4444-4444-8444-444444444444", SourceRevision: strings.Repeat("a", 40), StagingManifestDigest: strings.Repeat("c", 64), InputManifestDigest: digest, Entries: entries, TargetEnvironment: Environment, PolicyVersion: Policy}
}

func testBinding() Binding {
	return Binding{WorkspaceID: "55555555-5555-4555-8555-555555555555", InitiatorKind: "human", InitiatorID: "66666666-6666-4666-8666-666666666666", OriginatorUserID: "77777777-7777-4777-8777-777777777777", RuntimeID: "88888888-8888-4888-8888-888888888888", TaskGeneration: 1}
}

func TestArtifactSpecCanonical(t *testing.T) {
	r := testRequest(t)
	if r.InputManifestDigest != "bd8414ba690405f2a49c4552272a52ca35acfd0d14de253f125832d346665d5e" {
		t.Fatal("canonical input vector changed", r.InputManifestDigest)
	}
	a, err := RequestDigest(r, testBinding())
	if err != nil {
		t.Fatal(err)
	}
	r.Entries[0], r.Entries[1] = r.Entries[1], r.Entries[0]
	b, err := RequestDigest(r, testBinding())
	if err != nil || a != b {
		t.Fatalf("ordering changed digest: %s %s %v", a, b, err)
	}
	changed := testBinding()
	changed.InitiatorID = changed.WorkspaceID
	c, err := RequestDigest(r, changed)
	if err != nil || a == c {
		t.Fatalf("principal did not bind: %v", err)
	}
	s, err := makeSpec(r, testBinding(), strings.Repeat("d", 64), time.Date(2026, 11, 10, 0, 0, 0, 123456789, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	data, digest, err := CanonicalSpec(s)
	if err != nil {
		t.Fatal(err)
	}
	if s.CreatedAt != "2026-11-10T00:00:00.123456Z" || s.ExpiresAt != "2026-11-10T00:15:00.123456Z" {
		t.Fatal(s.CreatedAt, s.ExpiresAt)
	}
	parsed, err := ParseSpec(data)
	if err != nil {
		t.Fatal(err)
	}
	again, other, err := CanonicalSpec(parsed)
	if err != nil || !bytes.Equal(data, again) || digest != other {
		t.Fatal("canonical roundtrip", err)
	}
	if bytes.Contains(data, []byte("\n")) || digest == a || digest == r.InputManifestDigest {
		t.Fatal("bad canonical/domain contract")
	}
}

func TestArtifactSpecRejectsAmbiguousJSON(t *testing.T) {
	r := testRequest(t)
	data, err := canonicalJSON(r)
	if err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string][]byte{
		"unknown":          bytes.Replace(data, []byte(`{"agent_id"`), []byte(`{"other":true,"agent_id"`), 1),
		"duplicate":        bytes.Replace(data, []byte(`{"agent_id"`), []byte(`{"agent_id":"ignored","agent_id"`), 1),
		"nested_duplicate": bytes.Replace(data, []byte(`"size_bytes":1`), []byte(`"size_bytes":0,"size_bytes":1`), 1),
		"null_size":        bytes.Replace(data, []byte(`"size_bytes":1`), []byte(`"size_bytes":null`), 1),
		"missing_size":     bytes.Replace(data, []byte(`"size_bytes":1,`), nil, 1),
		"trailing":         append(append([]byte{}, data...), []byte(` {}`)...),
		"oversize":         []byte(strings.Repeat(" ", MaxRequestBytes+1)),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseRequest(bytes.NewReader(body)); !errors.Is(err, ErrInvalid) {
				t.Fatalf("accepted malformed body: %v", err)
			}
		})
	}
	if _, err := ParseRequest(bytes.NewReader(data)); err != nil {
		t.Fatal(err)
	}
}

func TestArtifactSpecPathAndManifestBoundary(t *testing.T) {
	for _, path := range []string{"", "../a", "/a", "a//b", "a/./b", "a/../b", "a\\b", "C:a", "NUL", "con.txt", "a.", "é", strings.Repeat("x", 97), "a/b/c/d/e/f/g/h/i"} {
		t.Run(path, func(t *testing.T) {
			if _, err := InputDigest([]Entry{{Path: path, SHA256: strings.Repeat("a", 64)}}); err == nil {
				t.Fatal("accepted path")
			}
		})
	}
	for name, entries := range map[string][]Entry{
		"duplicate": {{Path: "a", SHA256: strings.Repeat("a", 64)}, {Path: "a", SHA256: strings.Repeat("a", 64)}},
		"case":      {{Path: "A", SHA256: strings.Repeat("a", 64)}, {Path: "a", SHA256: strings.Repeat("a", 64)}},
		"parent":    {{Path: "A", SHA256: strings.Repeat("a", 64)}, {Path: "a/b", SHA256: strings.Repeat("a", 64)}},
		"negative":  {{Path: "a", SizeBytes: -1, SHA256: strings.Repeat("a", 64)}},
		"large":     {{Path: "a", SizeBytes: MaxFileBytes + 1, SHA256: strings.Repeat("a", 64)}},
		"digest":    {{Path: "a", SHA256: strings.Repeat("A", 64)}},
		"empty":     {},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := InputDigest(entries); err == nil {
				t.Fatal("accepted manifest")
			}
		})
	}
	r := testRequest(t)
	r.InputManifestDigest = strings.Repeat("0", 64)
	if _, err := NormalizeRequest(r); err == nil {
		t.Fatal("accepted incorrect input digest")
	}
}

func TestArtifactSpecStrictSmallBody(t *testing.T) {
	for _, body := range []string{`{"EXPECTED_SPEC_DIGEST":"abc"}`, `{"expected_spec_digest":"a","expected_spec_digest":"b"}`, `{"expected_spec_digest":null}`, `{"expected_spec_digest":"a"} {}`} {
		var dst struct {
			Digest string `json:"expected_spec_digest"`
		}
		if err := DecodeStrict(strings.NewReader(body), &dst); !errors.Is(err, ErrInvalid) {
			t.Fatalf("accepted %q: %v", body, err)
		}
	}
}

func TestArtifactSpecExactLimits(t *testing.T) {
	entries := make([]Entry, MaxFiles)
	for i := range entries {
		entries[i] = Entry{Path: fmt.Sprintf("file-%02d", i), SHA256: strings.Repeat("a", 64)}
	}
	if _, err := InputDigest(entries); err != nil {
		t.Fatal("64 empty files refused", err)
	}
	if _, err := InputDigest(append(entries, Entry{Path: "excess", SHA256: strings.Repeat("a", 64)})); err == nil {
		t.Fatal("65 files accepted")
	}
	entries = entries[:4]
	for i := range entries {
		entries[i].SizeBytes = MaxFileBytes
	}
	if _, err := InputDigest(entries); err != nil {
		t.Fatal("32MiB refused", err)
	}
	if _, err := InputDigest(append(entries, Entry{Path: "extra", SizeBytes: 1, SHA256: strings.Repeat("a", 64)})); err == nil {
		t.Fatal("aggregate overflow accepted")
	}
	r := testRequest(t)
	mutations := map[string]func(*Request){
		"agent": func(r *Request) { r.AgentID = "wrong" }, "task": func(r *Request) { r.TaskID = "wrong" }, "resource_kind": func(r *Request) { r.ResourceKind = "project" }, "resource": func(r *Request) { r.ResourceID = "wrong" }, "action": func(r *Request) { r.Action = "deploy" }, "staging": func(r *Request) { r.StagingID = "../outside" }, "revision": func(r *Request) { r.SourceRevision = strings.Repeat("A", 40) }, "staging_digest": func(r *Request) { r.StagingManifestDigest = "short" }, "environment": func(r *Request) { r.TargetEnvironment = "production" }, "policy": func(r *Request) { r.PolicyVersion = "v2" },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			bad := r
			mutate(&bad)
			if _, err := NormalizeRequest(bad); !errors.Is(err, ErrInvalid) {
				t.Fatal("accepted mutated contract", err)
			}
		})
	}
}
