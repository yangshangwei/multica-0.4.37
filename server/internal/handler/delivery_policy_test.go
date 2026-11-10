package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/testutil"
)

type deliveryPolicyFixture struct {
	*artifactFixture
	project string
	receipt artifactResponse
	req     deliveryEvaluationRequest
	pr      string
}

func deliveryPolicyAttach(t *testing.T, f *artifactFixture, receipt artifactResponse, req deliveryEvidenceRequest, pr string) *deliveryPolicyFixture {
	t.Helper()
	project := dbfx.Project(t, "Delivery policy project", testutil.Cols{"workspace_id": f.ws, "lead_type": "member", "lead_id": f.actor})
	dbfx.Exec(t, `UPDATE issue SET project_id=$1,revision=revision+1 WHERE id=$2`, project, f.req.ResourceID)
	dbfx.QueryRow(t, `SELECT revision FROM issue WHERE id=$1`, f.req.ResourceID).Scan(&req.IssueRevision)
	var rev int64
	dbfx.QueryRow(t, `SELECT revision FROM project WHERE id=$1`, project).Scan(&rev)
	req.ProjectID = &project
	req.ProjectRevision = &rev
	return &deliveryPolicyFixture{f, project, receipt, deliveryEvaluationRequest{req, 1}, pr}
}
func newDeliveryPolicyFixture(t *testing.T, enable bool) *deliveryPolicyFixture {
	t.Helper()
	f, out, req, pr := deliverySuccess(t)
	d := deliveryPolicyAttach(t, f, out, req, pr)
	if enable {
		d.policy(t, "PUT", deliveryPolicyRequest{0, "observe", deliveryEvidencePolicy, deliveryPolicyStage, "Observe delivery"}, 200)
	}
	return d
}
func (f *deliveryPolicyFixture) policy(t *testing.T, method string, body any, status int) deliveryPolicyResponse {
	t.Helper()
	var out deliveryPolicyResponse
	req := testutil.WithURLParams(f.request(method, "/api/projects/"+f.project+"/delivery-policy", body), "id", f.project)
	call := f.h.GetDeliveryPolicy
	if method == "PUT" {
		call = f.h.UpdateDeliveryPolicy
	}
	testutil.Call(t, call, req).Want(status).JSON(&out)
	return out
}
func (f *deliveryPolicyFixture) evalRequest(method, key string, body any) *http.Request {
	path := "/api/issues/" + f.artifactFixture.req.ResourceID + "/delivery-evidence/evaluations"
	if method == "GET" {
		path += "/" + key
	}
	req := testutil.WithURLParams(f.request(method, path, body), "id", f.artifactFixture.req.ResourceID, "key", key)
	req.Header.Set("Idempotency-Key", key)
	return req
}
func (f *deliveryPolicyFixture) evaluation(t *testing.T, method, key string, body any, status int) deliveryEvaluationResponse {
	t.Helper()
	var out deliveryEvaluationResponse
	call := f.h.GetDeliveryEvaluation
	if method == "POST" {
		call = f.h.CreateDeliveryEvaluation
	}
	result := testutil.Call(t, call, f.evalRequest(method, key, body)).Want(status)
	result.JSON(&out)
	if result.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("evaluation cacheable")
	}
	return out
}
func (f *deliveryPolicyFixture) stored(t *testing.T, key string) string {
	t.Helper()
	var raw string
	dbfx.QueryRow(t, `SELECT to_jsonb(e)::text FROM delivery_evidence_evaluation e WHERE workspace_id=$1 AND evaluator_user_id=$2 AND request_key=$3`, f.ws, f.actor, key).Scan(&raw)
	return raw
}
func (f *deliveryPolicyFixture) evaluationCount(t *testing.T) int {
	t.Helper()
	return dbfx.Count(t, `SELECT count(*) FROM delivery_evidence_evaluation WHERE workspace_id=$1`, f.ws)
}

