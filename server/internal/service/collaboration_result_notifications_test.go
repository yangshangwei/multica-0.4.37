package service

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestCollaborationResultNotificationsRequireProvenTerminalGroup(t *testing.T) {
	for _, outcome := range []string{"success", "failure", "missing", "stopped", "never_launched"} {
		t.Run(outcome, func(t *testing.T) {
			f := newManagedCollaborationFixture(t)
			f.submitManaged(t, f.creator, "recorded run notification input")
			f.resumeManaged(t)
			runID, _, claim := f.claimManaged(t)
			p := protocol.CollaborationRunPermitRequest{Identity: claim.Identity, AttemptSeq: "1", AttemptNonce: uuid.NewString(), Containment: protocol.CollaborationContainmentLinuxPIDNamespace}
			if outcome != "never_launched" {
				permit, err := f.svc.PermitRuntimeRun(f.daemonCtx, runID, p)
				if err != nil {
					t.Fatal(err)
				}
				if outcome == "success" || outcome == "failure" || outcome == "stopped" {
					business := outcome
					if business == "stopped" {
						business = "success"
					}
					if err = f.svc.RecordRuntimeOutcome(context.Background(), runID, permit.BusinessToken, protocol.CollaborationRunOutcome{Identity: claim.Identity, EventID: uuid.NewString(), AttemptSeq: "1", AttemptNonce: p.AttemptNonce, Outcome: business, Body: "recorded final"}); err != nil {
						t.Fatal(err)
					}
				}
			}
			count := func() int {
				return f.fx.Count(t, `SELECT count(*) FROM collaboration_notification WHERE issue_id=$1 AND kind LIKE 'ai_%'`, f.issue.Issue.ID)
			}
			if count() != 0 {
				t.Fatal("business result notified before process proof")
			}
			if outcome == "stopped" || outcome == "never_launched" {
				run, err := f.svc.Queries.GetCollaborationRuntimeRun(f.ctx, runID)
				if err != nil {
					t.Fatal(err)
				}
				if _, err = f.svc.StopQueue(f.ctx, f.stopParams(run, queueTarget(t, f.collaborationQueueFixture, f.targetID).ControlVersion)); err != nil {
					t.Fatal(err)
				}
				if outcome == "stopped" && count() != 0 {
					t.Fatal("stop intent was announced as confirmed stop")
				}
			}
			if outcome != "never_launched" {
				proof := managedProof(claim.Identity, p.AttemptNonce)
				if _, err := f.svc.AcceptRuntimeProof(context.Background(), runID, claim.TerminationToken, proof); err != nil {
					t.Fatal(err)
				}
				if _, err := f.svc.AcceptRuntimeProof(context.Background(), runID, claim.TerminationToken, proof); err != nil {
					t.Fatal(err)
				}
			}
			if count() != 2 {
				t.Fatalf("one system result must notify both input author/resource owner and other participant, got %d", count())
			}
			if outcome == "missing" {
				page, err := f.svc.RunRecords(f.ctx, f.issue.Issue.ID, runID, CollaborationRunRecordsQuery{Limit: 50})
				if err != nil || page.Recording.Status != "partial" || !page.Run.RecordingIncomplete || page.Usage != nil {
					t.Fatalf("proof without a result hid incomplete recording: %+v %v", page, err)
				}
			}
			kind := "ai_failed"
			if outcome == "success" {
				kind = "ai_completed"
			}
			if outcome == "stopped" || outcome == "never_launched" {
				kind = "ai_stopped"
			}
			if f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1 AND kind=$2 AND actor_user_id IS NULL`, f.issue.Issue.ID, kind) != 1 {
				t.Fatal("group result event was duplicated or impersonated a human actor")
			}
			page, err := f.svc.ListNotifications(f.ctx, CollaborationNotificationsQuery{Limit: 100})
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, item := range page.Items {
				if item.Kind == kind && item.IssueID == f.issue.Issue.ID {
					found = item.CanOpen && item.SourceKind == "issue" && !item.SourceID.Valid
				}
			}
			if !found {
				t.Fatal("offline result notification lost its authorized issue destination")
			}
			if f.fx.Count(t, `SELECT count(*) FROM inbox_item WHERE issue_id=$1 AND recipient_id=$2`, f.issue.Issue.ID, util.MustParseUUID(f.handler)) != 0 {
				t.Fatal("system result also created an ordinary source inbox row")
			}
		})
	}
}

func TestCollaborationResultNotificationRollbackAndConcurrentReplay(t *testing.T) {
	f := newManagedCollaborationFixture(t)
	// Source-only membership is not notification participation. Revoked or
	// disabled participants are ineligible even though their retained rows exist.
	f.fx.Member(t, f.workspace, f.outsider, "member")
	for _, state := range []string{"revoked", "disabled"} {
		user := f.fx.User(t, "Ineligible result recipient", state+"-"+uuid.NewString()+"@example.test")
		f.fx.InsertNoID(t, "issue_participant", testutil.Cols{"issue_id": f.issue.Issue.ID, "user_id": user}, "issue_id=$1 AND user_id=$2", f.issue.Issue.ID, user)
		if state == "revoked" {
			f.fx.Exec(t, `UPDATE issue_participant SET revoked_at=now() WHERE issue_id=$1 AND user_id=$2`, f.issue.Issue.ID, user)
		} else {
			f.fx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=$1`, user)
		}
	}
	f.submitManaged(t, f.creator, "atomic result")
	f.resumeManaged(t)
	run, _, claim := f.claimManaged(t)
	permit, err := f.svc.PermitRuntimeRun(f.daemonCtx, run, protocol.CollaborationRunPermitRequest{Identity: claim.Identity, AttemptSeq: "1", AttemptNonce: uuid.NewString(), Containment: protocol.CollaborationContainmentLinuxPIDNamespace})
	if err != nil {
		t.Fatal(err)
	}
	if err = f.svc.RecordRuntimeOutcome(context.Background(), run, permit.BusinessToken, protocol.CollaborationRunOutcome{Identity: claim.Identity, EventID: uuid.NewString(), AttemptSeq: "1", AttemptNonce: permit.AttemptNonce, Outcome: "success", Body: "atomic result"}); err != nil {
		t.Fatal(err)
	}
	proof := managedProof(claim.Identity, permit.AttemptNonce)
	f.svc.TxStarter = collaborationFailureStarter{TxStarter: f.pool, queryName: "CreateCollaborationResultNotification"}
	if _, err = f.svc.AcceptRuntimeProof(context.Background(), run, claim.TerminationToken, proof); err == nil || !strings.Contains(err.Error(), "injected collaboration persistence failure") {
		t.Fatalf("required notification failure was ignored: %v", err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND settled_at IS NOT NULL`, run) != 0 || f.fx.Count(t, `SELECT count(*) FROM collaboration_termination WHERE run_id=$1 AND confirmed_at IS NOT NULL`, run) != 0 || f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1 AND kind LIKE 'ai_%'`, f.issue.Issue.ID) != 0 {
		t.Fatal("notification failure committed a partial terminal transition")
	}
	f.svc.TxStarter = f.pool
	ready := make(chan struct{})
	errors := make(chan error, 2)
	var done sync.WaitGroup
	for range 2 {
		done.Add(1)
		go func() {
			defer done.Done()
			<-ready
			_, err := f.svc.AcceptRuntimeProof(context.Background(), run, claim.TerminationToken, proof)
			errors <- err
		}()
	}
	close(ready)
	done.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatalf("concurrent original proof replay: %v", err)
		}
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_notification WHERE issue_id=$1 AND kind='ai_completed'`, f.issue.Issue.ID) != 2 {
		t.Fatal("concurrent proof replay duplicated or dropped recipients")
	}
	f.fx.InsertNoID(t, "issue_participant", testutil.Cols{"issue_id": f.issue.Issue.ID, "user_id": f.outsider}, "issue_id=$1 AND user_id=$2", f.issue.Issue.ID, f.outsider)
	if _, err = f.svc.AcceptRuntimeProof(context.Background(), run, claim.TerminationToken, proof); err != nil {
		t.Fatal(err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_notification WHERE issue_id=$1 AND kind='ai_completed'`, f.issue.Issue.ID) != 2 {
		t.Fatal("proof replay recomputed the original recipient list")
	}
}

