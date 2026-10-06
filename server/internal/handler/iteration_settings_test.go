package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/multica-ai/multica/server/internal/iteration"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/pkg/featureflag"
)

func iterationSettingsRequest(method, path string, body any) *http.Request {
	return withURLParam(newRequest(method, "/api/workspaces/"+testWorkspaceID+"/"+path, body), "id", testWorkspaceID)
}
func iterationSettingsHandler(t *testing.T) *Handler {
	t.Helper()
	h := *testHandler
	p := featureflag.NewStaticProvider()
	p.Set("iterations_i1", featureflag.Rule{Default: true})
	h.FeatureFlags = featureflag.NewService(p)
	dbfx.Cleanup(t, `DELETE FROM iteration_operation WHERE workspace_id=$1`, testWorkspaceID)
	dbfx.Cleanup(t, `DELETE FROM workspace_iteration_settings WHERE workspace_id=$1`, testWorkspaceID)
	return &h
}
func enableIterationBody(requestID string) map[string]any {
	return map[string]any{"request_id": requestID, "expected_revision": 1, "confirmed_timezone": "UTC"}
}
func TestIterationSettingsDefaultAndReleaseGate(t *testing.T) {
	var settings iteration.Settings
	var capabilities map[string]any
	testutil.Call(t, testHandler.GetIterationSettings, iterationSettingsRequest("GET", "iteration-settings", nil)).Want(200).JSON(&settings)
	if settings.Enabled || settings.Revision != 1 || settings.EffectiveTimezone != "UTC" || settings.WorkspaceID != testWorkspaceID {
		t.Fatalf("incorrect default settings: %+v", settings)
	}
	if n := dbfx.Count(t, `SELECT count(*) FROM workspace_iteration_settings WHERE workspace_id=$1`, testWorkspaceID); n != 0 {
		t.Fatal("read created settings")
	}
	testutil.Call(t, testHandler.GetIterationCapabilities, iterationSettingsRequest("GET", "iteration-capabilities", nil)).Want(200).JSON(&capabilities)
	if capabilities["supported"] != false || capabilities["enabled"] != false || capabilities["atomic_handoff"] != false || capabilities["manual"] != true {
		t.Fatalf("premature capability: %v", capabilities)
	}
	testutil.Call(t, testHandler.EnableIterationSettings, iterationSettingsRequest("POST", "iteration-settings/enable", enableIterationBody(uuid.NewString()))).Want(422)
	if n := dbfx.Count(t, `SELECT count(*) FROM iteration_operation WHERE workspace_id=$1`, testWorkspaceID); n != 0 {
		t.Fatal("closed release gate persisted operation")
	}
}
func TestEnableIterationSettingsDurableReplay(t *testing.T) {
	h := iterationSettingsHandler(t)
	id := uuid.NewString()
	body := enableIterationBody(id)
	var first, replayed iteration.WriteResult
	testutil.Call(t, h.EnableIterationSettings, iterationSettingsRequest("POST", "iteration-settings/enable", body)).Want(200).JSON(&first)
	testutil.Call(t, h.EnableIterationSettings, iterationSettingsRequest("POST", "iteration-settings/enable", body)).Want(200).JSON(&replayed)
	if first.Replayed || !replayed.Replayed || first.OperationID == "" || first.OperationID != replayed.OperationID || first.Result.SettingsRevision != 2 {
		t.Fatalf("unstable operation: %+v %+v", first, replayed)
	}
	if n := dbfx.Count(t, `SELECT count(*) FROM workspace_iteration_settings WHERE workspace_id=$1 AND enabled AND revision=2`, testWorkspaceID); n != 1 {
		t.Fatal("settings did not change exactly once")
	}
	if n := dbfx.Count(t, `SELECT count(*) FROM iteration_operation WHERE workspace_id=$1`, testWorkspaceID); n != 1 {
		t.Fatal("duplicate operation")
	}
	var recovered iteration.WriteResult
	testutil.Call(t, h.GetIterationOperation, iterationOperationRequest(id)).Want(200).JSON(&recovered)
	if recovered.OperationID != first.OperationID {
		t.Fatal("GET failed recovery")
	}
	body["expected_revision"] = 2
	testutil.Call(t, h.EnableIterationSettings, iterationSettingsRequest("POST", "iteration-settings/enable", body)).Want(409)
	body["request_id"] = uuid.NewString()
	testutil.Call(t, h.EnableIterationSettings, iterationSettingsRequest("POST", "iteration-settings/enable", body)).Want(200)
	if n := dbfx.Count(t, `SELECT count(*) FROM workspace_iteration_settings WHERE workspace_id=$1 AND revision=2`, testWorkspaceID); n != 1 {
		t.Fatal("already-enabled operation advanced revision")
	}
}
func TestEnableIterationSettingsValidatesCurrentRoleAndTimezone(t *testing.T) {
	h := iterationSettingsHandler(t)
	user := dbfx.User(t, "Settings member", uuid.NewString()+"@example.invalid")
	dbfx.Member(t, testWorkspaceID, user, "member")
	req := iterationSettingsRequest("POST", "iteration-settings/enable", enableIterationBody(uuid.NewString()))
	req.Header.Set("X-User-ID", user)
	testutil.Call(t, h.EnableIterationSettings, req).Want(403)
	req = iterationSettingsRequest("POST", "iteration-settings/enable", enableIterationBody(uuid.NewString()))
	req.Header.Set("X-Actor-Source", "task_token")
	testutil.Call(t, h.EnableIterationSettings, req).Want(403)
	for _, zone := range []string{"Local", "+08:00", "No/Such_Zone"} {
		body := enableIterationBody(uuid.NewString())
		body["confirmed_timezone"] = zone
		testutil.Call(t, h.EnableIterationSettings, iterationSettingsRequest("POST", "iteration-settings/enable", body)).Want(422)
	}
	dbfx.Exec(t, `UPDATE workspace SET planning_timezone='Asia/Shanghai' WHERE id=$1`, testWorkspaceID)
	t.Cleanup(func() { dbfx.Exec(t, `UPDATE workspace SET planning_timezone=NULL WHERE id=$1`, testWorkspaceID) })
	testutil.Call(t, h.EnableIterationSettings, iterationSettingsRequest("POST", "iteration-settings/enable", enableIterationBody(uuid.NewString()))).Want(409)
	body := enableIterationBody(uuid.NewString())
	body["confirmed_timezone"] = "Asia/Shanghai"
	testutil.Call(t, h.EnableIterationSettings, iterationSettingsRequest("POST", "iteration-settings/enable", body)).Want(200)
}

