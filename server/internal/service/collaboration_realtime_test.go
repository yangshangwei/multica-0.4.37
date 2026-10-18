package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/realtime"
	"github.com/multica-ai/multica/server/internal/util"
)

type collaborationPublishedFrame struct {
	scope, id, event string
	data             []byte
}

type collaborationPublisher struct {
	frames []collaborationPublishedFrame
	fail   bool
}

func (p *collaborationPublisher) PublishWithID(scope, id, exclude string, frame []byte, event string) error {
	p.frames = append(p.frames, collaborationPublishedFrame{scope: scope, id: id, event: event, data: append([]byte(nil), frame...)})
	if p.fail {
		return errors.New("relay unavailable")
	}
	return nil
}

func TestCollaborationRealtimeAuthorizationKeepsIndependentReadBases(t *testing.T) {
	f := newCollaborationFixture(t)
	issue, err := f.svc.Create(f.ctx, f.params(t))
	if err != nil {
		t.Fatal(err)
	}
	id := util.UUIDToString(issue.Issue.ID)
	check := func(ctx context.Context, user, scope, target string, expected bool) {
		t.Helper()
		allowed, err := f.svc.AuthorizeRealtimeScope(ctx, user, scope, target)
		if err != nil || allowed != expected {
			t.Fatalf("%s %s: allowed=%v want=%v error=%v", scope, user, allowed, expected, err)
		}
	}
	check(f.ctx, f.creator, realtime.ScopeAccount, f.creator, true)
	check(f.ctx, f.creator, realtime.ScopeAccount, f.handler, false)
	check(context.Background(), f.creator, realtime.ScopeIssue, id, false)
	check(collaborationContext(f.handler), f.handler, realtime.ScopeIssue, id, true)
	check(collaborationContext(f.outsider), f.outsider, realtime.ScopeIssue, id, false)
	for _, kind := range []auth.CredentialKind{auth.CredentialTask, auth.CredentialCloud, "unknown"} {
		ctx := auth.WithAuthenticatedPrincipal(context.Background(), auth.AuthenticatedPrincipal{UserID: f.creator, Kind: kind})
		check(ctx, f.creator, realtime.ScopeIssue, id, false)
		check(ctx, f.creator, realtime.ScopeAccount, f.creator, false)
	}
	// Removing source membership leaves the creator's separate participation.
	f.fx.Exec(t, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, f.workspace, f.creator)
	check(f.ctx, f.creator, realtime.ScopeIssue, id, true)
	// A source-only reader keeps their basic read after participant revocation.
	f.fx.Member(t, f.workspace, f.handler, "member")
	f.fx.Exec(t, `UPDATE issue_participant SET revoked_at=now(),epoch=epoch+1 WHERE issue_id=$1 AND user_id=$2`, issue.Issue.ID, f.handler)
	check(collaborationContext(f.handler), f.handler, realtime.ScopeIssue, id, true)
	f.fx.Exec(t, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, f.workspace, f.handler)
	check(collaborationContext(f.handler), f.handler, realtime.ScopeIssue, id, false)
	f.fx.Exec(t, `UPDATE "user" SET disabled_at=now(),disabled_reason='test' WHERE id=$1`, f.handler)
	check(collaborationContext(f.handler), f.handler, realtime.ScopeAccount, f.handler, false)
}

func TestCollaborationRealtimeOutboxRetriesWithoutPrivatePayloads(t *testing.T) {
	f := newCollaborationFixture(t)
	p := f.params(t)
	p.Title, p.Description = "PRIVATE_TITLE_SENTINEL", "PRIVATE_BODY_SENTINEL"
	issue, err := f.svc.Create(f.ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	publisher := &collaborationPublisher{fail: true}
	worker := &CollaborationRealtime{Queries: f.svc.Queries, TxStarter: f.pool, Publisher: publisher}
	if count, err := worker.DeliverPending(f.ctx, 100); err == nil || count != 0 {
		t.Fatalf("relay failure reported success: %d %v", count, err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1 AND outbox_state='pending'`, issue.Issue.ID) != 1 {
		t.Fatal("failed publish lost its outbox record")
	}
	first := publisher.frames[0]
	publisher.fail = false
	if _, err := worker.DeliverPending(f.ctx, 100); err != nil {
		t.Fatal(err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1 AND outbox_state='pending'`, issue.Issue.ID) != 0 {
		t.Fatal("successful delivery stayed pending")
	}
	replayedFrames, issueDelivered := 0, false
	for _, frame := range publisher.frames {
		if strings.Contains(string(frame.data), "PRIVATE_") {
			t.Fatalf("private content leaked: %s", frame.data)
		}
		if frame.event == first.event && frame.scope == first.scope && frame.id == first.id {
			replayedFrames++
		}
		if frame.scope == realtime.ScopeWorkspace || frame.scope == realtime.ScopeUser {
			t.Fatalf("collaboration was published to ordinary audience: %+v", frame)
		}
		if frame.scope == realtime.ScopeIssue {
			issueDelivered = frame.id == util.UUIDToString(issue.Issue.ID)
			var out struct {
				Payload collaborationRealtimeChange `json:"payload"`
			}
			if err := json.Unmarshal(frame.data, &out); err != nil || out.Payload.EventSequence != 1 || out.Payload.AuthorityEpoch != 1 {
				t.Fatalf("invalid revision metadata: %s %v", frame.data, err)
			}
		}
	}
	if replayedFrames < 2 || !issueDelivered {
		t.Fatal("stable replay or issue delivery missing")
	}
	before := len(publisher.frames)
	if count, err := worker.DeliverPending(f.ctx, 100); err != nil || count != 0 || len(publisher.frames) != before {
		t.Fatalf("delivered record repeated: %d %v", count, err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_notification WHERE issue_id=$1`, issue.Issue.ID) != 1 {
		t.Fatal("outbox delivery recreated notifications")
	}
}

func TestCollaborationRealtimeRemovalReachesRevokedAccount(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	publisher := &collaborationPublisher{}
	worker := &CollaborationRealtime{Queries: f.svc.Queries, TxStarter: f.pool, Publisher: publisher}
	if _, err := worker.DeliverPending(f.ctx, 100); err != nil {
		t.Fatal(err)
	}
	publisher.frames = nil
	f.fx.Cleanup(t, `DELETE FROM issue_handoff WHERE issue_id=$1`, f.issue.Issue.ID)
	request := workflowRequest(f, 1)
	request.RequestID = uuid.NewString()
	if _, err := f.svc.RemoveParticipant(f.ctx, CollaborationParticipantParams{CollaborationWorkflowRequest: request, TargetUserID: util.MustParseUUID(f.handler)}); err != nil {
		t.Fatal(err)
	}
	if _, err := worker.DeliverPending(f.ctx, 100); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, frame := range publisher.frames {
		var out struct {
			Type    string            `json:"type"`
			Payload map[string]string `json:"payload"`
		}
		if err := json.Unmarshal(frame.data, &out); err != nil {
			t.Fatal(err)
		}
		if frame.scope == realtime.ScopeAccount && frame.id == f.handler && out.Type == "collaboration:access_changed" {
			found = true
			if out.Payload["issue_id"] != util.UUIDToString(f.issue.Issue.ID) || len(out.Payload) != 4 {
				t.Fatalf("wrong removal projection: %s", frame.data)
			}
		}
	}
	if !found {
		t.Fatal("the revoked viewer did not receive an account invalidation")
	}
}
