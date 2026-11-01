package service

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// No argument is reserved for trusted server-owned transitions. Every HTTP
// mutation passes exactly one element, including nil for an older client.
// A supplied fence is always checked, even after negotiation was downgraded.
func validateTaskKnowledgeFences(task db.AgentTaskQueue, fences []*protocol.TaskKnowledgeFence) error {
	if len(fences) == 0 {
		return nil
	}
	if len(fences) != 1 {
		return knowledgeFailure(400, "invalid_knowledge_request")
	}
	if fence := fences[0]; fence != nil {
		if !task.DispatchedAt.Valid || fence.ClaimGeneration != strconv.FormatInt(task.ClaimGeneration, 10) || fence.DispatchedAt != task.DispatchedAt.Time.UTC().Format(time.RFC3339Nano) {
			return knowledgeFailure(409, "knowledge_context_changed")
		}
		return nil
	}
	if task.KnowledgeContextVersion.Valid || len(task.KnowledgeDelivery) > 0 {
		return knowledgeFailure(409, "knowledge_context_changed")
	}
	return nil
}

func ValidateTaskKnowledgeReportFence(ctx context.Context, q *db.Queries, id pgtype.UUID, fence *protocol.TaskKnowledgeFence) error {
	task, err := q.LockTaskForClaimFinalization(ctx, id)
	if err != nil {
		return err
	}
	return validateTaskKnowledgeFences(task, []*protocol.TaskKnowledgeFence{fence})
}

// ValidateTaskKnowledgeSessionPointer is called under the writer's existing
// Chat -> task transaction. It records no new authority and acquires no source
// or workspace locks. Only a directly observed admitted session may be pinned.
func ValidateTaskKnowledgeSessionPointer(ctx context.Context, q *db.Queries, taskID pgtype.UUID, native string) error {
	if native == "" {
		return nil
	}
	task, err := q.LockTaskForClaimFinalization(ctx, taskID)
	if err != nil {
		return err
	}
	if taskKnowledgePointerAllowed(task, native) {
		return nil
	}
	return knowledgeFailure(409, "knowledge_context_changed")
}

func taskKnowledgePointerAllowed(task db.AgentTaskQueue, native string) bool {
	d, err := decodeTaskKnowledge(task.KnowledgeContextVersion, task.KnowledgeDelivery)
	if err != nil {
		return false
	}
	if d == nil {
		return true
	}
	if native == "" {
		return true
	}
	c, err := taskKnowledgeClaimAt(d, task.ClaimGeneration)
	if err != nil {
		return false
	}
	for _, attempt := range c.Attempts {
		if attempt.BoundSessionID == native {
			return true
		}
	}
	return false
}

