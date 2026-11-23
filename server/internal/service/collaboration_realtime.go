package service

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/realtime"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// Authorization is deliberately uncached: source membership and participation
// are independent and either can change while a frame waits in the send queue.
func (s *CollaborationService) AuthorizeRealtimeScope(ctx context.Context, userID, scope, id string) (bool, error) {
	principal, ok := collaboration.PrincipalFromContext(ctx)
	if !ok || !principal.Human() || principal.UserID != userID {
		return false, nil
	}
	var err error
	switch scope {
	case realtime.ScopeAccount:
		if id != userID {
			return false, nil
		}
		_, err = collaborationNotificationViewer(ctx, s.Queries)
	case realtime.ScopeIssue:
		issue, parseErr := util.ParseUUID(id)
		if parseErr != nil {
			return false, nil
		}
		_, err = s.ResolveIssueAccess(ctx, nil, issue)
	default:
		return false, nil
	}
	var denial *CollaborationError
	if errors.As(err, &denial) && (denial.Status == http.StatusForbidden || denial.Status == http.StatusNotFound) {
		return false, nil
	}
	return err == nil, err
}

// CollaborationRealtime drains the existing event outbox. It only publishes
// invalidation metadata, never event payloads, source content or run credentials.
type CollaborationRealtime struct {
	Queries   *db.Queries
	TxStarter TxStarter
	Publisher realtime.RelayPublisher
}

type collaborationRealtimeChange struct {
	IssueID        pgtype.UUID `json:"issue_id"`
	EventSequence  int64       `json:"event_seq,string"`
	Revision       int64       `json:"collaboration_revision,string"`
	AuthorityEpoch int64       `json:"authority_epoch,string"`
}

func (s *CollaborationRealtime) Run(ctx context.Context) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		if _, err := s.DeliverPending(ctx, 100); err != nil && ctx.Err() == nil {
			// Database/relay errors can contain untrusted detail. Keep this
			// operational signal free of persisted event payloads or credentials.
			slog.Warn("collaboration realtime delivery deferred")
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *CollaborationRealtime) DeliverPending(ctx context.Context, limit int) (int, error) {
	if limit < 1 || limit > 1000 || s.Publisher == nil {
		return 0, errors.New("invalid collaboration realtime configuration")
	}
	for delivered := 0; delivered < limit; delivered++ {
		found, err := s.deliverNext(ctx)
		if err != nil || !found {
			return delivered, err
		}
	}
	return limit, nil
}

func (s *CollaborationRealtime) deliverNext(ctx context.Context) (bool, error) {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer rollbackCollaborationQueue(ctx, tx)
	q := s.Queries.WithTx(tx)
	event, err := q.LockNextCollaborationRealtimeEvent(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	change := collaborationRealtimeChange{IssueID: event.IssueID, EventSequence: event.EventSeq, Revision: event.Revision, AuthorityEpoch: event.AuthorityEpoch}
	publish := func(scope, id, suffix, kind string, payload any) error {
		frame, err := json.Marshal(struct {
			Type    string `json:"type"`
			Payload any    `json:"payload"`
		}{kind, payload})
		if err != nil {
			return err
		}
		return s.Publisher.PublishWithID(scope, id, "", frame, util.UUIDToString(event.ID)+":"+suffix)
	}
	if id, err := util.ParseUUID(event.RevokedUserID); err == nil {
		participant, err := q.GetCollaborationParticipant(ctx, db.GetCollaborationParticipantParams{IssueID: event.IssueID, UserID: id})
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return false, err
		}
		// A delayed old removal must not discard a newly joined viewer's draft.
		if errors.Is(err, pgx.ErrNoRows) || participant.RevokedAt.Valid {
			if err = publish(realtime.ScopeAccount, util.UUIDToString(id), "access", "collaboration:access_changed", change); err != nil {
				return false, err
			}
		}
	}
	// A shared-context change alters no list projection, so only open issue
	// views refresh; account and source workbenches receive no tick.
	if event.Kind != CollaborationMemoryChangedEvent {
		accounts, err := q.ListCollaborationRealtimeAccounts(ctx, db.ListCollaborationRealtimeAccountsParams{IssueID: event.IssueID, EventSeq: event.EventSeq})
		if err != nil {
			return false, err
		}
		for _, account := range accounts {
			id := util.UUIDToString(account)
			if err = publish(realtime.ScopeAccount, id, "account:"+id, "collaboration:workbench_changed", struct{}{}); err != nil {
				return false, err
			}
		}
		issue, err := q.GetIssue(ctx, event.IssueID)
		if err != nil {
			return false, err
		}
		// Source members may not be participants or subscribe to this issue. The
		// workspace tick only invalidates their authorized lists; it carries no
		// issue identity, content, actor or execution metadata.
		if err = publish(realtime.ScopeWorkspace, util.UUIDToString(issue.WorkspaceID), "source", "collaboration:workbench_changed", struct{}{}); err != nil {
			return false, err
		}
	}
	if err = publish(realtime.ScopeIssue, util.UUIDToString(event.IssueID), "issue", "collaboration:changed", change); err != nil {
		return false, err
	}
	if err = q.MarkCollaborationRealtimeEventDelivered(ctx, event.ID); err != nil {
		return false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}

// Runtime progress uses the same ordered outbox as human changes. Keep the
// invalidation fact independent of the private run event/termination payload.
func recordCollaborationExecutionChange(ctx context.Context, q *db.Queries, issue, group, run pgtype.UUID, kind string) error {
	seq, err := q.AdvanceCollaborationEventSequence(ctx, issue)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(struct {
		GroupID pgtype.UUID `json:"group_id"`
		RunID   pgtype.UUID `json:"run_id"`
	}{group, run})
	if err != nil {
		return err
	}
	_, err = q.CreateCollaborationEvent(ctx, db.CreateCollaborationEventParams{IssueID: issue, EventSeq: seq, Kind: kind, Payload: payload})
	return err
}
