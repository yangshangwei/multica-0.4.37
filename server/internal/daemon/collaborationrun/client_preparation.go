package collaborationrun

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strings"

	"github.com/multica-ai/multica/server/pkg/protocol"
)

func validDigest(value string) bool {
	_, err := hex.DecodeString(value)
	return len(value) == 64 && strings.ToLower(value) == value && err == nil
}

func (c *Client) capabilities(ctx context.Context, runtimeID string) (protocol.CollaborationRuntimeCapabilitiesResponse, error) {
	var out protocol.CollaborationRuntimeCapabilitiesResponse
	var raw struct {
		RuntimeID          string          `json:"runtime_id"`
		RuntimeConfigEpoch string          `json:"runtime_config_epoch"`
		Registration       json.RawMessage `json:"registration"`
	}
	if c.managedToken == "" || !validUUID(runtimeID) {
		return out, ErrProtocol
	}
	if err := c.request(ctx, http.MethodGet, "/api/collaboration/runtime/capabilities?runtime_id="+runtimeID, c.managedToken, nil, &raw); err != nil {
		return out, err
	}
	if raw.RuntimeID != runtimeID || !positiveDecimal(raw.RuntimeConfigEpoch) || len(raw.Registration) == 0 {
		return out, ErrProtocol
	}
	out.RuntimeID, out.RuntimeConfigEpoch = raw.RuntimeID, raw.RuntimeConfigEpoch
	if string(raw.Registration) != "null" && decodeStrict(raw.Registration, &out.Registration) != nil {
		return out, ErrProtocol
	}
	if out.Registration != nil && !validCapabilityRegistration(*out.Registration) {
		return out, ErrProtocol
	}
	return out, nil
}

func validCapabilityRegistration(r protocol.CollaborationRuntimeCapabilityRegistration) bool {
	return validUUID(r.DeploymentID) && validUUID(r.InstallationID) && validUUID(r.BindingID) && positiveDecimal(r.BindingEpoch) &&
		positiveDecimal(r.RuntimeConfigEpoch) && protocol.ValidateCollaborationRuntimeCapabilities(r.Capabilities) == nil
}

// EnsureCapabilities never adopts a newer epoch to replace a registration. A
// lost write is resolved by a read of the exact current full summary; a stale
// registration requires a new, explicit local configuration observation.
func (c *Client) EnsureCapabilities(ctx context.Context, identity protocol.CollaborationRunClaimRequest, expectedEpoch string, caps protocol.CollaborationRuntimeCapabilities) (protocol.CollaborationRuntimeCapabilitiesResponse, error) {
	var empty protocol.CollaborationRuntimeCapabilitiesResponse
	if c.managedToken == "" || !validUUID(identity.DeploymentID) || !validUUID(identity.InstallationID) || !validUUID(identity.RuntimeID) ||
		!validUUID(identity.BindingID) || !positiveDecimal(identity.BindingEpoch) || !positiveDecimal(expectedEpoch) ||
		protocol.ValidateCollaborationRuntimeCapabilities(caps) != nil {
		return empty, ErrProtocol
	}
	current := func(out protocol.CollaborationRuntimeCapabilitiesResponse) bool {
		r := out.Registration
		return out.RuntimeID == identity.RuntimeID && positiveDecimal(out.RuntimeConfigEpoch) && r != nil && validCapabilityRegistration(*r) &&
			r.RuntimeConfigEpoch == out.RuntimeConfigEpoch && r.DeploymentID == identity.DeploymentID && r.InstallationID == identity.InstallationID &&
			r.BindingID == identity.BindingID && r.BindingEpoch == identity.BindingEpoch && reflect.DeepEqual(r.Capabilities, caps)
	}
	out, err := c.capabilities(ctx, identity.RuntimeID)
	if err != nil {
		return empty, err
	}
	if current(out) {
		return out, nil
	}
	if out.RuntimeConfigEpoch != expectedEpoch {
		return empty, ErrProtocol
	}
	in := protocol.CollaborationRuntimeCapabilitiesRequest{ProtocolVersion: protocol.CollaborationRuntimeProtocolVersion,
		DeploymentID: identity.DeploymentID, InstallationID: identity.InstallationID, RuntimeID: identity.RuntimeID, BindingID: identity.BindingID,
		BindingEpoch: identity.BindingEpoch, ExpectedRuntimeConfigEpoch: expectedEpoch, Capabilities: caps}
	out = empty
	writeErr := c.request(ctx, http.MethodPost, "/api/collaboration/runtime/capabilities", c.managedToken, in, &out)
	if writeErr == nil && current(out) {
		return out, nil
	}
	// The read is safe even for an explicit refusal, and also covers malformed
	// success responses. There is deliberately no replacement POST here.
	out, err = c.capabilities(ctx, identity.RuntimeID)
	if err == nil && current(out) {
		return out, nil
	}
	return empty, errors.Join(ErrProtocol, writeErr, err)
}

