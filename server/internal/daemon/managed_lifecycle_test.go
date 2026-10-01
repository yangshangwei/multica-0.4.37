package daemon

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"github.com/google/uuid"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func currentManagedChallenge(t *testing.T, id *ManagedInstallation, handoff ManagedStartupHandoff, purpose, workspace, epoch string) (InstallationChallenge, InstallationProof) {
	t.Helper()
	var epochPointer *string
	if epoch != "" {
		epochPointer = &epoch
	}
	body, _ := json.Marshal(struct {
		InstallationID string  `json:"installation_id"`
		WorkspaceID    string  `json:"workspace_id"`
		DaemonID       string  `json:"daemon_id"`
		PublicKey      string  `json:"public_key"`
		Epoch          *string `json:"expected_binding_epoch"`
	}{handoff.InstallationID, workspace, handoff.ManagedDaemonID, id.PublicKeyBase64(), epochPointer})
	digest := sha256.Sum256(body)
	path := "/api/daemon/installation-bindings"
	if purpose == "renew" {
		path += "/renew"
	}
	payload := managedSignaturePayload{ProtocolVersion: "1", Purpose: purpose, ChallengeID: uuid.NewString(), DeploymentID: handoff.DeploymentID, OrganizationID: handoff.OrganizationID, UserID: handoff.UserID, AuthVersion: handoff.AuthVersion, InstallationID: &handoff.InstallationID, WorkspaceID: &workspace, DaemonID: &handoff.ManagedDaemonID, PublicKeyFingerprint: id.Fingerprint(), ExpectedBindingEpoch: epochPointer, Nonce: base64.RawURLEncoding.EncodeToString(make([]byte, 32)), ExpiresAt: strconv.FormatInt(time.Now().Add(2*time.Minute).Unix(), 10), Method: "POST", Path: path, BodySHA256: hex.EncodeToString(digest[:])}
	raw, _ := json.Marshal(payload)
	challenge := InstallationChallenge{ChallengeID: payload.ChallengeID, Nonce: payload.Nonce, DeploymentID: payload.DeploymentID, ExpiresAt: payload.ExpiresAt, SignaturePayload: base64.RawURLEncoding.EncodeToString(raw), BodyPayload: base64.RawURLEncoding.EncodeToString(body)}
	proof, err := id.SignChallenge(challenge, ManagedChallengeScope{Purpose: purpose, DeploymentID: handoff.DeploymentID, OrganizationID: handoff.OrganizationID, UserID: handoff.UserID, AuthVersion: handoff.AuthVersion, InstallationID: handoff.InstallationID, WorkspaceID: workspace, DaemonID: handoff.ManagedDaemonID, ExpectedBindingEpoch: epoch, Method: "POST", Path: path, BodyPayload: challenge.BodyPayload}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return challenge, proof
}

func TestManagedBootstrapPersistsAndRenewsWithoutDesktop(t *testing.T) {
	identity, handoff := managedHandoffFixture(t)
	home := filepath.Dir(filepath.Dir(filepath.Dir(identity.directory)))
	profile := "desktop-managed-test"
	directory, err := managedProfileDirectory(home, profile)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(directory, "config.json"), []byte(`{"token":"mul_discovery"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(directory, ".desktop-user-id"), []byte(handoff.UserID), 0600); err != nil {
		t.Fatal(err)
	}
	workspace := handoff.Bindings[0].WorkspaceID
	challenge, proof := currentManagedChallenge(t, identity, handoff, "bind", workspace, "")
	handoff.Bindings[0].Challenge = challenge
	handoff.Bindings[0].Proof = proof
	var renewals atomic.Int32
	var rejectRenewal atomic.Bool
	var dropBindingReply atomic.Bool
	var remotelyBound atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var response any
		switch r.URL.Path {
		case "/api/config":
			response = map[string]string{"auth_mode": "password", "deployment_id": handoff.DeploymentID}
		case "/api/daemon/workspaces", "/api/workspaces":
			response = []WorkspaceInfo{{ID: workspace}}
		case "/api/me":
			response = map[string]string{"id": handoff.UserID}
		case "/api/daemon/installations/challenges":
			if rejectRenewal.Load() {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			if r.Header.Get("Authorization") != "Bearer mdt_original" {
				t.Error("renew challenge did not use scoped credential")
			}
			challenge, _ := currentManagedChallenge(t, identity, handoff, "renew", workspace, "1")
			response = challenge
		case "/api/daemon/installation-bindings", "/api/daemon/installation-bindings/renew":
			marker, readErr := os.ReadFile(filepath.Join(directory, "config.json"))
			if readErr != nil || !bytes.Contains(marker, []byte("management_deployment_id")) {
				t.Error("management marker was not persisted before remote binding")
			}
			var received InstallationProof
			if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
				t.Error(err)
				w.WriteHeader(400)
				return
			}
			public, _ := base64.RawURLEncoding.DecodeString(identity.PublicKeyBase64())
			raw, _ := base64.RawURLEncoding.DecodeString(received.SignaturePayload)
			signature, _ := base64.RawURLEncoding.DecodeString(received.Signature)
			if !ed25519.Verify(public, raw, signature) {
				t.Error("binding did not carry original installation signature")
			}
			token := "mdt_original"
			if strings.HasSuffix(r.URL.Path, "/renew") {
				if r.Header.Get("Authorization") != "Bearer mdt_original" {
					t.Error("renew redeemed using discovery PAT")
				}
				token = "mdt_renewed"
				renewals.Add(1)
			} else if r.Header.Get("Authorization") != "Bearer mul_discovery" {
				t.Error("initial bind did not use daemon's original credential")
			}
			if !strings.HasSuffix(r.URL.Path, "/renew") && dropBindingReply.Load() {
				remotelyBound.Store(true)
				panic(http.ErrAbortHandler)
			}
			response = ManagedDaemonCredential{BindingID: "99999999-9999-4999-8999-999999999999", BindingEpoch: "1", CapabilityVersion: "1", DaemonToken: token, ExpiresAt: time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339), PrincipalUserID: handoff.UserID, AuthVersion: handoff.AuthVersion}
		default:
			t.Errorf("unexpected managed fixture request %s", r.URL.Path)
			w.WriteHeader(404)
			return
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()
	handoff.ServerURL = server.URL
	config := Config{ServerBaseURL: server.URL, ManagementDeploymentID: handoff.DeploymentID, ManagedHandoff: &handoff, DaemonID: handoff.ManagedDaemonID, Profile: profile, WorkspacesRoot: filepath.Join(home, "workspaces")}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	t.Run("unknown_remote_commit_cannot_restart_as_legacy", func(t *testing.T) {
		dropBindingReply.Store(true)
		lost := New(config, logger)
		lost.client.SetToken("mul_discovery")
		if err := lost.initializeManagedTransport(context.Background(), home); err == nil || !remotelyBound.Load() {
			t.Fatal("fixture did not lose a committed binding reply")
		}
		marker, err := os.ReadFile(filepath.Join(directory, "config.json"))
		if err != nil || !bytes.Contains(marker, []byte("management_deployment_id")) {
			t.Fatal("crash erased managed profile marker")
		}
		restartConfig := config
		restartConfig.ManagedHandoff = nil
		restarted := New(restartConfig, logger)
		restarted.client.SetToken("mul_discovery")
		if err := restarted.initializeManagedTransport(context.Background(), home); err == nil {
			t.Fatal("unknown binding accepted a no-proof restart")
		}
		if restarted.client.managed == nil {
			t.Fatal("unknown remote commit downgraded to legacy transport")
		}
		dropBindingReply.Store(false)
	})
	first := New(config, logger)
	first.client.SetToken("mul_discovery")
	if err = first.initializeManagedTransport(context.Background(), home); err != nil {
		t.Fatal(err)
	}
	if first.cfg.ManagedHandoff != nil {
		t.Fatal("consumed proof retained for automatic restart")
	}
	stored, err := LoadManagedCredentialStore(home, handoff.DeploymentID, handoff.UserID, profile, server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if len(stored.Bindings) != 1 || stored.Bindings[0].Credential.DaemonToken != "mdt_original" {
		t.Fatal("scoped credential not persisted")
	}
	marker, err := os.ReadFile(filepath.Join(directory, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(marker, []byte("management_deployment_id")) {
		t.Fatal("managed profile marker missing")
	}
	config.ManagedHandoff = nil
	second := New(config, logger)
	second.client.SetToken("mul_discovery")
	if err = second.initializeManagedTransport(context.Background(), home); err != nil {
		t.Fatal(err)
	}
	if renewals.Load() != 1 {
		t.Fatal("restart did not renew with installation proof")
	}
	if second.client.managed.workspaces[workspace].client.Token() != "mdt_renewed" {
		t.Fatal("scoped renewal was not installed")
	}
	// Main/renderer never participate in this restart path.
	if second.managed.store.Bindings[0].Credential.DaemonToken != "mdt_renewed" {
		t.Fatal("renewed credential was not stored")
	}
	second.managed.store.Bindings[0].Credential.ExpiresAt = time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	if err = SaveManagedCredentialStore(home, second.managed.store); err != nil {
		t.Fatal(err)
	}
	rejectRenewal.Store(true)
	stale := New(config, logger)
	stale.client.SetToken("mul_discovery")
	if err = stale.initializeManagedTransport(context.Background(), home); err == nil {
		t.Fatal("expired no-handoff startup accepted")
	}
	freshChallenge, freshProof := currentManagedChallenge(t, identity, handoff, "bind", workspace, "1")
	handoff.Bindings[0].Challenge = freshChallenge
	handoff.Bindings[0].Proof = freshProof
	config.ManagedHandoff = &handoff
	restored := New(config, logger)
	restored.client.SetToken("mul_discovery")
	if err = restored.initializeManagedTransport(context.Background(), home); err != nil {
		t.Fatalf("fresh human handoff did not recover expired scope: %v", err)
	}
}

func TestManagedControlAuthenticatesWithoutLeakingCredentials(t *testing.T) {
	identity, handoff := managedHandoffFixture(t)
	daemon := New(Config{Profile: "desktop-test", ManagementDeploymentID: handoff.DeploymentID, ServerBaseURL: handoff.ServerURL, WorkspacesRoot: t.TempDir()}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	daemon.managed = &managedDaemonLifecycle{identity: identity, controlToken: strings.Repeat("A", 43), store: &ManagedCredentialStore{DeploymentID: handoff.DeploymentID, OrganizationID: handoff.OrganizationID, UserID: handoff.UserID, AuthVersion: handoff.AuthVersion, InstallationID: handoff.InstallationID, ManagedDaemonID: handoff.ManagedDaemonID, Profile: "desktop-test", Bindings: []ManagedStoredBinding{}}}
	daemon.ready.Store(true)
	response := httptest.NewRecorder()
	daemon.managementSessionHandler()(response, httptest.NewRequest("GET", "/management/session", nil))
	if response.Code != 401 {
		t.Fatalf("unauthenticated IPC = %d", response.Code)
	}
	request := httptest.NewRequest("GET", "/management/session", nil)
	request.Header, _ = ManagementRequestHeaders(daemon.managed.controlToken, request.Method, request.URL.Path, nil)
	response = httptest.NewRecorder()
	daemon.managementSessionHandler()(response, request)
	if response.Code != 200 || strings.Contains(response.Body.String(), daemon.managed.controlToken) || strings.Contains(response.Body.String(), "daemon_token") {
		t.Fatal("management session leaked a credential")
	}
}
