package service

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/installation"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func startResourceCollaboration(t *testing.T, f *managedCollaborationFixture) (pgtype.UUID, protocol.CollaborationRunClaimResponse, protocol.CollaborationRunPermitRequest) {
	t.Helper()
	f.submitManaged(t, f.creator, "resource invalidation regression")
	f.resumeManaged(t)
	run, _, claim := f.claimManaged(t)
	permit := protocol.CollaborationRunPermitRequest{Identity: claim.Identity, AttemptSeq: "1", AttemptNonce: uuid.NewString(), Containment: protocol.CollaborationContainmentLinuxPIDNamespace}
	if _, err := f.svc.PermitRuntimeRun(f.daemonCtx, run, permit); err != nil {
		t.Fatal(err)
	}
	return run, claim, permit
}

func deleteCollaborationRuntime(ctx context.Context, f *managedCollaborationFixture, cancel bool) error {
	tx, err := f.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	q := f.svc.Queries.WithTx(tx)
	if _, err = TeardownRuntime(ctx, q, f.runtime.ID, RuntimeTeardownOptions{CancelNonTerminalTasks: cancel}); err != nil {
		return err
	}
	if err = q.DeleteAgentRuntime(ctx, f.runtime.ID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func resourceChannelState(t *testing.T, f *managedCollaborationFixture) string {
	t.Helper()
	var state string
	f.fx.QueryRow(t, `SELECT to_jsonb(c)::text FROM collaboration_channel c JOIN collaboration_target t ON t.channel_id=c.id WHERE t.id=$1`, f.targetID).Scan(&state)
	return state
}

func unrelatedResourceTarget(t *testing.T, f *managedCollaborationFixture) (db.CollaborationTarget, db.CollaborationGrant) {
	t.Helper()
	other := *f
	daemon := uuid.NewString()
	runtime := f.fx.Runtime(t, "Unrelated runtime", testutil.Cols{"owner_id": f.creator, "daemon_id": daemon, "provider": "codex", "status": "online"})
	binding := f.fx.Insert(t, "installation_daemon_binding", testutil.Cols{"installation_id": f.installation.ID, "workspace_id": f.workspace, "daemon_id": daemon, "principal_user_id": f.creator, "auth_version": 1, "binding_epoch": 7})
	var err error
	other.runtime, err = f.svc.Queries.GetAgentRuntime(f.ctx, util.MustParseUUID(runtime))
	if err != nil {
		t.Fatal(err)
	}
	other.binding, err = f.svc.Queries.GetInstallationBinding(f.ctx, util.MustParseUUID(binding))
	if err != nil {
		t.Fatal(err)
	}
	return registerManagedControlTarget(t, &other, false)
}

func TestCollaborationRuntimeTeardownStopsAndRetainsProof(t *testing.T) {
	f := newManagedCollaborationFixture(t)
	unrelated, grant := unrelatedResourceTarget(t, f)
	run, claim, permit := startResourceCollaboration(t, f)
	unrelated = queueTarget(t, f.collaborationQueueFixture, unrelated.ID)
	waiting := f.submitManaged(t, f.handler, "waiting input survives resource deletion")
	beforeChannel := resourceChannelState(t, f)
	if err := deleteCollaborationRuntime(context.Background(), f, true); err != nil {
		t.Fatal(err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM agent_runtime WHERE id=$1`, f.runtime.ID) != 0 ||
		f.fx.Count(t, `SELECT count(*) FROM agent WHERE id=$1 AND runtime_id IS NULL`, f.grant.AgentID) != 1 {
		t.Fatal("real teardown did not delete the runtime and preserve its user agent")
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_grant WHERE id=$1 AND revoked_at IS NOT NULL AND epoch=$2`, f.grant.ID, f.grant.Epoch+1) != 1 {
		t.Fatal("runtime teardown left its collaboration grant active")
	}
	if !reflect.DeepEqual(unrelated, queueTarget(t, f.collaborationQueueFixture, unrelated.ID)) ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_grant WHERE id=$1 AND revoked_at IS NULL AND epoch=$2`, grant.ID, grant.Epoch) != 1 {
		t.Fatal("runtime deletion changed an unrelated target or binding")
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_target t WHERE id=$1 AND to_jsonb(t)->>'hold_reason'='grant_required'`, f.targetID) != 1 || resourceChannelState(t, f) != beforeChannel {
		t.Fatal("resource-only invalidation must hold its target without changing channel control or its live barrier")
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='waiting'`, waiting.CommandID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_runtime_reservation WHERE run_id=$1 AND released_at IS NULL`, run) != 1 {
		t.Fatal("teardown consumed waiting input or released an unproven reservation")
	}
	intent, err := f.svc.RuntimeStopIntent(context.Background(), run, claim.TerminationToken)
	if err != nil || !intent.StopRequested || intent.Identity != claim.Identity {
		t.Fatalf("teardown did not retain the original supervisor stop identity: %+v %v", intent, err)
	}
	proof := managedProof(claim.Identity, permit.AttemptNonce)
	if _, err = f.svc.AcceptRuntimeProof(context.Background(), run, claim.TerminationToken, proof); err != nil {
		t.Fatalf("real resource deletion stranded historical proof: %v", err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND forced_disposition='stopped' AND proof_state='exited_proven' AND settled_at IS NOT NULL`, run) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_runtime_reservation WHERE run_id=$1 AND released_at IS NOT NULL`, run) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_target t WHERE id=$1 AND to_jsonb(t)->>'hold_reason'='grant_required'`, f.targetID) != 1 {
		t.Fatal("proof must settle only the old run while retaining target hold")
	}
}

func TestCollaborationRuntimeTeardownFindsRetainedRunWithoutGrant(t *testing.T) {
	f := newManagedCollaborationFixture(t)
	run, claim, permit := startResourceCollaboration(t, f)
	f.fx.Exec(t, `DELETE FROM collaboration_grant WHERE id=$1`, f.grant.ID)
	if err := deleteCollaborationRuntime(context.Background(), f, true); err != nil {
		t.Fatal(err)
	}
	intent, err := f.svc.RuntimeStopIntent(context.Background(), run, claim.TerminationToken)
	if err != nil || !intent.StopRequested || intent.Identity != claim.Identity {
		t.Fatalf("retained run dependency was omitted without a current grant row: %+v %v", intent, err)
	}
	if _, err = f.svc.AcceptRuntimeProof(context.Background(), run, claim.TerminationToken, managedProof(claim.Identity, permit.AttemptNonce)); err != nil {
		t.Fatalf("missing current grant stranded historical proof: %v", err)
	}
}

func TestCollaborationRuntimeTeardownRefusesBusySourceWithoutPartialChanges(t *testing.T) {
	for _, table := range []string{"issue", "collaboration_channel", "collaboration_target", "collaboration_grant", "collaboration_command", "collaboration_execution_group", "collaboration_run", "collaboration_termination"} {
		t.Run(table, func(t *testing.T) {
			f := newManagedCollaborationFixture(t)
			run, claim, _ := startResourceCollaboration(t, f)
			id, field := f.issue.Issue.ID, "id"
			switch table {
			case "collaboration_channel":
				id = util.MustParseUUID(claim.Identity.ChannelID)
			case "collaboration_target":
				id = f.targetID
			case "collaboration_grant":
				id = f.grant.ID
			case "collaboration_command":
				stored, err := f.svc.Queries.GetCollaborationRuntimeRun(f.ctx, run)
				if err != nil {
					t.Fatal(err)
				}
				id = stored.CommandID
			case "collaboration_execution_group":
				id = util.MustParseUUID(claim.Identity.GroupID)
			case "collaboration_run":
				id = run
			case "collaboration_termination":
				id, field = run, "run_id"
			}
			lock, err := f.pool.Begin(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			defer lock.Rollback(context.Background())
			if _, err = lock.Exec(context.Background(), "SELECT 1 FROM "+table+" WHERE "+field+"=$1 FOR UPDATE", id); err != nil {
				t.Fatal(err)
			}
			before := controlStateSnapshot(t, f.collaborationQueueFixture)
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			err = deleteCollaborationRuntime(ctx, f, true)
			var conflict *pgconn.PgError
			if !errors.As(err, &conflict) || conflict.Code != "55P03" {
				t.Fatalf("resource-first teardown must return NOWAIT conflict, never wait or commit: %v", err)
			}
			if controlStateSnapshot(t, f.collaborationQueueFixture) != before ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_grant WHERE id=$1 AND revoked_at IS NULL AND epoch=$2`, f.grant.ID, f.grant.Epoch) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM agent_runtime WHERE id=$1`, f.runtime.ID) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM agent WHERE id=$1 AND runtime_id=$2`, f.grant.AgentID, f.runtime.ID) != 1 {
				t.Fatal("busy source left a partial resource/grant/control mutation")
			}
		})
	}
}

