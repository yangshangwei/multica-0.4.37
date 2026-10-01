package daemon

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"strings"
	"time"
)

const MaxManagedHandoffBytes = 256 << 10

// ManagedStartupHandoff is carried only over the one-shot inherited stdin pipe.
// It contains scoped challenge proofs, never a human JWT or installation seed.
type ManagedStartupHandoff struct {
	Version         string                  `json:"version"`
	ServerURL       string                  `json:"server_url"`
	DeploymentID    string                  `json:"deployment_id"`
	OrganizationID  string                  `json:"organization_id"`
	UserID          string                  `json:"user_id"`
	AuthVersion     string                  `json:"auth_version"`
	InstallationID  string                  `json:"installation_id"`
	ManagedDaemonID string                  `json:"managed_daemon_id"`
	Bindings        []ManagedBindingHandoff `json:"bindings"`
}

type ManagedBindingHandoff struct {
	WorkspaceID string                `json:"workspace_id"`
	Challenge   InstallationChallenge `json:"challenge"`
	Proof       InstallationProof     `json:"proof"`
}

func (h *ManagedStartupHandoff) String() string   { return "ManagedStartupHandoff{proofs redacted}" }
func (h *ManagedStartupHandoff) GoString() string { return h.String() }

func canonicalManagedServer(raw string) (string, error) {
	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("invalid management server URL")
	}
	return strings.TrimRight(parsed.String(), "/"), nil
}

func ReadManagedStartupHandoff(reader io.Reader) ([]byte, *ManagedStartupHandoff, error) {
	raw, err := io.ReadAll(io.LimitReader(reader, MaxManagedHandoffBytes+1))
	if err != nil {
		return nil, nil, errors.New("read managed startup handoff failed")
	}
	if len(raw) == 0 || len(raw) > MaxManagedHandoffBytes {
		return nil, nil, errors.New("managed startup handoff size is invalid")
	}
	var handoff ManagedStartupHandoff
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&handoff); err != nil {
		return nil, nil, errors.New("invalid managed startup handoff")
	}
	if err = managedCanonicalJSON(bytes.TrimSpace(raw), &handoff); err != nil {
		return nil, nil, err
	}
	if handoff.Version != "1" || !managedUUID.MatchString(handoff.DeploymentID) || !managedUUID.MatchString(handoff.OrganizationID) || !managedUUID.MatchString(handoff.UserID) || !managedUUID.MatchString(handoff.InstallationID) || !managedUUID.MatchString(handoff.ManagedDaemonID) || handoff.Bindings == nil || len(handoff.Bindings) > 100 {
		return nil, nil, errors.New("invalid managed startup scope")
	}
	if _, ok := managedNumber(handoff.AuthVersion); !ok {
		return nil, nil, errors.New("invalid managed startup account version")
	}
	if _, err = canonicalManagedServer(handoff.ServerURL); err != nil {
		return nil, nil, err
	}
	seen := make(map[string]bool, len(handoff.Bindings))
	for _, binding := range handoff.Bindings {
		if !managedUUID.MatchString(binding.WorkspaceID) || seen[binding.WorkspaceID] {
			return nil, nil, errors.New("invalid or duplicated managed workspace")
		}
		seen[binding.WorkspaceID] = true
	}
	return raw, &handoff, nil
}

// Validate checks all proofs before any remote bind or registration is attempted.
func (h *ManagedStartupHandoff) Validate(identity *ManagedInstallation, serverURL string, now time.Time) error {
	expected, err := canonicalManagedServer(serverURL)
	if err != nil {
		return err
	}
	actual, err := canonicalManagedServer(h.ServerURL)
	if err != nil {
		return err
	}
	if actual != expected || identity.DeploymentID() != h.DeploymentID || identity.InstallationID() != h.InstallationID || identity.ManagedDaemonID(h.UserID) != h.ManagedDaemonID {
		return errors.New("managed startup identity does not match this process")
	}
	for _, binding := range h.Bindings {
		body, err := managedDecode(binding.Challenge.BodyPayload, 65536)
		if err != nil {
			return err
		}
		var parameters struct {
			InstallationID       string  `json:"installation_id"`
			WorkspaceID          string  `json:"workspace_id"`
			DaemonID             string  `json:"daemon_id"`
			PublicKey            string  `json:"public_key"`
			ExpectedBindingEpoch *string `json:"expected_binding_epoch"`
		}
		if err = managedCanonicalJSON(body, &parameters); err != nil {
			return err
		}
		epoch := ""
		if parameters.ExpectedBindingEpoch != nil {
			epoch = *parameters.ExpectedBindingEpoch
		}
		scope := ManagedChallengeScope{Purpose: "bind", DeploymentID: h.DeploymentID, OrganizationID: h.OrganizationID, UserID: h.UserID, AuthVersion: h.AuthVersion, InstallationID: h.InstallationID, WorkspaceID: binding.WorkspaceID, DaemonID: h.ManagedDaemonID, ExpectedBindingEpoch: epoch, Method: "POST", Path: "/api/daemon/installation-bindings", BodyPayload: base64.RawURLEncoding.EncodeToString(body)}
		verified, err := identity.SignChallenge(binding.Challenge, scope, now)
		if err != nil {
			return err
		}
		if verified != binding.Proof {
			return errors.New("managed handoff signature or original bytes changed")
		}
	}
	return nil
}
