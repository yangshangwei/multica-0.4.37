package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func (f *deliveryPolicyFixture) hintRequest(body any) *http.Request {
	id := f.artifactFixture.req.ResourceID
	return testutil.WithURLParams(f.request("POST", "/api/issues/"+id+"/delivery-evidence/completion-hint", body), "id", id)
}

func (f *deliveryPolicyFixture) hint(t *testing.T, body any, status int) deliveryCompletionHintResponse {
	t.Helper()
	var out deliveryCompletionHintResponse
	result := testutil.Call(t, f.h.GetDeliveryCompletionHint, f.hintRequest(body)).Want(status)
	result.JSON(&out)
	if result.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("completion hint is cacheable")
	}
	return out
}

func deliveryHintSnapshot(t *testing.T, f *deliveryPolicyFixture) string {
	t.Helper()
	var snapshot string
	dbfx.QueryRow(t, `SELECT jsonb_build_object(
 'issue', (SELECT to_jsonb(i) FROM issue i WHERE i.id=$1),
 'project', (SELECT to_jsonb(p) FROM project p WHERE p.id=$2),
 'statuses', (SELECT jsonb_agg(to_jsonb(s) ORDER BY key) FROM issue_status s WHERE workspace_id=(SELECT workspace_id FROM issue WHERE id=$1)),
 'policy', (SELECT jsonb_agg(to_jsonb(p) ORDER BY revision) FROM project_delivery_policy p WHERE project_id=$2),
 'evaluations', (SELECT jsonb_agg(to_jsonb(e) ORDER BY evaluation_id) FROM delivery_evidence_evaluation e WHERE issue_id=$1),
 'executions', (SELECT jsonb_agg(to_jsonb(e) ORDER BY execution_id) FROM agent_artifact_execution e WHERE resource_id=$1),
 'approvals', (SELECT jsonb_agg(to_jsonb(a) ORDER BY id) FROM agent_approval_request a WHERE issue_id=$1),
 'tasks', (SELECT jsonb_agg(to_jsonb(t) ORDER BY id) FROM agent_task_queue t WHERE issue_id=$1),
 'comments', (SELECT jsonb_agg(to_jsonb(c) ORDER BY id) FROM comment c WHERE issue_id=$1)
 )::text`, f.artifactFixture.req.ResourceID, f.project).Scan(&snapshot)
	return snapshot
}

func deliveryHintCustomStatus(t *testing.T, f *deliveryPolicyFixture, key, category string) string {
	t.Helper()
	return dbfx.Insert(t, "issue_status", testutil.Cols{"workspace_id": f.ws, "key": key, "name": key, "description": "Hint target", "category": category, "color": "#123456", "position": 1})
}

func TestDeliveryCompletionHintMatchingPreservesPersistedFacts(t *testing.T) {
	f := newDeliveryPolicyFixture(t, true)
	key := uuid.NewString()
	saved := f.evaluation(t, "POST", key, f.req, 200)
	counter := &artifactCounted{artifactExecutor: f.h.ArtifactPackager}
	f.h.ArtifactPackager = counter
	custom := "accepted_hint"
	deliveryHintCustomStatus(t, f, custom, "done")
	before := deliveryHintSnapshot(t, f)
	for _, target := range []string{"done", " DONE ", custom} {
		t.Run(target, func(t *testing.T) {
			out := f.hint(t, deliveryCompletionHintRequest{f.req.IssueRevision, target, &key}, 200)
			if out.Hint != "matching" || out.Reason != "observation_still_matches" || out.TargetStatus != strings.ToLower(strings.TrimSpace(target)) || out.TargetCategory != "done" || out.PolicyMode != "observe" || out.PolicyRevision != 1 || out.ProjectID == nil || *out.ProjectID != f.project || out.IssueID != f.artifactFixture.req.ResourceID || out.IssueRevision != f.req.IssueRevision {
				t.Fatalf("unbound completion hint: %+v", out)
			}
			if out.Evaluation == nil || out.Evaluation.EvaluationID != saved.EvaluationID || !reflect.DeepEqual(out.Evaluation.Observation, saved.Observation) || !out.CheckedAt.Equal(out.Evaluation.Applicability.CheckedAt) || out.CheckedAt.Before(saved.Observation.ObservedAt) || out.Enforced || out.UsableForCompletion || out.Evaluation.UsableForCompletion {
				t.Fatalf("hint changed evidence or granted authority: %+v", out)
			}
		})
	}
	if deliveryHintSnapshot(t, f) != before || counter.calls.Load() != 0 {
		t.Fatal("hint changed persisted business facts or invoked packaging")
	}
	// Recovery must not need the packager or any process-local evaluation cache.
	f.h = *testHandler
	f.h.ArtifactPackager = nil
	if out := f.hint(t, deliveryCompletionHintRequest{f.req.IssueRevision, "done", &key}, 200); out.Hint != "matching" {
		t.Fatal("restart lost durable evidence", out)
	}
}