// ObserveTaskKnowledgeLaunch is a binding-only writer. The authorized attempt
// already contains the immutable manifest. Native pointer and exposure become
// visible in the same commit; current source authorization is not re-entered.
func (s *TaskService) ObserveTaskKnowledgeLaunch(ctx context.Context, caller TaskKnowledgeCaller, taskID pgtype.UUID, req protocol.TaskKnowledgeObservation) error {
	if protocol.ValidateTaskKnowledgeRequest(req.ProtocolVersion, req.RequestID, req.ClaimGeneration, req.DispatchedAt) != nil || req.AttemptSequence < 1 || req.AttemptSequence > 32 {
		return knowledgeFailure(400, "invalid_knowledge_request")
	}
	switch req.Kind {
	case "started":
		if req.ProcessID <= 0 || !protocol.ValidTaskKnowledgeIdentity(req.ProcessInstanceID) {
			return knowledgeFailure(400, "invalid_knowledge_request")
		}
	case "native_session":
		if !protocol.ValidTaskKnowledgeIdentity(req.NativeSessionID) || (req.Source != "system" && req.Source != "result") {
			return knowledgeFailure(400, "invalid_knowledge_request")
		}
	case "terminated":
		if req.TerminationKind != "managed_launch_ended" && req.TerminationKind != "not_started" && req.TerminationKind != "unknown" {
			return knowledgeFailure(400, "invalid_knowledge_request")
		}
	default:
		return knowledgeFailure(400, "invalid_knowledge_request")
	}
	if s.TxStarter == nil {
		return knowledgeFailure(503, "knowledge_unavailable")
	}
	task, err := s.Queries.GetAgentTask(ctx, taskID)
	if err != nil {
		return taskKnowledgeError(err)
	}
	runtime, err := s.runtimeLookup(s.Queries).Get(ctx, task.RuntimeID)
	if err != nil {
		return taskKnowledgeError(err)
	}
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return taskKnowledgeError(err)
	}
	defer tx.Rollback(ctx)
	q := s.Queries.WithTx(tx)
	if err = lockTaskKnowledgeCredential(ctx, q, caller, task, runtime); err != nil {
		return taskKnowledgeError(err)
	}
	if err = lockChatSessionForTaskWrite(ctx, q, taskID); err != nil {
		return taskKnowledgeError(err)
	}
	task, err = q.LockTaskForClaimFinalization(ctx, taskID)
	if err != nil {
		return taskKnowledgeError(err)
	}
	if task.RuntimeID != caller.RuntimeID {
		return knowledgeFailure(403, "knowledge_authorization_denied")
	}
	if err = checkTaskKnowledgeIdentity(task, req.ClaimGeneration, req.DispatchedAt); err != nil {
		return err
	}
	d, err := decodeTaskKnowledge(task.KnowledgeContextVersion, task.KnowledgeDelivery)
	if err != nil {
		return err
	}
	c, err := taskKnowledgeClaimAt(d, task.ClaimGeneration)
	if err != nil {
		return err
	}
	if c.Prepared == nil || c.Prepared.ID != req.PrepareID || req.AttemptSequence > int64(len(c.Attempts)) {
		return knowledgeFailure(409, "knowledge_context_changed")
	}
	a := &c.Attempts[req.AttemptSequence-1]
	if a.Authorization.AttemptNonce != req.AttemptNonce {
		return knowledgeFailure(409, "knowledge_context_changed")
	}
	for _, prior := range a.Observations {
		if prior.RequestID == req.RequestID {
			if !reflect.DeepEqual(prior, req) {
				return knowledgeFailure(409, "idempotency_conflict")
			}
			if slices.Contains(a.RefusedObservationIDs, req.RequestID) {
				return knowledgeFailure(409, "knowledge_context_changed")
			}
			return nil
		}
	}
	if len(a.Observations) >= 8 {
		return knowledgeFailure(409, "knowledge_context_changed")
	}
	refused := false
	switch req.Kind {
	case "started":
		if a.Started || a.Terminated {
			return knowledgeFailure(409, "knowledge_context_changed")
		}
		a.Started = true
	case "native_session":
		if !a.Started || a.Terminated {
			return knowledgeFailure(409, "knowledge_context_changed")
		}
		if a.Refused || (a.Authorization.Session.NativeSessionID != "" && a.Authorization.Session.NativeSessionID != req.NativeSessionID) || (a.BoundSessionID != "" && a.BoundSessionID != req.NativeSessionID) {
			a.Refused = true
			refused = true
		} else {
			a.BoundSessionID = req.NativeSessionID
		}
	case "terminated":
		if a.Terminated {
			return knowledgeFailure(409, "knowledge_context_changed")
		}
		if req.TerminationKind == "not_started" && a.Started || req.TerminationKind == "managed_launch_ended" && (!a.Started || !req.WaitObserved) {
			return knowledgeFailure(409, "knowledge_context_changed")
		}
		a.Terminated = req.TerminationKind != "unknown"
		if !a.Terminated {
			a.Refused = true
		}
	}
	a.Observations = append(a.Observations, req)
	if refused {
		a.RefusedObservationIDs = append(a.RefusedObservationIDs, req.RequestID)
	}
	raw, err := encodeTaskKnowledge(d)
	if err != nil {
		return err
	}
	if _, err = q.SetTaskKnowledgeDelivery(ctx, db.SetTaskKnowledgeDeliveryParams{TaskID: task.ID, ClaimGeneration: task.ClaimGeneration, Delivery: raw}); err != nil {
		return taskKnowledgeError(err)
	}
	if req.Kind == "native_session" && !refused {
		if task.Status != "dispatched" && task.Status != "running" && task.Status != "cancelled" {
			return knowledgeFailure(409, "knowledge_context_changed")
		}
		if err = q.UpdateAgentTaskSession(ctx, db.UpdateAgentTaskSessionParams{ID: task.ID, SessionID: pgtype.Text{String: req.NativeSessionID, Valid: true}}); err != nil {
			return taskKnowledgeError(err)
		}
		if err = q.AdvanceCancelledChatSessionPointer(ctx, task.ID); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return taskKnowledgeError(err)
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return taskKnowledgeError(err)
	}
	if refused {
		return knowledgeFailure(409, "knowledge_context_changed")
	}
	return nil
}
