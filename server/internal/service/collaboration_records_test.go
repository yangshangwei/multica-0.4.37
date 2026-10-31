package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func recordedOutcome(claim protocol.CollaborationRunClaimResponse, permit protocol.CollaborationRunPermitResponse, count int) protocol.CollaborationRunOutcome {
	zero, large, cost := "0", "9007199254740993", "0.0123456789"
	notError := false
	messages := make([]protocol.CollaborationRecordedMessage, 0, count+2)
	for i := 0; i < count; i++ {
		messages = append(messages, protocol.CollaborationRecordedMessage{Kind: "message", Role: "assistant", Body: fmt.Sprintf("Recorded response %d", i)})
	}
	messages = append(messages, protocol.CollaborationRecordedMessage{Kind: "tool_call", CallID: "read-1", Name: "read", Body: "explicit tool input"}, protocol.CollaborationRecordedMessage{Kind: "tool_result", CallID: "read-1", Body: "explicit tool result", IsError: &notError})
	return protocol.CollaborationRunOutcome{Identity: claim.Identity, EventID: uuid.NewString(), AttemptSeq: permit.AttemptSeq, AttemptNonce: permit.AttemptNonce, Outcome: "success", Body: "final recorded result", Record: &protocol.CollaborationRunRecord{Version: 1, Completeness: "partial", Messages: messages, Usage: &protocol.CollaborationRunUsage{InputTokens: &large, OutputTokens: &zero, CostUSD: &cost}}}
}

