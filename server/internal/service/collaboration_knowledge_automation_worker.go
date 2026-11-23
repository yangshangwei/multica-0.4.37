package service

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/llm"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// One instance belongs to one API process. The semaphore bounds active batches;
// durable PostgreSQL leases, not these in-memory slots, own recoverable work.
type KnowledgeAutomationWorker struct {
	Service *CollaborationService
	Model   *llm.Client
	slots   chan struct{}
}

func NewKnowledgeAutomationWorker(service *CollaborationService, model *llm.Client) *KnowledgeAutomationWorker {
	return &KnowledgeAutomationWorker{Service: service, Model: model, slots: make(chan struct{}, 2)}
}

func (w *KnowledgeAutomationWorker) Run(ctx context.Context) {
	var workers sync.WaitGroup
	for range 2 {
		workers.Go(func() {
			for ctx.Err() == nil {
				worked, err := w.ProcessNext(ctx)
				if err != nil && ctx.Err() == nil {
					slog.Warn("knowledge automation pass failed", "code", "knowledge_unavailable")
				}
				if !worked || err != nil {
					timer := time.NewTimer(500 * time.Millisecond)
					select {
					case <-ctx.Done():
						timer.Stop()
						return
					case <-timer.C:
					}
				}
			}
		})
	}
	workers.Wait()
}

