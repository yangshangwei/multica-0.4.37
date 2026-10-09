package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/iteration"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/pkg/featureflag"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func iterationSettingsRequest(method, path string, body any) *http.Request {
	return withURLParam(newRequest(method, "/api/workspaces/"+testWorkspaceID+"/"+path, body), "id", testWorkspaceID)
}
func iterationSettingsHandler(t *testing.T) *Handler {
	t.Helper()
	h := *testHandler
	dbfx.Cleanup(t, `DELETE FROM iteration_operation WHERE workspace_id=$1`, testWorkspaceID)
	dbfx.Cleanup(t, `DELETE FROM workspace_iteration_settings WHERE workspace_id=$1`, testWorkspaceID)
	return &h
}
func enableIterationBody(requestID string) map[string]any {
	return map[string]any{"request_id": requestID, "expected_revision": 1, "confirmed_timezone": "Asia/Shanghai"}
}
func iterationLegacyDeploymentFlag(t *testing.T, absent bool) *featureflag.Service {
	t.Helper()
	t.Setenv("FF_ITERATIONS_I1", "false")
	if absent {
		if err := os.Unsetenv("FF_ITERATIONS_I1"); err != nil {
			t.Fatal(err)
		}
	}
	return featureflag.NewService(featureflag.NewEnvProvider("FF_"))
}

func TestIterationSettingsDefaultAndEnableWithoutDeploymentFlag(t *testing.T) {
	for _, flag := range []string{"absent", "false"} {
		t.Run(flag, func(t *testing.T) {
			h := iterationSettingsHandler(t)
			h.FeatureFlags = iterationLegacyDeploymentFlag(t, flag == "absent")
			var settings iteration.Settings
			var capabilities map[string]any
			testutil.Call(t, h.GetIterationSettings, iterationSettingsRequest("GET", "iteration-settings", nil)).Want(200).JSON(&settings)
			if settings.Enabled || settings.Revision != 1 || settings.EffectiveTimezone != "Asia/Shanghai" || settings.PlanningTimezone != nil || settings.TimezoneConfigured || settings.WorkspaceID != testWorkspaceID {
				t.Fatalf("incorrect default settings: %+v", settings)
			}
			testutil.Call(t, h.GetIterationCapabilities, iterationSettingsRequest("GET", "iteration-capabilities", nil)).Want(200).JSON(&capabilities)
			if capabilities["supported"] != true || capabilities["enabled"] != false || capabilities["atomic_handoff"] != true || capabilities["manual"] != true || capabilities["workspace_id"] != testWorkspaceID || capabilities["schema_version"] != float64(1) {
				t.Fatalf("incorrect capability contract: %v", capabilities)
			}
			if n := dbfx.Count(t, `SELECT count(*) FROM workspace_iteration_settings WHERE workspace_id=$1`, testWorkspaceID); n != 0 {
				t.Fatal("read created settings")
			}
			counts := make(map[string]int)
			for _, table := range []string{"iteration", "issue", "iteration_notification"} {
				counts[table] = dbfx.Count(t, "SELECT count(*) FROM "+table+" WHERE workspace_id=$1", testWorkspaceID)
			}
			const taskCountSQL = "SELECT count(*) FROM agent_task_queue t JOIN agent a ON a.id=t.agent_id WHERE a.workspace_id=$1"
			tasksBefore := dbfx.Count(t, taskCountSQL, testWorkspaceID)
			testutil.Call(t, h.EnableIterationSettings, iterationSettingsRequest("POST", "iteration-settings/enable", enableIterationBody(uuid.NewString()))).Want(200)
			testutil.Call(t, h.GetIterationSettings, iterationSettingsRequest("GET", "iteration-settings", nil)).Want(200).JSON(&settings)
			if !settings.Enabled || settings.Revision != 2 {
				t.Fatalf("workspace did not enable: %+v", settings)
			}
			for table, before := range counts {
				if n := dbfx.Count(t, "SELECT count(*) FROM "+table+" WHERE workspace_id=$1", testWorkspaceID); n != before {
					t.Fatalf("enabling changed %s rows: before=%d after=%d", table, before, n)
				}
			}
			if n := dbfx.Count(t, taskCountSQL, testWorkspaceID); n != tasksBefore {
				t.Fatalf("enabling changed execution rows: before=%d after=%d", tasksBefore, n)
			}
		})
	}
}

