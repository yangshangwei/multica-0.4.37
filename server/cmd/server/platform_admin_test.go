package main

import (
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestPlatformAdminBootstrapRejectsAmbiguousOperatorInput(t *testing.T) {
	t.Setenv("MULTICA_AUTH_MODE", "password")
	for _, args := range [][]string{nil, {"unknown"}, {"bootstrap"}, {"bootstrap", "--user", "invalid", "--reason", "initial setup"}, {"bootstrap", "--user", "00000000-0000-4000-8000-000000000001"}, {"bootstrap", "--user", "00000000-0000-4000-8000-000000000001", "--reason", "   "}} {
		if err := runPlatformAdmin(args); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}

func TestPlatformControlsRequireUsableIndexesAndStateTrigger(t *testing.T) {
	f := newManagedRouterFixture(t)
	schema := strings.Split(f.pool.Config().ConnConfig.RuntimeParams["search_path"], ",")[0]
	indexes := []string{"managed_installation_id_uidx", "managed_installation_key_uidx", "installation_binding_id_uidx", "installation_binding_active_uidx", "installation_binding_scope_idx", "admin_audit_operation_phase_uidx", "admin_cancel_root_lookup_idx", "admin_operation_followers_idx", "admin_operation_due_idx", "admin_operation_installation_time_idx"}
	for _, name := range indexes {
		var definition string
		f.fx.QueryRow(t, "SELECT pg_get_indexdef(to_regclass($1))", "public."+name).Scan(&definition)
		f.fx.Exec(t, "DROP INDEX IF EXISTS "+pgx.Identifier{schema, name}.Sanitize())
		f.fx.Exec(t, strings.Replace(definition, " ON public.", " ON "+pgx.Identifier{schema}.Sanitize()+".", 1))
	}
	f.fx.Exec(t, `CREATE TRIGGER agent_task_queue_state_version BEFORE UPDATE OF status,runtime_id,dispatched_at ON agent_task_queue FOR EACH ROW WHEN (ROW(OLD.status,OLD.runtime_id,OLD.dispatched_at) IS DISTINCT FROM ROW(NEW.status,NEW.runtime_id,NEW.dispatched_at)) EXECUTE FUNCTION public.multica_advance_task_state_version()`)
	if err := validatePlatformControlSchema(t.Context(), f.pool); err != nil {
		t.Fatal(err)
	}
	f.fx.Exec(t, "ALTER TABLE agent_task_queue DISABLE TRIGGER agent_task_queue_state_version")
	if err := validatePlatformControlSchema(t.Context(), f.pool); err == nil {
		t.Fatal("controls accepted a disabled task version trigger")
	}
	f.fx.Exec(t, "ALTER TABLE agent_task_queue ENABLE TRIGGER agent_task_queue_state_version")
	f.fx.Exec(t, "DROP INDEX "+pgx.Identifier{schema, "admin_operation_due_idx"}.Sanitize())
	if err := validatePlatformControlSchema(t.Context(), f.pool); err == nil {
		t.Fatal("controls accepted a missing coordination index")
	}
}
