package migrations

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type adminRollbackCase struct {
	name, file, setup, up, write, preserved, removed string
}

var adminRollbackCases = []adminRollbackCase{
	{
		name: "first administrative audit", file: "465_platform_admin.down.sql",
		setup: `CREATE TABLE "user" (id uuid); CREATE TABLE workspace (id uuid);`, up: "465_platform_admin.up.sql",
		write:     `INSERT INTO admin_audit_event(organization_id,actor_kind,target_kind,target_id,action,phase,request_id,reason,result_code) VALUES(gen_random_uuid(),'deployment_operator','user',gen_random_uuid(),'user.role.bootstrap','applied','rollback-race','initial administrator','role_changed')`,
		preserved: `SELECT count(*)=1 FROM admin_audit_event`, removed: `SELECT to_regclass('admin_audit_event') IS NULL`,
	},
	{
		name: "first legacy revocation", file: "475_legacy_password_revocation.down.sql",
		setup:     `CREATE TABLE "user" (id int,legacy_password_sessions_revoked_at timestamptz); INSERT INTO "user"(id) VALUES(1);`,
		write:     `UPDATE "user" SET legacy_password_sessions_revoked_at=now() WHERE id=1`,
		preserved: `SELECT legacy_password_sessions_revoked_at IS NOT NULL FROM "user" WHERE id=1`,
		removed:   `SELECT NOT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='user' AND column_name='legacy_password_sessions_revoked_at')`,
	},
	{
		name: "first enrollment challenge", file: "477_managed_installations.down.sql",
		setup:     `CREATE TABLE managed_installation(id uuid); CREATE TABLE installation_daemon_binding(id uuid); CREATE TABLE installation_challenge(id uuid); CREATE TABLE installation_report_cursor(id uuid); CREATE TABLE installation_user(id uuid); CREATE TABLE daemon_token(installation_binding_id uuid,installation_binding_epoch bigint);`,
		write:     `INSERT INTO installation_challenge VALUES(gen_random_uuid())`,
		preserved: `SELECT count(*)=1 FROM installation_challenge`, removed: `SELECT to_regclass('installation_challenge') IS NULL`,
	},
	{
		name: "first queue observation", file: "500_admin_observability.down.sql",
		setup: `CREATE TABLE agent_task_queue(id uuid DEFAULT gen_random_uuid(),status text,created_at timestamptz DEFAULT now()); CREATE TABLE admin_audit_event(id uuid);`, up: "500_admin_observability.up.sql",
		write:     `INSERT INTO agent_task_queue(status) VALUES('queued')`,
		preserved: `SELECT count(*)=1 AND bool_and(queued_at IS NOT NULL AND queued_at_source='transition') FROM agent_task_queue`,
		removed:   `SELECT NOT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='agent_task_queue' AND column_name='queued_at')`,
	},
}