type iterationEnableFaultState struct {
	mu                          sync.Mutex
	starts, operations, commits int
	mode                        string
	identities                  []any
	onBegin                     func()
	afterRollback               func()
	missed                      bool
}
type iterationEnableFaultStarter struct {
	inner txStarter
	state *iterationEnableFaultState
}

func (s iterationEnableFaultStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	s.state.mu.Lock()
	s.state.starts++
	s.state.mu.Unlock()
	if s.state.starts == 1 && s.state.onBegin != nil {
		s.state.onBegin()
	}

	tx, e := s.inner.Begin(ctx)
	if e != nil {
		return nil, e
	}
	return &iterationEnableFaultTx{Tx: tx, state: s.state}, nil
}

type iterationEnableFaultTx struct {
	pgx.Tx
	state *iterationEnableFaultState
}

func (tx *iterationEnableFaultTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if strings.Contains(sql, "GetIterationOperation") && tx.state.mode == "collision" && !tx.state.missed {
		tx.state.missed = true
		return iterationEnableFaultRow{pgx.ErrNoRows}
	}

	if strings.Contains(sql, "InsertIterationOperation") {
		tx.state.mu.Lock()
		tx.state.operations++
		n := tx.state.operations
		tx.state.identities = append(tx.state.identities, args[0])
		tx.state.mu.Unlock()
		switch tx.state.mode {
		case "other_unique":
			return iterationEnableFaultRow{&pgconn.PgError{Code: "23505", ConstraintName: "iteration_operation_id"}}
		case "rollback":
			return iterationEnableFaultRow{errors.New("injected operation persistence failure")}
		case "retry":
			if n == 1 {
				return iterationEnableFaultRow{&pgconn.PgError{Code: "40001"}}
			}
		case "exhaust":
			return iterationEnableFaultRow{&pgconn.PgError{Code: "55P03"}}
		}
	}
	return tx.Tx.QueryRow(ctx, sql, args...)
}

