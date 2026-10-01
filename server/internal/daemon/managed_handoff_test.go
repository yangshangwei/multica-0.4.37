package daemon

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func managedHandoffFixture(t *testing.T) (*ManagedInstallation, ManagedStartupHandoff) {
	t.Helper()
	id, err := LoadManagedInstallation(t.TempDir(), managedDeployment)
	if err != nil {
		t.Fatal(err)
	}
	if err = id.SaveEnrollment(managedInstallation, "1"); err != nil {
		t.Fatal(err)
	}
	workspace := "88888888-8888-4888-8888-888888888888"
	body, _ := json.Marshal(struct {
		InstallationID string  `json:"installation_id"`
		WorkspaceID    string  `json:"workspace_id"`
		DaemonID       string  `json:"daemon_id"`
		PublicKey      string  `json:"public_key"`
		Epoch          *string `json:"expected_binding_epoch"`
	}{managedInstallation, workspace, id.ManagedDaemonID("33333333-3333-4333-8333-333333333333"), id.PublicKeyBase64(), nil})
	digest := sha256.Sum256(body)
	installation := managedInstallation
	daemonID := id.ManagedDaemonID("33333333-3333-4333-8333-333333333333")
	payload := managedSignaturePayload{ProtocolVersion: "1", Purpose: "bind", ChallengeID: "55555555-5555-4555-8555-555555555555", DeploymentID: managedDeployment, OrganizationID: "44444444-4444-4444-8444-444444444444", UserID: "33333333-3333-4333-8333-333333333333", AuthVersion: "1", InstallationID: &installation, WorkspaceID: &workspace, DaemonID: &daemonID, PublicKeyFingerprint: id.Fingerprint(), Nonce: base64.RawURLEncoding.EncodeToString(make([]byte, 32)), ExpiresAt: "2000000060", Method: "POST", Path: "/api/daemon/installation-bindings", BodySHA256: hex.EncodeToString(digest[:])}
	raw, _ := json.Marshal(payload)
	challenge := InstallationChallenge{ChallengeID: payload.ChallengeID, Nonce: payload.Nonce, DeploymentID: managedDeployment, ExpiresAt: payload.ExpiresAt, SignaturePayload: base64.RawURLEncoding.EncodeToString(raw), BodyPayload: base64.RawURLEncoding.EncodeToString(body)}
	proof, err := id.SignChallenge(challenge, ManagedChallengeScope{Purpose: "bind", DeploymentID: managedDeployment, OrganizationID: payload.OrganizationID, UserID: payload.UserID, AuthVersion: "1", InstallationID: installation, WorkspaceID: workspace, DaemonID: daemonID, Method: "POST", Path: payload.Path, BodyPayload: challenge.BodyPayload}, time.Unix(2_000_000_000, 0))
	if err != nil {
		t.Fatal(err)
	}
	return id, ManagedStartupHandoff{Version: "1", ServerURL: "https://managed.example", DeploymentID: managedDeployment, OrganizationID: payload.OrganizationID, UserID: payload.UserID, AuthVersion: "1", InstallationID: installation, ManagedDaemonID: daemonID, Bindings: []ManagedBindingHandoff{{WorkspaceID: workspace, Challenge: challenge, Proof: proof}}}
}

func TestManagedHandoffOriginalBytesAndScope(t *testing.T) {
	id, handoff := managedHandoffFixture(t)
	raw, _ := json.Marshal(handoff)
	original, parsed, err := ReadManagedStartupHandoff(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(original, raw) {
		t.Fatal("handoff bytes rewritten")
	}
	if err = parsed.Validate(id, "https://managed.example", time.Unix(2_000_000_000, 0)); err != nil {
		t.Fatal(err)
	}
	if err = parsed.Validate(id, "https://another.example", time.Unix(2_000_000_000, 0)); err == nil {
		t.Fatal("wrong deployment URL accepted")
	}
	parsed.UserID = parsed.OrganizationID
	if err = parsed.Validate(id, "https://managed.example", time.Unix(2_000_000_000, 0)); err == nil {
		t.Fatal("wrong principal accepted")
	}
}

func TestManagedHandoffRejectsOversizeUnknownAndDuplicateScopes(t *testing.T) {
	if _, _, err := ReadManagedStartupHandoff(strings.NewReader(strings.Repeat("x", MaxManagedHandoffBytes+1))); err == nil {
		t.Fatal("oversized handoff accepted")
	}
	_, handoff := managedHandoffFixture(t)
	handoff.Bindings = append(handoff.Bindings, handoff.Bindings[0])
	raw, _ := json.Marshal(handoff)
	if _, _, err := ReadManagedStartupHandoff(bytes.NewReader(raw)); err == nil {
		t.Fatal("duplicate workspace accepted")
	}
	raw = []byte(strings.Replace(string(raw), `"version":"1"`, `"human_jwt":"secret","version":"1"`, 1))
	if _, _, err := ReadManagedStartupHandoff(bytes.NewReader(raw)); err == nil {
		t.Fatal("unexpected secret field accepted")
	}
}
