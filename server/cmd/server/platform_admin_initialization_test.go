package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func initializationRunner(t *testing.T) (platformAdminInitializer, *platformAdminRouterFixture) {
	t.Helper()
	f := newPlatformAdminRouterFixture(t)
	deployment := uuid.New()
	t.Setenv("MULTICA_DEPLOYMENT_ID", deployment.String())
	t.Setenv("ALLOW_SIGNUP", "false")
	org, err := db.New(f.pool).GetInternalOrganization(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return platformAdminInitializer{queries: db.New(f.pool), transactions: f.pool, organization: org.ID, deployment: pgtype.UUID{Bytes: deployment, Valid: true}, path: filepath.Join(t.TempDir(), "admin-initial-password.txt"), reason: "Test deployment initialization", openFile: openPlatformAdminCredentialFile}, f
}
func initializationFile(t *testing.T, path string) platformAdminCredentials {
	t.Helper()
	file, err := openPlatformAdminCredentialFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.close()
	c, exists, err := file.read()
	if err != nil || !exists {
		t.Fatal("expected private credentials")
	}
	return c
}
func assertInitializationRows(t *testing.T, f *platformAdminRouterFixture, want int) {
	t.Helper()
	for _, name := range []string{"user", "user_password_credential", "platform_role_binding", "platform_admin_initialization", "admin_operation", "admin_audit_event"} {
		var n int
		f.fx.QueryRow(t, "SELECT count(*) FROM "+pgx.Identifier{name}.Sanitize()).Scan(&n)
		if n != want {
			t.Fatalf("%s rows=%d want %d", name, n, want)
		}
	}
}
func TestPlatformAdminInitializeSkipsWithoutDatabaseOrFile(t *testing.T) {
	for _, mode := range []string{"classic", "password"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("MULTICA_AUTH_MODE", mode)
			t.Setenv("MULTICA_PLATFORM_ADMIN_ENABLED", "false")
			t.Setenv("DATABASE_URL", "invalid database containing SECRET")
			path := filepath.Join(t.TempDir(), "missing", "password.txt")
			var output bytes.Buffer
			if err := runPlatformAdminInitialize([]string{"--credentials-file", path, "--reason", "Controlled deployment"}, &output); err != nil {
				t.Fatal(err)
			}
			var receipt platformAdminInitializationReceipt
			if err := json.Unmarshal(output.Bytes(), &receipt); err != nil {
				t.Fatal(err)
			}
			want := "skipped_mode_disabled"
			if mode == "password" {
				want = "skipped_admin_disabled"
			}
			if receipt.Status != want || receipt.CredentialsFile != "" {
				t.Fatalf("invalid skip receipt: %s", output.String())
			}
			if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("skip touched credential storage")
			}
		})
	}
	t.Setenv("MULTICA_AUTH_MODE", "password")
	t.Setenv("MULTICA_PLATFORM_ADMIN_ENABLED", "true")
	t.Setenv("MULTICA_DEPLOYMENT_ID", "")
	var output bytes.Buffer
	err := runPlatformAdminInitialize([]string{"--credentials-file", filepath.Join(t.TempDir(), "password.txt"), "--reason", "Controlled deployment"}, &output)
	if err == nil || err.Error() != "invalid_deployment_id" || output.Len() != 0 {
		t.Fatal("missing deployment identity did not fail closed")
	}
	err = runPlatformAdminInitialize([]string{"--password", "SECRET"}, &output)
	if err == nil || strings.Contains(err.Error(), "SECRET") {
		t.Fatal("argument errors leaked input")
	}
	if got := safePlatformInitializationError(errors.New("database rejected SECRET password hash")); got.Error() != "database_unavailable" {
		t.Fatal("database error details escaped")
	}
}
func TestPlatformAdminInitializeIdempotentAndMissingCredentials(t *testing.T) {
	r, f := initializationRunner(t)
	ctx := context.Background()
	receipt, err := r.run(ctx)
	if err != nil || receipt.Status != "initialized" || receipt.CredentialsFile != r.path {
		t.Fatalf("initialize=%+v, %v", receipt, err)
	}
	c := initializationFile(t, r.path)
	marker, err := r.queries.GetPlatformAdminInitialization(ctx)
	if err != nil {
		t.Fatal(err)
	}
	credential, err := r.queries.GetPasswordCredential(ctx, marker.UserID)
	if err != nil {
		t.Fatal(err)
	}
	valid, err := auth.VerifyPassword(ctx, credential.PasswordHash, c.InitialPassword)
	if err != nil || !valid {
		t.Fatal("delivered password cannot authenticate")
	}
	encoded, err := json.Marshal(receipt)
	if err != nil || strings.Contains(string(encoded), c.InitialPassword) || strings.Contains(string(encoded), credential.PasswordHash) {
		t.Fatal("receipt exposed a secret")
	}
	again, err := r.run(ctx)
	if err != nil || again.Status != "already_initialized" || again.UserID != receipt.UserID || again.CredentialsFile != r.path {
		t.Fatal("repeat initialization changed result")
	}
	other := r
	other.path = filepath.Join(t.TempDir(), "admin-initial-password.txt")
	_, err = other.run(ctx)
	var unavailable credentialsUnavailableError
	if !errors.As(err, &unavailable) {
		t.Fatalf("other host must report unavailable: %v", err)
	}
	if _, err = os.Stat(other.path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("other host generated a second password")
	}
	if err = os.Remove(r.path); err != nil {
		t.Fatal(err)
	}
	_, err = r.run(ctx)
	if !errors.As(err, &unavailable) {
		t.Fatalf("deleted pending file must report unavailable: %v", err)
	}
	unchanged, err := r.queries.GetPasswordCredential(ctx, marker.UserID)
	if err != nil || unchanged.PasswordHash != credential.PasswordHash {
		t.Fatal("missing file changed the password")
	}
	// A completed change no longer depends on the initial file.
	f.fx.Exec(t, "UPDATE user_password_credential SET session_version=2,must_change_password=false WHERE user_id=$1", marker.UserID)
	done, err := r.run(ctx)
	if err != nil || done.Status != "already_initialized" || done.CredentialsFile != "" {
		t.Fatalf("completed account requires stale credentials: %+v %v", done, err)
	}
	if _, err = os.Stat(r.path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("completed account recreated initial credentials")
	}
	assertInitializationRows(t, f, 1)
}
func TestPlatformAdminInitializeConcurrentSameAndDifferentPaths(t *testing.T) {
	for _, shared := range []bool{true, false} {
		t.Run(map[bool]string{true: "same_host", false: "different_hosts"}[shared], func(t *testing.T) {
			r, f := initializationRunner(t)
			other := r
			if !shared {
				other.path = filepath.Join(t.TempDir(), "admin-initial-password.txt")
			}
			// Prepare both before either can commit, reproducing the actual race.
			a, err := r.prepare(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			defer a.file.close()
			b, err := other.prepare(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			defer b.file.close()
			var wg sync.WaitGroup
			receipts := make([]platformAdminInitializationReceipt, 2)
			errs := make([]error, 2)
			for i, pair := range []struct {
				runner platformAdminInitializer
				prep   initializationPreparation
			}{{r, a}, {other, b}} {
				wg.Add(1)
				go func(i int, runner platformAdminInitializer, p initializationPreparation) {
					defer wg.Done()
					receipts[i], _, errs[i] = runner.attempt(context.Background(), p, false)
					if errors.Is(errs[i], errInitializationSnapshotChanged) {
						receipts[i], errs[i] = runner.run(context.Background())
					}
				}(i, pair.runner, pair.prep)
			}
			wg.Wait()
			created, available, unavailable := 0, 0, 0
			for i, err := range errs {
				if err != nil {
					var missing credentialsUnavailableError
					if !errors.As(err, &missing) {
						t.Fatalf("concurrent error: %v", err)
					}
					unavailable++
					continue
				}
				available++
				if receipts[i].Status == "initialized" {
					created++
				}
			}
			if created != 1 || shared && (available != 2 || unavailable != 0) || !shared && (available != 1 || unavailable != 1) {
				t.Fatalf("created=%d available=%d unavailable=%d", created, available, unavailable)
			}
			assertInitializationRows(t, f, 1)
		})
	}
}
func TestPlatformAdminInitializeSyncFailurePreventsCommitAndRetainsIntent(t *testing.T) {
	r, f := initializationRunner(t)
	original := r.openFile
	r.openFile = func(path string) (*platformAdminCredentialFile, error) {
		file, err := original(path)
		if err != nil {
			return nil, err
		}
		sync := file.syncFile
		file.syncFile = func(current *os.File) error {
			if current == file.directory {
				return errors.New("directory unavailable")
			}
			return sync(current)
		}
		return file, nil
	}
	for attempt := 0; attempt < 2; attempt++ {
		if _, err := r.run(context.Background()); err == nil {
			t.Fatal("directory sync failure allowed commit")
		}
		assertInitializationRows(t, f, 0)
	}
	before := initializationFile(t, r.path)
	r.openFile = original
	receipt, err := r.run(context.Background())
	if err != nil || receipt.InitializationID != before.InitializationID {
		t.Fatalf("retry did not preserve intent: %v", err)
	}
	if after := initializationFile(t, r.path); after != before {
		t.Fatal("retry changed password file")
	}
	assertInitializationRows(t, f, 1)
}

type initializationCommitStarter struct {
	service.TxStarter
	applied bool
}
type initializationCommitTx struct {
	pgx.Tx
	applied bool
}

func (s initializationCommitStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return initializationCommitTx{tx, s.applied}, nil
}
func (t initializationCommitTx) Commit(ctx context.Context) error {
	if t.applied {
		if err := t.Tx.Commit(ctx); err != nil {
			return err
		}
	} else {
		if err := t.Tx.Rollback(ctx); err != nil {
			return err
		}
	}
	return errors.New("lost commit response")
}
func TestPlatformAdminInitializeReconcilesUncertainCommitWithoutReplay(t *testing.T) {
	for _, applied := range []bool{true, false} {
		t.Run(map[bool]string{true: "committed", false: "rolled_back"}[applied], func(t *testing.T) {
			r, f := initializationRunner(t)
			r.transactions = initializationCommitStarter{f.pool, applied}
			receipt, err := r.run(context.Background())
			c := initializationFile(t, r.path)
			if applied {
				if err != nil || receipt.Status != "already_initialized" || receipt.InitializationID != c.InitializationID {
					t.Fatalf("commit reconciliation: %+v %v", receipt, err)
				}
				assertInitializationRows(t, f, 1)
			} else {
				if err == nil {
					t.Fatal("rolled-back commit reported success")
				}
				assertInitializationRows(t, f, 0)
				r.transactions = f.pool
				receipt, err = r.run(context.Background())
				if err != nil || receipt.InitializationID != c.InitializationID {
					t.Fatal("explicit retry lost uncommitted intent")
				}
				assertInitializationRows(t, f, 1)
			}
		})
	}
}
func TestPlatformAdminInitializeAuditFailureLeavesReusableFile(t *testing.T) {
	r, f := initializationRunner(t)
	f.fx.Exec(t, "ALTER TABLE admin_audit_event ADD CONSTRAINT fail_audit CHECK (false) NOT VALID")
	if _, err := r.run(context.Background()); err == nil {
		t.Fatal("audit failure ignored")
	}
	assertInitializationRows(t, f, 0)
	before := initializationFile(t, r.path)
	f.fx.Exec(t, "ALTER TABLE admin_audit_event DROP CONSTRAINT fail_audit")
	receipt, err := r.run(context.Background())
	if err != nil || receipt.InitializationID != before.InitializationID {
		t.Fatal("database rollback replaced durable file")
	}
	assertInitializationRows(t, f, 1)
}
