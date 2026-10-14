package service

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func editString(value string) *string { return &value }

func editParams(f *collaborationQueueFixture, revision int64) CollaborationEditParams {
	return CollaborationEditParams{CollaborationWorkflowRequest: workflowRequest(f, revision), Title: editString("Edited title")}
}

func TestCollaborationEditRecordsCurrentActorAndImmutableExecution(t *testing.T) {
	for _, role := range []string{"creator", "external_handler", "departed_source_creator"} {
		t.Run(role, func(t *testing.T) {
			f := newCollaborationQueueFixture(t)
			actor := f.creator
			if role == "external_handler" {
				actor = f.handler
			}
			if role == "departed_source_creator" {
				f.fx.Exec(t, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, f.workspace, actor)
			}
			f.fx.Exec(t, `UPDATE issue SET start_date='2026-10-01' WHERE id=$1`, f.issue.Issue.ID)
			first := f.submit(t, f.creator, "original command snapshot")
			waiting := f.submit(t, f.handler, "immutable waiting snapshot")
			run := f.run(t, first, 1)
			grant := workflowGrant(t, f, f.handler)
			f.fx.Exec(t, `UPDATE collaboration_session SET active_grant_id=$2 WHERE id=$1`, f.issue.SessionID, grant)
			requestID := f.fx.Insert(t, "issue_handoff", testutil.Cols{"issue_id": f.issue.Issue.ID, "actor_user_id": f.creator,
				"from_user_id": f.creator, "to_user_id": f.handler, "purpose": "information", "requester_user_id": f.creator, "previous_status": "todo"})
			f.fx.Exec(t, `UPDATE issue_collaboration SET active_information_request_id=$2 WHERE issue_id=$1`, f.issue.Issue.ID, requestID)
			notifications := f.fx.Count(t, `SELECT count(*) FROM collaboration_notification WHERE issue_id=$1`, f.issue.Issue.ID)
			p := editParams(f, 1)
			p.Title, p.Description = editString("Edited\x00 title"), editString("New body\x00 with @AI")
			p.SetStartDate, p.SetDueDate = true, true
			p.StartDate, p.DueDate = editString(""), editString("2026-10-31T00:00:00Z")
			started := time.Now().UTC()
			r, err := f.svc.EditIssue(collaborationContext(actor), p)
			if err != nil || r.Code != "issue_edited" || r.CollaborationRevision != 2 || r.ControlVersion != 1 || r.QueueState != "active" {
				t.Fatalf("authorized edit: %+v %v", r, err)
			}
			access, err := f.svc.ResolveIssueAccess(f.ctx, nil, f.issue.Issue.ID)
			if err != nil || access.Issue.Title != "Edited title" || access.Issue.Description.String != "New body with @AI" || access.Issue.StartDate.Valid ||
				!access.Issue.DueDate.Valid || access.Issue.DueDate.Time.Format(time.DateOnly) != "2026-10-31" || access.Issue.Revision != f.issue.Issue.Revision+1 {
				t.Fatalf("wrong persisted edit: %+v %v", access.Issue, err)
			}
			var payload []byte
			var recordedActor string
			var recordedAt time.Time
			f.fx.QueryRow(t, `SELECT payload, actor_user_id::text, created_at FROM collaboration_event WHERE issue_id=$1 AND kind='issue_edited'`, f.issue.Issue.ID).
				Scan(&payload, &recordedActor, &recordedAt)
			var history struct {
				Before struct {
					Title, Description string
					StartDate          *string `json:"start_date"`
					DueDate            *string `json:"due_date"`
				}
				After struct {
					Title, Description string
					StartDate          *string `json:"start_date"`
					DueDate            *string `json:"due_date"`
				}
				RequestID string    `json:"request_id"`
				EditedAt  time.Time `json:"edited_at"`
			}
			if err = json.Unmarshal(payload, &history); err != nil {
				t.Fatal(err)
			}
			if recordedActor != actor || recordedAt.IsZero() || history.EditedAt.Before(started) || history.EditedAt.After(time.Now().UTC()) || history.RequestID != p.RequestID ||
				history.Before.Title != f.issue.Issue.Title || history.Before.Description != f.issue.Issue.Description.String || history.Before.StartDate == nil || *history.Before.StartDate != "2026-10-01" || history.Before.DueDate != nil ||
				history.After.Title != "Edited title" || history.After.Description != "New body with @AI" || history.After.StartDate != nil || history.After.DueDate == nil || *history.After.DueDate != "2026-10-31" {
				t.Fatalf("history lost actual actor/time/before/after: %s, actor=%s, at=%s", payload, recordedActor, recordedAt)
			}
			if f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='in_flight' AND content='original command snapshot' AND author_user_id=$2 AND seq=1`, first.CommandID, f.creator) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='waiting' AND content='immutable waiting snapshot' AND author_user_id=$2 AND seq=2`, waiting.CommandID, f.handler) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND forced_disposition IS NULL AND settled_at IS NULL AND authorized_attempt_seq=1 AND fence=$2 AND launch_nonce=$3`, run.ID, run.Fence, run.LaunchNonce) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_session WHERE id=$1 AND queue_state='active' AND control_version=1 AND active_grant_id=$2 AND resource_epoch=1 AND active_run_id=$3`, f.issue.SessionID, grant, run.ID) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_grant WHERE id=$1 AND revoked_at IS NULL AND epoch=1`, grant) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM issue_collaboration WHERE issue_id=$1 AND authority_epoch=1 AND lifecycle_epoch=1 AND active_information_request_id=$2`, f.issue.Issue.ID, requestID) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM issue_handoff WHERE id=$1 AND resolution='open'`, requestID) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND assignee_id=$2 AND status='todo'`, f.issue.Issue.ID, f.handler) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_notification WHERE issue_id=$1`, f.issue.Issue.ID) != notifications ||
				f.fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1`, f.issue.Issue.ID) != 0 ||
				f.fx.Count(t, `SELECT count(*) FROM member WHERE workspace_id=$1 AND user_id=$2`, f.workspace, f.handler) != 0 {
				t.Fatal("metadata edit changed execution, identity, information chain, membership or notifications")
			}
		})
	}
}

