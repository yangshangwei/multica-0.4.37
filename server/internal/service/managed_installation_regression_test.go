package service

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/installation"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func (f *installationFixture) bindingProof(t *testing.T, inst string) (installation.Proof, InstallationChallengeParams, context.Context) {
	t.Helper()
	ws := f.fx.Workspace(t, "Binding regression", uuid.NewString())
	f.fx.Member(t, ws, util.UUIDToString(f.user), "owner")
	f.fx.InsertNoID(t, "organization_workspace", testutil.Cols{"organization_id": f.org, "workspace_id": ws}, "workspace_id=$1", ws)
	p := InstallationChallengeParams{Purpose: "bind", InstallationID: inst, WorkspaceID: ws, DaemonID: uuid.NewString()}
	ctx := auth.WithPasswordSession(context.Background(), auth.PasswordSession{UserID: util.UUIDToString(f.user), Version: 1, Kind: "pat"})
	return f.challenge(t, p), p, ctx
}

func TestManagedInstallationAuditFailureRollsBackAndRetriesOriginalProof(t *testing.T) {
	f := newInstallationFixture(t)
	p := f.challenge(t, InstallationChallengeParams{Purpose: "enroll", DesktopVersion: "0.4.40", OS: "macos"})
	f.management.TxStarter = platformAdminFailAuditStarter{f.pool}
	if _, err := f.management.Enroll(f.ctx, p); err == nil {
		t.Fatal("enrollment ignored audit failure")
	}
	if f.fx.Count(t, "SELECT count(*) FROM managed_installation") != 0 || f.fx.Count(t, "SELECT count(*) FROM installation_challenge WHERE consumed_at IS NOT NULL") != 0 {
		t.Fatal("failed enrollment partially committed")
	}
	f.management.TxStarter = f.pool
	enrolled, err := f.management.Enroll(f.ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	proof, _, ctx := f.bindingProof(t, enrolled.InstallationID)
	f.management.TxStarter = platformAdminFailAuditStarter{f.pool}
	if _, err = f.management.Bind(ctx, proof); err == nil {
		t.Fatal("binding ignored audit failure")
	}
	if f.fx.Count(t, "SELECT count(*) FROM installation_daemon_binding") != 0 || f.fx.Count(t, "SELECT count(*) FROM daemon_token") != 0 {
		t.Fatal("failed binding partially committed")
	}
	f.management.TxStarter = f.pool
	if _, err = f.management.Bind(ctx, proof); err != nil {
		t.Fatalf("same proof retry failed: %v", err)
	}
}

func TestManagedInstallationConcurrentSharedKeyAndConflictingDaemon(t *testing.T) {
	f := newInstallationFixture(t)
	other := *f
	other.user = f.platformAdminFixture.user(t, "")
	other.ctx = auth.WithPasswordSession(context.Background(), auth.PasswordSession{UserID: util.UUIDToString(other.user), Version: 1, Kind: "jwt"})
	p1 := f.challenge(t, InstallationChallengeParams{Purpose: "enroll", DesktopVersion: "0.4.40", OS: "macos"})
	p2 := other.challenge(t, InstallationChallengeParams{Purpose: "enroll", DesktopVersion: "0.4.40", OS: "macos"})
	type response struct {
		result InstallationEnrollment
		err    error
	}
	results := make(chan response, 2)
	go func() { r, e := f.management.Enroll(f.ctx, p1); results <- response{r, e} }()
	go func() { r, e := f.management.Enroll(other.ctx, p2); results <- response{r, e} }()
	a, b := <-results, <-results
	if a.err != nil || b.err != nil || a.result.InstallationID != b.result.InstallationID || f.fx.Count(t, "SELECT count(*) FROM managed_installation") != 1 || f.fx.Count(t, "SELECT count(*) FROM installation_user") != 2 {
		t.Fatalf("shared physical key registration: %v %v", a.err, b.err)
	}
	second := *f
	var err error
	second.public, second.private, err = ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	newInst, err := second.management.Enroll(second.ctx, second.challenge(t, InstallationChallengeParams{Purpose: "enroll", DesktopVersion: "0.4.40", OS: "macos"}))
	if err != nil {
		t.Fatal(err)
	}
	firstProof, params, daemonCtx := f.bindingProof(t, a.result.InstallationID)
	params.InstallationID = newInst.InstallationID
	secondProof := second.challenge(t, params)
	errorsOut := make(chan error, 2)
	go func() { _, e := f.management.Bind(daemonCtx, firstProof); errorsOut <- e }()
	go func() { _, e := f.management.Bind(daemonCtx, secondProof); errorsOut <- e }()
	firstErr, secondErr := <-errorsOut, <-errorsOut
	if (firstErr == nil) == (secondErr == nil) || f.fx.Count(t, "SELECT count(*) FROM installation_daemon_binding WHERE state='active'") != 1 {
		t.Fatalf("conflicting keys both bound or both failed: %v %v", firstErr, secondErr)
	}
}

type installationWorkspaceWaitStarter struct {
	pool    *pgxpool.Pool
	reached chan struct{}
}

func (s installationWorkspaceWaitStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.pool.Begin(ctx)
	return installationWorkspaceWaitTx{Tx: tx, reached: s.reached}, err
}

type installationWorkspaceWaitTx struct {
	pgx.Tx
	reached chan struct{}
}

func (tx installationWorkspaceWaitTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if strings.Contains(sql, "-- name: LockWorkspaceForChatSessionCreate") {
		select {
		case tx.reached <- struct{}{}:
		default:
		}
	}
	return tx.Tx.QueryRow(ctx, sql, args...)
}

func TestManagedInstallationBindingDoesNotInvertWorkspaceDeletionLocks(t *testing.T) {
	f := newInstallationFixture(t)
	enrolled, err := f.management.Enroll(f.ctx, f.challenge(t, InstallationChallengeParams{Purpose: "enroll", DesktopVersion: "0.4.40", OS: "macos"}))
	if err != nil {
		t.Fatal(err)
	}
	proof, p, ctx := f.bindingProof(t, enrolled.InstallationID)
	deleteTx, err := f.pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer deleteTx.Rollback(context.Background())
	ws, err := util.ParseUUID(p.WorkspaceID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.New(deleteTx).LockWorkspaceForDelete(context.Background(), ws); err != nil {
		t.Fatal(err)
	}
	reached := make(chan struct{}, 1)
	f.management.TxStarter = installationWorkspaceWaitStarter{f.pool, reached}
	done := make(chan error, 1)
	go func() { _, e := f.management.Bind(ctx, proof); done <- e }()
	select {
	case <-reached:
	case <-time.After(5 * time.Second):
		t.Fatal("binding did not reach workspace fence")
	}
	deadline, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err = db.New(deleteTx).ExpireWorkspaceInstallationChallenges(deadline, ws); err != nil {
		t.Fatalf("deletion blocked behind a challenge held by binding: %v", err)
	}
	if err = deleteTx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	select {
	case err = <-done:
		if err == nil {
			t.Fatal("binding ignored expired deletion challenge")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("binding did not finish after deletion fence released")
	}
	if f.fx.Count(t, "SELECT count(*) FROM installation_daemon_binding") != 0 || f.fx.Count(t, "SELECT count(*) FROM daemon_token") != 0 {
		t.Fatal("deletion loser committed a binding")
	}
}

func TestManagedInstallationRenewalAuditAndMetadataRevocations(t *testing.T) {
	f := newInstallationFixture(t)
	enrolled, err := f.management.Enroll(f.ctx, f.challenge(t, InstallationChallengeParams{Purpose: "enroll", DesktopVersion: "0.4.40", OS: "macos"}))
	if err != nil {
		t.Fatal(err)
	}
	proof, p, ctx := f.bindingProof(t, enrolled.InstallationID)
	bound, err := f.management.Bind(ctx, proof)
	if err != nil {
		t.Fatal(err)
	}
	identity, err := auth.CheckPasswordToken(context.Background(), db.New(f.pool), bound.DaemonToken, true)
	if err != nil {
		t.Fatal(err)
	}
	boundCtx := auth.WithPasswordSession(context.Background(), identity.Session)
	p.Purpose = "renew"
	p.ExpectedBindingEpoch = &bound.BindingEpoch
	p.PublicKey = base64.RawURLEncoding.EncodeToString(f.public)
	challenge, err := f.management.IssueChallenge(boundCtx, p)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := base64.RawURLEncoding.DecodeString(challenge.SignaturePayload)
	renewProof := installation.Proof{ChallengeID: challenge.ChallengeID, SignaturePayload: challenge.SignaturePayload, BodyPayload: challenge.BodyPayload, Signature: base64.RawURLEncoding.EncodeToString(ed25519.Sign(f.private, raw))}
	f.management.TxStarter = platformAdminFailAuditStarter{f.pool}
	if _, err = f.management.Renew(boundCtx, renewProof); err == nil {
		t.Fatal("renewal ignored audit failure")
	}
	if f.fx.Count(t, "SELECT count(*) FROM daemon_token") != 1 {
		t.Fatal("renewal leaked a new credential before audit")
	}
	f.management.TxStarter = f.pool
	if _, err = f.management.Renew(boundCtx, renewProof); err != nil {
		t.Fatal(err)
	}
	session, _ := auth.PasswordSessionFromContext(f.ctx)
	if _, err = auth.ValidateInstallationMetadata(f.ctx, db.New(f.pool), enrolled.MetadataProof, session); err != nil {
		t.Fatal(err)
	}
	f.fx.Exec(t, "UPDATE managed_installation SET key_version=key_version+1 WHERE id=$1", enrolled.InstallationID)
	if _, err = auth.ValidateInstallationMetadata(f.ctx, db.New(f.pool), enrolled.MetadataProof, session); !errors.Is(err, auth.ErrInstallationMetadata) {
		t.Fatalf("rotated key metadata remained valid: %v", err)
	}
	f.fx.Exec(t, "UPDATE managed_installation SET key_version=1 WHERE id=$1", enrolled.InstallationID)
	t.Setenv("MULTICA_DEPLOYMENT_ID", "00000000-0000-4000-8000-000000000008")
	if _, err = auth.ValidateInstallationMetadata(f.ctx, db.New(f.pool), enrolled.MetadataProof, session); !errors.Is(err, auth.ErrInstallationMetadata) {
		t.Fatal("proof crossed deployment")
	}
	t.Setenv("MULTICA_DEPLOYMENT_ID", f.management.DeploymentID)
	f.fx.Exec(t, "UPDATE user_password_credential SET session_version=2 WHERE user_id=$1", f.user)
	if _, err = auth.ValidateInstallationMetadata(f.ctx, db.New(f.pool), enrolled.MetadataProof, session); !errors.Is(err, auth.ErrPasswordSession) {
		t.Fatal("proof survived account revocation")
	}
}