func businessPreparationToken(token string) bool {
	return strings.HasPrefix(token, "mcb_") && len(token) > 4 && !strings.ContainsAny(token, "\r\n\x00")
}

// CheckPreparation rechecks current admission without replacing any frozen
// field. A changed manifest cannot become a new local preparation implicitly.
func (c *Client) CheckPreparation(ctx context.Context, expected protocol.CollaborationPreparation, inputs []protocol.CollaborationRunInput, token string) error {
	if !businessPreparationToken(token) || protocol.ValidateCollaborationPreparation(&expected, expected.Identity, inputs) != nil {
		return ErrProtocol
	}
	var got protocol.CollaborationPreparation
	if err := c.request(ctx, http.MethodGet, runPath(expected.Identity.RunID, "preparation"), token, nil, &got); err != nil {
		return err
	}
	if protocol.ValidateCollaborationPreparation(&got, expected.Identity, inputs) != nil || !reflect.DeepEqual(got, expected) {
		return ErrProtocol
	}
	return nil
}

// DownloadAttachment writes only exact, run-bound bytes to the caller's private
// staging file. Names are display metadata, never paths. Partial staging must
// be discarded by the caller on any error, including a short/long response.
func (c *Client) DownloadAttachment(ctx context.Context, manifest protocol.CollaborationPreparation, file protocol.CollaborationPreparationAttachment, token string, dst io.Writer) error {
	if !businessPreparationToken(token) || validateIdentity(manifest.Identity) != nil || !validUUID(file.ID) || !positiveDecimal(file.Version) ||
		file.SizeBytes < 0 || file.SizeBytes > manifest.Limits.PerFileBytes || file.SizeBytes > 100<<20 || dst == nil {
		return ErrProtocol
	}
	found := false
	for _, expected := range manifest.Attachments {
		if expected == file {
			found = true
			break
		}
	}
	if !found {
		return ErrProtocol
	}
	path := runPath(manifest.Identity.RunID, "attachments/"+file.ID+"/content?version="+file.Version)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return ErrProtocol
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept-Encoding", "identity")
	resp, err := c.http.Do(req)
	if err != nil {
		return ErrTransport
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return &HTTPError{Status: resp.StatusCode}
	}
	for name, value := range map[string]string{"X-Multica-Run-ID": manifest.Identity.RunID, "X-Multica-Attachment-ID": file.ID, "X-Multica-Attachment-Version": file.Version} {
		if len(resp.Header.Values(name)) != 1 || resp.Header.Get(name) != value {
			return ErrProtocol
		}
	}
	if encoding := resp.Header.Get("Content-Encoding"); encoding != "" && encoding != "identity" {
		return ErrProtocol
	}
	if resp.Uncompressed || resp.ContentLength >= 0 && resp.ContentLength != file.SizeBytes || resp.Header.Get("Content-Range") != "" {
		return ErrProtocol
	}
	n, err := io.Copy(dst, io.LimitReader(resp.Body, file.SizeBytes+1))
	if err != nil || n != file.SizeBytes {
		return ErrProtocol
	}
	return nil
}

func (c *Client) ObservePreparation(ctx context.Context, observation protocol.CollaborationPreparationObservation, token string) error {
	if !businessPreparationToken(token) || protocol.ValidateCollaborationPreparationObservation(observation) != nil {
		return ErrProtocol
	}
	var receipt protocol.CollaborationPreparationObservationReceipt
	if err := c.request(ctx, http.MethodPost, runPath(observation.Identity.RunID, "preparation/observations"), token, observation, &receipt); err != nil {
		return err
	}
	if receipt.Identity != observation.Identity || receipt.EventID != observation.EventID || receipt.ManifestDigest != observation.ManifestDigest || !receipt.Recorded {
		return ErrProtocol
	}
	return nil
}
