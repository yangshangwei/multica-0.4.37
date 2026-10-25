package service

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func enableHostCutover(t *testing.T, f *managedCollaborationFixture) {
	t.Helper()
	gate, err := f.svc.Queries.GetCollaborationGate(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	f.fx.Cleanup(t, `UPDATE collaboration_execution_gate SET host_execution_enabled=$1 WHERE id=1`, gate.HostExecutionEnabled)
	f.fx.Exec(t, `UPDATE collaboration_execution_gate SET host_execution_enabled=true,epoch=epoch+1 WHERE id=1`)
	f.gateEpoch = gate.Epoch + 1
}

func TestCollaborationHostCutoverDoesNotPromoteNewNamespaceRoots(t *testing.T) {
	for _, prepared := range []bool{false, true} {
		name := "legacy_no_preparation"
		if prepared {
			name = "legacy_bundle"
		}
		t.Run(name, func(t *testing.T) {
			f := newManagedCollaborationFixture(t)
			if prepared {
				registerPreparationFixture(t, f)
				refreshPreparationConsent(t, f)
			}
			enableHostCutover(t, f)
			waiting := f.submitManaged(t, f.handler, "requires owner-confirmed host capability after cutover")
			f.resumeManaged(t)
			listed, err := f.svc.ListRuntimeRuns(f.daemonCtx, f.runtime.ID)
			if err != nil && !collaborationErrorCode(err, "execution_mode_unsupported") {
				t.Fatal(err)
			}
			if len(listed.RunIDs) != 0 || f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE session_id=$1`, f.issue.SessionID) != 0 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_runtime_reservation WHERE runtime_id=$1`, f.runtime.ID) != 0 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='waiting'`, waiting.CommandID) != 1 {
				t.Fatal("host cutover promoted new namespace work or consumed its waiting input")
			}
		})
	}
}

func TestCollaborationHostCutoverRejectsUnclaimedNamespaceRootWithoutReinterpretation(t *testing.T) {
	f := newManagedCollaborationFixture(t)
	f.submitManaged(t, f.handler, "prepared before cutover but never claimed")
	f.resumeManaged(t)
	listed, err := f.svc.ListRuntimeRuns(f.daemonCtx, f.runtime.ID)
	if err != nil || len(listed.RunIDs) != 1 {
		t.Fatalf("pre-cutover reservation: %+v %v", listed, err)
	}
	id := util.MustParseUUID(listed.RunIDs[0])
	enableHostCutover(t, f)
	request := preparationClaimRequest(f)
	request.PreparationVersion, request.BundleDigest = 0, ""
	if _, err = f.svc.ClaimRuntimeRun(f.daemonCtx, id, request); !collaborationErrorCode(err, "execution_mode_unsupported") {
		t.Fatalf("cutover admitted an unclaimed namespace root: %v", err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE id=$1 AND containment IS NULL AND prompt_snapshot IS NULL AND authorized_attempt_seq=0`, id) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_termination WHERE run_id=$1`, id) != 0 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_attempt WHERE run_id=$1`, id) != 0 {
		t.Fatal("refused legacy claim acquired an execution identity or was silently converted to host mode")
	}
	// The already reserved record retains its identity. Existing human Stop
	// can prove this zero-permit record unlaunched without contacting a daemon.
	if _, err = f.svc.StopQueue(f.ctx, f.issueControl(t)); err != nil {
		t.Fatal(err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_runtime_reservation WHERE run_id=$1 AND released_at IS NOT NULL`, id) != 1 {
		t.Fatal("pre-cutover zero-permit reservation could not drain through ordinary Stop")
	}
}

func TestCollaborationHostCutoverDrainsAlreadyClaimedNamespaceSquad(t *testing.T) {
	f, member := newManagedSquadFixture(t)
	f.submitManaged(t, f.handler, "old Linux squad already claimed before cutover")
	f.resumeManaged(t)
	root, request, claim := f.claimManaged(t)
	token, nonce := permitSquadRun(t, f, root, claim)
	dispatch, err := f.svc.CreateRuntimeComment(context.Background(), root, token, CollaborationRuntimeCommentParams{CommentID: util.MustParseUUID(uuid.NewString()),
		Content: "[@member](mention://agent/" + util.UUIDToString(member) + ") finish retained work"})
	if err != nil || len(dispatch.RunIDs) != 1 {
		t.Fatalf("old Linux member admission: %+v %v", dispatch, err)
	}
	enableHostCutover(t, f)
	replayed, err := f.svc.ClaimRuntimeRun(f.daemonCtx, root, request)
	if err != nil || replayed.Identity != claim.Identity || replayed.TerminationToken != claim.TerminationToken || replayed.ProtocolVersion != 0 {
		t.Fatalf("cutover changed an already claimed namespace root: %v", err)
	}
	finishSquadRun(t, f, root, claim, token, nonce, "success")
	memberRun := dispatch.RunIDs[0]
	memberClaim, err := f.svc.ClaimRuntimeRun(f.daemonCtx, memberRun, request)
	if err != nil {
		t.Fatalf("cutover stranded the retained group's Linux member: %v", err)
	}
	memberToken, memberNonce := permitSquadRun(t, f, memberRun, memberClaim)
	finishSquadRun(t, f, memberRun, memberClaim, memberToken, memberNonce, "success")
	continuation, _, continuationClaim := f.claimManaged(t)
	continuationToken, continuationNonce := permitSquadRun(t, f, continuation, continuationClaim)
	finishSquadRun(t, f, continuation, continuationClaim, continuationToken, continuationNonce, "success")
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE session_id=$1 AND containment=$2 AND proof_state='exited_proven' AND settled_at IS NOT NULL`,
		f.issue.SessionID, protocol.CollaborationContainmentLinuxPIDNamespace) != 3 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE session_id=$1 AND containment=$2`, f.issue.SessionID, protocol.CollaborationContainmentHostProcessSupervision) != 0 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_execution_group WHERE session_id=$1 AND state='succeeded' AND settled_at IS NOT NULL`, f.issue.SessionID) != 1 {
		t.Fatal("retained Linux group was converted, stranded or released without each original proof")
	}
}