func TestDeliveryCompletionHintApplicabilityBranchesDoNotReadKeys(t *testing.T) {
	for _, kind := range []string{"non_done", "custom_non_done", "off", "no_project", "missing_key"} {
		t.Run(kind, func(t *testing.T) {
			f := newDeliveryPolicyFixture(t, kind != "off")
			key := uuid.NewString()
			req := deliveryCompletionHintRequest{f.req.IssueRevision, "done", &key}
			wantHint, wantReason := "not_applicable", ""
			switch kind {
			case "non_done":
				req.TargetStatus, wantReason = "in_review", "target_not_done"
			case "custom_non_done":
				req.TargetStatus, wantReason = "accepted_later", "target_not_done"
				deliveryHintCustomStatus(t, f, req.TargetStatus, "in_review")
			case "off":
				wantReason = "policy_off"
			case "no_project":
				dbfx.Exec(t, `UPDATE issue SET project_id=NULL, revision=revision+1 WHERE id=$1`, f.artifactFixture.req.ResourceID)
				req.IssueRevision++
				wantReason = "no_project"
			case "missing_key":
				req.EvaluationKey = nil
				wantHint, wantReason = "attention", "evaluation_required"
			}
			before := deliveryHintSnapshot(t, f)
			out := f.hint(t, req, 200)
			if out.Hint != wantHint || out.Reason != wantReason || out.Evaluation != nil || out.Enforced || out.UsableForCompletion || out.CheckedAt.IsZero() {
				t.Fatalf("incorrect applicability branch: %+v", out)
			}
			if kind == "no_project" && (out.ProjectID != nil || out.PolicyRevision != 0 || out.PolicyMode != "off") {
				t.Fatal("detached issue inherited a policy", out)
			}
			if deliveryHintSnapshot(t, f) != before {
				t.Fatal("non-evaluating hint changed persisted facts")
			}
		})
	}
}

