package auth

import (
	"context"
	"errors"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// ManagedDeploymentID is a stable non-secret deployment identity. Missing or
// invalid configuration never grants access to an existing managed binding.
func ManagedDeploymentID() string {
	value := strings.TrimSpace(os.Getenv("MULTICA_DEPLOYMENT_ID"))
	id, err := uuid.Parse(value)
	if err != nil || id == uuid.Nil || id.String() != value {
		return ""
	}
	return value
}

// CheckManagedBinding supplements the password-version fence. It is called
// for HTTP authentication and again by WS/transaction authorization so a
// revoked binding cannot survive in an otherwise valid account session.
func CheckManagedBinding(ctx context.Context, q *db.Queries, session PasswordSession) error {
	if session.BindingID == "" {
		return nil
	}
	if (session.Kind != "daemon_token" && session.Kind != "task_token") || session.BindingEpoch <= 0 || session.WorkspaceID == "" || session.DaemonID == "" {
		return ErrPasswordSession
	}
	id, err := util.ParseUUID(session.BindingID)
	if err != nil {
		return ErrPasswordSession
	}
	binding, err := q.GetInstallationBinding(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrPasswordSession
	}
	if err != nil {
		return err
	}
	if binding.State != "active" || binding.BindingEpoch != session.BindingEpoch || binding.AuthVersion != session.Version || util.UUIDToString(binding.PrincipalUserID) != session.UserID || util.UUIDToString(binding.WorkspaceID) != session.WorkspaceID || binding.DaemonID != session.DaemonID {
		return ErrPasswordSession
	}
	inst, err := q.GetManagedInstallation(ctx, binding.InstallationID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrPasswordSession
	}
	if err != nil {
		return err
	}
	if inst.Lifecycle != "active" || ManagedDeploymentID() == "" || util.UUIDToString(inst.DeploymentID) != ManagedDeploymentID() {
		return ErrPasswordSession
	}
	org, err := q.GetInternalOrganization(ctx)
	if err != nil {
		return errors.New("managed organization unavailable")
	}
	if org.ID != inst.OrganizationID {
		return ErrPasswordSession
	}
	workspaceOrg, err := q.GetInstallationWorkspaceOrganization(ctx, binding.WorkspaceID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrPasswordSession
	}
	if err != nil {
		return err
	}
	if workspaceOrg != inst.OrganizationID {
		return ErrPasswordSession
	}
	_, err = q.GetMemberByUserAndWorkspace(ctx, db.GetMemberByUserAndWorkspaceParams{UserID: binding.PrincipalUserID, WorkspaceID: binding.WorkspaceID})
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrPasswordSession
	}
	return err
}

// PasswordSessionForTask carries the credential's minting provenance without
// rewriting a historical task's execution snapshot. Callers still validate
// the account/version and binding through CheckPasswordVersion before use.
func PasswordSessionForTask(ctx context.Context, q *db.Queries, token db.TaskToken) (PasswordSession, error) {
	s := PasswordSession{UserID: util.UUIDToString(token.UserID), Version: token.AuthVersion, Kind: "task_token"}
	if !token.InstallationBindingID.Valid {
		return s, nil
	}
	if !token.InstallationBindingEpoch.Valid || token.InstallationBindingEpoch.Int64 <= 0 {
		return s, ErrPasswordSession
	}
	task, err := q.GetAgentTask(ctx, token.TaskID)
	if errors.Is(err, pgx.ErrNoRows) {
		return s, ErrPasswordSession
	}
	if err != nil {
		return s, err
	}
	if task.AgentID != token.AgentID || (task.Status != "preparing" && task.Status != "dispatched" && task.Status != "running" && task.Status != "waiting_local_directory") {
		return s, ErrPasswordSession
	}
	if task.ExecutionBindingID.Valid && (task.ExecutionBindingID != token.InstallationBindingID || !task.ExecutionBindingEpoch.Valid || task.ExecutionBindingEpoch.Int64 != token.InstallationBindingEpoch.Int64) {
		return s, ErrPasswordSession
	}
	binding, err := q.GetInstallationBinding(ctx, token.InstallationBindingID)
	if errors.Is(err, pgx.ErrNoRows) {
		return s, ErrPasswordSession
	}
	if err != nil {
		return s, err
	}
	s.WorkspaceID = util.UUIDToString(token.WorkspaceID)
	s.DaemonID = binding.DaemonID
	s.BindingID = util.UUIDToString(token.InstallationBindingID)
	s.BindingEpoch = token.InstallationBindingEpoch.Int64
	return s, nil
}
