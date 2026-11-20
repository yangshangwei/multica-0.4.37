package handler

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
)

const r33PolicyLockBudget = 250 * time.Millisecond
const r33PolicyShortRelease = 50 * time.Millisecond
const r33PolicyWitnessSQL = `SELECT $2::integer = ANY(pg_blocking_pids($1::integer))`

type r33PolicyQuery struct {
	Name, SHA256, Actor, Kind string
	PID                       uint32
	Args                      []string
	At                        time.Time
	Err                       error
}

type r33PolicyProbe struct {
	mu                    sync.Mutex
	pid                   uint32
	readFence             bool
	budget, creates       int
	projectRow            r33PolicyQuery
	events                []map[string]any
	hold                  string
	held, release         chan struct{}
	entered, finished     chan r33PolicyQuery
	holdOnce, releaseOnce sync.Once
}

func r33NewPolicyProbe(hold string) *r33PolicyProbe {
	return &r33PolicyProbe{hold: hold, held: make(chan struct{}), release: make(chan struct{}), entered: make(chan r33PolicyQuery, 1), finished: make(chan r33PolicyQuery, 1)}
}

func (p *r33PolicyProbe) unblock() { p.releaseOnce.Do(func() { close(p.release) }) }

func (p *r33PolicyProbe) record(event map[string]any) {
	p.mu.Lock()
	defer p.mu.Unlock()
	event["at"] = time.Now().UTC().Format(time.RFC3339Nano)
	p.events = append(p.events, event)
}

func (p *r33PolicyProbe) state() (uint32, bool, int, int, r33PolicyQuery) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.pid, p.readFence, p.budget, p.creates, p.projectRow
}

func (p *r33PolicyProbe) starter(base txStarter) txStarter {
	return progressTxStarter{base: base, wrap: func(tx pgx.Tx) pgx.Tx {
		// Pool-backed transactions release Conn on completion. Cache its real
		// backend identity now, never by dereferencing it after Commit/Rollback.
		p.mu.Lock()
		p.pid = tx.Conn().PgConn().PID()
		p.mu.Unlock()
		return &r33PolicyTx{Tx: tx, probe: p}
	}}
}

type r33PolicyTx struct {
	pgx.Tx
	probe *r33PolicyProbe
}

func (tx *r33PolicyTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	tag, err := tx.Tx.Exec(ctx, sql, args...)
	if sql == "SET LOCAL lock_timeout='250ms'" && err == nil {
		tx.probe.mu.Lock()
		tx.probe.budget++
		tx.probe.mu.Unlock()
		tx.probe.record(map[string]any{"phase": "original_lock_budget", "milliseconds": 250})
	}
	return tag, err
}

func r33PolicyProjectQuery(name string) bool {
	return name == "LockDeliveryPolicyProject" || name == "LockDeliveryPolicyProjectWait"
}