func TestDeliveryCompletionHintStrictRequestAndStatus(t *testing.T) {
	f := newDeliveryPolicyFixture(t, true)
	key := "abcdefab-cdef-4abc-8def-abcdefabcdef"
	valid := fmt.Sprintf(`{"expected_issue_revision":%d,"target_status":"done","evaluation_key":null}`, f.req.IssueRevision)
	cases := map[string]string{
		"missing_fields": `{}`, "null_object": `null`, "array": `[]`,
		"missing_key":         strings.Replace(valid, `,"evaluation_key":null`, "", 1),
		"missing_revision":    `{"target_status":"done","evaluation_key":null}`,
		"null_revision":       strings.Replace(valid, fmt.Sprintf(`"expected_issue_revision":%d`, f.req.IssueRevision), `"expected_issue_revision":null`, 1),
		"negative_revision":   `{"expected_issue_revision":-1,"target_status":"done","evaluation_key":null}`,
		"fractional_revision": `{"expected_issue_revision":0.5,"target_status":"done","evaluation_key":null}`,
		"overflow_revision":   `{"expected_issue_revision":9223372036854775808,"target_status":"done","evaluation_key":null}`,
		"string_revision":     `{"expected_issue_revision":"0","target_status":"done","evaluation_key":null}`,
		"null_status":         strings.Replace(valid, `"done"`, `null`, 1),
		"blank_status":        strings.Replace(valid, `"done"`, `"  "`, 1),
		"boolean_status":      strings.Replace(valid, `"done"`, `true`, 1),
		"nul_status":          strings.Replace(valid, `"done"`, `"done\u0000"`, 1),
		"blank_key":           strings.Replace(valid, `:null`, `:""`, 1),
		"noncanonical_key":    strings.Replace(valid, `:null`, `:"`+strings.ToUpper(key)+`"`, 1),
		"zero_key":            strings.Replace(valid, `:null`, `:"00000000-0000-0000-0000-000000000000"`, 1),
		"invalid_key":         strings.Replace(valid, `:null`, `:"bad"`, 1),
		"duplicate":           strings.TrimSuffix(valid, "}") + `,"target_status":"todo"}`,
		"case_alias":          strings.Replace(valid, `"target_status"`, `"Target_Status"`, 1),
		"skip":                strings.TrimSuffix(valid, "}") + `,"skip":true}`,
		"verdict":             strings.TrimSuffix(valid, "}") + `,"verdict":"pass"}`,
		"trailing":            valid + `{}`,
	}
	archived := deliveryHintCustomStatus(t, f, "retired_hint", "done")
	dbfx.Exec(t, `UPDATE issue_status SET archived_at=now() WHERE id=$1`, archived)
	before := deliveryHintSnapshot(t, f)
	for name, body := range cases {
		t.Run(name, func(t *testing.T) { f.hint(t, body, 400) })
	}
	t.Run("oversize", func(t *testing.T) { f.hint(t, valid+strings.Repeat(" ", 4096), 413) })
	t.Run("media_type", func(t *testing.T) {
		req := f.hintRequest(valid)
		req.Header.Set("Content-Type", "text/plain")
		testutil.Call(t, f.h.GetDeliveryCompletionHint, req).Want(415)
	})
	for _, status := range []string{"nonexistent_hint", "retired_hint"} {
		t.Run(status, func(t *testing.T) {
			result := testutil.Call(t, f.h.GetDeliveryCompletionHint, f.hintRequest(deliveryCompletionHintRequest{f.req.IssueRevision, status, nil})).Want(400)
			if !strings.Contains(result.Body.String(), "invalid_delivery_completion_status") {
				t.Fatal("invalid status not distinguished", result.Body.String())
			}
		})
	}
	if deliveryHintSnapshot(t, f) != before {
		t.Fatal("rejected hint changed persisted facts")
	}
}

func TestDeliveryCompletionHintKeyIsolationAndCurrentIdentity(t *testing.T) {
	f := newDeliveryPolicyFixture(t, true)
	key := uuid.NewString()
	f.evaluation(t, "POST", key, f.req, 200)
	before := deliveryHintSnapshot(t, f)
	req := deliveryCompletionHintRequest{f.req.IssueRevision, "done", &key}
	t.Run("missing", func(t *testing.T) {
		missing := uuid.NewString()
		f.hint(t, deliveryCompletionHintRequest{f.req.IssueRevision, "done", &missing}, 404)
	})
	t.Run("other_evaluator", func(t *testing.T) {
		original := f.actor
		f.actor = dbfx.User(t, "Other evaluator", "hint-"+uuid.NewString()+"@example.invalid")
		defer func() { f.actor = original }()
		dbfx.Member(t, f.ws, f.actor, "admin")
		f.hint(t, req, 404)
	})
	t.Run("other_issue", func(t *testing.T) {
		other := dbfx.Issue(t, "Other hint issue", testutil.Cols{"workspace_id": f.ws, "project_id": f.project})
		var rev int64
		dbfx.QueryRow(t, `SELECT revision FROM issue WHERE id=$1`, other).Scan(&rev)
		r := testutil.WithURLParams(f.request("POST", "/api/issues/"+other+"/delivery-evidence/completion-hint", deliveryCompletionHintRequest{rev, "done", &key}), "id", other)
		testutil.Call(t, f.h.GetDeliveryCompletionHint, r).Want(409)
	})
	for _, kind := range []string{"expired_credential", "forged_agent", "forged_task", "unverified", "machine"} {
		t.Run(kind, func(t *testing.T) {
			r := f.hintRequest(req)
			switch kind {
			case "expired_credential":
				r = r.WithContext(auth.WithAuthenticatedPrincipal(r.Context(), auth.AuthenticatedPrincipal{UserID: f.actor, Kind: auth.CredentialJWT, ExpiresAt: time.Now().Add(-time.Second)}))
			case "forged_agent":
				r.Header.Set("X-Agent-ID", f.artifactFixture.req.AgentID)
			case "forged_task":
				r.Header.Set("X-Task-ID", f.artifactFixture.req.TaskID)
			case "unverified":
				r = r.WithContext(context.Background())
			case "machine":
				r = r.WithContext(auth.WithAuthenticatedPrincipal(r.Context(), auth.AuthenticatedPrincipal{UserID: f.actor, Kind: auth.CredentialTask, ExpiresAt: time.Now().Add(time.Hour)}))
			}
			testutil.Call(t, f.h.GetDeliveryCompletionHint, r).Want(403)
		})
	}
	if deliveryHintSnapshot(t, f) != before {
		t.Fatal("unauthorized hint changed persisted facts")
	}
}

