package protocol

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
)

const CollaborationPreparationVersion = 1
const CollaborationProviderBundleVersion = 1
const CollaborationAttachmentFormat = "delivered_attachments_v1"
const CollaborationInvocationInstructionsLimit = CollaborationRecordTextLimit
const CollaborationInvocationModelLimit = 256
const CollaborationInvocationThinkingLimit = 64

// Preparation is negotiated separately from the retained run/proof identity.
// The private manifest digest must not enter human records. Attachment content
// hashes and credential values are never part of this wire contract.
type CollaborationPreparationProvider struct {
	Provider       string `json:"provider"`
	ProtocolFamily string `json:"protocol_family"`
	AdapterID      string `json:"adapter_id"`
	AdapterVersion int    `json:"adapter_version"`
	BundleDigest   string `json:"bundle_digest"`
	AccountSource  string `json:"account_source"`
	Resume         string `json:"resume"`
	Fallback       bool   `json:"fallback"`
}

type CollaborationPreparationLimits struct {
	FileCount        int   `json:"file_count"`
	PerFileBytes     int64 `json:"per_file_bytes"`
	TotalInputBytes  int64 `json:"total_input_bytes"`
	WorkspaceBytes   int64 `json:"workspace_bytes"`
	GitExpandedBytes int64 `json:"git_expanded_bytes"`
	EntryCount       int   `json:"entry_count"`
	TimeoutSeconds   int   `json:"timeout_seconds"`
}

// Git refers to an explicitly configured bundle and complete commit. No host
// path, remote credential, mutable ref or original working tree is transferable.
type CollaborationPreparationGit struct {
	ObjectFormat string `json:"object_format"`
	CommitOID    string `json:"commit_oid"`
	BundleDigest string `json:"bundle_digest"`
}

type CollaborationRuntimeCapabilities struct {
	Version            int                              `json:"version"`
	PreparationVersion int                              `json:"preparation_version"`
	OS                 string                           `json:"os"`
	Arch               string                           `json:"arch"`
	Containment        string                           `json:"containment"`
	AttachmentFormat   string                           `json:"attachment_format"`
	Provider           CollaborationPreparationProvider `json:"provider"`
	Git                *CollaborationPreparationGit     `json:"git,omitempty"`
	Limits             CollaborationPreparationLimits   `json:"limits"`
}

type CollaborationRuntimeCapabilitiesRequest struct {
	ProtocolVersion            int                              `json:"protocol_version"`
	DeploymentID               string                           `json:"deployment_id"`
	InstallationID             string                           `json:"installation_id"`
	RuntimeID                  string                           `json:"runtime_id"`
	BindingID                  string                           `json:"binding_id"`
	BindingEpoch               string                           `json:"binding_epoch"`
	ExpectedRuntimeConfigEpoch string                           `json:"expected_runtime_config_epoch"`
	Capabilities               CollaborationRuntimeCapabilities `json:"capabilities"`
}

// The recorded epoch is the epoch after registering this full summary. A later
// configuration change makes this registration stale until explicitly renewed.
type CollaborationRuntimeCapabilityRegistration struct {
	DeploymentID       string                           `json:"deployment_id"`
	InstallationID     string                           `json:"installation_id"`
	BindingID          string                           `json:"binding_id"`
	BindingEpoch       string                           `json:"binding_epoch"`
	RuntimeConfigEpoch string                           `json:"runtime_config_epoch"`
	Capabilities       CollaborationRuntimeCapabilities `json:"capabilities"`
}

type CollaborationRuntimeCapabilitiesResponse struct {
	RuntimeID          string                                      `json:"runtime_id"`
	RuntimeConfigEpoch string                                      `json:"runtime_config_epoch"`
	Registration       *CollaborationRuntimeCapabilityRegistration `json:"registration"`
}

type CollaborationPreparationResource struct {
	AgentID              string `json:"agent_id"`
	RuntimeID            string `json:"runtime_id"`
	OwnerUserID          string `json:"owner_user_id"`
	ExecutionWorkspaceID string `json:"execution_workspace_id"`
	TargetEpoch          string `json:"target_epoch"`
	ConfigVersion        string `json:"config_version"`
	RuntimeConfigEpoch   string `json:"runtime_config_epoch"`
}

