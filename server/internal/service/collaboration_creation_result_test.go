package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/entitlement"
	"github.com/multica-ai/multica/server/internal/entitlement/entitlementtest"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestCollaborationCreationRejectionRemainsSealed(t *testing.T) {
	for _, reason := range []string{"recipient", "gate"} {
		t.Run(reason, func(t *testing.T) {
			f := newCollaborationFixture(t)
			p := f.params(t)
			status, code := http.StatusForbidden, "account_unavailable"
			if reason == "recipient" {
				f.fx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=$1`, f.handler)
			} else {
				status, code = http.StatusServiceUnavailable, "collaboration_unavailable"
				f.fx.Exec(t, `UPDATE collaboration_execution_gate SET creation_enabled=false WHERE id=1`)
			}
			_, err := f.svc.Create(f.ctx, p)
			wantRecipientError(t, err, status, code)
			result, err := f.svc.GetCreationResult(f.ctx, p.SourceWorkspaceID, p.RequestID)
			if err != nil || result != (CollaborationCreationResult{RequestID: p.RequestID, SourceWorkspaceID: p.SourceWorkspaceID, ActorUserID: util.MustParseUUID(f.creator), Outcome: "rejected", Code: code}) {
				t.Fatalf("refused creation result lost its original identity: %+v %v", result, err)
			}
			f.fx.Exec(t, `UPDATE "user" SET disabled_at=NULL WHERE id=$1`, f.handler)
			f.fx.Exec(t, `UPDATE collaboration_execution_gate SET creation_enabled=true WHERE id=1`)
			_, err = f.svc.Create(f.ctx, p)
			wantRecipientError(t, err, status, code)
			if f.fx.Count(t, `SELECT count(*) FROM issue WHERE workspace_id=$1`, f.workspace) != 0 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_operation WHERE scope_kind='create' AND scope_id=$1 AND actor_user_id=$2 AND request_id=$3 AND result_code=$4 AND completed_at IS NOT NULL AND resolved_issue_id IS NULL`, f.workspace, f.creator, p.RequestID, code) != 1 {
				t.Fatal("refused creation was not permanently sealed without an issue")
			}
			p.Title = "Changed original payload"
			_, err = f.svc.Create(f.ctx, p)
			wantRecipientError(t, err, http.StatusConflict, "idempotency_conflict")
		})
	}
}