func TestCollaborationRuntimeRetentionGCRequiresRunProof(t *testing.T) {
	f := newManagedCollaborationFixture(t)
	run, _, _ := startResourceCollaboration(t, f)
	// The ordinary task ledger is not the collaboration launch ledger. Neither
	// absent engine tasks nor terminal projections prove its process has exited.
	f.fx.Exec(t, `UPDATE agent_task_queue SET status='cancelled',completed_at=now() WHERE id=(SELECT task_id FROM collaboration_run WHERE id=$1)`, run)
	before := controlStateSnapshot(t, f.collaborationQueueFixture)
	if err := deleteCollaborationRuntime(context.Background(), f, false); !errors.Is(err, ErrRuntimeNotDrained) {
		t.Fatalf("retention GC ignored a possibly-live collaboration reservation: %v", err)
	}
	if controlStateSnapshot(t, f.collaborationQueueFixture) != before ||
		f.fx.Count(t, `SELECT count(*) FROM agent_runtime WHERE id=$1`, f.runtime.ID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_grant WHERE id=$1 AND revoked_at IS NULL`, f.grant.ID) != 1 {
		t.Fatal("refused retention GC changed resource or collaboration state")
	}
}

func resourceBindingProof(t *testing.T, f *managedCollaborationFixture, version int64, purpose string) (*ManagedInstallationService, installation.Proof, context.Context) {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	f.fx.Exec(t, `UPDATE managed_installation SET public_key=$2,key_fingerprint=$3 WHERE id=$1`, f.installation.ID, []byte(public), installation.Digest(public))
	f.fx.Cleanup(t, `DELETE FROM installation_challenge WHERE installation_id=$1`, f.installation.ID)
	f.fx.Cleanup(t, `DELETE FROM daemon_token WHERE installation_binding_id IN (SELECT id FROM installation_daemon_binding WHERE installation_id=$1)`, f.installation.ID)
	f.fx.Cleanup(t, `DELETE FROM admin_audit_event WHERE target_kind='installation' AND target_id=$1`, f.installation.ID)
	f.fx.Cleanup(t, `DELETE FROM installation_daemon_binding WHERE installation_id=$1`, f.installation.ID)
	management := NewManagedInstallationService(f.svc.Queries, f.pool, auth.ManagedDeploymentID(), true)
	jwt := auth.WithPasswordSession(context.Background(), auth.PasswordSession{UserID: f.creator, Version: version, Kind: "jwt"})
	caller := auth.WithPasswordSession(context.Background(), auth.PasswordSession{UserID: f.creator, Version: version, Kind: "pat"})
	if purpose == "renew" {
		jwt, caller = f.daemonCtx, f.daemonCtx
	}
	epoch := strconv.FormatInt(f.binding.BindingEpoch, 10)
	challenge, err := management.IssueChallenge(jwt, InstallationChallengeParams{Purpose: purpose, PublicKey: base64.RawURLEncoding.EncodeToString(public), InstallationID: util.UUIDToString(f.installation.ID), WorkspaceID: f.workspace, DaemonID: f.binding.DaemonID, ExpectedBindingEpoch: &epoch})
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := base64.RawURLEncoding.DecodeString(challenge.SignaturePayload)
	if err != nil {
		t.Fatal(err)
	}
	proof := installation.Proof{ChallengeID: challenge.ChallengeID, SignaturePayload: challenge.SignaturePayload, BodyPayload: challenge.BodyPayload, Signature: base64.RawURLEncoding.EncodeToString(ed25519.Sign(private, canonical))}
	return management, proof, caller
}

func TestCollaborationManagedBindingReplacementStopsOnlyOldAuthority(t *testing.T) {
	f := newManagedCollaborationFixture(t)
	unrelated, unrelatedGrant := unrelatedResourceTarget(t, f)
	run, claim, permit := startResourceCollaboration(t, f)
	unrelated = queueTarget(t, f.collaborationQueueFixture, unrelated.ID)
	beforeChannel := resourceChannelState(t, f)
	f.fx.Exec(t, `UPDATE user_password_credential SET session_version=2 WHERE user_id=$1`, f.creator)
	management, proof, caller := resourceBindingProof(t, f, 2, "bind")
	rebound, err := management.Bind(caller, proof)
	if err != nil {
		t.Fatal(err)
	}
	if rebound.BindingID == util.UUIDToString(f.binding.ID) || rebound.BindingEpoch != "2" {
		t.Fatal("test did not replace the actual managed binding")
	}
	if !reflect.DeepEqual(unrelated, queueTarget(t, f.collaborationQueueFixture, unrelated.ID)) ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_grant WHERE id=$1 AND revoked_at IS NULL AND epoch=$2`, unrelatedGrant.ID, unrelatedGrant.Epoch) != 1 {
		t.Fatal("binding replacement revoked another namespace or binding epoch")
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_grant WHERE id=$1 AND revoked_at IS NOT NULL`, f.grant.ID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_target t WHERE id=$1 AND to_jsonb(t)->>'hold_reason'='config_invalid'`, f.targetID) != 1 ||
		resourceChannelState(t, f) != beforeChannel {
		t.Fatal("managed rebind must revoke old consent and hold its target without pausing the channel")
	}
	intent, err := f.svc.RuntimeStopIntent(context.Background(), run, claim.TerminationToken)
	if err != nil || !intent.StopRequested || intent.Identity != claim.Identity {
		t.Fatalf("rebind lost the original stop identity: %+v %v", intent, err)
	}
	if _, err = f.svc.AcceptRuntimeProof(context.Background(), run, claim.TerminationToken, managedProof(claim.Identity, permit.AttemptNonce)); err != nil {
		t.Fatal(err)
	}
	f.binding, err = f.svc.Queries.GetInstallationBinding(context.Background(), util.MustParseUUID(rebound.BindingID))
	if err != nil {
		t.Fatal(err)
	}
	f.ctx = collaborationPasswordContext(util.MustParseUUID(f.creator), 2)
	fresh := consentManagedControlTarget(t, f, queueTarget(t, f.collaborationQueueFixture, f.targetID), f.grant.AgentID)
	before := controlStateSnapshot(t, f.collaborationQueueFixture)
	replay, err := management.Bind(caller, proof)
	if err != nil || !reflect.DeepEqual(replay, rebound) || before != controlStateSnapshot(t, f.collaborationQueueFixture) ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_grant WHERE id=$1 AND revoked_at IS NULL`, fresh.ID) != 1 {
		t.Fatalf("original rebind replay invalidated freshly consented authority: %v", err)
	}
}

func TestCollaborationManagedBindingRenewalPreservesAuthority(t *testing.T) {
	for _, purpose := range []string{"bind", "renew"} {
		t.Run(purpose, func(t *testing.T) {
			f := newManagedCollaborationFixture(t)
			startResourceCollaboration(t, f)
			management, proof, caller := resourceBindingProof(t, f, 1, purpose)
			before := controlStateSnapshot(t, f.collaborationQueueFixture)
			var result InstallationBindingResult
			var err error
			if purpose == "renew" {
				result, err = management.Renew(caller, proof)
			} else {
				result, err = management.Bind(caller, proof)
			}
			if err != nil || result.BindingID != util.UUIDToString(f.binding.ID) || result.BindingEpoch != "1" || before != controlStateSnapshot(t, f.collaborationQueueFixture) ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_grant WHERE id=$1 AND revoked_at IS NULL`, f.grant.ID) != 1 {
				t.Fatalf("same-authority credential refresh disturbed active collaboration: %v", err)
			}
		})
	}
}

