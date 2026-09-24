package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/attribution"
	"github.com/multica-ai/multica/server/internal/issueguard"
	"github.com/multica-ai/multica/server/internal/issuestatus"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

var errLifecycleMetadataTooLarge = errors.New("lifecycle evidence exceeds the issue metadata budget")
var errLifecycleSnapshotChanged = errors.New("lifecycle source changed; retry the handoff")

type lifecycleCommitError struct{ error }

type lifecyclePreparation struct {
	target     db.Issue
	explicitID string
	agentID    pgtype.UUID
	squadID    pgtype.UUID
	task       service.PreparedIssueTaskEnqueue
	policy     service.IssueCountPolicy
}

type lifecyclePersistence struct {
	source     db.Issue
	child      service.IssueCreateResult
	created    bool
	task       db.AgentTaskQueue
	taskReused bool
	comment    db.CreateCommentRow
}

func lifecycleAttribution(actorType string, actorID pgtype.UUID, task db.AgentTaskQueue, issueID pgtype.UUID) attribution.Result {
	facts := attribution.DirectFacts{IssueID: issueID}
	if actorType == "member" {
		facts.ActorUserID = actorID
	} else {
		facts.OriginType = "agent_create"
		facts.OriginTaskID = task.ID
		facts.OriginOriginator = task.OriginatorUserID
		facts.OriginAccountable = task.AccountableUserID
	}
	return attribution.ClassifyDirect(facts)
}

func (h *Handler) lifecycleOrigin(r *http.Request, actorType, actorID string, workspace pgtype.UUID) (db.AgentTaskQueue, error) {
	if actorType != "agent" {
		return db.AgentTaskQueue{}, nil
	}
	task, ok := h.trustedIssueCreationTask(r, actorType, actorID, workspace)
	if !ok {
		return db.AgentTaskQueue{}, lifecycleAssigneeError{http.StatusForbidden, "lifecycle handoff requires a live task belonging to the acting agent"}
	}
	return task, nil
}

func (h *Handler) prepareLifecycleHandoff(r *http.Request, source db.Issue, req lifecycleHandoffRequest, actorType, actorID string, needsFollowUp bool) (lifecyclePreparation, error) {
	var prepared lifecyclePreparation
	origin, err := h.lifecycleOrigin(r, actorType, actorID, source.WorkspaceID)
	if err != nil {
		return prepared, err
	}
	if !needsFollowUp {
		return prepared, nil
	}
	if h.IssueService == nil {
		return prepared, errors.New("issue service unavailable")
	}
	prepared.explicitID = strings.TrimSpace(req.FollowUpIssueID)
	if req.Kind == "incident-learning" && prepared.explicitID == "" {
		if latest, ok := util.JSONObjectOrEmpty(source.Metadata)["lifecycle_handoff"].(map[string]any); ok {
			prepared.explicitID, _ = latest["prevention_issue_id"].(string)
		}
	}
	if prepared.explicitID != "" {
		prepared.target, err = h.resolveLifecycleIssue(r, prepared.explicitID, source.WorkspaceID, source.ID)
		if err != nil {
			return prepared, err
		}
	} else {
		if strings.TrimSpace(req.FollowUpTitle) == "" {
			return prepared, lifecycleAssigneeError{http.StatusBadRequest, "follow_up_title is required when follow_up_issue_id is absent"}
		}
		assigneeType, assigneeID, parseErr := lifecycleAssignee(req)
		if parseErr != nil {
			return prepared, lifecycleAssigneeError{http.StatusBadRequest, parseErr.Error()}
		}
		prepared.target = db.Issue{WorkspaceID: source.WorkspaceID, ParentIssueID: source.ID, ProjectID: source.ProjectID,
			Title: req.FollowUpTitle, Status: "todo", Priority: req.FollowUpPriority, AssigneeType: assigneeType, AssigneeID: assigneeID}
		terminal, statusErr := issuestatus.ExpandCategories(r.Context(), h.Queries, source.WorkspaceID, []string{issuestatus.Done, issuestatus.Cancelled})
		if statusErr != nil {
			return prepared, statusErr
		}
		duplicate, duplicateErr := h.Queries.FindActiveDuplicateIssue(r.Context(), db.FindActiveDuplicateIssueParams{
			WorkspaceID: source.WorkspaceID, ProjectID: source.ProjectID, ParentIssueID: source.ID,
			NormalizedTitle: issueguard.NormalizeTitle(req.FollowUpTitle), TerminalStatusKeys: terminal,
		})
		if duplicateErr == nil {
			prepared.target = duplicate
		} else if !errors.Is(duplicateErr, pgx.ErrNoRows) {
			return prepared, duplicateErr
		}
	}
	if err = h.authorizeLifecycleFollowUp(r, prepared.target); err != nil {
		return prepared, err
	}
	prepared.agentID, prepared.squadID, err = h.lifecycleDispatchTarget(r.Context(), prepared.target, false)
	if err != nil {
		return prepared, err
	}
	if prepared.agentID.Valid {
		if h.TaskService == nil {
			return prepared, errors.New("task service unavailable")
		}
		prepared.task, err = h.TaskService.PrepareIssueTaskEnqueue(r.Context(), prepared.agentID, lifecycleAttribution(actorType, parseUUID(actorID), origin, prepared.target.ID))
		if err != nil {
			return prepared, err
		}
	}
	prepared.policy = service.ResolveIssueCountPolicy(r.Context(), h.IssueService.Entitlements, source.WorkspaceID)
	return prepared, nil
}