func TestCollaborationEditRejectsNonEditors(t *testing.T) {
	for _, role := range []string{"source_member", "source_owner", "participant", "former_handler", "revoked_source_member", "disabled", "task", "cloud", "unknown", "agent_actor", "restricted"} {
		t.Run(role, func(t *testing.T) {
			f := newCollaborationQueueFixture(t)
			actor := f.handler
			kind := auth.CredentialJWT
			want := "forbidden"
			switch role {
			case "source_member", "source_owner":
				actor = f.outsider
				memberRole := "member"
				if role == "source_owner" {
					memberRole = "owner"
				}
				f.fx.Member(t, f.workspace, actor, memberRole)
			case "participant":
				actor = f.outsider
				f.fx.InsertNoID(t, "issue_participant", testutil.Cols{"issue_id": f.issue.Issue.ID, "user_id": actor}, "issue_id=$1 AND user_id=$2", f.issue.Issue.ID, actor)
			case "former_handler":
				f.fx.Exec(t, `UPDATE issue SET assignee_id=$2 WHERE id=$1`, f.issue.Issue.ID, f.creator)
			case "revoked_source_member":
				f.fx.Member(t, f.workspace, actor, "member")
				f.fx.Exec(t, `UPDATE issue_participant SET revoked_at=now(),epoch=2 WHERE issue_id=$1 AND user_id=$2`, f.issue.Issue.ID, actor)
			case "disabled":
				want = "not_found"
				f.fx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=$1`, actor)
			case "task":
				kind = auth.CredentialTask
			case "cloud":
				kind = auth.CredentialCloud
			case "unknown":
				kind = "unknown"
			}
			ctx := auth.WithAuthenticatedPrincipal(context.Background(), auth.AuthenticatedPrincipal{UserID: actor, Kind: kind, Restricted: role == "restricted", AgentActor: role == "agent_actor"})
			_, err := f.svc.EditIssue(ctx, editParams(f, 1))
			if !collaborationErrorCode(err, want) {
				t.Fatalf("%s edit: %v", role, err)
			}
			if f.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND title=$2 AND description=$3`, f.issue.Issue.ID, f.issue.Issue.Title, f.issue.Issue.Description.String) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1 AND kind='issue_edited'`, f.issue.Issue.ID) != 0 {
				t.Fatal("unauthorized editor changed content or history")
			}
		})
	}
}

func TestCollaborationEditNoopAndDateValidation(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	f.fx.Exec(t, `UPDATE issue SET start_date='2026-10-01',due_date='2026-10-31' WHERE id=$1`, f.issue.Issue.ID)
	var eventSeq int64
	f.fx.QueryRow(t, `SELECT event_seq FROM collaboration_session WHERE id=$1`, f.issue.SessionID).Scan(&eventSeq)
	for _, p := range []CollaborationEditParams{
		{CollaborationWorkflowRequest: workflowRequest(f, 1)},
		{CollaborationWorkflowRequest: workflowRequest(f, 1), Title: editString(f.issue.Issue.Title), Description: editString(f.issue.Issue.Description.String)},
		{CollaborationWorkflowRequest: workflowRequest(f, 1), SetStartDate: true, StartDate: editString("2026-10-01T08:00:00+08:00"), SetDueDate: true, DueDate: editString("2026-10-31")},
	} {
		r, err := f.svc.EditIssue(f.ctx, p)
		if err != nil || r.Code != "unchanged" || r.CollaborationRevision != 1 {
			t.Fatalf("same value is not an edit: %+v %v", r, err)
		}
	}
	for _, field := range []string{"start_date", "due_date"} {
		for _, value := range []string{"2026-02-30", "2026-10-01T01:00:00Z", " 2026-10-01 ", "tomorrow"} {
			p := editParams(f, 1)
			if field == "start_date" {
				p.SetStartDate, p.StartDate = true, editString(value)
			} else {
				p.SetDueDate, p.DueDate = true, editString(value)
			}
			if _, err := f.svc.EditIssue(f.ctx, p); !collaborationErrorCode(err, "invalid_"+field) {
				t.Fatalf("invalid %s accepted or leaked parser details: %v", field, err)
			}
		}
	}
	if f.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND title=$2 AND revision=$3 AND start_date='2026-10-01' AND due_date='2026-10-31'`, f.issue.Issue.ID, f.issue.Issue.Title, f.issue.Issue.Revision) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_session WHERE id=$1 AND event_seq=$2`, f.issue.SessionID, eventSeq) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1 AND kind='issue_edited'`, f.issue.Issue.ID) != 0 {
		t.Fatal("no-op or invalid fields manufactured a mutation/event")
	}
}

func TestCollaborationEditStaleVersionsAndExactRecovery(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	const revision = int64(9007199254740993)
	f.fx.Exec(t, `UPDATE issue_collaboration SET revision=$2,lifecycle_epoch=$2 WHERE issue_id=$1`, f.issue.Issue.ID, revision)
	f.fx.Exec(t, `UPDATE issue_participant SET epoch=$3 WHERE issue_id=$1 AND user_id=$2`, f.issue.Issue.ID, f.creator, revision)
	f.fx.Exec(t, `UPDATE collaboration_account_fence SET epoch=$2 WHERE user_id=$1`, f.creator, revision)
	valid := editParams(f, revision)
	valid.ExpectedLifecycleEpoch, valid.ExpectedParticipantEpoch, valid.ExpectedAccountEpoch = revision, revision, revision
	for _, field := range []string{"revision", "lifecycle", "participant", "account"} {
		p := valid
		p.RequestID = uuid.NewString()
		want := "stale_authority"
		switch field {
		case "revision":
			p.ExpectedCollaborationRevision--
			want = "stale_revision"
		case "lifecycle":
			p.ExpectedLifecycleEpoch--
		case "participant":
			p.ExpectedParticipantEpoch--
		case "account":
			p.ExpectedAccountEpoch--
		}
		if _, err := f.svc.EditIssue(f.ctx, p); !collaborationErrorCode(err, want) {
			t.Fatalf("stale %s: %v", field, err)
		}
	}
	r, err := f.svc.EditIssue(f.ctx, valid)
	if err != nil || r.CollaborationRevision != revision+1 {
		t.Fatalf("large edit revision: %+v %v", r, err)
	}
	replay, err := f.svc.EditIssue(f.ctx, valid)
	if err != nil || replay != r {
		t.Fatalf("same request changed result: %+v %v", replay, err)
	}
	encoded, err := json.Marshal(replay)
	if err != nil || !strings.Contains(string(encoded), `"collaboration_revision":"9007199254740994"`) {
		t.Fatalf("lost version precision: %s %v", encoded, err)
	}
	changed := valid
	changed.Title = editString("Changed same-key content")
	if _, err := f.svc.EditIssue(f.ctx, changed); !collaborationErrorCode(err, "idempotency_conflict") {
		t.Fatalf("changed-key digest: %v", err)
	}
	f.fx.Exec(t, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, f.workspace, f.creator)
	f.fx.Exec(t, `UPDATE issue_participant SET revoked_at=now() WHERE issue_id=$1 AND user_id=$2`, f.issue.Issue.ID, f.creator)
	if _, err := f.svc.EditIssue(f.ctx, valid); !collaborationErrorCode(err, "not_found") {
		t.Fatalf("revoked editor replay read content: %v", err)
	}
	recovered, err := f.svc.GetWorkflowOperation(f.ctx, f.issue.Issue.ID, valid.RequestID)
	if err != nil || recovered.Code != r.Code || recovered.CollaborationRevision != revision+1 {
		t.Fatalf("own receipt recovery: %+v %v", recovered, err)
	}
	encoded, err = json.Marshal(recovered)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"title", "description", "start_date", "due_date", "actor", "queue", "control", "payload"} {
		if strings.Contains(string(encoded), field) {
			t.Fatalf("recovery disclosed %s: %s", field, encoded)
		}
	}
}

