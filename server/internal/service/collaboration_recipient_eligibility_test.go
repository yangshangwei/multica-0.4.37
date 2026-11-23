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
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/util"
)

func recipientCompleteCredential(t *testing.T, f *collaborationFixture, user string) {
	t.Helper()
	recipientCredential(t, f, user, "recipient_"+strings.ReplaceAll(uuid.NewString(), "-", "")[:16], false)
}

func recipientPasswordContext(user string) context.Context {
	return auth.WithPasswordSession(collaborationContext(user), auth.PasswordSession{UserID: user, Version: 1, Kind: "jwt"})
}

// Durable refusal receipts are allowed; business effects and history are not.
func recipientBusinessState(t *testing.T, f *collaborationQueueFixture) string {
	t.Helper()
	var snapshot map[string]json.RawMessage
	if err := json.Unmarshal([]byte(recipientState(t, f)), &snapshot); err != nil {
		t.Fatal(err)
	}
	delete(snapshot, "operations")
	handoffs, err := json.Marshal(f.fx.Count(t, `SELECT count(*) FROM issue_handoff WHERE issue_id=$1`, f.issue.Issue.ID))
	if err != nil {
		t.Fatal(err)
	}
	snapshot["handoffs"] = handoffs
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func TestCollaborationRecipientKnownUUIDCannotBypassNewPasswordRestriction(t *testing.T) {
	for _, mode := range []string{"legacy", "password"} {
		for _, action := range []string{"invite", "transfer", "information", "return", "acceptance"} {
			t.Run(mode+"/"+action, func(t *testing.T) {
				f := newCollaborationHandoffFixture(t)
				for _, user := range []string{f.creator, f.handler, f.outsider} {
					recipientCompleteCredential(t, f.collaborationFixture, user)
				}
				t.Setenv("MULTICA_AUTH_MODE", mode)
				f.ctx = recipientPasswordContext(f.creator)
				actor, target := f.creator, f.outsider
				request := workflowRequest(f, 1)
				var write func() (CollaborationWorkflowReceipt, error)
				wantCode := "target_unavailable"
				switch action {
				case "invite":
					write = func() (CollaborationWorkflowReceipt, error) {
						return f.svc.InviteParticipant(recipientPasswordContext(actor), CollaborationParticipantParams{CollaborationWorkflowRequest: request, TargetUserID: util.MustParseUUID(target)})
					}
				case "transfer":
					write = func() (CollaborationWorkflowReceipt, error) {
						return f.svc.Transfer(recipientPasswordContext(actor), CollaborationTransferParams{CollaborationParticipantParams: CollaborationParticipantParams{CollaborationWorkflowRequest: request, TargetUserID: util.MustParseUUID(target)}})
					}
				case "information":
					p := handoffAsk(t, f, 1, target, "Need recipient input")
					write = func() (CollaborationWorkflowReceipt, error) {
						return f.svc.RequestInformation(recipientPasswordContext(actor), p)
					}
				case "return":
					if _, err := f.svc.RequestInformation(recipientPasswordContext(f.handler), handoffAsk(t, f, 1, f.creator, "Please clarify")); err != nil {
						t.Fatal(err)
					}
					target, wantCode = f.handler, "return_target_unavailable"
					p := handoffReturn(t, f, 2, activeInformationRequest(t, f), "todo")
					write = func() (CollaborationWorkflowReceipt, error) {
						return f.svc.ReturnInformation(recipientPasswordContext(actor), p)
					}
				case "acceptance":
					actor, target = f.handler, f.creator
					p := handoffAcceptance(t, f, 1, "Ready to review")
					write = func() (CollaborationWorkflowReceipt, error) {
						return f.svc.SubmitAcceptance(recipientPasswordContext(actor), p)
					}
				}
				resolved, err := f.svc.ResolveRecipient(recipientPasswordContext(actor), CollaborationRecipientQuery{Kind: "user_id", Value: target, IssueID: f.issue.Issue.ID})
				if err != nil || resolved.Recipient.UserID != util.MustParseUUID(target) {
					t.Fatalf("eligible target was not confirmed before restriction: %+v, %v", resolved, err)
				}
				f.fx.Exec(t, `UPDATE user_password_credential SET must_change_password=true WHERE user_id=$1`, target)
				before := recipientBusinessState(t, f)
				_, err = write()
				wantRecipientError(t, err, http.StatusConflict, wantCode)
				if recipientBusinessState(t, f) != before {
					t.Fatal("restricted target changed business state before refusal")
				}
				// A refused intent cannot become effective after account recovery.
				f.fx.Exec(t, `UPDATE user_password_credential SET must_change_password=false WHERE user_id=$1`, target)
				_, err = write()
				wantRecipientError(t, err, http.StatusConflict, wantCode)
				if recipientBusinessState(t, f) != before {
					t.Fatal("same refused key applied a handoff after account recovery")
				}
			})
		}
	}
}

func TestCollaborationRecipientSetupAccountCannotReceiveKnownUUIDWrites(t *testing.T) {
	for _, action := range []string{"invite", "transfer", "information"} {
		t.Run(action, func(t *testing.T) {
			f := newCollaborationHandoffFixture(t)
			recipientCompleteCredential(t, f.collaborationFixture, f.creator)
			t.Setenv("MULTICA_AUTH_MODE", "password")
			f.ctx = recipientPasswordContext(f.creator)
			p := CollaborationParticipantParams{CollaborationWorkflowRequest: workflowRequest(f, 1), TargetUserID: util.MustParseUUID(f.outsider)}
			before := recipientBusinessState(t, f)
			var err error
			switch action {
			case "invite":
				_, err = f.svc.InviteParticipant(f.ctx, p)
			case "transfer":
				_, err = f.svc.Transfer(f.ctx, CollaborationTransferParams{CollaborationParticipantParams: p})
			case "information":
				_, err = f.svc.RequestInformation(f.ctx, handoffAsk(t, f, 1, f.outsider, "Set up first"))
			}
			wantRecipientError(t, err, http.StatusConflict, "target_unavailable")
			if recipientBusinessState(t, f) != before {
				t.Fatal("setup-only target received membership or responsibility")
			}
		})
	}
}

func TestCollaborationRecipientCreationRejectsRestrictedAndSetupHandlers(t *testing.T) {
	for _, state := range []string{"restricted", "setup"} {
		t.Run(state, func(t *testing.T) {
			f := newCollaborationFixture(t)
			recipientCompleteCredential(t, f, f.creator)
			t.Setenv("MULTICA_AUTH_MODE", "password")
			ctx := recipientPasswordContext(f.creator)
			if state == "restricted" {
				recipientCompleteCredential(t, f, f.handler)
				if _, err := f.svc.ResolveRecipient(ctx, CollaborationRecipientQuery{Kind: "user_id", Value: f.handler, SourceWorkspaceID: util.MustParseUUID(f.workspace)}); err != nil {
					t.Fatal(err)
				}
				f.fx.Exec(t, `UPDATE user_password_credential SET must_change_password=true WHERE user_id=$1`, f.handler)
			}
			p := f.params(t)
			_, err := f.svc.Create(ctx, p)
			wantRecipientError(t, err, http.StatusForbidden, "account_unavailable")
			if f.fx.Count(t, `SELECT count(*) FROM issue WHERE workspace_id=$1`, f.workspace) != 0 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_account_fence WHERE user_id=ANY($1::uuid[])`, []string{f.creator, f.handler}) != 0 {
				t.Fatal("ineligible creation handler left a partial issue or authority")
			}
			if f.fx.Count(t, `SELECT count(*) FROM collaboration_operation WHERE scope_kind='create' AND scope_id=$1 AND actor_user_id=$2 AND request_id=$3 AND result_code='account_unavailable' AND resolved_issue_id IS NULL AND completed_at IS NOT NULL`, f.workspace, f.creator, p.RequestID) != 1 {
				t.Fatal("ineligible creation handler did not seal its original request")
			}
		})
	}
}

func recipientKnownUUIDAction(t *testing.T, f *collaborationQueueFixture, action, requestID string) func(context.Context, *CollaborationService) error {
	t.Helper()
	p := CollaborationParticipantParams{CollaborationWorkflowRequest: workflowRequest(f, 1), TargetUserID: util.MustParseUUID(f.outsider)}
	p.RequestID = requestID
	switch action {
	case "invite":
		return func(ctx context.Context, svc *CollaborationService) error {
			_, err := svc.InviteParticipant(ctx, p)
			return err
		}
	case "transfer":
		return func(ctx context.Context, svc *CollaborationService) error {
			_, err := svc.Transfer(ctx, CollaborationTransferParams{CollaborationParticipantParams: p})
			return err
		}
	case "create":
		creation := f.params(t)
		creation.RequestID, creation.HandlerID = requestID, p.TargetUserID
		return func(ctx context.Context, svc *CollaborationService) error {
			_, err := svc.Create(ctx, creation)
			return err
		}
	default:
		t.Fatalf("unknown recipient test action %q", action)
		return nil
	}
}

func TestCollaborationRecipientEligibleLegacyAndPasswordControls(t *testing.T) {
	for _, mode := range []string{"legacy", "password"} {
		for _, action := range []string{"create", "invite", "transfer"} {
			t.Run(mode+"/"+action, func(t *testing.T) {
				f := newCollaborationHandoffFixture(t)
				if mode == "password" {
					recipientCompleteCredential(t, f.collaborationFixture, f.creator)
					recipientCompleteCredential(t, f.collaborationFixture, f.outsider)
				}
				t.Setenv("MULTICA_AUTH_MODE", mode)
				write := recipientKnownUUIDAction(t, f, action, uuid.NewString())
				if err := write(recipientPasswordContext(f.creator), f.svc); err != nil {
					t.Fatalf("eligible %s account cannot receive %s: %v", mode, action, err)
				}
				if f.fx.Count(t, `SELECT count(*) FROM issue_participant WHERE user_id=$1 AND revoked_at IS NULL`, f.outsider) != 1 {
					t.Fatal("valid recipient did not join exactly one collaboration")
				}
			})
		}
	}
}

func TestCollaborationRecipientCompletedWritesReplayAfterTargetRestriction(t *testing.T) {
	for _, action := range []string{"create", "invite", "transfer"} {
		t.Run(action, func(t *testing.T) {
			f := newCollaborationHandoffFixture(t)
			recipientCompleteCredential(t, f.collaborationFixture, f.creator)
			recipientCompleteCredential(t, f.collaborationFixture, f.outsider)
			t.Setenv("MULTICA_AUTH_MODE", "password")
			ctx := recipientPasswordContext(f.creator)
			write := recipientKnownUUIDAction(t, f, action, uuid.NewString())
			if err := write(ctx, f.svc); err != nil {
				t.Fatal(err)
			}
			f.fx.Exec(t, `UPDATE user_password_credential SET must_change_password=true WHERE user_id=$1`, f.outsider)
			f.fx.Exec(t, `UPDATE collaboration_execution_gate SET creation_enabled=false WHERE id=1`)
			before := recipientState(t, f)
			issues := f.fx.Count(t, `SELECT count(*) FROM issue WHERE workspace_id=$1`, f.workspace)
			if err := write(ctx, f.svc); err != nil {
				t.Fatalf("completed %s key was re-admitted against a now-restricted target or gate: %v", action, err)
			}
			if recipientState(t, f) != before || f.fx.Count(t, `SELECT count(*) FROM issue WHERE workspace_id=$1`, f.workspace) != issues {
				t.Fatal("completed-key replay rewrote or duplicated its prior business result")
			}
		})
	}
}

type recipientEligibilityFaultStarter struct {
	TxStarter
	fired *bool
}

type recipientEligibilityFaultTx struct {
	pgx.Tx
	fired *bool
}

func (s recipientEligibilityFaultStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return recipientEligibilityFaultTx{Tx: tx, fired: s.fired}, nil
}

func (tx recipientEligibilityFaultTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if strings.Contains(sql, "-- name: FindCollaborationRecipients ") {
		*tx.fired = true
		return nil, errors.New("recipient credential dependency unavailable")
	}
	return tx.Tx.Query(ctx, sql, args...)
}

func TestCollaborationRecipientWriterDependencyFailureRollsBackAndKeepsOriginalKey(t *testing.T) {
	for _, action := range []string{"create", "invite", "transfer"} {
		t.Run(action, func(t *testing.T) {
			f := newCollaborationHandoffFixture(t)
			write := recipientKnownUUIDAction(t, f, action, uuid.NewString())
			fired := false
			svc := *f.svc
			svc.TxStarter = recipientEligibilityFaultStarter{TxStarter: f.pool, fired: &fired}
			before := recipientState(t, f)
			issues := f.fx.Count(t, `SELECT count(*) FROM issue WHERE workspace_id=$1`, f.workspace)
			err := write(f.ctx, &svc)
			if err == nil || !fired || collaborationErrorCode(err, "target_unavailable") || collaborationErrorCode(err, "account_unavailable") {
				t.Fatalf("credential query failure did not propagate as unavailable: %v (injected=%v)", err, fired)
			}
			if recipientState(t, f) != before || f.fx.Count(t, `SELECT count(*) FROM issue WHERE workspace_id=$1`, f.workspace) != issues {
				t.Fatal("credential query failure left a partial operation or business mutation")
			}
			if err := write(f.ctx, f.svc); err != nil {
				t.Fatalf("rolled-back dependency failure consumed the original request key: %v", err)
			}
		})
	}
}

func TestCollaborationRecipientEligibilityDoesNotChangeExistingNotificationRecipients(t *testing.T) {
	f := newCollaborationHandoffFixture(t)
	recipientCredential(t, f.collaborationFixture, f.handler, "notice_"+strings.ReplaceAll(uuid.NewString(), "-", "")[:16], true)
	p := CollaborationParticipantParams{CollaborationWorkflowRequest: workflowRequest(f, 1), TargetUserID: util.MustParseUUID(f.outsider)}
	if _, err := f.svc.InviteParticipant(f.ctx, p); err != nil {
		t.Fatal(err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_notification WHERE issue_id=$1 AND recipient_user_id=$2 AND kind='participant_added'`, f.issue.Issue.ID, f.handler) != 1 {
		t.Fatal("recipient admission silently changed existing participant notification fanout")
	}
}

type recipientCommitStarter struct {
	TxStarter
	reached chan int
	release <-chan struct{}
}

type recipientCommitTx struct {
	pgx.Tx
	reached chan int
	release <-chan struct{}
}

func (s recipientCommitStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return recipientCommitTx{Tx: tx, reached: s.reached, release: s.release}, nil
}

func (tx recipientCommitTx) Commit(ctx context.Context) error {
	tx.reached <- int(tx.Conn().PgConn().PID())
	select {
	case <-tx.release:
		return tx.Tx.Commit(ctx)
	case <-ctx.Done():
		return ctx.Err()
	}
}

func TestCollaborationRecipientWritersSerializeWithActualPasswordRecovery(t *testing.T) {
	for _, action := range []string{"create", "invite", "transfer"} {
		for _, first := range []string{"password", "collaboration"} {
			t.Run(action+"/"+first+"_first", func(t *testing.T) {
				f := newCollaborationHandoffFixture(t)
				recipientCompleteCredential(t, f.collaborationFixture, f.creator)
				recipientCompleteCredential(t, f.collaborationFixture, f.outsider)
				f.fx.Cleanup(t, `DELETE FROM admin_operation WHERE target_id=$1`, f.outsider)
				f.fx.Cleanup(t, `DELETE FROM admin_audit_event WHERE target_id=$1`, f.outsider)
				f.fx.Cleanup(t, `DELETE FROM collaboration_account_invalidation WHERE user_id=$1`, f.outsider)
				t.Setenv("MULTICA_AUTH_MODE", "password")
				ctx, cancel := context.WithTimeout(recipientPasswordContext(f.creator), 15*time.Second)
				defer cancel()
				p := DeploymentPasswordRecoveryParams{TargetUserID: util.MustParseUUID(f.outsider), PasswordHash: "test-only-precomputed-recovery-hash",
					Reason: "Recipient restriction ordering test", RequestID: uuid.NewString()}
				write := recipientKnownUUIDAction(t, f, action, uuid.NewString())
				before := recipientBusinessState(t, f)
				issues := f.fx.Count(t, `SELECT count(*) FROM issue WHERE workspace_id=$1`, f.workspace)
				if first == "password" {
					tx, err := f.pool.Begin(ctx)
					if err != nil {
						t.Fatal(err)
					}
					defer tx.Rollback(context.Background())
					if _, err = RecoverPasswordForDeploymentInTx(ctx, tx, p); err != nil {
						t.Fatal(err)
					}
					done := make(chan error, 1)
					go func() { done <- write(ctx, f.svc) }()
					waitWorkflowBlocked(t, f, int(tx.Conn().PgConn().PID()), done)
					if err = tx.Commit(ctx); err != nil {
						t.Fatal(err)
					}
					err = <-done
					status, code := http.StatusConflict, "target_unavailable"
					if action == "create" {
						status, code = http.StatusForbidden, "account_unavailable"
					}
					wantRecipientError(t, err, status, code)
					if recipientBusinessState(t, f) != before || f.fx.Count(t, `SELECT count(*) FROM issue WHERE workspace_id=$1`, f.workspace) != issues {
						t.Fatal("writer used credential availability from before waiting on password recovery")
					}
					return
				}
				release := make(chan struct{})
				var releaseOnce sync.Once
				unblock := func() { releaseOnce.Do(func() { close(release) }) }
				defer unblock()
				barrier := recipientCommitStarter{TxStarter: f.pool, reached: make(chan int, 1), release: release}
				svc := *f.svc
				svc.TxStarter = barrier
				written := make(chan error, 1)
				go func() { written <- write(ctx, &svc) }()
				var blocker int
				select {
				case blocker = <-barrier.reached:
				case err := <-written:
					t.Fatalf("writer did not reach commit: %v", err)
				case <-ctx.Done():
					t.Fatal(ctx.Err())
				}
				recovered := make(chan error, 1)
				go func() {
					tx, err := f.pool.Begin(ctx)
					if err != nil {
						recovered <- err
						return
					}
					defer tx.Rollback(context.Background())
					if _, err = RecoverPasswordForDeploymentInTx(ctx, tx, p); err == nil {
						err = tx.Commit(ctx)
					}
					recovered <- err
				}()
				waitWorkflowBlocked(t, f, blocker, recovered)
				unblock()
				if err := <-written; err != nil {
					t.Fatalf("eligible recipient write failed before password recovery: %v", err)
				}
				if err := <-recovered; err != nil {
					t.Fatal(err)
				}
				credential, err := f.svc.Queries.GetPasswordCredential(ctx, p.TargetUserID)
				if err != nil || !credential.MustChangePassword || credential.SessionVersion != 2 {
					t.Fatalf("actual recovery did not restrict the target after writer commit: %+v, %v", credential, err)
				}
			})
		}
	}
}
