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

type HTTPError struct{ Status int }

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
	if !validUUID(runID) || !validUUID(in.DeploymentID) || !validUUID(in.InstallationID) || !validUUID(in.RuntimeID) || !validUUID(in.BindingID) || !positiveDecimal(in.BindingEpoch) || in.ProtocolVersion != protocol.CollaborationRuntimeProtocolVersion || in.Containment != protocol.CollaborationContainmentLinuxPIDNamespace {
		return out, ErrProtocol
	}
	if err := c.request(ctx, http.MethodPost, runPath(runID, "claim"), c.managedToken, in, &out); err != nil {
		return out, err
	}
	if validateIdentity(out.Identity) != nil || out.Identity.DeploymentID != in.DeploymentID || out.Identity.InstallationID != in.InstallationID || out.Identity.RunID != runID || out.TerminationToken == "" || !validUUID(out.AgentID) || !validUUID(out.ResourceOwnerID) || !validUUID(out.ExecutionWorkspaceID) || len(out.Inputs) == 0 || len(out.Inputs) > 1024 {
		return out, ErrProtocol
	}
	seen := make(map[string]bool)
	for _, input := range out.Inputs {
		if !validUUID(input.CommandID) || !validUUID(input.AuthorUserID) || !positiveDecimal(input.ParticipantEpoch) || !positiveDecimal(input.AccountEpoch) || !positiveDecimal(input.LifecycleEpoch) || seen[input.CommandID] {
			return out, ErrProtocol
		}
		seen[input.CommandID] = true
		for _, id := range input.AttachmentIDs {
			if !validUUID(id) {
				return out, ErrProtocol
			}
		}
	}
	return out, nil
}

func (c *Client) Permit(ctx context.Context, in protocol.CollaborationRunPermitRequest) (protocol.CollaborationRunPermitResponse, error) {
	var out protocol.CollaborationRunPermitResponse
	if c.managedToken == "" {
		return out, ErrUnsupported
	}
	if validateIdentity(in.Identity) != nil || in.AttemptSeq != "1" || !validUUID(in.AttemptNonce) || in.Containment != protocol.CollaborationContainmentLinuxPIDNamespace {
		return out, ErrProtocol
	}
	if err := c.request(ctx, http.MethodPost, runPath(in.Identity.RunID, "permit"), c.managedToken, in, &out); err != nil {
		return out, err
	}
	if out.Identity != in.Identity || out.AttemptSeq != in.AttemptSeq || out.AttemptNonce != in.AttemptNonce || out.BusinessToken == "" {
		return out, ErrProtocol
	}
	return out, nil
}

func (c *Client) StopIntent(ctx context.Context, id protocol.CollaborationRunIdentity, token string) (protocol.CollaborationRunStopIntent, error) {
	var out protocol.CollaborationRunStopIntent
	var raw struct {
		Identity      protocol.CollaborationRunIdentity `json:"identity"`
		StopRequested *bool                             `json:"stop_requested"`
	}
	if validateIdentity(id) != nil {
		return out, ErrProtocol
	}
	if err := c.request(ctx, http.MethodGet, runPath(id.RunID, "stop-intent"), token, nil, &raw); err != nil {
		return out, err
	}
	if raw.Identity != id || raw.StopRequested == nil {
		return out, ErrProtocol
	}
	out.Identity, out.StopRequested = raw.Identity, *raw.StopRequested
	return out, nil
}

func (c *Client) Outcome(ctx context.Context, in protocol.CollaborationRunOutcome, token string) error {
	if validateIdentity(in.Identity) != nil || in.AttemptSeq != "1" || !validUUID(in.AttemptNonce) || !validUUID(in.EventID) || protocol.ValidateCollaborationProviderResult(protocol.CollaborationProviderResult{Outcome: in.Outcome, Body: in.Body, Record: in.Record}) != nil {
		return ErrProtocol
	}
	return c.request(ctx, http.MethodPost, runPath(in.Identity.RunID, "outcome"), token, in, nil)
}

func (c *Client) Proof(ctx context.Context, in protocol.CollaborationRunProof, token string) (protocol.CollaborationRunProofReceipt, error) {
	var out protocol.CollaborationRunProofReceipt
	if validateIdentity(in.Identity) != nil || !validUUID(in.ReceiptID) {
		return out, ErrProtocol
	}
	if err := c.request(ctx, http.MethodPost, runPath(in.Identity.RunID, "proof"), token, in, &out); err != nil {
		return out, err
	}
	if out.Identity != in.Identity || out.ReceiptID != in.ReceiptID || !out.Accepted {
		return out, ErrProtocol
	}
	return out, nil
}

func runPath(id, action string) string { return "/api/collaboration/runtime/runs/" + id + "/" + action }

func (c *Client) request(ctx context.Context, method, path, token string, in, out any) error {
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
		return &HTTPError{Status: resp.StatusCode}
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024+1))
	if err != nil || len(raw) > 2*1024*1024 {
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