func TestCollaborationEditConcurrentIdentityAndRevision(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	type result struct {
		receipt CollaborationWorkflowReceipt
		err     error
	}
	for _, sameKey := range []bool{true, false} {
		p := editParams(f, 1)
		if !sameKey {
			p.ExpectedCollaborationRevision = 2
			p.Title = editString("Next title")
		}
		start := make(chan struct{})
		results := make(chan result, 2)
		var wg sync.WaitGroup
		for i := range 2 {
			request := p
			if !sameKey && i == 1 {
				request.RequestID = uuid.NewString()
				request.Title = editString("Competing title")
			}
			wg.Go(func() { <-start; r, err := f.svc.EditIssue(f.ctx, request); results <- result{r, err} })
		}
		close(start)
		wg.Wait()
		a, b := <-results, <-results
		if sameKey && (a.err != nil || b.err != nil || a.receipt != b.receipt) {
			t.Fatalf("same key: %+v %+v", a, b)
		}
		if !sameKey && ((a.err == nil) == (b.err == nil) || (a.err != nil && !collaborationErrorCode(a.err, "stale_revision")) || (b.err != nil && !collaborationErrorCode(b.err, "stale_revision"))) {
			t.Fatalf("concurrent edits did not yield one version winner: %+v %+v", a, b)
		}
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1 AND kind='issue_edited'`, f.issue.Issue.ID) != 2 {
		t.Fatal("concurrent edits duplicated or dropped history")
	}
}

func TestCollaborationEditAndTransferSerializeAtCommit(t *testing.T) {
	for _, editFirst := range []bool{false, true} {
		t.Run(map[bool]string{false: "transfer_first", true: "edit_first"}[editFirst], func(t *testing.T) {
			f := newCollaborationQueueFixture(t)
			f.fx.Cleanup(t, `DELETE FROM issue_handoff WHERE issue_id=$1`, f.issue.Issue.ID)
			ready, release := make(chan int, 1), make(chan struct{})
			firstSvc := *f.svc
			firstSvc.TxStarter = lifecycleCommitBarrierStarter{TxStarter: f.pool, ready: ready, release: release}
			ctx, cancel := context.WithTimeout(collaborationContext(f.handler), 10*time.Second)
			defer cancel()
			edits := editParams(f, 1)
			transfer := CollaborationTransferParams{CollaborationParticipantParams: CollaborationParticipantParams{CollaborationWorkflowRequest: workflowRequest(f, 1), TargetUserID: util.MustParseUUID(f.creator)}}
			firstDone, secondDone := make(chan error, 1), make(chan error, 1)
			go func() {
				var err error
				if editFirst {
					_, err = firstSvc.EditIssue(ctx, edits)
				} else {
					_, err = firstSvc.Transfer(ctx, transfer)
				}
				firstDone <- err
			}()
			var blocker int
			select {
			case blocker = <-ready:
			case <-ctx.Done():
				t.Fatal("first writer did not reach commit barrier")
			}
			go func() {
				var err error
				if editFirst {
					_, err = f.svc.Transfer(ctx, transfer)
				} else {
					_, err = f.svc.EditIssue(ctx, edits)
				}
				secondDone <- err
			}()
			waitWorkflowBlocked(t, f, blocker, secondDone)
			close(release)
			if err := <-firstDone; err != nil {
				t.Fatalf("first writer: %v", err)
			}
			want := "forbidden"
			if editFirst {
				want = "stale_revision"
			}
			if err := <-secondDone; !collaborationErrorCode(err, want) {
				t.Fatalf("late writer bypassed current locked state: %v", err)
			}
			title, handler, historyCount := f.issue.Issue.Title, f.creator, 0
			if editFirst {
				title, handler, historyCount = *edits.Title, f.handler, 1
			}
			if f.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND title=$2 AND assignee_id=$3`, f.issue.Issue.ID, title, handler) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1 AND kind='issue_edited'`, f.issue.Issue.ID) != historyCount {
				t.Fatal("late writer overwrote a committed edit or responsibility")
			}
		})
	}
}

func TestCollaborationEditRechecksDeactivationAfterAccountLock(t *testing.T) {
	for _, disabled := range []bool{false, true} {
		t.Run(map[bool]string{false: "revoked_account_epoch", true: "disabled_account"}[disabled], func(t *testing.T) {
			f := newCollaborationQueueFixture(t)
			ctx, cancel := context.WithTimeout(collaborationContext(f.handler), 10*time.Second)
			defer cancel()
			tx, err := f.pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(context.Background())
			if _, err = db.New(tx).LockCollaborationUsers(ctx, []pgtype.UUID{util.MustParseUUID(f.handler)}); err != nil {
				t.Fatal(err)
			}
			blocker := int(tx.Conn().PgConn().PID())
			done := make(chan error, 1)
			go func() { _, err := f.svc.EditIssue(ctx, editParams(f, 1)); done <- err }()
			waitWorkflowBlocked(t, f, blocker, done)
			if disabled {
				_, err = tx.Exec(ctx, `UPDATE "user" SET disabled_at=now() WHERE id=$1`, f.handler)
			} else {
				_, err = tx.Exec(ctx, `UPDATE collaboration_account_fence SET epoch=epoch+1 WHERE user_id=$1`, f.handler)
			}
			if err != nil {
				t.Fatal(err)
			}
			if err = tx.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			want := "stale_authority"
			if disabled {
				want = "forbidden"
			}
			if err = <-done; !collaborationErrorCode(err, want) {
				t.Fatalf("edit accepted pre-lock authority: %v", err)
			}
			if f.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND title=$2`, f.issue.Issue.ID, f.issue.Issue.Title) != 1 {
				t.Fatal("deactivated editor changed content")
			}
		})
	}
}