func (h *Handler) lifecycleDispatchTarget(ctx context.Context, target db.Issue, lock bool) (pgtype.UUID, pgtype.UUID, error) {
	if !target.AssigneeType.Valid || !target.AssigneeID.Valid {
		return pgtype.UUID{}, pgtype.UUID{}, nil
	}
	switch target.AssigneeType.String {
	case "agent":
		return target.AssigneeID, pgtype.UUID{}, nil
	case "squad":
		var squad db.Squad
		var err error
		if lock {
			squad, err = h.Queries.LockLifecycleSquad(ctx, db.LockLifecycleSquadParams{ID: target.AssigneeID, WorkspaceID: target.WorkspaceID})
		} else {
			squad, err = h.Queries.GetSquadInWorkspace(ctx, db.GetSquadInWorkspaceParams{ID: target.AssigneeID, WorkspaceID: target.WorkspaceID})
		}
		if err != nil {
			return pgtype.UUID{}, pgtype.UUID{}, err
		}
		if squad.ArchivedAt.Valid {
			return pgtype.UUID{}, pgtype.UUID{}, lifecycleAssigneeError{http.StatusBadRequest, "cannot assign to an archived squad"}
		}
		return squad.LeaderID, squad.ID, nil
	default:
		return pgtype.UUID{}, pgtype.UUID{}, nil
	}
}