type CollaborationPreparationAttachment struct {
	ID          string `json:"id"`
	Version     string `json:"version"`
	Name        string `json:"name"`
	ContentType string `json:"content_type"`
	SizeBytes   int64  `json:"size_bytes"`
	Destination string `json:"destination"`
}

// These settings belong to the exact authorized agent at first claim. Empty
// model/effort values explicitly select that runtime's defaults.
type CollaborationPreparationInvocation struct {
	Instructions  string `json:"instructions"`
	Model         string `json:"model"`
	ThinkingLevel string `json:"thinking_level"`
}

func (p *CollaborationPreparationInvocation) UnmarshalJSON(raw []byte) error {
	type wire CollaborationPreparationInvocation
	var value wire
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return errors.New("invalid collaboration invocation")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || len(fields) != 3 {
		return errors.New("invalid collaboration invocation")
	}
	for _, key := range []string{"instructions", "model", "thinking_level"} {
		if value, ok := fields[key]; !ok || strings.TrimSpace(string(value)) == "null" {
			return errors.New("invalid collaboration invocation")
		}
	}
	keys := json.NewDecoder(strings.NewReader(string(raw)))
	if _, err := keys.Token(); err != nil {
		return errors.New("invalid collaboration invocation")
	}
	seen := map[string]bool{}
	for keys.More() {
		key, err := keys.Token()
		name, ok := key.(string)
		if err != nil || !ok || seen[name] {
			return errors.New("invalid collaboration invocation")
		}
		seen[name] = true
		var value json.RawMessage
		if keys.Decode(&value) != nil {
			return errors.New("invalid collaboration invocation")
		}
	}
	*p = CollaborationPreparationInvocation(value)
	return nil
}

// CommandIDs correlate with the actual first-claim prompt without publishing a
// hash of private input text. Both snapshots are committed together exactly once.
type CollaborationPreparation struct {
	Version        int                                  `json:"version"`
	Identity       CollaborationRunIdentity             `json:"identity"`
	CommandIDs     []string                             `json:"command_ids"`
	Resource       CollaborationPreparationResource     `json:"resource"`
	Provider       CollaborationPreparationProvider     `json:"provider"`
	Invocation     CollaborationPreparationInvocation   `json:"invocation"`
	Attachments    []CollaborationPreparationAttachment `json:"attachments"`
	Git            *CollaborationPreparationGit         `json:"git,omitempty"`
	Limits         CollaborationPreparationLimits       `json:"limits"`
	ManifestDigest string                               `json:"manifest_digest,omitempty"`
}

func (p *CollaborationPreparation) UnmarshalJSON(raw []byte) error {
	type wire CollaborationPreparation
	var value wire
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return errors.New("invalid collaboration preparation")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields["invocation"] == nil || strings.TrimSpace(string(fields["invocation"])) == "null" {
		return errors.New("invalid collaboration preparation")
	}
	*p = CollaborationPreparation(value)
	return nil
}

// Observations record preparation facts only. They neither authorize launch nor
// settle an outcome, release a reservation or assert process termination.
type CollaborationPreparationObservation struct {
	Identity       CollaborationRunIdentity `json:"identity"`
	EventID        string                   `json:"event_id"`
	AttemptSeq     string                   `json:"attempt_seq"`
	AttemptNonce   string                   `json:"attempt_nonce"`
	ManifestDigest string                   `json:"manifest_digest"`
	Stage          string                   `json:"stage"`
	State          string                   `json:"state"`
	Code           string                   `json:"code,omitempty"`
}

type CollaborationPreparationObservationReceipt struct {
	Identity       CollaborationRunIdentity `json:"identity"`
	EventID        string                   `json:"event_id"`
	ManifestDigest string                   `json:"manifest_digest"`
	Recorded       bool                     `json:"recorded"`
}

func preparationUUID(value string) bool {
	id, err := uuid.Parse(value)
	return err == nil && id != uuid.Nil && id.String() == value
}

func preparationVersion(value string) bool {
	n, err := strconv.ParseInt(value, 10, 64)
	return err == nil && n > 0 && strconv.FormatInt(n, 10) == value
}