func TestCollaborationCreationRejectionUnknownCommitMustBeRead(t *testing.T) {
	for _, committed := range []bool{false, true} {
		name := "rolled_back"
		if committed {
			name = "committed_ack_lost"
		}
		t.Run(name, func(t *testing.T) {
			f := newCollaborationFixture(t)
			p := f.params(t)
			f.fx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=$1`, f.handler)
			fault := &queueFaultStarter{TxStarter: f.pool, commitFailure: !committed, lostCommit: committed}
			f.svc.TxStarter = fault
			_, err := f.svc.Create(f.ctx, p)
			var domain *CollaborationError
			if err == nil || errors.As(err, &domain) || !strings.Contains(err.Error(), "commit collaboration creation") || fault.begins.Load() != 1 {
				t.Fatalf("unknown commit was reported as settled or automatically retried: %v", err)
			}
			f.svc.TxStarter = f.pool
			result, err := f.svc.GetCreationResult(f.ctx, p.SourceWorkspaceID, p.RequestID)
			if committed {
				if err != nil || result.Outcome != "rejected" || result.Code != "account_unavailable" {
					t.Fatalf("lost acknowledgement did not recover the committed refusal: %+v %v", result, err)
				}
			} else if !collaborationErrorCode(err, "operation_not_found") {
				t.Fatalf("uncommitted refusal was visible as a settled result: %+v %v", result, err)
			}
			f.fx.Exec(t, `UPDATE "user" SET disabled_at=NULL WHERE id=$1`, f.handler)
			_, err = f.svc.Create(f.ctx, p)
			if committed {
				wantRecipientError(t, err, http.StatusForbidden, "account_unavailable")
			} else if err != nil {
				t.Fatalf("rolled-back unknown request could not use its original identity: %v", err)
			}
		})
	}
}

func TestCollaborationCreationRejectionCorrectedIntentConsumesDraftOnce(t *testing.T) {
	f, _ := newCollaborationAttachmentFixture(t)
	upload := attachmentUploadParams(f)
	first, err := f.svc.UploadAttachmentDraft(f.ctx, upload)
	if err != nil {
		t.Fatal(err)
	}
	upload.RequestID = uuid.NewString()
	second, err := f.svc.UploadAttachmentDraft(f.ctx, upload)
	if err != nil {
		t.Fatal(err)
	}
	p := f.params(t)
	p.DraftID, p.AttachmentIDs = &upload.DraftID, []pgtype.UUID{first.ID}
	f.fx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=$1`, f.handler)
	_, err = f.svc.Create(f.ctx, p)
	wantRecipientError(t, err, http.StatusForbidden, "account_unavailable")
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_attachment WHERE draft_id=$1 AND issue_id IS NULL AND delivered_at IS NULL`, upload.DraftID) != 2 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_attachment_reference WHERE attachment_id=ANY($1::uuid[])`, []pgtype.UUID{first.ID, second.ID}) != 0 ||
		f.fx.Count(t, `SELECT count(*) FROM issue WHERE workspace_id=$1`, f.workspace) != 0 {
		t.Fatal("recipient refusal consumed uploads or left creation effects")
	}
	if _, err = f.svc.GetAttachment(f.ctx, pgtype.UUID{}, upload.DraftID, first.ID); err != nil {
		t.Fatalf("recipient refusal removed uploader access to the draft: %v", err)
	}
	corrected := p
	corrected.RequestID, corrected.HandlerID = uuid.NewString(), util.MustParseUUID(f.creator)
	created, err := f.svc.Create(f.ctx, corrected)
	if err != nil {
		t.Fatal(err)
	}
	f.fx.Exec(t, `UPDATE "user" SET disabled_at=NULL WHERE id=$1`, f.handler)
	// Revalidation of the consumed draft would change this to another error;
	// the original settled recipient refusal must win even after correction.
	_, err = f.svc.Create(f.ctx, p)
	wantRecipientError(t, err, http.StatusForbidden, "account_unavailable")
	remaining := corrected
	remaining.RequestID, remaining.AttachmentIDs = uuid.NewString(), []pgtype.UUID{second.ID}
	_, err = f.svc.Create(f.ctx, remaining)
	wantRecipientError(t, err, http.StatusConflict, "attachment_draft_consumed")
	result, err := f.svc.GetCreationResult(f.ctx, remaining.SourceWorkspaceID, remaining.RequestID)
	if err != nil || result.Code != "attachment_draft_consumed" || result.Outcome != "rejected" {
		t.Fatalf("consumed draft refusal was not durable: %+v %v", result, err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM issue WHERE workspace_id=$1`, f.workspace) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_attachment WHERE id=$1 AND issue_id=$2 AND delivered_at IS NOT NULL`, first.ID, created.Issue.ID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_attachment WHERE id=$1 AND issue_id IS NULL`, second.ID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM issue_participant WHERE user_id=$1`, f.handler) != 0 {
		t.Fatal("a sealed original request or a sibling draft file created another issue")
	}
}

func TestCollaborationCreationRejectionConcurrentOldAndCorrectedRequests(t *testing.T) {
	f := newCollaborationFixture(t)
	p := f.params(t)
	f.fx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=$1`, f.handler)
	start := make(chan struct{})
	var firstErrors [2]error
	var wg sync.WaitGroup
	for i := range firstErrors {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, firstErrors[i] = f.svc.Create(f.ctx, p)
		}()
	}
	close(start)
	wg.Wait()
	for _, err := range firstErrors {
		wantRecipientError(t, err, http.StatusForbidden, "account_unavailable")
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_operation WHERE scope_id=$1 AND request_id=$2`, f.workspace, p.RequestID) != 1 {
		t.Fatal("concurrent rejections did not share one operation")
	}
	f.fx.Exec(t, `UPDATE "user" SET disabled_at=NULL WHERE id=$1`, f.handler)
	corrected := p
	corrected.RequestID, corrected.HandlerID = uuid.NewString(), util.MustParseUUID(f.creator)
	start = make(chan struct{})
	var results [8]CollaborationIssueAccess
	var errs [8]error
	for i := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			request := p
			if i%2 == 1 {
				request = corrected
			}
			results[i], errs[i] = f.svc.Create(f.ctx, request)
		}()
	}
	close(start)
	wg.Wait()
	var id pgtype.UUID
	for i, result := range results {
		if i%2 == 0 {
			wantRecipientError(t, errs[i], http.StatusForbidden, "account_unavailable")
			if result.Issue.ID.Valid {
				t.Fatal("old refused request returned an issue")
			}
			continue
		}
		if errs[i] != nil || !result.Issue.ID.Valid || id.Valid && id != result.Issue.ID {
			t.Fatalf("corrected concurrent request changed identity: %v", errs[i])
		}
		id = result.Issue.ID
	}
	if f.fx.Count(t, `SELECT count(*) FROM issue WHERE workspace_id=$1`, f.workspace) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_operation WHERE scope_id=$1`, f.workspace) != 2 {
		t.Fatal("old and new concurrent requests duplicated creation or operations")
	}
}

func TestCollaborationCreationRejectionKeepsAdmissionFencesUntilCommit(t *testing.T) {
	for _, contender := range []string{"recipient", "source", "workspace", "gate", "result_reader"} {
		t.Run(contender, func(t *testing.T) {
			f := newCollaborationFixture(t)
			ctx, cancel := context.WithTimeout(f.ctx, 15*time.Second)
			defer cancel()
			p := f.params(t)
			p.AttachmentIDs = []pgtype.UUID{util.MustParseUUID(uuid.NewString())}
			release := make(chan struct{})
			var once sync.Once
			unblock := func() { once.Do(func() { close(release) }) }
			defer unblock()
			barrier := recipientCommitStarter{TxStarter: f.pool, reached: make(chan int, 1), release: release}
			svc := *f.svc
			svc.TxStarter = barrier
			written := make(chan error, 1)
			go func() {
				_, err := svc.Create(ctx, p)
				written <- err
			}()
			var blocker int
			select {
			case blocker = <-barrier.reached:
			case err := <-written:
				t.Fatalf("rejection did not reach its owning commit: %v", err)
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			done := make(chan error, 1)
			var recovered CollaborationCreationResult
			go func() {
				var err error
				switch contender {
				case "recipient":
					_, err = f.pool.Exec(ctx, `UPDATE "user" SET disabled_at=now() WHERE id=$1`, f.handler)
				case "source":
					_, err = f.pool.Exec(ctx, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, f.workspace, f.creator)
				case "workspace":
					_, err = f.svc.Queries.LockWorkspaceForDelete(ctx, p.SourceWorkspaceID)
				case "gate":
					_, err = f.pool.Exec(ctx, `UPDATE collaboration_execution_gate SET creation_enabled=false WHERE id=1`)
				case "result_reader":
					recovered, err = f.svc.GetCreationResult(ctx, p.SourceWorkspaceID, p.RequestID)
				}
				done <- err
			}()
			waitWorkflowBlocked(t, &collaborationQueueFixture{collaborationFixture: f}, blocker, done)
			unblock()
			wantRecipientError(t, <-written, http.StatusNotFound, "attachment_not_found")
			if err := <-done; err != nil {
				t.Fatalf("contender failed after the rejection committed: %v", err)
			}
			if contender == "result_reader" && (recovered.Outcome != "rejected" || recovered.Code != "attachment_not_found") {
				t.Fatal("reader failed to recover the refusal after waiting for its commit")
			}
			_, err := f.svc.Create(f.ctx, p)
			wantRecipientError(t, err, http.StatusNotFound, "attachment_not_found")
		})
	}
}

func TestCollaborationCreationRejectionBorrowedTransactionCanRollback(t *testing.T) {
	for _, reason := range []string{"recipient", "attachment"} {
		t.Run(reason, func(t *testing.T) {
			f, _ := newCollaborationAttachmentFixture(t)
			p := f.params(t)
			code, status := "account_unavailable", http.StatusForbidden
			if reason == "recipient" {
				f.fx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=$1`, f.handler)
			} else {
				p.AttachmentIDs = []pgtype.UUID{util.MustParseUUID(uuid.NewString())}
				code, status = "attachment_not_found", http.StatusNotFound
			}
			tx, err := f.pool.Begin(f.ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(context.Background())
			_, err = f.svc.CreateInTx(f.ctx, tx, p, IssueCountPolicy{})
			wantRecipientError(t, err, status, code)
			op, err := db.New(tx).GetCollaborationOperation(f.ctx, db.GetCollaborationOperationParams{ScopeKind: "create", ScopeID: p.SourceWorkspaceID, ActorUserID: util.MustParseUUID(f.creator), RequestID: p.RequestID})
			if err != nil || op.ResultCode.String != code || !op.CompletedAt.Valid {
				t.Fatalf("borrowed owner could not inspect its staged refusal: %v", err)
			}
			if err = tx.Rollback(f.ctx); err != nil {
				t.Fatal(err)
			}
			if f.fx.Count(t, `SELECT count(*) FROM issue WHERE workspace_id=$1`, f.workspace) != 0 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_operation WHERE scope_id=$1`, f.workspace) != 0 ||
				f.fx.Count(t, `SELECT issue_counter::bigint FROM workspace WHERE id=$1`, f.workspace) != 0 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_account_fence WHERE user_id=ANY($1::uuid[])`, []string{f.creator, f.handler}) != 0 {
				t.Fatal("borrowed rejection committed independently or retained attempted issue effects")
			}
			_, err = f.svc.GetCreationResult(f.ctx, p.SourceWorkspaceID, p.RequestID)
			wantRecipientError(t, err, http.StatusNotFound, "operation_not_found")
		})
	}
}