func (h *Handler) persistLifecycleHandoff(r *http.Request, source db.Issue, req lifecycleHandoffRequest, actorType, actorID string, decision service.LifecycleDecision, evidence map[string]any, needsFollowUp bool) (lifecycleHandoffResponse, error) {
	if h.TxStarter == nil {
		return lifecycleHandoffResponse{}, errors.New("lifecycle handoff requires a transaction starter")
	}
	var persisted lifecyclePersistence
	for attempt := 0; attempt < 4; attempt++ {
		current, err := h.Queries.GetIssueInWorkspace(r.Context(), db.GetIssueInWorkspaceParams{ID: source.ID, WorkspaceID: source.WorkspaceID})
		if err != nil {
			return lifecycleHandoffResponse{}, err
		}
		prepared, err := h.prepareLifecycleHandoff(r, current, req, actorType, actorID, needsFollowUp)
		if err != nil {
			return lifecycleHandoffResponse{}, err
		}
		persisted, err = h.writeLifecycleHandoffInTx(r, current, req, actorType, actorID, decision, evidence, needsFollowUp, prepared)
		if err == nil {
			break
		}
		var commitErr lifecycleCommitError
		var pgErr *pgconn.PgError
		retry := errors.Is(err, errLifecycleSnapshotChanged) || (errors.As(err, &pgErr) && (pgErr.Code == "55P03" || pgErr.Code == "40P01"))
		if errors.As(err, &commitErr) || !retry {
			return lifecycleHandoffResponse{}, err
		}
		if attempt == 3 {
			return lifecycleHandoffResponse{}, lifecycleAssigneeError{http.StatusConflict, "lifecycle data is busy; retry the handoff"}
		}
		timer := time.NewTimer(time.Duration(attempt+1) * 10 * time.Millisecond)
		select {
		case <-r.Context().Done():
			timer.Stop()
			return lifecycleHandoffResponse{}, r.Context().Err()
		case <-timer.C:
		}
	}
	if persisted.created {
		h.IssueService.FinalizeCreatedIssue(persisted.child, actorType, actorID)
	}
	comment := commentToResponse(persisted.comment.Comment(), nil, nil)
	comment.IssueRevision = persisted.comment.IssueRevision
	h.publish(protocol.EventCommentCreated, uuidToString(source.WorkspaceID), actorType, actorID, map[string]any{
		"comment": comment, "issue_title": persisted.source.Title, "issue_revision": persisted.comment.IssueRevision,
	})
	for _, issue := range []db.Issue{persisted.source, persisted.child.Issue} {
		if issue.ID.Valid {
			h.publish(protocol.EventIssueMetadataChanged, uuidToString(issue.WorkspaceID), actorType, actorID, map[string]any{
				"issue_id": uuidToString(issue.ID), "metadata": parseIssueMetadata(issue.Metadata), "issue_revision": issue.Revision,
			})
		}
	}
	if persisted.task.ID.Valid && !persisted.taskReused {
		h.TaskService.FinalizeIssueTaskEnqueue(r.Context(), persisted.task)
	}
	return lifecycleHandoffResponse{
		Kind: req.Kind, Decision: string(decision), SourceIssueID: uuidToString(source.ID),
		FollowUpIssueID: uuidToString(persisted.child.Issue.ID), FollowUpCreated: persisted.created,
		FollowUpReused: persisted.child.Issue.ID.Valid && !persisted.created, QueuedTaskID: uuidToString(persisted.task.ID),
		AuditCommentID: uuidToString(persisted.comment.ID), Metadata: parseIssueMetadata(persisted.source.Metadata),
	}, nil
}

