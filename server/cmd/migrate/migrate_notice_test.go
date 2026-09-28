package main

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"
)

func TestMigrationPoolReportsSkippedRowNotices(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("integration test requires Postgres at DATABASE_URL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var output bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&output, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	pool, err := newMigrationPool(ctx, databaseURL, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	// PostgreSQL sends these outside the normal result/error channel. A
	// successful migration must not silently lose the skipped-row report.
	if _, err := pool.Exec(ctx, `DO $$ BEGIN
		RAISE NOTICE 'Skipped built-in agent rename: agent_id=agent-fixture, workspace_id=workspace-fixture, name 小阿孚 is already in use';
	END $$`); err != nil {
		t.Fatalf("emit migration notice: %v", err)
	}
	for _, want := range []string{"Skipped built-in agent rename", "agent_id=agent-fixture", "workspace_id=workspace-fixture", "小阿孚"} {
		if !strings.Contains(output.String(), want) {
			t.Errorf("migration log omitted %q: %s", want, output.String())
		}
	}
}
