package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// StagePolicy is the current project stage policy. An absent policy is virtual
// revision 0, disabled. It is independent of the delivery (M1) policy, the task
// AI policy and any Git automation switch.
type StagePolicy struct {
	Contract       string     `json:"contract"`
	ProjectID      string     `json:"project_id"`
	PolicyRevision string     `json:"policy_revision"`
	Enabled        bool       `json:"enabled"`
	Reason         string     `json:"reason"`
	CreatedBy      *string    `json:"created_by"`
	CreatedAt      *time.Time `json:"created_at"`
	// ActivationAvailable is set by the transport that decides whether enabling
	// may currently be requested; the service itself never infers it.
	ActivationAvailable bool `json:"activation_available"`
}

type StagePolicyRevisionParams struct {
	WorkspaceID      pgtype.UUID
	ProjectID        pgtype.UUID
	RequestID        string
	ExpectedRevision int64
	Enabled          bool
	Reason           string
	Credential       ArtifactCredentialSnapshot
}

// StagePolicyReceipt is the minimal confirmation of one committed revision.
type StagePolicyReceipt struct {
	Code              string                         `json:"code"`
	StageConfirmation CollaborationStageConfirmation `json:"stage_confirmation"`
}

func stagePolicyView(project pgtype.UUID, row db.ProjectStagePolicy, found bool) StagePolicy {
	out := StagePolicy{Contract: collaboration.StageContract, ProjectID: util.UUIDToString(project), PolicyRevision: "0"}
	if !found {
		return out
	}
	actor, at := util.UUIDToString(row.CreatedBy), row.CreatedAt.Time.UTC()
	out.PolicyRevision, out.Enabled, out.Reason, out.CreatedBy, out.CreatedAt = stageVersion(row.Revision), row.Enabled, row.Reason, &actor, &at
	return out
}

func stagePolicyReceipt(row db.ProjectStagePolicy) StagePolicyReceipt {
	enabled := row.Enabled
	return StagePolicyReceipt{Code: "stage_policy_revised", StageConfirmation: CollaborationStageConfirmation{Action: "revise_policy",
		ProjectID: util.UUIDToString(row.ProjectID), PolicyRevision: stageVersion(row.Revision), Enabled: &enabled}}
}

// stagePolicyError maps the shared authority primitives onto the stage policy
// contract. Lock contention is a clean refusal: nothing was written.
func stagePolicyError(err error) error {
	var failure ArtifactError
	var pgerr *pgconn.PgError
	var domain *CollaborationError
	switch {
	case err == nil:
		return nil
	case errors.As(err, &domain):
		return err
	case errors.As(err, &failure):
		if failure.Status == http.StatusConflict {
			return collaborationError(http.StatusConflict, "stage_policy_busy")
		}
		return collaborationError(http.StatusForbidden, "stage_policy_denied")
	case errors.Is(err, auth.ErrPasswordSession), errors.Is(err, pgx.ErrNoRows):
		return collaborationError(http.StatusForbidden, "stage_policy_denied")
	case errors.As(err, &pgerr) && (pgerr.Code == "55P03" || pgerr.Code == "40P01" || pgerr.Code == "40001"):
		return collaborationError(http.StatusConflict, "stage_policy_busy")
	default:
		return err
	}
}

