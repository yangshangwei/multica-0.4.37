package daemon

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type ManagedStoredBinding struct {
	WorkspaceID string                  `json:"workspace_id"`
	Credential  ManagedDaemonCredential `json:"credential"`
}

type ManagedCredentialStore struct {
	Version         string                 `json:"version"`
	ServerURL       string                 `json:"server_url"`
	DeploymentID    string                 `json:"deployment_id"`
	OrganizationID  string                 `json:"organization_id"`
	UserID          string                 `json:"user_id"`
	AuthVersion     string                 `json:"auth_version"`
	InstallationID  string                 `json:"installation_id"`
	ManagedDaemonID string                 `json:"managed_daemon_id"`
	Profile         string                 `json:"profile"`
	Bindings        []ManagedStoredBinding `json:"bindings"`
}

func (s *ManagedCredentialStore) String() string   { return "ManagedCredentialStore{tokens redacted}" }
func (s *ManagedCredentialStore) GoString() string { return s.String() }

func managedCredentialsPath(homeDirectory, deploymentID, userID, profile string) (string, error) {
	if !managedUUID.MatchString(deploymentID) || !managedUUID.MatchString(userID) || profile == "" {
		return "", errors.New("invalid managed credential storage scope")
	}
	root := filepath.Join(homeDirectory, ".multica")
	paths := []string{root, filepath.Join(root, "management"), filepath.Join(root, "management", deploymentID), filepath.Join(root, "management", deploymentID, "credentials"), filepath.Join(root, "management", deploymentID, "credentials", userID)}
	for i, path := range paths {
		if err := managedEnsureDirectory(path, i > 0); err != nil {
			return "", err
		}
	}
	digest := sha256.Sum256([]byte(profile))
	return filepath.Join(paths[len(paths)-1], hex.EncodeToString(digest[:])+".json"), nil
}

func validateManagedCredentialStore(store *ManagedCredentialStore, deploymentID, userID, profile, serverURL string) error {
	actual, err := canonicalManagedServer(store.ServerURL)
	if err != nil {
		return err
	}
	expected, err := canonicalManagedServer(serverURL)
	if err != nil {
		return err
	}
	if store.Version != "1" || actual != expected || store.DeploymentID != deploymentID || store.UserID != userID || store.Profile != profile || !managedUUID.MatchString(store.OrganizationID) || !managedUUID.MatchString(store.InstallationID) || !managedUUID.MatchString(store.ManagedDaemonID) || store.Bindings == nil || len(store.Bindings) > 100 {
		return errors.New("managed credential store scope changed")
	}
	if _, ok := managedNumber(store.AuthVersion); !ok {
		return errors.New("invalid managed credential source version")
	}
	seen := make(map[string]bool)
	for _, binding := range store.Bindings {
		credential := binding.Credential
		if !managedUUID.MatchString(binding.WorkspaceID) || seen[binding.WorkspaceID] || credential.PrincipalUserID != userID || credential.AuthVersion != store.AuthVersion || !managedUUID.MatchString(credential.BindingID) || credential.CapabilityVersion != "1" || !strings.HasPrefix(credential.DaemonToken, "mdt_") {
			return errors.New("invalid managed credential binding")
		}
		if _, ok := managedNumber(credential.BindingEpoch); !ok {
			return errors.New("invalid managed credential epoch")
		}
		if _, err := time.Parse(time.RFC3339, credential.ExpiresAt); err != nil {
			return errors.New("invalid managed credential expiry")
		}
		seen[binding.WorkspaceID] = true
	}
	return nil
}

func LoadManagedCredentialStore(homeDirectory, deploymentID, userID, profile, serverURL string) (*ManagedCredentialStore, error) {
	path, err := managedCredentialsPath(homeDirectory, deploymentID, userID, profile)
	if err != nil {
		return nil, err
	}
	before, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !before.Mode().IsRegular() || before.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("managed credential store must be a regular file")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !os.SameFile(before, info) || info.Size() > MaxManagedHandoffBytes {
		return nil, errors.New("managed credential file changed while reading")
	}
	if err = managedCheckPermissions(info, true); err != nil {
		return nil, err
	}
	raw, err := io.ReadAll(io.LimitReader(file, MaxManagedHandoffBytes+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > MaxManagedHandoffBytes {
		return nil, errors.New("managed credential store too large")
	}
	var store ManagedCredentialStore
	if err = managedCanonicalJSON(bytes.TrimSpace(raw), &store); err != nil {
		return nil, err
	}
	if err = validateManagedCredentialStore(&store, deploymentID, userID, profile, serverURL); err != nil {
		return nil, err
	}
	return &store, nil
}

func SaveManagedCredentialStore(homeDirectory string, store *ManagedCredentialStore) error {
	if err := validateManagedCredentialStore(store, store.DeploymentID, store.UserID, store.Profile, store.ServerURL); err != nil {
		return err
	}
	path, err := managedCredentialsPath(homeDirectory, store.DeploymentID, store.UserID, store.Profile)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(store)
	if err != nil {
		return err
	}
	if len(raw) > MaxManagedHandoffBytes {
		return errors.New("managed credential store too large")
	}
	return withManagedIdentityLock(filepath.Dir(path), func() error {
		// Existing files receive the same strict checks before atomic replacement.
		if _, err := LoadManagedCredentialStore(homeDirectory, store.DeploymentID, store.UserID, store.Profile, store.ServerURL); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return writeManagementJSON(path, store)
	})
}
