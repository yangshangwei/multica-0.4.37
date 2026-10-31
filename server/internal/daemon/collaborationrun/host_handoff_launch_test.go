package collaborationrun

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type launchHandoffFixture struct {
	*hostRunFixture
	mu               sync.Mutex
	envelope         protocol.CollaborationHandoffEnvelope
	snapshots        []protocol.CollaborationHandoffSnapshotRequest
	bindings         []protocol.CollaborationHandoffBinding
	snapshotFailures []HTTPError
	bindingFailures  []HTTPError
	mutateSnapshot   func(*protocol.CollaborationHandoffEnvelope)
	mutateReceipt    func(*protocol.CollaborationHandoffBindingReceipt)
	loseSnapshot     bool
	loseBinding      bool
}

func newLaunchHandoffFixture(t *testing.T, mode string) *launchHandoffFixture {
	t.Helper()
	f := newHostRunFixture(t, mode)
	f.claim.HandoffVersion = protocol.CollaborationHandoffVersion
	parent := uuid.NewString()
	f.claim.HostContext.Role, f.claim.HostContext.StepKey, f.claim.HostContext.ParentRunID = "member", "dispatch-1", &parent
	f.memoryVersion, f.memoryContent = "3", "Use the verified shared database."
	sourceID, delegationID, transferID, handlerID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	resultRun, resultID := uuid.NewString(), uuid.NewString()
	note, delegation, result := "Please verify the chosen database.", "Only execute step B: verify the database choice.", "Step A verified the schema."
	h := &launchHandoffFixture{hostRunFixture: f, envelope: protocol.CollaborationHandoffEnvelope{
		ProtocolVersion: 1, IssueID: f.claim.Identity.IssueID, RunID: f.claim.Identity.RunID, Mode: "snapshot", EventSequence: "7",
		Authority:  protocol.CollaborationHandoffAuthority{Status: "in_progress", HandlerID: handlerID, CreatorID: f.claim.Inputs[0].AuthorUserID},
		Assignment: protocol.CollaborationHandoffAssignment{Role: "member", StepKey: "dispatch-1", ParentRunID: parent, DelegationID: delegationID, SourceID: sourceID},
		Handoff: &protocol.CollaborationHandoffTransfer{ID: transferID, AuthorID: f.claim.Inputs[0].AuthorUserID,
			FromID: f.claim.Inputs[0].AuthorUserID, ToID: handlerID, Purpose: "transfer", EventSequence: "7", SourceID: transferID},
		Memory: protocol.CollaborationHandoffMemory{Version: "3", SHA256: protocol.CollaborationHandoffHash(f.memoryContent), Availability: "available"},
		Inputs: protocol.CollaborationHandoffInputs{CollaborationHandoffPage: protocol.CollaborationHandoffPage{Total: 1, Returned: 1},
			Items: []protocol.CollaborationHandoffInput{{CommandID: f.claim.Inputs[0].CommandID, AuthorID: f.claim.Inputs[0].AuthorUserID, State: "consumed", ReaderRunIDs: []string{f.claim.Identity.RunID}}}},
		Results: protocol.CollaborationHandoffResults{CollaborationHandoffPage: protocol.CollaborationHandoffPage{Total: 1, Returned: 1},
			Items: []protocol.CollaborationHandoffResult{{RunID: resultRun, Role: "member", StepKey: "step-A", Progress: "completed", Outcome: "success", ProofState: "host_exit_observed", ResultState: "present", FinalAnswerState: "unknown", SourceID: resultID, SourceKind: "provider_result"}}},
		WakeupCauses: protocol.CollaborationHandoffCauses{Items: []protocol.CollaborationHandoffCause{}, Completeness: "unknown"},
		Sources: []protocol.CollaborationHandoffSource{
			{Kind: "handoff", ID: transferID, SHA256: protocol.CollaborationHandoffHash(note), Content: note, Availability: "available", Trust: "data_only"},
			{Kind: "delegation", ID: sourceID, RunID: parent, Version: "4", SHA256: protocol.CollaborationHandoffHash(delegation), Content: delegation, Availability: "available", Trust: "admitted_assignment"},
			{Kind: "provider_result", ID: resultID, RunID: resultRun, Version: "5", SHA256: protocol.CollaborationHandoffHash(result), Content: result, Availability: "available", Trust: "data_only"},
		}, Missing: []string{"final_answer_unavailable", "wake_causes_not_recorded"},
	}}
	informationID := uuid.NewString()
	h.envelope.InformationRequest = &protocol.CollaborationHandoffTransfer{ID: informationID, AuthorID: f.claim.Inputs[0].AuthorUserID,
		FromID: f.claim.Inputs[0].AuthorUserID, ToID: handlerID, RequesterID: f.claim.Inputs[0].AuthorUserID,
		Purpose: "request_information", EventSequence: "6", SourceID: informationID}
	informationNote := "Which configured database should this verification use?"
	h.envelope.Sources = append(h.envelope.Sources, protocol.CollaborationHandoffSource{Kind: "handoff", ID: informationID,
		SHA256: protocol.CollaborationHandoffHash(informationNote), Content: informationNote, Availability: "available", Trust: "data_only"})
	original := f.server.Config.Handler
	f.server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != runPath(f.claim.Identity.RunID, "handoff/snapshots") && r.URL.Path != runPath(f.claim.Identity.RunID, "handoff/bind") {
			original.ServeHTTP(w, r)
			return
		}
		h.mu.Lock()
		defer h.mu.Unlock()
		f.mu.Lock()
		authorized := false
		for seq := range f.permits {
			authorized = authorized || r.Header.Get("Authorization") == "Bearer mcb_"+strings.Repeat(seq, 64)
		}
		f.mu.Unlock()
		if r.Method != http.MethodPost || !authorized || r.URL.RawQuery != "" || r.Header.Get("Cookie") != "" || r.Header.Get("X-Workspace-ID") != "" {
			t.Error("handoff request lost its run business authority")
			w.WriteHeader(http.StatusForbidden)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/snapshots") {
			var in protocol.CollaborationHandoffSnapshotRequest
			if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
				t.Error(err)
				return
			}
			h.snapshots = append(h.snapshots, in)
			if serveLaunchHandoffFailure(w, &h.snapshotFailures) {
				return
			}
			out := h.envelope
			out.SnapshotID = in.RequestID
			if h.mutateSnapshot != nil {
				h.mutateSnapshot(&out)
			}
			if h.loseSnapshot {
				h.loseSnapshot = false
				conn, _, _ := w.(http.Hijacker).Hijack()
				_ = conn.Close()
				return
			}
			_ = json.NewEncoder(w).Encode(out)
			return
		}
		var in protocol.CollaborationHandoffBinding
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			t.Error(err)
			return
		}
		h.bindings = append(h.bindings, in)
		f.mu.Lock()
		permit, ok := f.permits[in.AttemptSeq]
		f.mu.Unlock()
		if !ok || permit.AttemptNonce != in.AttemptNonce || permit.LaunchDescriptor.Kind != "provider" {
			t.Error("snapshot was bound before provider authorization or to a preparation attempt")
		}
		if serveLaunchHandoffFailure(w, &h.bindingFailures) {
			return
		}
		out := protocol.CollaborationHandoffBindingReceipt{ProtocolVersion: 1, IssueID: f.claim.Identity.IssueID, RunID: f.claim.Identity.RunID,
			SnapshotID: in.SnapshotID, PromptSHA256: in.PromptSHA256, AttemptSeq: in.AttemptSeq, AttemptNonce: in.AttemptNonce, Accepted: true}
		if h.mutateReceipt != nil {
			h.mutateReceipt(&out)
		}
		if h.loseBinding {
			h.loseBinding = false
			conn, _, _ := w.(http.Hijacker).Hijack()
			_ = conn.Close()
			return
		}
		_ = json.NewEncoder(w).Encode(out)
	})
	return h
}

