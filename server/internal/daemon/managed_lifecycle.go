package daemon

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type managedDaemonLifecycle struct {
	mu            sync.Mutex
	controlMu     sync.Mutex
	controlNonces map[string]int64
	homeDirectory string
	identity      *ManagedInstallation
	store         *ManagedCredentialStore
	controlToken  string
}

func managedProfileDirectory(homeDirectory, profile string) (string, error) {
	if profile == "" || profile == "." || profile == ".." || strings.ContainsAny(profile, "/\\") {
		return "", errors.New("managed mode requires a named local profile")
	}
	return filepath.Join(homeDirectory, ".multica", "profiles", profile), nil
}

func managedProfileUser(homeDirectory, profile string) (string, error) {
	directory, err := managedProfileDirectory(homeDirectory, profile)
	if err != nil {
		return "", err
	}
	raw, err := os.ReadFile(filepath.Join(directory, ".desktop-user-id"))
	if err != nil {
		return "", errors.New("managed profile has no user identity")
	}
	userID := strings.TrimSpace(string(raw))
	if !managedUUID.MatchString(userID) {
		return "", errors.New("managed profile user is invalid")
	}
	return userID, nil
}

func persistManagedProfileMarker(homeDirectory, profile, deploymentID string) error {
	directory, err := managedProfileDirectory(homeDirectory, profile)
	if err != nil {
		return err
	}
	path := filepath.Join(directory, "config.json")
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New("managed profile config must be a regular file")
	}
	if err = managedCheckPermissions(info, true); err != nil {
		return err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var config map[string]json.RawMessage
	if err = json.Unmarshal(raw, &config); err != nil || config == nil {
		return errors.New("invalid managed profile config")
	}
	marker, _ := json.Marshal(deploymentID)
	config["management_deployment_id"] = marker
	return writeManagementJSON(path, config)
}

