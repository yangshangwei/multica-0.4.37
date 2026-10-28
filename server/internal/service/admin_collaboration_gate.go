package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// Gate actions a super administrator may apply from the console. Closing
// execution is absent on purpose: E06 requires a durable freeze that stops
// every active queue before the gate reads closed, and that scan does not exist
// yet. Opening execution never writes any session or target active.
const (
	AdminCollaborationGateOpenCreation  = "open_creation"
	AdminCollaborationGateCloseCreation = "close_creation"
	AdminCollaborationGateOpenExecution = "open_execution"
)

const adminCollaborationGateConflict = "collaboration_gate_conflict"

var adminCollaborationGateCodes = map[string]string{
	AdminCollaborationGateOpenCreation:  "collaboration_creation_opened",
	AdminCollaborationGateCloseCreation: "collaboration_creation_closed",
	AdminCollaborationGateOpenExecution: "collaboration_execution_opened",
}

type AdminCollaborationGateView struct {
	Scope                string   `json:"scope"`
	AsOf                 string   `json:"as_of"`
	CreationEnabled      bool     `json:"creation_enabled"`
	ExecutionEnabled     bool     `json:"execution_enabled"`
	HostExecutionEnabled bool     `json:"host_execution_enabled"`
	ExecutionEpoch       string   `json:"execution_epoch"`
	AllowedActions       []string `json:"allowed_actions"`
}

// adminCollaborationGateAllows is the single precondition for both the
// advertised controls and the locked write.
func adminCollaborationGateAllows(gate db.CollaborationExecutionGate, action string) bool {
	switch action {
	case AdminCollaborationGateOpenCreation:
		return !gate.CreationEnabled
	case AdminCollaborationGateCloseCreation:
		return gate.CreationEnabled
	case AdminCollaborationGateOpenExecution:
		// A recorded freeze must finish its scan before execution can reopen.
		return !gate.Enabled && !gate.FreezeOperationID.Valid
	}
	return false
}

func adminCollaborationGateView(scope pgtype.UUID, gate db.CollaborationExecutionGate, role string, now time.Time) AdminCollaborationGateView {
	actions := []string{}
	if role == PlatformRoleSuperAdmin {
		for _, action := range []string{AdminCollaborationGateOpenCreation, AdminCollaborationGateCloseCreation, AdminCollaborationGateOpenExecution} {
			if adminCollaborationGateAllows(gate, action) {
				actions = append(actions, action)
			}
		}
	}
	return AdminCollaborationGateView{Scope: util.UUIDToString(scope), AsOf: now.UTC().Format(time.RFC3339Nano),
		CreationEnabled: gate.CreationEnabled, ExecutionEnabled: gate.Enabled, HostExecutionEnabled: gate.HostExecutionEnabled,
		ExecutionEpoch: strconv.FormatInt(gate.Epoch, 10), AllowedActions: actions}
}

func adminCollaborationGateState(gate db.CollaborationExecutionGate) []byte {
	data, _ := json.Marshal(map[string]any{"creation_enabled": gate.CreationEnabled, "execution_enabled": gate.Enabled,
		"host_execution_enabled": gate.HostExecutionEnabled, "execution_epoch": strconv.FormatInt(gate.Epoch, 10)})
	return data
}

type AdminCollaborationGateService struct {
	Queries   *db.Queries
	TxStarter TxStarter
	Now       func() time.Time
}

func NewAdminCollaborationGateService(q *db.Queries, tx TxStarter) *AdminCollaborationGateService {
	return &AdminCollaborationGateService{Queries: q, TxStarter: tx, Now: time.Now}
}

// Get is a platform read; observers see the state but no actions.
func (s *AdminCollaborationGateService) Get(ctx context.Context) (AdminCollaborationGateView, error) {
	actor, err := NewPlatformAdminService(s.Queries, s.TxStarter).Authorize(ctx, false)
	if err != nil {
		return AdminCollaborationGateView{}, err
	}
	gate, err := s.Queries.GetCollaborationGate(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return AdminCollaborationGateView{}, platformError(http.StatusServiceUnavailable, "collaboration_gate_unavailable", "The collaboration gate is unavailable")
	}
	if err != nil {
		return AdminCollaborationGateView{}, err
	}
	return adminCollaborationGateView(actor.OrganizationID, gate, actor.Role, s.Now()), nil
}

type AdminCollaborationGateParams struct {
	OrganizationID, IdempotencyKey pgtype.UUID
	Action, Reason, RequestID      string
	// ExpectedExecutionEpoch is the epoch the administrator saw, required by
	// open_execution only. Every close raises the epoch, so a late or retried
	// open cannot reopen execution that was closed after it was prepared.
	ExpectedExecutionEpoch int64
}

type AdminCollaborationGateResult struct {
	Operation db.AdminOperation
	Gate      AdminCollaborationGateView
	Replayed  bool
}

