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
	ProjectID     string     `json:"project_id"`
	Revision      int64      `json:"revision"`
	Mode          string     `json:"mode"`
	PolicyVersion string     `json:"policy_version"`
	Stage         string     `json:"stage"`
	Reason        string     `json:"reason"`
	CreatedBy     *string    `json:"created_by"`
	CreatedAt     *time.Time `json:"created_at"`
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
	out := DeliveryPolicy{ProjectID: util.UUIDToString(project), Mode: "off", PolicyVersion: DeliveryEvidencePolicy, Stage: DeliveryPolicyStage}
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
