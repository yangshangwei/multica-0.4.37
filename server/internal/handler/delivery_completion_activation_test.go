package handler

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestDeliveryCompletionActivationExplicitCASAndCapabilities(t *testing.T) {
	f := newDeliveryPolicyFixture(t, false)
	var initial map[string]any
	req := testutil.WithURLParams(f.artifactFixture.request("GET", "/api/projects/"+f.project+"/delivery-policy", nil), "id", f.project)
	testutil.Call(t, f.h.GetDeliveryPolicy, req).Want(200).JSON(&initial)
	if initial["mode"] != "off" || initial["completion_contract_version"] != service.DeliveryCompletionContract || initial["skip_supported"] != false {
		t.Fatalf("policy did not report non-authorizing capabilities: %+v", initial)
	}
	checks, ok := initial["waivable_checks"].([]any)
	if !ok || len(checks) != 0 {
		t.Fatal("fixed hard checks exposed a skip capability")
	}
	policy := f.policy(t, "PUT", deliveryPolicyRequest{0, "enforce", deliveryEvidencePolicy, deliveryPolicyStage, "Explicitly enable all completion writers"}, 200)
	if policy.Mode != "enforce" || policy.Revision != 1 {
		t.Fatal(policy)
	}
	f.policy(t, "PUT", deliveryPolicyRequest{0, "off", deliveryEvidencePolicy, deliveryPolicyStage, "Stale downgrade"}, 409)
	repeated := f.policy(t, "PUT", deliveryPolicyRequest{1, "enforce", deliveryEvidencePolicy, deliveryPolicyStage, "New same-mode decision"}, 200)
	if repeated.Revision != 2 {
		t.Fatal("same mode did not append independent policy revision")
	}
	f.policy(t, "PUT", deliveryPolicyRequest{2, "off", deliveryEvidencePolicy, deliveryPolicyStage, "Retained operational rollback"}, 200)
	if dbfx.Count(t, `SELECT count(*) FROM project_delivery_policy WHERE project_id=$1`, f.project) != 3 {
		t.Fatal("rollback removed earlier policy decisions")
	}
}

func TestDeliveryCompletionActivationEnforceEvaluationAndHintRemainObservations(t *testing.T) {
	f := newDeliveryPolicyFixture(t, true)
	oldKey := uuid.NewString()
	old := f.evaluation(t, "POST", oldKey, f.req, 200)
	oldBytes := f.stored(t, oldKey)
	// Until policy activation is opened this is an explicitly internal fixture;
	// it tests observation compatibility, not a public rollout claim.
	dbfx.InsertNoID(t, "project_delivery_policy", testutil.Cols{"workspace_id": f.ws, "project_id": f.project, "revision": 2, "policy_version": deliveryEvidencePolicy, "mode": "enforce", "stage": "done", "reason": "internal current-mode compatibility", "created_by": f.actor}, "workspace_id=$1 AND project_id=$2 AND revision=2", f.ws, f.project)
	stale := f.evaluation(t, "GET", oldKey, nil, 200)
	if stale.Applicability.Status != "stale" || stale.Applicability.Reason != "policy_changed" || stale.UsableForCompletion || f.stored(t, oldKey) != oldBytes {
		t.Fatal("old observe result upgraded or changed after enforce")
	}
	f.req.PolicyRevision = 2
	newKey := uuid.NewString()
	fresh := f.evaluation(t, "POST", newKey, f.req, 200)
	if fresh.Observation.Verdict != "pass" || fresh.Applicability.Status != "current" || fresh.UsableForCompletion {
		t.Fatal("fresh enforce observation misreported permission")
	}
	var hint deliveryCompletionHintResponse
	req := testutil.WithURLParams(f.artifactFixture.request("POST", "/api/issues/"+f.artifactFixture.req.ResourceID+"/delivery-evidence/completion-hint", deliveryCompletionHintRequest{f.req.IssueRevision, "done", &newKey}), "id", f.artifactFixture.req.ResourceID)
	testutil.Call(t, f.h.GetDeliveryCompletionHint, req).Want(200).JSON(&hint)
	if hint.Hint != "matching" || hint.PolicyMode != "enforce" || hint.Enforced || hint.UsableForCompletion {
		t.Fatal("enforce hint became completion permission")
	}
	if old.Observation.ValidUntil == nil || stale.Observation.ValidUntil == nil || !old.Observation.ValidUntil.Equal(*stale.Observation.ValidUntil) {
		t.Fatal("policy transition renewed original evidence")
	}
	if dbfx.Count(t, `SELECT count(*) FROM delivery_completion_authorization WHERE issue_id=$1`, f.artifactFixture.req.ResourceID) != 0 {
		t.Fatal("evaluation or hint signed authorization")
	}
}

