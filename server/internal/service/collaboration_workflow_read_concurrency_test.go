package service

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/util"
)

type workflowReceiptReadResult struct {
	receipt CollaborationWorkflowOperationReceipt
	err     error
}

type workflowHeldReceiptRead struct {
	pid     uint32
	release func()
	done    <-chan workflowReceiptReadResult
}

// The existing barrier forwards all SQL and holds the actual account fence
// after authorization, before looking up the actor's immutable operation.
func holdWorkflowReceiptRead(t *testing.T, f *stageReviewFixture, request CollaborationWorkflowRequest) workflowHeldReceiptRead {
	t.Helper()
	ctx, cancel := context.WithTimeout(f.reviewerCtx, 5*time.Second)
	reached, resume := make(chan uint32, 1), make(chan struct{})
	done := make(chan workflowReceiptReadResult, 1)
	var once sync.Once
	release := func() { once.Do(func() { close(resume) }) }
	t.Cleanup(func() { release(); cancel(); <-done })
	reader := *f.svc
	reader.TxStarter = stageProjectionBarrierStarter{TxStarter: f.pool, needle: "GetCollaborationOperation", reached: reached, resume: resume}
	go func() {
		defer close(done)
		receipt, err := reader.GetWorkflowOperation(ctx, request.IssueID, request.RequestID)
		done <- workflowReceiptReadResult{receipt: receipt, err: err}
	}()
	select {
	case pid := <-reached:
		return workflowHeldReceiptRead{pid: pid, release: release, done: done}
	case result := <-done:
		t.Fatalf("receipt reader did not reach its post-account-authorization barrier: %v", result.err)
	case <-ctx.Done():
		t.Fatal("receipt reader did not reach its barrier within the unchanged five-second deadline")
	}
	return workflowHeldReceiptRead{}
}

func TestCollaborationWorkflowReceiptReadThenStageEvidence(t *testing.T) {
	f := newStageReviewFixture(t)
	stageArtifactReadFixture(t, f)
	candidate, request, original := f.approved(t)
	prior := f.evidence(t, f.reviewerCtx, candidate)
	if prior.Candidate.Baseline == nil || prior.Candidate.Baseline.Artifact == nil {
		t.Fatal("receipt/R4 fixture did not retain an actual artifact-bound candidate")
	}
	ctx, cancel := context.WithTimeout(f.reviewerCtx, 5*time.Second)
	defer cancel()
	held := holdWorkflowReceiptRead(t, f, request.CollaborationWorkflowRequest)
	began := make(chan uint32, 1)
	reader := *f.svc
	reader.TxStarter = stageProjectionBarrierStarter{TxStarter: f.pool, began: began}
	view, err := reader.StageCodeEvidence(ctx, f.issue.Issue.ID, candidate.CandidateID)
	var secondPID uint32
	select {
	case secondPID = <-began:
	default:
		t.Fatal("stage reader did not begin its actual PostgreSQL transaction")
	}
	held.release()
	first := <-held.done
	if secondPID == held.pid {
		t.Fatal("receipt/R4 fixture did not use distinct PostgreSQL connections")
	}
	if err != nil {
		t.Fatalf("same-actor R4 contended with a held immutable receipt reader: SQLSTATE=%s err=%v", stagePgCode(err), err)
	}
	if view.Candidate.CandidateID != prior.Candidate.CandidateID || !reflect.DeepEqual(view.Candidate.Baseline, prior.Candidate.Baseline) {
		t.Fatalf("concurrent R4 lost its exact currently readable artifact candidate: %+v", view.Candidate)
	}
	if first.err != nil || first.receipt.Status != original.Status || first.receipt.Code != original.Code || first.receipt.CollaborationRevision != original.CollaborationRevision ||
		!reflect.DeepEqual(first.receipt.StageConfirmation, original.StageConfirmation) {
		t.Fatalf("held receipt read changed the original actor's minimal result: %+v %v", first.receipt, first.err)
	}
}

func TestCollaborationWorkflowStageEvidenceThenReceiptRead(t *testing.T) {
	f := newStageReviewFixture(t)
	stageArtifactReadFixture(t, f)
	candidate, request, original := f.approved(t)
	prior := f.evidence(t, f.reviewerCtx, candidate)
	if prior.Candidate.Baseline == nil || prior.Candidate.Baseline.Artifact == nil {
		t.Fatal("R4/receipt fixture did not retain an actual artifact-bound candidate")
	}
	ctx, cancel := context.WithTimeout(f.reviewerCtx, 5*time.Second)
	defer cancel()
	held := holdStageArtifactR4(t, f, candidate)
	began := make(chan uint32, 1)
	reader := *f.svc
	reader.TxStarter = stageProjectionBarrierStarter{TxStarter: f.pool, began: began}
	receipt, err := reader.GetWorkflowOperation(ctx, request.IssueID, request.RequestID)
	var secondPID uint32
	select {
	case secondPID = <-began:
	default:
		t.Fatal("receipt reader did not begin its actual PostgreSQL transaction")
	}
	// The immutable receipt must finish before the held R4 is released.
	held.release()
	first := <-held.done
	if secondPID == held.pid {
		t.Fatal("R4/receipt fixture did not use distinct PostgreSQL connections")
	}
	if err != nil {
		t.Fatalf("same-actor receipt read blocked behind an authorized R4 reader: SQLSTATE=%s err=%v", stagePgCode(err), err)
	}
	if receipt.Status != original.Status || receipt.Code != original.Code || receipt.CollaborationRevision != original.CollaborationRevision ||
		!reflect.DeepEqual(receipt.StageConfirmation, original.StageConfirmation) {
		t.Fatalf("concurrent receipt read changed the original minimal result: %+v", receipt)
	}
	if first.err != nil || first.view.Candidate.CandidateID != prior.Candidate.CandidateID || !reflect.DeepEqual(first.view.Candidate.Baseline, prior.Candidate.Baseline) {
		t.Fatalf("held R4 lost its exact authorized candidate during receipt recovery: %+v %v", first.view.Candidate, first.err)
	}
}