func TestCollaborationCreationResultRequiresOwnCurrentActor(t *testing.T) {
	f := newCollaborationFixture(t)
	p := f.params(t)
	f.fx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=$1`, f.handler)
	_, err := f.svc.Create(f.ctx, p)
	wantRecipientError(t, err, http.StatusForbidden, "account_unavailable")
	for _, query := range []struct {
		actor, source, key string
	}{
		{f.outsider, f.workspace, p.RequestID},
		{f.creator, uuid.NewString(), p.RequestID},
		{f.creator, f.workspace, uuid.NewString()},
	} {
		_, err := f.svc.GetCreationResult(collaborationContext(query.actor), util.MustParseUUID(query.source), query.key)
		wantRecipientError(t, err, http.StatusNotFound, "operation_not_found")
	}
	f.fx.Exec(t, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, f.workspace, f.creator)
	result, err := f.svc.GetCreationResult(f.ctx, p.SourceWorkspaceID, p.RequestID)
	if err != nil || result.Outcome != "rejected" || result.ActorUserID != util.MustParseUUID(f.creator) {
		t.Fatalf("source departure hid the caller's content-free settled result: %+v %v", result, err)
	}
	_, err = f.svc.GetCreation(f.ctx, p.SourceWorkspaceID, p.RequestID)
	wantRecipientError(t, err, http.StatusNotFound, "not_found")
	f.fx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=$1`, f.creator)
	_, err = f.svc.GetCreationResult(f.ctx, p.SourceWorkspaceID, p.RequestID)
	wantRecipientError(t, err, http.StatusForbidden, "forbidden")
}