// stagePolicyAuthority reuses the project delivery policy write boundary: the
// verified human credential, the workspace fence, the locked current user and
// member row, and the project row. It returns the actor and their member row.
func stagePolicyAuthority(ctx context.Context, tx pgx.Tx, q *db.Queries, credential ArtifactCredentialSnapshot, ws, project pgtype.UUID, write bool) (pgtype.UUID, db.Member, error) {
	kind, id, err := credential.Actor()
	if err != nil || kind != "human" {
		return pgtype.UUID{}, db.Member{}, collaborationError(http.StatusForbidden, "stage_policy_denied")
	}
	actor, err := util.ParseUUID(id)
	if err != nil || !ws.Valid || !project.Valid {
		return pgtype.UUID{}, db.Member{}, collaborationError(http.StatusForbidden, "stage_policy_denied")
	}
	if _, err = tx.Exec(ctx, "SET LOCAL lock_timeout='250ms'"); err != nil {
		return actor, db.Member{}, err
	}
	if err = LockArtifactWorkspace(ctx, q, ws); err != nil {
		return actor, db.Member{}, stagePolicyError(err)
	}
	members, err := LockArtifactUsers(ctx, q, ws, id)
	if err != nil {
		return actor, db.Member{}, stagePolicyError(err)
	}
	if err = CheckArtifactCredentialInTx(ctx, tx, credential); err != nil {
		return actor, db.Member{}, stagePolicyError(err)
	}
	if write {
		_, err = q.LockDeliveryPolicyProject(ctx, db.LockDeliveryPolicyProjectParams{ID: project, WorkspaceID: ws})
	} else {
		_, err = q.LockProjectForAssociationNowait(ctx, db.LockProjectForAssociationNowaitParams{ID: project, WorkspaceID: ws})
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return actor, db.Member{}, collaborationError(http.StatusNotFound, "stage_policy_project_not_found")
	}
	if err != nil {
		return actor, db.Member{}, stagePolicyError(err)
	}
	var ready bool
	if err = tx.QueryRow(ctx, collaborationStageReadinessSQL).Scan(&ready); err != nil {
		return actor, db.Member{}, err
	}
	if !ready {
		return actor, db.Member{}, collaborationError(http.StatusServiceUnavailable, "stage_schema_unavailable")
	}
	return actor, members[id], nil
}

func currentStagePolicy(ctx context.Context, q *db.Queries, ws, project pgtype.UUID) (db.ProjectStagePolicy, bool, error) {
	row, err := q.GetProjectStagePolicy(ctx, db.GetProjectStagePolicyParams{WorkspaceID: ws, ProjectID: project})
	if errors.Is(err, pgx.ErrNoRows) {
		return db.ProjectStagePolicy{}, false, nil
	}
	return row, err == nil, err
}

// GetStagePolicy reads the current policy for a current human workspace member.
func (s *CollaborationService) GetStagePolicy(ctx context.Context, ws, project pgtype.UUID, credential ArtifactCredentialSnapshot) (StagePolicy, error) {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return StagePolicy{}, err
	}
	defer tx.Rollback(ctx)
	q := s.Queries.WithTx(tx)
	if _, _, err = stagePolicyAuthority(ctx, tx, q, credential, ws, project, false); err != nil {
		return StagePolicy{}, err
	}
	row, found, err := currentStagePolicy(ctx, q, ws, project)
	if err != nil {
		return StagePolicy{}, stagePolicyError(err)
	}
	if err = tx.Commit(ctx); err != nil {
		return StagePolicy{}, err
	}
	return stagePolicyView(project, row, found), nil
}

