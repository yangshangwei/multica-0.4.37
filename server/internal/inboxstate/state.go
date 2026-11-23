// Package inboxstate owns personal notification-group transitions and the
// shared write fence used by mutations, delivery and restart-safe expiry.
package inboxstate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
)

var (
	ErrConflict    = errors.New("inbox snooze changed")
	ErrInvalidTime = errors.New("inbox snooze must be in the future")
)

type TxStarter interface {
	Begin(context.Context) (pgx.Tx, error)
}

type Group struct {
	WorkspaceID   pgtype.UUID
	RecipientType string
	RecipientID   pgtype.UUID
	Kind          string
	ID            pgtype.UUID
}

type State = db.InboxSnooze

func ForItem(item db.InboxItem) Group {
	g := Group{item.WorkspaceID, item.RecipientType, item.RecipientID, "item", item.ID}
	if item.IssueID.Valid {
		g.Kind, g.ID = "issue", item.IssueID
	}
	return g
}

func ForState(state State) Group {
	return Group{state.WorkspaceID, state.RecipientType, state.RecipientID, state.GroupKind, state.GroupID}
}

func (g Group) args() []any {
	return []any{g.WorkspaceID, g.RecipientType, g.RecipientID, g.Kind, g.ID}
}

// LockScope follows workspace deletion and member revocation ordering. Holding
// this fence also makes multi-group inbox bulk operations safe from insertion.
func LockScope(ctx context.Context, tx pgx.Tx, ws, recipient pgtype.UUID, recipientType string) error {
	q := db.New(tx)
	if _, err := q.LockWorkspaceForChatSessionCreate(ctx, ws); err != nil {
		return err
	}
	if recipientType != "member" {
		return nil
	}
	if err := q.LockSubscriberWrites(ctx, db.LockSubscriberWritesParams{WorkspaceID: ws, UserID: recipient}); err != nil {
		return err
	}
	_, err := q.LockActiveMember(ctx, db.LockActiveMemberParams{WorkspaceID: ws, UserID: recipient})
	return err
}

func Lock(ctx context.Context, tx pgx.Tx, g Group) (State, error) {
	if err := LockScope(ctx, tx, g.WorkspaceID, g.RecipientID, g.RecipientType); err != nil {
		return State{}, err
	}
	key := "inbox-snooze:" + util.UUIDToString(g.WorkspaceID) + ":" + g.RecipientType + ":" + util.UUIDToString(g.RecipientID) + ":" + g.Kind + ":" + util.UUIDToString(g.ID)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, key); err != nil {
		return State{}, err
	}
	if g.Kind == "issue" {
		var issueID pgtype.UUID
		if err := tx.QueryRow(ctx, `SELECT id FROM issue WHERE workspace_id=$1 AND id=$2 FOR KEY SHARE`, g.WorkspaceID, g.ID).Scan(&issueID); err != nil {
			return State{}, err
		}
	}
	q := db.New(tx)
	if err := q.EnsureInboxSnooze(ctx, db.EnsureInboxSnoozeParams{WorkspaceID: g.WorkspaceID, RecipientType: g.RecipientType, RecipientID: g.RecipientID, GroupKind: g.Kind, GroupID: g.ID}); err != nil {
		return State{}, err
	}
	return q.LockInboxSnooze(ctx, db.LockInboxSnoozeParams{WorkspaceID: g.WorkspaceID, RecipientType: g.RecipientType, RecipientID: g.RecipientID, GroupKind: g.Kind, GroupID: g.ID})
}

func Read(ctx context.Context, executor db.DBTX, g Group) (State, error) {
	return db.New(executor).GetInboxSnooze(ctx, db.GetInboxSnoozeParams{WorkspaceID: g.WorkspaceID, RecipientType: g.RecipientType, RecipientID: g.RecipientID, GroupKind: g.Kind, GroupID: g.ID})
}

func Save(ctx context.Context, tx pgx.Tx, state State) error {
	args := ForState(state).args()
	args = append(args, state.Revision, state.SnoozedUntil, state.SnoozedAt, state.IssueRevisionWatermark, state.RemindedAt, state.ReminderReason, state.UndoToken, state.UndoSnapshot)
	_, err := tx.Exec(ctx, `UPDATE inbox_snooze SET revision=$6,snoozed_until=$7,snoozed_at=$8,issue_revision_watermark=$9,reminded_at=$10,reminder_reason=$11,undo_token=$12,undo_snapshot=$13 WHERE workspace_id=$1 AND recipient_type=$2 AND recipient_id=$3 AND group_kind=$4 AND group_id=$5`, args...)
	return err
}