func (tx *r33PolicyTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	query := r33PolicyQuery{At: time.Now(), SHA256: fmt.Sprintf("%x", sha256.Sum256([]byte(sql)))}
	if fields := strings.Fields(strings.SplitN(sql, "\n", 2)[0]); len(fields) == 4 && fields[0] == "--" && fields[1] == "name:" {
		query.Name = fields[2]
	}
	query.PID, _, _, _, _ = tx.probe.state()
	if principal, ok := auth.AuthenticatedPrincipalFromContext(ctx); ok {
		query.Actor, query.Kind = principal.UserID, string(principal.Kind)
	}
	if r33PolicyProjectQuery(query.Name) || query.Name == "LockProjectForAssociationNowait" {
		for _, arg := range args {
			if id, ok := arg.(pgtype.UUID); ok && id.Valid {
				query.Args = append(query.Args, util.UUIDToString(id))
			}
		}
	}
	if r33PolicyProjectQuery(query.Name) {
		tx.probe.entered <- query
	}
	// Forward the actual SQL, arguments, generated destinations and Scan error.
	// The entry channel alone is not proof of a database lock attempt: the test
	// additionally requires a real failure or pg_blocking_pids before release.
	row := tx.Tx.QueryRow(ctx, sql, args...)
	return progressObservedRow{Row: row, after: func(err error) {
		query.Err = err
		tx.probe.mu.Lock()
		if query.Name == "LockProjectForAssociationNowait" && err == nil {
			tx.probe.readFence = true
		}
		if query.Name == "CreateDeliveryPolicy" && err == nil {
			tx.probe.creates++
		}
		if r33PolicyProjectQuery(query.Name) {
			tx.probe.projectRow = query
		}
		tx.probe.mu.Unlock()
		if r33PolicyProjectQuery(query.Name) {
			// Publish the real Scan result before collecting frames or logs so
			// the coordinator can consume a prompt NOWAIT failure immediately.
			tx.probe.finished <- query
		}
		if r33PolicyProjectQuery(query.Name) || query.Name == "LockProjectForAssociationNowait" || query.Name == "GetDeliveryPolicy" || query.Name == "CreateDeliveryPolicy" || err != nil && !errors.Is(err, pgx.ErrNoRows) {
			event := map[string]any{"phase": "actual_scan", "query": query.Name, "query_sha256": query.SHA256, "pid": query.PID, "actor": query.Actor, "credential_kind": query.Kind, "uuid_args": query.Args, "elapsed_ms": time.Since(query.At).Seconds() * 1000, "succeeded": err == nil, "no_rows": errors.Is(err, pgx.ErrNoRows)}
			var pgError *pgconn.PgError
			if errors.As(err, &pgError) {
				event["sqlstate"], event["error_type"] = pgError.Code, fmt.Sprintf("%T", err)
				pcs := make([]uintptr, 24)
				frames := runtime.CallersFrames(pcs[:runtime.Callers(2, pcs)])
				stack := []map[string]any{}
				for {
					frame, more := frames.Next()
					if strings.Contains(frame.Function, "github.com/multica-ai/multica/server/") {
						stack = append(stack, map[string]any{"function": frame.Function, "file": frame.File, "line": frame.Line})
					}
					if !more {
						break
					}
				}
				event["source_frames"] = stack
			}
			tx.probe.record(event)
		}
		if query.Name == tx.probe.hold && (err == nil || query.Name == "GetDeliveryPolicy" && errors.Is(err, pgx.ErrNoRows)) {
			tx.probe.holdOnce.Do(func() {
				tx.probe.record(map[string]any{"phase": "actual_transaction_held", "query": query.Name, "pid": query.PID, "tx_status": string(tx.Tx.Conn().PgConn().TxStatus())})
				close(tx.probe.held)
				select {
				case <-tx.probe.release:
				case <-ctx.Done():
				}
			})
		}
	}}
}

func r33PolicySnapshot(t *testing.T, f *deliveryPolicyFixture) map[string]json.RawMessage {
	t.Helper()
	var out map[string]json.RawMessage
	if err := json.Unmarshal([]byte(deliveryHintSnapshot(t, f)), &out); err != nil {
		t.Fatal(err)
	}
	var authority string
	dbfx.QueryRow(t, `SELECT jsonb_build_object(
 'workspace', (SELECT to_jsonb(w) FROM workspace w WHERE id=$3),
 'members', (SELECT jsonb_agg(to_jsonb(m) ORDER BY id) FROM member m WHERE workspace_id=$3),
 'users', (SELECT jsonb_agg(to_jsonb(u) ORDER BY u.id) FROM "user" u WHERE id IN (SELECT user_id FROM member WHERE workspace_id=$3)),
 'passwords', (SELECT jsonb_agg(to_jsonb(c) ORDER BY user_id) FROM user_password_credential c WHERE user_id IN (SELECT user_id FROM member WHERE workspace_id=$3)),
 'stage_task', (SELECT to_jsonb(s) FROM collaboration_stage_task s WHERE issue_id=$1),
 'stage_policy', (SELECT jsonb_agg(to_jsonb(s) ORDER BY revision) FROM project_stage_policy s WHERE project_id=$2),
 'authorizations', (SELECT jsonb_agg(to_jsonb(a) ORDER BY id) FROM delivery_completion_authorization a WHERE issue_id=$1 OR evidence_issue_id=$1),
 'terminals', (SELECT jsonb_agg(to_jsonb(s) ORDER BY authorization_id) FROM delivery_completion_terminal s JOIN delivery_completion_authorization a ON a.id=s.authorization_id WHERE a.issue_id=$1 OR a.evidence_issue_id=$1)
 )::text`, f.artifactFixture.req.ResourceID, f.project, f.ws).Scan(&authority)
	out["authority"] = json.RawMessage(authority)
	return out
}