func TestCollaborationRunRecordsPagingFrozenInputsAndRetention(t *testing.T) {
	f := newManagedCollaborationFixture(t)
	first := f.submitManaged(t, f.creator, "first immutable input")
	second := f.submitManaged(t, f.handler, "second immutable input")
	f.resumeManaged(t)
	runID, _, claim := f.claimManaged(t)
	permit, err := f.svc.PermitRuntimeRun(f.daemonCtx, runID, protocol.CollaborationRunPermitRequest{Identity: claim.Identity, AttemptSeq: "1", AttemptNonce: uuid.NewString(), Containment: protocol.CollaborationContainmentLinuxPIDNamespace})
	if err != nil {
		t.Fatal(err)
	}
	f.submitManaged(t, f.handler, "later waiting input must not appear")
	internalEvent := func(kind string) {
		t.Helper()
		_, err := f.svc.Queries.CreateCollaborationBusinessEvent(f.ctx, db.CreateCollaborationBusinessEventParams{IssueID: f.issue.Issue.ID, RunID: runID, EventID: uuid.NewString(), Kind: kind, Payload: json.RawMessage(`{"private":"PRIVATE-INTERNAL"}`)})
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, kind := range []string{"handoff_snapshot", "handoff_binding", "wakeup_pending", "wakeup_assigned"} {
		internalEvent(kind)
	}
	outcome := recordedOutcome(claim, permit, 65)
	if err = f.svc.RecordRuntimeOutcome(context.Background(), runID, permit.BusinessToken, outcome); err != nil {
		t.Fatal(err)
	}
	internalEvent("wakeup_assigned")
	if err = f.svc.RecordRuntimeOutcome(context.Background(), runID, permit.BusinessToken, outcome); err != nil {
		t.Fatal(err)
	}
	// A new allocation of equal typed data is still an exact replay.
	raw, _ := json.Marshal(outcome)
	var replay protocol.CollaborationRunOutcome
	if err = json.Unmarshal(raw, &replay); err != nil {
		t.Fatal(err)
	}
	if err = f.svc.RecordRuntimeOutcome(context.Background(), runID, permit.BusinessToken, replay); err != nil {
		t.Fatal(err)
	}
	changed := "1"
	replay.Record.Usage.OutputTokens = &changed
	if err = f.svc.RecordRuntimeOutcome(context.Background(), runID, permit.BusinessToken, replay); !collaborationErrorCode(err, "outcome_conflict") {
		t.Fatalf("changed recorded usage replaced immutable outcome: %v", err)
	}
	viewer := collaborationPasswordContext(util.MustParseUUID(f.handler), 1)
	page, err := f.svc.RunRecords(viewer, f.issue.Issue.ID, runID, CollaborationRunRecordsQuery{Limit: 9})
	if err != nil {
		t.Fatal(err)
	}
	if page.Run.ID != util.UUIDToString(runID) || page.Run.GroupID != claim.Identity.GroupID || page.Recording.Status != "partial" || !page.Recording.InputsRecorded || !page.Run.RecordingIncomplete || page.Usage == nil || *page.Usage.InputTokens != "9007199254740993" || *page.Usage.OutputTokens != "0" || page.Usage.TotalTokens != nil {
		t.Fatalf("lost recording/usage distinction: %+v", page)
	}
	if page.Resource.Provider == nil || *page.Resource.Provider != "codex" || page.Resource.ResourceOwnerUserID != f.creator {
		t.Fatalf("lost frozen resource provenance: %+v", page.Resource)
	}
	items := append([]CollaborationRunRecordItem(nil), page.Items...)
	// A later source event must not move the captured page's upper boundary.
	commentID := uuid.NewString()
	_, err = f.svc.CreateRuntimeComment(context.Background(), runID, permit.BusinessToken, CollaborationRuntimeCommentParams{CommentID: util.MustParseUUID(commentID), Content: "later recorded comment"})
	if err != nil {
		t.Fatal(err)
	}
	internalEvent("handoff_binding")
	for page.NextCursor != nil {
		page, err = f.svc.RunRecords(viewer, f.issue.Issue.ID, runID, CollaborationRunRecordsQuery{Limit: 9, Cursor: *page.NextCursor})
		if err != nil {
			t.Fatal(err)
		}
		items = append(items, page.Items...)
	}
	if len(items) != 70 || items[0].CommandID != util.UUIDToString(first.CommandID) || items[1].CommandID != util.UUIDToString(second.CommandID) || items[0].Body != "first immutable input" || items[1].AuthorUserID != f.handler || items[len(items)-1].Kind != "result" {
		t.Fatalf("paging lost or invented frozen records: %d entries", len(items))
	}
	seen := map[string]bool{}
	for _, item := range items {
		if seen[item.ID] || strings.Contains(item.Body, "later") {
			t.Fatal("captured history duplicated or spliced a newer input/event")
		}
		seen[item.ID] = true
	}
	if _, err = f.svc.AcceptRuntimeProof(context.Background(), runID, claim.TerminationToken, managedProof(claim.Identity, permit.AttemptNonce)); err != nil {
		t.Fatal(err)
	}
	// Source-owned identity/provenance remains readable after engine cleanup and
	// a resource rename/removal. No live resource is joined by the record reader.
	f.fx.Exec(t, `UPDATE agent_runtime SET provider='changed-provider',name='PRIVATE-REPLACEMENT' WHERE id=$1`, f.runtime.ID)
	f.fx.Exec(t, `DELETE FROM agent_task_queue WHERE id=(SELECT task_id FROM collaboration_run WHERE id=$1)`, runID)
	f.fx.Exec(t, `UPDATE agent SET runtime_id=NULL WHERE runtime_id=$1`, f.runtime.ID)
	f.fx.Exec(t, `DELETE FROM agent_runtime WHERE id=$1`, f.runtime.ID)
	page, err = f.svc.RunRecords(viewer, f.issue.Issue.ID, runID, CollaborationRunRecordsQuery{Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 71 || page.Recording.Status != "partial" || !page.Run.RecordingIncomplete || *page.Resource.Provider != "codex" {
		t.Fatal("proof/engine cleanup rewrote immutable recording or erased partial coverage")
	}
	encoded, _ := json.Marshal(page)
	for _, private := range []string{"launch_nonce", "installation_id", "binding_id", "mcb_", "mct_", "PRIVATE-REPLACEMENT", "PRIVATE-INTERNAL", "later waiting input"} {
		if strings.Contains(string(encoded), private) {
			t.Fatalf("record projection leaked %s", private)
		}
	}
}

func TestCollaborationRunRecordsCurrentAuthorityAndLegacyAbsence(t *testing.T) {
	f := newManagedCollaborationFixture(t)
	f.submitManaged(t, f.creator, "legacy input")
	f.resumeManaged(t)
	runID, _, claim := f.claimManaged(t)
	permit, err := f.svc.PermitRuntimeRun(f.daemonCtx, runID, protocol.CollaborationRunPermitRequest{Identity: claim.Identity, AttemptSeq: "1", AttemptNonce: uuid.NewString(), Containment: protocol.CollaborationContainmentLinuxPIDNamespace})
	if err != nil {
		t.Fatal(err)
	}
	if err = f.svc.RecordRuntimeOutcome(context.Background(), runID, permit.BusinessToken, protocol.CollaborationRunOutcome{Identity: claim.Identity, EventID: uuid.NewString(), AttemptSeq: "1", AttemptNonce: permit.AttemptNonce, Outcome: "success", Body: "historic result without telemetry"}); err != nil {
		t.Fatal(err)
	}
	viewer := collaborationPasswordContext(util.MustParseUUID(f.handler), 1)
	page, err := f.svc.RunRecords(viewer, f.issue.Issue.ID, runID, CollaborationRunRecordsQuery{Limit: 1})
	if err != nil || page.Recording.Status != "not_recorded" || page.Usage != nil || page.NextCursor == nil {
		t.Fatalf("legacy telemetry was invented: %+v %v", page, err)
	}
	cursor := *page.NextCursor
	invalidQueries := []CollaborationRunRecordsQuery{{Limit: 0}, {Limit: 101}, {Limit: 1, Cursor: "malformed"}, {Limit: 1, Cursor: "e30"}, {Limit: 1, Cursor: strings.Repeat("a", 1025)}}
	for _, omitted := range []string{"run_id", "through_seq", "input_offset", "item_offset"} {
		raw, err := base64.RawURLEncoding.DecodeString(cursor)
		if err != nil {
			t.Fatal(err)
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(raw, &fields); err != nil {
			t.Fatal(err)
		}
		delete(fields, omitted)
		raw, err = json.Marshal(fields)
		if err != nil {
			t.Fatal(err)
		}
		invalidQueries = append(invalidQueries, CollaborationRunRecordsQuery{Limit: 1, Cursor: base64.RawURLEncoding.EncodeToString(raw)})
	}
	for _, p := range invalidQueries {
		if _, err = f.svc.RunRecords(viewer, f.issue.Issue.ID, runID, p); !collaborationErrorCode(err, "invalid_records_query") {
			t.Fatalf("invalid paging was accepted: %v", err)
		}
	}
	if _, err = f.svc.RunRecords(viewer, f.issue.Issue.ID, util.MustParseUUID(uuid.NewString()), CollaborationRunRecordsQuery{Limit: 1}); !collaborationErrorCode(err, "run_not_found") {
		t.Fatalf("missing run confused current issue authority: %v", err)
	}
	machine := auth.WithAuthenticatedPrincipal(viewer, auth.AuthenticatedPrincipal{UserID: f.handler, Kind: auth.CredentialTask})
	if _, err = f.svc.RunRecords(machine, f.issue.Issue.ID, runID, CollaborationRunRecordsQuery{Limit: 1, Cursor: cursor}); !collaborationErrorCode(err, "not_found") {
		t.Fatalf("machine received source run recording: %v", err)
	}
	f.fx.Exec(t, `UPDATE issue_participant SET revoked_at=now() WHERE issue_id=$1 AND user_id=$2`, f.issue.Issue.ID, f.handler)
	if _, err = f.svc.RunRecords(viewer, f.issue.Issue.ID, runID, CollaborationRunRecordsQuery{Limit: 1, Cursor: cursor}); !collaborationErrorCode(err, "not_found") {
		t.Fatalf("old cursor retained revoked content access: %v", err)
	}
	f.fx.Member(t, f.workspace, f.handler, "member")
	page, err = f.svc.RunRecords(viewer, f.issue.Issue.ID, runID, CollaborationRunRecordsQuery{Limit: 1, Cursor: cursor})
	if err != nil || len(page.Items) != 1 || page.Items[0].Kind != "result" {
		t.Fatalf("source-only member lost independent historical read basis: %+v %v", page, err)
	}
	f.fx.Exec(t, `DELETE FROM collaboration_run_event WHERE run_id=$1 AND kind='resource_snapshot'`, runID)
	// Simulate provenance which was never recorded without consulting current
	// provider configuration or resource names.
	page, err = f.svc.RunRecords(viewer, f.issue.Issue.ID, runID, CollaborationRunRecordsQuery{Limit: 50})
	if err != nil || !page.Recording.InputsRecorded || page.Resource.Provider != nil || page.Resource.AccountSource != nil || len(page.Items) != 2 {
		t.Fatalf("absent history was reconstructed from current data: %+v %v", page, err)
	}
	legacyItems := page.Items
	f.fx.Insert(t, "collaboration_run_event", testutil.Cols{"issue_id": f.issue.Issue.ID, "run_id": runID, "producer_event_id": "unknown-record", "seq": 99, "kind": "unknown_private", "payload": json.RawMessage(`{"secret":"PRIVATE-UNKNOWN"}`)})
	// The explicit display-event allowlist withholds unrelated metadata. It
	// must not infer recording completeness or leak a new kind through paging.
	items := []CollaborationRunRecordItem{}
	p := CollaborationRunRecordsQuery{Limit: 1}
	for pages := 0; ; pages++ {
		if pages >= 3 {
			t.Fatal("excluded private event prevented paging from advancing")
		}
		page, err = f.svc.RunRecords(viewer, f.issue.Issue.ID, runID, p)
		if err != nil || page.Recording.Status != "not_recorded" || page.Recording.Retention != nil || page.Usage != nil {
			t.Fatalf("unknown event changed legacy recording coverage: %+v: %v", page.Recording, err)
		}
		encoded, err := json.Marshal(page)
		if err != nil || strings.Contains(string(encoded), "PRIVATE-UNKNOWN") || strings.Contains(string(encoded), "unknown_private") {
			t.Fatal("unknown stored payload escaped the display allowlist")
		}
		items = append(items, page.Items...)
		if page.NextCursor == nil {
			break
		}
		p.Cursor = *page.NextCursor
	}
	if !reflect.DeepEqual(items, legacyItems) {
		t.Fatal("internal metadata changed, duplicated or removed retained display items")
	}
}

func TestCollaborationRunRecordsMalformedDisplayPayloadRemainsRejected(t *testing.T) {
	for _, kind := range []string{"outcome", "record_checkpoint", "comment", "squad_activity"} {
		t.Run(kind, func(t *testing.T) {
			f := newManagedCollaborationFixture(t)
			f.submitManaged(t, f.creator, "retained input before malformed display event")
			f.resumeManaged(t)
			run, _, _ := f.claimManaged(t)
			f.fx.Insert(t, "collaboration_run_event", testutil.Cols{"issue_id": f.issue.Issue.ID, "run_id": run, "producer_event_id": "malformed-display", "seq": 99, "kind": kind, "payload": json.RawMessage(`{"private":"PRIVATE-MALFORMED"}`)})
			page, err := f.svc.RunRecords(f.ctx, f.issue.Issue.ID, run, CollaborationRunRecordsQuery{Limit: 100})
			encoded, marshalErr := json.Marshal(page)
			if err == nil || strings.Contains(err.Error(), "PRIVATE-MALFORMED") || marshalErr != nil || strings.Contains(string(encoded), "PRIVATE-MALFORMED") {
				t.Fatal("malformed display payload was ignored, marked complete or leaked")
			}
		})
	}
}

func TestCollaborationRunRecordsUnclaimedInputsAreNotInvented(t *testing.T) {
	f := newManagedCollaborationFixture(t)
	f.submitManaged(t, f.creator, "queued content is not a recorded prompt")
	f.resumeManaged(t)
	runs, err := f.svc.ListRuntimeRuns(f.daemonCtx, f.runtime.ID)
	if err != nil || len(runs.RunIDs) != 1 {
		t.Fatalf("prepare: %+v %v", runs, err)
	}
	page, err := f.svc.RunRecords(f.ctx, f.issue.Issue.ID, util.MustParseUUID(runs.RunIDs[0]), CollaborationRunRecordsQuery{Limit: 50})
	if err != nil || page.Recording.InputsRecorded || page.Recording.Status != "not_recorded" || page.Usage != nil || page.Resource.Provider != nil || len(page.Items) != 0 {
		t.Fatalf("unclaimed batch was invented as this run's recorded input: %+v %v", page, err)
	}
}
