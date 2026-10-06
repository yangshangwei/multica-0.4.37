package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/iteration"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func storedIterationOperationFixture(t *testing.T, actor string) iteration.WriteResult {
	t.Helper()
	result := iteration.WriteResult{WorkspaceID: testWorkspaceID, RequestID: uuid.NewString(), OperationID: uuid.NewString(), Operation: "delete", IterationIDs: []string{uuid.NewString()}, Result: iteration.WriteSummary{Deleted: true, SettingsRevision: 1}, CommittedAt: time.Now().UTC()}
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	dbfx.Insert(t, "iteration_operation", testutil.Cols{"id": result.OperationID, "workspace_id": testWorkspaceID, "actor_user_id": actor, "request_id": result.RequestID, "operation": "delete", "payload_hash": "stored-intent", "result": string(raw)})
	return result
}

func iterationOperationRequest(requestID string) *http.Request {
	r := withURLParam(newRequest("GET", "/api/workspaces/"+testWorkspaceID+"/iteration-operations/"+requestID, nil), "id", testWorkspaceID)
	chi.RouteContext(r.Context()).URLParams.Add("requestID", requestID)
	return r
}

func TestGetIterationOperationRetainsDeletedResultWithoutPayload(t *testing.T) {
	stored := storedIterationOperationFixture(t, testUserID)
	var got iteration.WriteResult
	response := testutil.Call(t, testHandler.GetIterationOperation, iterationOperationRequest(stored.RequestID)).Want(200)
	response.JSON(&got)
	if got.OperationID != stored.OperationID || got.RequestID != stored.RequestID || !got.Replayed || !got.Result.Deleted || len(got.IterationIDs) != 1 || got.IterationIDs[0] != stored.IterationIDs[0] {
		t.Fatalf("lost durable deleted result: %+v", got)
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("protected operation result must not be cached")
	}
}

func TestGetIterationOperationScopesActorAndRejectsMalformedResult(t *testing.T) {
	other := dbfx.User(t, "Other operation owner", uuid.NewString()+"@example.invalid")
	dbfx.Member(t, testWorkspaceID, other, "member")
	stored := storedIterationOperationFixture(t, other)
	testutil.Call(t, testHandler.GetIterationOperation, iterationOperationRequest(stored.RequestID)).Want(404)
	request := iterationOperationRequest(stored.RequestID)
	request.Header.Set("X-User-ID", other)
	testutil.Call(t, testHandler.GetIterationOperation, request).Want(200)
	dbfx.Exec(t, "UPDATE iteration_operation SET result='{}'::jsonb WHERE id=$1", stored.OperationID)
	testutil.Call(t, testHandler.GetIterationOperation, request).Want(503)
	testutil.Call(t, testHandler.GetIterationOperation, iterationOperationRequest(uuid.NewString())).Want(404)
	testutil.Call(t, testHandler.GetIterationOperation, iterationOperationRequest("invalid")).Want(400)
}

func TestGetIterationOperationRechecksCurrentMembershipAndScope(t *testing.T) {
	other := dbfx.User(t, "Revoked operation owner", uuid.NewString()+"@example.invalid")
	member := dbfx.Member(t, testWorkspaceID, other, "member")
	stored := storedIterationOperationFixture(t, other)
	request := iterationOperationRequest(stored.RequestID)
	request.Header.Set("X-User-ID", other)
	testutil.Call(t, testHandler.GetIterationOperation, request).Want(200)
	dbfx.Exec(t, "DELETE FROM member WHERE id=$1", member)
	testutil.Call(t, testHandler.GetIterationOperation, request).Want(403)
	request = iterationOperationRequest(stored.RequestID)
	request.Header.Set("X-Workspace-ID", uuid.NewString())
	testutil.Call(t, testHandler.GetIterationOperation, request).Want(403)
}

type operationReadBarrierStarter struct {
	inner     txStarter
	began     chan<- uint32
	reading   chan<- uint32
	release   <-chan struct{}
	readCalls *atomic.Int32
}

func (s operationReadBarrierStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.inner.Begin(ctx)
	if err != nil {
		return nil, err
	}
	// The read owner must explicitly select RC even when its starter's
	// inherited default would retain a snapshot taken before revocation.
	if _, err = tx.Exec(ctx, "SET TRANSACTION ISOLATION LEVEL REPEATABLE READ"); err != nil {
		_ = tx.Rollback(ctx)
		return nil, err
	}
	if s.began != nil {
		s.began <- tx.Conn().PgConn().PID()
	}
	return &operationReadBarrierTx{Tx: tx, owner: s}, nil
}

type operationReadBarrierTx struct {
	pgx.Tx
	owner operationReadBarrierStarter
}

func (tx *operationReadBarrierTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if strings.Contains(sql, "GetIterationOperation") {
		if tx.owner.readCalls != nil {
			tx.owner.readCalls.Add(1)
		}
		if tx.owner.reading != nil {
			tx.owner.reading <- tx.Conn().PgConn().PID()
			select {
			case <-tx.owner.release:
			case <-ctx.Done():
			}
		}
	}
	return tx.Tx.QueryRow(ctx, sql, args...)
}

