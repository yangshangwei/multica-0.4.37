package cli

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strconv"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// CollaborationHandoffReference names one envelope source exactly as the
// server listed it. The server rechecks every field before returning text.
type CollaborationHandoffReference struct {
	SourceID    string
	Kind        string
	SourceRunID string
	Version     string
	SHA256      string
	Offset      int
}

// Handoff cursors are opaque to the CLI. Bound their size and alphabet here;
// the server decodes them and checks that they belong to this run.
func validCollaborationHandoffCursor(cursor string) bool {
	if cursor == "" || len(cursor) > 1024 {
		return false
	}
	for _, c := range cursor {
		if !(c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
			return false
		}
	}
	return true
}

// ReadHandoff returns the run's live handoff projection, not the frozen
// startup snapshot that was injected into the prompt.
func (c *CollaborationContext) ReadHandoff(ctx context.Context, client *APIClient, cursor string) (protocol.CollaborationHandoffEnvelope, error) {
	var out protocol.CollaborationHandoffEnvelope
	path := c.Path("/handoff")
	if cursor != "" {
		if !validCollaborationHandoffCursor(cursor) {
			return out, errors.New("--cursor must be a next_cursor or readers_cursor value copied from a handoff page")
		}
		path += "?" + url.Values{"cursor": {cursor}}.Encode()
	}
	var raw json.RawMessage
	if err := client.GetJSON(ctx, path, &raw); err != nil {
		return out, CollaborationError(err, "")
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return protocol.CollaborationHandoffEnvelope{}, err
	}
	if out.IssueID != c.Identity.IssueID || out.RunID != c.Identity.RunID || out.Mode != "live" || out.SnapshotID != "" {
		return protocol.CollaborationHandoffEnvelope{}, errors.New("collaboration handoff does not match the admitted run and issue")
	}
	// A page that hands back the cursor it was given would loop forever.
	if cursor != "" {
		next := []string{out.Inputs.NextCursor, out.Results.NextCursor, out.WakeupCauses.NextCursor}
		for _, input := range out.Inputs.Items {
			next = append(next, input.ReadersCursor)
		}
		for _, value := range next {
			if value == cursor {
				return protocol.CollaborationHandoffEnvelope{}, errors.New("collaboration handoff page did not advance its cursor")
			}
		}
	}
	return out, nil
}

// ReadHandoffSource returns one chunk of a handoff source's full text. The
// envelope bounds source text; this read follows next_offset to the end.
func (c *CollaborationContext) ReadHandoffSource(ctx context.Context, client *APIClient, ref CollaborationHandoffReference) (protocol.CollaborationHandoffSourcePage, error) {
	var out protocol.CollaborationHandoffSourcePage
	id, err := uuid.Parse(ref.SourceID)
	if err != nil || id == uuid.Nil || id.String() != ref.SourceID {
		return out, errors.New("handoff source requires the source UUID from the envelope")
	}
	switch ref.Kind {
	case "handoff":
		// Handoff notes are issue rows, not run events: they have no run or version.
		if ref.SourceRunID != "" || ref.Version != "" {
			return out, errors.New("a handoff-kind source has no --source-run-id or --version")
		}
	case "delegation", "provider_result", "squad_activity":
		run, err := uuid.Parse(ref.SourceRunID)
		if err != nil || run == uuid.Nil || run.String() != ref.SourceRunID || !protocol.HandoffDecimal(ref.Version, false) {
			return out, errors.New("this source kind requires the source's run_id as --source-run-id and its positive version as --version")
		}
	default:
		return out, errors.New("--kind must be handoff, delegation, provider_result or squad_activity")
	}
	if !protocol.HandoffDigest(ref.SHA256) {
		return out, errors.New("--sha256 must be the source's lowercase 64-character sha256")
	}
	if ref.Offset < 0 {
		return out, errors.New("--offset must be 0 or a next_offset value")
	}
	query := url.Values{"kind": {ref.Kind}, "sha256": {ref.SHA256}}
	if ref.SourceRunID != "" {
		query.Set("source_run_id", ref.SourceRunID)
		query.Set("version", ref.Version)
	}
	if ref.Offset > 0 {
		query.Set("offset", strconv.Itoa(ref.Offset))
	}
	var raw json.RawMessage
	if err := client.GetJSON(ctx, c.Path("/handoff/sources/"+url.PathEscape(ref.SourceID)+"?")+query.Encode(), &raw); err != nil {
		return out, CollaborationError(err, "")
	}
	out, err = protocol.DecodeCollaborationHandoffSourcePage(raw)
	if err != nil {
		return protocol.CollaborationHandoffSourcePage{}, err
	}
	s := out.Source
	if out.IssueID != c.Identity.IssueID || out.RunID != c.Identity.RunID || s.ID != ref.SourceID || s.Kind != ref.Kind ||
		s.RunID != ref.SourceRunID || s.Version != ref.Version || s.SHA256 != ref.SHA256 || out.Offset != ref.Offset {
		return protocol.CollaborationHandoffSourcePage{}, errors.New("handoff source does not match the admitted read")
	}
	return out, nil
}
