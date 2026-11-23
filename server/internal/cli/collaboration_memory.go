package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/multica-ai/multica/server/pkg/protocol"
)

// ReadMemory returns the latest shared-context ("memory") version of the
// admitted issue. Every response must match the admitted run and issue.
func (c *CollaborationContext) ReadMemory(ctx context.Context, client *APIClient) (protocol.CollaborationRunMemory, error) {
	var out protocol.CollaborationRunMemory
	if err := client.GetJSON(ctx, c.Path("/memory"), &out); err != nil {
		return out, CollaborationMemoryError(err, c.Identity.IssueID, "")
	}
	if out.IssueID != c.Identity.IssueID || out.RunID != c.Identity.RunID {
		return out, errors.New("collaboration response does not match the admitted run and issue")
	}
	return out, nil
}

// UpdateMemory replaces the whole document from the version the caller merged.
// A failure keeps the original request ID as the retry identity.
func (c *CollaborationContext) UpdateMemory(ctx context.Context, client *APIClient, in protocol.CollaborationRunMemoryUpdate) (protocol.CollaborationRunMemoryReceipt, error) {
	var out protocol.CollaborationRunMemoryReceipt
	if err := client.PutJSON(ctx, c.Path("/memory"), in, &out); err != nil {
		return out, CollaborationMemoryError(err, c.Identity.IssueID, in.RequestID)
	}
	if out.IssueID != c.Identity.IssueID || out.RunID != c.Identity.RunID || out.RequestID != in.RequestID || out.Code != "memory_updated" || out.Version == nil {
		return out, CollaborationError(errors.New("invalid collaboration memory receipt"), in.RequestID)
	}
	return out, nil
}

// CollaborationMemoryError turns a definite shared-context refusal into the
// next action. Capability rejection and unknown results keep the shared
// collaboration guidance, including retrying with the original request ID.
func CollaborationMemoryError(err error, issueID, requestID string) error {
	var response *HTTPError
	if !errors.As(err, &response) || response.StatusCode < 400 || response.StatusCode >= 500 ||
		response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return CollaborationError(err, requestID)
	}
	var refusal protocol.CollaborationRunMemoryReceipt
	_ = json.Unmarshal([]byte(response.Body), &refusal)
	switch refusal.Code {
	case "stale_memory_version":
		return WithUserMessage(fmt.Sprintf("Collaboration memory changed: the current version is %s. Run `multica collaboration memory show %s`, merge your changes into that version, then update again with --base-version %s.", refusal.CurrentVersion, issueID, refusal.CurrentVersion), err)
	case "memory_too_large":
		size := "the submitted document"
		if refusal.Bytes != nil {
			size = fmt.Sprintf("%d bytes", *refusal.Bytes)
		}
		return WithUserMessage(fmt.Sprintf("Collaboration memory is too large: %s exceeds the %d-byte limit. Shorten it (keep conclusions and pointers, move details to a comment) and update again.", size, refusal.MaxBytes), err)
	case "memory_write_conflict":
		// The server compares both the base version and the content.
		return WithUserMessage("This --request-id was already used for a different collaboration memory update (another --base-version or content). Use a new request ID for a new update.", err)
	case "memory_unsupported":
		return WithUserMessage("Collaboration memory is unavailable for this run.", err)
	case "invalid_memory_content":
		return WithUserMessage("Collaboration memory must be UTF-8 text without NUL characters.", err)
	}
	if response.StatusCode == http.StatusRequestEntityTooLarge {
		// The request body exceeded the read bound before the service could
		// measure the document, so there is no receipt to report sizes from.
		return WithUserMessage(fmt.Sprintf("Collaboration memory request is too large for the server. Shorten the document (keep conclusions and pointers, move details to a comment), then run `multica collaboration memory show %s` to check the limit and update again.", issueID), err)
	}
	return CollaborationError(err, requestID)
}
