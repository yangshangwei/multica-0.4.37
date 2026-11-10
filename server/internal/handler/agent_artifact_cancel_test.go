package handler

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/artifact"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type cancellationArtifactCounter struct {
	artifactExecutor
	calls atomic.Int32
}

func (p *cancellationArtifactCounter) Pack(ctx context.Context, id string, s artifact.Spec) artifact.Outcome {
	p.calls.Add(1)
	return p.artifactExecutor.Pack(ctx, id, s)
}

// CancelTaskInTx is the production cancellation core used by CancelTaskWithResult.
// Its return before the caller's commit is the real barrier: no handcrafted task
// lock substitutes for the chat-session lock and cancelled-state UPDATE.
func TestControlledArtifactTaskCancellationBeforeClaim(t *testing.T) {
	f := newArtifactFixture(t)
	chat := dbfx.ChatSession(t, f.req.AgentID)
	dbfx.Exec(t, `UPDATE agent_task_queue SET chat_session_id=$1 WHERE id=$2`, chat, f.req.TaskID)
	out := f.create(t)
	f.approve(t, out)
	counter := &cancellationArtifactCounter{artifactExecutor: f.h.ArtifactPackager}
	f.h.ArtifactPackager = counter
	tx, err := testPool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	q := db.New(tx)
	svc := service.NewTaskService(f.h.Queries, testPool, nil, nil)
	effects, err := svc.CancelTaskInTx(t.Context(), q, parseUUID(f.req.TaskID), service.CancelTaskOptions{UserInitiated: true, ClientSupportsDraftRestore: true})
	if err != nil {
		t.Fatal(err)
	}
	if !effects.Changed || effects.Result.Task.Status != "cancelled" || !effects.Result.Task.CompletedAt.Valid {
		t.Fatal("actual cancellation core did not update the task")
	}
	if taskStatus(t, f.req.TaskID) != "running" {
		t.Fatal("uncommitted cancellation leaked before barrier")
	}
	req := f.route(out.Approval.ID, "artifact-package/execution", map[string]string{"expected_spec_digest": out.SpecDigest})
	blocked := testutil.Call(t, f.h.ExecuteArtifactPackage, req).Want(409)
	if !strings.Contains(blocked.Text(), "artifact_authority_busy") {
		t.Fatal("claim did not intersect actual cancellation locks")
	}
	if counter.calls.Load() != 0 || dbfx.Count(t, `SELECT count(*) FROM agent_artifact_execution WHERE approval_id=$1 AND claimed_at IS NOT NULL`, out.Approval.ID) != 0 {
		t.Fatal("cancel-first contention invoked or claimed")
	}
	if err = tx.Commit(t.Context()); err != nil {
		t.Fatal(err)
	}
	if taskStatus(t, f.req.TaskID) != "cancelled" {
		t.Fatal("actual cancellation did not commit")
	}
	testutil.Call(t, f.h.ExecuteArtifactPackage, f.route(out.Approval.ID, "artifact-package/execution", map[string]string{"expected_spec_digest": out.SpecDigest})).Want(403)
	if counter.calls.Load() != 0 {
		t.Fatal("cancelled task invoked after retry")
	}
}

func TestControlledArtifactTaskCancellationAfterClaim(t *testing.T) {
	f := newArtifactFixture(t)
	chat := dbfx.ChatSession(t, f.req.AgentID)
	dbfx.Exec(t, `UPDATE agent_task_queue SET chat_session_id=$1 WHERE id=$2`, chat, f.req.TaskID)
	out := f.create(t)
	f.approve(t, out)
	gate := &gatedArtifact{artifactExecutor: f.h.ArtifactPackager, entered: make(chan struct{}), release: make(chan struct{})}
	f.h.ArtifactPackager = gate
	var release sync.Once
	t.Cleanup(func() { release.Do(func() { close(gate.release) }) })
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		w := httptest.NewRecorder()
		f.h.ExecuteArtifactPackage(w, f.route(out.Approval.ID, "artifact-package/execution", map[string]string{"expected_spec_digest": out.SpecDigest}))
		done <- w
	}()
	select {
	case <-gate.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("sole admitted adapter never entered")
	}
	if dbfx.Count(t, `SELECT count(*) FROM agent_artifact_execution WHERE approval_id=$1 AND claimed_at IS NOT NULL AND result_status='unknown'`, out.Approval.ID) != 1 {
		t.Fatal("claim was not durable before side effect")
	}
	tx, err := testPool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	svc := service.NewTaskService(f.h.Queries, testPool, nil, nil)
	effects, err := svc.CancelTaskInTx(t.Context(), db.New(tx), parseUUID(f.req.TaskID), service.CancelTaskOptions{UserInitiated: true, ClientSupportsDraftRestore: true})
	if err != nil {
		t.Fatal(err)
	}
	if !effects.Changed || effects.Result.Task.Status != "cancelled" {
		t.Fatal("actual cancellation did not update admitted task")
	}
	if err = tx.Commit(t.Context()); err != nil {
		t.Fatal(err)
	}
	if taskStatus(t, f.req.TaskID) != "cancelled" {
		t.Fatal("cancellation status not committed")
	}
	release.Do(func() { close(gate.release) })
	var w *httptest.ResponseRecorder
	select {
	case w = <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("original owner did not finalize after cancellation")
	}
	if w.Code != 200 {
		t.Fatalf("original execution status=%d body=%s", w.Code, w.Body.String())
	}
	var result artifactResponse
	if err = json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Status != "succeeded" || result.Approval.Status != "executed" || result.ReceiptDigest == "" {
		t.Fatal("admitted operation did not retain atomic success")
	}
	replay := f.execute(t, out, 200)
	if replay.ExecutionID != result.ExecutionID || replay.ReceiptDigest != result.ReceiptDigest || gate.calls.Load() != 1 {
		t.Fatal("post-cancellation replay reran or changed receipt")
	}
	if taskStatus(t, f.req.TaskID) != "cancelled" {
		t.Fatal("receipt persistence revived cancelled task")
	}
}
