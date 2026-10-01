package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type AdminAdmissionParams struct {
	OrganizationID, InstallationID, IdempotencyKey pgtype.UUID
	ExpectedVersion                                int64
	Admission, Reason, RequestID                   string
}

type AdminAdmissionResult struct {
	Operation    db.AdminOperation
	Installation db.ManagedInstallation
	Replayed     bool
}

func (s *ManagedInstallationService) ChangeAdmission(ctx context.Context, p AdminAdmissionParams, tasks *TaskService) (AdminAdmissionResult, error) {
	var result AdminAdmissionResult
	if !p.OrganizationID.Valid || !p.InstallationID.Valid || !p.IdempotencyKey.Valid || p.ExpectedVersion < 1 || (p.Admission != "accepting" && p.Admission != "stopped") {
		return result, platformError(400, "invalid_request", "A valid installation, version and admission state are required")
	}
	if err := validateAdminReason(p.Reason, p.RequestID); err != nil {
		return result, err
	}
	if s.Queries == nil || s.TxStarter == nil {
		return result, installationError(503, "installation_unavailable")
	}
	admin := NewPlatformAdminService(s.Queries, s.TxStarter)
	if _, err := admin.Authorize(ctx, true); err != nil {
		return result, err
	}
	payload, err := json.Marshal(struct {
		InstallationID                     pgtype.UUID
		Admission, ExpectedVersion, Reason string
	}{p.InstallationID, p.Admission, strconv.FormatInt(p.ExpectedVersion, 10), strings.TrimSpace(p.Reason)})
	if err != nil {
		return result, err
	}
	digest := sha256.Sum256(payload)
	code := "admission_" + p.Admission
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return result, err
	}
	defer tx.Rollback(ctx)
	q := db.New(tx)
	op, replayed, err := admin.CreateOperationInTx(ctx, tx, db.CreateAdminOperationParams{OrganizationID: p.OrganizationID, TargetKind: "installation", TargetID: p.InstallationID, Kind: "installation.admission", IdempotencyKey: p.IdempotencyKey, PayloadHash: hex.EncodeToString(digest[:]), Reason: strings.TrimSpace(p.Reason), State: "applied", ResultCode: code, AppliedAt: pgtype.Timestamptz{Time: s.Now().UTC(), Valid: true}})
	if err != nil {
		return result, err
	}
	inst, err := q.LockManagedInstallation(ctx, p.InstallationID)
	if errors.Is(err, pgx.ErrNoRows) {
		return result, installationError(404, "installation_not_found")
	}
	if err != nil {
		return result, err
	}
	if inst.OrganizationID != p.OrganizationID || util.UUIDToString(inst.DeploymentID) != s.DeploymentID {
		return result, installationError(404, "installation_not_found")
	}
	if replayed {
		return AdminAdmissionResult{op, inst, true}, nil
	}
	if inst.Lifecycle != "active" {
		return result, installationError(409, "installation_not_active")
	}
	if inst.AdmissionVersion != p.ExpectedVersion {
		return result, installationError(409, "admission_version_conflict")
	}
	before, _ := json.Marshal(map[string]any{"admission": inst.Admission, "admission_version": strconv.FormatInt(inst.AdmissionVersion, 10)})
	inst, err = q.UpdateManagedInstallationAdmission(ctx, db.UpdateManagedInstallationAdmissionParams{ID: inst.ID, AdmissionVersion: p.ExpectedVersion, Admission: p.Admission})
	if errors.Is(err, pgx.ErrNoRows) {
		return result, installationError(409, "admission_version_conflict")
	}
	if err != nil {
		return result, err
	}
	op, err = q.CompleteAdminAdmissionOperation(ctx, op.ID)
	if err != nil {
		return result, err
	}
	after, _ := json.Marshal(map[string]any{"admission": inst.Admission, "admission_version": strconv.FormatInt(inst.AdmissionVersion, 10)})
	for _, phase := range []string{"request", "applied", "succeeded"} {
		if err = q.CreateAdminAuditEvent(ctx, db.CreateAdminAuditEventParams{OperationID: op.ID, OrganizationID: op.OrganizationID, ActorKind: op.ActorKind, ActorUserID: op.ActorID, TargetKind: op.TargetKind, TargetID: op.TargetID, Action: op.Kind, Phase: phase, RequestID: p.RequestID, Reason: op.Reason, BeforeState: before, AfterState: after, ResultCode: op.ResultCode}); err != nil {
			return result, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return result, err
	}
	// Authentication/UI flags cannot disable a policy already persisted here.
	// Notifications are hints only, and never change an acknowledged commit.
	if p.Admission == "accepting" && tasks != nil {
		wakeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		runtimes, lookupErr := s.Queries.ListInstallationAdmissionRuntimes(wakeCtx, inst.ID)
		if lookupErr != nil {
			slog.Warn("admission resumed; runtime wakeup lookup failed", "installation_id", util.UUIDToString(inst.ID), "error", lookupErr)
		} else {
			for _, runtime := range runtimes {
				tasks.ReclaimCheck.Invalidate(wakeCtx, util.UUIDToString(runtime.ID))
				tasks.notifyRuntimeMayHaveWork(runtime.ID, "")
			}
		}
	}
	return AdminAdmissionResult{op, inst, false}, nil
}