func TestIterationCapabilitiesUsePersistedWorkspaceState(t *testing.T) {
	for _, flag := range []string{"absent", "false"} {
		for _, enabled := range []bool{false, true} {
			name := flag + "/disabled"
			if enabled {
				name = flag + "/enabled"
			}
			t.Run(name, func(t *testing.T) {
				h := iterationSettingsHandler(t)
				h.FeatureFlags = iterationLegacyDeploymentFlag(t, flag == "absent")
				dbfx.InsertNoID(t, "workspace_iteration_settings", testutil.Cols{"workspace_id": testWorkspaceID, "enabled": enabled, "revision": 7}, "workspace_id=$1", testWorkspaceID)
				var capabilities map[string]any
				testutil.Call(t, h.GetIterationCapabilities, iterationSettingsRequest("GET", "iteration-capabilities", nil)).Want(200).JSON(&capabilities)
				if capabilities["supported"] != true || capabilities["manual"] != true || capabilities["atomic_handoff"] != true || capabilities["enabled"] != enabled {
					t.Fatalf("capabilities ignored workspace state: %v", capabilities)
				}
				var triage TriageSettings
				testutil.Call(t, h.GetTriageSettings, newRequest("GET", "/api/triage/settings", nil)).Want(200).JSON(&triage)
				if triage.IterationAssignment != enabled {
					t.Fatalf("triage assignment ignored workspace state: %+v", triage)
				}
				var settings iteration.Settings
				testutil.Call(t, h.GetIterationSettings, iterationSettingsRequest("GET", "iteration-settings", nil)).Want(200).JSON(&settings)
				if settings.Enabled != enabled || settings.Revision != 7 {
					t.Fatalf("discovery changed persisted settings: %+v", settings)
				}
			})
		}
	}
}
func TestEnableIterationSettingsDurableReplay(t *testing.T) {
	h := iterationSettingsHandler(t)
	h.Bus = events.New()
	var published []events.Event
	h.Bus.Subscribe(protocol.EventIterationUpdated, func(event events.Event) {
		published = append(published, event)
		if n := dbfx.Count(t, `SELECT count(*) FROM workspace_iteration_settings WHERE workspace_id=$1 AND enabled AND revision=2`, testWorkspaceID); n != 1 {
			t.Fatal("settings update published before commit")
		}
	})
	id := uuid.NewString()
	body := enableIterationBody(id)
	stale := enableIterationBody(uuid.NewString())
	stale["confirmed_timezone"] = "UTC"
	testutil.Call(t, h.EnableIterationSettings, iterationSettingsRequest("POST", "iteration-settings/enable", stale)).Want(409)
	var first, replayed iteration.WriteResult
	testutil.Call(t, h.EnableIterationSettings, iterationSettingsRequest("POST", "iteration-settings/enable", body)).Want(200).JSON(&first)
	testutil.Call(t, h.EnableIterationSettings, iterationSettingsRequest("POST", "iteration-settings/enable", body)).Want(200).JSON(&replayed)
	if first.Replayed || !replayed.Replayed || first.OperationID == "" || first.OperationID != replayed.OperationID || first.Result.SettingsRevision != 2 {
		t.Fatalf("unstable operation: %+v %+v", first, replayed)
	}
	if len(published) != 1 || published[0].WorkspaceID != testWorkspaceID || published[0].ActorType != "member" || published[0].ActorID != testUserID {
		t.Fatalf("enable must publish once with the committed actor/workspace: %+v", published)
	}
	payload, ok := published[0].Payload.(map[string]any)
	if !ok || payload["operation_id"] != first.OperationID || payload["request_id"] != id {
		t.Fatalf("enable event lost operation identity: %+v", published[0])
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
	dbfx.Exec(t, `UPDATE workspace SET planning_timezone='UTC' WHERE id=$1`, testWorkspaceID)
	t.Cleanup(func() { dbfx.Exec(t, `UPDATE workspace SET planning_timezone=NULL WHERE id=$1`, testWorkspaceID) })
	testutil.Call(t, h.EnableIterationSettings, iterationSettingsRequest("POST", "iteration-settings/enable", enableIterationBody(uuid.NewString()))).Want(409)
	body := enableIterationBody(uuid.NewString())
	body["confirmed_timezone"] = "UTC"
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
			h.Bus = events.New()
			published := 0
			h.Bus.Subscribe(protocol.EventIterationUpdated, func(events.Event) { published++ })
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
			wantPublished := 0
			if mode == "retry" {
				wantPublished = 1
			}
			if published != wantPublished {
				t.Fatalf("published %d events for %s; want %d", published, mode, wantPublished)
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

func TestEnableIterationSettingsReplayDoesNotReenableDisabledWorkspace(t *testing.T) {
	h := iterationSettingsHandler(t)
	body := enableIterationBody(uuid.NewString())
	var initial, replayed iteration.WriteResult
	testutil.Call(t, h.EnableIterationSettings, iterationSettingsRequest("POST", "iteration-settings/enable", body)).Want(200).JSON(&initial)
	dbfx.Exec(t, "UPDATE workspace_iteration_settings SET enabled=false,revision=3 WHERE workspace_id=$1", testWorkspaceID)
	testutil.Call(t, h.EnableIterationSettings, iterationSettingsRequest("POST", "iteration-settings/enable", body)).Want(200).JSON(&replayed)
	if !replayed.Replayed || replayed.OperationID != initial.OperationID {
		t.Fatal("disabled workspace lost committed result")
	}
	var settings iteration.Settings
	testutil.Call(t, h.GetIterationSettings, iterationSettingsRequest("GET", "iteration-settings", nil)).Want(200).JSON(&settings)
	if settings.Enabled || settings.Revision != 3 {
		t.Fatalf("old enable replay changed later workspace settings: %+v", settings)
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
