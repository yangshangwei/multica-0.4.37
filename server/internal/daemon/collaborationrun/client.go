package collaborationrun

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/multica-ai/multica/server/pkg/protocol"
)

var ErrProtocol = errors.New("invalid collaboration runtime response")

// ErrTransport means no HTTP response was observed. The server may still have
// committed the request, so callers replay the same idempotent payload.
var ErrTransport = errors.New("collaboration runtime transport failed")

// Client has no workspace-header, cookie, PAT or ordinary-task fallback. Even
// when the managed credential has been revoked, Proof uses only its old narrow
// termination capability and can be replayed independently of Claim.
type Client struct {
	baseURL      string
	managedToken string
	http         *http.Client
}

func (c *Client) String() string   { return "collaboration client (credentials redacted)" }
func (c *Client) GoString() string { return c.String() }

// HTTPError carries the response status and, when the bounded error body is a
// collaboration refusal, its stable code. The body text itself is discarded.
type HTTPError struct {
	Status int
	Code   string
}

func (e *HTTPError) Error() string { return fmt.Sprintf("collaboration runtime HTTP %d", e.Status) }

func NewClient(baseURL, managedToken string, transport http.RoundTripper) (*Client, error) {
	u, err := url.Parse(baseURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (managedToken != "" && !strings.HasPrefix(managedToken, "mdt_")) {
		return nil, errors.New("collaboration runtime requires a managed endpoint and credential")
	}
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), managedToken: managedToken, http: &http.Client{Transport: transport, Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}

func (c *Client) List(ctx context.Context, runtimeID string) (protocol.CollaborationRunListResponse, error) {
	var out protocol.CollaborationRunListResponse
	if c.managedToken == "" {
		return out, ErrUnsupported
	}
	if !validUUID(runtimeID) {
		return out, ErrProtocol
	}
	err := c.request(ctx, http.MethodGet, "/api/collaboration/runtime/runs?runtime_id="+runtimeID, c.managedToken, nil, &out)
	if err != nil {
		return out, err
	}
	seen := make(map[string]bool)
	if out.RunIDs == nil || len(out.RunIDs) > 1000 {
		return out, ErrProtocol
	}
	for _, id := range out.RunIDs {
		if !validUUID(id) || seen[id] {
			return out, ErrProtocol
		}
		seen[id] = true
	}
	return out, nil
}

func (c *Client) Claim(ctx context.Context, runID string, in protocol.CollaborationRunClaimRequest) (protocol.CollaborationRunClaimResponse, error) {
	var out protocol.CollaborationRunClaimResponse
	if c.managedToken == "" {
		return out, ErrUnsupported
	}
	if !validUUID(runID) || !validUUID(in.DeploymentID) || !validUUID(in.InstallationID) || !validUUID(in.RuntimeID) || !validUUID(in.BindingID) || !positiveDecimal(in.BindingEpoch) || !protocol.CollaborationRuntimeModeVersion(in.ProtocolVersion, in.Containment) {
		return out, ErrProtocol
	}
	host := in.ProtocolVersion == protocol.CollaborationHostRuntimeProtocolVersion
	if host {
		if in.PreparationVersion != protocol.CollaborationHostPreparationVersion || !validDigest(in.CapabilityDigest) || in.BundleDigest != "" ||
			in.MemoryVersion != 0 && in.MemoryVersion != protocol.CollaborationMemoryVersion {
			return out, ErrProtocol
		}
	} else if in.MemoryVersion != 0 || in.CapabilityDigest != "" || in.PreparationVersion != 0 && in.PreparationVersion != protocol.CollaborationPreparationVersion ||
		in.PreparationVersion == 0 && in.BundleDigest != "" || in.PreparationVersion != 0 && !validDigest(in.BundleDigest) {
		return out, ErrProtocol
	}
	if err := c.request(ctx, http.MethodPost, runPath(runID, "claim"), c.managedToken, in, &out); err != nil {
		return out, err
	}
	return out, validateClaimResponse(out, runID, in)
}

// Retained claims pass the same validation before they can resume waiting for
// their session. A private ledger file is not a substitute for input validation.
func validateClaimResponse(out protocol.CollaborationRunClaimResponse, runID string, in protocol.CollaborationRunClaimRequest) error {
	if validateIdentity(out.Identity) != nil || out.Identity.DeploymentID != in.DeploymentID || out.Identity.InstallationID != in.InstallationID || out.Identity.RunID != runID || out.TerminationToken == "" || !validUUID(out.AgentID) || !validUUID(out.ResourceOwnerID) || !validUUID(out.ExecutionWorkspaceID) || len(out.Inputs) == 0 || len(out.Inputs) > 1024 {
		return ErrProtocol
	}
	if in.ProtocolVersion == protocol.CollaborationHostRuntimeProtocolVersion {
		if out.ProtocolVersion != in.ProtocolVersion || out.Containment != in.Containment || out.HostContext == nil || out.HostContext.RuntimeID != in.RuntimeID {
			return ErrProtocol
		}
		if _, err := hostSessionKey(out); err != nil || out.HostContext.ParentRunID != nil && !validUUID(*out.HostContext.ParentRunID) ||
			out.Preparation.Resource.AgentOwnerUserID != out.HostContext.AgentOwnerUserID {
			return ErrProtocol
		}
	} else if out.ProtocolVersion != 0 || out.Containment != "" || out.HostContext != nil {
		return ErrProtocol
	}
	seen := make(map[string]bool)
	for _, input := range out.Inputs {
		if !validUUID(input.CommandID) || !validUUID(input.AuthorUserID) || !positiveDecimal(input.ParticipantEpoch) || !positiveDecimal(input.AccountEpoch) || !positiveDecimal(input.LifecycleEpoch) || seen[input.CommandID] {
			return ErrProtocol
		}
		seen[input.CommandID] = true
		for _, id := range input.AttachmentIDs {
			if !validUUID(id) || in.PreparationVersion == 0 {
				return ErrProtocol
			}
		}
	}
	if in.PreparationVersion == 0 {
		if out.Preparation != nil {
			return ErrProtocol
		}
	} else if protocol.ValidateCollaborationPreparation(out.Preparation, out.Identity, out.Inputs) != nil || out.Preparation.Version != in.PreparationVersion ||
		out.Preparation.Provider.BundleDigest != in.BundleDigest || out.Preparation.CapabilityDigest != in.CapabilityDigest || out.Preparation.Resource.RuntimeID != in.RuntimeID ||
		out.Preparation.Resource.AgentID != out.AgentID || out.Preparation.Resource.OwnerUserID != out.ResourceOwnerID ||
		out.Preparation.Resource.ExecutionWorkspaceID != out.ExecutionWorkspaceID {
		return ErrProtocol
	}
	return nil
}

func (c *Client) Permit(ctx context.Context, in protocol.CollaborationRunPermitRequest) (protocol.CollaborationRunPermitResponse, error) {
	var out protocol.CollaborationRunPermitResponse
	if c.managedToken == "" {
		return out, ErrUnsupported
	}
	if validateIdentity(in.Identity) != nil || !validUUID(in.AttemptNonce) {
		return out, ErrProtocol
	}
	if in.ProtocolVersion == protocol.CollaborationHostRuntimeProtocolVersion {
		if in.Containment != protocol.CollaborationContainmentHostProcessSupervision || !validHostAttempt(in.AttemptSeq) || in.LaunchDescriptor == nil || protocol.ValidateCollaborationLaunchDescriptor(*in.LaunchDescriptor) != nil {
			return out, ErrProtocol
		}
	} else if in.ProtocolVersion != 0 || in.AttemptSeq != "1" || in.Containment != protocol.CollaborationContainmentLinuxPIDNamespace || in.LaunchDescriptor != nil {
		return out, ErrProtocol
	}
	if err := c.request(ctx, http.MethodPost, runPath(in.Identity.RunID, "permit"), c.managedToken, in, &out); err != nil {
		return out, err
	}
	if out.ProtocolVersion != in.ProtocolVersion || out.Identity != in.Identity || out.AttemptSeq != in.AttemptSeq || out.AttemptNonce != in.AttemptNonce || out.BusinessToken == "" {
		return out, ErrProtocol
	}
	return out, nil
}

func (c *Client) StopIntent(ctx context.Context, id protocol.CollaborationRunIdentity, token string) (protocol.CollaborationRunStopIntent, error) {
	return c.stopIntent(ctx, id, token, 0, "")
}

func (c *Client) HostStopIntent(ctx context.Context, id protocol.CollaborationRunIdentity, token string) (protocol.CollaborationRunStopIntent, error) {
	return c.stopIntent(ctx, id, token, protocol.CollaborationHostRuntimeProtocolVersion, protocol.CollaborationContainmentHostProcessSupervision)
}

func (c *Client) stopIntent(ctx context.Context, id protocol.CollaborationRunIdentity, token string, version int, containment string) (protocol.CollaborationRunStopIntent, error) {
	var out protocol.CollaborationRunStopIntent
	var raw struct {
		ProtocolVersion int                               `json:"protocol_version,omitempty"`
		Containment     string                            `json:"containment,omitempty"`
		Identity        protocol.CollaborationRunIdentity `json:"identity"`
		StopRequested   *bool                             `json:"stop_requested"`
	}
	if validateIdentity(id) != nil {
		return out, ErrProtocol
	}
	if err := c.request(ctx, http.MethodGet, runPath(id.RunID, "stop-intent"), token, nil, &raw); err != nil {
		return out, err
	}
	if raw.Identity != id || raw.StopRequested == nil || raw.ProtocolVersion != version || raw.Containment != containment {
		return out, ErrProtocol
	}
	out.Identity, out.StopRequested = raw.Identity, *raw.StopRequested
	out.ProtocolVersion, out.Containment = raw.ProtocolVersion, raw.Containment
	return out, nil
}

func (c *Client) Outcome(ctx context.Context, in protocol.CollaborationRunOutcome, token string) error {
	if validateIdentity(in.Identity) != nil || !validUUID(in.AttemptNonce) || !validUUID(in.EventID) || protocol.ValidateCollaborationProviderResult(protocol.CollaborationProviderResult{Outcome: in.Outcome, Body: in.Body, Record: in.Record}) != nil {
		return ErrProtocol
	}
	if in.ProtocolVersion == protocol.CollaborationHostRuntimeProtocolVersion {
		if in.Containment != protocol.CollaborationContainmentHostProcessSupervision || !validHostAttempt(in.AttemptSeq) {
			return ErrProtocol
		}
	} else if in.ProtocolVersion != 0 || in.Containment != "" || in.AttemptSeq != "1" {
		return ErrProtocol
	}
	return c.request(ctx, http.MethodPost, runPath(in.Identity.RunID, "outcome"), token, in, nil)
}

func (c *Client) Proof(ctx context.Context, in protocol.CollaborationRunProof, token string) (protocol.CollaborationRunProofReceipt, error) {
	var out protocol.CollaborationRunProofReceipt
	if validateIdentity(in.Identity) != nil || !validUUID(in.ReceiptID) {
		return out, ErrProtocol
	}
	if in.ProtocolVersion == protocol.CollaborationHostRuntimeProtocolVersion {
		if protocol.ValidateCollaborationHostProof(in) != nil {
			return out, ErrProtocol
		}
	} else if in.ProtocolVersion != 0 || in.Containment != protocol.CollaborationContainmentLinuxPIDNamespace || in.Launches != nil || in.Cleanup != nil {
		return out, ErrProtocol
	}
	if err := c.request(ctx, http.MethodPost, runPath(in.Identity.RunID, "proof"), token, in, &out); err != nil {
		return out, err
	}
	if out.ProtocolVersion != in.ProtocolVersion || out.Identity != in.Identity || out.ReceiptID != in.ReceiptID || !out.Accepted {
		return out, ErrProtocol
	}
	return out, nil
}

// collaborationMemoryResponseLimit bounds a shared-context response: the
// 512 KiB hard ceiling can expand six-fold under JSON escaping.
const collaborationMemoryResponseLimit = 6*512<<10 + 64<<10

// MemoryBase returns the shared-context version this run starts from. Repeated
// calls for the same run, including a fresh-session retry, return the same
// version unless that version was purged.
func (c *Client) MemoryBase(ctx context.Context, id protocol.CollaborationRunIdentity, token string) (protocol.CollaborationRunMemory, error) {
	var out protocol.CollaborationRunMemory
	if validateIdentity(id) != nil {
		return out, ErrProtocol
	}
	if err := c.requestLimit(ctx, http.MethodPost, runPath(id.RunID, "memory/base"), token, struct{}{}, &out, collaborationMemoryResponseLimit); err != nil {
		return out, err
	}
	if out.IssueID != id.IssueID || out.RunID != id.RunID || out.Version != "0" && !positiveDecimal(out.Version) ||
		out.Version == "0" && out.Content != "" || out.Bytes != len(out.Content) || !utf8.ValidString(out.Content) || strings.ContainsRune(out.Content, 0) {
		return out, ErrProtocol
	}
	return out, nil
}

func runPath(id, action string) string { return "/api/collaboration/runtime/runs/" + id + "/" + action }

func (c *Client) request(ctx context.Context, method, path, token string, in, out any) error {
	return c.requestLimit(ctx, method, path, token, in, out, 2*1024*1024)
}

func (c *Client) requestLimit(ctx context.Context, method, path, token string, in, out any, limit int64) error {
	if token == "" || strings.ContainsAny(token, "\r\n") {
		return ErrProtocol
	}
	var body []byte
	var err error
	if in != nil {
		body, err = json.Marshal(in)
		if err != nil {
			return ErrProtocol
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return ErrProtocol
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return ErrTransport
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		failure := &HTTPError{Status: resp.StatusCode}
		var refusal struct {
			Code string `json:"code"`
		}
		if body, err := io.ReadAll(io.LimitReader(resp.Body, 4096)); err == nil && json.Unmarshal(body, &refusal) == nil && len(refusal.Code) <= 128 {
			failure.Code = refusal.Code
		}
		return failure
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return ErrTransport
	}
	if int64(len(raw)) > limit {
		return ErrProtocol
	}
	if out == nil {
		return nil
	}
	if err := decodeStrict(raw, out); err != nil {
		return ErrProtocol
	}
	return nil
}
