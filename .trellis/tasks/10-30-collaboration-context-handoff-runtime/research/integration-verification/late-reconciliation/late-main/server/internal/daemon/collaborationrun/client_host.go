package collaborationrun

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/multica-ai/multica/server/pkg/protocol"
)

func validHostAttempt(value string) bool {
	n, err := strconv.ParseInt(value, 10, 64)
	return err == nil && n > 0 && n <= protocol.CollaborationHostMaxLaunches && strconv.FormatInt(n, 10) == value
}

func (c *Client) LaunchTermination(ctx context.Context, in protocol.CollaborationLaunchTermination, token string) (protocol.CollaborationLaunchTerminationReceipt, error) {
	var out protocol.CollaborationLaunchTerminationReceipt
	if protocol.ValidateCollaborationLaunchTermination(in) != nil {
		return out, ErrProtocol
	}
	var raw struct {
		ProtocolVersion int                               `json:"protocol_version"`
		Identity        protocol.CollaborationRunIdentity `json:"identity"`
		LaunchID        string                            `json:"launch_id"`
		EventID         string                            `json:"event_id"`
		Accepted        *bool                             `json:"accepted"`
		Authorized      *bool                             `json:"authorized"`
	}
	if err := c.request(ctx, http.MethodPost, runPath(in.Identity.RunID, "launches/"+in.LaunchID+"/termination"), token, in, &raw); err != nil {
		return out, err
	}
	if raw.ProtocolVersion != protocol.CollaborationHostRuntimeProtocolVersion || raw.Identity != in.Identity || raw.LaunchID != in.LaunchID ||
		raw.EventID != in.EventID || raw.Accepted == nil || !*raw.Accepted || raw.Authorized == nil || !*raw.Authorized && in.Kind != "not_started" {
		return out, ErrProtocol
	}
	return protocol.CollaborationLaunchTerminationReceipt{ProtocolVersion: raw.ProtocolVersion, Identity: raw.Identity, LaunchID: raw.LaunchID,
		EventID: raw.EventID, Accepted: *raw.Accepted, Authorized: *raw.Authorized}, nil
}

func (c *Client) Checkpoint(ctx context.Context, in protocol.CollaborationRecordingCheckpoint, token string) (protocol.CollaborationCheckpointReceipt, error) {
	var out protocol.CollaborationCheckpointReceipt
	if protocol.ValidateCollaborationRecordingCheckpoint(in) != nil {
		return out, ErrProtocol
	}
	if err := c.request(ctx, http.MethodPost, runPath(in.Identity.RunID, "checkpoints"), token, in, &out); err != nil {
		return out, err
	}
	if out.Version != 1 || out.Identity != in.Identity || out.StreamID != in.StreamID || out.Sequence != in.Sequence || !out.Accepted || out.RetainedThrough != in.MessageOffset+len(in.Messages) {
		return protocol.CollaborationCheckpointReceipt{}, ErrProtocol
	}
	return out, nil
}

func (c *Client) RecordingSeal(ctx context.Context, in protocol.CollaborationRecordingSeal, token string) (protocol.CollaborationRecordingRetention, error) {
	var out protocol.CollaborationRecordingRetention
	if protocol.ValidateCollaborationRecordingSeal(in) != nil {
		return out, ErrProtocol
	}
	// Preserve the shared DTO while distinguishing missing bounds from valid zero/null.
	var raw struct {
		protocol.CollaborationRecordingRetention
		RetainedThrough *int            `json:"retained_through"`
		ObservedThrough json.RawMessage `json:"observed_through"`
	}
	if err := c.request(ctx, http.MethodPost, runPath(in.Identity.RunID, "recording/seal"), token, in, &raw); err != nil {
		return out, err
	}
	if raw.RetainedThrough == nil || len(raw.ObservedThrough) == 0 {
		return out, ErrProtocol
	}
	out = raw.CollaborationRecordingRetention
	out.RetainedThrough = *raw.RetainedThrough
	if err := json.Unmarshal(raw.ObservedThrough, &out.ObservedThrough); err != nil {
		return protocol.CollaborationRecordingRetention{}, ErrProtocol
	}
	retention := in
	retention.AcknowledgedThrough = out.RetainedThrough
	retention.ObservedThrough = out.ObservedThrough
	if out.Version != 1 || out.StreamID != in.StreamID || !out.Sealed || out.Reason != in.Reason || protocol.ValidateCollaborationRecordingSeal(retention) != nil {
		return protocol.CollaborationRecordingRetention{}, ErrProtocol
	}
	return out, nil
}
