package daemon

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

const managedDeployment = "11111111-1111-4111-8111-111111111111"
const managedInstallation = "22222222-2222-4222-8222-222222222222"

func TestManagedIdentityConcurrentPersistence(t *testing.T) {
	home := t.TempDir()
	var wg sync.WaitGroup
	keys := make(chan string, 8)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id, err := LoadManagedInstallation(home, managedDeployment)
			if err != nil {
				t.Error(err)
				return
			}
			keys <- id.PublicKeyBase64()
		}()
	}
	wg.Wait()
	close(keys)
	first := ""
	for key := range keys {
		if first == "" {
			first = key
		}
		if key != first {
			t.Fatal("concurrent creation produced multiple identities")
		}
	}
	id, err := LoadManagedInstallation(home, managedDeployment)
	if err != nil {
		t.Fatal(err)
	}
	if err = id.SaveEnrollment(managedInstallation, "1"); err != nil {
		t.Fatal(err)
	}
	again, err := LoadManagedInstallation(home, managedDeployment)
	if err != nil {
		t.Fatal(err)
	}
	if again.InstallationID() != managedInstallation || again.PublicKeyBase64() != first {
		t.Fatal("enrollment identity did not persist")
	}
	other, err := LoadManagedInstallation(t.TempDir(), managedDeployment)
	if err != nil {
		t.Fatal(err)
	}
	if other.PublicKeyBase64() == first || other.ManagedDaemonID("33333333-3333-4333-8333-333333333333") == id.ManagedDaemonID("33333333-3333-4333-8333-333333333333") {
		t.Fatal("different OS homes reused identity")
	}
}

func TestManagedIdentityRejectsCorruption(t *testing.T) {
	home := t.TempDir()
	_, err := LoadManagedInstallation(home, managedDeployment)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(home, ".multica", "management", managedDeployment, "installation.json")
	if err = os.WriteFile(path, []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = LoadManagedInstallation(home, managedDeployment); err == nil {
		t.Fatal("corrupt key was silently replaced")
	}
	raw, _ := os.ReadFile(path)
	if string(raw) != "corrupt" {
		t.Fatal("corrupt file changed")
	}
}

func TestManagedIdentitySignsOriginalScope(t *testing.T) {
	id, err := LoadManagedInstallation(t.TempDir(), managedDeployment)
	if err != nil {
		t.Fatal(err)
	}
	body := []byte(fmt.Sprintf(`{"public_key":%q,"desktop_version":"test","os":"linux"}`, id.PublicKeyBase64()))
	digest := sha256.Sum256(body)
	payload := managedSignaturePayload{
		ProtocolVersion: "1", Purpose: "enroll", ChallengeID: "55555555-5555-4555-8555-555555555555", DeploymentID: managedDeployment,
		OrganizationID: "44444444-4444-4444-8444-444444444444", UserID: "33333333-3333-4333-8333-333333333333", AuthVersion: "1",
		PublicKeyFingerprint: id.Fingerprint(), Nonce: base64.RawURLEncoding.EncodeToString(make([]byte, 32)), ExpiresAt: "2000000060", Method: "POST", Path: "/api/installations/enroll", BodySHA256: hex.EncodeToString(digest[:]),
	}
	raw, _ := json.Marshal(payload)
	challenge := InstallationChallenge{ChallengeID: payload.ChallengeID, Nonce: payload.Nonce, DeploymentID: managedDeployment, ExpiresAt: payload.ExpiresAt, SignaturePayload: base64.RawURLEncoding.EncodeToString(raw), BodyPayload: base64.RawURLEncoding.EncodeToString(body)}
	scope := ManagedChallengeScope{Purpose: "enroll", DeploymentID: managedDeployment, OrganizationID: payload.OrganizationID, UserID: payload.UserID, AuthVersion: "1", Method: "POST", Path: payload.Path, BodyPayload: challenge.BodyPayload}
	proof, err := id.SignChallenge(challenge, scope, time.Unix(2_000_000_000, 0))
	if err != nil {
		t.Fatal(err)
	}
	public, _ := base64.RawURLEncoding.DecodeString(id.PublicKeyBase64())
	signature, _ := base64.RawURLEncoding.DecodeString(proof.Signature)
	if !ed25519.Verify(public, raw, signature) {
		t.Fatal("signature did not cover original payload bytes")
	}
	scope.AuthVersion = "2"
	if _, err = id.SignChallenge(challenge, scope, time.Unix(2_000_000_000, 0)); err == nil {
		t.Fatal("wrong account version accepted")
	}
	scope.AuthVersion = "1"
	if _, err = id.SignChallenge(challenge, scope, time.Unix(2_000_000_061, 0)); err == nil {
		t.Fatal("expired proof accepted")
	}
}

func TestManagedIdentitySharedNodeFixture(t *testing.T) {
	home := t.TempDir()
	if _, err := LoadManagedInstallation(home, managedDeployment); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("testdata/managed-identity-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(home, ".multica", "management", managedDeployment, "installation.json")
	if err = os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	id, err := LoadManagedInstallation(home, managedDeployment)
	if err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile("testdata/managed-challenge-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Challenge InstallationChallenge `json:"challenge"`
		Signature string                `json:"signature"`
	}
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	scope := ManagedChallengeScope{Purpose: "enroll", DeploymentID: managedDeployment, OrganizationID: "44444444-4444-4444-8444-444444444444", UserID: "33333333-3333-4333-8333-333333333333", AuthVersion: "1", Method: "POST", Path: "/api/installations/enroll", BodyPayload: fixture.Challenge.BodyPayload}
	proof, err := id.SignChallenge(fixture.Challenge, scope, time.Unix(2_000_000_000, 0))
	if err != nil {
		t.Fatal(err)
	}
	if proof.Signature != fixture.Signature {
		t.Fatal("Go signature differs from shared Node fixture")
	}
	raw, err = os.ReadFile("testdata/managed-daemon-id-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	var vector struct {
		Namespace string `json:"namespace"`
		UserID    string `json:"user_id"`
		DaemonID  string `json:"daemon_id"`
	}
	if err = json.Unmarshal(raw, &vector); err != nil {
		t.Fatal(err)
	}
	if id.ManagedDaemonID(vector.UserID) != vector.DaemonID {
		t.Fatal("managed daemon derivation differs from Node fixture")
	}
	if id.ManagedDaemonID("44444444-4444-4444-8444-444444444444") == vector.DaemonID {
		t.Fatal("another user inherited daemon namespace")
	}
}
