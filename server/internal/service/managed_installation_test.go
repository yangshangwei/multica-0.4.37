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
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/installation"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type installationFixture struct {
	*platformAdminFixture
	management *ManagedInstallationService
	user       pgtype.UUID
	ctx        context.Context
	public     ed25519.PublicKey
	private    ed25519.PrivateKey
}

func newInstallationFixture(t *testing.T) *installationFixture {
	f := newPlatformAdminFixture(t)
	t.Setenv("MULTICA_DEPLOYMENT_ID", "00000000-0000-4000-8000-000000000009")
	for _, table := range []string{"workspace", "member", "managed_installation", "installation_user", "installation_daemon_binding", "installation_challenge", "installation_report_cursor"} {
		f.fx.Exec(t, "CREATE TABLE "+pgx.Identifier{table}.Sanitize()+" (LIKE "+pgx.Identifier{"public", table}.Sanitize()+" INCLUDING ALL)")
	}
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	user := f.user(t, "")
	ctx := auth.WithPasswordSession(context.Background(), auth.PasswordSession{UserID: util.UUIDToString(user), Version: 1, Kind: "jwt"})
	svc := NewManagedInstallationService(db.New(f.pool), f.pool, "00000000-0000-4000-8000-000000000009", true)
	return &installationFixture{f, svc, user, ctx, public, private}
}