type iterationEnableFaultRow struct{ err error }

func (r iterationEnableFaultRow) Scan(...any) error { return r.err }
func (tx *iterationEnableFaultTx) Commit(ctx context.Context) error {
	err := tx.Tx.Commit(ctx)
	if err == nil {
		tx.state.commits++
		if tx.state.mode == "unknown" {
			return io.ErrUnexpectedEOF
		}
	}
	return err
}
func TestEnableIterationSettingsAtomicFailureRetryAndUnknownCommit(t *testing.T) {
	for _, mode := range []string{"rollback", "retry", "exhaust", "unknown"} {
		t.Run(mode, func(t *testing.T) {
			h := iterationSettingsHandler(t)
			state := &iterationEnableFaultState{mode: mode}
			h.TxStarter = iterationEnableFaultStarter{h.TxStarter, state}
			body := enableIterationBody(uuid.NewString())
			want := 503
			if mode == "retry" {
				want = 200
			}
			testutil.Call(t, h.EnableIterationSettings, iterationSettingsRequest("POST", "iteration-settings/enable", body)).Want(want)
			switch mode {
			case "rollback", "exhaust":
				if n := dbfx.Count(t, `SELECT count(*) FROM workspace_iteration_settings WHERE workspace_id=$1`, testWorkspaceID); n != 0 {
					t.Fatal("failed operation left settings")
				}
				if mode == "exhaust" && state.starts != 3 {
					t.Fatalf("retry budget=%d", state.starts)
				}
			case "retry":
				if state.starts != 2 || state.identities[0] != state.identities[1] {
					t.Fatalf("retry identity changed: %+v", state)
				}
			case "unknown":
				if state.starts != 1 || state.commits != 1 {
					t.Fatal("unknown commit auto-retried")
				}
				var got iteration.WriteResult
				testutil.Call(t, testHandler.GetIterationOperation, iterationOperationRequest(body["request_id"].(string))).Want(200).JSON(&got)
				if !got.Replayed {
					t.Fatal("unknown commit not recoverable")
				}
				state.mode = ""
				testutil.Call(t, h.EnableIterationSettings, iterationSettingsRequest("POST", "iteration-settings/enable", body)).Want(200)
			}
		})
	}
}
func TestEnableIterationSettingsRejectsMalformedEnvelope(t *testing.T) {
	h := iterationSettingsHandler(t)
	for _, raw := range []string{`{}`, `{"request_id":"bad","expected_revision":1,"confirmed_timezone":"UTC"}`, `{"request_id":"` + uuid.NewString() + `","expected_revision":9007199254740992,"confirmed_timezone":"UTC"}`, `{"request_id":"` + uuid.NewString() + `","expected_revision":1,"confirmed_timezone":"UTC","extra":true}`} {
		var body map[string]any
		_ = json.Unmarshal([]byte(raw), &body)
		testutil.Call(t, h.EnableIterationSettings, iterationSettingsRequest("POST", "iteration-settings/enable", body)).Want(400)
	}
}

func (tx *iterationEnableFaultTx) Rollback(ctx context.Context) error {
	err := tx.Tx.Rollback(ctx)
	if err == nil && tx.state.afterRollback != nil {
		callback := tx.state.afterRollback
		tx.state.afterRollback = nil
		callback()
	}
	return err
}

