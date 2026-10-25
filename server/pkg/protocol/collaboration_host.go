package protocol

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strconv"
	"strings"
)

// The legacy constant and omitted-version wire shapes remain protocol 2.
const CollaborationHostRuntimeProtocolVersion = 3
const CollaborationContainmentHostProcessSupervision = "host_process_supervision_v1"
const CollaborationHostPreparationVersion = 2
const CollaborationHostCapabilityVersion = 2
const CollaborationHostMaxLaunches = 64

// Capability fingerprints include managed permissions and account-source
// identity, never token values. Native instance identifiers stay in the ledger.
type CollaborationHostCapability struct {
	ProviderVersion     string   `json:"provider_version"`
	ProviderMode        string   `json:"provider_mode"`
	ConfigurationDigest string   `json:"configuration_digest"`
	SupervisionScopes   []string `json:"supervision_scopes"`
}

type CollaborationHostContext struct {
	AgentOwnerUserID  string  `json:"agent_owner_user_id"`
	RuntimeID         string  `json:"runtime_id"`
	Role              string  `json:"role"`
	StepKey           string  `json:"step_key"`
	ParentRunID       *string `json:"parent_run_id"`
	SessionGeneration string  `json:"session_generation"`
}

type CollaborationLaunchDescriptor struct {
	Kind                string `json:"kind"`
	CapabilityDigest    string `json:"capability_digest"`
	ConfigurationDigest string `json:"configuration_digest"`
}

// Cleanup is independent of both the business outcome and managed termination.
// Scope describes the actual observation, not a promise about every descendant.
type CollaborationCleanupObservation struct {
	State     string `json:"state"`
	Scope     string `json:"scope"`
	Attempted bool   `json:"attempted"`
}

type CollaborationLaunchTermination struct {
	ProtocolVersion int                             `json:"protocol_version"`
	Identity        CollaborationRunIdentity        `json:"identity"`
	Containment     string                          `json:"containment"`
	AttemptSeq      string                          `json:"attempt_seq"`
	AttemptNonce    string                          `json:"attempt_nonce"`
	LaunchID        string                          `json:"launch_id"`
	EventID         string                          `json:"event_id"`
	Kind            string                          `json:"kind"`
	Cleanup         CollaborationCleanupObservation `json:"cleanup"`
	ExitCode        *int                            `json:"exit_code,omitempty"`
}

type CollaborationLaunchTerminationReceipt struct {
	ProtocolVersion int                      `json:"protocol_version"`
	Identity        CollaborationRunIdentity `json:"identity"`
	LaunchID        string                   `json:"launch_id"`
	EventID         string                   `json:"event_id"`
	Accepted        bool                     `json:"accepted"`
	Authorized      bool                     `json:"authorized"`
}

// Required false/zero facts must not silently become defaults when an
// installed peer omits a field. Decode unknown and repeated names strictly.
func decodeCollaborationHostObject(raw []byte, target any, required ...string) error {
	invalid := errors.New("invalid collaboration host object")
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil || decoder.Decode(new(any)) != io.EOF {
		return invalid
	}
	keys := json.NewDecoder(bytes.NewReader(raw))
	open, err := keys.Token()
	if err != nil || open != json.Delim('{') {
		return invalid
	}
	seen := map[string]bool{}
	for keys.More() {
		key, err := keys.Token()
		name, ok := key.(string)
		if err != nil || !ok || seen[name] {
			return invalid
		}
		var value json.RawMessage
		if err = keys.Decode(&value); err != nil {
			return invalid
		}
		seen[name] = true
		for _, field := range required {
			if name == field && bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
				return invalid
			}
		}
	}
	for _, field := range required {
		if !seen[field] {
			return invalid
		}
	}
	return nil
}

func (c *CollaborationCleanupObservation) UnmarshalJSON(raw []byte) error {
	type wire CollaborationCleanupObservation
	var value wire
	if err := decodeCollaborationHostObject(raw, &value, "state", "scope", "attempted"); err != nil {
		return err
	}
	*c = CollaborationCleanupObservation(value)
	return nil
}

func (t *CollaborationLaunchTermination) UnmarshalJSON(raw []byte) error {
	type wire CollaborationLaunchTermination
	var value wire
	if err := decodeCollaborationHostObject(raw, &value, "protocol_version", "identity", "containment", "attempt_seq", "attempt_nonce", "launch_id", "event_id", "kind", "cleanup"); err != nil {
		return err
	}
	*t = CollaborationLaunchTermination(value)
	return ValidateCollaborationLaunchTermination(*t)
}

func CollaborationRuntimeModeVersion(version int, containment string) bool {
	return version == CollaborationRuntimeProtocolVersion && containment == CollaborationContainmentLinuxPIDNamespace ||
		version == CollaborationHostRuntimeProtocolVersion && containment == CollaborationContainmentHostProcessSupervision
}

func hostProvider(p CollaborationPreparationProvider, h *CollaborationHostCapability) bool {
	if h == nil || p.AdapterID != "ordinary_backend_v1" || p.AdapterVersion != 1 || p.BundleDigest != "" ||
		p.AccountSource != "runtime_configuration" || p.Resume != "isolated" || h.ProviderMode != "local" ||
		!preparationHex(h.ConfigurationDigest, 64) || len(h.SupervisionScopes) == 0 || len(h.SupervisionScopes) > 3 ||
		len(h.ProviderVersion) == 0 || len(h.ProviderVersion) > 128 {
		return false
	}
	for _, value := range []string{p.Provider, h.ProviderVersion} {
		for _, c := range value {
			if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune("._+-", c)) {
				return false
			}
		}
	}
	// Remote/gateway adapters need their own terminal contract before admission.
	switch p.ProtocolFamily {
	case "claude", "codex", "opencode", "gemini", "cursor", "amp", "copilot", "qwen", "kilo", "pi", "qoder", "crush", "auggie", "droid", "kimi", "goose", "cline":
	default:
		return false
	}
	if p.Provider == "" {
		return false
	}
	seen := map[string]bool{}
	for _, scope := range h.SupervisionScopes {
		if !collaborationHostScope(scope) || scope == "remote_session" || seen[scope] {
			return false
		}
		seen[scope] = true
	}
	return true
}