func Now(ctx context.Context, tx pgx.Tx) (time.Time, error) {
	var now time.Time
	err := tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now)
	return now, err
}

func Active(ctx context.Context, tx pgx.Tx, g Group) (bool, error) {
	var active bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM inbox_item i WHERE workspace_id=$1 AND recipient_type=$2 AND recipient_id=$3 AND archived=false AND CASE WHEN issue_id IS NULL THEN 'item' ELSE 'issue' END=$4 AND COALESCE(issue_id,id)=$5)`, g.args()...).Scan(&active)
	if err != nil || !active || g.Kind != "issue" {
		return active, err
	}
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM issue WHERE workspace_id=$1 AND id=$2)`, g.WorkspaceID, g.ID).Scan(&active)
	return active, err
}

func InvalidateUndo(state *State) {
	state.Revision++
	state.UndoToken = pgtype.UUID{}
	state.UndoSnapshot = nil
}

// Remind changes only the rendered newest active row, preserving historical
// siblings' read values and all original notification/comment identities.
func Remind(ctx context.Context, tx pgx.Tx, state *State, at time.Time, reason string) error {
	g := ForState(*state)
	if _, err := tx.Exec(ctx, `UPDATE inbox_item SET read=false WHERE id=(SELECT id FROM inbox_item WHERE workspace_id=$1 AND recipient_type=$2 AND recipient_id=$3 AND archived=false AND CASE WHEN issue_id IS NULL THEN 'item' ELSE 'issue' END=$4 AND COALESCE(issue_id,id)=$5 ORDER BY created_at DESC,id DESC LIMIT 1)`, g.args()...); err != nil {
		return err
	}
	InvalidateUndo(state)
	state.SnoozedUntil = pgtype.Timestamptz{}
	state.RemindedAt = pgtype.Timestamptz{Time: at, Valid: true}
	state.ReminderReason = pgtype.Text{String: reason, Valid: true}
	return nil
}

func Expire(ctx context.Context, tx pgx.Tx, state *State, now time.Time) (bool, error) {
	if !state.SnoozedUntil.Valid || state.SnoozedUntil.Time.After(now) {
		return false, nil
	}
	active, err := Active(ctx, tx, ForState(*state))
	if err != nil {
		return false, err
	}
	if active {
		err = Remind(ctx, tx, state, state.SnoozedUntil.Time, "due")
	} else {
		Clear(state)
	}
	if err != nil {
		return false, err
	}
	return true, Save(ctx, tx, *state)
}

func Clear(state *State) {
	InvalidateUndo(state)
	state.SnoozedUntil = pgtype.Timestamptz{}
	state.RemindedAt = pgtype.Timestamptz{}
	state.ReminderReason = pgtype.Text{}
}

type snapshot struct {
	Until      pgtype.Timestamptz `json:"until"`
	At         pgtype.Timestamptz `json:"at"`
	Watermark  pgtype.Int8        `json:"watermark"`
	RemindedAt pgtype.Timestamptz `json:"reminded_at"`
	Reason     pgtype.Text        `json:"reason"`
	Reads      map[string]bool    `json:"reads,omitempty"`
}

func capture(ctx context.Context, tx pgx.Tx, state State, reads bool) ([]byte, error) {
	s := snapshot{Until: state.SnoozedUntil, At: state.SnoozedAt, Watermark: state.IssueRevisionWatermark, RemindedAt: state.RemindedAt, Reason: state.ReminderReason}
	if reads {
		var id pgtype.UUID
		var read bool
		err := tx.QueryRow(ctx, `SELECT id,read FROM inbox_item WHERE workspace_id=$1 AND recipient_type=$2 AND recipient_id=$3 AND archived=false AND CASE WHEN issue_id IS NULL THEN 'item' ELSE 'issue' END=$4 AND COALESCE(issue_id,id)=$5 ORDER BY created_at DESC,id DESC LIMIT 1`, ForState(state).args()...).Scan(&id, &read)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		if err == nil {
			s.Reads = map[string]bool{util.UUIDToString(id): read}
		}
	}
	return json.Marshal(s)
}