func TestCollaborationSquadRecordingAndOneFinalLeaderNotification(t *testing.T) {
	f, member := newManagedSquadFixture(t)
	f.submitManaged(t, f.creator, "squad recording root input")
	f.resumeManaged(t)
	root, request, claim := f.claimManaged(t)
	token, nonce := permitSquadRun(t, f, root, claim)
	dispatched, err := f.svc.CreateRuntimeComment(context.Background(), root, token, CollaborationRuntimeCommentParams{CommentID: util.MustParseUUID(uuid.NewString()), Content: "[@member](mention://agent/" + util.UUIDToString(member) + ") investigate"})
	if err != nil || len(dispatched.RunIDs) != 1 {
		t.Fatalf("dispatch: %+v %v", dispatched, err)
	}
	finish := func(run pgtype.UUID, claim protocol.CollaborationRunClaimResponse, token, nonce, outcome string) {
		t.Helper()
		activity := "no_action"
		if outcome == "failure" {
			activity = "failed"
		}
		if err := f.svc.RuntimeSquadActivity(context.Background(), run, token, uuid.NewString(), activity, "recorded turn result"); err != nil {
			t.Fatal(err)
		}
		record := recordedOutcome(claim, protocol.CollaborationRunPermitResponse{AttemptSeq: "1", AttemptNonce: nonce}, 1)
		record.Outcome = outcome
		record.Record.Completeness = "complete"
		wrong := record
		wrong.Outcome = "failure"
		if outcome == "failure" {
			wrong.Outcome = "success"
		}
		if err := f.svc.RecordRuntimeOutcome(context.Background(), run, token, wrong); !collaborationErrorCode(err, "outcome_already_recorded") {
			t.Fatalf("recording replaced prior conflicting activity: %v", err)
		}
		if err := f.svc.RecordRuntimeOutcome(context.Background(), run, token, record); err != nil {
			t.Fatal(err)
		}
		if err := f.svc.RecordRuntimeOutcome(context.Background(), run, token, record); err != nil {
			t.Fatal(err)
		}
		if _, err := f.svc.AcceptRuntimeProof(context.Background(), run, claim.TerminationToken, managedProof(claim.Identity, nonce)); err != nil {
			t.Fatal(err)
		}
		page, err := f.svc.RunRecords(f.ctx, f.issue.Issue.ID, run, CollaborationRunRecordsQuery{Limit: 50})
		if err != nil || page.Recording.Status != "complete" || page.Usage == nil {
			t.Fatalf("squad turn lost its typed recording: %+v %v", page, err)
		}
	}
	count := func() int {
		return f.fx.Count(t, `SELECT count(*) FROM collaboration_notification WHERE issue_id=$1 AND kind LIKE 'ai_%'`, f.issue.Issue.ID)
	}
	finish(root, claim, token, nonce, "success")
	if count() != 0 {
		t.Fatal("first leader turn notified whole-squad completion")
	}
	memberRun := dispatched.RunIDs[0]
	memberClaim, err := f.svc.ClaimRuntimeRun(f.daemonCtx, memberRun, request)
	if err != nil {
		t.Fatal(err)
	}
	memberToken, memberNonce := permitSquadRun(t, f, memberRun, memberClaim)
	joined := f.submitManaged(t, f.handler, "joined input read only by the continuation")
	finish(memberRun, memberClaim, memberToken, memberNonce, "failure")
	if count() != 0 {
		t.Fatal("member failure notified before final leader convergence")
	}
	continuation, _, lastClaim := f.claimManaged(t)
	lastToken, lastNonce := permitSquadRun(t, f, continuation, lastClaim)
	finish(continuation, lastClaim, lastToken, lastNonce, "success")
	if count() != 2 || f.fx.Count(t, `SELECT count(*) FROM collaboration_notification WHERE issue_id=$1 AND kind='ai_completed' AND run_id=$2`, f.issue.Issue.ID, continuation) != 2 {
		t.Fatal("group result did not link exactly once to its final leader turn")
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_notification WHERE issue_id=$1 AND kind='ai_failed'`, f.issue.Issue.ID) != 0 {
		t.Fatal("recovered member failure created a separate group result")
	}
	for _, id := range []pgtype.UUID{root, memberRun, continuation} {
		page, err := f.svc.RunRecords(f.ctx, f.issue.Issue.ID, id, CollaborationRunRecordsQuery{Limit: 50})
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, item := range page.Items {
			if item.Kind == "input" && item.CommandID == util.UUIDToString(joined.CommandID) {
				found = true
			}
		}
		if found != (id == continuation) {
			t.Fatal("a sibling run's current group inputs replaced its immutable prompt readership")
		}
	}
}