func (d *Daemon) initializeManagedTransport(ctx context.Context, homeDirectory string) error {
	if d.cfg.ManagementDeploymentID == "" {
		return nil
	}
	d.client.client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	identity, err := LoadManagedInstallation(homeDirectory, d.cfg.ManagementDeploymentID)
	if err != nil {
		return err
	}
	userID, err := managedProfileUser(homeDirectory, d.cfg.Profile)
	if err != nil {
		return err
	}
	if identity.InstallationID() == "" || identity.ManagedDaemonID(userID) != d.cfg.DaemonID {
		return errors.New("managed profile identity mismatch")
	}
	var config struct {
		AuthMode     string `json:"auth_mode"`
		DeploymentID string `json:"deployment_id"`
	}
	if err = d.client.getJSON(ctx, "/api/config", &config); err != nil {
		return err
	}
	if config.AuthMode != "password" || config.DeploymentID != identity.DeploymentID() {
		return errors.New("managed daemon deployment changed")
	}
	var actor struct {
		ID             string `json:"id"`
		RequiresSetup  bool   `json:"requires_account_setup"`
		RequiresChange bool   `json:"requires_password_change"`
	}
	if err = d.client.getJSON(ctx, "/api/me", &actor); err != nil {
		return err
	}
	if actor.ID != userID || actor.RequiresSetup || actor.RequiresChange {
		return errors.New("daemon credential principal differs from managed profile")
	}
	store, loadErr := LoadManagedCredentialStore(homeDirectory, identity.DeploymentID(), userID, d.cfg.Profile, d.cfg.ServerBaseURL)
	if loadErr != nil && !errors.Is(loadErr, os.ErrNotExist) {
		return loadErr
	}
	handoff := d.cfg.ManagedHandoff
	if handoff != nil {
		if handoff.UserID != actor.ID {
			return errors.New("desktop and daemon principals differ")
		}
		if err = handoff.Validate(identity, d.cfg.ServerBaseURL, time.Now()); err != nil {
			return err
		}
		if store == nil || store.AuthVersion != handoff.AuthVersion || store.InstallationID != handoff.InstallationID {
			store = &ManagedCredentialStore{Version: "1", ServerURL: d.cfg.ServerBaseURL, DeploymentID: handoff.DeploymentID, OrganizationID: handoff.OrganizationID, UserID: handoff.UserID, AuthVersion: handoff.AuthVersion, InstallationID: handoff.InstallationID, ManagedDaemonID: handoff.ManagedDaemonID, Profile: d.cfg.Profile, Bindings: []ManagedStoredBinding{}}
		}
	} else if store == nil {
		return errors.New("managed credentials are unavailable; sign in through Desktop to bind again")
	}
	if store.InstallationID != identity.InstallationID() || store.ManagedDaemonID != identity.ManagedDaemonID(userID) {
		return errors.New("managed credentials belong to another installation")
	}
	// Commit the local mode before any remote side effect. A crash after a
	// bind must restart closed, never through the legacy PAT registration path.
	if err = persistManagedProfileMarker(homeDirectory, d.cfg.Profile, identity.DeploymentID()); err != nil {
		return err
	}
	workspaces, err := d.client.ListWorkspaces(ctx)
	if err != nil {
		return err
	}
	desired := make(map[string]bool, len(workspaces))
	for _, workspace := range workspaces {
		if !managedUUID.MatchString(workspace.ID) {
			return errors.New("invalid managed discovery workspace")
		}
		desired[workspace.ID] = true
	}
	retained := make([]ManagedStoredBinding, 0, len(store.Bindings))
	for _, binding := range store.Bindings {
		if desired[binding.WorkspaceID] {
			retained = append(retained, binding)
		}
	}
	store.Bindings = retained
	covered := make(map[string]bool)
	if handoff != nil {
		for _, binding := range handoff.Bindings {
			if !desired[binding.WorkspaceID] {
				return errors.New("managed handoff workspace is no longer accessible")
			}
			covered[binding.WorkspaceID] = true
		}
	}
	state := &managedDaemonLifecycle{homeDirectory: homeDirectory, identity: identity, store: store}
	if err = SaveManagedCredentialStore(homeDirectory, store); err != nil {
		return err
	}
	d.client.enableManagedTransport()
	// Fresh human-authorized proofs recover covered expired scopes first.
	// Only remaining desired scopes depend on their stored mdt renewal.
	if handoff != nil {
		if err = d.applyManagedHandoff(ctx, state, handoff, false); err != nil {
			return err
		}
	}
	for index, binding := range store.Bindings {
		if !covered[binding.WorkspaceID] {
			if err = d.renewManagedBinding(ctx, state, index); err != nil {
				return err
			}
		}
	}
	if len(store.Bindings) == 0 && len(desired) > 0 {
		return errors.New("managed profile is awaiting an authenticated workspace binding")
	}
	for _, binding := range state.store.Bindings {
		if err = d.client.installManagedCredential(binding.WorkspaceID, binding.Credential); err != nil {
			return err
		}
	}
	if err = SaveManagedCredentialStore(homeDirectory, state.store); err != nil {
		return err
	}
	if err = d.createManagementControlToken(state); err != nil {
		return err
	}
	d.managed = state
	// A proof cannot survive into automatic restart or be accidentally replayed.
	d.cfg.ManagedHandoff = nil
	return nil
}

func (d *Daemon) applyManagedHandoff(ctx context.Context, state *managedDaemonLifecycle, handoff *ManagedStartupHandoff, live bool) error {
	if handoff.UserID != state.store.UserID || handoff.AuthVersion != state.store.AuthVersion || handoff.OrganizationID != state.store.OrganizationID {
		return errors.New("management handoff scope changed")
	}
	if err := handoff.Validate(state.identity, d.cfg.ServerBaseURL, time.Now()); err != nil {
		return err
	}
	for _, binding := range handoff.Bindings {
		existing := -1
		for i, current := range state.store.Bindings {
			if current.WorkspaceID == binding.WorkspaceID {
				existing = i
				break
			}
		}
		// The caller holds the idle claim barrier for live handoff. Same-
		// principal rebind can advance its epoch without interrupting work.

		var credential ManagedDaemonCredential
		if err := d.client.postJSONWithToken(ctx, "/api/daemon/installation-bindings", d.client.Token(), binding.Proof, &credential); err != nil {
			return err
		}
		if credential.PrincipalUserID != state.store.UserID || credential.AuthVersion != state.store.AuthVersion {
			return errors.New("daemon binding source account does not match Desktop")
		}
		if err := d.client.installManagedCredential(binding.WorkspaceID, credential); err != nil {
			return err
		}
		next := ManagedStoredBinding{WorkspaceID: binding.WorkspaceID, Credential: credential}
		if existing >= 0 {
			state.store.Bindings[existing] = next
		} else {
			state.store.Bindings = append(state.store.Bindings, next)
		}
		if err := SaveManagedCredentialStore(state.homeDirectory, state.store); err != nil {
			return err
		}
	}
	return nil
}