func TestDeliveryPolicyDefaultAppendAndCAS(t *testing.T) {
	f := newDeliveryPolicyFixture(t, false)
	before := *f.req.ProjectRevision
	off := f.policy(t, "GET", nil, 200)
	if off.Revision != 0 || off.Mode != "off" || off.CreatedBy != nil || off.CreatedAt != nil {
		t.Fatal(off)
	}
	f.req.PolicyRevision = 0
	f.evaluation(t, "POST", uuid.NewString(), f.req, 409)
	if f.evaluationCount(t) != 0 {
		t.Fatal("disabled policy wrote evaluation")
	}
	for i, mode := range []string{"observe", "observe", "off", "observe"} {
		out := f.policy(t, "PUT", deliveryPolicyRequest{int64(i), mode, deliveryEvidencePolicy, deliveryPolicyStage, " audit reason "}, 200)
		if out.Revision != int64(i+1) || out.Mode != mode || out.CreatedBy == nil || *out.CreatedBy != f.actor || out.Reason != "audit reason" || out.CreatedAt == nil {
			t.Fatal(out)
		}
	}
	f.policy(t, "PUT", deliveryPolicyRequest{0, "off", deliveryEvidencePolicy, deliveryPolicyStage, "stale"}, 409)
	var rev int64
	dbfx.QueryRow(t, `SELECT revision FROM project WHERE id=$1`, f.project).Scan(&rev)
	if rev != before || dbfx.Count(t, `SELECT count(*) FROM project_delivery_policy WHERE project_id=$1`, f.project) != 4 {
		t.Fatal("policy CAS rewrote history or project revision")
	}
}

func TestDeliveryPolicyManagementAuthority(t *testing.T) {
	for _, role := range []string{"owner", "admin", "member"} {
		t.Run(role, func(t *testing.T) {
			f := newDeliveryPolicyFixture(t, false)
			dbfx.Exec(t, `UPDATE member SET role=$1 WHERE workspace_id=$2 AND user_id=$3`, role, f.ws, f.actor)
			f.policy(t, "GET", nil, 200)
			status := 403
			if role == "owner" || role == "admin" {
				status = 200
			}
			f.policy(t, "PUT", deliveryPolicyRequest{0, "observe", deliveryEvidencePolicy, deliveryPolicyStage, "managed"}, status)
			want := 0
			if status == 200 {
				want = 1
			}
			if dbfx.Count(t, `SELECT count(*) FROM project_delivery_policy WHERE project_id=$1`, f.project) != want {
				t.Fatal("wrong authority write")
			}
		})
	}
}

func TestDeliveryPolicyStrictRequests(t *testing.T) {
	bodies := []string{
		`{}`, `null`, `{"expected_policy_revision":0,"mode":"skip","policy_version":"delivery-artifact-observe-v1","stage":"done","reason":"x"}`,
		`{"expected_policy_revision":0,"mode":"observe","policy_version":"delivery-artifact-observe-v1","stage":"done","reason":null}`,
		`{"expected_policy_revision":0,"mode":"observe","policy_version":"delivery-artifact-observe-v1","stage":"done","reason":" "}`,
		`{"expected_policy_revision":0,"mode":"observe","policy_version":"delivery-artifact-observe-v1","stage":"done","reason":"x","reason":"y"}`,
		`{"expected_policy_revision":0,"mode":"observe","policy_version":"delivery-artifact-observe-v1","stage":"in_review","reason":"x"}`,
		`{"expected_policy_revision":0,"mode":"observe","policy_version":"delivery-artifact-observe-v1","stage":"done","Reason":"x"}`,
		`{"expected_policy_revision":-1,"mode":"observe","policy_version":"delivery-artifact-observe-v1","stage":"done","reason":"x"}`,
		`{"expected_policy_revision":0,"mode":"observe","policy_version":"delivery-artifact-observe-v2","stage":"done","reason":"x"}`,
		`{"expected_policy_revision":0,"mode":"observe","policy_version":"delivery-artifact-observe-v1","stage":"done","reason":"x","skip":true}`,
	}
	f := newDeliveryPolicyFixture(t, false)
	for i, body := range bodies {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			req := testutil.WithURLParams(f.request("PUT", "/api/projects/"+f.project+"/delivery-policy", nil), "id", f.project)
			req.Body = io.NopCloser(strings.NewReader(body))
			testutil.Call(t, f.h.UpdateDeliveryPolicy, req).Want(400)
		})
	}
	if dbfx.Count(t, `SELECT count(*) FROM project_delivery_policy WHERE project_id=$1`, f.project) != 0 {
		t.Fatal("malformed policy wrote history")
	}
}