func TestDeliveryCompletionHintOldNegativeNeverUpgrades(t *testing.T) {
	for _, verdict := range []string{"unknown", "hold"} {
		t.Run(verdict, func(t *testing.T) {
			f := newDeliveryPolicyFixture(t, true)
			if verdict == "unknown" {
				dbfx.Exec(t, `DELETE FROM issue_pull_request WHERE issue_id=$1`, f.artifactFixture.req.ResourceID)
			} else {
				dbfx.Exec(t, `UPDATE github_pull_request SET head_sha=$1 WHERE id=$2`, strings.Repeat("b", 40), f.pr)
			}
			key := uuid.NewString()
			saved := f.evaluation(t, "POST", key, f.req, 200)
			if saved.Observation.Verdict != verdict {
				t.Fatal("negative fixture did not produce its declared observation", saved)
			}
			if verdict == "unknown" {
				dbfx.InsertNoID(t, "issue_pull_request", testutil.Cols{"issue_id": f.artifactFixture.req.ResourceID, "pull_request_id": f.pr}, "issue_id=$1 AND pull_request_id=$2", f.artifactFixture.req.ResourceID, f.pr)
			} else {
				dbfx.Exec(t, `UPDATE github_pull_request SET head_sha=$1 WHERE id=$2`, f.req.SourceRevision, f.pr)
			}
			before := deliveryHintSnapshot(t, f)
			out := f.hint(t, deliveryCompletionHintRequest{f.req.IssueRevision, "done", &key}, 200)
			if out.Hint != "attention" || out.Evaluation == nil || out.Evaluation.Applicability.Status == "current" || !reflect.DeepEqual(out.Evaluation.Observation, saved.Observation) || deliveryHintSnapshot(t, f) != before {
				t.Fatal("hint promoted or replaced a retained negative observation", out)
			}
		})
	}
}

func TestDeliveryCompletionHintNonEvaluatingBranchesRequireCurrentAuthority(t *testing.T) {
	for _, branch := range []string{"missing_key", "policy_off", "non_done", "no_project"} {
		for _, revoke := range []string{"credential", "membership", "user"} {
			t.Run(branch+"/"+revoke, func(t *testing.T) {
				f := newDeliveryPolicyFixture(t, branch != "policy_off")
				body := deliveryCompletionHintRequest{f.req.IssueRevision, "done", nil}
				if branch == "non_done" {
					body.TargetStatus = "in_review"
				}
				if branch == "no_project" {
					dbfx.Exec(t, `UPDATE issue SET project_id=NULL, revision=revision+1 WHERE id=$1`, f.artifactFixture.req.ResourceID)
					body.IssueRevision++
				}
				// These paths would succeed without recovering any retained evaluation.
				f.hint(t, body, 200)
				r := f.hintRequest(body)
				switch revoke {
				case "credential":
					r = r.WithContext(auth.WithAuthenticatedPrincipal(r.Context(), auth.AuthenticatedPrincipal{UserID: f.actor, Kind: auth.CredentialJWT, ExpiresAt: time.Now().Add(-time.Second)}))
				case "membership":
					dbfx.Exec(t, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, f.ws, f.actor)
				case "user":
					dbfx.Exec(t, `UPDATE "user" SET disabled_at=now() WHERE id=$1`, f.actor)
				}
				before := deliveryHintSnapshot(t, f)
				result := testutil.Call(t, f.h.GetDeliveryCompletionHint, r).Want(403)
				var out deliveryCompletionHintResponse
				result.JSON(&out)
				if out.Hint != "" || out.Evaluation != nil || deliveryHintSnapshot(t, f) != before {
					t.Fatal("non-evaluating branch bypassed current authority", out)
				}
			})
		}
	}
}