// baselineVisibleSources consumes only source identities visible to this
// transaction at the user's snooze observation. A source transaction which
// started earlier but has not committed is deliberately absent. Database now()
// timestamps are not commit-order fences.
func baselineVisibleSources(ctx context.Context, tx pgx.Tx, state State) error {
	if state.GroupKind != "issue" {
		return nil
	}
	_, err := tx.Exec(ctx, `WITH exhaustion AS (
  SELECT DISTINCT ON(c.source_task_id) c.id
  FROM comment c WHERE c.workspace_id=$1 AND c.issue_id=$5
   AND c.author_type='system' AND c.type='system' AND c.source_task_id IS NOT NULL
  ORDER BY c.source_task_id,c.created_at,c.id
 ),visible_sources AS (
  SELECT c.id,'mentioned'::text AS notification_type,'comment_mention'::text AS source_kind
   FROM comment c WHERE c.workspace_id=$1 AND c.issue_id=$5
  UNION ALL
  SELECT task.id,'task_failed'::text,'task_failure'::text
   FROM agent_task_queue task JOIN issue i ON i.id=task.issue_id AND i.workspace_id=$1
   WHERE task.issue_id=$5 AND task.status='failed' AND task.completed_at IS NOT NULL
  UNION ALL
  SELECT e.id,'task_failed'::text,'delegated_failure_exhaustion'::text FROM exhaustion e
 ) INSERT INTO inbox_critical_source(workspace_id,recipient_type,recipient_id,group_kind,group_id,notification_type,source_kind,source_id,source_revision)
 SELECT $1,$2,$3,$4,$5,v.notification_type,v.source_kind,v.id,0 FROM visible_sources v
 ON CONFLICT(workspace_id,recipient_type,recipient_id,group_kind,group_id,notification_type,source_kind,source_id,source_revision) DO NOTHING`, ForState(state).args()...)
	return err
}

func Snooze(ctx context.Context, tx pgx.Tx, state *State, until time.Time, revision int64) error {
	// Match pgx/PostgreSQL precision before validating or returning the state.
	until = until.UTC().Truncate(time.Microsecond)
	now, err := Now(ctx, tx)
	if err != nil {
		return err
	}
	if !until.After(now) {
		return ErrInvalidTime
	}
	if state.Revision != revision {
		return ErrConflict
	}
	active, err := Active(ctx, tx, ForState(*state))
	if err != nil {
		return err
	}
	if !active {
		return pgx.ErrNoRows
	}
	if _, err = Expire(ctx, tx, state, now); err != nil {
		return err
	}
	prior, err := capture(ctx, tx, *state, false)
	if err != nil {
		return err
	}
	if state.GroupKind == "issue" {
		if err = tx.QueryRow(ctx, `SELECT revision FROM issue WHERE workspace_id=$1 AND id=$2`, state.WorkspaceID, state.GroupID).Scan(&state.IssueRevisionWatermark); err != nil {
			return err
		}
	}
	if err = baselineVisibleSources(ctx, tx, *state); err != nil {
		return err
	}
	state.Revision++
	state.SnoozedUntil = pgtype.Timestamptz{Time: until, Valid: true}
	state.SnoozedAt = pgtype.Timestamptz{Time: now, Valid: true}
	state.RemindedAt = pgtype.Timestamptz{}
	state.ReminderReason = pgtype.Text{}
	state.UndoToken, state.UndoSnapshot = dbid.NewV7(), prior
	return Save(ctx, tx, *state)
}

func Unsnooze(ctx context.Context, tx pgx.Tx, state *State, revision int64) error {
	if state.Revision != revision {
		return ErrConflict
	}
	if !state.SnoozedUntil.Valid {
		return ErrConflict
	}
	now, err := Now(ctx, tx)
	if err != nil {
		return err
	}
	prior, err := capture(ctx, tx, *state, true)
	if err != nil {
		return err
	}
	if err = Remind(ctx, tx, state, now, "manual"); err != nil {
		return err
	}
	state.UndoToken, state.UndoSnapshot = dbid.NewV7(), prior
	return Save(ctx, tx, *state)
}