func TestDeliveryEvaluationPersistentIdentityAndRestart(t *testing.T) {
	f := newDeliveryPolicyFixture(t, true)
	key := uuid.NewString()
	counter := &artifactCounted{artifactExecutor: f.h.ArtifactPackager}
	f.h.ArtifactPackager = counter
	first := f.evaluation(t, "POST", key, f.req, 200)
	if first.Observation.Verdict != "pass" || first.Observation.ReceiptDigest != f.receipt.ReceiptDigest || first.Observation.ExecutionID != f.receipt.ExecutionID || first.Applicability.Status != "current" || first.UsableForCompletion || first.EvaluatorUserID != f.actor || first.PolicyRevision != 1 || first.EvaluationID == "" {
		t.Fatal(first)
	}
	saved := f.stored(t, key)
	restarted := *testHandler
	restarted.ArtifactPackager = nil
	f.h = restarted
	for _, method := range []string{"POST", "GET"} {
		t.Run(method, func(t *testing.T) {
			replay := f.evaluation(t, method, key, f.req, 200)
			if replay.EvaluationID != first.EvaluationID || !reflect.DeepEqual(replay.Observation, first.Observation) || replay.Applicability.Status != "current" || replay.UsableForCompletion || f.stored(t, key) != saved {
				t.Fatal("recovery mutated snapshot", replay)
			}
		})
	}
	changed := f.req
	changed.ArtifactDigest = strings.Repeat("0", 64)
	f.evaluation(t, "POST", key, changed, 409)
	if f.evaluationCount(t) != 1 || counter.calls.Load() != 0 {
		t.Fatal("recovery inserted or executed")
	}
	// The original endpoint remains stateless even with an enabled project policy.
	deliveryCall(t, f.artifactFixture, f.req.deliveryEvidenceRequest, 200)
	if f.evaluationCount(t) != 1 {
		t.Fatal("old observer persisted an evaluation")
	}
}

func TestDeliveryEvaluationInvalidationAndImmutableRecovery(t *testing.T) {
	for _, change := range []string{"policy_off", "policy_aba", "issue", "project", "move", "detach", "head", "missing_head", "ambiguous_head", "receipt", "missing_execution", "producer_generation"} {
		t.Run(change, func(t *testing.T) {
			f := newDeliveryPolicyFixture(t, true)
			key := uuid.NewString()
			first := f.evaluation(t, "POST", key, f.req, 200)
			saved := f.stored(t, key)
			status, want := 200, "stale"
			switch change {
			case "policy_off":
				f.policy(t, "PUT", deliveryPolicyRequest{1, "off", deliveryEvidencePolicy, deliveryPolicyStage, "off"}, 200)
			case "policy_aba":
				f.policy(t, "PUT", deliveryPolicyRequest{1, "off", deliveryEvidencePolicy, deliveryPolicyStage, "off"}, 200)
				f.policy(t, "PUT", deliveryPolicyRequest{2, "observe", deliveryEvidencePolicy, deliveryPolicyStage, "back"}, 200)
			case "issue":
				dbfx.Exec(t, `UPDATE issue SET revision=revision+1 WHERE id=$1`, f.artifactFixture.req.ResourceID)
			case "project":
				dbfx.Exec(t, `UPDATE project SET revision=revision+1 WHERE id=$1`, f.project)
			case "move":
				other := dbfx.Project(t, "Moved", testutil.Cols{"workspace_id": f.ws})
				dbfx.Exec(t, `UPDATE issue SET project_id=$1,revision=revision+1 WHERE id=$2`, other, f.artifactFixture.req.ResourceID)
			case "detach":
				dbfx.Exec(t, `UPDATE issue SET project_id=NULL,revision=revision+1 WHERE id=$1`, f.artifactFixture.req.ResourceID)
			case "head":
				dbfx.Exec(t, `UPDATE github_pull_request SET head_sha=$1 WHERE id=$2`, strings.Repeat("b", 40), f.pr)
			case "missing_head":
				dbfx.Exec(t, `DELETE FROM issue_pull_request WHERE issue_id=$1`, f.artifactFixture.req.ResourceID)
				want = "unknown"
			case "ambiguous_head":
				var at time.Time
				dbfx.QueryRow(t, `SELECT pr_updated_at FROM github_pull_request WHERE id=$1`, f.pr).Scan(&at)
				deliveryPR(t, f.artifactFixture, strings.Repeat("b", 40), at)
				want = "unknown"
			case "receipt":
				dbfx.Exec(t, `UPDATE agent_artifact_execution SET receipt_json=receipt_json || '{"result":"failed"}'::jsonb WHERE approval_id=$1`, f.req.ApprovalID)
			case "missing_execution":
				dbfx.Exec(t, `DELETE FROM agent_artifact_execution WHERE approval_id=$1`, f.req.ApprovalID)
				status = 409
			case "producer_generation":
				dbfx.Exec(t, `UPDATE agent_task_queue SET claim_generation=claim_generation+1 WHERE id=$1`, f.artifactFixture.req.TaskID)
				status = 409
			}
			for _, method := range []string{"GET", "POST"} {
				out := f.evaluation(t, method, key, f.req, status)
				if status == 200 && (out.Applicability.Status != want || !reflect.DeepEqual(out.Observation, first.Observation) || out.UsableForCompletion) {
					t.Fatal(out)
				}
			}
			if f.stored(t, key) != saved || f.evaluationCount(t) != 1 {
				t.Fatal("invalidation changed immutable row")
			}
		})
	}
}

