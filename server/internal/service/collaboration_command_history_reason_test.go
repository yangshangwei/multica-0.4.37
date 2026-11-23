package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestCollaborationCommandHistoryReasonPreservesNullableValues(t *testing.T) {
	for _, tc := range []struct {
		name     string
		reason   pgtype.Text
		want     string
		wantNull bool
	}{
		{name: "sql_null", wantNull: true},
		{name: "sql_null_with_stale_value", reason: pgtype.Text{String: "stale", Valid: false}, wantNull: true},
		{name: "valid_empty", reason: pgtype.Text{Valid: true}, wantNull: true},
		{name: "cancellation", reason: pgtype.Text{String: "cancelled_by_participant", Valid: true}, want: "cancelled_by_participant"},
		{name: "failure", reason: pgtype.Text{String: "runtime_capability_missing", Valid: true}, want: "runtime_capability_missing"},
		{name: "whitespace", reason: pgtype.Text{String: " \t ", Valid: true}, want: " \t "},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := collaborationCommandHistoryReason(tc.reason)
			if tc.wantNull {
				if got != nil {
					t.Fatalf("absent history reason must be nil, got %q", *got)
				}
				return
			}
			if got == nil || *got != tc.want {
				t.Fatalf("nonempty history reason must remain exact: got %v, want %q", got, tc.want)
			}
		})
	}
}

func TestCollaborationCommandHistoryReasonFromSucceededManagedCommand(t *testing.T) {
	f := newManagedCollaborationFixture(t)
	input := f.submitManaged(t, f.creator, "managed command with no terminal reason")
	f.resumeManaged(t)
	runID, _, claim := f.claimManaged(t)
	nonce := uuid.NewString()
	permit, err := f.svc.PermitRuntimeRun(f.daemonCtx, runID, protocol.CollaborationRunPermitRequest{
		Identity: claim.Identity, AttemptSeq: "1", AttemptNonce: nonce, Containment: protocol.CollaborationContainmentLinuxPIDNamespace,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = f.svc.RecordRuntimeOutcome(context.Background(), runID, permit.BusinessToken, protocol.CollaborationRunOutcome{
		Identity: claim.Identity, EventID: uuid.NewString(), AttemptSeq: "1", AttemptNonce: nonce, Outcome: "success", Body: "completed without a reason",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.AcceptRuntimeProof(context.Background(), runID, claim.TerminationToken, managedProof(claim.Identity, nonce)); err != nil {
		t.Fatal(err)
	}
	var state string
	var reason pgtype.Text
	f.fx.QueryRow(t, `SELECT state,reason FROM collaboration_command WHERE id=$1 AND session_id=$2`, input.CommandID, f.issue.SessionID).Scan(&state, &reason)
	if state != "succeeded" || !reason.Valid || reason.String != "" {
		t.Fatalf("managed success did not persist the valid empty terminal reason: state=%q reason=%+v", state, reason)
	}
	wire, err := json.Marshal(struct {
		Reason *string `json:"reason"`
	}{Reason: collaborationCommandHistoryReason(reason)})
	if err != nil {
		t.Fatal(err)
	}
	if string(wire) != `{"reason":null}` {
		t.Fatalf("persisted empty terminal reason must project JSON null: %s", wire)
	}
}