func (h *Handler) writeLifecycleHandoffInTx(r *http.Request, source db.Issue, req lifecycleHandoffRequest, actorType, actorID string, decision service.LifecycleDecision, evidence map[string]any, needsFollowUp bool, prepared lifecyclePreparation) (out lifecyclePersistence, err error) {
	ctx := r.Context()
	tx, err := h.TxStarter.Begin(ctx)
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	q := h.Queries.WithTx(tx)
	bound := &Handler{Queries: q}
	if _, err = q.LockLifecycleWorkspace(ctx, source.WorkspaceID); err != nil {
		return out, err
	}
	target := prepared.target
	if needsFollowUp && prepared.explicitID == "" {
		// Advisory lock precedes source/agent/counter locks. Calling the same
		// guard again inside CreateInTx is reentrant on this transaction.
		duplicate, found, guardErr := issueguard.LockAndFindActiveDuplicate(ctx, q, source.WorkspaceID, source.ProjectID, source.ID, req.FollowUpTitle, false)
		if guardErr != nil {
			return out, guardErr
		}
		if found {
			target = duplicate
		} else if target.ID.Valid {
			return out, errLifecycleSnapshotChanged
		}
	}
	agentID, squadID, err := bound.lifecycleDispatchTarget(ctx, target, true)
	if err != nil {
		return out, err
	}
	if agentID != prepared.agentID || squadID != prepared.squadID {
		return out, errLifecycleSnapshotChanged
	}
	if agentID.Valid {
		if _, err = q.LockLifecycleAgent(ctx, db.LockLifecycleAgentParams{ID: agentID, WorkspaceID: source.WorkspaceID}); err != nil {
			return out, err
		}
	}
	ids := []pgtype.UUID{source.ID}
	if target.ID.Valid {
		ids = append(ids, target.ID)
	}
	sort.Slice(ids, func(i, j int) bool { return uuidToString(ids[i]) < uuidToString(ids[j]) })
	for _, id := range ids {
		locked, lockErr := q.LockLifecycleIssue(ctx, db.LockLifecycleIssueParams{ID: id, WorkspaceID: source.WorkspaceID})
		if lockErr != nil {
			return out, lockErr
		}
		if id == source.ID {
			if locked.ProjectID != source.ProjectID {
				return out, errLifecycleSnapshotChanged
			}
			if req.Kind == "incident-learning" && req.FollowUpIssueID == "" {
				var saved string
				if latest, ok := util.JSONObjectOrEmpty(locked.Metadata)["lifecycle_handoff"].(map[string]any); ok {
					saved, _ = latest["prevention_issue_id"].(string)
				}
				if saved != prepared.explicitID {
					return out, errLifecycleSnapshotChanged
				}
			}
			source = locked
		} else {
			if !sameLifecycleParent(locked, source.ID) {
				return out, lifecycleAssigneeError{http.StatusBadRequest, "follow-up issue is not a child of the source issue"}
			}
			if locked.AssigneeID != target.AssigneeID || locked.AssigneeType != target.AssigneeType {
				return out, errLifecycleSnapshotChanged
			}
			target = locked
		}
	}
	if needsFollowUp && prepared.explicitID == "" && target.ID.Valid {
		// Ordinary edits do not take the duplicate advisory lock. Re-evaluate
		// the full active/title/project predicate after locking the selected row.
		duplicate, found, guardErr := issueguard.LockAndFindActiveDuplicate(ctx, q, source.WorkspaceID, source.ProjectID, source.ID, req.FollowUpTitle, false)
		if guardErr != nil {
			return out, guardErr
		}
		if !found || duplicate.ID != target.ID {
			return out, errLifecycleSnapshotChanged
		}
	}
	origin, err := bound.lifecycleOrigin(r, actorType, actorID, source.WorkspaceID)
	if err != nil {
		return out, err
	}
	if origin.ID.Valid {
		origin, err = q.LockLifecycleOriginTask(ctx, db.LockLifecycleOriginTaskParams{ID: origin.ID, WorkspaceID: source.WorkspaceID})
		if err != nil {
			return out, err
		}
		if uuidToString(origin.AgentID) != actorID || isTerminalTaskStatus(origin.Status) {
			return out, lifecycleAssigneeError{http.StatusForbidden, "lifecycle origin task is no longer active"}
		}
	}
	if err = bound.validateExistingLifecyclePreventionTasks(r, source, req.ExistingPreventionTasks); err != nil {
		return out, err
	}
	if needsFollowUp {
		if err = bound.authorizeLifecycleFollowUp(r, target); err != nil {
			return out, err
		}
		if !target.ID.Valid {
			priority := req.FollowUpPriority
			if priority == "" {
				priority = "none"
			}
			params := service.IssueCreateParams{WorkspaceID: source.WorkspaceID, ParentIssueID: source.ID, ProjectID: source.ProjectID,
				Title: req.FollowUpTitle, Description: ptrToText(&req.FollowUpDescription), Status: "todo", Priority: priority,
				AssigneeType: target.AssigneeType, AssigneeID: target.AssigneeID, CreatorType: actorType, CreatorID: parseUUID(actorID)}
			if origin.ID.Valid {
				params.OriginType = pgtype.Text{String: "agent_create", Valid: true}
				params.OriginID = origin.ID
			}
			out.child, err = h.IssueService.CreateInTx(ctx, tx, params, prepared.policy)
			if err != nil {
				return out, err
			}
			target = out.child.Issue
			out.created = true
		}
		childMetadata := lifecycleChildMetadata(target.Metadata, source.ID, req)
		if len(childMetadata) > 0 {
			target, err = persistLifecycleMetadata(ctx, q, target, childMetadata)
			if err != nil {
				return out, err
			}
		}
		out.child.Issue = target
	}
	entry := map[string]any{"kind": req.Kind, "decision": string(decision), "source_issue_id": uuidToString(source.ID),
		"evidence": evidence, "recorded_at": time.Now().UTC().Format(time.RFC3339Nano)}
	if target.ID.Valid {
		entry["follow_up_issue_id"] = uuidToString(target.ID)
		if req.Kind == "incident-learning" {
			entry["prevention_issue_id"] = uuidToString(target.ID)
		}
	}
	metadata := lifecycleMetadataObject(source.Metadata)
	metadata["lifecycle_handoff"] = entry
	history := append(lifecycleHandoffHistory(source.Metadata), entry)
	if len(history) > 20 {
		history = history[len(history)-20:]
	}
	for {
		metadata["lifecycle_handoff_history"] = history
		encoded, encodeErr := json.Marshal(metadata)
		if encodeErr != nil {
			return out, encodeErr
		}
		size, sizeErr := q.LifecycleMetadataSize(ctx, encoded)
		if sizeErr != nil {
			return out, sizeErr
		}
		if size <= 8192 {
			break
		}
		if len(history) <= 1 {
			return out, errLifecycleMetadataTooLarge
		}
		history = history[1:]
	}
	out.source, err = persistLifecycleMetadata(ctx, q, source, metadata)
	if err != nil {
		return out, err
	}
	evidenceBytes, err := json.Marshal(evidence)
	if err != nil {
		return out, err
	}
	out.comment, err = q.CreateComment(ctx, db.CreateCommentParams{ID: dbid.NewV7(), IssueID: source.ID, WorkspaceID: source.WorkspaceID,
		AuthorType: actorType, AuthorID: parseUUID(actorID), Type: "progress_update", SourceTaskID: origin.ID,
		Content: fmt.Sprintf("lifecycle-handoff kind=%s decision=%s follow_up=%s evidence=%s", req.Kind, decision, uuidToString(target.ID), evidenceBytes),
	})
	if err != nil {
		return out, err
	}
	if agentID.Valid {
		out.task, out.taskReused, err = h.TaskService.EnqueuePreparedIssueTaskInTx(ctx, tx, target, agentID, squadID, req.HandoffNote,
			lifecycleAttribution(actorType, parseUUID(actorID), origin, target.ID), prepared.task)
		if err != nil {
			return out, err
		}
	}
	out.source.Revision = out.comment.IssueRevision
	if err = tx.Commit(ctx); err != nil {
		slog.ErrorContext(ctx, "lifecycle commit failed; reconcile before retrying", "error", err,
			"source_issue_id", uuidToString(source.ID), "follow_up_issue_id", uuidToString(target.ID), "task_id", uuidToString(out.task.ID))
		return out, lifecycleCommitError{err}
	}
	return out, nil
}

