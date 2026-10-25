package collaborationrun

import (
	"context"
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
