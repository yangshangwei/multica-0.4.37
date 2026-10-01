package service

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"context"
	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/installation"
	"github.com/multica-ai/multica/server/internal/util"
)

func TestManagedInstallationHeartbeatCannotReplayOrCrossAccount(t *testing.T) {
	f := newInstallationFixture(t)
	inst, err := f.management.Enroll(f.ctx, f.challenge(t, InstallationChallengeParams{Purpose: "enroll", DesktopVersion: "0.4.40", OS: "macos"}))
	if err != nil {
		t.Fatal(err)
	}
	p := installation.Heartbeat{ProtocolVersion: "1", Purpose: "heartbeat", DeploymentID: f.management.DeploymentID, InstallationID: inst.InstallationID, UserID: util.UUIDToString(f.user), AuthVersion: "1", BootID: uuid.NewString(), Sequence: "1", ReportedAt: strconv.FormatInt(time.Now().Unix(), 10), DesktopVersion: "0.4.40", OS: "macos", Method: "POST", Path: "/api/installations/" + inst.InstallationID + "/heartbeat"}
	sign := func() installation.HeartbeatProof {
		raw, _ := json.Marshal(p)
		return installation.HeartbeatProof{SignaturePayload: base64.RawURLEncoding.EncodeToString(raw), Signature: base64.RawURLEncoding.EncodeToString(ed25519.Sign(f.private, raw))}
	}
	first, err := f.management.Heartbeat(f.ctx, inst.InstallationID, sign())
	if err != nil || !first.Accepted {
		t.Fatalf("valid report: %+v %v", first, err)
	}
	f.fx.Exec(t, "UPDATE managed_installation SET client_seen_at=now()-interval '10 minutes' WHERE id=$1", inst.InstallationID)
	replay, err := f.management.Heartbeat(f.ctx, inst.InstallationID, sign())
	if err != nil || replay.Accepted {
		t.Fatalf("replay: %+v %v", replay, err)
	}
	if f.fx.Count(t, "SELECT count(*) FROM managed_installation WHERE id=$1 AND client_seen_at<now()-interval '9 minutes'", inst.InstallationID) != 1 {
		t.Fatal("replay refreshed liveness")
	}
	// A current credential version can safely continue the same client boot;
	// old-version requests still fail the account-version fence.
	f.fx.Exec(t, "UPDATE user_password_credential SET session_version=2 WHERE user_id=$1", f.user)
	f.ctx = auth.WithPasswordSession(context.Background(), auth.PasswordSession{UserID: util.UUIDToString(f.user), Version: 2, Kind: "jwt"})
	p.AuthVersion = "2"
	p.Sequence = "2"
	advanced, err := f.management.Heartbeat(f.ctx, inst.InstallationID, sign())
	if err != nil || !advanced.Accepted {
		t.Fatalf("fresh login could not advance prior boot cursor: %+v %v", advanced, err)
	}
	p.UserID = uuid.NewString()
	p.Sequence = "2"
	if _, err = f.management.Heartbeat(f.ctx, inst.InstallationID, sign()); err == nil {
		t.Fatal("key proof replaced session principal")
	}
	p.UserID = util.UUIDToString(f.user)
	p.ReportedAt = strconv.FormatInt(time.Now().Add(-4*time.Minute).Unix(), 10)
	if _, err = f.management.Heartbeat(f.ctx, inst.InstallationID, sign()); err == nil {
		t.Fatal("stale signed report refreshed liveness")
	}
}