func Undo(ctx context.Context, tx pgx.Tx, state *State, token pgtype.UUID) error {
	if !token.Valid || !state.UndoToken.Valid || token != state.UndoToken {
		return ErrConflict
	}
	now, err := Now(ctx, tx)
	if err != nil {
		return err
	}
	if state.SnoozedUntil.Valid && !state.SnoozedUntil.Time.After(now) {
		return ErrConflict
	}
	var prior snapshot
	if err = json.Unmarshal(state.UndoSnapshot, &prior); err != nil {
		return err
	}
	for rawID, read := range prior.Reads {
		id, parseErr := util.ParseUUID(rawID)
		if parseErr != nil {
			return parseErr
		}
		g := ForState(*state)
		args := append(g.args(), id, read)
		if _, err = tx.Exec(ctx, `UPDATE inbox_item SET read=$7 WHERE workspace_id=$1 AND recipient_type=$2 AND recipient_id=$3 AND archived=false AND CASE WHEN issue_id IS NULL THEN 'item' ELSE 'issue' END=$4 AND COALESCE(issue_id,id)=$5 AND id=$6`, args...); err != nil {
			return err
		}
	}
	state.SnoozedUntil, state.SnoozedAt = prior.Until, prior.At
	state.IssueRevisionWatermark = prior.Watermark
	state.RemindedAt, state.ReminderReason = prior.RemindedAt, prior.Reason
	InvalidateUndo(state)
	if _, err = Expire(ctx, tx, state, now); err != nil {
		return err
	}
	return Save(ctx, tx, *state)
}

type Delivery struct {
	Item       db.InboxItem
	Created    bool
	Suppressed bool
	State      State
}

func Deliver(ctx context.Context, starter TxStarter, params db.CreateInboxItemParams) (Delivery, error) {
	tx, err := starter.Begin(ctx)
	if err != nil {
		return Delivery{}, err
	}
	defer tx.Rollback(ctx)
	out, err := DeliverTx(ctx, tx, params)
	if err != nil {
		return Delivery{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Delivery{}, err
	}
	return out, nil
}

type source struct {
	kind     string
	id       pgtype.UUID
	revision int64
	at       pgtype.Timestamptz
}

func criticalSource(ctx context.Context, tx pgx.Tx, params db.CreateInboxItemParams, details map[string]string) (source, error) {
	var src source
	parse := func(raw string) pgtype.UUID {
		id, err := util.ParseUUID(raw)
		if err != nil || id.Bytes == [16]byte{} {
			return pgtype.UUID{}
		}
		return id
	}
	switch params.Type {
	case "mentioned":
		if details["mention_scope"] != "member" {
			return src, nil
		}
		if id := parse(details["comment_id"]); id.Valid {
			src.kind, src.id = "comment_mention", id
		} else if params.IssueID.Valid && details["source_kind"] == "issue_description_mention" {
			src.kind, src.id = "issue_description_mention", params.IssueID
		}
	case "issue_assigned":
		if params.IssueID.Valid {
			src.kind, src.id = "issue_assignment", params.IssueID
		}
	case "task_failed":
		if id := parse(details["exhaustion_comment_id"]); id.Valid {
			src.kind, src.id = "delegated_failure_exhaustion", id
		} else if id := parse(details["task_id"]); id.Valid {
			src.kind, src.id = "task_failure", id
		} else if id := parse(details["failed_task_id"]); id.Valid {
			src.kind, src.id = "task_failure", id
		}
	}
	if !src.id.Valid {
		return source{}, nil
	}
	if src.kind == "issue_assignment" || src.kind == "issue_description_mention" {
		rev, err := strconv.ParseInt(details["source_revision"], 10, 64)
		if err != nil || rev <= 0 {
			return source{}, nil
		}
		src.revision = rev
		return src, nil
	}
	var err error
	if src.kind == "task_failure" {
		err = tx.QueryRow(ctx, `SELECT completed_at FROM agent_task_queue WHERE id=$1 AND status='failed'`, src.id).Scan(&src.at)
	} else {
		err = tx.QueryRow(ctx, `SELECT created_at FROM comment WHERE id=$1 AND workspace_id=$2`, src.id, params.WorkspaceID).Scan(&src.at)
	}
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !src.at.Valid) {
		return source{}, nil
	}
	return src, err
}