func r33PolicyEffects(t *testing.T, before, after map[string]json.RawMessage, policyRows int) {
	t.Helper()
	var rows []json.RawMessage
	if err := json.Unmarshal(after["policy"], &rows); err != nil || len(rows) != policyRows {
		t.Fatalf("actual policy history count=%d, want%d: %v", len(rows), policyRows, err)
	}
	beforeRaw, _ := json.Marshal(before)
	afterRaw, _ := json.Marshal(after)
	beforePolicy, afterPolicy := before["policy"], after["policy"]
	delete(before, "policy")
	delete(after, "policy")
	equal := reflect.DeepEqual(before, after)
	before["policy"], after["policy"] = beforePolicy, afterPolicy
	t.Logf("R33_POLICY_EFFECTS before=%x after=%x protected_equal=%t policy_rows=%d", sha256.Sum256(beforeRaw), sha256.Sum256(afterRaw), equal, len(rows))
	if !equal {
		t.Fatal("policy request changed fixture-owned non-policy facts")
	}
}

func r33PolicyRequest(ctx context.Context, f *deliveryPolicyFixture, method string, body any) (*http.Request, context.CancelFunc) {
	req := testutil.WithURLParams(f.request(method, "/api/projects/"+f.project+"/delivery-policy", body), "id", f.project)
	deadline, _ := ctx.Deadline()
	requestCtx, cancel := context.WithDeadline(req.Context(), deadline)
	return req.WithContext(requestCtx), cancel
}

func r33PolicyAwait(t *testing.T, ctx context.Context, done <-chan *testutil.Response) *testutil.Response {
	t.Helper()
	select {
	case response := <-done:
		return response
	case <-ctx.Done():
		t.Fatal("actual policy handler exceeded the original five-second test bound")
		return nil
	}
}

func r33PolicyFinishedAttempt(t *testing.T, firstPID uint32, query r33PolicyQuery, probe *r33PolicyProbe) bool {
	t.Helper()
	select {
	case result := <-probe.finished:
		var pgError *pgconn.PgError
		if !errors.As(result.Err, &pgError) || pgError.Code != "55P03" {
			t.Fatal("project lock finished while the real holder remained uncommitted without native55P03")
		}
		probe.record(map[string]any{"phase": "failed_before_holder_release", "pid": query.PID, "holder_pid": firstPID})
		return true
	default:
		return false
	}
}

func r33PolicyProveAttempt(t *testing.T, ctx context.Context, witness *pgxpool.Conn, firstPID uint32, query r33PolicyQuery, probe *r33PolicyProbe) {
	t.Helper()
	observeCtx, cancel := context.WithDeadline(ctx, query.At.Add(r33PolicyShortRelease))
	defer cancel()
	for {
		if r33PolicyFinishedAttempt(t, firstPID, query, probe) {
			return
		}
		var blocked bool
		err := witness.QueryRow(observeCtx, r33PolicyWitnessSQL, int32(query.PID), int32(firstPID)).Scan(&blocked)
		if err != nil {
			if r33PolicyFinishedAttempt(t, firstPID, query, probe) {
				return
			}
			t.Fatalf("real project attempt was not witnessed before short release: %v", err)
		}
		if blocked {
			probe.record(map[string]any{"phase": "actual_backend_wait", "pid": query.PID, "holder_pid": firstPID})
			return
		}
		select {
		case <-observeCtx.Done():
			if r33PolicyFinishedAttempt(t, firstPID, query, probe) {
				return
			}
			t.Fatal("query-entry notification alone did not establish the actual lock attempt")
		case <-time.After(time.Millisecond):
		}
	}
}

func r33PolicyDelayUntil(t *testing.T, ctx context.Context, at time.Time) {
	t.Helper()
	select {
	case <-time.After(time.Until(at)):
	case <-ctx.Done():
		t.Fatal("policy transaction barrier exceeded the original test bound")
	}
}

func r33PolicyCheckAppend(t *testing.T, f *deliveryPolicyFixture, body deliveryPolicyRequest) {
	t.Helper()
	out := f.policy(t, http.MethodGet, nil, http.StatusOK)
	if out.ProjectID != f.project || out.Revision != 1 || out.Mode != body.Mode || out.PolicyVersion != body.PolicyVersion || out.Stage != body.Stage || out.Reason != body.Reason || out.CreatedBy == nil || *out.CreatedBy != f.actor || out.CreatedAt == nil || out.CreatedAt.IsZero() || out.CreatedAt.Location() != time.UTC || out.CreatedAt.Nanosecond()%1000 != 0 || out.SkipSupported || len(out.WaivableChecks) != 0 {
		t.Fatalf("actual policy append lost its exact requested/current-human binding: %+v", out)
	}
}