func CollaborationRuntimeCapabilityDigest(c CollaborationRuntimeCapabilities) (string, error) {
	if err := ValidateCollaborationRuntimeCapabilities(c); err != nil {
		return "", err
	}
	raw, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func ValidateCollaborationLaunchDescriptor(d CollaborationLaunchDescriptor) error {
	if (d.Kind != "provider" && d.Kind != "preparation") || !preparationHex(d.CapabilityDigest, 64) || !preparationHex(d.ConfigurationDigest, 64) {
		return errors.New("invalid collaboration launch descriptor")
	}
	return nil
}

func collaborationHostScope(scope string) bool {
	switch scope {
	case "process_group", "windows_job", "direct_process", "remote_session":
		return true
	default:
		return false
	}
}

func ValidateCollaborationCleanup(c CollaborationCleanupObservation, summary bool) error {
	if !collaborationHostScope(c.Scope) && !(summary && c.Scope == "mixed") {
		return errors.New("invalid collaboration cleanup scope")
	}
	switch c.State {
	case "tracked_scope_complete", "unverified":
		return nil
	case "best_effort":
		if c.Attempted {
			return nil
		}
	}
	return errors.New("invalid collaboration cleanup observation")
}

func ValidateCollaborationLaunchTermination(t CollaborationLaunchTermination) error {
	seq, err := strconv.ParseInt(t.AttemptSeq, 10, 64)
	if t.ProtocolVersion != CollaborationHostRuntimeProtocolVersion || t.Containment != CollaborationContainmentHostProcessSupervision ||
		!preparationIdentity(t.Identity) || !preparationVersion(t.AttemptSeq) || err != nil || seq > CollaborationHostMaxLaunches ||
		!preparationUUID(t.AttemptNonce) || t.LaunchID != t.AttemptNonce || !preparationUUID(t.EventID) ||
		ValidateCollaborationCleanup(t.Cleanup, false) != nil {
		return errors.New("invalid collaboration launch termination")
	}
	switch t.Kind {
	case "not_started":
		if t.ExitCode != nil || t.Cleanup.Attempted || t.Cleanup.State != "unverified" {
			return errors.New("contradictory collaboration nonlaunch")
		}
	case "managed_launch_ended":
		// Windows may return an unsigned 32-bit process status.
		if t.ExitCode != nil && (int64(*t.ExitCode) < -2147483648 || int64(*t.ExitCode) > 4294967295) {
			return errors.New("invalid collaboration launch exit code")
		}
	default:
		return errors.New("invalid collaboration launch observation kind")
	}
	return nil
}

func SummarizeCollaborationCleanup(launches []CollaborationLaunchTermination) (*CollaborationCleanupObservation, error) {
	if len(launches) == 0 {
		return nil, nil
	}
	out := launches[0].Cleanup
	for _, launch := range launches {
		if err := ValidateCollaborationLaunchTermination(launch); err != nil {
			return nil, err
		}
		if out.Scope != launch.Cleanup.Scope {
			out.Scope = "mixed"
		}
		out.Attempted = out.Attempted || launch.Cleanup.Attempted
		if launch.Cleanup.State == "unverified" || out.State == "unverified" {
			out.State = "unverified"
		} else if launch.Cleanup.State == "best_effort" || out.State == "best_effort" {
			out.State = "best_effort"
		}
	}
	return &out, nil
}

func ValidateCollaborationHostProof(p CollaborationRunProof) error {
	invalid := errors.New("invalid collaboration host proof")
	count, err := strconv.ParseInt(p.SealedAttemptSeq, 10, 64)
	if p.ProtocolVersion != CollaborationHostRuntimeProtocolVersion || p.Containment != CollaborationContainmentHostProcessSupervision ||
		!preparationIdentity(p.Identity) || !preparationUUID(p.ReceiptID) || err != nil || count < 0 || count > CollaborationHostMaxLaunches ||
		strconv.FormatInt(count, 10) != p.SealedAttemptSeq || p.LastAuthorizedAttemptSeq != p.SealedAttemptSeq || int64(len(p.Launches)) != count || p.ExitCode != nil {
		return invalid
	}
	if p.State != "host_exit_observed" && p.State != "never_launched_proven" || p.State == "host_exit_observed" && count == 0 {
		return invalid
	}
	seen := map[string]bool{}
	for i, launch := range p.Launches {
		if ValidateCollaborationLaunchTermination(launch) != nil || launch.Identity != p.Identity || launch.AttemptSeq != strconv.Itoa(i+1) || seen[launch.LaunchID] ||
			p.State == "never_launched_proven" && launch.Kind != "not_started" {
			return invalid
		}
		seen[launch.LaunchID] = true
	}
	if count > 0 && p.AttemptNonce != p.Launches[count-1].AttemptNonce || count == 0 && p.AttemptNonce != "" {
		return invalid
	}
	cleanup, err := SummarizeCollaborationCleanup(p.Launches)
	if err != nil || !reflect.DeepEqual(cleanup, p.Cleanup) {
		return invalid
	}
	return nil
}