// DeliverTx must be called before insertion in a borrowed transaction. Callers
// publish only after their transaction commits; suppressed rows invalidate
// caches without producing ordinary system banners.
func DeliverTx(ctx context.Context, tx pgx.Tx, params db.CreateInboxItemParams) (Delivery, error) {
	if params.Type == "project_update" || params.Type == "iteration" {
		if params.RecipientType != "member" {
			return Delivery{}, errors.New("scheduled inbox recipient must be a member")
		}
		params.IssueID = pgtype.UUID{}
	}
	if !params.ID.Valid {
		params.ID = dbid.NewV7()
	}
	g := Group{params.WorkspaceID, params.RecipientType, params.RecipientID, "item", params.ID}
	if params.IssueID.Valid {
		g.Kind, g.ID = "issue", params.IssueID
	}
	state, err := Lock(ctx, tx, g)
	if err != nil {
		return Delivery{}, err
	}
	now, err := Now(ctx, tx)
	if err != nil {
		return Delivery{}, err
	}
	out := Delivery{State: state, Suppressed: state.SnoozedUntil.Valid && state.SnoozedUntil.Time.After(now)}
	rawDetails := map[string]json.RawMessage{}
	if len(params.Details) > 0 {
		if err = json.Unmarshal(params.Details, &rawDetails); err != nil {
			return Delivery{}, err
		}
	}
	details := map[string]string{}
	for _, key := range []string{"mention_scope", "comment_id", "source_kind", "source_revision", "task_id", "failed_task_id", "exhaustion_comment_id"} {
		var value string
		if raw, ok := rawDetails[key]; ok && json.Unmarshal(raw, &value) == nil {
			details[key] = value
		}
	}
	src, err := criticalSource(ctx, tx, params, details)
	if err != nil {
		return Delivery{}, err
	}
	if src.id.Valid {
		args := append(g.args(), params.Type, src.kind, src.id, src.revision)
		claim, claimErr := tx.Exec(ctx, `INSERT INTO inbox_critical_source(workspace_id,recipient_type,recipient_id,group_kind,group_id,notification_type,source_kind,source_id,source_revision) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT(workspace_id,recipient_type,recipient_id,group_kind,group_id,notification_type,source_kind,source_id,source_revision) DO NOTHING`, args...)
		if claimErr != nil {
			return Delivery{}, claimErr
		}
		if claim.RowsAffected() == 0 {
			return out, nil
		}
		// Issue revisions remain an occurrence watermark; comments and tasks
		// use the committed visible-source receipts captured at snooze.
		if src.revision > 0 && state.IssueRevisionWatermark.Valid && src.revision <= state.IssueRevisionWatermark.Int64 {
			return out, nil
		}
		var legacy pgtype.UUID
		legacyKey := "source_id"
		if src.kind == "comment_mention" {
			legacyKey = "comment_id"
		}
		legacyErr := tx.QueryRow(ctx, `SELECT id FROM inbox_item WHERE workspace_id=$1 AND recipient_type=$2 AND recipient_id=$3 AND CASE WHEN issue_id IS NULL THEN 'item' ELSE 'issue' END=$4 AND COALESCE(issue_id,id)=$5 AND type=$6 AND lower(details->>$7)=$8 AND ($9::bigint=0 OR details->>'source_revision'=$9::text) ORDER BY created_at,id LIMIT 1`, append(g.args(), params.Type, legacyKey, util.UUIDToString(src.id), src.revision)...).Scan(&legacy)
		if legacyErr == nil {
			return out, nil
		}
		if !errors.Is(legacyErr, pgx.ErrNoRows) {
			return Delivery{}, legacyErr
		}
		if rawDetails == nil {
			rawDetails = map[string]json.RawMessage{}
		}
		rawDetails["source_kind"], _ = json.Marshal(src.kind)
		rawDetails["source_id"], _ = json.Marshal(util.UUIDToString(src.id))
		if src.revision > 0 {
			rawDetails["source_revision"], _ = json.Marshal(strconv.FormatInt(src.revision, 10))
		}
		params.Details, err = json.Marshal(rawDetails)
		if err != nil {
			return Delivery{}, err
		}
	}
	q := db.New(tx)
	if params.Type == "new_comment" && params.RecipientType == "member" && params.IssueID.Valid {
		commentID, parseErr := util.ParseUUID(details["comment_id"])
		if parseErr == nil && commentID.Bytes != [16]byte{} {
			out.Item, err = q.CreateCommentInboxItem(ctx, db.CreateCommentInboxItemParams{ID: params.ID, WorkspaceID: params.WorkspaceID, RecipientID: params.RecipientID, IssueID: params.IssueID, Severity: params.Severity, Title: params.Title, Body: params.Body, ActorType: params.ActorType, ActorID: params.ActorID, Details: params.Details, CommentID: commentID})
		} else {
			out.Item, err = q.CreateInboxItem(ctx, params)
		}
	} else {
		switch params.Type {
		case "project_update":
			out.Item, err = q.CreateProjectUpdateInbox(ctx, db.CreateProjectUpdateInboxParams{ID: params.ID, WorkspaceID: params.WorkspaceID, RecipientID: params.RecipientID, Title: params.Title, ActorID: params.ActorID, Details: params.Details})
		case "iteration":
			out.Item, err = q.CreateIterationInbox(ctx, db.CreateIterationInboxParams{ID: params.ID, WorkspaceID: params.WorkspaceID, RecipientID: params.RecipientID, Title: params.Title, Details: params.Details})
		case "triage":
			_, err = tx.Exec(ctx, `INSERT INTO inbox_item(id,workspace_id,recipient_type,recipient_id,type,severity,issue_id,title,body,actor_type,details) VALUES($1,$2,'member',$3,'triage','info',$4,$5,'','system',$6) ON CONFLICT(id) DO UPDATE SET title=EXCLUDED.title,details=EXCLUDED.details`, params.ID, params.WorkspaceID, params.RecipientID, params.IssueID, params.Title, params.Details)
			if err == nil {
				out.Item, err = q.GetInboxItem(ctx, params.ID)
			}
		default:
			out.Item, err = q.CreateInboxItem(ctx, params)
		}
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return out, nil
	}
	if err != nil {
		return Delivery{}, err
	}
	out.Created = true
	if src.id.Valid {
		args := append(g.args(), params.Type, src.kind, src.id, src.revision, out.Item.ID)
		if _, err = tx.Exec(ctx, `UPDATE inbox_critical_source SET inbox_item_id=$10 WHERE workspace_id=$1 AND recipient_type=$2 AND recipient_id=$3 AND group_kind=$4 AND group_id=$5 AND notification_type=$6 AND source_kind=$7 AND source_id=$8 AND source_revision=$9`, args...); err != nil {
			return Delivery{}, err
		}
		if state.SnoozedUntil.Valid {
			if err = Remind(ctx, tx, &state, now, params.Type); err != nil {
				return Delivery{}, err
			}
			if err = Save(ctx, tx, state); err != nil {
				return Delivery{}, err
			}
			out.Suppressed = false
		} else if state.UndoToken.Valid {
			// A new important source must prevent an outstanding manual-reminder
			// undo from putting that new notification back into an old snooze.
			InvalidateUndo(&state)
			if err = Save(ctx, tx, state); err != nil {
				return Delivery{}, err
			}
		}
	}
	out.State = state
	return out, nil
}