func TestEnableIterationSettingsCannotPromoteLegacyAgentWhileWaiting(t *testing.T) {
	h := iterationSettingsHandler(t)
	agent := dbfx.Agent(t, "Legacy settings actor", testRuntimeID)
	task := dbfx.Task(t, agent, testutil.Cols{"status": "running", "runtime_id": testRuntimeID})
	state := &iterationEnableFaultState{onBegin: func() { dbfx.Exec(t, `DELETE FROM agent_task_queue WHERE id=$1`, task) }}
	h.TxStarter = iterationEnableFaultStarter{h.TxStarter, state}
	req := iterationSettingsRequest("POST", "iteration-settings/enable", enableIterationBody(uuid.NewString()))
	req.Header.Set("X-Agent-ID", agent)
	req.Header.Set("X-Task-ID", task)
	testutil.Call(t, h.EnableIterationSettings, req).Want(403)
	if n := dbfx.Count(t, `SELECT count(*) FROM iteration_operation WHERE workspace_id=$1`, testWorkspaceID); n != 0 {
		t.Fatal("legacy agent acquired human enable authority")
	}
}

func TestEnableIterationSettingsConcurrentSameRequest(t *testing.T) {
	h := iterationSettingsHandler(t)
	id := uuid.NewString()
	results := make(chan *testutil.Response, 2)
	start := make(chan struct{})
	for i := 0; i < 2; i++ {
		go func() {
			<-start
			results <- testutil.Call(t, h.EnableIterationSettings, iterationSettingsRequest("POST", "iteration-settings/enable", enableIterationBody(id)))
		}()
	}
	close(start)
	var a, b iteration.WriteResult
	for i := 0; i < 2; i++ {
		select {
		case r := <-results:
			if i == 0 {
				r.Want(200).JSON(&a)
			} else {
				r.Want(200).JSON(&b)
			}
		case <-time.After(10 * time.Second):
			t.Fatal("same-request enable blocked")
		}
	}
	if a.OperationID != b.OperationID || a.Replayed == b.Replayed {
		t.Fatalf("concurrent operation not deduplicated: %+v %+v", a, b)
	}
	if n := dbfx.Count(t, `SELECT count(*) FROM iteration_operation WHERE workspace_id=$1`, testWorkspaceID); n != 1 {
		t.Fatal("duplicate concurrent operation")
	}
}

func TestEnableIterationSettingsRecoversRealRequestUniqueConflict(t *testing.T) {
	h := iterationSettingsHandler(t)
	testutil.Call(t, h.EnableIterationSettings, iterationSettingsRequest("POST", "iteration-settings/enable", enableIterationBody(uuid.NewString()))).Want(200)
	body := enableIterationBody(uuid.NewString())
	body["expected_revision"] = 2
	var initial, replayed iteration.WriteResult
	testutil.Call(t, h.EnableIterationSettings, iterationSettingsRequest("POST", "iteration-settings/enable", body)).Want(200).JSON(&initial)
	// A stale/missing first lookup forces the real PostgreSQL request unique
	// constraint at insert. Recovery must roll back and reauthorize a new tx.
	state := &iterationEnableFaultState{mode: "collision"}
	h.TxStarter = iterationEnableFaultStarter{h.TxStarter, state}
	testutil.Call(t, h.EnableIterationSettings, iterationSettingsRequest("POST", "iteration-settings/enable", body)).Want(200).JSON(&replayed)
	if state.starts != 2 || state.operations != 1 || !replayed.Replayed || replayed.OperationID != initial.OperationID {
		t.Fatalf("unique recovery state=%+v result=%+v", state, replayed)
	}
}

func TestEnableIterationSettingsReauthorizesRetryAndDoesNotRetryOtherUnique(t *testing.T) {
	for _, mode := range []string{"retry", "other_unique"} {
		t.Run(mode, func(t *testing.T) {
			h := iterationSettingsHandler(t)
			state := &iterationEnableFaultState{mode: mode}
			want := 503
			if mode == "retry" {
				want = 403
				state.afterRollback = func() {
					dbfx.Exec(t, `UPDATE member SET role='member' WHERE workspace_id=$1 AND user_id=$2`, testWorkspaceID, testUserID)
				}
				t.Cleanup(func() {
					dbfx.Exec(t, `UPDATE member SET role='owner' WHERE workspace_id=$1 AND user_id=$2`, testWorkspaceID, testUserID)
				})
			}
			h.TxStarter = iterationEnableFaultStarter{h.TxStarter, state}
			testutil.Call(t, h.EnableIterationSettings, iterationSettingsRequest("POST", "iteration-settings/enable", enableIterationBody(uuid.NewString()))).Want(want)
			if state.operations != 1 {
				t.Fatal("forbidden or unrelated-unique write retried")
			}
			if n := dbfx.Count(t, `SELECT count(*) FROM workspace_iteration_settings WHERE workspace_id=$1`, testWorkspaceID); n != 0 {
				t.Fatal("retry retained partial settings")
			}
		})
	}
}