func stagePolicyDigest(p StagePolicyRevisionParams) (string, error) {
	encoded, err := json.Marshal(struct {
		Contract         string      `json:"contract"`
		WorkspaceID      pgtype.UUID `json:"workspace_id"`
		ProjectID        pgtype.UUID `json:"project_id"`
		ExpectedRevision int64       `json:"expected_policy_revision"`
		Enabled          bool        `json:"enabled"`
		Reason           string      `json:"reason"`
	}{collaboration.StageContract, p.WorkspaceID, p.ProjectID, p.ExpectedRevision, p.Enabled, p.Reason})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

// ReviseStagePolicy appends one immutable policy revision by reason and
// revision CAS. Only a current source owner/admin may write. Enabling creates
// the revision alone: it assigns no role, approves no child and starts or stops
// no work. A refusal stores nothing; the original key and body replay the
// original revision after the current credential and membership check.
func (s *CollaborationService) ReviseStagePolicy(ctx context.Context, p StagePolicyRevisionParams) (StagePolicyReceipt, error) {
	p.Reason = strings.TrimSpace(p.Reason)
	if p.RequestID == "" || len(p.RequestID) > 128 || p.ExpectedRevision < 0 || !utf8.ValidString(p.Reason) || strings.ContainsRune(p.Reason, 0) ||
		utf8.RuneCountInString(p.Reason) < 1 || utf8.RuneCountInString(p.Reason) > 1000 {
		return StagePolicyReceipt{}, collaborationError(http.StatusBadRequest, "invalid_stage_policy_request")
	}
	digest, err := stagePolicyDigest(p)
	if err != nil {
		return StagePolicyReceipt{}, err
	}
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return StagePolicyReceipt{}, err
	}
	defer tx.Rollback(ctx)
	q := s.Queries.WithTx(tx)
	actor, member, err := stagePolicyAuthority(ctx, tx, q, p.Credential, p.WorkspaceID, p.ProjectID, true)
	if err != nil {
		return StagePolicyReceipt{}, err
	}
	// The project row lock serializes every writer of this key and revision.
	previous, err := q.GetProjectStagePolicyByRequest(ctx, db.GetProjectStagePolicyByRequestParams{WorkspaceID: p.WorkspaceID, ProjectID: p.ProjectID, CreatedBy: actor, RequestID: p.RequestID})
	if err == nil {
		if previous.RequestDigest != digest {
			return StagePolicyReceipt{}, collaborationError(http.StatusConflict, "idempotency_conflict")
		}
		if err = tx.Commit(ctx); err != nil {
			return StagePolicyReceipt{}, err
		}
		return stagePolicyReceipt(previous), nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return StagePolicyReceipt{}, stagePolicyError(err)
	}
	if member.Role != "owner" && member.Role != "admin" {
		return StagePolicyReceipt{}, collaborationError(http.StatusForbidden, "stage_policy_management_denied")
	}
	current, _, err := currentStagePolicy(ctx, q, p.WorkspaceID, p.ProjectID)
	if err != nil {
		return StagePolicyReceipt{}, stagePolicyError(err)
	}
	if current.Revision != p.ExpectedRevision {
		return StagePolicyReceipt{}, collaborationError(http.StatusConflict, "stage_policy_revision_conflict")
	}
	if p.Enabled {
		if err = stageExecutionReady(ctx, q); err != nil {
			return StagePolicyReceipt{}, err
		}
		if err = checkStageSubmissionSchema(ctx, tx); err != nil {
			return StagePolicyReceipt{}, err
		}
	}
	created, err := q.CreateProjectStagePolicy(ctx, db.CreateProjectStagePolicyParams{WorkspaceID: p.WorkspaceID, ProjectID: p.ProjectID,
		Revision: current.Revision + 1, Contract: collaboration.StageContract, Enabled: p.Enabled, Reason: p.Reason,
		CreatedBy: actor, RequestID: p.RequestID, RequestDigest: digest})
	if err != nil {
		return StagePolicyReceipt{}, stagePolicyError(err)
	}
	// A credential may expire or be revoked while the revision is being written.
	if err = CheckArtifactCredentialInTx(ctx, tx, p.Credential); err != nil {
		return StagePolicyReceipt{}, stagePolicyError(err)
	}
	if p.Enabled {
		if err = stageExecutionReady(ctx, q); err != nil {
			return StagePolicyReceipt{}, err
		}
		if err = checkStageSubmissionSchema(ctx, tx); err != nil {
			return StagePolicyReceipt{}, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		// Unknown outcome: the caller recovers with the original key, never a new one.
		return StagePolicyReceipt{}, err
	}
	return stagePolicyReceipt(created), nil
}

// GetStagePolicyOperation returns the actor's own committed revision for an
// original key. It never infers success from the current policy value.
func (s *CollaborationService) GetStagePolicyOperation(ctx context.Context, ws, project pgtype.UUID, credential ArtifactCredentialSnapshot, requestID string) (StagePolicyReceipt, error) {
	if requestID == "" || len(requestID) > 128 {
		return StagePolicyReceipt{}, collaborationError(http.StatusBadRequest, "invalid_request_id")
	}
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return StagePolicyReceipt{}, err
	}
	defer tx.Rollback(ctx)
	q := s.Queries.WithTx(tx)
	actor, _, err := stagePolicyAuthority(ctx, tx, q, credential, ws, project, false)
	if err != nil {
		return StagePolicyReceipt{}, err
	}
	row, err := q.GetProjectStagePolicyByRequest(ctx, db.GetProjectStagePolicyByRequestParams{WorkspaceID: ws, ProjectID: project, CreatedBy: actor, RequestID: requestID})
	if errors.Is(err, pgx.ErrNoRows) {
		return StagePolicyReceipt{}, collaborationError(http.StatusNotFound, "stage_policy_operation_not_found")
	}
	if err != nil {
		return StagePolicyReceipt{}, stagePolicyError(err)
	}
	if err = tx.Commit(ctx); err != nil {
		return StagePolicyReceipt{}, err
	}
	return stagePolicyReceipt(row), nil
}