func (d *Daemon) renewManagedBinding(ctx context.Context, state *managedDaemonLifecycle, index int) error {
	binding := state.store.Bindings[index]
	current := binding.Credential
	client := NewClient(d.cfg.ServerBaseURL)
	client.client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	client.SetToken(current.DaemonToken)
	parameters := struct {
		Purpose              string `json:"purpose"`
		InstallationID       string `json:"installation_id"`
		WorkspaceID          string `json:"workspace_id"`
		DaemonID             string `json:"daemon_id"`
		PublicKey            string `json:"public_key"`
		ExpectedBindingEpoch string `json:"expected_binding_epoch"`
	}{"renew", state.store.InstallationID, binding.WorkspaceID, state.store.ManagedDaemonID, state.identity.PublicKeyBase64(), current.BindingEpoch}
	var challenge InstallationChallenge
	if err := client.postJSON(ctx, "/api/daemon/installations/challenges", parameters, &challenge); err != nil {
		return err
	}
	body, _ := json.Marshal(struct {
		InstallationID       string `json:"installation_id"`
		WorkspaceID          string `json:"workspace_id"`
		DaemonID             string `json:"daemon_id"`
		PublicKey            string `json:"public_key"`
		ExpectedBindingEpoch string `json:"expected_binding_epoch"`
	}{state.store.InstallationID, binding.WorkspaceID, state.store.ManagedDaemonID, state.identity.PublicKeyBase64(), current.BindingEpoch})
	scope := ManagedChallengeScope{Purpose: "renew", DeploymentID: state.store.DeploymentID, OrganizationID: state.store.OrganizationID, UserID: state.store.UserID, AuthVersion: state.store.AuthVersion, InstallationID: state.store.InstallationID, WorkspaceID: binding.WorkspaceID, DaemonID: state.store.ManagedDaemonID, ExpectedBindingEpoch: current.BindingEpoch, Method: "POST", Path: "/api/daemon/installation-bindings/renew", BodyPayload: base64.RawURLEncoding.EncodeToString(body)}
	proof, err := state.identity.SignChallenge(challenge, scope, time.Now())
	if err != nil {
		return err
	}
	var renewed ManagedDaemonCredential
	if err = client.postJSON(ctx, "/api/daemon/installation-bindings/renew", proof, &renewed); err != nil {
		return err
	}
	if renewed.BindingID != current.BindingID || renewed.BindingEpoch != current.BindingEpoch || renewed.PrincipalUserID != state.store.UserID || renewed.AuthVersion != state.store.AuthVersion {
		return errors.New("renewal changed managed binding scope")
	}
	if err = d.client.installManagedCredential(binding.WorkspaceID, renewed); err != nil {
		return err
	}
	state.store.Bindings[index].Credential = renewed
	return SaveManagedCredentialStore(state.homeDirectory, state.store)
}

func (d *Daemon) managedCredentialRenewalLoop(ctx context.Context) {
	if d.managed == nil {
		return
	}
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			state := d.managed
			state.mu.Lock()
			renewed := false
			for index, binding := range state.store.Bindings {
				expires, err := time.Parse(time.RFC3339, binding.Credential.ExpiresAt)
				if err == nil && time.Until(expires) > time.Hour {
					continue
				}
				if err = d.renewManagedBinding(ctx, state, index); err != nil {
					d.logger.Warn("managed workspace credential renewal failed; no PAT fallback", "workspace_id", binding.WorkspaceID)
				} else {
					renewed = true
				}
			}
			state.mu.Unlock()
			if renewed {
				d.notifyRuntimeSetChanged()
			}
		}
	}
}