func Payload(state State) map[string]any {
	return map[string]any{"workspace_id": util.UUIDToString(state.WorkspaceID), "recipient_type": state.RecipientType, "recipient_id": util.UUIDToString(state.RecipientID), "group_kind": state.GroupKind, "group_id": util.UUIDToString(state.GroupID), "revision": state.Revision}
}

// CleanupUnavailable handles orphaned state left by deleted targets or users.
// Normal application cleanup also removes these rows inside its transaction.
func CleanupUnavailable(ctx context.Context, executor db.DBTX) error {
	for _, table := range []string{"inbox_critical_source", "inbox_snooze"} {
		_, err := executor.Exec(ctx, fmt.Sprintf(`DELETE FROM %s s WHERE NOT EXISTS(SELECT 1 FROM workspace w WHERE w.id=s.workspace_id) OR (s.recipient_type='member' AND NOT EXISTS(SELECT 1 FROM member m JOIN "user" u ON u.id=m.user_id WHERE m.workspace_id=s.workspace_id AND m.user_id=s.recipient_id)) OR (s.group_kind='issue' AND NOT EXISTS(SELECT 1 FROM issue i WHERE i.id=s.group_id AND i.workspace_id=s.workspace_id)) OR (s.group_kind='item' AND NOT EXISTS(SELECT 1 FROM inbox_item i WHERE i.id=s.group_id AND i.workspace_id=s.workspace_id AND i.recipient_id=s.recipient_id AND i.recipient_type=s.recipient_type))`, table))
		if err != nil {
			return err
		}
	}
	return nil
}