func TestDeliveryEvaluationUnknownAndHoldNeverUpgrade(t *testing.T) {
	for _, verdict := range []string{"unknown", "hold", "unbound_unknown"} {
		t.Run(verdict, func(t *testing.T) {
			f := newDeliveryPolicyFixture(t, true)
			key := uuid.NewString()
			switch verdict {
			case "unknown":
				dbfx.Exec(t, `DELETE FROM issue_pull_request WHERE issue_id=$1`, f.artifactFixture.req.ResourceID)
			case "hold":
				dbfx.Exec(t, `UPDATE github_pull_request SET head_sha=$1 WHERE id=$2`, strings.Repeat("b", 40), f.pr)
			case "unbound_unknown":
				f.req.ApprovalID = uuid.NewString()
			}
			first := f.evaluation(t, "POST", key, f.req, 200)
			saved := f.stored(t, key)
			if first.Observation.Verdict == "pass" {
				t.Fatal(first)
			}
			if verdict == "unknown" {
				dbfx.InsertNoID(t, "issue_pull_request", testutil.Cols{"issue_id": f.artifactFixture.req.ResourceID, "pull_request_id": f.pr}, "issue_id=$1 AND pull_request_id=$2", f.artifactFixture.req.ResourceID, f.pr)
			}
			if verdict == "hold" {
				dbfx.Exec(t, `UPDATE github_pull_request SET head_sha=$1 WHERE id=$2`, f.req.SourceRevision, f.pr)
			}
			for _, method := range []string{"POST", "GET"} {
				out := f.evaluation(t, method, key, f.req, 200)
				if out.Applicability.Status == "current" || !reflect.DeepEqual(out.Observation, first.Observation) || f.stored(t, key) != saved {
					t.Fatal("old negative upgraded", out)
				}
			}
			if verdict != "unbound_unknown" {
				fresh := f.evaluation(t, "POST", uuid.NewString(), f.req, 200)
				if fresh.Observation.Verdict != "pass" || fresh.Applicability.Status != "current" {
					t.Fatal(fresh)
				}
			}
		})
	}
}

func TestDeliveryEvaluationActorIsolationAndStrictBody(t *testing.T) {
	f := newDeliveryPolicyFixture(t, true)
	key := uuid.NewString()
	f.evaluation(t, "POST", key, f.req, 200)
	owner := f.actor
	other := dbfx.User(t, "Other observer", "delivery-"+uuid.NewString()+"@example.invalid")
	dbfx.Member(t, f.ws, other, "admin")
	f.actor = other
	f.evaluation(t, "GET", key, nil, 404)
	f.actor = owner
	for _, bad := range []string{"", "not-uuid", "00000000-0000-0000-0000-000000000000", strings.ToUpper(key)} {
		t.Run("key_"+bad, func(t *testing.T) { f.evaluation(t, "POST", bad, f.req, 400) })
	}
	raw, _ := json.Marshal(f.req)
	for _, bad := range []string{strings.TrimSuffix(string(raw), "}") + `,"skip":true}`, strings.TrimSuffix(string(raw), "}") + `,"expected_policy_revision":1}`, strings.Replace(string(raw), `"expected_policy_revision":1`, `"expected_policy_revision":null`, 1)} {
		req := f.evalRequest("POST", uuid.NewString(), nil)
		req.Body = io.NopCloser(strings.NewReader(bad))
		testutil.Call(t, f.h.CreateDeliveryEvaluation, req).Want(400)
	}
	duplicate := f.evalRequest("POST", uuid.NewString(), f.req)
	duplicate.Header.Add("Idempotency-Key", uuid.NewString())
	testutil.Call(t, f.h.CreateDeliveryEvaluation, duplicate).Want(400)
	if f.evaluationCount(t) != 1 {
		t.Fatal("bad request wrote row")
	}
}

