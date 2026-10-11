package migrations

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

var initializationMigrationNames = []string{"568_platform_admin_initialization", "569_platform_admin_initialization_org", "570_platform_admin_initialization_singleton"}

const preservedInitializationSQL = `INSERT INTO platform_admin_initialization(organization_id,initialization_id,state) VALUES(gen_random_uuid(),gen_random_uuid(),'preserved_existing_admin')`
const createdInitializationSQL = `INSERT INTO platform_admin_initialization(organization_id,deployment_id,initialization_id,state,user_id,initial_credential_version) VALUES(gen_random_uuid(),gen_random_uuid(),gen_random_uuid(),'created',gen_random_uuid(),1)`

func TestPlatformAdminInitializationMigrationRetention(t *testing.T) {
	for _, name := range initializationMigrationNames {
		for _, state := range []string{"created", "preserved_existing_admin"} {
			t.Run(name+"/"+state, func(t *testing.T) {
				ctx, guard, _ := adminRollbackConnections(t, adminRollbackCase{up: initializationMigrationNames[0] + ".up.sql"})
				for _, index := range initializationMigrationNames[1:] {
					applyMigrationFile(t, ctx, guard, index+".up.sql")
				}
				insert := preservedInitializationSQL
				if state == "created" {
					insert = createdInitializationSQL
				}
				if _, err := guard.Exec(ctx, insert); err != nil {
					t.Fatal(err)
				}
				_, err := guard.Exec(ctx, readMigrationFile(t, name+".down.sql"))
				var pgErr *pgconn.PgError
				if !errors.As(err, &pgErr) || pgErr.Code != "P0001" {
					t.Fatalf("terminal marker lost rollback guard: %v", err)
				}
				var count int
				if err = guard.QueryRow(ctx, "SELECT count(*) FROM platform_admin_initialization").Scan(&count); err != nil || count != 1 {
					t.Fatal("rollback lost initialization evidence")
				}
				// Database-wide once-only state also prevents a new organization from
				// silently gaining a second automatic administrator.
				_, err = guard.Exec(ctx, preservedInitializationSQL)
				if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
					t.Fatalf("database singleton not enforced: %v", err)
				}
				if err = guard.QueryRow(ctx, `SELECT count(*) FROM pg_constraint WHERE conrelid='platform_admin_initialization'::regclass AND contype='f'`).Scan(&count); err != nil || count != 0 {
					t.Fatal("initialization must not cascade with account/org deletion")
				}
			})
		}
	}
}
func TestPlatformAdminInitializationMigrationFencesFirstWriter(t *testing.T) {
	for _, name := range initializationMigrationNames {
		t.Run(name, func(t *testing.T) {
			ctx, guard, writer := adminRollbackConnections(t, adminRollbackCase{up: initializationMigrationNames[0] + ".up.sql"})
			for _, index := range initializationMigrationNames[1:] {
				applyMigrationFile(t, ctx, guard, index+".up.sql")
			}
			tx, err := writer.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			if _, err = tx.Exec(ctx, preservedInitializationSQL); err != nil {
				t.Fatal(err)
			}
			_, err = guard.Exec(ctx, readMigrationFile(t, name+".down.sql"))
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != "55P03" {
				t.Fatalf("rollback must refuse a concurrent first writer: %v", err)
			}
			if err = tx.Rollback(ctx); err != nil {
				t.Fatal(err)
			}
			// With the writer stopped and the table empty, maintenance rollback works.
			applyMigrationFile(t, ctx, guard, name+".down.sql")
		})
	}
}
