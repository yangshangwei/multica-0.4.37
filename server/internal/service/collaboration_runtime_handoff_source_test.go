package service

import (
	"context"
	"encoding/json"
	"reflect"
	"slices"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestCollaborationRuntimeHandoffTracksActiveInformationRequest(t *testing.T) {
	f, caps := newHostCollaborationFixture(t)
	f.fx.Cleanup(t, `DELETE FROM issue_handoff WHERE issue_id=$1`, f.issue.Issue.ID)
	f.fx.Cleanup(t, `DELETE FROM issue_status WHERE workspace_id=$1`, f.workspace)
	if err := f.svc.Queries.SeedIssueStatusEntries(f.ctx, util.MustParseUUID(f.workspace)); err != nil {
		t.Fatal(err)
	}
	submitHostCommand(t, f, f.handler, "keep working while the humans clarify")
	run, claim := claimHostRun(t, f, caps)
	permit, err := f.svc.PermitRuntimeRun(f.daemonCtx, run, hostPermitRequest(claim, 1, "provider"))
	if err != nil {
		t.Fatal(err)
	}
	base := f.collaborationQueueFixture
	ask := func(actor, target, note string) pgtype.UUID {
		t.Helper()
		access, err := f.svc.ResolveIssueAccess(f.ctx, nil, f.issue.Issue.ID)
		if err != nil {
			t.Fatal(err)
		}
		p := handoffAsk(t, base, access.Revision, target, note)
		p.ExecutionViewVersion = 6
		if _, err = f.svc.RequestInformation(collaborationPasswordContext(util.MustParseUUID(actor), 1), p); err != nil {
			t.Fatal(err)
		}
		return activeInformationRequest(t, base)
	}
	first := ask(f.handler, f.creator, "original parent question")
	var firstSeq int64
	f.fx.QueryRow(t, `SELECT event_seq FROM collaboration_event WHERE issue_id=$1 AND kind='information_requested' AND payload->>'handoff_id'=$2`, f.issue.Issue.ID, handoffID(first)).Scan(&firstSeq)
	check := func(request pgtype.UUID, wantSeq int64) protocol.CollaborationHandoffEnvelope {
		t.Helper()
		access, err := f.svc.ResolveIssueAccess(f.ctx, nil, f.issue.Issue.ID)
		if err != nil {
			t.Fatal(err)
		}
		live, err := f.svc.RuntimeHandoff(context.Background(), run, permit.BusinessToken, CollaborationHandoffQuery{})
		if err != nil {
			t.Fatal(err)
		}
		if access.EventSequence == 0 || live.EventSequence != handoffDecimal(access.EventSequence) {
			t.Fatalf("runtime watermark %q differs from locked session %d", live.EventSequence, access.EventSequence)
		}
		if live.InformationRequest == nil || live.InformationRequest.ID != handoffID(request) || live.InformationRequest.EventSequence != handoffDecimal(wantSeq) || live.InformationRequest.SourceID != handoffID(request) {
			t.Fatalf("active request lost its own event: %+v", live.InformationRequest)
		}
		human, err := f.svc.RunHandoff(f.ctx, f.issue.Issue.ID, run, CollaborationHandoffQuery{})
		if err != nil || !reflect.DeepEqual(human.InformationRequest, live.InformationRequest) || human.EventSequence != live.EventSequence {
			t.Fatalf("runtime and human provenance differ: %+v / %+v: %v", live.InformationRequest, human.InformationRequest, err)
		}
		p := protocol.CollaborationHandoffSnapshotRequest{RequestID: uuid.NewString()}
		snapshot, err := f.svc.RuntimeHandoffSnapshot(context.Background(), run, permit.BusinessToken, p)
		if err != nil || !reflect.DeepEqual(snapshot.InformationRequest, live.InformationRequest) || snapshot.EventSequence != live.EventSequence {
			t.Fatalf("snapshot omitted current provenance: %+v: %v", snapshot.InformationRequest, err)
		}
		return snapshot
	}
	frozen := check(first, firstSeq)
	if frozen.InformationRequest.RequesterID != f.handler || frozen.InformationRequest.ToID != f.creator || frozen.InformationRequest.AuthorID != f.handler {
		t.Fatalf("requester/recipient identity changed: %+v", frozen.InformationRequest)
	}
	second := ask(f.creator, f.handler, "nested clarification")
	access, err := f.svc.ResolveIssueAccess(f.ctx, nil, f.issue.Issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	back := handoffReturn(t, base, access.Revision, second, "blocked")
	back.ExecutionViewVersion = 6
	if _, err = f.svc.ReturnInformation(collaborationPasswordContext(util.MustParseUUID(f.handler), 1), back); err != nil {
		t.Fatal(err)
	}
	restored := check(first, firstSeq)
	if restored.Handoff == nil || restored.Handoff.Purpose != "information_return" || restored.Handoff.EventSequence == restored.InformationRequest.EventSequence {
		t.Fatal("the restored parent was mislabeled with the latest return event")
	}
	replay, err := f.svc.RuntimeHandoffSnapshot(context.Background(), run, permit.BusinessToken, protocol.CollaborationHandoffSnapshotRequest{RequestID: frozen.SnapshotID})
	before, _ := json.Marshal(frozen)
	after, _ := json.Marshal(replay)
	if err != nil || string(before) != string(after) {
		t.Fatalf("later requests rewrote the frozen snapshot: %v", err)
	}
	// A legacy request can lack an event link. Never borrow the latest event.
	legacy := f.fx.Insert(t, "issue_handoff", testutil.Cols{"issue_id": f.issue.Issue.ID, "actor_user_id": f.handler, "from_user_id": f.handler, "to_user_id": f.creator, "purpose": "information", "requester_user_id": f.handler, "previous_status": "blocked", "note": "legacy question"})
	f.fx.Exec(t, `UPDATE issue_collaboration SET active_information_request_id=$2 WHERE issue_id=$1`, f.issue.Issue.ID, legacy)
	unknown := check(util.MustParseUUID(legacy), 0)
	if !slices.Contains(unknown.Missing, "information_request_event_unknown:"+legacy) {
		t.Fatalf("missing provenance was not explicit: %v", unknown.Missing)
	}
}

func TestCollaborationRuntimeHandoffSourceRejectsFalseAttribution(t *testing.T) {
	f, caps := newHostCollaborationFixture(t)
	submitHostCommand(t, f, f.handler, "read authorized handoff notes")
	run, claim := claimHostRun(t, f, caps)
	permit, err := f.svc.PermitRuntimeRun(f.daemonCtx, run, hostPermitRequest(claim, 1, "provider"))
	if err != nil {
		t.Fatal(err)
	}
	body := "same issue handoff source"
	id := f.fx.Insert(t, "issue_handoff", testutil.Cols{"issue_id": f.issue.Issue.ID, "actor_user_id": f.creator, "from_user_id": f.creator, "to_user_id": f.handler, "purpose": "transfer", "previous_status": "todo", "note": body})
	source := protocol.CollaborationHandoffSource{Kind: "handoff", ID: id, SHA256: protocol.CollaborationHandoffHash(body)}
	page, err := f.svc.RuntimeHandoffSource(context.Background(), run, permit.BusinessToken, source, 0)
	if err != nil || page.Source.Content != body || page.Source.RunID != "" || page.Source.Version != "" || page.Source.Trust != "data_only" {
		t.Fatalf("valid handoff read: %+v: %v", page, err)
	}
	for _, fields := range [][2]string{{uuid.NewString(), "42"}, {uuid.NewString(), ""}, {"", "42"}} {
		forged := source
		forged.RunID, forged.Version = fields[0], fields[1]
		if _, err = f.svc.RuntimeHandoffSource(context.Background(), run, permit.BusinessToken, forged, 0); !collaborationErrorCode(err, "invalid_handoff_source") {
			t.Fatalf("accepted false run/version attribution %v: %v", fields, err)
		}
	}
	foreign := f.fx.Insert(t, "issue_handoff", testutil.Cols{"issue_id": uuid.NewString(), "actor_user_id": f.creator, "from_user_id": f.creator, "to_user_id": f.handler, "purpose": "transfer", "previous_status": "todo", "note": body})
	source.ID = foreign
	if _, err = f.svc.RuntimeHandoffSource(context.Background(), run, permit.BusinessToken, source, 0); err == nil {
		t.Fatal("source reference widened issue authority")
	}
}