func adminRollbackConnections(t *testing.T, scenario adminRollbackCase) (context.Context, *pgx.Conn, *pgx.Conn) {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("requires task-owned DATABASE_URL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)
	guard, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	writer, err := pgx.Connect(ctx, url)
	if err != nil {
		guard.Close(context.Background())
		t.Fatal(err)
	}
	schema := pgx.Identifier{fmt.Sprintf("admin_rollback_%d", time.Now().UnixNano())}.Sanitize()
	t.Cleanup(func() {
		cleanup, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		writer.Close(cleanup)
		if _, err := guard.Exec(cleanup, "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Errorf("remove private rollback fixture: %v", err)
		}
		guard.Close(cleanup)
	})
	for _, statement := range []string{"CREATE SCHEMA " + schema, "SET search_path TO " + schema, scenario.setup} {
		if _, err = guard.Exec(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = writer.Exec(ctx, "SET search_path TO "+schema); err != nil {
		t.Fatal(err)
	}
	if scenario.up != "" {
		applyMigrationFile(t, ctx, guard, scenario.up)
	}
	return ctx, guard, writer
}

func TestAdminRollbackEmptyMaintenanceAndPopulatedRefusal(t *testing.T) {
	for _, scenario := range adminRollbackCases {
		t.Run(scenario.name, func(t *testing.T) {
			ctx, guard, _ := adminRollbackConnections(t, scenario)
			if _, err := guard.Exec(ctx, scenario.write); err != nil {
				t.Fatal(err)
			}
			_, err := guard.Exec(ctx, readMigrationFile(t, scenario.file))
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != "P0001" {
				t.Fatalf("populated rollback must refuse retained evidence: %v", err)
			}
			var preserved bool
			if err := guard.QueryRow(ctx, scenario.preserved).Scan(&preserved); err != nil || !preserved {
				t.Fatalf("populated refusal lost evidence: %v", err)
			}
		})
		t.Run(scenario.name+" empty", func(t *testing.T) {
			ctx, guard, _ := adminRollbackConnections(t, scenario)
			applyMigrationFile(t, ctx, guard, scenario.file)
			var removed bool
			if err := guard.QueryRow(ctx, scenario.removed).Scan(&removed); err != nil || !removed {
				t.Fatalf("empty maintenance rollback did not remove owned schema: %v", err)
			}
		})
	}
}

func TestAdminRollbackRefusesConcurrentFirstProtectedWrite(t *testing.T) {
	for _, scenario := range adminRollbackCases {
		t.Run(scenario.name, func(t *testing.T) {
			ctx, guard, writer := adminRollbackConnections(t, scenario)
			tx, err := writer.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(context.Background())
			if _, err = tx.Exec(ctx, scenario.write); err != nil {
				t.Fatal(err)
			}
			down := readMigrationFile(t, scenario.file)
			result := make(chan error, 1)
			go func() { _, err := guard.Exec(ctx, down); result <- err }()
			var rollbackErr error
			finished := false
			// The old guard reaches DDL and waits for the invisible writer.
			// The fixed guard refuses immediately before inspecting or dropping.
			for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); {
				select {
				case rollbackErr = <-result:
					finished = true
				default:
				}
				if finished {
					break
				}
				var waiting bool
				if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_locks WHERE pid=$1 AND NOT granted)`, guard.PgConn().PID()).Scan(&waiting); err != nil {
					t.Fatal(err)
				}
				if waiting {
					break
				}
				time.Sleep(5 * time.Millisecond)
			}
			if err = tx.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			if !finished {
				select {
				case rollbackErr = <-result:
				case <-ctx.Done():
					t.Fatal("rollback did not finish after releasing fixture writer")
				}
			}
			var preserved bool
			preservationErr := guard.QueryRow(ctx, scenario.preserved).Scan(&preserved)
			var pgErr *pgconn.PgError
			if !errors.As(rollbackErr, &pgErr) || pgErr.Code != "55P03" {
				t.Fatalf("live writer must cause fail-fast lock refusal; rollback result: %v; committed evidence preserved: %t (%v)", rollbackErr, preserved, preservationErr)
			}
			if preservationErr != nil || !preserved {
				t.Fatalf("concurrent application commit lost schema or evidence: %v", preservationErr)
			}
		})
	}
}

func TestAdminRollbackGuardCoverage(t *testing.T) {
	files, err := filepath.Glob(filepath.Join(realMigrationsDir(t), "*.down.sql"))
	if err != nil {
		t.Fatal(err)
	}
	from := regexp.MustCompile(`(?i)\bFROM\s+("user"|[a-z_]+)`)
	lock := regexp.MustCompile(`(?is)LOCK TABLE\s+(.+?)\s+IN ACCESS EXCLUSIVE MODE NOWAIT;`)
	count := 0
	for _, file := range files {
		name := filepath.Base(file)
		number, err := strconv.Atoi(strings.SplitN(name, "_", 2)[0])
		if err != nil || number < 465 || number > 511 {
			continue
		}
		sql := readMigrationFile(t, name)
		if !strings.Contains(sql, "RAISE EXCEPTION") {
			continue
		}
		count++
		t.Run(name, func(t *testing.T) {
			locked := lock.FindStringSubmatch(sql)
			if len(locked) != 2 || strings.Index(sql, "LOCK TABLE") > strings.Index(sql, "IF EXISTS") {
				t.Fatal("retained-data checks require fail-fast exclusive table locks first")
			}
			for _, match := range from.FindAllStringSubmatch(sql, -1) {
				if !strings.Contains(","+strings.ReplaceAll(locked[1], " ", "")+",", ","+match[1]+",") {
					t.Errorf("checked table %s is not protected by the lock", match[1])
				}
			}
			if strings.LastIndex(sql, "DROP ") > strings.LastIndex(sql, "END") || !strings.HasSuffix(strings.TrimSpace(sql), "$$;") {
				t.Fatal("guard and destructive DDL must remain in the same DO block")
			}
		})
	}
	if count != 43 {
		t.Fatalf("review coverage changed: got %d guarded platform down files, want 43", count)
	}
}