func preparationHex(value string, size int) bool {
	if len(value) != size {
		return false
	}
	for _, c := range value {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

func preparationIdentity(id CollaborationRunIdentity) bool {
	for _, value := range []string{id.DeploymentID, id.InstallationID, id.IssueID, id.SessionID, id.TargetID, id.ChannelID, id.BatchID, id.GroupID, id.RunID, id.LaunchNonce} {
		if !preparationUUID(value) {
			return false
		}
	}
	return preparationVersion(id.Fence) && preparationVersion(id.ClaimGeneration)
}

func preparationLimits(l CollaborationPreparationLimits) bool {
	return l.FileCount >= 1 && l.FileCount <= 32 && l.PerFileBytes >= 1 && l.PerFileBytes <= 100<<20 &&
		l.TotalInputBytes >= 1 && l.TotalInputBytes <= 256<<20 && l.WorkspaceBytes >= l.TotalInputBytes && l.WorkspaceBytes <= 512<<20 &&
		l.GitExpandedBytes >= 1 && l.GitExpandedBytes <= 256<<20 && l.EntryCount >= 1 && l.EntryCount <= 65536 &&
		l.TimeoutSeconds >= 1 && l.TimeoutSeconds <= 600
}

func preparationProvider(p CollaborationPreparationProvider) bool {
	label := func(v string) bool {
		if len(v) == 0 || len(v) > 64 {
			return false
		}
		for _, c := range v {
			if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_' || c == '-') {
				return false
			}
		}
		return true
	}
	if !label(p.Provider) || !label(p.ProtocolFamily) || p.AdapterVersion != 1 || !preparationHex(p.BundleDigest, 64) ||
		p.AccountSource != "runtime_configuration" || p.Resume != "fresh" || p.Fallback {
		return false
	}
	return p.AdapterID == "static_json_v1" || p.AdapterID == "claude_stream_json_v1" && p.Provider == "claude" && p.ProtocolFamily == "claude"
}

func preparationGit(g *CollaborationPreparationGit) bool {
	if g == nil {
		return true
	}
	return preparationHex(g.BundleDigest, 64) && (g.ObjectFormat == "sha1" && preparationHex(g.CommitOID, 40) ||
		g.ObjectFormat == "sha256" && preparationHex(g.CommitOID, 64))
}

func ValidateCollaborationRuntimeCapabilities(c CollaborationRuntimeCapabilities) error {
	if c.Version != CollaborationProviderBundleVersion || c.PreparationVersion != CollaborationPreparationVersion || c.OS != "linux" ||
		(c.Arch != "amd64" && c.Arch != "arm64") || c.Containment != CollaborationContainmentLinuxPIDNamespace ||
		c.AttachmentFormat != CollaborationAttachmentFormat || !preparationProvider(c.Provider) || !preparationLimits(c.Limits) || !preparationGit(c.Git) {
		return errors.New("invalid collaboration runtime capabilities")
	}
	return nil
}

func ValidateCollaborationPreparationInvocation(p CollaborationPreparationInvocation) error {
	invalid := errors.New("invalid collaboration invocation")
	if len(p.Instructions) > CollaborationInvocationInstructionsLimit || !utf8.ValidString(p.Instructions) || strings.ContainsRune(p.Instructions, 0) {
		return invalid
	}
	for _, token := range []struct {
		value string
		limit int
		extra string
	}{{p.Model, CollaborationInvocationModelLimit, "._-/:[]+@"}, {p.ThinkingLevel, CollaborationInvocationThinkingLimit, "._-"}} {
		if len(token.value) > token.limit {
			return invalid
		}
		for i, c := range token.value {
			alphaNumeric := c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
			if !alphaNumeric && (i == 0 || !strings.ContainsRune(token.extra, c)) {
				return invalid
			}
		}
	}
	return nil
}

// Destinations are generated from validated identities, not display filenames.
func CollaborationPreparationAttachmentDestination(id, version string) string {
	return "attachments/" + id + "/" + version
}

func validatePreparationShape(p *CollaborationPreparation) error {
	invalid := errors.New("invalid collaboration preparation")
	if p == nil || p.Version != CollaborationPreparationVersion || !preparationIdentity(p.Identity) || !preparationProvider(p.Provider) || ValidateCollaborationPreparationInvocation(p.Invocation) != nil ||
		!preparationLimits(p.Limits) || !preparationGit(p.Git) || len(p.CommandIDs) == 0 || len(p.CommandIDs) > 1024 ||
		p.Attachments == nil || len(p.Attachments) > p.Limits.FileCount {
		return invalid
	}
	for _, id := range []string{p.Resource.AgentID, p.Resource.RuntimeID, p.Resource.OwnerUserID, p.Resource.ExecutionWorkspaceID} {
		if !preparationUUID(id) {
			return invalid
		}
	}
	for _, version := range []string{p.Resource.TargetEpoch, p.Resource.ConfigVersion, p.Resource.RuntimeConfigEpoch} {
		if !preparationVersion(version) {
			return invalid
		}
	}
	commands := map[string]bool{}
	for _, id := range p.CommandIDs {
		if !preparationUUID(id) || commands[id] {
			return invalid
		}
		commands[id] = true
	}
	files, total := map[string]bool{}, int64(0)
	for _, file := range p.Attachments {
		if !preparationUUID(file.ID) || !preparationVersion(file.Version) || files[file.ID] || file.SizeBytes < 0 || file.SizeBytes > p.Limits.PerFileBytes ||
			strings.TrimSpace(file.Name) == "" || len(file.Name) > 1024 || !utf8.ValidString(file.Name) || strings.ContainsRune(file.Name, 0) ||
			file.ContentType == "" || len(file.ContentType) > 256 || strings.ContainsAny(file.ContentType, "\x00\r\n") ||
			file.Destination != CollaborationPreparationAttachmentDestination(file.ID, file.Version) {
			return invalid
		}
		files[file.ID] = true
		total += file.SizeBytes
	}
	if total > p.Limits.TotalInputBytes || p.Git != nil && total+p.Limits.GitExpandedBytes > p.Limits.WorkspaceBytes {
		return invalid
	}
	return nil
}

func preparationDigest(p CollaborationPreparation) (string, error) {
	p.ManifestDigest = ""
	raw, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:]), nil
}