func TestEnableIterationSettingsRetainsTimezoneThroughCommit(t *testing.T) {
	h := iterationSettingsHandler(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	reached, release := make(chan struct{}, 1), make(chan struct{})
	var releaseOnce sync.Once
	defer releaseOnce.Do(func() { close(release) })
	h.TxStarter = projectAssociationCommitStarter{base: h.TxStarter, reached: reached, release: release, once: &sync.Once{}}
	result := make(chan *testutil.Response, 1)
	req := iterationSettingsRequest("POST", "iteration-settings/enable", enableIterationBody(uuid.NewString()))
	go func() { result <- testutil.Call(t, h.EnableIterationSettings, req) }()
	waitIterationBarrier(t, reached)
	tx, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `SELECT id FROM workspace WHERE id=$1 FOR NO KEY UPDATE NOWAIT`, testWorkspaceID)
	var conflict *pgconn.PgError
	if !errors.As(err, &conflict) || conflict.Code != "55P03" {
		t.Fatalf("timezone writable during enable commit: %v", err)
	}
	_ = tx.Rollback(ctx)
	releaseOnce.Do(func() { close(release) })
	select {
	case response := <-result:
		response.Want(200)
	case <-ctx.Done():
		t.Fatal("enable did not complete")
	}
}

func TestEnableIterationSettingsReplaySurvivesClosedFlag(t *testing.T) {
	h := iterationSettingsHandler(t)
	body := enableIterationBody(uuid.NewString())
	var initial, replayed iteration.WriteResult
	testutil.Call(t, h.EnableIterationSettings, iterationSettingsRequest("POST", "iteration-settings/enable", body)).Want(200).JSON(&initial)
	h.FeatureFlags = nil
	testutil.Call(t, h.EnableIterationSettings, iterationSettingsRequest("POST", "iteration-settings/enable", body)).Want(200).JSON(&replayed)
	if !replayed.Replayed || replayed.OperationID != initial.OperationID {
		t.Fatal("closed rollout lost committed result")
	}
	var settings iteration.Settings
	testutil.Call(t, h.GetIterationSettings, iterationSettingsRequest("GET", "iteration-settings", nil)).Want(200).JSON(&settings)
	if !settings.Enabled {
		t.Fatal("rollout flag erased persisted settings")
	}
}

func TestEnableIterationSettingsAdminAndCloudCredentialBoundary(t *testing.T) {
	h := iterationSettingsHandler(t)
	admin := dbfx.User(t, "Iteration admin", uuid.NewString()+"@example.invalid")
	dbfx.Member(t, testWorkspaceID, admin, "admin")
	body := enableIterationBody(uuid.NewString())
	request := iterationSettingsRequest("POST", "iteration-settings/enable", body)
	request.Header.Set("X-User-ID", admin)
	request.Header.Set("X-Actor-Source", "cloud_pat")
	testutil.Call(t, h.EnableIterationSettings, request).Want(403)
	request = iterationSettingsRequest("POST", "iteration-settings/enable", body)
	request.Header.Set("X-User-ID", admin)
	var result iteration.WriteResult
	testutil.Call(t, h.EnableIterationSettings, request).Want(200).JSON(&result)
	if n := dbfx.Count(t, `SELECT count(*) FROM iteration_operation WHERE id=$1 AND actor_user_id=$2`, result.OperationID, admin); n != 1 {
		t.Fatal("admin operation attributed to another user")
	}
	request = iterationSettingsRequest("POST", "iteration-settings/enable", nil)
	request.Body = io.NopCloser(strings.NewReader(`{} {}`))
	testutil.Call(t, h.EnableIterationSettings, request).Want(400)
}