func r33PolicyWaitCase(t *testing.T, kind string) {
	t.Helper()
	f := newDeliveryPolicyFixture(t, false)
	initial := f.policy(t, http.MethodGet, nil, http.StatusOK)
	if initial.Revision != 0 || initial.Mode != "off" {
		t.Fatal("fixture is not a genuine current-authorized virtual0/off policy")
	}
	before := r33PolicySnapshot(t, f)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	var witness *pgxpool.Conn
	first, second := r33NewPolicyProbe("GetDeliveryPolicy"), r33NewPolicyProbe("")
	firstMethod := http.MethodGet
	firstBody := deliveryPolicyRequest{0, "observe", deliveryEvidencePolicy, deliveryPolicyStage, "first policy writer"}
	secondBody := deliveryPolicyRequest{0, "observe", deliveryEvidencePolicy, deliveryPolicyStage, "policy after short reader"}
	if kind == "competing_writer" {
		first.hold, firstMethod = "CreateDeliveryPolicy", http.MethodPut
	}
	var wg sync.WaitGroup
	cancels := []context.CancelFunc{}
	defer func() {
		first.unblock()
		second.unblock()
		for _, stop := range cancels {
			stop()
		}
		cancel()
		wg.Wait()
		if witness != nil {
			witness.Release()
		}
		for index, probe := range []*r33PolicyProbe{first, second} {
			for _, event := range probe.events {
				event["operation"] = index + 1
				raw, _ := json.Marshal(event)
				t.Logf("R33_POLICY_WAIT_JSON %s", raw)
			}
		}
	}()
	var err error
	witness, err = testPool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire the real read-only witness before holding a policy transaction: %v", err)
	}
	witnessPID := int32(witness.Conn().PgConn().PID())
	var warm bool
	if err = witness.QueryRow(ctx, r33PolicyWitnessSQL, witnessPID, int32(0)).Scan(&warm); err != nil || warm {
		t.Fatalf("warm the exact read-only witness before holding a policy transaction: blocked=%t err=%v", warm, err)
	}
	first.record(map[string]any{"phase": "read_only_witness_ready", "pid": witnessPID})
	start := func(probe *r33PolicyProbe, method string, body any) <-chan *testutil.Response {
		h := f.h
		h.TxStarter = probe.starter(f.h.TxStarter)
		req, stop := r33PolicyRequest(ctx, f, method, body)
		cancels = append(cancels, stop)
		action := h.GetDeliveryPolicy
		if method == http.MethodPut {
			action = h.UpdateDeliveryPolicy
		}
		done := make(chan *testutil.Response, 1)
		wg.Add(1)
		go func() { defer wg.Done(); done <- testutil.Call(t, action, req) }()
		return done
	}
	var firstInput any
	if firstMethod == http.MethodPut {
		firstInput = firstBody
	}
	firstDone := start(first, firstMethod, firstInput)
	select {
	case <-first.held:
	case response := <-firstDone:
		t.Fatalf("first actual policy operation missed its acquired-lock barrier: %d %s", response.Code, response.Text())
	case <-ctx.Done():
		t.Fatal("first actual policy transaction did not reach its barrier")
	}
	firstPID, readFence, budget, creates, _ := first.state()
	if firstPID == 0 || budget != 1 || firstMethod == http.MethodGet && !readFence || firstMethod == http.MethodPut && creates != 1 {
		t.Fatal("first barrier lacks its actual shared read or exclusive append/current250ms preparation")
	}
	secondDone := start(second, http.MethodPut, secondBody)
	var query r33PolicyQuery
	var secondResponse *testutil.Response
	select {
	case query = <-second.entered:
	case response := <-secondDone:
		// NOWAIT may finish before this select. Its buffered entry still proves
		// the actual query was called; retain the already completed response.
		select {
		case query = <-second.entered:
			secondResponse = response
		default:
			t.Fatalf("second actual policy PUT missed its canonical project-query entry: %d %s", response.Code, response.Text())
		}
	case <-ctx.Done():
		t.Fatal("second actual policy PUT did not reach its project lock")
	}
	if query.PID == 0 || query.PID == firstPID || query.Actor != f.actor || query.Kind != string(auth.CredentialJWT) || !reflect.DeepEqual(query.Args, []string{f.project, f.ws}) {
		t.Fatal("project lock attempt has the wrong current human/project/workspace or backend identity")
	}
	r33PolicyProveAttempt(t, ctx, witness, firstPID, query, second)
	if kind == "long_reader" {
		resultCtx, stop := context.WithTimeout(ctx, time.Second)
		if secondResponse == nil {
			secondResponse = r33PolicyAwait(t, resultCtx, secondDone)
		}
		stop()
		r33PolicyDelayUntil(t, ctx, query.At.Add(r33PolicyLockBudget+100*time.Millisecond))
		r33PolicyEffects(t, before, r33PolicySnapshot(t, f), 0)
	} else {
		r33PolicyDelayUntil(t, ctx, query.At.Add(r33PolicyShortRelease))
		if time.Since(query.At) >= r33PolicyLockBudget {
			t.Fatal("short release did not occur inside the unchanged250ms lock budget")
		}
	}
	first.record(map[string]any{"phase": "holder_release", "pid": firstPID, "second_pid": query.PID, "since_project_entry_ms": time.Since(query.At).Seconds() * 1000})
	first.unblock()
	firstResponse := r33PolicyAwait(t, ctx, firstDone)
	if secondResponse == nil {
		secondResponse = r33PolicyAwait(t, ctx, secondDone)
	}
	firstResponse.Want(http.StatusOK)
	if firstMethod == http.MethodGet {
		var observed deliveryPolicyResponse
		firstResponse.JSON(&observed)
		if !reflect.DeepEqual(observed, initial) {
			t.Fatal("held GET changed the original policy observation")
		}
	}
	_, _, secondBudget, secondCreates, lockResult := second.state()
	if secondBudget != 1 || secondResponse.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("policy PUT lost original250ms preparation or no-store response")
	}
	policyRows := 0
	if firstMethod == http.MethodPut || secondResponse.Code == http.StatusOK {
		policyRows = 1
	}
	r33PolicyEffects(t, before, r33PolicySnapshot(t, f), policyRows)
	t.Logf("R33_POLICY_HTTP first=%d second=%d second_body=%s", firstResponse.Code, secondResponse.Code, secondResponse.Text())
	if kind == "competing_writer" {
		r33PolicyCheckAppend(t, f, firstBody)
		if secondCreates != 0 {
			t.Fatal("losing policy writer attempted another append")
		}
		if secondResponse.Want(http.StatusConflict).Map()["error"] != "delivery_policy_revision_conflict" {
			t.Fatal("waiting competing writer did not reach the exact stale-revision CAS refusal")
		}
	} else if kind == "long_reader" {
		var pgError *pgconn.PgError
		if !errors.As(lockResult.Err, &pgError) || pgError.Code != "55P03" || secondCreates != 0 || secondResponse.Want(http.StatusConflict).Map()["error"] != "artifact_authority_busy" {
			t.Fatal("long-held actual reader did not retain native55P03/busy refusal and zero append")
		}
	} else {
		secondResponse.Want(http.StatusOK)
		if secondCreates != 1 || lockResult.Err != nil {
			t.Fatal("short-reader release did not allow exactly one real policy append")
		}
		r33PolicyCheckAppend(t, f, secondBody)
	}
}