func TestDeliveryEvaluationCommitUncertainty(t *testing.T) {
	for _, after := range []bool{false, true} {
		t.Run(fmt.Sprint(after), func(t *testing.T) {
			f := newDeliveryPolicyFixture(t, true)
			key := uuid.NewString()
			original := f.h.TxStarter
			f.h.TxStarter = &artifactFaultStarter{txStarter: original, failAt: 1, after: after}
			out := f.evaluation(t, "POST", key, f.req, 503)
			if out.EvaluationID != "" || out.Observation.Verdict != "" {
				t.Fatal("uncertain commit returned success")
			}
			want := 0
			if after {
				want = 1
			}
			if f.evaluationCount(t) != want {
				t.Fatal("wrong actual commit state")
			}
			f.h.TxStarter = original
			if after {
				saved := f.stored(t, key)
				f.evaluation(t, "GET", key, nil, 200)
				if saved != f.stored(t, key) {
					t.Fatal("recovery wrote")
				}
			} else {
				f.evaluation(t, "GET", key, nil, 404)
				f.evaluation(t, "POST", key, f.req, 200)
			}
			if f.evaluationCount(t) != 1 {
				t.Fatal("explicit recovery duplicated")
			}
		})
	}
}

func TestDeliveryPolicyCommitUncertainty(t *testing.T) {
	for _, after := range []bool{false, true} {
		t.Run(fmt.Sprint(after), func(t *testing.T) {
			f := newDeliveryPolicyFixture(t, false)
			original := f.h.TxStarter
			f.h.TxStarter = &artifactFaultStarter{txStarter: original, failAt: 1, after: after}
			body := deliveryPolicyRequest{0, "observe", deliveryEvidencePolicy, deliveryPolicyStage, "retained"}
			out := f.policy(t, "PUT", body, 503)
			if out.Revision != 0 {
				t.Fatal("uncertain policy commit returned success")
			}
			f.h.TxStarter = original
			want := 0
			if after {
				want = 1
			}
			if got := f.policy(t, "GET", nil, 200); got.Revision != int64(want) {
				t.Fatal(got)
			}
			if after {
				f.policy(t, "PUT", body, 409)
			} else {
				f.policy(t, "PUT", body, 200)
			}
		})
	}
}

func TestDeliveryEvaluationConcurrentSingleRow(t *testing.T) {
	f := newDeliveryPolicyFixture(t, true)
	key := uuid.NewString()
	original := f.h.TxStarter
	entered, release := make(chan struct{}), make(chan struct{})
	f.h.TxStarter = &artifactBarrierStarter{txStarter: original, entered: entered, release: release}
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		w := httptest.NewRecorder()
		f.h.CreateDeliveryEvaluation(w, f.evalRequest("POST", key, f.req))
		done <- w
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("no commit barrier")
	}
	// Use another handler to avoid sharing the barrier's test-only counter.
	concurrent := f.h
	concurrent.TxStarter = original
	w := httptest.NewRecorder()
	concurrent.CreateDeliveryEvaluation(w, f.evalRequest("POST", key, f.req))
	if w.Code != 409 {
		close(release)
		t.Fatalf("concurrent response %d %s", w.Code, w.Body.String())
	}
	close(release)
	first := <-done
	if first.Code != 200 {
		t.Fatalf("first response %d %s", first.Code, first.Body.String())
	}
	f.h.TxStarter = original
	f.evaluation(t, "POST", key, f.req, 200)
	if f.evaluationCount(t) != 1 {
		t.Fatal("duplicate row")
	}
}

