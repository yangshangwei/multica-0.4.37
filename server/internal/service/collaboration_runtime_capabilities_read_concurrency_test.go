package service

import (
	"context"
	"reflect"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type stageCapabilitiesReadFixture struct {
	*stageRuntimeFixture
	daemon       *managedCollaborationFixture
	registration protocol.CollaborationRuntimeCapabilitiesRequest
	capabilities protocol.CollaborationRuntimeCapabilitiesResponse
	evidence     CollaborationStageCodeEvidenceView
}

// The capability GET and R4 use the same real QA account. The persisted
// managed namespace has no run; this fixture proves read authority, not Host
// process execution or the holder of a previously observed runtime failure.
func newStageCapabilitiesReadFixture(t *testing.T) *stageCapabilitiesReadFixture {
	t.Helper()
	f := newStageRuntimeFixture(t, true, stageArtifactReadFixture)
	f.fx.Exec(t, `UPDATE member SET role='admin' WHERE workspace_id=$1 AND user_id=$2`, f.workspace, f.stage.qa)
	daemon := ownerSwitchNamespace(t, f.managedCollaborationFixture, f.stage.qa, "QA capability reader")
	registration := preparationRegistrationRequest(daemon)
	if _, err := daemon.svc.RegisterRuntimeCapabilities(daemon.daemonCtx, registration); err != nil {
		t.Fatal(err)
	}
	daemon.runtime = mustRuntime(t, daemon.svc.Queries, daemon.runtime.ID)
	capabilities, err := daemon.svc.RuntimeCapabilities(daemon.daemonCtx, daemon.runtime.ID)
	if err != nil || capabilities.Registration == nil || capabilities.RuntimeID != util.UUIDToString(daemon.runtime.ID) ||
		capabilities.RuntimeConfigEpoch != "2" || !reflect.DeepEqual(capabilities.Registration.Capabilities, registration.Capabilities) {
		t.Fatalf("actual capability registration/read prerequisite: %+v %v", capabilities, err)
	}
	evidence := f.stage.evidence(t, f.stage.qaCtx, f.candidate)
	if evidence.Candidate.CandidateID != util.UUIDToString(f.candidate.CandidateID) || evidence.Candidate.Baseline == nil || evidence.Candidate.Baseline.Artifact == nil {
		t.Fatal("QA read prerequisite did not retain the exact currently readable artifact-bound candidate")
	}
	session, ok := auth.PasswordSessionFromContext(daemon.daemonCtx)
	if !ok || session.UserID != f.stage.qa || daemon.runtime.OwnerID != util.MustParseUUID(f.stage.qa) ||
		session.BindingID != util.UUIDToString(daemon.binding.ID) || daemon.binding.PrincipalUserID != daemon.runtime.OwnerID {
		t.Fatal("capability/R4 fixture did not bind its real daemon credential to the same QA account")
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE session_id=$1`, f.issue.SessionID) != 0 {
		t.Fatal("capability read fixture unexpectedly created a run")
	}
	return &stageCapabilitiesReadFixture{stageRuntimeFixture: f, daemon: daemon, registration: registration, capabilities: capabilities, evidence: evidence}
}

type stageCapabilitiesReadResult struct {
	response protocol.CollaborationRuntimeCapabilitiesResponse
	err      error
}

type stageCapabilitiesHeldRequest struct {
	pid     uint32
	release func()
	done    <-chan stageCapabilitiesReadResult
}

// LockAgentRuntime is the actual next query after LockManagedRuntime has
// finished account authorization and acquired its original user fence.
// The passthrough barrier does not substitute any SQL result or permission.
func holdStageCapabilitiesRequest(t *testing.T, f *stageCapabilitiesReadFixture, write bool) stageCapabilitiesHeldRequest {
	t.Helper()
	ctx, cancel := context.WithTimeout(f.daemon.daemonCtx, 5*time.Second)
	reached, resume := make(chan uint32, 1), make(chan struct{})
	done := make(chan stageCapabilitiesReadResult, 1)
	var once sync.Once
	release := func() { once.Do(func() { close(resume) }) }
	t.Cleanup(func() { release(); cancel(); <-done })
	service := *f.daemon.svc
	service.TxStarter = stageProjectionBarrierStarter{TxStarter: f.pool, needle: "LockAgentRuntime", reached: reached, resume: resume}
	go func() {
		defer close(done)
		var response protocol.CollaborationRuntimeCapabilitiesResponse
		var err error
		if write {
			response, err = service.RegisterRuntimeCapabilities(ctx, f.registration)
		} else {
			response, err = service.RuntimeCapabilities(ctx, f.daemon.runtime.ID)
		}
		done <- stageCapabilitiesReadResult{response: response, err: err}
	}()
	select {
	case pid := <-reached:
		return stageCapabilitiesHeldRequest{pid: pid, release: release, done: done}
	case result := <-done:
		t.Fatalf("actual capability request did not reach its post-account-fence query: response=%+v err=%v", result.response, result.err)
	case <-ctx.Done():
		t.Fatal("capability request did not reach its barrier within the unchanged five-second deadline")
	}
	return stageCapabilitiesHeldRequest{}
}

func TestCollaborationRuntimeCapabilitiesReadThenStageEvidence(t *testing.T) {
	f := newStageCapabilitiesReadFixture(t)
	// Start the second deadline first so the held request's expiry cannot
	// accidentally release its account fence and make the old read pass.
	ctx, cancel := context.WithTimeout(f.stage.qaCtx, 5*time.Second)
	defer cancel()
	held := holdStageCapabilitiesRequest(t, f, false)
	began := make(chan uint32, 1)
	reader := *f.svc
	reader.TxStarter = stageProjectionBarrierStarter{TxStarter: f.pool, began: began}
	view, err := reader.StageCodeEvidence(ctx, f.issue.Issue.ID, f.candidate.CandidateID)
	var secondPID uint32
	select {
	case secondPID = <-began:
	default:
		t.Fatal("R4 did not begin its actual PostgreSQL transaction")
	}
	held.release()
	first := <-held.done
	if secondPID == held.pid {
		t.Fatal("capability/R4 fixture did not use distinct PostgreSQL connections")
	}
	if err != nil {
		t.Fatalf("same-QA R4 contended with a held capability GET: SQLSTATE=%s err=%v", stagePgCode(err), err)
	}
	if view.Candidate.CandidateID != f.evidence.Candidate.CandidateID || !reflect.DeepEqual(view.Candidate.Baseline, f.evidence.Candidate.Baseline) {
		t.Fatalf("concurrent R4 lost the exact currently readable artifact candidate: %+v", view.Candidate)
	}
	if first.err != nil || !reflect.DeepEqual(first.response, f.capabilities) {
		t.Fatalf("held capability GET changed its original registration/epoch: %+v %v", first.response, first.err)
	}
}

func TestCollaborationRuntimeCapabilitiesStageEvidenceThenRead(t *testing.T) {
	f := newStageCapabilitiesReadFixture(t)
	ctx, cancel := context.WithTimeout(f.daemon.daemonCtx, 5*time.Second)
	defer cancel()
	qa := *f.stage
	qa.reviewerCtx = f.stage.qaCtx
	held := holdStageArtifactR4(t, &qa, f.candidate)
	began := make(chan uint32, 1)
	reader := *f.daemon.svc
	reader.TxStarter = stageProjectionBarrierStarter{TxStarter: f.pool, began: began}
	response, err := reader.RuntimeCapabilities(ctx, f.daemon.runtime.ID)
	var secondPID uint32
	select {
	case secondPID = <-began:
	default:
		t.Fatal("capability GET did not begin its actual PostgreSQL transaction")
	}
	// The capability GET must finish before the real R4 fence is released.
	held.release()
	first := <-held.done
	if secondPID == held.pid {
		t.Fatal("R4/capability fixture did not use distinct PostgreSQL connections")
	}
	if err != nil {
		t.Fatalf("same-QA capability GET blocked behind an authorized R4: SQLSTATE=%s err=%v", stagePgCode(err), err)
	}
	if !reflect.DeepEqual(response, f.capabilities) {
		t.Fatalf("concurrent capability GET changed its original registration/epoch: %+v", response)
	}
	if first.err != nil || first.view.Candidate.CandidateID != f.evidence.Candidate.CandidateID || !reflect.DeepEqual(first.view.Candidate.Baseline, f.evidence.Candidate.Baseline) {
		t.Fatalf("held R4 lost its exact authorized candidate during capability read: %+v %v", first.view.Candidate, first.err)
	}
}

func TestCollaborationRuntimeCapabilitiesReadOriginalWriterFence(t *testing.T) {
	f := newStageCapabilitiesReadFixture(t)
	// Use an actual changed summary rather than a declarative no-op so the
	// control also observes the original registration writer's final commit.
	f.registration.ExpectedRuntimeConfigEpoch = f.capabilities.RuntimeConfigEpoch
	f.registration.Capabilities.Limits.FileCount--
	before := f.fx.Count(t, `SELECT count(*) FROM collaboration_config_invalidation WHERE resource_kind='runtime' AND resource_id=$1`, f.daemon.runtime.ID)
	held := holdStageCapabilitiesRequest(t, f, true)
	ctx, cancel := context.WithTimeout(f.stage.qaCtx, 5*time.Second)
	defer cancel()
	began := make(chan uint32, 1)
	reader := *f.svc
	reader.TxStarter = stageProjectionBarrierStarter{TxStarter: f.pool, began: began}
	view, err := reader.StageCodeEvidence(ctx, f.issue.Issue.ID, f.candidate.CandidateID)
	var secondPID uint32
	select {
	case secondPID = <-began:
	default:
		t.Fatal("registration-writer control did not begin its actual R4 transaction")
	}
	held.release()
	writer := <-held.done
	if secondPID == held.pid {
		t.Fatal("registration-writer/R4 control did not use distinct PostgreSQL connections")
	}
	if stagePgCode(err) != "55P03" || !reflect.DeepEqual(view, CollaborationStageCodeEvidenceView{}) {
		t.Fatalf("R4 crossed the original registration writer fence or exposed private evidence: SQLSTATE=%s view=%+v", stagePgCode(err), view.Candidate)
	}
	epoch, parseErr := strconv.ParseInt(f.capabilities.RuntimeConfigEpoch, 10, 64)
	if parseErr != nil || writer.err != nil || writer.response.RuntimeConfigEpoch != strconv.FormatInt(epoch+1, 10) || writer.response.Registration == nil ||
		!reflect.DeepEqual(writer.response.Registration.Capabilities, f.registration.Capabilities) {
		t.Fatalf("original registration writer did not commit its exact new summary: %+v %v", writer.response, writer.err)
	}
	if after := f.fx.Count(t, `SELECT count(*) FROM collaboration_config_invalidation WHERE resource_kind='runtime' AND resource_id=$1`, f.daemon.runtime.ID); after != before+1 {
		t.Fatalf("original registration writer produced %d new invalidations, want one", after-before)
	}
	current, readErr := f.daemon.svc.RuntimeCapabilities(f.daemon.daemonCtx, f.daemon.runtime.ID)
	if readErr != nil || !reflect.DeepEqual(current, writer.response) {
		t.Fatalf("fresh capability GET did not recover the committed writer summary: %+v %v", current, readErr)
	}
}

func TestCollaborationRuntimeCapabilitiesReadCurrentAuthority(t *testing.T) {
	for _, name := range []string{"password_changed", "disabled_account", "binding_revoked", "wrong_binding"} {
		t.Run(name, func(t *testing.T) {
			f := newStageCapabilitiesReadFixture(t)
			ctx := f.daemon.daemonCtx
			switch name {
			case "password_changed":
				f.fx.Exec(t, `UPDATE user_password_credential SET session_version=session_version+1 WHERE user_id=$1`, f.daemon.runtime.OwnerID)
			case "disabled_account":
				f.fx.Exec(t, `UPDATE "user" SET disabled_at=clock_timestamp() WHERE id=$1`, f.daemon.runtime.OwnerID)
			case "binding_revoked":
				f.fx.Exec(t, `UPDATE installation_daemon_binding SET state='revoked' WHERE id=$1`, f.daemon.binding.ID)
			case "wrong_binding":
				// This is the other real daemon's credential, not a forged
				// claimed identity for the QA runtime's binding.
				ctx = f.managedCollaborationFixture.daemonCtx
			}
			response, err := f.daemon.svc.RuntimeCapabilities(ctx, f.daemon.runtime.ID)
			if !collaborationErrorCode(err, "managed_runtime_required") || !reflect.DeepEqual(response, protocol.CollaborationRuntimeCapabilitiesResponse{}) {
				t.Fatalf("current %s boundary exposed a managed capability summary: %+v %v", name, response, err)
			}
		})
	}
}