func TestCollaborationCreationResultCreatedDoesNotGrantContentAccess(t *testing.T) {
	f := newCollaborationFixture(t)
	p := f.params(t)
	created, err := f.svc.Create(f.ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	f.fx.Exec(t, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, f.workspace, f.creator)
	f.fx.Exec(t, `UPDATE issue_participant SET revoked_at=now(),epoch=epoch+1 WHERE issue_id=$1 AND user_id=$2`, created.Issue.ID, f.creator)
	result, err := f.svc.GetCreationResult(f.ctx, p.SourceWorkspaceID, p.RequestID)
	if err != nil || result.Outcome != "created" || result.Code != "created" {
		t.Fatalf("own minimal success result could not be recovered: %+v %v", result, err)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), p.Description) || strings.Contains(string(encoded), p.Title) || strings.Contains(string(encoded), util.UUIDToString(created.Issue.ID)) {
		t.Fatal("minimal result exposed the inaccessible issue")
	}
	_, err = f.svc.GetCreation(f.ctx, p.SourceWorkspaceID, p.RequestID)
	wantRecipientError(t, err, http.StatusNotFound, "not_found")
	_, err = f.svc.Create(f.ctx, p)
	wantRecipientError(t, err, http.StatusNotFound, "not_found")
}

func TestCollaborationCreationResultAndReplayRevalidatePasswordActor(t *testing.T) {
	for _, outcome := range []string{"created", "rejected"} {
		t.Run(outcome, func(t *testing.T) {
			f := newCollaborationFixture(t)
			for _, user := range []string{f.creator, f.handler} {
				recipientCompleteCredential(t, f, user)
			}
			t.Setenv("MULTICA_AUTH_MODE", "password")
			ctx, p := recipientPasswordContext(f.creator), f.params(t)
			if outcome == "rejected" {
				f.fx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=$1`, f.handler)
			}
			_, err := f.svc.Create(ctx, p)
			if outcome == "rejected" {
				wantRecipientError(t, err, http.StatusForbidden, "account_unavailable")
			} else if err != nil {
				t.Fatal(err)
			}
			original, err := f.svc.GetCreationResult(ctx, p.SourceWorkspaceID, p.RequestID)
			if err != nil {
				t.Fatal(err)
			}
			issues := f.fx.Count(t, `SELECT count(*) FROM issue WHERE workspace_id=$1`, f.workspace)
			f.fx.Exec(t, `UPDATE user_password_credential SET session_version=2 WHERE user_id=$1`, f.creator)
			deny := func(ctx context.Context) {
				t.Helper()
				_, err := f.svc.Create(ctx, p)
				wantRecipientError(t, err, http.StatusForbidden, "forbidden")
				result, err := f.svc.GetCreationResult(ctx, p.SourceWorkspaceID, p.RequestID)
				wantRecipientError(t, err, http.StatusForbidden, "forbidden")
				if result != (CollaborationCreationResult{}) {
					t.Fatal("invalid current actor received a completed creation result")
				}
			}
			deny(ctx)
			deny(f.ctx)
			deny(auth.WithPasswordSession(f.ctx, auth.PasswordSession{UserID: f.handler, Version: 1, Kind: "jwt"}))
			fresh := auth.WithPasswordSession(f.ctx, auth.PasswordSession{UserID: f.creator, Version: 2, Kind: "jwt"})
			f.fx.Exec(t, `UPDATE user_password_credential SET must_change_password=true WHERE user_id=$1`, f.creator)
			deny(fresh)
			f.fx.Exec(t, `UPDATE user_password_credential SET must_change_password=false WHERE user_id=$1`, f.creator)
			result, err := f.svc.GetCreationResult(fresh, p.SourceWorkspaceID, p.RequestID)
			if err != nil || result != original {
				t.Fatalf("current password session did not recover the immutable result: %+v %v", result, err)
			}
			_, err = f.svc.Create(fresh, p)
			if outcome == "rejected" {
				wantRecipientError(t, err, http.StatusForbidden, "account_unavailable")
			} else if err != nil {
				t.Fatal(err)
			}
			if f.fx.Count(t, `SELECT count(*) FROM issue WHERE workspace_id=$1`, f.workspace) != issues ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_operation WHERE scope_id=$1`, f.workspace) != 1 {
				t.Fatal("completed-key credential checks changed the creation outcome")
			}
		})
	}
}

