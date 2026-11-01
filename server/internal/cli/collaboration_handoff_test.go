package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// Decode-level malformed wire is owned by protocol's source-page tests; these
// cases cover the CLI's request encoding, pre-HTTP checks and identity binding.

func handoffCLIRun(t *testing.T, handler http.HandlerFunc) (*CollaborationContext, *atomic.Int32) {
	t.Helper()
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.Method != http.MethodGet || r.Header.Get("Authorization") != "Bearer mcb_"+strings.Repeat("a", 64) || r.Header.Get("X-Workspace-ID") != "" {
			t.Errorf("handoff read escaped its run capability: %s %s", r.Method, r.URL.Path)
		}
		handler(w, r)
	}))
	t.Cleanup(server.Close)
	collaborationEnvironmentFixture(t)
	t.Setenv(CollaborationServerEnv, server.URL)
	run, err := ReadCollaborationContext()
	if err != nil {
		t.Fatal(err)
	}
	return run, &requests
}

func liveHandoffEnvelope(id protocol.CollaborationRunIdentity) protocol.CollaborationHandoffEnvelope {
	return protocol.CollaborationHandoffEnvelope{ProtocolVersion: 1, IssueID: id.IssueID, RunID: id.RunID, Mode: "live", EventSequence: "3",
		Authority: protocol.CollaborationHandoffAuthority{Status: "in_progress"}, Assignment: protocol.CollaborationHandoffAssignment{Role: "agent", StepKey: "agent"},
		Memory: protocol.CollaborationHandoffMemory{Version: "0", Availability: "absent"},
		Inputs: protocol.CollaborationHandoffInputs{Items: []protocol.CollaborationHandoffInput{}}, Results: protocol.CollaborationHandoffResults{Items: []protocol.CollaborationHandoffResult{}},
		WakeupCauses: protocol.CollaborationHandoffCauses{Items: []protocol.CollaborationHandoffCause{}, Completeness: "recorded"},
		Sources:      []protocol.CollaborationHandoffSource{}, Missing: []string{}}
}

func TestCollaborationReadHandoffEncodesCursorAndBindsTheLiveRun(t *testing.T) {
	cursor := "eyJraW5kIjoicmVzdWx0cyJ9-_0"
	for _, tc := range []struct {
		name, cursor, query string
		mutate              func(*protocol.CollaborationHandoffEnvelope, protocol.CollaborationRunIdentity)
		ok                  bool
	}{
		{name: "first_page", ok: true},
		{name: "cursor", cursor: cursor, query: "cursor=" + cursor, ok: true},
		{name: "other_run", mutate: func(v *protocol.CollaborationHandoffEnvelope, _ protocol.CollaborationRunIdentity) {
			v.RunID = uuid.NewString()
		}},
		{name: "other_issue", mutate: func(v *protocol.CollaborationHandoffEnvelope, _ protocol.CollaborationRunIdentity) {
			v.IssueID = uuid.NewString()
		}},
		{name: "snapshot_mode", mutate: func(v *protocol.CollaborationHandoffEnvelope, _ protocol.CollaborationRunIdentity) {
			v.Mode, v.SnapshotID = "snapshot", uuid.NewString()
		}},
		{name: "live_with_snapshot_id", mutate: func(v *protocol.CollaborationHandoffEnvelope, _ protocol.CollaborationRunIdentity) {
			v.SnapshotID = uuid.NewString()
		}},
		{name: "unknown_version", mutate: func(v *protocol.CollaborationHandoffEnvelope, _ protocol.CollaborationRunIdentity) {
			v.ProtocolVersion = 2
		}},
		{name: "cursor_echo", cursor: cursor, query: "cursor=" + cursor, mutate: func(v *protocol.CollaborationHandoffEnvelope, _ protocol.CollaborationRunIdentity) {
			v.Results.Total, v.Results.Truncated, v.Results.NextCursor = 1, true, cursor
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var run *CollaborationContext
			run, requests := handoffCLIRun(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/collaboration/runtime/runs/"+run.Identity.RunID+"/handoff" || r.URL.RawQuery != tc.query {
					t.Errorf("handoff read used %s?%s", r.URL.Path, r.URL.RawQuery)
				}
				envelope := liveHandoffEnvelope(run.Identity)
				if tc.mutate != nil {
					tc.mutate(&envelope, run.Identity)
				}
				_ = json.NewEncoder(w).Encode(envelope)
			})
			got, err := run.ReadHandoff(t.Context(), run.Client(), tc.cursor)
			if (err == nil) != tc.ok || requests.Load() != 1 {
				t.Fatalf("requests=%d err=%v", requests.Load(), err)
			}
			if !tc.ok && got.RunID != "" {
				t.Fatal("a refused handoff still returned its content")
			}
		})
	}
}