func lifecycleChildMetadata(raw []byte, sourceID pgtype.UUID, req lifecycleHandoffRequest) map[string]any {
	metadata := lifecycleMetadataObject(raw)
	if req.Kind == "incident-learning" && req.Prevention != nil {
		metadata["lifecycle_owner"] = req.Prevention.Owner
		metadata["lifecycle_priority"] = req.Prevention.Priority
		metadata["lifecycle_acceptance_signal"] = req.Prevention.AcceptanceSignal
		metadata["lifecycle_source_issue"] = uuidToString(sourceID)
	}
	if req.Kind == "rca" {
		if strings.TrimSpace(req.DiagnosisRef) != "" {
			metadata["lifecycle_diagnosis_ref"] = strings.TrimSpace(req.DiagnosisRef)
			metadata["lifecycle_regression_test"] = strings.TrimSpace(req.RegressionTest)
		}
		if strings.TrimSpace(req.Conclusion) != "" {
			metadata["lifecycle_rca_conclusion"] = strings.TrimSpace(req.Conclusion)
		}
		if len(req.Evidence) > 0 {
			metadata["lifecycle_rca_evidence"] = req.Evidence
		}
		if len(req.Unknowns) > 0 {
			metadata["lifecycle_rca_unknowns"] = req.Unknowns
		}
	}
	return metadata
}

func persistLifecycleMetadata(ctx context.Context, q *db.Queries, issue db.Issue, metadata map[string]any) (db.Issue, error) {
	encoded, err := json.Marshal(metadata)
	if err != nil {
		return db.Issue{}, err
	}
	size, err := q.LifecycleMetadataSize(ctx, encoded)
	if err != nil {
		return db.Issue{}, err
	}
	if size > 8192 {
		return db.Issue{}, errLifecycleMetadataTooLarge
	}
	updated, err := q.SetLifecycleMetadata(ctx, db.SetLifecycleMetadataParams{ID: issue.ID, WorkspaceID: issue.WorkspaceID, Metadata: encoded})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23514" && pgErr.ConstraintName == "issue_metadata_size_limit" {
		return db.Issue{}, errLifecycleMetadataTooLarge
	}
	return updated, err
}

// Retain unrelated JSON numbers and objects verbatim when replacing the whole
// metadata value. Decoding those keys through float64 can corrupt large integers.
func lifecycleMetadataObject(raw []byte) map[string]any {
	var values map[string]json.RawMessage
	_ = json.Unmarshal(raw, &values)
	result := make(map[string]any, len(values))
	for key, value := range values {
		result[key] = value
	}
	return result
}