func SealCollaborationPreparation(p *CollaborationPreparation) error {
	if err := validatePreparationShape(p); err != nil {
		return err
	}
	digest, err := preparationDigest(*p)
	if err == nil {
		p.ManifestDigest = digest
	}
	return err
}

func ValidateCollaborationPreparation(p *CollaborationPreparation, identity CollaborationRunIdentity, inputs []CollaborationRunInput) error {
	if err := validatePreparationShape(p); err != nil {
		return err
	}
	invalid := errors.New("invalid collaboration preparation")
	if p.Identity != identity || !preparationHex(p.ManifestDigest, 64) || len(p.CommandIDs) != len(inputs) {
		return invalid
	}
	files := map[string]bool{}
	for _, file := range p.Attachments {
		files[file.ID] = true
	}
	for i, input := range inputs {
		if input.CommandID != p.CommandIDs[i] {
			return invalid
		}
		for _, id := range input.AttachmentIDs {
			if !files[id] {
				return invalid
			}
		}
	}
	digest, err := preparationDigest(*p)
	if err != nil || digest != p.ManifestDigest {
		return invalid
	}
	return nil
}

func ValidateCollaborationPreparationObservation(p CollaborationPreparationObservation) error {
	invalid := errors.New("invalid collaboration preparation observation")
	if !preparationIdentity(p.Identity) || !preparationUUID(p.EventID) || p.AttemptSeq != "1" || !preparationUUID(p.AttemptNonce) || !preparationHex(p.ManifestDigest, 64) {
		return invalid
	}
	switch p.Stage {
	case "attachments", "bundle", "git", "workspace":
	default:
		return invalid
	}
	if p.State == "prepared" && p.Code == "" {
		return nil
	}
	if p.State != "failed" {
		return invalid
	}
	switch p.Code {
	case "attachment_unavailable", "input_limit_exceeded", "bundle_unavailable", "git_unavailable", "configuration_changed", "cancelled", "preparation_unavailable":
		return nil
	default:
		return invalid
	}
}