func TestCollaborationReadHandoffRefusesBadCursorBeforeHTTP(t *testing.T) {
	for name, cursor := range map[string]string{"space": "a b", "slash": "a/b", "padding": "abc=", "plus": "a+b", "query": "a&kind=x", "oversized": strings.Repeat("a", 1025)} {
		t.Run(name, func(t *testing.T) {
			run, requests := handoffCLIRun(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(500) })
			if _, err := run.ReadHandoff(t.Context(), run.Client(), cursor); err == nil || requests.Load() != 0 {
				t.Fatalf("bad cursor reached HTTP: requests=%d err=%v", requests.Load(), err)
			}
		})
	}
}

func TestCollaborationReadHandoffKeepsCapabilityRefusal(t *testing.T) {
	run, _ := handoffCLIRun(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusForbidden) })
	_, err := run.ReadHandoff(t.Context(), run.Client(), "")
	if err == nil || !strings.Contains(FormatError(err, false), "Stop this turn") {
		t.Fatalf("capability refusal lost its stop guidance: %v", err)
	}
}

// handoffSourcePage answers with exactly the requested reference and offset.
func handoffSourcePage(id protocol.CollaborationRunIdentity, ref CollaborationHandoffReference, body string, chunk int) protocol.CollaborationHandoffSourcePage {
	end := min(len(body), ref.Offset+chunk)
	trust := "data_only"
	if ref.Kind == "delegation" {
		trust = "admitted_assignment"
	}
	page := protocol.CollaborationHandoffSourcePage{ProtocolVersion: 1, IssueID: id.IssueID, RunID: id.RunID, Offset: ref.Offset, TotalBytes: len(body),
		Source: protocol.CollaborationHandoffSource{Kind: ref.Kind, ID: ref.SourceID, RunID: ref.SourceRunID, Version: ref.Version, SHA256: ref.SHA256,
			Content: body[ref.Offset:end], Truncated: end < len(body), Availability: "available", Trust: trust}}
	if end < len(body) {
		page.NextOffset = &end
	}
	return page
}

func TestCollaborationReadHandoffSourceEncodesTheExactReference(t *testing.T) {
	const body = "Step A verified the schema; step B remains."
	result := CollaborationHandoffReference{SourceID: uuid.NewString(), Kind: "provider_result", SourceRunID: uuid.NewString(), Version: "5", SHA256: protocol.CollaborationHandoffHash(body), Offset: 12}
	note := CollaborationHandoffReference{SourceID: uuid.NewString(), Kind: "handoff", SHA256: protocol.CollaborationHandoffHash(body)}
	for name, tc := range map[string]struct {
		ref   CollaborationHandoffReference
		query url.Values
	}{
		"provider_result": {result, url.Values{"kind": {"provider_result"}, "source_run_id": {result.SourceRunID}, "version": {"5"}, "sha256": {result.SHA256}, "offset": {"12"}}},
		"handoff":         {note, url.Values{"kind": {"handoff"}, "sha256": {note.SHA256}}},
	} {
		t.Run(name, func(t *testing.T) {
			var run *CollaborationContext
			run, requests := handoffCLIRun(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/collaboration/runtime/runs/"+run.Identity.RunID+"/handoff/sources/"+tc.ref.SourceID || r.URL.RawQuery != tc.query.Encode() {
					t.Errorf("source read used %s?%s", r.URL.Path, r.URL.RawQuery)
				}
				_ = json.NewEncoder(w).Encode(handoffSourcePage(run.Identity, tc.ref, body, 32<<10))
			})
			page, err := run.ReadHandoffSource(t.Context(), run.Client(), tc.ref)
			if err != nil || requests.Load() != 1 || page.Source.Content != body[tc.ref.Offset:] || page.NextOffset != nil {
				t.Fatalf("source read failed: requests=%d page=%+v err=%v", requests.Load(), page, err)
			}
		})
	}
}

