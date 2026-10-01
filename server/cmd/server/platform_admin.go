package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// runPlatformAdmin runs before listeners and workers. It never chooses an
// account implicitly or makes an HTTP request with deployment privileges.
func runPlatformAdmin(args []string) error {
	if !auth.PasswordMode() {
		return errors.New("platform-admin requires MULTICA_AUTH_MODE=password")
	}
	if len(args) == 0 || args[0] != "bootstrap" {
		return errors.New("usage: platform-admin bootstrap --user <UUID> --reason <reason>")
	}
	flags := flag.NewFlagSet("platform-admin bootstrap", flag.ContinueOnError)
	user := flags.String("user", "", "Existing password account UUID")
	reason := flags.String("reason", "", "Reason for administrator initialization")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	id, err := util.ParseUUID(*user)
	if err != nil || flags.NArg() != 0 || strings.TrimSpace(*reason) == "" || len(*reason) > 1000 {
		return errors.New("an existing user UUID and a reason of 1–1000 bytes are required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		return err
	}
	defer pool.Close()
	if err = preparePlatformOrganization(ctx, pool); err != nil {
		return err
	}
	q := db.New(pool)
	org, err := q.GetInternalOrganization(ctx)
	if err != nil {
		return err
	}
	result, err := service.NewPlatformAdminService(q, pool).Bootstrap(ctx, service.PlatformAdminBootstrapParams{OrganizationID: org.ID, TargetUserID: id, Reason: strings.TrimSpace(*reason), RequestID: uuid.NewString()})
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "Administrator initialized for %s (operation %s). Previous credentials are revoked; password login is required again.\n", *user, util.UUIDToString(result.Operation.ID))
	return nil
}

func preparePlatformOrganization(ctx context.Context, pool *pgxpool.Pool) error {
	var conflict bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM workspace WHERE slug='admin')`).Scan(&conflict); err != nil {
		return err
	}
	if conflict {
		return errors.New("reserved admin workspace slug exists; resolve it before enabling administration")
	}
	// An interrupted concurrent build can exist in the ledger but be invalid.
	indexes := []string{"organization_id_uidx", "organization_internal_uidx", "organization_workspace_uidx", "platform_role_user_uidx", "admin_operation_id_uidx", "admin_operation_actor_key_uidx", "admin_audit_id_uidx", "admin_audit_scope_time_idx", "organization_workspace_scope_idx"}
	for _, name := range indexes {
		var valid bool
		if err := pool.QueryRow(ctx, `SELECT COALESCE((SELECT indisvalid AND indisready FROM pg_index WHERE indexrelid=to_regclass($1)),false)`, name).Scan(&valid); err != nil {
			return err
		}
		if !valid {
			return fmt.Errorf("required platform index %s is missing or invalid; repair migrations before activation", name)
		}
	}
	if err := validatePlatformControlSchema(ctx, pool); err != nil {
		return err
	}
	q := db.New(pool)
	if _, err := q.EnsureInternalOrganization(ctx); err != nil {
		return err
	}
	for {
		rows, err := q.BackfillWorkspaceOrganizations(ctx)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			break
		}
	}
	missing, err := q.CountUnassignedWorkspaces(ctx)
	if err != nil {
		return err
	}
	if missing != 0 {
		return fmt.Errorf("%d workspaces remain without organization ownership", missing)
	}
	return nil
}

func validatePlatformControlSchema(ctx context.Context, pool *pgxpool.Pool) error {
	for _, required := range [][2]string{
		{"managed_installation_id_uidx", "managed_installation"},
		{"managed_installation_key_uidx", "managed_installation"},
		{"installation_binding_id_uidx", "installation_daemon_binding"},
		{"installation_binding_active_uidx", "installation_daemon_binding"},
		{"installation_binding_scope_idx", "installation_daemon_binding"},
		{"admin_audit_operation_phase_uidx", "admin_audit_event"},
		{"admin_cancel_root_lookup_idx", "admin_operation"},
		{"admin_operation_followers_idx", "admin_operation"},
		{"admin_operation_due_idx", "admin_operation"},
		{"admin_operation_installation_time_idx", "admin_operation"},
	} {
		var valid bool
		if err := pool.QueryRow(ctx, `SELECT COALESCE((SELECT indisvalid AND indisready AND indrelid=to_regclass($2) FROM pg_index WHERE indexrelid=to_regclass($1)),false)`, required[0], required[1]).Scan(&valid); err != nil {
			return err
		}
		if !valid {
			return fmt.Errorf("required control index %s is missing or invalid; repair migrations before activation", required[0])
		}
	}
	var trigger bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_trigger WHERE tgrelid=to_regclass('agent_task_queue') AND tgname='agent_task_queue_state_version' AND tgenabled IN ('O','A') AND tgfoid=to_regprocedure('multica_advance_task_state_version()'))`).Scan(&trigger); err != nil {
		return err
	}
	if !trigger {
		return errors.New("task state-version trigger is missing or disabled; repair migrations before control activation")
	}
	return nil
}
