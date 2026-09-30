package service

import (
	"context"
	"fmt"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"os"
	"testing"
	"time"
)

func TestPasswordCallbackCapturesAuthorizedVersion(t *testing.T) {
	t.Setenv("MULTICA_AUTH_MODE", "password")
	id := testInstallationID(t)
	callbacks := NewCallbackTokens()
	ctx := auth.WithPasswordSession(context.Background(), auth.PasswordSession{UserID: uuidString(id), Version: 7, Kind: "jwt"})
	token, err := callbacks.Issue(ctx, HookInvocation{Actor: HookActor{Type: "member", ID: id}})
	if err != nil {
		t.Fatal(err)
	}
	grant, err := callbacks.Resolve(token)
	if err != nil || grant.AuthVersion != 7 {
		t.Fatalf("grant lost source version: %+v %v", grant, err)
	}
}

func TestPasswordCallbackRejectsMissingOrRestrictedSource(t *testing.T) {
	t.Setenv("MULTICA_AUTH_MODE", "password")
	id := testInstallationID(t)
	for _, source := range []auth.PasswordSession{{}, {UserID: uuidString(id), Version: 1, Setup: true}, {UserID: uuidString(id), Version: 1, Change: true}, {UserID: "other", Version: 1}} {
		ctx := auth.WithPasswordSession(context.Background(), source)
		if _, err := NewCallbackTokens().Issue(ctx, HookInvocation{Actor: HookActor{Type: "member", ID: id}}); err == nil {
			t.Fatalf("accepted invalid source %+v", source)
		}
	}
	if _, err := NewCallbackTokens().Issue(context.Background(), HookInvocation{Actor: HookActor{Type: "plugin", ID: id}}); err != nil {
		t.Fatalf("independent plugin actor blocked: %v", err)
	}
}

func TestPasswordCallbackMintRejectsStaleSourceAfterReset(t *testing.T) {
	t.Setenv("MULTICA_AUTH_MODE", "password")
	if os.Getenv("DATABASE_URL") == "" {
		t.Skip("requires isolated DATABASE_URL")
	}
	pool := newPluginStoragePool(t)
	fx := testutil.New(pool, "", "")
	id := fx.User(t, "Callback fence", fmt.Sprintf("callback-fence-%d@example.com", time.Now().UnixNano()))
	fx.InsertNoID(t, "user_password_credential", testutil.Cols{"user_id": id, "username": fmt.Sprintf("callback%d", time.Now().UnixNano()), "password_hash": "unused-test-hash", "session_version": int64(2)}, "user_id=$1", id)
	parsed, err := parseUUIDValue(id)
	if err != nil {
		t.Fatal(err)
	}
	s := &PluginService{Queries: db.New(pool), TxStarter: pool, Callbacks: NewCallbackTokens()}
	ctx := auth.WithPasswordSession(context.Background(), auth.PasswordSession{UserID: id, Version: 1, Kind: "jwt"})
	invocation := HookInvocation{Actor: HookActor{Type: "member", ID: parsed}}
	if _, err := s.issueCallbackToken(ctx, invocation); err == nil {
		t.Fatal("stale authorization minted a new callback")
	}
	// A mint already waiting for the user row must still check the old source
	// version after the password transaction commits its replacement.
	tx, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if _, err = db.New(tx).LockPasswordUser(context.Background(), parsed); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(context.Background(), "UPDATE user_password_credential SET session_version=3 WHERE user_id=$1", parsed); err != nil {
		t.Fatal(err)
	}
	waitingCtx := auth.WithPasswordSession(context.Background(), auth.PasswordSession{UserID: id, Version: 2, Kind: "jwt"})
	result := make(chan error, 1)
	go func() { _, err := s.issueCallbackToken(waitingCtx, invocation); result <- err }()
	if err = tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	select {
	case err = <-result:
		if err == nil {
			t.Fatal("waiting mint upgraded the previous session")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("callback mint remained blocked after password transaction")
	}

	ctx = auth.WithPasswordSession(context.Background(), auth.PasswordSession{UserID: id, Version: 3, Kind: "jwt"})
	token, err := s.issueCallbackToken(ctx, invocation)
	if err != nil {
		t.Fatal(err)
	}
	grant, err := s.Callbacks.Resolve(token)
	if err != nil || grant.AuthVersion != 3 {
		t.Fatalf("fresh grant: %+v %v", grant, err)
	}
}