func waitForOperationReadBlock(t *testing.T, ctx context.Context, blockedPID, ownerPID uint32) {
	t.Helper()
	for ctx.Err() == nil {
		var blocked bool
		if err := testPool.QueryRow(ctx, `SELECT $1::int=ANY(pg_blocking_pids($2::int))`, ownerPID, blockedPID).Scan(&blocked); err != nil {
			t.Fatal(err)
		}
		if blocked {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("connection %d never blocked on %d: %v", blockedPID, ownerPID, ctx.Err())
}

func TestGetIterationOperationConcurrentRevocation(t *testing.T) {
	t.Run("revoke_first_denies_without_loading_result", func(t *testing.T) {
		other := dbfx.User(t, "Revoking operation owner", uuid.NewString()+"@example.invalid")
		member := dbfx.Member(t, testWorkspaceID, other, "member")
		stored := storedIterationOperationFixture(t, other)
		request := iterationOperationRequest(stored.RequestID)
		request.Header.Set("X-User-ID", other)
		ctx, cancel := context.WithTimeout(request.Context(), 5*time.Second)
		defer cancel()
		request = request.WithContext(ctx)
		revoke, err := testPool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer revoke.Rollback(ctx)
		if err = db.New(revoke).LockSubscriberWrites(ctx, db.LockSubscriberWritesParams{WorkspaceID: parseUUID(testWorkspaceID), UserID: parseUUID(other)}); err != nil {
			t.Fatal(err)
		}
		if _, err = revoke.Exec(ctx, `DELETE FROM member WHERE id=$1`, member); err != nil {
			t.Fatal(err)
		}
		began := make(chan uint32, 1)
		var readCalls atomic.Int32
		h := *testHandler
		h.TxStarter = operationReadBarrierStarter{inner: h.TxStarter, began: began, readCalls: &readCalls}
		response := httptest.NewRecorder()
		done := make(chan struct{})
		go func() { h.GetIterationOperation(response, request); close(done) }()
		var readerPID uint32
		select {
		case readerPID = <-began:
		case <-ctx.Done():
			t.Fatal("operation read did not open its transaction")
		}
		waitForOperationReadBlock(t, ctx, readerPID, revoke.Conn().PgConn().PID())
		if err = revoke.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		select {
		case <-done:
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
		if response.Code != http.StatusForbidden || readCalls.Load() != 0 {
			t.Fatalf("revoked reader accessed result: status=%d reads=%d body=%s", response.Code, readCalls.Load(), response.Body.String())
		}
	})

	t.Run("read_first_holds_authorization_until_commit", func(t *testing.T) {
		other := dbfx.User(t, "Reading operation owner", uuid.NewString()+"@example.invalid")
		member := dbfx.Member(t, testWorkspaceID, other, "member")
		stored := storedIterationOperationFixture(t, other)
		request := iterationOperationRequest(stored.RequestID)
		request.Header.Set("X-User-ID", other)
		ctx, cancel := context.WithTimeout(request.Context(), 5*time.Second)
		defer cancel()
		request = request.WithContext(ctx)
		reading, release := make(chan uint32, 1), make(chan struct{})
		var releaseOnce sync.Once
		releaseRead := func() { releaseOnce.Do(func() { close(release) }) }
		defer releaseRead()
		h := *testHandler
		h.TxStarter = operationReadBarrierStarter{inner: h.TxStarter, reading: reading, release: release}
		response := httptest.NewRecorder()
		readDone := make(chan struct{})
		go func() { h.GetIterationOperation(response, request); close(readDone) }()
		var readerPID uint32
		select {
		case readerPID = <-reading:
		case <-ctx.Done():
			t.Fatal("operation did not reach authorized result read")
		}
		revoke, err := testPool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer revoke.Rollback(ctx)
		revokerPID := revoke.Conn().PgConn().PID()
		revokeDone := make(chan error, 1)
		go func() {
			err := db.New(revoke).LockSubscriberWrites(ctx, db.LockSubscriberWritesParams{WorkspaceID: parseUUID(testWorkspaceID), UserID: parseUUID(other)})
			if err == nil {
				_, err = revoke.Exec(ctx, `DELETE FROM member WHERE id=$1`, member)
			}
			if err == nil {
				err = revoke.Commit(ctx)
			}
			revokeDone <- err
		}()
		waitForOperationReadBlock(t, ctx, revokerPID, readerPID)
		releaseRead()
		select {
		case <-readDone:
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
		if response.Code != http.StatusOK {
			t.Fatalf("authorized read failed before revocation: %d %s", response.Code, response.Body.String())
		}
		var result iteration.WriteResult
		if err = json.Unmarshal(response.Body.Bytes(), &result); err != nil || result.OperationID != stored.OperationID {
			t.Fatalf("authorized result lost its identity: %+v %v", result, err)
		}
		select {
		case err = <-revokeDone:
			if err != nil {
				t.Fatal(err)
			}
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
		testutil.Call(t, testHandler.GetIterationOperation, request).Want(http.StatusForbidden)
	})
}
