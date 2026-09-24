package service

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/multica-ai/multica/server/internal/runtimeapps"
	"log/slog"
	"reflect"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/attribution"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

var ErrIssueTaskContextChanged = errors.New("assignment or dispatch context changed; retry the handoff")
var ErrIssueTaskUnavailable = errors.New("assigned agent runtime is unavailable")

// PreparedIssueTaskEnqueue holds external work performed before taking database
// locks. Its agent and attribution snapshots are checked again before insertion.
type PreparedIssueTaskEnqueue struct {
	agent   db.Agent
	attr    attribution.Result
	overlay runtimeMCPOverlayData
}

func (s *TaskService) PrepareIssueTaskEnqueue(ctx context.Context, agentID pgtype.UUID, attr attribution.Result) (PreparedIssueTaskEnqueue, error) {
	agent, err := s.Queries.GetAgent(ctx, agentID)
	if err != nil {
		return PreparedIssueTaskEnqueue{}, err
	}
	if agent.ArchivedAt.Valid || !agent.RuntimeID.Valid {
		return PreparedIssueTaskEnqueue{}, ErrIssueTaskUnavailable
	}
	attr, err = s.applyAttributionFallback(ctx, attr, agent)
	if err != nil {
		return PreparedIssueTaskEnqueue{}, err
	}
	return PreparedIssueTaskEnqueue{agent: agent, attr: attr, overlay: s.buildRuntimeMCPOverlay(ctx, attr.UserID, agent)}, nil
}

// EnqueuePreparedIssueTaskInTx performs database work only. The caller has
// authorized the persisted assignee in this transaction, and owns commit and
// FinalizeIssueTaskEnqueue. Reused rows must never be finalized a second time.
func (s *TaskService) EnqueuePreparedIssueTaskInTx(ctx context.Context, tx pgx.Tx, issue db.Issue, agentID, squadID pgtype.UUID, note string, attr attribution.Result, prepared PreparedIssueTaskEnqueue) (task db.AgentTaskQueue, reused bool, err error) {
	q := s.Queries.WithTx(tx)
	agent, err := q.LockLifecycleAgent(ctx, db.LockLifecycleAgentParams{ID: agentID, WorkspaceID: issue.WorkspaceID})
	if err != nil {
		return task, false, err
	}
	// Config changes invalidate the external overlay snapshot. Status/updated_at
	// may legitimately change during ordinary execution and do not select apps.
	if agent.ID != prepared.agent.ID || agent.RuntimeID != prepared.agent.RuntimeID || agent.OwnerID != prepared.agent.OwnerID ||
		agent.ArchivedAt.Valid || !reflect.DeepEqual(agent.ComposioToolkitAllowlist, prepared.agent.ComposioToolkitAllowlist) {
		return task, false, ErrIssueTaskContextChanged
	}
	attr, err = applyAttributionFallbackWithQueries(ctx, q, attr, agent)
	if err != nil {
		return task, false, err
	}
	expected := prepared.attr
	expected.EvidenceRefID = attr.EvidenceRefID
	if expected != attr {
		return task, false, ErrIssueTaskContextChanged
	}
	runtime, err := q.LockLifecycleRuntime(ctx, db.LockLifecycleRuntimeParams{ID: agent.RuntimeID, WorkspaceID: issue.WorkspaceID})
	if err != nil {
		return task, false, err
	}
	if runtimeVerdict(runtime).Blocked() {
		return task, false, ErrIssueTaskUnavailable
	}
	source, delegated, evidenceKind, evidenceRef := attributionCreateParams(attr)
	// Resolve the current head on the transaction connection, not a stale
	// preflight snapshot or another connection while owner locks are held.
	queryService := &TaskService{Queries: q}
	params := db.CreateAgentTaskParams{
		ID: dbid.NewV7(), IssueID: issue.ID, AgentID: agent.ID, RuntimeID: agent.RuntimeID,
		Priority: priorityToInt(issue.Priority), HandoffNote: pgtype.Text{String: note, Valid: note != ""},
		SquadID: squadID, IsLeaderTask: pgtype.Bool{Bool: squadID.Valid, Valid: squadID.Valid},
		OriginatorUserID: attr.UserID, AccountableUserID: attr.AccountableUserID,
		OriginatorSource: source, DelegatedFromTaskID: delegated, RuleVersionID: attr.RuleVersionID,
		TriggerEvidenceKind: evidenceKind, TriggerEvidenceRefID: evidenceRef,
		RuntimeMcpOverlay: prepared.overlay.Overlay, RuntimeConnectedApps: prepared.overlay.ConnectedApps,
		HeadSha: headShaText(queryService.ResolveIssueReviewSHA(ctx, issue.ID)),
	}
	for attempt := 0; attempt < 3; attempt++ {
		pending, lookupErr := q.LockPendingLifecycleTask(ctx, db.LockPendingLifecycleTaskParams{IssueID: issue.ID, AgentID: agent.ID})
		if lookupErr == nil {
			if !compatibleIssueHandoffTask(pending, params) {
				return task, false, ErrIssueTaskContextChanged
			}
			return pending, true, nil
		}
		if !errors.Is(lookupErr, pgx.ErrNoRows) {
			return task, false, lookupErr
		}
		// A unique violation aborts its transaction. A savepoint confines the
		// racing insert so reading the winner never uses an aborted outer tx.
		sp, beginErr := tx.Begin(ctx)
		if beginErr != nil {
			return task, false, beginErr
		}
		task, err = q.WithTx(sp).CreateAgentTask(ctx, params)
		if err == nil {
			if err = sp.Commit(ctx); err != nil {
				return db.AgentTaskQueue{}, false, err
			}
			return task, false, nil
		}
		if rollbackErr := sp.Rollback(ctx); rollbackErr != nil {
			return db.AgentTaskQueue{}, false, rollbackErr
		}
		if !isDuplicatePendingTaskErr(err) {
			return db.AgentTaskQueue{}, false, err
		}
	}
	return db.AgentTaskQueue{}, false, ErrIssueTaskContextChanged
}