// ProcessNext processes at most one fixed batch under one180-second lease.
// Tests drive this same durable path with an explicit fake HTTP model.
func (w *KnowledgeAutomationWorker) ProcessNext(ctx context.Context) (bool, error) {
	select {
	case w.slots <- struct{}{}:
		defer func() { <-w.slots }()
	default:
		return false, nil
	}
	job, err := collaborationRuntimeMutation(ctx, w.Service, func(tx pgx.Tx) (db.CollaborationKnowledgeJob, error) {
		return w.Service.Queries.WithTx(tx).ClaimKnowledgeAutomationJob(ctx, pgtype.UUID{Bytes: uuid.New(), Valid: true})
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	batchCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	err = w.processJob(batchCtx, job)
	if err == nil {
		return true, nil
	}
	phase, reason := "review_pending", "knowledge_unavailable"
	var stop *knowledgeAutomationStop
	var model *knowledgeAutomationModelError
	if errors.As(err, &stop) {
		phase, reason = stop.Phase, stop.Reason
	} else if errors.As(err, &model) {
		reason = model.Reason
	}
	retries := job.DbRetryCount
	if phase == "review_pending" && reason == "knowledge_unavailable" && retries < 3 {
		phase = "queued"
		retries++
	}
	recordCtx, done := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer done()
	_, releaseErr := w.release(recordCtx, job, phase, reason, nil, retries)
	if releaseErr != nil {
		return true, knowledgeFailure(503, "knowledge_unavailable")
	}
	return true, nil
}

func (w *KnowledgeAutomationWorker) processJob(ctx context.Context, job db.CollaborationKnowledgeJob) error {
	if len(job.BatchManifest) == 0 {
		_, err := collaborationRuntimeMutation(ctx, w.Service, func(tx pgx.Tx) (bool, error) {
			q := w.Service.Queries.WithTx(tx)
			if _, err := w.snapshot(ctx, q, job, nil); err != nil {
				return false, err
			}
			if !w.Model.Enabled() {
				return false, &knowledgeAutomationStop{"model_not_configured", "waiting_configuration"}
			}
			rows, err := q.ListKnowledgeAutomationRevalidationCandidates(ctx, db.ListKnowledgeAutomationRevalidationCandidatesParams{SourceIssueID: job.SourceIssueID, TargetProjectID: job.ProjectID, TargetWorkspaceID: job.SourceWorkspaceID})
			if err != nil {
				return false, err
			}
			if len(rows) > 16 {
				return false, automationPending("revalidation_budget_exceeded")
			}
			var ids []pgtype.UUID
			for _, row := range rows {
				_, valid, e := knowledgeAutomaticValidity(ctx, q, row)
				if e != nil {
					return false, e
				}
				if !valid {
					ids = append(ids, row.ID)
				}
			}
			manifest := []string{}
			for offset, index := 0, int16(0); offset < len(ids); offset, index = offset+3, index+1 {
				end := min(offset+3, len(ids))
				id := knowledgeAutomationID(job.ID, "batch:"+strconv.Itoa(int(index)))
				if _, err = q.CreateKnowledgeAutomationBatch(ctx, db.CreateKnowledgeAutomationBatchParams{ID: id, JobID: job.ID, BatchIndex: index, Intent: "revalidate", PredecessorIds: ids[offset:end], LeaseToken: job.LeaseToken, LeaseGeneration: job.LeaseGeneration}); err != nil {
					return false, err
				}
				manifest = append(manifest, knowledgeUUIDString(id))
			}
			id := knowledgeAutomationID(job.ID, "batch:6")
			if _, err = q.CreateKnowledgeAutomationBatch(ctx, db.CreateKnowledgeAutomationBatchParams{ID: id, JobID: job.ID, BatchIndex: 6, Intent: "create", PredecessorIds: []pgtype.UUID{}, LeaseToken: job.LeaseToken, LeaseGeneration: job.LeaseGeneration}); err != nil {
				return false, err
			}
			manifest = append(manifest, knowledgeUUIDString(id))
			raw, _ := json.Marshal(manifest)
			_, err = q.FreezeKnowledgeAutomationBatches(ctx, db.FreezeKnowledgeAutomationBatchesParams{ID: job.ID, BatchManifest: raw, LeaseToken: job.LeaseToken, LeaseGeneration: job.LeaseGeneration})
			return true, err
		})
		if err != nil {
			return err
		}
	}
	_, err := w.Service.Queries.MarkKnowledgeAbandonedAttemptsUncertain(ctx, db.MarkKnowledgeAbandonedAttemptsUncertainParams{JobID: job.ID, LeaseToken: job.LeaseToken, LeaseGeneration: job.LeaseGeneration})
	if err != nil {
		return err
	}
	batches, err := w.Service.Queries.ListKnowledgeAutomationBatches(ctx, job.ID)
	if err != nil {
		return err
	}
	var selected *db.CollaborationKnowledgeBatch
	for index := range batches {
		if batches[index].Phase == "queued" || batches[index].Phase == "generating" || batches[index].Phase == "checking" || batches[index].Phase == "settling" {
			selected = &batches[index]
			break
		}
	}
	if selected != nil {
		if !w.Model.Enabled() {
			return &knowledgeAutomationStop{"model_not_configured", "waiting_configuration"}
		}
		if err = w.processBatch(ctx, job, *selected); err != nil {
			var stop *knowledgeAutomationStop
			if errors.As(err, &stop) && stop.Phase != "review_pending" {
				return err
			}
			reason := "knowledge_unavailable"
			var model *knowledgeAutomationModelError
			if errors.As(err, &stop) {
				reason = stop.Reason
			}
			if errors.As(err, &model) {
				reason = model.Reason
			}
			if reason == "knowledge_unavailable" {
				return err
			}
			if err = w.pendingBatch(ctx, job, *selected, reason); err != nil {
				return err
			}
		}
	}
	batches, err = w.Service.Queries.ListKnowledgeAutomationBatches(ctx, job.ID)
	if err != nil {
		return err
	}
	phase, reason := "completed", "no_reusable_knowledge"
	for _, batch := range batches {
		if batch.Phase == "review_pending" {
			phase, reason = "review_pending", "review_required"
		}
	}
	for _, batch := range batches {
		if batch.Phase == "queued" || batch.Phase == "generating" || batch.Phase == "checking" || batch.Phase == "settling" {
			phase, reason = "queued", ""
			break
		}
	}
	if phase == "completed" {
		for _, batch := range batches {
			var result knowledgeAutomationBatchResult
			if json.Unmarshal(batch.Result, &result) != nil {
				return automationPending("result_unavailable")
			}
			for _, outcome := range result.Outcomes {
				if outcome.Outcome == "saved" || outcome.Outcome == "restored" || outcome.Outcome == "revised" {
					reason = "knowledge_saved"
				}
			}
		}
	}
	_, err = w.release(ctx, job, phase, reason, []byte(`{}`), 0)
	return err
}

func knowledgeAutomationID(namespace pgtype.UUID, value string) pgtype.UUID {
	return pgtype.UUID{Bytes: uuid.NewSHA1(uuid.UUID(namespace.Bytes), []byte(value)), Valid: true}
}

func (w *KnowledgeAutomationWorker) release(ctx context.Context, job db.CollaborationKnowledgeJob, phase, reason string, result []byte, retries int16) (db.CollaborationKnowledgeJob, error) {
	next := time.Now()
	if phase == "waiting_configuration" {
		next = next.Add(time.Minute)
	} else if retries > 0 {
		next = next.Add(time.Duration(retries) * time.Second)
	}
	return collaborationRuntimeMutation(ctx, w.Service, func(tx pgx.Tx) (db.CollaborationKnowledgeJob, error) {
		return w.Service.Queries.WithTx(tx).ReleaseKnowledgeAutomationJob(ctx, db.ReleaseKnowledgeAutomationJobParams{
			ID: job.ID, ExecutionRevision: job.ExecutionRevision, LeaseToken: job.LeaseToken, LeaseGeneration: job.LeaseGeneration, Phase: phase, ReasonCode: pgtype.Text{String: reason, Valid: reason != ""}, Result: result, DbRetryCount: retries, NextAttemptAt: pgtype.Timestamptz{Time: next, Valid: true}})
	})
}

type knowledgeAutomationStage struct {
	Snapshot knowledgeAutomationSnapshot
	Attempt  db.CollaborationKnowledgeAttempt
	Request  knowledgeAutomationModelRequest
	Reused   bool
}

func (w *KnowledgeAutomationWorker) stage(ctx context.Context, job db.CollaborationKnowledgeJob, batch db.CollaborationKnowledgeBatch, stage string, candidates []knowledgeAutomationCandidate, expected *knowledgeAutomationSnapshot) (knowledgeAutomationStage, error) {
	prepared, err := collaborationRuntimeMutation(ctx, w.Service, func(tx pgx.Tx) (knowledgeAutomationStage, error) {
		q := w.Service.Queries.WithTx(tx)
		snapshot, err := w.snapshot(ctx, q, job, &batch)
		out := knowledgeAutomationStage{Snapshot: snapshot}
		if err != nil {
			return out, err
		}
		if expected != nil && (snapshot.SourceHash != expected.SourceHash || snapshot.TargetHash != expected.TargetHash) {
			return out, automationPending("snapshot_changed")
		}
		if stage == "generating" {
			out.Request, err = buildKnowledgeAutomationGenerationRequest(snapshot.Input)
		} else {
			out.Request, err = buildKnowledgeAutomationCheckRequest(snapshot.Input, candidates)
		}
		if err != nil {
			return out, err
		}
		raw, _ := json.Marshal(struct {
			Request       knowledgeAutomationModelRequest
			Configuration string
		}{out.Request, w.Model.ConfigurationFingerprint()})
		requestHash := protocol.CollaborationKnowledgeHash(string(raw))
		attempts, err := q.ListKnowledgeExecutionModelAttempts(ctx, db.ListKnowledgeExecutionModelAttemptsParams{BatchID: batch.ID, ExecutionRevision: job.ExecutionRevision})
		if err != nil {
			return out, err
		}
		for _, attempt := range attempts {
			if attempt.ExecutionRevision != job.ExecutionRevision || attempt.Stage != stage {
				continue
			}
			if attempt.State != "succeeded" {
				return out, automationPending("model_attempt_uncertain")
			}
			if attempt.SourceSnapshotSha256 != snapshot.SourceHash || attempt.TargetSnapshotSha256 != snapshot.TargetHash || attempt.ConfigurationFingerprint != w.Model.ConfigurationFingerprint() || attempt.RequestSha256 != requestHash {
				return out, automationPending("snapshot_changed")
			}
			out.Attempt, out.Reused = attempt, true
		}
		if !out.Reused {
			closure := snapshot.Closure
			if stage == "generating" {
				closure = snapshot.Sources
			}
			closureJSON, _ := json.Marshal(closure)
			supportJSON, _ := json.Marshal(snapshot.Sources)
			out.Attempt, err = q.CreateKnowledgeModelAttempt(ctx, db.CreateKnowledgeModelAttemptParams{ID: knowledgeAutomationID(batch.ID, stage+":"+strconv.FormatInt(job.ExecutionRevision, 10)), JobID: job.ID, BatchID: batch.ID, ExecutionRevision: job.ExecutionRevision, Stage: stage, AttemptNumber: 1,
				LeaseToken: job.LeaseToken, LeaseGeneration: job.LeaseGeneration, ConfigurationFingerprint: w.Model.ConfigurationFingerprint(), RequestedModel: w.Model.DefaultModel(), SourceSnapshotSha256: snapshot.SourceHash, TargetSnapshotSha256: snapshot.TargetHash, TargetRevision: snapshot.Counter.KnowledgeRevision, RequestSha256: requestHash, InputSourceClosure: closureJSON, SupportDependencies: supportJSON})
			if err != nil {
				return out, err
			}
		}
		current, err := q.GetKnowledgeAutomationBatch(ctx, db.GetKnowledgeAutomationBatchParams{ID: batch.ID, JobID: job.ID})
		if err != nil {
			return out, err
		}
		if stage == "generating" {
			current.GenerationAttemptID = out.Attempt.ID
		} else {
			current.CheckAttemptID = out.Attempt.ID
		}
		if _, err = q.UpdateKnowledgeAutomationBatch(ctx, db.UpdateKnowledgeAutomationBatchParams{ID: batch.ID, JobID: job.ID, ExecutionRevision: job.ExecutionRevision, LeaseToken: job.LeaseToken, LeaseGeneration: job.LeaseGeneration, Phase: stage, GenerationAttemptID: current.GenerationAttemptID, CheckAttemptID: current.CheckAttemptID, Result: current.Result}); err != nil {
			return out, err
		}
		_, err = q.UpdateKnowledgeAutomationJob(ctx, db.UpdateKnowledgeAutomationJobParams{ID: job.ID, ExecutionRevision: job.ExecutionRevision, LeaseToken: job.LeaseToken, LeaseGeneration: job.LeaseGeneration, Phase: stage})
		return out, err
	})
	if err != nil || prepared.Reused {
		return prepared, err
	}
	stageCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	result, modelErr := callKnowledgeAutomationModel(stageCtx, w.Model, prepared.Request)
	cancel()
	if modelErr == nil {
		if stage == "generating" {
			_, modelErr = parseKnowledgeAutomationGeneration(result.Text, prepared.Snapshot.Input)
		} else {
			_, modelErr = parseKnowledgeAutomationChecks(result.Text, prepared.Snapshot.Input, candidates)
		}
	}
	state, reason := "succeeded", ""
	var response []byte
	if modelErr == nil {
		response = []byte(result.Text)
	} else {
		state, reason = "uncertain", "model_attempt_uncertain"
		var invalid *knowledgeAutomationModelError
		if errors.As(modelErr, &invalid) && invalid.Reason != "model_request_failed" && !errors.Is(modelErr, context.DeadlineExceeded) && !errors.Is(modelErr, context.Canceled) {
			state, reason = "failed", invalid.Reason
		}
	}
	usage, _ := json.Marshal(result.Usage)
	recordCtx, done := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer done()
	prepared.Attempt, err = w.Service.Queries.FinishKnowledgeModelAttempt(recordCtx, db.FinishKnowledgeModelAttemptParams{ID: prepared.Attempt.ID, JobID: job.ID, ExecutionRevision: job.ExecutionRevision, LeaseToken: job.LeaseToken, LeaseGeneration: job.LeaseGeneration, State: state, ResponseJson: response, ActualModel: pgtype.Text{String: result.Model, Valid: result.Model != ""}, Usage: usage, ReasonCode: pgtype.Text{String: reason, Valid: reason != ""}})
	if err != nil {
		return prepared, err
	}
	if modelErr != nil {
		return prepared, automationPending(reason)
	}
	return prepared, nil
}

func (w *KnowledgeAutomationWorker) processBatch(ctx context.Context, job db.CollaborationKnowledgeJob, batch db.CollaborationKnowledgeBatch) error {
	retained, settled, err := knowledgeAutomationRetained(batch)
	if err != nil {
		return err
	}
	if batch.Intent == "revalidate" && len(settled) == len(batch.PredecessorIds) {
		raw, _ := json.Marshal(knowledgeAutomationBatchResult{Outcomes: retained})
		_, err = w.Service.Queries.UpdateKnowledgeAutomationBatch(ctx, db.UpdateKnowledgeAutomationBatchParams{ID: batch.ID, JobID: job.ID, ExecutionRevision: job.ExecutionRevision, LeaseToken: job.LeaseToken, LeaseGeneration: job.LeaseGeneration, Phase: "completed", Result: raw})
		return err
	}
	generation, err := w.stage(ctx, job, batch, "generating", nil, nil)
	if err != nil {
		return err
	}
	candidates, err := parseKnowledgeAutomationGeneration(string(generation.Attempt.ResponseJson), generation.Snapshot.Input)
	if err != nil {
		return err
	}
	if len(candidates) == 0 {
		_, err = collaborationRuntimeMutation(ctx, w.Service, func(tx pgx.Tx) (bool, error) {
			q := w.Service.Queries.WithTx(tx)
			snap, e := w.snapshot(ctx, q, job, &batch)
			if e != nil {
				return false, e
			}
			if snap.SourceHash != generation.Snapshot.SourceHash || snap.TargetHash != generation.Snapshot.TargetHash {
				return false, automationPending("snapshot_changed")
			}
			raw, _ := json.Marshal(knowledgeAutomationBatchResult{Outcomes: retained})
			_, e = q.UpdateKnowledgeAutomationBatch(ctx, db.UpdateKnowledgeAutomationBatchParams{ID: batch.ID, JobID: job.ID, ExecutionRevision: job.ExecutionRevision, LeaseToken: job.LeaseToken, LeaseGeneration: job.LeaseGeneration, Phase: "completed", GenerationAttemptID: generation.Attempt.ID, Result: raw})
			return true, e
		})
		return err
	}
	checking, err := w.stage(ctx, job, batch, "checking", candidates, &generation.Snapshot)
	if err != nil {
		return err
	}
	checks, err := parseKnowledgeAutomationChecks(string(checking.Attempt.ResponseJson), checking.Snapshot.Input, candidates)
	if err != nil {
		return err
	}
	return w.settle(ctx, job, batch, checking.Snapshot, candidates, checks)
}

func (w *KnowledgeAutomationWorker) pendingBatch(ctx context.Context, job db.CollaborationKnowledgeJob, batch db.CollaborationKnowledgeBatch, reason string) error {
	current, err := w.Service.Queries.GetKnowledgeAutomationBatch(ctx, db.GetKnowledgeAutomationBatchParams{ID: batch.ID, JobID: job.ID})
	if err != nil {
		return err
	}
	retained, settled, err := knowledgeAutomationRetained(current)
	if err != nil {
		return err
	}
	result := knowledgeAutomationBatchResult{Outcomes: retained}
	if len(batch.PredecessorIds) == 0 {
		var previous knowledgeAutomationBatchResult
		if len(current.Result) > 0 && json.Unmarshal(current.Result, &previous) != nil {
			return automationPending("result_unavailable")
		}
		for _, outcome := range previous.Outcomes {
			if outcome.Outcome == "review_pending" {
				outcome.ReasonCode = reason
				result.Outcomes = append(result.Outcomes, outcome)
			}
		}
		if len(result.Outcomes) == len(retained) {
			result.Outcomes = append(result.Outcomes, knowledgeAutomationOutcome{ExecutionRevision: job.ExecutionRevision, Outcome: "review_pending", ReasonCode: reason})
		}
	} else {
		for index, id := range batch.PredecessorIds {
			ref := "k" + strconv.Itoa(index+1)
			if settled[ref] {
				continue
			}
			result.Outcomes = append(result.Outcomes, knowledgeAutomationOutcome{ExecutionRevision: job.ExecutionRevision, Ref: ref, CandidateID: knowledgeUUIDString(id), Outcome: "review_pending", ReasonCode: reason})
		}
	}
	raw, _ := json.Marshal(result)
	_, err = w.Service.Queries.UpdateKnowledgeAutomationBatch(ctx, db.UpdateKnowledgeAutomationBatchParams{ID: batch.ID, JobID: job.ID, ExecutionRevision: job.ExecutionRevision, LeaseToken: job.LeaseToken, LeaseGeneration: job.LeaseGeneration, Phase: "review_pending", GenerationAttemptID: current.GenerationAttemptID, CheckAttemptID: current.CheckAttemptID, Result: raw})
	return err
}
