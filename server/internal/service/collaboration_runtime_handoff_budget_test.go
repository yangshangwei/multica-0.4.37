package service

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestCollaborationHandoffSnapshotPreservesMaximumAssignment(t *testing.T) {
	for name, char := range map[string]string{"html": "<", "ampersand": "&", "control": "\x01", "multibyte": "中"} {
		t.Run(name, func(t *testing.T) {
			f, member := newManagedSquadFixture(t)
			f.submitManaged(t, f.creator, "original user intent")
			f.resumeManaged(t)
			root, request, claim := f.claimManaged(t)
			token, nonce := permitSquadRun(t, f, root, claim)
			assignment := "[@member](mention://agent/" + handoffID(member) + ") "
			assignment += strings.Repeat(char, ((64<<10)-len(assignment))/len(char))
			assignment += strings.Repeat("x", (64<<10)-len(assignment))
			receipt, err := f.svc.CreateRuntimeComment(context.Background(), root, token, CollaborationRuntimeCommentParams{CommentID: util.MustParseUUID(uuid.NewString()), Content: assignment})
			if err != nil {
				t.Fatal(err)
			}
			finishSquadRun(t, f, root, claim, token, nonce, "success")
			run := receipt.RunIDs[0]
			memberClaim, err := f.svc.ClaimRuntimeRun(f.daemonCtx, run, request)
			if err != nil {
				t.Fatal(err)
			}
			memberToken, _ := permitSquadRun(t, f, run, memberClaim)
			p := protocol.CollaborationHandoffSnapshotRequest{RequestID: uuid.NewString()}
			snapshot, err := f.svc.RuntimeHandoffSnapshot(context.Background(), run, memberToken, p)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, source := range snapshot.Sources {
				if source.Kind == "delegation" {
					found = source.Content == assignment && !source.Truncated && source.SHA256 == protocol.CollaborationHandoffHash(assignment)
				}
			}
			if !found {
				t.Fatal("admitted assignment was truncated or changed")
			}
			raw, err := json.Marshal(snapshot)
			if err != nil {
				t.Fatal(err)
			}
			material, wire, err := protocol.CollaborationHandoffSizes(snapshot)
			if err != nil || wire != len(raw)+1 || material > protocol.CollaborationHandoffMaxBytes || wire > protocol.CollaborationHandoffMaxJSONBytes {
				t.Fatalf("serialized budget disagrees: material=%d wire=%d raw=%d: %v", material, wire, len(raw), err)
			}
			replay, err := f.svc.RuntimeHandoffSnapshot(context.Background(), run, memberToken, p)
			again, _ := json.Marshal(replay)
			if err != nil || string(raw) != string(again) || protocol.CollaborationHandoffHash(string(raw)) != protocol.CollaborationHandoffHash(string(again)) {
				t.Fatalf("frozen source bytes/hash changed: %v", err)
			}
		})
	}
}

func TestCollaborationHandoffBudgetKeepsAssignmentAndSourceReferences(t *testing.T) {
	out := protocol.CollaborationHandoffEnvelope{ProtocolVersion: 1, IssueID: uuid.NewString(), RunID: uuid.NewString(), Mode: "live", EventSequence: "1", Authority: protocol.CollaborationHandoffAuthority{Status: "todo"}, Assignment: protocol.CollaborationHandoffAssignment{Role: "member"}, Memory: protocol.CollaborationHandoffMemory{Version: "0", Availability: "absent"}, Inputs: protocol.CollaborationHandoffInputs{Items: []protocol.CollaborationHandoffInput{}}, Results: protocol.CollaborationHandoffResults{Items: []protocol.CollaborationHandoffResult{}}, WakeupCauses: protocol.CollaborationHandoffCauses{Items: []protocol.CollaborationHandoffCause{}, Completeness: "recorded"}, Sources: []protocol.CollaborationHandoffSource{}, Missing: []string{}}
	out.Sources = append(out.Sources, handoffSource("delegation", uuid.NewString(), uuid.NewString(), "1", strings.Repeat("<", 64<<10), 64<<10))
	for i := 0; i < 32; i++ {
		input := protocol.CollaborationHandoffInput{CommandID: uuid.NewString(), AuthorID: uuid.NewString(), State: "consumed", ReaderRunIDs: []string{}}
		for j := 0; j < 32; j++ {
			input.ReaderRunIDs = append(input.ReaderRunIDs, uuid.NewString())
		}
		out.Inputs.Items = append(out.Inputs.Items, input)
		source := handoffSource("provider_result", uuid.NewString(), uuid.NewString(), "1", strings.Repeat("&", 4096), 4096)
		out.Sources = append(out.Sources, source)
		out.Results.Items = append(out.Results.Items, protocol.CollaborationHandoffResult{RunID: source.RunID, Role: "member", StepKey: "review", Progress: "ended", Outcome: "success", ProofState: "host_exit_observed", ResultState: "present", FinalAnswerState: "unknown", SourceID: source.ID, SourceKind: source.Kind})
		out.WakeupCauses.Items = append(out.WakeupCauses.Items, protocol.CollaborationHandoffCause{ID: uuid.NewString(), SourceKind: "proof", SourceID: uuid.NewString(), SourceRunID: uuid.NewString(), Sequence: "1"})
	}
	out.Inputs.Total, out.Inputs.Returned = 32, 32
	out.Results.Total, out.Results.Returned = 32, 32
	out.WakeupCauses.Total, out.WakeupCauses.Returned = 32, 32
	full := append([]protocol.CollaborationHandoffSource{}, out.Sources...)
	material, _, _ := protocol.CollaborationHandoffSizes(out)
	if material <= protocol.CollaborationHandoffMaxBytes {
		t.Fatal("fixture must exceed the material budget")
	}
	if err := boundCollaborationHandoff(&out); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(out.Sources[0], full[0]) || len(out.Sources) != len(full) {
		t.Fatal("budget lost an assignment or recall reference")
	}
	truncated := false
	for i, source := range out.Sources {
		if source.SHA256 != full[i].SHA256 || source.ID != full[i].ID || !strings.HasPrefix(full[i].Content, source.Content) || source.Truncated != (len(source.Content) != len(full[i].Content)) {
			t.Fatal("omitted source lost its precise prefix/hash/marker")
		}
		truncated = truncated || source.Truncated
	}
	if !truncated {
		t.Fatal("budget silently claimed full content")
	}
	// Snapshot framing consumes budget too. Restoring original source text and
	// applying the same deterministic bound must reproduce exact snapshot bytes.
	out.Mode, out.SnapshotID = "snapshot", uuid.NewString()
	if err := boundCollaborationHandoff(&out); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(out)
	out.Sources = full
	if err := boundCollaborationHandoff(&out); err != nil {
		t.Fatal(err)
	}
	again, _ := json.Marshal(out)
	if string(raw) != string(again) {
		t.Fatal("rehydration changed the frozen prefix allocation")
	}
	out.Sources[0].Content = strings.Repeat("x", protocol.CollaborationHandoffMaxBytes)
	if boundCollaborationHandoff(&out) == nil {
		t.Fatal("oversized mandatory assignment was silently shortened")
	}
}