func TestDeliveryCompletionHintLiveInvalidationAndRefusal(t *testing.T) {
	for _, change := range []string{"policy_aba", "issue", "project", "head", "missing_head", "receipt", "missing_execution", "revoked_producer", "stale_request"} {
		t.Run(change, func(t *testing.T) {
			f := newDeliveryPolicyFixture(t, true)
			producer := f.actor
			if change == "revoked_producer" {
				f.actor = dbfx.User(t, "Hint reader", "hint-reader-"+uuid.NewString()+"@example.invalid")
				dbfx.Member(t, f.ws, f.actor, "admin")
			}
			key := uuid.NewString()
			saved := f.evaluation(t, "POST", key, f.req, 200)
			req := deliveryCompletionHintRequest{f.req.IssueRevision, "done", &key}
			status, reason := 200, ""
			switch change {
			case "policy_aba":
				f.policy(t, "PUT", deliveryPolicyRequest{1, "off", deliveryEvidencePolicy, deliveryPolicyStage, "off"}, 200)
				f.policy(t, "PUT", deliveryPolicyRequest{2, "observe", deliveryEvidencePolicy, deliveryPolicyStage, "observe again"}, 200)
				reason = "policy_changed"
			case "issue", "stale_request":
				dbfx.Exec(t, `UPDATE issue SET revision=revision+1 WHERE id=$1`, f.artifactFixture.req.ResourceID)
				if change == "issue" {
					req.IssueRevision++
					reason = "resource_revision_changed"
				} else {
					status = 409
				}
			case "project":
				dbfx.Exec(t, `UPDATE project SET revision=revision+1 WHERE id=$1`, f.project)
				reason = "resource_revision_changed"
			case "head":
				dbfx.Exec(t, `UPDATE github_pull_request SET head_sha=$1 WHERE id=$2`, strings.Repeat("b", 40), f.pr)
				reason = "source_revision_mismatch"
			case "missing_head":
				dbfx.Exec(t, `DELETE FROM issue_pull_request WHERE issue_id=$1`, f.artifactFixture.req.ResourceID)
				reason = "source_head_missing"
			case "receipt":
				dbfx.Exec(t, `UPDATE agent_artifact_execution SET receipt_json=receipt_json || '{"result":"failed"}'::jsonb WHERE approval_id=$1`, f.req.ApprovalID)
				reason = "receipt_invalid"
			case "missing_execution":
				dbfx.Exec(t, `DELETE FROM agent_artifact_execution WHERE approval_id=$1`, f.req.ApprovalID)
				status = 409
			case "revoked_producer":
				dbfx.Exec(t, `UPDATE issue SET revision=revision+1 WHERE id=$1`, f.artifactFixture.req.ResourceID)
				req.IssueRevision++
				dbfx.Exec(t, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, f.ws, producer)
				status = 404
			}
			before := deliveryHintSnapshot(t, f)
			out := f.hint(t, req, status)
			if status == 200 && (out.Hint != "attention" || out.Reason != reason || out.Evaluation == nil || !reflect.DeepEqual(out.Evaluation.Observation, saved.Observation)) {
				t.Fatal("changed evidence appeared matching", out)
			}
			if status != 200 && (out.Hint != "" || out.Evaluation != nil) {
				t.Fatal("refusal leaked historical evidence", out)
			}
			if deliveryHintSnapshot(t, f) != before {
				t.Fatal("invalidation rewrote evidence")
			}
		})
	}
}