func TestDeliveryCompletionActivationRejectsPartialSchemaAllowsOff(t *testing.T) {
	for _, missing := range []string{"authorization_index", "project_change_index", "github_trust", "vcs_trust", "github_trust_weakened"} {
		t.Run(missing, func(t *testing.T) {
			f := newDeliveryPolicyFixture(t, true)
			schema := "completion_activation_" + strings.ReplaceAll(uuid.NewString(), "-", "")
			dbfx.Exec(t, "CREATE SCHEMA "+schema)
			table := "delivery_completion_authorization"
			switch missing {
			case "project_change_index":
				table = "delivery_project_change"
			case "github_trust", "github_trust_weakened":
				table = "github_installation"
			case "vcs_trust":
				table = "vcs_connection"
			}
			dbfx.Exec(t, "CREATE TABLE "+schema+"."+table+" (LIKE public."+table+" INCLUDING DEFAULTS INCLUDING CONSTRAINTS)")
			if strings.HasSuffix(missing, "trust") {
				dbfx.Exec(t, "ALTER TABLE "+schema+"."+table+" DROP COLUMN trust_revision")
			}
			if missing == "github_trust_weakened" {
				dbfx.Exec(t, "ALTER TABLE "+schema+"."+table+" DROP CONSTRAINT github_installation_trust_revision_positive")
				dbfx.Exec(t, "ALTER TABLE "+schema+"."+table+" ADD CONSTRAINT github_installation_trust_revision_positive CHECK(trust_revision>=0)")
			}
			t.Cleanup(func() { dbfx.Exec(t, "DROP TABLE "+schema+"."+table); dbfx.Exec(t, "DROP SCHEMA "+schema) })
			cfg := testPool.Config()
			cfg.MaxConns = 2
			cfg.MinConns = 0
			cfg.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
			pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
			if err != nil {
				t.Fatal(err)
			}
			defer pool.Close()
			f.h.TxStarter = pool
			f.policy(t, "PUT", deliveryPolicyRequest{1, "enforce", deliveryEvidencePolicy, deliveryPolicyStage, "Must not enable incomplete installation"}, 503)
			if dbfx.Count(t, `SELECT count(*) FROM project_delivery_policy WHERE project_id=$1`, f.project) != 1 {
				t.Fatal("incomplete schema appended enforce")
			}
			f.policy(t, "PUT", deliveryPolicyRequest{1, "off", deliveryEvidencePolicy, deliveryPolicyStage, "Turn off with retained old policy schema"}, 200)
		})
	}
}

func TestDeliveryCompletionActivationExpiryAndOffRetention(t *testing.T) {
	f := newCompletionFixture(t)
	key := uuid.NewString()
	saved := f.evaluation(t, "POST", key, f.req, 200)
	original := f.stored(t, key)
	var deadline time.Time
	dbfx.QueryRow(t, `SELECT clock_timestamp()+interval '1 second'`).Scan(&deadline)
	f.request.ExpiresAt = deadline.UTC().Truncate(time.Microsecond)
	grant := f.create(t, uuid.NewString(), 201)
	// Wait on the actual database clock for this newly signed short-lived grant.
	// The producer's immutable 15-minute specification and observation are untouched.
	dbfx.QueryRow(t, `SELECT pg_sleep(GREATEST(0,EXTRACT(EPOCH FROM ($1::timestamptz-clock_timestamp())))+0.02)`, deadline).Scan(new(any))
	body := map[string]any{"status": "done", "expected_revision": grant.IssueRevision, "completion": service.DeliveryCompletionBinding{AuthorizationID: grant.AuthorizationID, OperationID: grant.OperationID, ExpectedIssueRevision: grant.IssueRevision}}
	request := testutil.WithURLParams(f.artifactFixture.request("PUT", "/api/issues/"+f.request.Target.IssueID, body), "id", f.request.Target.IssueID)
	testutil.Call(t, f.h.UpdateIssue, request).Want(409)
	if dbfx.Count(t, `SELECT count(*) FROM delivery_completion_terminal WHERE authorization_id=$1`, grant.AuthorizationID) != 0 {
		t.Fatal("expired acceptance was consumed")
	}
	recovered := f.evaluation(t, "GET", key, nil, 200)
	if recovered.Observation.Verdict != saved.Observation.Verdict || recovered.UsableForCompletion || f.stored(t, key) != original {
		t.Fatal("grant expiry rewrote original private observation")
	}
	view := f.get(t, grant.AuthorizationID, 200)
	if view.State != "issued" || view.Applicability != "expired" || !view.ExpiresAt.Equal(grant.ExpiresAt) {
		t.Fatal("expired grant recovery renewed its window")
	}
	f.policy(t, "PUT", deliveryPolicyRequest{1, "off", deliveryEvidencePolicy, deliveryPolicyStage, "preserve observation and expired acceptance on disable"}, 200)
	recovered = f.evaluation(t, "GET", key, nil, 200)
	if recovered.Applicability.Status != "stale" || recovered.Applicability.Reason != "policy_changed" || recovered.UsableForCompletion || f.stored(t, key) != original {
		t.Fatal("policy rollback discarded or renewed history")
	}
}

func TestDeliveryCompletionActivationDependencyReadinessBeforePublicEnable(t *testing.T) {
	f := newDeliveryPolicyFixture(t, false)
	tx, err := testPool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if err = service.CheckDeliveryEnforcementActivationSchema(context.Background(), tx); err != nil {
		t.Fatalf("complete migrated installation was not activation-ready: %v", err)
	}
	if dbfx.Count(t, `SELECT count(*) FROM project_delivery_policy WHERE project_id=$1`, f.project) != 0 {
		t.Fatal("schema readiness silently enabled policy")
	}
}
