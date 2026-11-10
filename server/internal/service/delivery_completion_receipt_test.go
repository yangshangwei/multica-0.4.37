package service

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func completionReceiptFixture(kind string) (db.DeliveryCompletionAuthorization, db.DeliveryCompletionTerminal) {
	id := func() pgtype.UUID { return completionUUIDPG(uuid.NewString()) }
	row := db.DeliveryCompletionAuthorization{ID: id(), IssueID: id(), OperationID: id(), IssueRevision: 4, TargetStatus: "done", TargetKind: "ordinary", ConsumerKind: "human", ConsumerUserID: id()}
	terminal := db.DeliveryCompletionTerminal{AuthorizationID: row.ID, Kind: "consumed", OperationID: row.OperationID, ActorKind: "human", ActorUserID: row.ConsumerUserID,
		MutationDigest: completionText(strings.Repeat("a", 64)), IssueRevisionBefore: completionInt(4), IssueRevisionAfter: completionInt(5), StatusAfter: completionText("done"), RecordedAt: completionTime(time.Now())}
	switch kind {
	case "task":
		row.ConsumerKind, row.ConsumerUserID, row.ConsumerAgentID = "task", pgtype.UUID{}, id()
		terminal.ActorKind, terminal.ActorAgentID = "task", row.ConsumerAgentID
	case "github", "vcs":
		row.ConsumerKind, row.ConsumerUserID, row.IntegrationID = kind, pgtype.UUID{}, id()
		terminal.ActorKind, terminal.ActorUserID, terminal.IntegrationID = kind, pgtype.UUID{}, row.IntegrationID
		terminal.ProviderEventID, terminal.ProviderEventDigest = completionText("exact-event"), completionText(strings.Repeat("b", 64))
	case "collaboration_self", "collaboration_submission", "collaboration_admin":
		row.TargetKind, row.CollaborationRevision = kind, completionInt(8)
		terminal.CollaborationOperationID, terminal.ResultCollaborationRevision = id(), completionInt(9)
		if kind == "collaboration_admin" {
			row.ConsumerKind, terminal.ActorKind, terminal.AdminOperationID = "admin", "admin", id()
		} else {
			terminal.ResultSubmissionID = id()
			if kind == "collaboration_submission" {
				row.SubmissionID = terminal.ResultSubmissionID
			}
		}
	}
	return row, terminal
}

func TestDeliveryCompletionReceiptRejectsMismatchedRetainedFacts(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*db.DeliveryCompletionTerminal)
	}{
		{"different_authorization", func(t *db.DeliveryCompletionTerminal) { t.AuthorizationID = completionUUIDPG(uuid.NewString()) }},
		{"missing_before", func(t *db.DeliveryCompletionTerminal) { t.IssueRevisionBefore = pgtype.Int8{} }},
		{"different_before", func(t *db.DeliveryCompletionTerminal) { t.IssueRevisionBefore = completionInt(3) }},
		{"unchanged_after", func(t *db.DeliveryCompletionTerminal) { t.IssueRevisionAfter = completionInt(4) }},
		{"older_after", func(t *db.DeliveryCompletionTerminal) { t.IssueRevisionAfter = completionInt(3) }},
		{"different_actor", func(t *db.DeliveryCompletionTerminal) { t.ActorUserID = completionUUIDPG(uuid.NewString()) }},
		{"missing_mutation", func(t *db.DeliveryCompletionTerminal) { t.MutationDigest = pgtype.Text{} }},
		{"unexpected_collaboration_operation", func(t *db.DeliveryCompletionTerminal) {
			t.CollaborationOperationID = completionUUIDPG(uuid.NewString())
		}},
		{"revocation_fields", func(t *db.DeliveryCompletionTerminal) { t.RevokerUserID = completionUUIDPG(uuid.NewString()) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			row, terminal := completionReceiptFixture("ordinary")
			tc.change(&terminal)
			if _, err := completionReceipt(row, terminal, true); err == nil {
				t.Fatal("mismatched retained fact became a successful completion receipt")
			}
		})
	}
}

func TestDeliveryCompletionReceiptValidatesClosedVariants(t *testing.T) {
	for _, kind := range []string{"ordinary", "task", "github", "vcs", "collaboration_self", "collaboration_submission", "collaboration_admin"} {
		t.Run(kind, func(t *testing.T) {
			row, terminal := completionReceiptFixture(kind)
			if _, err := completionReceipt(row, terminal, true); err != nil {
				t.Fatalf("exact retained variant refused: %v", err)
			}
			switch kind {
			case "ordinary":
				terminal.ProviderEventID = completionText("unrelated")
			case "task":
				terminal.ActorAgentID = completionUUIDPG(uuid.NewString())
			case "github", "vcs":
				terminal.IntegrationID = completionUUIDPG(uuid.NewString())
			case "collaboration_self":
				terminal.ResultSubmissionID = pgtype.UUID{}
			case "collaboration_submission":
				terminal.ResultSubmissionID = completionUUIDPG(uuid.NewString())
			case "collaboration_admin":
				terminal.AdminOperationID = pgtype.UUID{}
			}
			if _, err := completionReceipt(row, terminal, true); err == nil {
				t.Fatal("wrong variant operation/actor fact became a completion receipt")
			}
		})
	}
}