func TestCollaborationManagedBindingRollbackAndUnknownCommit(t *testing.T) {
	for _, lostCommit := range []bool{false, true} {
		name := "event_failure"
		if lostCommit {
			name = "unknown_commit"
		}
		t.Run(name, func(t *testing.T) {
			f := newManagedCollaborationFixture(t)
			run, claim, _ := startResourceCollaboration(t, f)
			f.fx.Exec(t, `UPDATE user_password_credential SET session_version=2 WHERE user_id=$1`, f.creator)
			management, proof, caller := resourceBindingProof(t, f, 2, "bind")
			before := controlStateSnapshot(t, f.collaborationQueueFixture)
			fault := &queueFaultStarter{TxStarter: f.pool, eventFailure: !lostCommit, lostCommit: lostCommit}
			management.TxStarter = fault
			if _, err := management.Bind(caller, proof); err == nil || IsRuntimeResourceBusy(err) {
				t.Fatalf("failed or unknown commit must remain a non-retryable operation result: %v", err)
			}
			if fault.begins.Load() != 1 {
				t.Fatal("binding automatically replayed an event failure or uncertain commit")
			}
			if !lostCommit && (before != controlStateSnapshot(t, f.collaborationQueueFixture) ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_grant WHERE id=$1 AND revoked_at IS NULL`, f.grant.ID) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM installation_daemon_binding WHERE id=$1 AND state='active'`, f.binding.ID) != 1) {
				t.Fatal("event failure committed binding, grant or source effects separately")
			}
			management.TxStarter = f.pool
			result, err := management.Bind(caller, proof)
			if err != nil || result.BindingEpoch != "2" {
				t.Fatalf("original signed proof could not reconcile the binding: %v", err)
			}
			intent, err := f.svc.RuntimeStopIntent(context.Background(), run, claim.TerminationToken)
			if err != nil || !intent.StopRequested || intent.Identity != claim.Identity ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_grant WHERE id=$1 AND epoch=$2 AND revoked_at IS NOT NULL`, f.grant.ID, f.grant.Epoch+1) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1 AND kind='resource_invalidated'`, f.issue.Issue.ID) != 1 {
				t.Fatal("binding reconciliation duplicated invalidation or changed historical stop identity")
			}
		})
	}
}