func TestCollaborationCreationRejectionAuthorizationHasNoSideEffects(t *testing.T) {
	for _, scenario := range []string{"machine", "unclassified", "agent_actor", "missing_actor", "disabled_actor", "non_member", "non_member_closed_gate", "missing_source", "stale_password"} {
		t.Run(scenario, func(t *testing.T) {
			f := newCollaborationFixture(t)
			p, ctx := f.params(t), f.ctx
			f.fx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=$1`, f.handler)
			switch scenario {
			case "machine":
				ctx = auth.WithAuthenticatedPrincipal(ctx, auth.AuthenticatedPrincipal{UserID: f.creator, Kind: auth.CredentialTask})
			case "unclassified":
				ctx = context.Background()
			case "agent_actor":
				ctx = auth.WithAuthenticatedPrincipal(ctx, auth.AuthenticatedPrincipal{UserID: f.creator, Kind: auth.CredentialJWT, AgentActor: true})
			case "missing_actor":
				ctx = collaborationContext(uuid.NewString())
			case "disabled_actor":
				f.fx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=$1`, f.creator)
			case "non_member", "non_member_closed_gate":
				f.fx.Exec(t, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, f.workspace, f.creator)
				if scenario == "non_member_closed_gate" {
					f.fx.Exec(t, `UPDATE collaboration_execution_gate SET creation_enabled=false WHERE id=1`)
				}
			case "missing_source":
				p.SourceWorkspaceID = util.MustParseUUID(uuid.NewString())
			case "stale_password":
				recipientCompleteCredential(t, f, f.creator)
				t.Setenv("MULTICA_AUTH_MODE", "password")
				ctx = auth.WithPasswordSession(f.ctx, auth.PasswordSession{UserID: f.creator, Version: 2, Kind: "jwt"})
			}
			_, err := f.svc.Create(ctx, p)
			wantRecipientError(t, err, http.StatusForbidden, "forbidden")
			if f.fx.Count(t, `SELECT count(*) FROM collaboration_operation WHERE scope_id=$1`, p.SourceWorkspaceID) != 0 ||
				f.fx.Count(t, `SELECT count(*) FROM issue WHERE workspace_id=$1`, f.workspace) != 0 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_account_fence WHERE user_id=ANY($1::uuid[])`, []string{f.creator, f.handler}) != 0 {
				t.Fatal("invalid actor/source created a refusal operation or gained authority")
			}
		})
	}
}

func TestCollaborationCreationRejectionSelectionAndSourceFailuresAreAtomic(t *testing.T) {
	for _, scenario := range []string{"invalid_selection", "missing_attachment", "wrong_draft", "missing_parent", "missing_project", "parent_triage", "capacity"} {
		t.Run(scenario, func(t *testing.T) {
			f, _ := newCollaborationAttachmentFixture(t)
			upload := attachmentUploadParams(f)
			file, err := f.svc.UploadAttachmentDraft(f.ctx, upload)
			if err != nil {
				t.Fatal(err)
			}
			p := f.params(t)
			p.DraftID, p.AttachmentIDs = &upload.DraftID, []pgtype.UUID{file.ID}
			code, status := "", http.StatusBadRequest
			fix := func() {}
			switch scenario {
			case "invalid_selection":
				p.AttachmentIDs = append(p.AttachmentIDs, file.ID)
				code = "invalid_attachment_selection"
			case "missing_attachment":
				p.AttachmentIDs = append(p.AttachmentIDs, util.MustParseUUID(uuid.NewString()))
				code, status = "attachment_not_found", http.StatusNotFound
			case "wrong_draft":
				wrong := util.MustParseUUID(uuid.NewString())
				p.DraftID = &wrong
				code, status = "attachment_scope_mismatch", http.StatusConflict
			case "missing_parent":
				p.ParentIssueID = util.MustParseUUID(uuid.NewString())
				fix = func() { f.fx.Issue(t, "Restored parent", testutil.Cols{"id": p.ParentIssueID}) }
				code = "parent_issue_not_found"
			case "missing_project":
				p.ProjectID = util.MustParseUUID(uuid.NewString())
				fix = func() { f.fx.Project(t, "Restored project", testutil.Cols{"id": p.ProjectID}) }
				code = "project_not_found"
			case "parent_triage":
				parent := f.fx.Issue(t, "Parent awaiting triage", testutil.Cols{"admission_status": "pending"})
				p.ParentIssueID = util.MustParseUUID(parent)
				fix = func() { f.fx.Exec(t, `UPDATE issue SET admission_status='accepted' WHERE id=$1`, parent) }
				code, status = "triage_review_required", http.StatusConflict
			case "capacity":
				f.fx.Issue(t, "Existing issue consumes capacity")
				limit := 1
				provider := entitlementtest.New()
				provider.Set(uuid.UUID(p.SourceWorkspaceID.Bytes), entitlement.GateIssueCount, entitlement.Decision{Gate: entitlement.Gate{Action: entitlement.ActionEnforce, Limit: &limit}})
				f.svc.Issues.Entitlements = provider
				fix = func() { f.svc.Issues.Entitlements = nil }
				code, status = "issue_limit_reached", http.StatusPaymentRequired
			}
			issues := f.fx.Count(t, `SELECT count(*) FROM issue WHERE workspace_id=$1`, f.workspace)
			counter := f.fx.Count(t, `SELECT issue_counter::bigint FROM workspace WHERE id=$1`, f.workspace)
			_, err = f.svc.Create(f.ctx, p)
			wantRecipientError(t, err, status, code)
			if f.fx.Count(t, `SELECT count(*) FROM issue WHERE workspace_id=$1`, f.workspace) != issues ||
				f.fx.Count(t, `SELECT issue_counter::bigint FROM workspace WHERE id=$1`, f.workspace) != counter ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_attachment WHERE id=$1 AND issue_id IS NULL AND delivered_at IS NULL`, file.ID) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_attachment_reference WHERE attachment_id=$1`, file.ID) != 0 ||
				f.fx.Count(t, `SELECT count(*) FROM issue_participant WHERE user_id=ANY($1::uuid[])`, []string{f.creator, f.handler}) != 0 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_notification WHERE recipient_user_id=$1`, f.handler) != 0 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE actor_user_id=$1`, f.creator) != 0 {
				t.Fatal("settled selection refusal left attempted creation effects")
			}
			fix()
			_, err = f.svc.Create(f.ctx, p)
			wantRecipientError(t, err, status, code)
			result, err := f.svc.GetCreationResult(f.ctx, p.SourceWorkspaceID, p.RequestID)
			if err != nil || result.Outcome != "rejected" || result.Code != code {
				t.Fatalf("selection refusal was not recoverable: %+v %v", result, err)
			}
		})
	}
}

func TestCollaborationCreationResultInvalidOrUnknownIsNotSettled(t *testing.T) {
	f := newCollaborationFixture(t)
	p := f.params(t)
	f.fx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=$1`, f.handler)
	_, err := f.svc.Create(f.ctx, p)
	wantRecipientError(t, err, http.StatusForbidden, "account_unavailable")
	original, err := f.svc.Queries.GetCollaborationOperation(f.ctx, db.GetCollaborationOperationParams{ScopeKind: "create", ScopeID: p.SourceWorkspaceID, ActorUserID: util.MustParseUUID(f.creator), RequestID: p.RequestID})
	if err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []string{"request", "source", "actor", "outcome", "code", "null_result", "resolved_issue", "unfinished"} {
		t.Run(scenario, func(t *testing.T) {
			var saved CollaborationCreationResult
			if err := json.Unmarshal(original.Result, &saved); err != nil {
				t.Fatal(err)
			}
			resolved, completed := original.ResolvedIssueID, original.CompletedAt
			code := original.ResultCode.String
			switch scenario {
			case "request":
				saved.RequestID = "private-request-sentinel"
			case "source":
				saved.SourceWorkspaceID = util.MustParseUUID(uuid.NewString())
			case "actor":
				saved.ActorUserID = util.MustParseUUID(f.outsider)
			case "outcome":
				saved.Outcome = "created"
			case "code":
				code, saved.Code = "private-code-sentinel", "private-code-sentinel"
			case "resolved_issue":
				resolved = util.MustParseUUID(uuid.NewString())
			case "unfinished":
				completed = pgtype.Timestamptz{}
			}
			encoded, err := json.Marshal(saved)
			if err != nil {
				t.Fatal(err)
			}
			if scenario == "null_result" {
				encoded = []byte("null")
			}
			f.fx.Exec(t, `UPDATE collaboration_operation SET result=$2, result_code=$3, resolved_issue_id=$4, completed_at=$5 WHERE id=$1`, original.ID, encoded, code, resolved, completed)
			out, err := f.svc.GetCreationResult(f.ctx, p.SourceWorkspaceID, p.RequestID)
			if err == nil || out != (CollaborationCreationResult{}) || strings.Contains(err.Error(), "sentinel") {
				t.Fatalf("invalid stored receipt became a settled or disclosed result: %+v %v", out, err)
			}
			if scenario == "unfinished" {
				wantRecipientError(t, err, http.StatusConflict, "operation_incomplete")
			}
		})
	}
	f.fx.Exec(t, `UPDATE collaboration_operation SET result=$2, result_code=$3, resolved_issue_id=$4, completed_at=$5 WHERE id=$1`, original.ID, original.Result, original.ResultCode, original.ResolvedIssueID, original.CompletedAt)
	f.svc.TxStarter = &queueFaultStarter{TxStarter: f.pool, lostCommit: true}
	result, err := f.svc.GetCreationResult(f.ctx, p.SourceWorkspaceID, p.RequestID)
	if err == nil || result != (CollaborationCreationResult{}) {
		t.Fatalf("unknown read completion returned a settled receipt: %+v %v", result, err)
	}
}