func compatibleIssueHandoffTask(task db.AgentTaskQueue, p db.CreateAgentTaskParams) bool {
	if task.Status != "queued" && task.Status != "dispatched" {
		return false
	}
	var contextData struct {
		HeadSHA string `json:"head_sha"`
	}
	_ = json.Unmarshal(task.Context, &contextData)
	return task.RuntimeID == p.RuntimeID && task.SquadID == p.SquadID && task.IsLeaderTask == p.IsLeaderTask.Bool &&
		task.HandoffNote.String == p.HandoffNote.String && task.OriginatorUserID == p.OriginatorUserID &&
		task.AccountableUserID == p.AccountableUserID && task.OriginatorSource == p.OriginatorSource &&
		task.DelegatedFromTaskID == p.DelegatedFromTaskID && task.RuleVersionID == p.RuleVersionID &&
		task.TriggerEvidenceKind == p.TriggerEvidenceKind && task.TriggerEvidenceRefID == p.TriggerEvidenceRefID &&
		!task.TriggerCommentID.Valid && !task.ForceFreshSession &&
		sameIssueTaskCapabilities(task.RuntimeConnectedApps, p.RuntimeConnectedApps, task.RuntimeMcpOverlay, p.RuntimeMcpOverlay) &&
		(!p.HeadSha.Valid || p.HeadSha.String == "" || contextData.HeadSHA == p.HeadSha.String)
}

// Session URLs and authentication headers change on every overlay preparation.
// Compare stable capabilities instead, excluding ordering and display names.
func sameIssueTaskCapabilities(priorApps, currentApps, priorOverlay, currentOverlay []byte) bool {
	capabilities := func(raw []byte) (map[[3]string]bool, bool) {
		var apps []runtimeapps.ConnectedApp
		if len(raw) != 0 && json.Unmarshal(raw, &apps) != nil {
			return nil, false
		}
		set := make(map[[3]string]bool, len(apps))
		for _, app := range apps {
			set[[3]string{app.Provider, app.ServerName, app.ToolkitSlug}] = true
		}
		return set, true
	}
	overlayPresent := func(raw []byte) (bool, bool) {
		var overlay map[string]json.RawMessage
		if len(raw) != 0 && json.Unmarshal(raw, &overlay) != nil {
			return false, false
		}
		return len(overlay) != 0, true
	}
	prior, priorOK := capabilities(priorApps)
	current, currentOK := capabilities(currentApps)
	priorMounted, priorOverlayOK := overlayPresent(priorOverlay)
	currentMounted, currentOverlayOK := overlayPresent(currentOverlay)
	return priorOK && currentOK && priorOverlayOK && currentOverlayOK && priorMounted == currentMounted && reflect.DeepEqual(prior, current)
}

func (s *TaskService) FinalizeIssueTaskEnqueue(ctx context.Context, task db.AgentTaskQueue) {
	// The row is already durable. A notifier failure cannot turn this into a
	// failed handoff and invite a duplicate retry; normal daemon polls recover it.
	defer func() {
		if failure := recover(); failure != nil {
			slog.ErrorContext(ctx, "committed issue task notification failed", "task_id", task.ID, "failure", failure)
		}
	}()
	s.broadcastTaskEvent(ctx, protocol.EventTaskQueued, task)
	s.NotifyTaskEnqueued(ctx, task)
}
