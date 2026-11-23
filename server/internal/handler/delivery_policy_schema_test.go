package handler

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestDeliveryPolicyRuntimeSchemaRefusesPartialAndWrongIndexes(t *testing.T) {
	for _, shape := range []string{"missing", "nonunique", "reordered", "partial", "included"} {
		t.Run(shape, func(t *testing.T) {
			f := newDeliveryPolicyFixture(t, true)
			key := uuid.NewString()
			f.evaluation(t, "POST", key, f.req, 200)
			schema := "delivery_policy_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
			dbfx.Exec(t, `CREATE SCHEMA `+schema)
			dbfx.Exec(t, `CREATE TABLE `+schema+`.project_delivery_policy (LIKE public.project_delivery_policy INCLUDING DEFAULTS INCLUDING CONSTRAINTS)`)
			dbfx.Exec(t, `CREATE TABLE `+schema+`.delivery_evidence_evaluation (LIKE public.delivery_evidence_evaluation INCLUDING DEFAULTS INCLUDING CONSTRAINTS)`)
			t.Cleanup(func() {
				dbfx.Exec(t, `DROP TABLE `+schema+`.delivery_evidence_evaluation, `+schema+`.project_delivery_policy`)
				dbfx.Exec(t, `DROP SCHEMA `+schema)
			})
			dbfx.Exec(t, `CREATE UNIQUE INDEX project_delivery_policy_revision_idx ON `+schema+`.project_delivery_policy(workspace_id,project_id,revision)`)
			dbfx.Exec(t, `CREATE UNIQUE INDEX delivery_evaluation_id_idx ON `+schema+`.delivery_evidence_evaluation(evaluation_id)`)
			switch shape {
			case "nonunique":
				dbfx.Exec(t, `CREATE INDEX delivery_evaluation_key_idx ON `+schema+`.delivery_evidence_evaluation(workspace_id,evaluator_user_id,request_key)`)
			case "reordered":
				dbfx.Exec(t, `CREATE UNIQUE INDEX delivery_evaluation_key_idx ON `+schema+`.delivery_evidence_evaluation(evaluator_user_id,workspace_id,request_key)`)
			case "partial":
				dbfx.Exec(t, `CREATE UNIQUE INDEX delivery_evaluation_key_idx ON `+schema+`.delivery_evidence_evaluation(workspace_id,evaluator_user_id,request_key) WHERE policy_revision>0`)
			case "included":
				dbfx.Exec(t, `CREATE UNIQUE INDEX delivery_evaluation_key_idx ON `+schema+`.delivery_evidence_evaluation(workspace_id,evaluator_user_id,request_key) INCLUDE(issue_id)`)
			}
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
			f.policy(t, "GET", nil, 503)
			f.policy(t, "PUT", deliveryPolicyRequest{1, "off", deliveryEvidencePolicy, deliveryPolicyStage, "partial"}, 503)
			f.evaluation(t, "GET", key, nil, 503)
			f.evaluation(t, "POST", uuid.NewString(), f.req, 503)
			if dbfx.Count(t, `SELECT count(*) FROM `+schema+`.project_delivery_policy`)+dbfx.Count(t, `SELECT count(*) FROM `+schema+`.delivery_evidence_evaluation`) != 0 {
				t.Fatal("partial schema admitted writes")
			}
			if f.evaluationCount(t) != 1 {
				t.Fatal("partial schema changed public evidence")
			}
		})
	}
}