func TestDeliveryCompletionHintCommitFailureHasNoSuccessOrWrites(t *testing.T) {
	for _, after := range []bool{false, true} {
		t.Run(fmt.Sprint(after), func(t *testing.T) {
			f := newDeliveryPolicyFixture(t, true)
			key := uuid.NewString()
			f.evaluation(t, "POST", key, f.req, 200)
			before := deliveryHintSnapshot(t, f)
			original := f.h.TxStarter
			f.h.TxStarter = &artifactFaultStarter{txStarter: original, failAt: 1, after: after}
			out := f.hint(t, deliveryCompletionHintRequest{f.req.IssueRevision, "done", &key}, 503)
			if out.Hint != "" || out.Evaluation != nil || deliveryHintSnapshot(t, f) != before {
				t.Fatal("failed read commit exposed success or changed facts", out)
			}
			f.h.TxStarter = original
			if got := f.hint(t, deliveryCompletionHintRequest{f.req.IssueRevision, "done", &key}, 200); got.Hint != "matching" || deliveryHintSnapshot(t, f) != before {
				t.Fatal("explicit recovery changed retained evidence", got)
			}
		})
	}
}

func TestDeliveryCompletionHintConcurrentWriterFirst(t *testing.T) {
	for _, target := range []string{"policy", "issue", "status_catalog"} {
		t.Run(target, func(t *testing.T) {
			f := newDeliveryPolicyFixture(t, true)
			key := uuid.NewString()
			f.evaluation(t, "POST", key, f.req, 200)
			req := deliveryCompletionHintRequest{f.req.IssueRevision, "done", &key}
			ctx := context.Background()
			tx, err := testPool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			stmt, id := `UPDATE project SET revision=revision+1 WHERE id=$1`, f.project
			if target == "issue" {
				stmt, id = `UPDATE issue SET revision=revision+1 WHERE id=$1`, f.artifactFixture.req.ResourceID
			}
			if target == "status_catalog" {
				req.TargetStatus = "accepted_busy"
				deliveryHintCustomStatus(t, f, req.TargetStatus, "done")
				stmt, id = `SELECT pg_advisory_xact_lock(hashtextextended($1::uuid::text || ':issue_status', 0))`, f.ws
			}
			if _, err = tx.Exec(ctx, stmt, id); err != nil {
				t.Fatal(err)
			}
			f.hint(t, req, 409)
			if err = tx.Rollback(ctx); err != nil {
				t.Fatal(err)
			}
			if out := f.hint(t, req, 200); out.Hint != "matching" {
				t.Fatal("released contention did not restore explicit read", out)
			}
		})
	}
}

func TestDeliveryCompletionHintFencesPolicyAndStatusUntilCommit(t *testing.T) {
	for _, target := range []string{"policy", "status_catalog", "issue", "member"} {
		t.Run(target, func(t *testing.T) {
			f := newDeliveryPolicyFixture(t, true)
			key := uuid.NewString()
			f.evaluation(t, "POST", key, f.req, 200)
			custom := "accepted_fenced"
			deliveryHintCustomStatus(t, f, custom, "done")
			req := f.hintRequest(deliveryCompletionHintRequest{f.req.IssueRevision, custom, &key})
			original := f.h.TxStarter
			entered, release := make(chan struct{}), make(chan struct{})
			releaseRead := sync.OnceFunc(func() { close(release) })
			defer releaseRead()
			f.h.TxStarter = &artifactBarrierStarter{txStarter: original, entered: entered, release: release}
			done := make(chan *testutil.Response, 1)
			go func() { done <- testutil.Call(t, f.h.GetDeliveryCompletionHint, req) }()
			select {
			case <-entered:
			case <-time.After(5 * time.Second):
				t.Fatal("hint did not reach commit")
			}
			// Always release the read before reporting a writer failure.
			err := func() error {
				ctx := context.Background()
				tx, err := testPool.Begin(ctx)
				if err != nil {
					return err
				}
				defer tx.Rollback(ctx)
				if _, err = tx.Exec(ctx, `SET LOCAL lock_timeout='100ms'`); err != nil {
					return err
				}
				stmt, id := `UPDATE project SET revision=revision+1 WHERE id=$1`, f.project
				switch target {
				case "status_catalog":
					stmt, id = `SELECT pg_advisory_xact_lock(hashtextextended($1::uuid::text || ':issue_status', 0))`, f.ws
				case "issue":
					stmt, id = `UPDATE issue SET revision=revision+1 WHERE id=$1`, f.artifactFixture.req.ResourceID
				case "member":
					stmt, id = `UPDATE member SET role='member' WHERE workspace_id=$1`, f.ws
				}
				_, err = tx.Exec(ctx, stmt, id)
				return err
			}()
			releaseRead()
			result := <-done
			var pgerr *pgconn.PgError
			if !errors.As(err, &pgerr) || pgerr.Code != "55P03" {
				t.Fatalf("concurrent %s bypassed the read fence: %v", target, err)
			}
			var out deliveryCompletionHintResponse
			result.Want(200).JSON(&out)
			if out.Hint != "matching" || out.Evaluation == nil || !out.CheckedAt.Equal(out.Evaluation.Applicability.CheckedAt) {
				t.Fatal("fenced hint lost matching evidence", out)
			}
		})
	}
}