func adminCollaborationGateHash(p AdminCollaborationGateParams) (string, error) {
	payload, err := json.Marshal(struct {
		Kind          string `json:"kind"`
		Action        string `json:"action"`
		ExpectedEpoch string `json:"expected_execution_epoch"`
		Reason        string `json:"reason"`
	}{"collaboration.gate", p.Action, strconv.FormatInt(p.ExpectedExecutionEpoch, 10), strings.TrimSpace(p.Reason)})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

// Change records a refused precondition as a failed receipt under the original
// key, like alert conflicts. The gate has no monotonic version for creation,
// so an unrecorded refusal would leave a lost original request able to commit
// later; the committed failure makes that request replay the refusal instead.
func (s *AdminCollaborationGateService) Change(ctx context.Context, p AdminCollaborationGateParams) (AdminCollaborationGateResult, error) {
	var result AdminCollaborationGateResult
	code := adminCollaborationGateCodes[p.Action]
	if code == "" || !p.OrganizationID.Valid || !p.IdempotencyKey.Valid {
		return result, platformError(http.StatusBadRequest, "invalid_request", "A valid gate action and idempotency key are required")
	}
	if (p.Action == AdminCollaborationGateOpenExecution) != (p.ExpectedExecutionEpoch > 0) {
		return result, platformError(http.StatusBadRequest, "invalid_request", "Only opening execution takes the observed execution epoch, and it requires one")
	}
	if err := validateAdminReason(p.Reason, p.RequestID); err != nil {
		return result, err
	}
	if s.Queries == nil || s.TxStarter == nil {
		return result, platformError(http.StatusServiceUnavailable, "admin_unavailable", "Administration is unavailable")
	}
	admin := NewPlatformAdminService(s.Queries, s.TxStarter)
	actor, err := admin.Authorize(ctx, true)
	if err != nil {
		return result, err
	}
	hash, err := adminCollaborationGateHash(p)
	if err != nil {
		return result, err
	}
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return result, err
	}
	defer tx.Rollback(ctx)
	q := db.New(tx)
	now := s.Now().UTC()
	op, replayed, err := admin.CreateOperationInTx(ctx, tx, db.CreateAdminOperationParams{OrganizationID: p.OrganizationID, TargetKind: "collaboration_gate", TargetID: p.OrganizationID,
		Kind: "collaboration.gate", IdempotencyKey: p.IdempotencyKey, PayloadHash: hash, Reason: strings.TrimSpace(p.Reason), State: "applied", ResultCode: code})
	if err != nil {
		return result, err
	}
	if replayed {
		// The receipt is the answer; reading the current state needs no lock
		// that would hold back collaboration writers.
		gate, err := q.GetCollaborationGate(ctx)
		if err != nil {
			return result, err
		}
		return AdminCollaborationGateResult{op, adminCollaborationGateView(actor.OrganizationID, gate, actor.Role, now), true}, nil
	}
	gate, err := q.LockCollaborationGateForAdmin(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return result, platformError(http.StatusServiceUnavailable, "collaboration_gate_unavailable", "The collaboration gate is unavailable")
	}
	if err != nil {
		return result, err
	}
	before := adminCollaborationGateState(gate)
	var conflict error
	if !adminCollaborationGateAllows(gate, p.Action) || p.Action == AdminCollaborationGateOpenExecution && gate.Epoch != p.ExpectedExecutionEpoch {
		conflict = platformError(http.StatusConflict, adminCollaborationGateConflict, "The collaboration gate changed; refresh its state")
	} else {
		switch p.Action {
		case AdminCollaborationGateOpenCreation, AdminCollaborationGateCloseCreation:
			gate, err = q.SetCollaborationCreationEnabled(ctx, p.Action == AdminCollaborationGateOpenCreation)
		case AdminCollaborationGateOpenExecution:
			gate, err = q.OpenCollaborationExecution(ctx)
		}
		if err != nil {
			return result, err
		}
	}
	state, applied, phase := "succeeded", pgtype.Timestamptz{Time: now, Valid: true}, "applied"
	if conflict != nil {
		code, state, applied, phase = adminCollaborationGateConflict, "failed", pgtype.Timestamptz{}, "failed"
	}
	op, err = q.FinishAdminCollaborationGateOperation(ctx, db.FinishAdminCollaborationGateOperationParams{ID: op.ID, State: state, ResultCode: code, AppliedAt: applied})
	if err != nil {
		return result, err
	}
	phases := []string{"request", phase}
	if conflict == nil {
		phases = append(phases, "succeeded")
	}
	after := adminCollaborationGateState(gate)
	for _, eventPhase := range phases {
		if err = q.CreateAdminAuditEvent(ctx, db.CreateAdminAuditEventParams{OperationID: op.ID, OrganizationID: op.OrganizationID, ActorKind: op.ActorKind, ActorUserID: op.ActorID, TargetKind: op.TargetKind, TargetID: op.TargetID, Action: op.Kind, Phase: eventPhase, RequestID: p.RequestID, Reason: op.Reason, BeforeState: before, AfterState: after, ResultCode: code}); err != nil {
			return result, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return result, err
	}
	return AdminCollaborationGateResult{op, adminCollaborationGateView(actor.OrganizationID, gate, actor.Role, now), false}, conflict
}