func TestCollaborationWorkflowReceiptReadOriginalWriterFence(t *testing.T) {
	f := newStageReviewFixture(t)
	stageArtifactReadFixture(t, f)
	candidate, _, _ := f.approved(t)
	request := f.reviewParams(t, candidate, "return")
	ctx, cancel := context.WithTimeout(f.reviewerCtx, 5*time.Second)
	defer cancel()
	reached, resume := make(chan uint32, 1), make(chan struct{})
	done := make(chan struct {
		receipt CollaborationWorkflowReceipt
		err     error
	}, 1)
	var once sync.Once
	release := func() { once.Do(func() { close(resume) }) }
	t.Cleanup(func() { release(); cancel(); <-done })
	writer := *f.svc
	writer.TxStarter = stageProjectionBarrierStarter{TxStarter: f.pool, needle: "GetCollaborationStageCodeHead", reached: reached, resume: resume}
	go func() {
		defer close(done)
		receipt, err := writer.DecideStage(ctx, request)
		done <- struct {
			receipt CollaborationWorkflowReceipt
			err     error
		}{receipt: receipt, err: err}
	}()
	var writerPID uint32
	select {
	case writerPID = <-reached:
	case result := <-done:
		t.Fatalf("original decision writer did not reach its post-user-lock barrier: receipt=%+v err=%v", result.receipt, result.err)
	case <-ctx.Done():
		t.Fatal("decision writer did not reach its barrier within the unchanged five-second deadline")
	}
	readCtx, readCancel := context.WithTimeout(f.reviewerCtx, 5*time.Second)
	defer readCancel()
	began := make(chan uint32, 1)
	reader := *f.svc
	reader.TxStarter = stageProjectionBarrierStarter{TxStarter: f.pool, began: began}
	view, err := reader.StageCodeEvidence(readCtx, f.issue.Issue.ID, candidate.CandidateID)
	var readerPID uint32
	select {
	case readerPID = <-began:
	default:
		t.Fatal("writer-fence control did not begin its actual R4 transaction")
	}
	release()
	result := <-done
	if readerPID == writerPID {
		t.Fatal("writer/R4 control did not use distinct PostgreSQL connections")
	}
	if stagePgCode(err) != "55P03" || !reflect.DeepEqual(view, CollaborationStageCodeEvidenceView{}) {
		t.Fatalf("R4 crossed the original writer's exclusive account fence or exposed partial evidence: SQLSTATE=%s view=%+v", stagePgCode(err), view.Candidate)
	}
	if result.err != nil || result.receipt.Status != http.StatusOK || result.receipt.StageConfirmation == nil || result.receipt.StageConfirmation.Decision != "returned" ||
		result.receipt.StageConfirmation.CandidateID != util.UUIDToString(request.Candidate.CandidateID) {
		t.Fatalf("original decision writer did not commit its exact return after reader refusal: %+v %v", result.receipt, result.err)
	}
}

func TestCollaborationWorkflowReceiptReadCurrentAuthority(t *testing.T) {
	t.Run("wrong_actor", func(t *testing.T) {
		f := newStageReviewFixture(t)
		_, request, original := f.approved(t)
		receipt, err := f.svc.GetWorkflowOperation(f.ctx, request.IssueID, request.RequestID)
		var refused *CollaborationError
		if !errors.As(err, &refused) || refused.Status != http.StatusNotFound || refused.Code != "not_found" || !reflect.DeepEqual(receipt, CollaborationWorkflowOperationReceipt{}) {
			t.Fatalf("another current actor read the original reviewer's receipt: %+v %v", receipt, err)
		}
		own, err := f.svc.GetWorkflowOperation(f.reviewerCtx, request.IssueID, request.RequestID)
		if err != nil || own.Code != original.Code || !reflect.DeepEqual(own.StageConfirmation, original.StageConfirmation) {
			t.Fatalf("wrong-actor refusal changed original-key recovery: %+v %v", own, err)
		}
	})
	t.Run("disabled_account", func(t *testing.T) {
		f := newStageReviewFixture(t)
		_, request, _ := f.approved(t)
		f.fx.Exec(t, `UPDATE "user" SET disabled_at=clock_timestamp() WHERE id=$1`, f.reviewer)
		receipt, err := f.svc.GetWorkflowOperation(f.reviewerCtx, request.IssueID, request.RequestID)
		var refused *CollaborationError
		if !errors.As(err, &refused) || refused.Status != http.StatusForbidden || refused.Code != "forbidden" || !reflect.DeepEqual(receipt, CollaborationWorkflowOperationReceipt{}) {
			t.Fatalf("disabled account recovered a retained receipt: %+v %v", receipt, err)
		}
	})
}