func TestDeliveryPolicyEvaluationFencesChanges(t *testing.T) {
	for _, kind := range []string{"policy", "member", "project", "issue"} {
		t.Run(kind, func(t *testing.T) {
			f := newDeliveryPolicyFixture(t, true)
			original := f.h.TxStarter
			entered, release := make(chan struct{}), make(chan struct{})
			f.h.TxStarter = &artifactBarrierStarter{txStarter: original, entered: entered, release: release}
			key := uuid.NewString()
			done := make(chan *httptest.ResponseRecorder, 1)
			go func() {
				w := httptest.NewRecorder()
				f.h.CreateDeliveryEvaluation(w, f.evalRequest("POST", key, f.req))
				done <- w
			}()
			select {
			case <-entered:
			case <-time.After(5 * time.Second):
				t.Fatal("missing evaluation commit")
			}
			if kind == "policy" {
				concurrent := f.h
				concurrent.TxStarter = original
				req := testutil.WithURLParams(f.request("PUT", "/api/projects/"+f.project+"/delivery-policy", deliveryPolicyRequest{1, "off", deliveryEvidencePolicy, deliveryPolicyStage, "concurrent"}), "id", f.project)
				w := httptest.NewRecorder()
				concurrent.UpdateDeliveryPolicy(w, req)
				if w.Code != 409 {
					close(release)
					t.Fatalf("policy bypassed %d", w.Code)
				}
			} else {
				tx, err := testPool.Begin(context.Background())
				if err != nil {
					close(release)
					t.Fatal(err)
				}
				if _, err = tx.Exec(context.Background(), `SET LOCAL lock_timeout='100ms'`); err != nil {
					t.Fatal(err)
				}
				stmt, id := `UPDATE issue SET revision=revision+1 WHERE id=$1`, f.artifactFixture.req.ResourceID
				if kind == "project" {
					stmt, id = `UPDATE project SET revision=revision+1 WHERE id=$1`, f.project
				}
				if kind == "member" {
					stmt, id = `UPDATE member SET role='member' WHERE workspace_id=$1`, f.ws
				}
				_, err = tx.Exec(context.Background(), stmt, id)
				pgerr, ok := err.(*pgconn.PgError)
				if !ok || pgerr.Code != "55P03" {
					close(release)
					t.Fatalf("writer not fenced: %v", err)
				}
				tx.Rollback(context.Background())
			}
			close(release)
			w := <-done
			if w.Code != 200 {
				t.Fatalf("original %d %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestDeliveryEvaluationDeletionRetention(t *testing.T) {
	for _, target := range []string{"project", "issue", "workspace"} {
		t.Run(target, func(t *testing.T) {
			f := newDeliveryPolicyFixture(t, true)
			key := uuid.NewString()
			f.evaluation(t, "POST", key, f.req, 200)
			saved := f.stored(t, key)
			switch target {
			case "project":
				testutil.Call(t, f.h.DeleteProject, testutil.WithURLParams(f.request("DELETE", "/api/projects/"+f.project, nil), "id", f.project)).Want(204)
				f.policy(t, "GET", nil, 404)
			case "issue":
				testutil.Call(t, f.h.DeleteIssue, testutil.WithURLParams(f.request("DELETE", "/api/issues/"+f.artifactFixture.req.ResourceID, nil), "id", f.artifactFixture.req.ResourceID)).Want(204)
			case "workspace":
				f.deleteWorkspace(t)
			}
			if f.stored(t, key) != saved || dbfx.Count(t, `SELECT count(*) FROM project_delivery_policy WHERE workspace_id=$1`, f.ws) != 1 {
				t.Fatal("resource deletion discarded evidence")
			}
			if target == "project" {
				out := f.evaluation(t, "GET", key, nil, 200)
				if out.Applicability.Status != "stale" {
					t.Fatal(out)
				}
			} else {
				f.evaluation(t, "GET", key, nil, 404)
			}
		})
	}
}

func TestDeliveryEvaluationRevocationEvenAfterVersionDrift(t *testing.T) {
	for _, verdict := range []string{"pass", "hold", "unknown"} {
		t.Run(verdict, func(t *testing.T) {
			f := newDeliveryPolicyFixture(t, true)
			key := uuid.NewString()
			producer := f.actor
			observer := dbfx.User(t, "Observer", "observer-"+uuid.NewString()+"@example.invalid")
			dbfx.Member(t, f.ws, observer, "admin")
			f.actor = observer
			if verdict == "hold" {
				dbfx.Exec(t, `UPDATE github_pull_request SET head_sha=$1 WHERE id=$2`, strings.Repeat("b", 40), f.pr)
			}
			if verdict == "unknown" {
				dbfx.Exec(t, `DELETE FROM issue_pull_request WHERE issue_id=$1`, f.artifactFixture.req.ResourceID)
			}
			first := f.evaluation(t, "POST", key, f.req, 200)
			if first.Observation.Verdict != verdict {
				t.Fatal(first)
			}
			saved := f.stored(t, key)
			dbfx.Exec(t, `UPDATE issue SET revision=revision+1 WHERE id=$1`, f.artifactFixture.req.ResourceID)
			f.policy(t, "PUT", deliveryPolicyRequest{1, "off", deliveryEvidencePolicy, deliveryPolicyStage, "off"}, 200)
			dbfx.Exec(t, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, f.ws, producer)
			for _, method := range []string{"GET", "POST"} {
				f.evaluation(t, method, key, f.req, 404)
			}
			if saved != f.stored(t, key) {
				t.Fatal("revocation rewrote history")
			}
		})
	}
}

func TestDeliveryEvaluationCurrentCredentialBackstop(t *testing.T) {
	f := newDeliveryPolicyFixture(t, true)
	key := uuid.NewString()
	f.evaluation(t, "POST", key, f.req, 200)
	for _, kind := range []string{"expired", "forged_agent", "unverified"} {
		t.Run(kind, func(t *testing.T) {
			req := f.evalRequest("GET", key, nil)
			switch kind {
			case "expired":
				req = req.WithContext(auth.WithAuthenticatedPrincipal(req.Context(), auth.AuthenticatedPrincipal{UserID: f.actor, Kind: auth.CredentialJWT, ExpiresAt: time.Now().Add(-time.Second)}))
			case "forged_agent":
				req.Header.Set("X-Agent-ID", f.artifactFixture.req.AgentID)
			case "unverified":
				req = req.WithContext(context.Background())
			}
			testutil.Call(t, f.h.GetDeliveryEvaluation, req).Want(403)
		})
	}
}

func TestDeliveryEvaluationNoProjectAndVersionConflict(t *testing.T) {
	f := newDeliveryPolicyFixture(t, true)
	for _, kind := range []string{"issue", "project", "policy", "no_project"} {
		t.Run(kind, func(t *testing.T) {
			req := f.req
			switch kind {
			case "issue":
				req.IssueRevision++
			case "project":
				v := *req.ProjectRevision + 1
				req.ProjectRevision = &v
			case "policy":
				req.PolicyRevision++
			case "no_project":
				dbfx.Exec(t, `UPDATE issue SET project_id=NULL WHERE id=$1`, f.artifactFixture.req.ResourceID)
			}
			f.evaluation(t, "POST", uuid.NewString(), req, 409)
		})
	}
	if f.evaluationCount(t) != 0 {
		t.Fatal("rejected admission created evidence")
	}
}

func TestDeliveryPolicyConcurrentCAS(t *testing.T) {
	f := newDeliveryPolicyFixture(t, false)
	original := f.h.TxStarter
	entered, release := make(chan struct{}), make(chan struct{})
	f.h.TxStarter = &artifactBarrierStarter{txStarter: original, entered: entered, release: release}
	body := deliveryPolicyRequest{0, "observe", deliveryEvidencePolicy, deliveryPolicyStage, "one"}
	request := func() *http.Request {
		return testutil.WithURLParams(f.request("PUT", "/api/projects/"+f.project+"/delivery-policy", body), "id", f.project)
	}
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() { w := httptest.NewRecorder(); f.h.UpdateDeliveryPolicy(w, request()); done <- w }()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("no policy commit")
	}
	concurrent := f.h
	concurrent.TxStarter = original
	w := httptest.NewRecorder()
	concurrent.UpdateDeliveryPolicy(w, request())
	if w.Code != 409 {
		close(release)
		t.Fatalf("CAS raced through: %d %s", w.Code, w.Body.String())
	}
	close(release)
	if first := <-done; first.Code != 200 {
		t.Fatal(first.Code, first.Body.String())
	}
	f.h.TxStarter = original
	f.policy(t, "PUT", body, 409)
	if dbfx.Count(t, `SELECT count(*) FROM project_delivery_policy WHERE project_id=$1`, f.project) != 1 {
		t.Fatal("concurrent policy appended twice")
	}
}

func TestDeliveryPolicyWriterFirstRefuses(t *testing.T) {
	for _, kind := range []string{"project", "member"} {
		t.Run(kind, func(t *testing.T) {
			f := newDeliveryPolicyFixture(t, false)
			tx, err := testPool.Begin(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(context.Background())
			stmt, id := `UPDATE project SET revision=revision+1 WHERE id=$1`, f.project
			if kind == "member" {
				stmt, id = `UPDATE member SET role='member' WHERE workspace_id=$1`, f.ws
			}
			if _, err = tx.Exec(context.Background(), stmt, id); err != nil {
				t.Fatal(err)
			}
			f.policy(t, "PUT", deliveryPolicyRequest{0, "observe", deliveryEvidencePolicy, deliveryPolicyStage, "blocked"}, 409)
			if dbfx.Count(t, `SELECT count(*) FROM project_delivery_policy WHERE project_id=$1`, f.project) != 0 {
				t.Fatal("busy policy wrote")
			}
		})
	}
}

func TestDeliveryEvaluationWriterFirstRefuses(t *testing.T) {
	for _, kind := range []string{"issue", "project", "member", "pr", "approval", "execution"} {
		t.Run(kind, func(t *testing.T) {
			f := newDeliveryPolicyFixture(t, true)
			key := uuid.NewString()
			tx, err := testPool.Begin(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(context.Background())
			stmt, id := `UPDATE issue SET revision=revision+1 WHERE id=$1`, f.artifactFixture.req.ResourceID
			switch kind {
			case "project":
				stmt, id = `UPDATE project SET revision=revision+1 WHERE id=$1`, f.project
			case "member":
				stmt, id = `UPDATE member SET role='member' WHERE workspace_id=$1`, f.ws
			case "pr":
				stmt, id = `UPDATE github_pull_request SET head_sha=head_sha WHERE id=$1`, f.pr
			case "approval":
				stmt, id = `UPDATE agent_approval_request SET decision_note=decision_note WHERE id=$1`, f.req.ApprovalID
			case "execution":
				stmt, id = `UPDATE agent_artifact_execution SET result_code=result_code WHERE approval_id=$1`, f.req.ApprovalID
			}
			if _, err = tx.Exec(context.Background(), stmt, id); err != nil {
				t.Fatal(err)
			}
			f.evaluation(t, "POST", key, f.req, 409)
			if f.evaluationCount(t) != 0 {
				t.Fatal("busy evaluation wrote")
			}
		})
	}
}

func TestDeliveryEvaluationDistinctHistoricalPrincipals(t *testing.T) {
	for _, role := range []string{"originator", "runtime_owner", "initiator", "approver", "claimant", "invoke_originator"} {
		t.Run(role, func(t *testing.T) {
			a := newArtifactFixture(t, true)
			people := map[string]string{"runtime_owner": a.actor}
			for _, name := range []string{"observer", "originator", "initiator", "approver", "claimant"} {
				user := dbfx.User(t, name, name+uuid.NewString()+"@example.invalid")
				dbfx.Member(t, a.ws, user, "admin")
				people[name] = user
				dbfx.Insert(t, "agent_invocation_target", testutil.Cols{"agent_id": a.req.AgentID, "target_type": "member", "target_id": user})
			}
			dbfx.Exec(t, `UPDATE agent SET permission_mode='public_to' WHERE id=$1`, a.req.AgentID)
			dbfx.Exec(t, `UPDATE agent_task_queue SET originator_user_id=$1,accountable_user_id=$1 WHERE id=$2`, people["originator"], a.req.TaskID)
			a.actor = people["initiator"]
			receipt := a.create(t)
			a.actor = people["approver"]
			a.approve(t, receipt)
			a.actor = people["claimant"]
			receipt = a.execute(t, receipt, 200)
			pr := deliveryPR(t, a, a.req.SourceRevision, time.Now())
			req := deliveryRequest(t, a, receipt)
			a.actor = people["observer"]
			f := deliveryPolicyAttach(t, a, receipt, req, pr)
			f.policy(t, "PUT", deliveryPolicyRequest{0, "observe", deliveryEvidencePolicy, deliveryPolicyStage, "evidence"}, 200)
			dbfx.Exec(t, `UPDATE agent_task_queue SET status='completed' WHERE id=$1`, a.req.TaskID)
			key := uuid.NewString()
			first := f.evaluation(t, "POST", key, f.req, 200)
			if first.Observation.Verdict != "pass" {
				t.Fatal(first)
			}
			saved := f.stored(t, key)
			dbfx.Exec(t, `UPDATE issue SET revision=revision+1 WHERE id=$1`, a.req.ResourceID)
			status := 403
			if role == "invoke_originator" {
				dbfx.Exec(t, `DELETE FROM agent_invocation_target WHERE agent_id=$1 AND target_id=$2`, a.req.AgentID, people["originator"])
			} else {
				dbfx.Exec(t, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, a.ws, people[role])
				if role == "originator" || role == "runtime_owner" {
					status = 404
				}
			}
			f.evaluation(t, "GET", key, nil, status)
			f.evaluation(t, "POST", key, f.req, status)
			if saved != f.stored(t, key) {
				t.Fatal("revocation changed prior evidence")
			}
		})
	}
}