func TestCollaborationManagedBindingRetriesOnlyAfterSourceRollback(t *testing.T) {
	f := newManagedCollaborationFixture(t)
	startResourceCollaboration(t, f)
	f.fx.Exec(t, `UPDATE user_password_credential SET session_version=2 WHERE user_id=$1`, f.creator)
	management, proof, caller := resourceBindingProof(t, f, 2, "bind")
	blocker, err := f.pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer blocker.Rollback(context.Background())
	if _, err = blocker.Exec(context.Background(), `SELECT id FROM issue WHERE id=$1 FOR UPDATE`, f.issue.Issue.ID); err != nil {
		t.Fatal(err)
	}
	before := controlStateSnapshot(t, f.collaborationQueueFixture)
	gate := &fieldRetryGate{TxStarter: f.pool, ready: make(chan struct{}), release: make(chan struct{})}
	defer func() {
		select {
		case <-gate.release:
		default:
			close(gate.release)
		}
	}()
	management.TxStarter = gate
	ctx, cancel := context.WithTimeout(caller, 10*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := management.Bind(ctx, proof); done <- err }()
	select {
	case <-gate.ready:
	case <-ctx.Done():
		t.Fatal("binding waited on a source lock while holding its namespace")
	}
	if before != controlStateSnapshot(t, f.collaborationQueueFixture) ||
		f.fx.Count(t, `SELECT count(*) FROM installation_daemon_binding WHERE id=$1 AND state='active'`, f.binding.ID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_grant WHERE id=$1 AND revoked_at IS NULL`, f.grant.ID) != 1 {
		t.Fatal("first binding attempt retained mutations after rollback")
	}
	if err = blocker.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	close(gate.release)
	select {
	case err = <-done:
		if err != nil || gate.begins.Load() != 2 {
			t.Fatalf("released source did not allow exactly one fresh whole-transaction retry: begins=%d, %v", gate.begins.Load(), err)
		}
	case <-ctx.Done():
		t.Fatal("binding did not recover after source conflict was removed")
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1 AND kind='resource_invalidated'`, f.issue.Issue.ID) != 1 {
		t.Fatal("resource retry duplicated committed source effects")
	}
}