func TestDeliveryPolicyProjectWaitShortReader(t *testing.T) {
	t.Run("short_reader_release", func(t *testing.T) { r33PolicyWaitCase(t, "short_reader_release") })
}

func TestDeliveryPolicyProjectWaitBoundaries(t *testing.T) {
	t.Run("long_reader", func(t *testing.T) { r33PolicyWaitCase(t, "long_reader") })
	t.Run("competing_writer", func(t *testing.T) { r33PolicyWaitCase(t, "competing_writer") })
	t.Run("member_denied", func(t *testing.T) {
		f := newDeliveryPolicyFixture(t, false)
		memberID := dbfx.User(t, "Policy member", "policy-member-"+uuid.NewString()+"@example.invalid")
		dbfx.Member(t, f.ws, memberID, "member")
		member, source := *f, *f.artifactFixture
		source.actor = memberID
		member.artifactFixture = &source
		member.policy(t, http.MethodGet, nil, http.StatusOK)
		before := r33PolicySnapshot(t, f)
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()
		probe := r33NewPolicyProbe("")
		h := member.h
		h.TxStarter = probe.starter(h.TxStarter)
		req, stop := r33PolicyRequest(ctx, &member, http.MethodPut, deliveryPolicyRequest{0, "observe", deliveryEvidencePolicy, deliveryPolicyStage, "member cannot manage"})
		defer stop()
		response := testutil.Call(t, h.UpdateDeliveryPolicy, req).Want(http.StatusForbidden)
		if response.Map()["error"] != "delivery_policy_management_denied" {
			t.Fatal("current member received a different policy refusal")
		}
		_, _, budget, creates, projectQuery := probe.state()
		if budget != 1 || creates != 0 || projectQuery.Name != "" || len(probe.entered) != 0 {
			t.Fatal("current member denial reached a project-writer query or append")
		}
		r33PolicyEffects(t, before, r33PolicySnapshot(t, f), 0)
	})
}
