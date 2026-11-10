package service

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const DeliveryPolicyStage = "done"

type DeliveryPolicy struct {
	CompletionContractVersion string     `json:"completion_contract_version"`
	SkipSupported             bool       `json:"skip_supported"`
	WaivableChecks            []string   `json:"waivable_checks"`
	ProjectID                 string     `json:"project_id"`
	Revision                  int64      `json:"revision"`
	Mode                      string     `json:"mode"`
	PolicyVersion             string     `json:"policy_version"`
	Stage                     string     `json:"stage"`
	Reason                    string     `json:"reason"`
	CreatedBy                 *string    `json:"created_by"`
	CreatedAt                 *time.Time `json:"created_at"`
}

// CheckDeliveryPolicySchema retains the exact policy/evaluation index guard.
// All reads use the supplied transaction, never a separately supplied query pool.
func CheckDeliveryPolicySchema(ctx context.Context, tx pgx.Tx) error {
	q := db.New(tx)
	ready, err := q.DeliveryPolicySchemaReady(ctx)
	if err != nil {
		return err
	}
	if !ready.Valid || !ready.Bool {
		return ArtifactError{503, "delivery_policy_schema_unavailable"}
	}
	return nil
}

// CurrentDeliveryPolicyInTx reads the policy in the supplied transaction after
// the caller locks the project. An absent policy has virtual revision 0/off.
func CurrentDeliveryPolicyInTx(ctx context.Context, tx pgx.Tx, ws, project pgtype.UUID) (DeliveryPolicy, error) {
	q := db.New(tx)
	out := DeliveryPolicy{CompletionContractVersion: DeliveryCompletionContract, WaivableChecks: []string{}, ProjectID: util.UUIDToString(project), Mode: "off", PolicyVersion: DeliveryEvidencePolicy, Stage: DeliveryPolicyStage}
	row, err := q.GetDeliveryPolicy(ctx, db.GetDeliveryPolicyParams{WorkspaceID: ws, ProjectID: project})
	if errors.Is(err, pgx.ErrNoRows) {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	out.Revision = row.Revision
	out.Mode = row.Mode
	out.PolicyVersion = row.PolicyVersion
	out.Stage = row.Stage
	out.Reason = row.Reason
	actor, at := util.UUIDToString(row.CreatedBy), row.CreatedAt.Time.UTC()
	out.CreatedBy = &actor
	out.CreatedAt = &at
	return out, nil
}

// CheckDeliveryEnforcementActivationSchema composes the actual writer dependencies
// before a new enforce decision. Off/observe rollback does not depend on these
// new tables: it keeps the original policy/current-authority contract.
func CheckDeliveryEnforcementActivationSchema(ctx context.Context, tx pgx.Tx) error {
	if err := PrepareArtifactTransaction(ctx, tx); err != nil {
		return err
	}
	if err := CheckDeliveryPolicySchema(ctx, tx); err != nil {
		return err
	}
	if err := CheckDeliveryCompletionSchema(ctx, tx); err != nil {
		return err
	}
	var projectReady bool
	if err := tx.QueryRow(ctx, deliveryProjectChangeReadinessSQL).Scan(&projectReady); err != nil {
		return err
	}
	if !projectReady {
		return ArtifactError{Status: 503, Code: "delivery_enforcement_schema_unavailable"}
	}
	// These are exactly the 792/793 trust columns and positive CHECKs. Match
	// type/nullability and actual validated expression, not a constraint name or
	// migration-ledger entry that could survive incomplete/conditional DDL.
	var trustReady bool
	if err := tx.QueryRow(ctx, `SELECT NOT EXISTS (
  SELECT 1 FROM (VALUES ('github_installation'),('vcs_connection')) AS target(table_name)
  WHERE NOT EXISTS (
   SELECT 1 FROM pg_catalog.pg_attribute a
   WHERE a.attrelid=to_regclass(target.table_name) AND a.attname='trust_revision'
    AND a.atttypid='pg_catalog.int8'::regtype AND a.attnotnull AND NOT a.attisdropped
  ) OR NOT EXISTS (
   SELECT 1 FROM pg_catalog.pg_constraint c
   WHERE c.conrelid=to_regclass(target.table_name) AND c.contype='c' AND c.convalidated
    AND pg_get_constraintdef(c.oid)='CHECK ((trust_revision > 0))'
  )
 )`).Scan(&trustReady); err != nil {
		return err
	}
	if !trustReady {
		return ArtifactError{Status: 503, Code: "delivery_enforcement_schema_unavailable"}
	}
	return nil
}