func TestCollaborationEditRequiredWritesRollbackTogether(t *testing.T) {
	for _, needle := range []string{"UPDATE issue SET title", "UPDATE issue_collaboration SET revision", "UPDATE collaboration_session SET event_seq", "INSERT INTO collaboration_event", "UPDATE collaboration_operation", "INSERT INTO iteration_event", "UPDATE iteration SET scope_revision"} {
		t.Run(needle, func(t *testing.T) {
			f := newCollaborationQueueFixture(t)
			iid := f.fx.Insert(t, "iteration", testutil.Cols{"workspace_id": f.workspace, "name": "Edit facts", "timezone": "UTC", "start_date": "2026-10-01", "end_date": "2026-10-31", "created_by": f.creator})
			f.fx.InsertNoID(t, "iteration_participation", testutil.Cols{"workspace_id": f.workspace, "iteration_id": iid, "issue_id": f.issue.Issue.ID, "first_joined_at": time.Now(), "current_joined_at": time.Now()}, "iteration_id=$1 AND issue_id=$2", iid, f.issue.Issue.ID)
			f.fx.Cleanup(t, `DELETE FROM iteration_event WHERE iteration_id=$1`, iid)
			f.fx.Exec(t, `UPDATE issue SET current_iteration_id=$2 WHERE id=$1`, f.issue.Issue.ID, iid)
			var eventSeq int64
			f.fx.QueryRow(t, `SELECT event_seq FROM collaboration_session WHERE id=$1`, f.issue.SessionID).Scan(&eventSeq)
			f.svc.TxStarter = workflowFaultStarter{TxStarter: f.pool, needle: needle}
			p := editParams(f, 1)
			if _, err := f.svc.EditIssue(f.ctx, p); err == nil {
				t.Fatal("required edit write failure reported success")
			}
			if f.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND title=$2 AND revision=$3`, f.issue.Issue.ID, f.issue.Issue.Title, f.issue.Issue.Revision) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM issue_collaboration WHERE issue_id=$1 AND revision=1`, f.issue.Issue.ID) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_session WHERE id=$1 AND event_seq=$2`, f.issue.SessionID, eventSeq) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1 AND kind='issue_edited'`, f.issue.Issue.ID) != 0 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_operation WHERE scope_id=$1 AND request_id=$2`, f.issue.Issue.ID, p.RequestID) != 0 ||
				f.fx.Count(t, `SELECT count(*) FROM iteration_event WHERE iteration_id=$1`, iid) != 0 {
				t.Fatal("failed edit committed content, version, facts, history or receipt")
			}
		})
	}
}

func TestCollaborationEditRecordsIterationTitleWithoutDateFacts(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	iid := f.fx.Insert(t, "iteration", testutil.Cols{"workspace_id": f.workspace, "name": "Edit facts", "timezone": "UTC", "start_date": "2026-10-01", "end_date": "2026-10-31", "created_by": f.creator})
	f.fx.InsertNoID(t, "iteration_participation", testutil.Cols{"workspace_id": f.workspace, "iteration_id": iid, "issue_id": f.issue.Issue.ID, "first_joined_at": time.Now(), "current_joined_at": time.Now()}, "iteration_id=$1 AND issue_id=$2", iid, f.issue.Issue.ID)
	f.fx.Cleanup(t, `DELETE FROM iteration_event WHERE iteration_id=$1`, iid)
	f.fx.Exec(t, `UPDATE issue SET current_iteration_id=$2 WHERE id=$1`, f.issue.Issue.ID, iid)
	p := editParams(f, 1)
	if _, err := f.svc.EditIssue(collaborationContext(f.handler), p); err != nil {
		t.Fatal(err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM iteration_event WHERE iteration_id=$1 AND actor->>'user_id'=$2 AND before_facts->>'title'=$3 AND after_facts->>'title'=$4`, iid, f.handler, f.issue.Issue.Title, *p.Title) != 1 {
		t.Fatal("title edit lost the existing iteration recorder's actual actor and facts")
	}
	p = CollaborationEditParams{CollaborationWorkflowRequest: workflowRequest(f, 2), Description: editString("Content-only change"), SetDueDate: true, DueDate: editString("2026-11-01")}
	if _, err := f.svc.EditIssue(collaborationContext(f.handler), p); err != nil {
		t.Fatal(err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM iteration_event WHERE iteration_id=$1`, iid) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND current_iteration_id=$2 AND iteration_rollover_count=0`, f.issue.Issue.ID, iid) != 1 {
		t.Fatal("description/date edits manufactured iteration activity or changed membership")
	}
}

func TestCollaborationEditUnknownCommitAndRefusedKeysNeverReapply(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	p := editParams(f, 1)
	fault := &queueFaultStarter{TxStarter: f.pool, lostCommit: true}
	f.svc.TxStarter = fault
	if _, err := f.svc.EditIssue(f.ctx, p); err == nil || fault.begins.Load() != 1 {
		t.Fatalf("uncertain commit was retried or reported successful: %v", err)
	}
	f.svc.TxStarter = f.pool
	r, err := f.svc.EditIssue(f.ctx, p)
	if err != nil || r.Code != "issue_edited" || r.CollaborationRevision != 2 {
		t.Fatalf("original key recovery: %+v %v", r, err)
	}
	p.RequestID, p.ExpectedCollaborationRevision, p.Title = uuid.NewString(), 3, editString("Future edit")
	refused, err := f.svc.EditIssue(f.ctx, p)
	if !collaborationErrorCode(err, "stale_revision") {
		t.Fatalf("stale edit: %v", err)
	}
	f.fx.Exec(t, `UPDATE issue_collaboration SET revision=3 WHERE issue_id=$1`, f.issue.Issue.ID)
	replay, err := f.svc.EditIssue(f.ctx, p)
	if !collaborationErrorCode(err, "stale_revision") || !reflect.DeepEqual(replay, refused) {
		t.Fatalf("refused key later became accepted: %+v %v", replay, err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1 AND kind='issue_edited'`, f.issue.Issue.ID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND title='Edited title'`, f.issue.Issue.ID) != 1 {
		t.Fatal("recovery reapplied a mutation")
	}
}

func TestCollaborationEditInt64OverflowRollsBack(t *testing.T) {
	for _, field := range []string{"collaboration_revision", "issue_revision", "event_sequence"} {
		t.Run(field, func(t *testing.T) {
			f := newCollaborationQueueFixture(t)
			p := editParams(f, 1)
			switch field {
			case "collaboration_revision":
				f.fx.Exec(t, `UPDATE issue_collaboration SET revision=$2 WHERE issue_id=$1`, f.issue.Issue.ID, int64(math.MaxInt64))
				p.ExpectedCollaborationRevision = math.MaxInt64
			case "issue_revision":
				f.fx.Exec(t, `UPDATE issue SET revision=$2 WHERE id=$1`, f.issue.Issue.ID, int64(math.MaxInt64))
			case "event_sequence":
				f.fx.Exec(t, `UPDATE collaboration_session SET event_seq=$2 WHERE id=$1`, f.issue.SessionID, int64(math.MaxInt64))
			}
			if _, err := f.svc.EditIssue(f.ctx, p); err == nil {
				t.Fatal("overflow accepted an edit")
			}
			if f.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND title=$2`, f.issue.Issue.ID, f.issue.Issue.Title) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1 AND kind='issue_edited'`, f.issue.Issue.ID) != 0 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_operation WHERE scope_id=$1 AND request_id=$2`, f.issue.Issue.ID, p.RequestID) != 0 {
				t.Fatal("overflow committed a partial edit")
			}
		})
	}
}

func TestCollaborationEditDoesNotNeedOpenExecutionOrBusinessState(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	f.fx.Exec(t, `UPDATE issue SET status='done' WHERE id=$1`, f.issue.Issue.ID)
	f.fx.Exec(t, `UPDATE collaboration_session SET queue_state='closed' WHERE id=$1`, f.issue.SessionID)
	f.fx.Exec(t, `UPDATE collaboration_execution_gate SET enabled=false,creation_enabled=false WHERE id=1`)
	r, err := f.svc.EditIssue(f.ctx, editParams(f, 1))
	if err != nil || r.Status != http.StatusOK || r.QueueState != "closed" || r.ControlVersion != 1 {
		t.Fatalf("metadata edit invented an execution prerequisite: %+v %v", r, err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND status='done'`, f.issue.Issue.ID) != 1 {
		t.Fatal("editing closed content reopened the issue")
	}
}
