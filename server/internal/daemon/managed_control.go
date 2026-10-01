package daemon

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

type managementControlFile struct {
	Version string `json:"version"`
	Token   string `json:"token"`
	PID     int    `json:"pid"`
}

func (d *Daemon) createManagementControlToken(state *managedDaemonLifecycle) error {
	directory, err := managedProfileDirectory(state.homeDirectory, d.cfg.Profile)
	if err != nil {
		return err
	}
	if err = managedEnsureDirectory(directory, false); err != nil {
		return err
	}
	secret := make([]byte, 32)
	if _, err = rand.Read(secret); err != nil {
		return err
	}
	state.controlToken = base64.RawURLEncoding.EncodeToString(secret)
	return writeManagementJSON(filepath.Join(directory, "management-control.json"), managementControlFile{Version: "1", Token: state.controlToken, PID: os.Getpid()})
}

func (d *Daemon) managementAuthorized(w http.ResponseWriter, r *http.Request) bool {
	w.Header().Set("Cache-Control", "no-store")
	// ready publishes the initialized immutable lifecycle pointer. While
	// bootstrap is pending, health stays available but control stays closed.
	if d.cfg.ManagementDeploymentID != "" && !d.ready.Load() {
		http.Error(w, "Managed daemon is starting", http.StatusServiceUnavailable)
		return false
	}
	if d.managed == nil {
		http.NotFound(w, r)
		return false
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, MaxManagedHandoffBytes+1))
	if err != nil || len(raw) > MaxManagedHandoffBytes || !d.authenticateManagementRequest(r, raw) {
		http.Error(w, "Management authentication required", http.StatusUnauthorized)
		return false
	}
	r.Body = io.NopCloser(bytes.NewReader(raw))
	return true
}

func (d *Daemon) managementSessionHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !d.managementAuthorized(w, r) {
			return
		}
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		state := d.managed
		state.mu.Lock()
		defer state.mu.Unlock()
		workspaceIDs := make([]string, 0, len(state.store.Bindings))
		for _, binding := range state.store.Bindings {
			workspaceIDs = append(workspaceIDs, binding.WorkspaceID)
		}
		d.claimMu.Lock()
		var drainIntent *string
		if d.managedDrainIntent != "" {
			value := d.managedDrainIntent
			drainIntent = &value
		}
		d.claimMu.Unlock()
		d.writeManagementResponse(w, r, http.StatusOK, map[string]any{"drain_intent_id": drainIntent, "workspace_ids": workspaceIDs, "capability_version": "1", "deployment_id": state.store.DeploymentID, "organization_id": state.store.OrganizationID, "user_id": state.store.UserID, "auth_version": state.store.AuthVersion, "installation_id": state.store.InstallationID, "managed_daemon_id": state.store.ManagedDaemonID, "profile": state.store.Profile, "active_task_count": d.activeTasks.Load(), "ready": d.ready.Load()})
	}
}

func (d *Daemon) managementHandoffHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !d.managementAuthorized(w, r) {
			return
		}
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if r.Header.Get("Content-Type") != "application/json" {
			http.Error(w, "JSON required", http.StatusUnsupportedMediaType)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, MaxManagedHandoffBytes)
		_, handoff, err := ReadManagedStartupHandoff(r.Body)
		if err != nil {
			http.Error(w, "Invalid management handoff", http.StatusBadRequest)
			return
		}
		if !d.ready.Load() {
			http.Error(w, "Daemon is starting", http.StatusServiceUnavailable)
			return
		}
		// Reuse the existing claim/active-task barrier, including its claim-to-
		// dispatch invariant, so an idle check cannot race new work.
		if !d.trySetClaimBarrier() {
			d.writeManagementResponse(w, r, http.StatusConflict, map[string]string{"error": "Daemon is busy"})
			return
		}
		defer d.releaseClaimBarrier()
		state := d.managed
		state.mu.Lock()
		err = d.applyManagedHandoff(r.Context(), state, handoff, true)
		state.mu.Unlock()
		if err != nil {
			d.writeManagementResponse(w, r, http.StatusConflict, map[string]string{"error": "Management scope or proof was rejected"})
			return
		}
		d.notifyRuntimeSetChanged()
		if d.workspaceChanges != nil {
			d.workspaceChanges.broadcast()
		}
		d.writeManagementResponse(w, r, http.StatusOK, map[string]bool{"accepted": true})
	}
}

// ReadManagementControlToken is main/CLI-only. The returned secret is never
// part of daemon status output, environment variables or process arguments.
func readManagementControlFile(homeDirectory, profile string) (managementControlFile, error) {
	directory, err := managedProfileDirectory(homeDirectory, profile)
	if err != nil {
		return managementControlFile{}, err
	}
	path := filepath.Join(directory, "management-control.json")
	info, err := os.Lstat(path)
	if err != nil {
		return managementControlFile{}, err
	}
	if !info.Mode().IsRegular() || info.Size() > 4096 {
		return managementControlFile{}, errors.New("invalid management control credential file")
	}
	if err = managedCheckPermissions(info, true); err != nil {
		return managementControlFile{}, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return managementControlFile{}, err
	}
	var control managementControlFile
	if err = managedCanonicalJSON(bytes.TrimSpace(raw), &control); err != nil {
		return managementControlFile{}, err
	}
	decoded, err := managedDecode(control.Token, 32)
	if err != nil || len(decoded) != 32 || control.Version != "1" || control.PID <= 0 {
		return managementControlFile{}, errors.New("invalid management control credential")
	}
	return control, nil
}

func ReadManagementControlToken(homeDirectory, profile string) (string, error) {
	control, err := readManagementControlFile(homeDirectory, profile)
	return control.Token, err
}