func TestDeliveryCompletionHintRejectsUnreadySchemaEvenWhenOff(t *testing.T) {
	for _, shape := range []string{"missing", "nonunique"} {
		t.Run(shape, func(t *testing.T) {
			f := newDeliveryPolicyFixture(t, false)
			before := deliveryHintSnapshot(t, f)
			schema := "delivery_hint_" + strings.ReplaceAll(uuid.NewString(), "-", "")
			dbfx.Exec(t, `CREATE SCHEMA `+schema)
			dbfx.Exec(t, `CREATE TABLE `+schema+`.project_delivery_policy (LIKE public.project_delivery_policy INCLUDING DEFAULTS INCLUDING CONSTRAINTS)`)
			dbfx.Exec(t, `CREATE TABLE `+schema+`.delivery_evidence_evaluation (LIKE public.delivery_evidence_evaluation INCLUDING DEFAULTS INCLUDING CONSTRAINTS)`)
			t.Cleanup(func() {
				dbfx.Exec(t, `DROP TABLE `+schema+`.delivery_evidence_evaluation, `+schema+`.project_delivery_policy`)
				dbfx.Exec(t, `DROP SCHEMA `+schema)
			})
			dbfx.Exec(t, `CREATE UNIQUE INDEX project_delivery_policy_revision_idx ON `+schema+`.project_delivery_policy(workspace_id,project_id,revision)`)
			dbfx.Exec(t, `CREATE UNIQUE INDEX delivery_evaluation_id_idx ON `+schema+`.delivery_evidence_evaluation(evaluation_id)`)
			if shape == "nonunique" {
				dbfx.Exec(t, `CREATE INDEX delivery_evaluation_key_idx ON `+schema+`.delivery_evidence_evaluation(workspace_id,evaluator_user_id,request_key)`)
			}
			cfg := testPool.Config()
			cfg.MaxConns, cfg.MinConns = 2, 0
			cfg.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
			pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
			if err != nil {
				t.Fatal(err)
			}
			defer pool.Close()
			f.h.TxStarter = pool
			out := f.hint(t, deliveryCompletionHintRequest{f.req.IssueRevision, "done", nil}, 503)
			if out.Hint != "" || out.Evaluation != nil || deliveryHintSnapshot(t, f) != before {
				t.Fatal("partial schema produced advisory success or changed public history", out)
			}
			if dbfx.Count(t, `SELECT count(*) FROM `+schema+`.project_delivery_policy`)+dbfx.Count(t, `SELECT count(*) FROM `+schema+`.delivery_evidence_evaluation`) != 0 {
				t.Fatal("partial schema gained persistent rows")
			}
		})
	}
}

func TestDeliveryCompletionHintErrorBodyDoesNotContainEvaluation(t *testing.T) {
	f := newDeliveryPolicyFixture(t, true)
	key := uuid.NewString()
	f.evaluation(t, "POST", key, f.req, 200)
	f.h.TxStarter = nil
	res := testutil.Call(t, f.h.GetDeliveryCompletionHint, f.hintRequest(deliveryCompletionHintRequest{f.req.IssueRevision, "done", &key})).Want(503)
	var body map[string]json.RawMessage
	res.JSON(&body)
	if len(body) != 1 || body["error"] == nil || body["evaluation"] != nil || body["hint"] != nil {
		t.Fatal("unavailable transaction exposed a hint", res.Body.String())
	}
}