type collaborationCreationSavepointFaultStarter struct {
	TxStarter
	statement string
}

type collaborationCreationSavepointFaultTx struct {
	pgx.Tx
	statement string
}

func (s collaborationCreationSavepointFaultStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return collaborationCreationSavepointFaultTx{Tx: tx, statement: s.statement}, nil
}

func (tx collaborationCreationSavepointFaultTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if sql == tx.statement {
		return pgconn.CommandTag{}, errors.New("injected creation savepoint failure")
	}
	return tx.Tx.Exec(ctx, sql, args...)
}

func TestCollaborationCreationRejectionPersistenceFailureRemainsUnknown(t *testing.T) {
	for _, failure := range []string{"parent_query", "rejection_write", "savepoint", "rollback", "release"} {
		t.Run(failure, func(t *testing.T) {
			f := newCollaborationFixture(t)
			p := f.params(t)
			p.AttachmentIDs = []pgtype.UUID{util.MustParseUUID(uuid.NewString())}
			switch failure {
			case "parent_query":
				p.ParentIssueID = util.MustParseUUID(uuid.NewString())
				f.svc.TxStarter = collaborationFailureStarter{TxStarter: f.pool, queryName: "GetIssueInWorkspace"}
			case "rejection_write":
				f.svc.TxStarter = collaborationFailureStarter{TxStarter: f.pool, queryName: "CompleteCollaborationOperation"}
			default:
				statement := map[string]string{"savepoint": "SAVEPOINT collaboration_creation", "rollback": "ROLLBACK TO SAVEPOINT collaboration_creation", "release": "RELEASE SAVEPOINT collaboration_creation"}[failure]
				f.svc.TxStarter = collaborationCreationSavepointFaultStarter{TxStarter: f.pool, statement: statement}
			}
			_, err := f.svc.Create(f.ctx, p)
			var domain *CollaborationError
			if err == nil || errors.As(err, &domain) || errors.Is(err, ErrParentIssueNotFound) {
				t.Fatalf("database failure was treated as a settled business refusal: %v", err)
			}
			if f.fx.Count(t, `SELECT count(*) FROM issue WHERE workspace_id=$1`, f.workspace) != 0 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_operation WHERE scope_id=$1`, f.workspace) != 0 ||
				f.fx.Count(t, `SELECT issue_counter::bigint FROM workspace WHERE id=$1`, f.workspace) != 0 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_account_fence WHERE user_id=ANY($1::uuid[])`, []string{f.creator, f.handler}) != 0 {
				t.Fatal("failed refusal persistence retained attempted business writes")
			}
			f.svc.TxStarter = f.pool
			_, err = f.svc.GetCreationResult(f.ctx, p.SourceWorkspaceID, p.RequestID)
			wantRecipientError(t, err, http.StatusNotFound, "operation_not_found")
		})
	}
}

