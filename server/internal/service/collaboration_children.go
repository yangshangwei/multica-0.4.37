package service

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/util"
)

const CollaborationChildrenChanged = "collaboration:children_changed"

// Ordinary relationship owners call this only after commit. It changes no
// collaboration version/participant/queue state and carries no child content.
// Like ordinary issue events it is a transient hint; snapshot reads recover loss.
func (s *IssueService) PublishCollaborationChildrenChanged(ctx context.Context, parents ...pgtype.UUID) {
	if s == nil || s.Bus == nil || s.Queries == nil {
		return
	}
	seen := make(map[pgtype.UUID]bool, len(parents))
	for _, parent := range parents {
		if !parent.Valid || seen[parent] {
			continue
		}
		seen[parent] = true
		shared, err := s.Queries.IsCollaborationIssue(ctx, parent)
		if err != nil {
			slog.Warn("collaboration child refresh deferred")
			continue
		}
		if shared {
			s.Bus.Publish(events.Event{Type: CollaborationChildrenChanged, Payload: map[string]any{"issue_id": util.UUIDToString(parent)}})
		}
	}
}
