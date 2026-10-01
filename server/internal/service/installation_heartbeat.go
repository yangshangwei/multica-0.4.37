package service

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/installation"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type InstallationHeartbeatResult struct {
	ServerTime      string `json:"server_time"`
	NextReportAfter int    `json:"next_report_after"`
	Accepted        bool   `json:"accepted"`
	MetadataProof   string `json:"metadata_proof,omitempty"`
}

func (s *ManagedInstallationService) Heartbeat(ctx context.Context, installationID string, proof installation.HeartbeatProof) (InstallationHeartbeatResult, error) {
	result := InstallationHeartbeatResult{ServerTime: s.Now().UTC().Format(time.RFC3339), NextReportAfter: 60}
	if s.Queries == nil || s.TxStarter == nil {
		return result, installationError(503, "installation_unavailable")
	}
	id, err := installationUUID(installationID)
	if err != nil {
		return result, err
	}
	actor, user, err := s.source(ctx, s.Queries, true)
	if err != nil {
		return result, err
	}
	inst, err := s.Queries.GetManagedInstallation(ctx, id)
	if err != nil {
		return result, err
	}
	p, err := installation.VerifyHeartbeat(proof, inst.PublicKey)
	if err != nil || p.InstallationID != installationID || p.DeploymentID != s.DeploymentID || p.UserID != actor.UserID || p.AuthVersion != strconv.FormatInt(actor.Version, 10) || !validInstallationVersion(p.DesktopVersion, p.OS) {
		return result, installationError(403, "invalid_installation_report")
	}
	verifiedFingerprint := inst.KeyFingerprint
	reported, _ := strconv.ParseInt(p.ReportedAt, 10, 64)
	reportedAt := time.Unix(reported, 0)
	now := s.Now().UTC()
	if reportedAt.Before(now.Add(-180*time.Second)) || reportedAt.After(now.Add(30*time.Second)) {
		return result, installationError(400, "installation_report_stale")
	}
	sequence, _ := strconv.ParseInt(p.Sequence, 10, 64)
	boot, err := installationUUID(p.BootID)
	if err != nil {
		return result, err
	}
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return result, err
	}
	defer tx.Rollback(ctx)
	q := db.New(tx)
	if _, err = auth.LockPasswordSession(ctx, q); err != nil {
		return result, err
	}
	inst, err = q.LockManagedInstallation(ctx, id)
	if err != nil {
		return result, err
	}
	if inst.Lifecycle != "active" || util.UUIDToString(inst.DeploymentID) != s.DeploymentID || inst.KeyFingerprint != verifiedFingerprint {
		return result, installationError(403, "installation_scope_forbidden")
	}
	org, err := q.GetInternalOrganization(ctx)
	if err != nil {
		return result, installationError(503, "installation_unavailable")
	}
	if inst.OrganizationID != org.ID {
		return result, installationError(403, "installation_scope_forbidden")
	}
	_, err = q.AdvanceInstallationReport(ctx, db.AdvanceInstallationReportParams{InstallationID: id, UserID: user, AuthVersion: actor.Version, BootID: boot, Sequence: sequence, ReportedAt: pgtype.Timestamptz{Time: reportedAt, Valid: true}})
	if errors.Is(err, pgx.ErrNoRows) {
		return result, nil
	}
	if err != nil {
		return result, err
	}
	if err = q.UpdateInstallationClientReport(ctx, db.UpdateInstallationClientReportParams{ID: id, DesktopVersion: pgtype.Text{String: p.DesktopVersion, Valid: true}, Os: pgtype.Text{String: p.OS, Valid: true}}); err != nil {
		return result, err
	}
	if err = q.TouchInstallationUser(ctx, db.TouchInstallationUserParams{InstallationID: id, UserID: user}); err != nil {
		return result, err
	}
	if err = tx.Commit(ctx); err != nil {
		return result, err
	}
	result.Accepted = true
	result.MetadataProof, err = auth.MintInstallationMetadataProof(actor, inst, s.Now().UTC())
	if err != nil {
		return result, err
	}
	return result, nil
}
