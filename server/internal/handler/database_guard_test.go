package handler

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

const handlerGuardPGOptions = "-c default_transaction_read_only=on -c statement_timeout=2000"

func TestHandlerTestMainRequiresExplicitDatabase(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, blank := range []string{"absent", "whitespace"} {
		t.Run(blank, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, executable, "-test.run=^$")
			for _, entry := range os.Environ() {
				if !strings.HasPrefix(entry, "DATABASE_URL=") && !strings.HasPrefix(entry, "PGHOST=") && !strings.HasPrefix(entry, "PGPORT=") && !strings.HasPrefix(entry, "PGOPTIONS=") {
					cmd.Env = append(cmd.Env, entry)
				}
			}
			// A restored literal URI would override PGHOST/PGPORT, so force
			// read-only startup options as well as the empty-config fence.
			cmd.Env = append(cmd.Env, "PGHOST="+filepath.Join(t.TempDir(), "no-postgres-socket"), "PGPORT=1", "PGOPTIONS="+handlerGuardPGOptions)
			if blank == "whitespace" {
				cmd.Env = append(cmd.Env, "DATABASE_URL= \t\n")
			}
			output, err := cmd.CombinedOutput()
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 2 || !strings.Contains(string(output), "handler tests require an explicit DATABASE_URL") {
				t.Fatalf("test entry did not refuse before database setup: exit=%v output=%s", err, output)
			}
			if strings.Contains(string(output), "Skipping tests") || strings.Contains(string(output), "llm retry policy") {
				t.Fatal("database or handler initialization ran before the explicit database guard")
			}
		})
	}
}

func TestHandlerDatabaseGuardPreservesReadOnlyPGOptions(t *testing.T) {
	t.Setenv("PGOPTIONS", handlerGuardPGOptions)
	// Parse only: an explicit URI must not drop the inherited startup options.
	cfg, err := pgx.ParseConfig("postgres://guard@localhost:5432/guard?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.RuntimeParams["options"] != handlerGuardPGOptions {
		t.Fatal("pgx lost the forced read-only startup options")
	}
}