func TestCollaborationReadHandoffSourceRefusesBadReferenceBeforeHTTP(t *testing.T) {
	valid := CollaborationHandoffReference{SourceID: uuid.NewString(), Kind: "provider_result", SourceRunID: uuid.NewString(), Version: "5", SHA256: protocol.CollaborationHandoffHash("x")}
	for name, mutate := range map[string]func(*CollaborationHandoffReference){
		"source_id":            func(r *CollaborationHandoffReference) { r.SourceID = "source" },
		"uppercase_source_id":  func(r *CollaborationHandoffReference) { r.SourceID = strings.ToUpper(r.SourceID) },
		"nil_source_id":        func(r *CollaborationHandoffReference) { r.SourceID = uuid.Nil.String() },
		"kind":                 func(r *CollaborationHandoffReference) { r.Kind = "record_checkpoint" },
		"empty_kind":           func(r *CollaborationHandoffReference) { r.Kind = "" },
		"missing_run":          func(r *CollaborationHandoffReference) { r.SourceRunID = "" },
		"bad_run":              func(r *CollaborationHandoffReference) { r.SourceRunID = "run" },
		"missing_version":      func(r *CollaborationHandoffReference) { r.Version = "" },
		"zero_version":         func(r *CollaborationHandoffReference) { r.Version = "0" },
		"noncanonical_version": func(r *CollaborationHandoffReference) { r.Version = "05" },
		"handoff_with_run":     func(r *CollaborationHandoffReference) { r.Kind, r.Version = "handoff", "" },
		"handoff_with_version": func(r *CollaborationHandoffReference) { r.Kind, r.SourceRunID = "handoff", "" },
		"uppercase_sha":        func(r *CollaborationHandoffReference) { r.SHA256 = strings.ToUpper(r.SHA256) },
		"short_sha":            func(r *CollaborationHandoffReference) { r.SHA256 = r.SHA256[:63] },
		"negative_offset":      func(r *CollaborationHandoffReference) { r.Offset = -1 },
	} {
		t.Run(name, func(t *testing.T) {
			run, requests := handoffCLIRun(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(500) })
			ref := valid
			mutate(&ref)
			if _, err := run.ReadHandoffSource(t.Context(), run.Client(), ref); err == nil || requests.Load() != 0 {
				t.Fatalf("bad reference reached HTTP: requests=%d err=%v", requests.Load(), err)
			}
		})
	}
}

func TestCollaborationReadHandoffSourceRejectsAnotherReference(t *testing.T) {
	const body = "full provider result"
	ref := CollaborationHandoffReference{SourceID: uuid.NewString(), Kind: "provider_result", SourceRunID: uuid.NewString(), Version: "5", SHA256: protocol.CollaborationHandoffHash(body)}
	for name, mutate := range map[string]func(*protocol.CollaborationHandoffSourcePage){
		"issue":     func(p *protocol.CollaborationHandoffSourcePage) { p.IssueID = uuid.NewString() },
		"run":       func(p *protocol.CollaborationHandoffSourcePage) { p.RunID = uuid.NewString() },
		"source_id": func(p *protocol.CollaborationHandoffSourcePage) { p.Source.ID = uuid.NewString() },
		"kind":      func(p *protocol.CollaborationHandoffSourcePage) { p.Source.Kind = "squad_activity" },
		"source_run": func(p *protocol.CollaborationHandoffSourcePage) {
			p.Source.RunID = uuid.NewString()
		},
		"version": func(p *protocol.CollaborationHandoffSourcePage) { p.Source.Version = "6" },
		"sha256": func(p *protocol.CollaborationHandoffSourcePage) {
			p.Source.SHA256 = protocol.CollaborationHandoffHash("other")
		},
		"offset": func(p *protocol.CollaborationHandoffSourcePage) {
			p.Offset, p.TotalBytes = 4, len(body)+4
		},
		"malformed": func(p *protocol.CollaborationHandoffSourcePage) { p.ProtocolVersion = 2 },
	} {
		t.Run(name, func(t *testing.T) {
			var run *CollaborationContext
			run, _ = handoffCLIRun(t, func(w http.ResponseWriter, _ *http.Request) {
				page := handoffSourcePage(run.Identity, ref, body, 32<<10)
				mutate(&page)
				_ = json.NewEncoder(w).Encode(page)
			})
			if page, err := run.ReadHandoffSource(t.Context(), run.Client(), ref); err == nil || page.Source.Content != "" {
				t.Fatalf("another reference was accepted: %+v %v", page, err)
			}
		})
	}
}
