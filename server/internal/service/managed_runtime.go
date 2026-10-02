package service

import (
	"context"
	"errors"
	"sort"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

var ErrManagedRuntimeSource = errors.New("managed runtime requires its current bound daemon credential")
var ErrRuntimeOwnerConflict = errors.New("runtime registration cannot change ownership")

func installationNamespace(workspace pgtype.UUID, daemon string) string {
	return "daemon:" + util.UUIDToString(workspace) + ":" + daemon
}

// ValidateManagedNamespace never treats a claimed daemon ID or owner PAT as
// proof of a managed installation. Only namespaces with no binding history
// retain legacy authorization; revocation cannot opt a namespace out of management.
func ValidateManagedNamespace(ctx context.Context, q *db.Queries, workspace pgtype.UUID, daemon string) (db.InstallationDaemonBinding, error) {
	var empty db.InstallationDaemonBinding
	source, hasSource := auth.PasswordSessionFromContext(ctx)
	if hasSource && source.BindingID != "" {
		if source.WorkspaceID != util.UUIDToString(workspace) || source.DaemonID != daemon {
			return empty, ErrManagedRuntimeSource
		}
	}
	if daemon == "" {
		return empty, nil
	}
	binding, err := q.GetActiveInstallationBinding(ctx, db.GetActiveInstallationBindingParams{WorkspaceID: workspace, DaemonID: daemon})
	if errors.Is(err, pgx.ErrNoRows) {
		if hasSource && source.BindingID != "" {
			return empty, auth.ErrPasswordSession
		}
		_, err = q.GetLatestInstallationBinding(ctx, db.GetLatestInstallationBindingParams{WorkspaceID: workspace, DaemonID: daemon})
		if errors.Is(err, pgx.ErrNoRows) {
			return empty, nil
		}
		if err != nil {
			return empty, err
		}
		return empty, ErrManagedRuntimeSource
	}
	if err != nil {
		return empty, err
	}
	if !hasSource || source.Kind != "daemon_token" || source.BindingID != util.UUIDToString(binding.ID) || source.BindingEpoch != binding.BindingEpoch || source.UserID != util.UUIDToString(binding.PrincipalUserID) || source.Version != binding.AuthVersion {
		return empty, ErrManagedRuntimeSource
	}
	// CheckPasswordVersion also validates the binding carried by this context.
	if _, err := auth.CheckPasswordVersion(ctx, q, source.UserID, source.Version); err != nil {
		return empty, err
	}
	return binding, nil
}

func ValidateManagedRuntime(ctx context.Context, lookup RuntimeLookup, runtime db.AgentRuntime) (db.InstallationDaemonBinding, error) {
	q := lookup.Queries
	binding, err := ValidateManagedNamespace(ctx, q, runtime.WorkspaceID, runtime.DaemonID.String)
	if err != nil {
		return binding, err
	}
	if binding.ID.Valid {
		current, err := lookup.Get(ctx, runtime.ID)
		if err != nil {
			return binding, err
		}
		if runtime.OwnerID != binding.PrincipalUserID || current.OwnerID != binding.PrincipalUserID || current.WorkspaceID != binding.WorkspaceID || current.DaemonID.String != binding.DaemonID {
			return binding, ErrManagedRuntimeSource
		}
	}
	return binding, nil
}

// LockManagedDaemonRegistration follows the same user -> namespace order as
// first bind, then checks ownership before any legacy COALESCE upsert executes.
func LockManagedDaemonRegistration(ctx context.Context, q *db.Queries, workspace pgtype.UUID, daemon string, owner pgtype.UUID) (pgtype.UUID, error) {
	if source, ok := auth.PasswordSessionFromContext(ctx); ok {
		if _, err := auth.LockPasswordSession(ctx, q); err != nil {
			return owner, err
		}
		if source.Kind == "daemon_token" && (source.WorkspaceID != util.UUIDToString(workspace) || source.DaemonID != daemon) {
			return owner, ErrManagedRuntimeSource
		}
		var err error
		owner, err = util.ParseUUID(source.UserID)
		if err != nil {
			return owner, auth.ErrPasswordSession
		}
	} else if owner.Valid {
		if _, err := q.LockPasswordUser(ctx, owner); err != nil {
			return owner, err
		}
	}
	if err := q.LockInstallationNamespace(ctx, installationNamespace(workspace, daemon)); err != nil {
		return owner, err
	}
	binding, err := ValidateManagedNamespace(ctx, q, workspace, daemon)
	if err != nil {
		return owner, err
	}
	if binding.ID.Valid {
		count, err := q.CountForeignInstallationDaemonRuntimes(ctx, db.CountForeignInstallationDaemonRuntimesParams{WorkspaceID: workspace, DaemonID: pgtype.Text{String: daemon, Valid: true}, PrincipalUserID: binding.PrincipalUserID})
		if err != nil {
			return owner, err
		}
		if count != 0 {
			return owner, ErrManagedRuntimeSource
		}
		owner = binding.PrincipalUserID
	}
	return owner, nil
}

// LockManagedRuntime fences a claim against binding, account and runtime-owner
// changes. Call before taking an agent/task lock, using transaction queries.
func LockManagedRuntime(ctx context.Context, lookup RuntimeLookup, runtime db.AgentRuntime) (db.InstallationDaemonBinding, error) {
	q := lookup.Queries
	if _, ok := auth.PasswordSessionFromContext(ctx); ok {
		if _, err := auth.LockPasswordSession(ctx, q); err != nil {
			return db.InstallationDaemonBinding{}, err
		}
	}
	if runtime.DaemonID.Valid && runtime.DaemonID.String != "" {
		if err := q.LockInstallationNamespace(ctx, installationNamespace(runtime.WorkspaceID, runtime.DaemonID.String)); err != nil {
			return db.InstallationDaemonBinding{}, err
		}
	}
	current, err := lookup.Get(ctx, runtime.ID)
	if err != nil {
		return db.InstallationDaemonBinding{}, err
	}
	if current.WorkspaceID != runtime.WorkspaceID || current.DaemonID != runtime.DaemonID {
		return db.InstallationDaemonBinding{}, ErrManagedRuntimeSource
	}
	binding, err := ValidateManagedRuntime(ctx, lookup, current)
	if err != nil || !binding.ID.Valid {
		return binding, err
	}
	// Admission changes and binding changes serialize before agent/task locks.
	// Stopped admission still permits completion and recovery of prior claims.
	if _, err = q.LockManagedInstallationForAdmissionRead(ctx, binding.InstallationID); err != nil {
		return binding, err
	}
	return binding, nil
}

func LockManagedRuntimes(ctx context.Context, lookup RuntimeLookup, ids []pgtype.UUID) error {
	runtimes := make([]db.AgentRuntime, 0, len(ids))
	for _, id := range ids {
		runtime, err := lookup.Get(ctx, id)
		if err != nil {
			return err
		}
		runtimes = append(runtimes, runtime)
	}
	sort.Slice(runtimes, func(i, j int) bool {
		return installationNamespace(runtimes[i].WorkspaceID, runtimes[i].DaemonID.String) < installationNamespace(runtimes[j].WorkspaceID, runtimes[j].DaemonID.String)
	})
	for _, runtime := range runtimes {
		if _, err := LockManagedRuntime(ctx, lookup, runtime); err != nil {
			return err
		}
	}
	return nil
}

func lockManagedTaskSource(ctx context.Context, q *db.Queries) error {
	source, ok := auth.PasswordSessionFromContext(ctx)
	if !ok || source.BindingID == "" || source.Kind != "daemon_token" {
		return nil
	}
	if _, err := auth.LockPasswordSession(ctx, q); err != nil {
		return err
	}
	workspace, err := util.ParseUUID(source.WorkspaceID)
	if err != nil {
		return auth.ErrPasswordSession
	}
	if err = q.LockInstallationNamespace(ctx, installationNamespace(workspace, source.DaemonID)); err != nil {
		return err
	}
	_, err = ValidateManagedNamespace(ctx, q, workspace, source.DaemonID)
	return err
}

func ValidateManagedTaskBinding(ctx context.Context, task db.AgentTaskQueue) error {
	if !task.ExecutionBindingID.Valid {
		return nil
	}
	source, ok := auth.PasswordSessionFromContext(ctx)
	if !ok || source.BindingID != util.UUIDToString(task.ExecutionBindingID) || source.BindingEpoch != task.ExecutionBindingEpoch.Int64 {
		return ErrManagedRuntimeSource
	}
	return nil
}

// continuationSubmissionInstallation uses verified request metadata or a stored
// source-task snapshot for automatic continuations. Manual reruns use only
// the new request context. Current runtime binding is execution identity, not
// evidence of where the original submission came from.
func continuationSubmissionInstallation(ctx context.Context, q *db.Queries, sourceTask pgtype.UUID) (pgtype.UUID, error) {
	if supplied := auth.SubmissionInstallationFromContext(ctx); supplied.Valid {
		return supplied, nil
	}
	if !sourceTask.Valid {
		return pgtype.UUID{}, nil
	}
	source, err := q.GetAgentTask(ctx, sourceTask)
	if err != nil {
		return pgtype.UUID{}, err
	}
	return source.SubmittedInstallationID, nil
}

// PreserveDaemonRuntimeOwner protects the ownership recorded before managed
// enrollment as well. Registration is not an ownership-transfer operation;
// unknown legacy owners stay unknown instead of being claimed by an asserted ID.
func PreserveDaemonRuntimeOwner(ctx context.Context, q *db.Queries, workspace pgtype.UUID, daemon, provider string, profile, sourceOwner pgtype.UUID) (pgtype.UUID, error) {
	existing, err := q.GetRuntimeRegistrationOwner(ctx, db.GetRuntimeRegistrationOwnerParams{WorkspaceID: workspace, DaemonID: pgtype.Text{String: daemon, Valid: true}, Provider: provider, ProfileID: profile})
	if errors.Is(err, pgx.ErrNoRows) {
		return sourceOwner, nil
	}
	if err != nil {
		return pgtype.UUID{}, err
	}
	if existing.OwnerID.Valid && sourceOwner.Valid && existing.OwnerID != sourceOwner {
		return pgtype.UUID{}, ErrRuntimeOwnerConflict
	}
	return existing.OwnerID, nil
}