func TestIssueCreateInTxParentDistinguishesMissingFromDependencyFailure(t *testing.T) {
	for _, scenario := range []string{"missing", "dependency", "valid"} {
		t.Run(scenario, func(t *testing.T) {
			f := newCollaborationFixture(t)
			parent := uuid.NewString()
			if scenario != "missing" {
				parent = f.fx.Issue(t, "Existing ordinary parent")
				f.fx.Exec(t, `UPDATE workspace SET issue_counter=1 WHERE id=$1`, f.workspace)
			}
			before := f.fx.Count(t, `SELECT count(*) FROM issue WHERE workspace_id=$1`, f.workspace)
			tx, err := f.pool.Begin(f.ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(context.Background())
			writer := pgx.Tx(tx)
			if scenario == "dependency" {
				writer = collaborationFailureTx{Tx: tx, queryName: "GetIssueInWorkspace"}
			}
			result, err := f.svc.Issues.CreateInTx(f.ctx, writer, IssueCreateParams{WorkspaceID: util.MustParseUUID(f.workspace), ParentIssueID: util.MustParseUUID(parent), Title: "Ordinary child", Status: "todo", Priority: "none", CreatorType: "member", CreatorID: util.MustParseUUID(f.creator), AllowDuplicate: true}, IssueCountPolicy{})
			switch scenario {
			case "missing":
				if !errors.Is(err, ErrParentIssueNotFound) {
					t.Fatalf("actual absent parent lost its business refusal: %v", err)
				}
			case "dependency":
				if err == nil || errors.Is(err, ErrParentIssueNotFound) || !strings.Contains(err.Error(), "injected") {
					t.Fatalf("ordinary parent dependency failure was collapsed to missing: %v", err)
				}
			case "valid":
				if err != nil || result.Issue.ParentIssueID != util.MustParseUUID(parent) {
					t.Fatalf("valid ordinary parent creation regressed: %v", err)
				}
			}
			if err = tx.Rollback(f.ctx); err != nil {
				t.Fatal(err)
			}
			if f.fx.Count(t, `SELECT count(*) FROM issue WHERE workspace_id=$1`, f.workspace) != before {
				t.Fatal("borrowed ordinary creation committed independently")
			}
		})
	}
}