func serveLaunchHandoffFailure(w http.ResponseWriter, failures *[]HTTPError) bool {
	if len(*failures) == 0 {
		return false
	}
	failure := (*failures)[0]
	*failures = (*failures)[1:]
	if failure.Status == 0 {
		return false
	}
	w.WriteHeader(failure.Status)
	_ = json.NewEncoder(w).Encode(map[string]string{"code": failure.Code})
	return true
}

func TestHostHandoffLaunchInjectsAndBindsTheActualFirstInput(t *testing.T) {
	h := newLaunchHandoffFixture(t, "result")
	if err := h.runner.Run(t.Context(), h.claim.Identity.RunID, h.request); err != nil {
		t.Fatal(err)
	}
	starts := h.starts(t)
	if len(starts) != 1 {
		t.Fatalf("provider starts=%d", len(starts))
	}
	prompt, input := hostStartPrompt(t, starts[0])
	var admitted []protocol.CollaborationRunInput
	if err := json.Unmarshal(input["inputs"], &admitted); err != nil || !reflect.DeepEqual(admitted, h.claim.Inputs) {
		t.Fatal("startup snapshot replaced the full frozen input set")
	}
	var handoff protocol.CollaborationHandoffEnvelope
	if err := json.Unmarshal(input["handoff"], &handoff); err != nil {
		t.Fatalf("actual provider input omitted the handoff: %v", err)
	}
	if len(h.snapshots) != 1 || h.snapshots[0].MemoryVersion != "3" || handoff.SnapshotID != h.snapshots[0].RequestID {
		t.Fatal("snapshot identity or memory base was lost")
	}
	expected := h.envelope
	expected.SnapshotID = h.snapshots[0].RequestID
	if !reflect.DeepEqual(handoff, expected) {
		t.Fatal("provider input lost authority, request/note, assignment, prior result, source text or missing facts")
	}
	if len(h.bindings) != 1 || h.bindings[0].PromptSHA256 != protocol.CollaborationHandoffHash(prompt) || h.bindings[0].SnapshotID != handoff.SnapshotID || h.bindings[0].AttemptSeq != "1" {
		t.Fatal("actual input was not bound to its provider attempt")
	}
	if !strings.Contains(prompt, h.envelope.Sources[1].Content) {
		t.Fatal("delegation requires a discretionary tool read")
	}
	brief, err := os.ReadFile(filepath.Join(starts[0]["cwd"].(string), "CLAUDE.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, source := range h.envelope.Sources {
		if strings.Contains(string(brief), source.Content) {
			t.Fatal("handoff plaintext reached the brief")
		}
		if err := filepath.WalkDir(h.runner.Ledger.dir, func(path string, entry os.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return err
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if strings.Contains(string(raw), source.Content) {
				t.Error("handoff plaintext reached the ledger")
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestHostHandoffLaunchKeepsMaximumAdmittedInputsBeyondMetadataPage(t *testing.T) {
	h := newLaunchHandoffFixture(t, "result")
	template := h.claim.Inputs[0]
	h.claim.Inputs = make([]protocol.CollaborationRunInput, 1024)
	h.claim.Preparation.CommandIDs = make([]string, len(h.claim.Inputs))
	for i := range h.claim.Inputs {
		in := template
		if i != 0 {
			in.CommandID = uuid.NewString()
		}
		in.Body = strings.Repeat("x", 1400)
		h.claim.Inputs[i], h.claim.Preparation.CommandIDs[i] = in, in.CommandID
	}
	h.envelope.Inputs.Total, h.envelope.Inputs.Truncated, h.envelope.Inputs.NextCursor = 1024, true, "remaining-input-metadata"
	if err := protocol.SealCollaborationPreparation(h.claim.Preparation); err != nil {
		t.Fatal(err)
	}
	if err := h.runner.Run(t.Context(), h.claim.Identity.RunID, h.request); err != nil {
		t.Fatal(err)
	}
	starts := h.starts(t)
	if len(starts) != 1 {
		t.Fatal("valid maximum-count inputs were not launched")
	}
	_, input := hostStartPrompt(t, starts[0])
	var admitted []protocol.CollaborationRunInput
	if err := json.Unmarshal(input["inputs"], &admitted); err != nil || !reflect.DeepEqual(admitted, h.claim.Inputs) {
		t.Fatal("bounded handoff metadata truncated executable inputs")
	}
}

func TestHostHandoffLaunchFreshFallbackKeepsSnapshotAndBindsNewInput(t *testing.T) {
	h := newLaunchHandoffFixture(t, "reject_resume")
	seedHostResume(t, h.hostRunFixture)
	if err := h.runner.Run(t.Context(), h.claim.Identity.RunID, h.request); err != nil {
		t.Fatal(err)
	}
	starts := h.starts(t)
	if len(starts) != 2 || len(h.snapshots) != 2 || len(h.bindings) != 2 {
		t.Fatalf("starts=%d snapshots=%d bindings=%d", len(starts), len(h.snapshots), len(h.bindings))
	}
	if h.snapshots[0] != h.snapshots[1] {
		t.Fatal("fresh fallback minted a different logical snapshot request")
	}
	for i, start := range starts {
		prompt, input := hostStartPrompt(t, start)
		var handoff protocol.CollaborationHandoffEnvelope
		if err := json.Unmarshal(input["handoff"], &handoff); err != nil {
			t.Fatal(err)
		}
		if handoff.SnapshotID != h.snapshots[0].RequestID || h.bindings[i].SnapshotID != handoff.SnapshotID || h.bindings[i].PromptSHA256 != protocol.CollaborationHandoffHash(prompt) {
			t.Fatalf("launch %d lost snapshot or exact input binding", i+1)
		}
	}
	if h.bindings[0].AttemptNonce == h.bindings[1].AttemptNonce || h.bindings[0].PromptSHA256 == h.bindings[1].PromptSHA256 {
		t.Fatal("fresh fallback failed to bind its own changed continuity prompt")
	}
}

func TestHostHandoffLaunchRejectsAssignmentThatDiffersFromClaim(t *testing.T) {
	h := newLaunchHandoffFixture(t, "result")
	h.mutateSnapshot = func(v *protocol.CollaborationHandoffEnvelope) { v.Assignment.Role = "leader" }
	err := h.runner.Run(t.Context(), h.claim.Identity.RunID, h.request)
	if !errors.Is(err, ErrProtocol) || len(h.starts(t)) != 0 {
		t.Fatalf("mismatched assignment launched: err=%v starts=%d", err, len(h.starts(t)))
	}
}

func TestHostHandoffLaunchSnapshotFailuresNeverStartTheProvider(t *testing.T) {
	for _, tc := range []struct {
		name      string
		mutate    func(*protocol.CollaborationHandoffEnvelope)
		failures  []HTTPError
		wantReads int
	}{
		{name: "wrong_issue", mutate: func(v *protocol.CollaborationHandoffEnvelope) { v.IssueID = uuid.NewString() }, wantReads: 1},
		{name: "wrong_run", mutate: func(v *protocol.CollaborationHandoffEnvelope) { v.RunID = uuid.NewString() }, wantReads: 1},
		{name: "wrong_snapshot", mutate: func(v *protocol.CollaborationHandoffEnvelope) { v.SnapshotID = uuid.NewString() }, wantReads: 1},
		{name: "unknown_version", mutate: func(v *protocol.CollaborationHandoffEnvelope) { v.ProtocolVersion = 2 }, wantReads: 1},
		{name: "live_instead_of_snapshot", mutate: func(v *protocol.CollaborationHandoffEnvelope) { v.Mode = "live" }, wantReads: 1},
		{name: "wrong_step", mutate: func(v *protocol.CollaborationHandoffEnvelope) { v.Assignment.StepKey = "other-step" }, wantReads: 1},
		{name: "wrong_parent", mutate: func(v *protocol.CollaborationHandoffEnvelope) { v.Assignment.ParentRunID = uuid.NewString() }, wantReads: 1},
		{name: "missing_delegation", mutate: func(v *protocol.CollaborationHandoffEnvelope) { v.Sources = v.Sources[:1] }, wantReads: 1},
		{name: "extra_executable_delegation", mutate: func(v *protocol.CollaborationHandoffEnvelope) {
			source := v.Sources[1]
			source.ID = uuid.NewString()
			v.Sources = append(v.Sources, source)
		}, wantReads: 1},
		{name: "incomplete_delegation", mutate: func(v *protocol.CollaborationHandoffEnvelope) { v.Sources[1].Truncated = true }, wantReads: 1},
		{name: "wrong_memory_hash", mutate: func(v *protocol.CollaborationHandoffEnvelope) { v.Memory.SHA256 = strings.Repeat("a", 64) }, wantReads: 1},
		{name: "memory_marked_unsupported", mutate: func(v *protocol.CollaborationHandoffEnvelope) { v.Memory.Availability = "unsupported" }, wantReads: 1},
		{name: "refused", failures: []HTTPError{{Status: 403, Code: "business_capability_required"}}, wantReads: 1},
		{name: "declared_but_unsupported", failures: []HTTPError{{Status: 409, Code: "handoff_unsupported"}}, wantReads: 1},
		{name: "unavailable", failures: []HTTPError{{Status: 503}, {Status: 503}}, wantReads: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newLaunchHandoffFixture(t, "result")
			h.mutateSnapshot, h.snapshotFailures = tc.mutate, tc.failures
			if err := h.runner.Run(t.Context(), h.claim.Identity.RunID, h.request); err == nil {
				t.Fatal("invalid supported handoff succeeded")
			}
			if len(h.starts(t)) != 0 || len(h.bindings) != 0 || len(h.snapshots) != tc.wantReads {
				t.Fatalf("starts=%d bindings=%d reads=%d", len(h.starts(t)), len(h.bindings), len(h.snapshots))
			}
			if end, ok := h.terminations["1"]; !ok || end.Kind != "not_started" || len(h.proofs) != 1 {
				t.Fatal("unlaunched handoff failure lost original proof cleanup")
			}
		})
	}
}

func TestHostHandoffLaunchBindingFailuresNeverStartTheProvider(t *testing.T) {
	for _, tc := range []struct {
		name         string
		mutate       func(*protocol.CollaborationHandoffBindingReceipt)
		failures     []HTTPError
		wantBindings int
	}{
		{name: "wrong_issue", mutate: func(v *protocol.CollaborationHandoffBindingReceipt) { v.IssueID = uuid.NewString() }, wantBindings: 1},
		{name: "wrong_run", mutate: func(v *protocol.CollaborationHandoffBindingReceipt) { v.RunID = uuid.NewString() }, wantBindings: 1},
		{name: "wrong_snapshot", mutate: func(v *protocol.CollaborationHandoffBindingReceipt) { v.SnapshotID = uuid.NewString() }, wantBindings: 1},
		{name: "wrong_hash", mutate: func(v *protocol.CollaborationHandoffBindingReceipt) { v.PromptSHA256 = strings.Repeat("a", 64) }, wantBindings: 1},
		{name: "wrong_nonce", mutate: func(v *protocol.CollaborationHandoffBindingReceipt) { v.AttemptNonce = uuid.NewString() }, wantBindings: 1},
		{name: "wrong_sequence", mutate: func(v *protocol.CollaborationHandoffBindingReceipt) { v.AttemptSeq = "2" }, wantBindings: 1},
		{name: "not_accepted", mutate: func(v *protocol.CollaborationHandoffBindingReceipt) { v.Accepted = false }, wantBindings: 1},
		{name: "unknown_version", mutate: func(v *protocol.CollaborationHandoffBindingReceipt) { v.ProtocolVersion = 2 }, wantBindings: 1},
		{name: "revoked", failures: []HTTPError{{Status: 403, Code: "business_capability_required"}}, wantBindings: 1},
		{name: "purged", failures: []HTTPError{{Status: 409, Code: "handoff_source_unavailable"}}, wantBindings: 1},
		{name: "unavailable", failures: []HTTPError{{Status: 503}, {Status: 503}}, wantBindings: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newLaunchHandoffFixture(t, "result")
			h.mutateReceipt, h.bindingFailures = tc.mutate, tc.failures
			_ = h.runner.Run(t.Context(), h.claim.Identity.RunID, h.request)
			if len(h.starts(t)) != 0 || len(h.bindings) != tc.wantBindings {
				t.Fatalf("starts=%d bindings=%d", len(h.starts(t)), len(h.bindings))
			}
			if end, ok := h.terminations["1"]; !ok || end.Kind != "not_started" || len(h.proofs) != 1 {
				t.Fatal("binding failure prevented original unlaunched cleanup")
			}
		})
	}
}

func TestHostHandoffLaunchLostRepliesReplayExactRequests(t *testing.T) {
	h := newLaunchHandoffFixture(t, "result")
	h.loseSnapshot, h.loseBinding = true, true
	if err := h.runner.Run(t.Context(), h.claim.Identity.RunID, h.request); err != nil {
		t.Fatal(err)
	}
	if len(h.snapshots) != 2 || h.snapshots[0] != h.snapshots[1] || len(h.bindings) != 2 || h.bindings[0] != h.bindings[1] || len(h.starts(t)) != 1 {
		t.Fatal("uncertain replies changed identity/payload or repeated execution")
	}
}

func TestHostHandoffLaunchUnsupportedClaimKeepsLegacyPath(t *testing.T) {
	h := newLaunchHandoffFixture(t, "result")
	h.claim.HandoffVersion = 0
	if err := h.runner.Run(t.Context(), h.claim.Identity.RunID, h.request); err != nil {
		t.Fatal(err)
	}
	starts := h.starts(t)
	if len(starts) != 1 || len(h.snapshots) != 0 || len(h.bindings) != 0 {
		t.Fatal("legacy claim entered negotiated handoff")
	}
	_, input := hostStartPrompt(t, starts[0])
	if _, exists := input["handoff"]; exists {
		t.Fatal("unsupported claim fabricated an empty handoff")
	}
}

func TestHostHandoffLaunchAcceptsRedactedSourceWithOriginalHash(t *testing.T) {
	h := newLaunchHandoffFixture(t, "result")
	h.envelope.Sources[1].SHA256 = protocol.CollaborationHandoffHash("original assignment before redaction")
	h.envelope.Sources[1].Content = "Only execute step B; credential [REDACTED]."
	if err := h.runner.Run(t.Context(), h.claim.Identity.RunID, h.request); err != nil {
		t.Fatal(err)
	}
	if len(h.starts(t)) != 1 {
		t.Fatal("valid redacted assignment was refused")
	}
}

func TestHostHandoffLaunchFreshFallbackRejectsSnapshotDrift(t *testing.T) {
	h := newLaunchHandoffFixture(t, "reject_resume")
	seedHostResume(t, h.hostRunFixture)
	h.mutateSnapshot = func(v *protocol.CollaborationHandoffEnvelope) {
		if len(h.snapshots) > 1 {
			v.Authority.HandlerID = uuid.NewString()
		}
	}
	if err := h.runner.Run(t.Context(), h.claim.Identity.RunID, h.request); !errors.Is(err, ErrProtocol) {
		t.Fatalf("snapshot drift was accepted: %v", err)
	}
	if len(h.starts(t)) != 1 || len(h.bindings) != 1 || h.snapshots[0].RequestID != h.snapshots[1].RequestID {
		t.Fatal("fresh fallback silently accepted new authority or started")
	}
	if _, exists := h.permits["2"]; exists {
		t.Fatal("fallback requested a new permit after its snapshot changed")
	}
}

func TestHostHandoffLaunchKeepsUnsupportedAndEmptyMemoryDistinct(t *testing.T) {
	for _, availability := range []string{"unsupported", "absent"} {
		t.Run(availability, func(t *testing.T) {
			h := newLaunchHandoffFixture(t, "result")
			h.envelope.Memory = protocol.CollaborationHandoffMemory{Version: "0", Availability: availability}
			h.memoryVersion, h.memoryContent = "0", ""
			wantVersion := "0"
			if availability == "unsupported" {
				h.memoryFailures = []HTTPError{{Status: 409, Code: "memory_unsupported"}}
				wantVersion = ""
			}
			if err := h.runner.Run(t.Context(), h.claim.Identity.RunID, h.request); err != nil {
				t.Fatal(err)
			}
			if len(h.snapshots) != 1 || h.snapshots[0].MemoryVersion != wantVersion || len(h.starts(t)) != 1 {
				t.Fatal("memory absence was confused with unsupported legacy memory")
			}
		})
	}
}

func TestHostHandoffLaunchFreshMemoryReanchorCannotRemintSnapshot(t *testing.T) {
	h := newLaunchHandoffFixture(t, "reject_resume")
	seedHostResume(t, h.hostRunFixture)
	h.mutateReceipt = func(receipt *protocol.CollaborationHandoffBindingReceipt) {
		if receipt.AttemptSeq != "1" {
			return
		}
		h.hostRunFixture.mu.Lock()
		h.memoryVersion, h.memoryContent = "4", "New memory after the old version was purged."
		h.hostRunFixture.mu.Unlock()
		h.envelope.Memory = protocol.CollaborationHandoffMemory{Version: "4", SHA256: protocol.CollaborationHandoffHash(h.memoryContent), Availability: "available"}
	}
	if err := h.runner.Run(t.Context(), h.claim.Identity.RunID, h.request); !errors.Is(err, ErrProtocol) {
		t.Fatalf("memory reanchor changed the existing snapshot: %v", err)
	}
	if len(h.snapshots) != 2 || h.snapshots[0].RequestID != h.snapshots[1].RequestID || h.snapshots[0].MemoryVersion != "3" || h.snapshots[1].MemoryVersion != "4" {
		t.Fatal("fallback reminted its logical request after memory reanchor")
	}
	if len(h.starts(t)) != 1 || len(h.bindings) != 1 {
		t.Fatal("fallback launched with facts from a different memory version")
	}
}

func TestHostHandoffLaunchGitHelpersDoNotBindProviderInput(t *testing.T) {
	h := newLaunchHandoffFixture(t, "result")
	h.runner.Host.Config.Git = hostGitFixture(t, false)
	caps, err := h.runner.Host.Config.Capabilities()
	if err != nil {
		t.Fatal(err)
	}
	digest, err := protocol.CollaborationRuntimeCapabilityDigest(caps)
	if err != nil {
		t.Fatal(err)
	}
	h.request.CapabilityDigest = digest
	h.claim.Preparation.Host, h.claim.Preparation.Git, h.claim.Preparation.CapabilityDigest = caps.Host, caps.Git, digest
	if err := protocol.SealCollaborationPreparation(h.claim.Preparation); err != nil {
		t.Fatal(err)
	}
	if err := h.runner.Run(t.Context(), h.claim.Identity.RunID, h.request); err != nil {
		t.Fatal(err)
	}
	if len(h.bindings) != 1 || len(h.starts(t)) != 1 || h.bindings[0].AttemptSeq == "1" {
		t.Fatalf("Git preparation was bound as a provider launch: %+v", h.bindings)
	}
	for seq, permit := range h.permits {
		if seq == h.bindings[0].AttemptSeq && permit.LaunchDescriptor.Kind != "provider" || seq != h.bindings[0].AttemptSeq && permit.LaunchDescriptor.Kind != "preparation" {
			t.Fatal("launch identity does not match recorded kind")
		}
	}
}