func (f *installationFixture) challenge(t *testing.T, p InstallationChallengeParams) installation.Proof {
	t.Helper()
	p.PublicKey = base64.RawURLEncoding.EncodeToString(f.public)
	c, err := f.management.IssueChallenge(f.ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := base64.RawURLEncoding.DecodeString(c.SignaturePayload)
	if err != nil {
		t.Fatal(err)
	}
	return installation.Proof{ChallengeID: c.ChallengeID, SignaturePayload: c.SignaturePayload, BodyPayload: c.BodyPayload, Signature: base64.RawURLEncoding.EncodeToString(ed25519.Sign(f.private, raw))}
}

func TestManagedInstallationEnrollmentIsScopedAndIdempotent(t *testing.T) {
	f := newInstallationFixture(t)
	proof := f.challenge(t, InstallationChallengeParams{Purpose: "enroll", DesktopVersion: "0.4.40", OS: "macos"})
	first, err := f.management.Enroll(f.ctx, proof)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := f.management.Enroll(f.ctx, proof)
	if err != nil || replay.InstallationID != first.InstallationID {
		t.Fatalf("replay: %+v %v", replay, err)
	}
	if f.fx.Count(t, "SELECT count(*) FROM managed_installation") != 1 || f.fx.Count(t, "SELECT count(*) FROM installation_user") != 1 || f.fx.Count(t, "SELECT count(*) FROM admin_audit_event") != 1 {
		t.Fatal("replay duplicated enrollment effects")
	}
	other := f.platformAdminFixture.user(t, "")
	otherCtx := auth.WithPasswordSession(context.Background(), auth.PasswordSession{UserID: util.UUIDToString(other), Version: 1, Kind: "jwt"})
	if _, err = f.management.Enroll(otherCtx, proof); err == nil {
		t.Fatal("proof crossed account scope")
	}
	patCtx := auth.WithPasswordSession(context.Background(), auth.PasswordSession{UserID: util.UUIDToString(f.user), Version: 1, Kind: "pat"})
	if _, err = f.management.IssueChallenge(patCtx, InstallationChallengeParams{Purpose: "enroll", PublicKey: base64.RawURLEncoding.EncodeToString(f.public)}); err == nil {
		t.Fatal("PAT issued human enrollment challenge")
	}
	f.fx.Exec(t, "UPDATE user_password_credential SET session_version=2 WHERE user_id=$1", f.user)
	if _, err = f.management.Enroll(f.ctx, proof); !errors.Is(err, auth.ErrPasswordSession) {
		t.Fatalf("revoked source replay: %v", err)
	}
}

func TestManagedInstallationRejectsExpiredAndForeignDeploymentProof(t *testing.T) {
	f := newInstallationFixture(t)
	now := time.Now().UTC()
	f.management.Now = func() time.Time { return now }
	p := f.challenge(t, InstallationChallengeParams{Purpose: "enroll", DesktopVersion: "0.4.40", OS: "macos"})
	f.management.Now = func() time.Time { return now.Add(3 * time.Minute) }
	if _, err := f.management.Enroll(f.ctx, p); err == nil {
		t.Fatal("expired challenge enrolled")
	}
	f.management.Now = func() time.Time { return now }
	f.management.DeploymentID = "00000000-0000-4000-8000-000000000008"
	if _, err := f.management.Enroll(f.ctx, p); err == nil {
		t.Fatal("challenge crossed deployment")
	}
	if f.fx.Count(t, "SELECT count(*) FROM managed_installation") != 0 {
		t.Fatal("failed proof persisted installation")
	}
}

func TestManagedInstallationBindingRequiresSamePrincipalAndRealHistoricalProof(t *testing.T) {
	f := newInstallationFixture(t)
	result, err := f.management.Enroll(f.ctx, f.challenge(t, InstallationChallengeParams{Purpose: "enroll", DesktopVersion: "0.4.40", OS: "macos"}))
	if err != nil {
		t.Fatal(err)
	}
	ws := f.fx.Workspace(t, "Installation workspace", "installation-test")
	f.fx.Member(t, ws, util.UUIDToString(f.user), "owner")
	f.fx.InsertNoID(t, "organization_workspace", testutil.Cols{"organization_id": f.org, "workspace_id": ws}, "workspace_id=$1", ws)
	daemonID := uuid.NewString()
	proof := f.challenge(t, InstallationChallengeParams{Purpose: "bind", InstallationID: result.InstallationID, WorkspaceID: ws, DaemonID: daemonID})
	pat := auth.WithPasswordSession(context.Background(), auth.PasswordSession{UserID: util.UUIDToString(f.user), Version: 1, Kind: "pat"})
	bound, err := f.management.Bind(pat, proof)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := f.management.Bind(pat, proof)
	if err != nil || replay.BindingID != bound.BindingID || replay.DaemonToken != bound.DaemonToken {
		t.Fatalf("binding replay lost result: %+v %v", replay, err)
	}
	if !strings.HasPrefix(bound.DaemonToken, "mdt_") || f.fx.Count(t, "SELECT count(*) FROM installation_daemon_binding") != 1 || f.fx.Count(t, "SELECT count(*) FROM daemon_token") != 1 {
		t.Fatal("binding credential not scoped/idempotent")
	}
	identity, err := auth.CheckPasswordToken(context.Background(), db.New(f.pool), bound.DaemonToken, true)
	if err != nil || identity.Session.BindingID != bound.BindingID || identity.Session.WorkspaceID != ws || identity.Session.DaemonID != daemonID {
		t.Fatalf("token lost binding scope: %+v %v", identity.Session, err)
	}
	f.fx.Exec(t, "UPDATE installation_daemon_binding SET state='revoked' WHERE id=$1", bound.BindingID)
	if _, err = auth.CheckPasswordToken(context.Background(), db.New(f.pool), bound.DaemonToken, true); !errors.Is(err, auth.ErrPasswordSession) {
		t.Fatalf("revoked binding token still valid: %v", err)
	}
	if _, err = auth.CheckPasswordVersion(auth.WithPasswordSession(context.Background(), identity.Session), db.New(f.pool), identity.Session.UserID, identity.Session.Version); !errors.Is(err, auth.ErrPasswordSession) {
		t.Fatalf("in-flight bound session survived revocation: %v", err)
	}
	oldID := uuid.NewString()
	f.fx.Runtime(t, "Historical runtime", testutil.Cols{"workspace_id": ws, "owner_id": f.user, "daemon_id": oldID})
	oldProof := f.challenge(t, InstallationChallengeParams{Purpose: "bind", InstallationID: result.InstallationID, WorkspaceID: ws, DaemonID: oldID})
	if _, err = f.management.Bind(pat, oldProof); err == nil {
		t.Fatal("PAT adopted historical runtime by asserted daemon id")
	}
	other := f.platformAdminFixture.user(t, "")
	wrong := auth.WithPasswordSession(context.Background(), auth.PasswordSession{UserID: util.UUIDToString(other), Version: 1, Kind: "pat"})
	if _, err = f.management.Bind(wrong, proof); err == nil {
		t.Fatal("different daemon principal accepted desktop proof")
	}
}
