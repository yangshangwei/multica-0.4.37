package migrations

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestPasswordRollbackProtectsConfiguredAccounts(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("integration test requires DATABASE_URL")
	}
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	schema := pgx.Identifier{fmt.Sprintf("password_rollback_%d", time.Now().UnixNano())}.Sanitize()
	if _, err = conn.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer conn.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
	if _, err = conn.Exec(ctx, "SET search_path TO "+schema); err != nil {
		t.Fatal(err)
	}
	if _, err = conn.Exec(ctx, `CREATE TABLE "user" (id UUID NOT NULL, email TEXT NOT NULL);
 CREATE TABLE personal_access_token (user_id UUID);
 CREATE TABLE task_token (user_id UUID);
 CREATE TABLE daemon_token (token_hash TEXT);`); err != nil {
		t.Fatal(err)
	}
	up := []string{"462_password_credentials.up.sql", "463_password_user_unique.up.sql", "464_password_username_unique.up.sql"}
	for _, name := range up {
		applyMigrationFile(t, ctx, conn, name)
	}
	if _, err = conn.Exec(ctx, `INSERT INTO user_password_credential (user_id,username,password_hash) VALUES ('c7fd813c-8e44-47aa-a42d-44e008ced83f','rollback_user','preserve-hash')`); err != nil {
		t.Fatal(err)
	}
	down := []string{"464_password_username_unique.down.sql", "463_password_user_unique.down.sql", "462_password_credentials.down.sql"}
	for _, name := range down {
		_, err = conn.Exec(ctx, readMigrationFile(t, name))
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "P0001" || !strings.Contains(pgErr.Message, "password accounts exist") {
			t.Fatalf("%s: expected protected rollback refusal, got %v", name, err)
		}
		var hash string
		if err = conn.QueryRow(ctx, `SELECT password_hash FROM user_password_credential WHERE username='rollback_user'`).Scan(&hash); err != nil || hash != "preserve-hash" {
			t.Fatalf("%s changed credential: %q %v", name, hash, err)
		}
		for _, index := range []string{"user_password_credential_user_uidx", "user_password_credential_username_uidx"} {
			var valid bool
			if err = conn.QueryRow(ctx, `SELECT indisunique AND indisvalid FROM pg_index WHERE indexrelid=to_regclass($1)`, index).Scan(&valid); err != nil || !valid {
				t.Fatalf("%s removed/invalidated %s: %v", name, index, err)
			}
		}
	}
	// Before activation, the empty schema remains reversible and can migrate up again.
	if _, err = conn.Exec(ctx, `DELETE FROM user_password_credential`); err != nil {
		t.Fatal(err)
	}
	for _, name := range down {
		applyMigrationFile(t, ctx, conn, name)
	}
	var removed bool
	if err = conn.QueryRow(ctx, `SELECT to_regclass('user_password_credential') IS NULL`).Scan(&removed); err != nil || !removed {
		t.Fatal("empty credential table was not removed", err)
	}
	for _, name := range up {
		applyMigrationFile(t, ctx, conn, name)
	}
}
