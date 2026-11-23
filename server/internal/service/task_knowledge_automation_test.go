package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func automaticConsumerTask(t *testing.T, source *collaborationQueueFixture, project pgtype.UUID) *taskKnowledgeFixture {
	t.Helper()
	// This reader is a source member but is not an issue participant. Reopen
	// therefore cannot be serialized merely by a shared participant-user lock.
	reader := source.outsider
	source.fx.Member(t, source.workspace, reader, "member")
	daemon := "automatic-knowledge-" + uuid.NewString()
	runtime := source.fx.Runtime(t, "Automatic knowledge consumer", testutil.Cols{"provider": "claude", "runtime_mode": "local", "daemon_id": daemon, "owner_id": reader})
	agent := source.fx.Agent(t, "Automatic knowledge consumer", runtime, testutil.Cols{"runtime_mode": "local", "owner_id": reader})
	issue := source.fx.Issue(t, "Subsequent project task", testutil.Cols{"project_id": project, "assignee_type": "agent", "assignee_id": agent})
	id := source.fx.Task(t, agent, testutil.Cols{"runtime_id": runtime, "issue_id": issue, "status": "dispatched", "dispatched_at": testutil.Raw("now()"), "claim_generation": 1, "originator_user_id": reader, "accountable_user_id": reader})
	task, err := source.svc.Queries.GetAgentTask(source.ctx, util.MustParseUUID(id))
	if err != nil {
		t.Fatal(err)
	}
	hash := auth.HashToken("mdt_" + uuid.NewString())
	source.fx.Insert(t, "daemon_token", testutil.Cols{"token_hash": hash, "workspace_id": source.workspace, "daemon_id": daemon, "user_id": reader, "expires_at": testutil.Raw("now()+interval '1 hour'")})
	tasks := NewTaskService(source.svc.Queries, source.pool, nil, events.New())
	tasks.KnowledgeContextEnabled = true
	consumer := *source
	consumer.creator = reader
	return &taskKnowledgeFixture{collaborationQueueFixture: &consumer, tasks: tasks, task: task,
		caller: TaskKnowledgeCaller{RuntimeID: task.RuntimeID, Kind: "daemon_token", TokenHash: hash},
		inv:    protocol.TaskKnowledgeInvocation{ProtocolVersion: 2, Provider: "claude", Mode: "local", Builtin: true, ConfigurationID: "automation-consumer-v2"}}
}

func TestKnowledgeReopenFencesPreparationAndAuthorizationBothOrders(t *testing.T) {
	for _, phase := range []string{"prepare", "authorize"} {
		for _, order := range []string{"reopen_first", "consumer_first"} {
			t.Run(phase+"/"+order, func(t *testing.T) {
				source, ref := knowledgeAcceptedSourceFixture(t, "Accepted reusable conclusion", "The exact result remains reusable")
				candidate := publishConsumptionProof(t, source, ref, nil, 0, nil)
				f := automaticConsumerTask(t, source, candidate.TargetProjectID)
				d := f.finalize(t, "")
				var prepared protocol.TaskKnowledgePrepareResponse
				var err error
				if phase == "authorize" {
					prepared, err = f.tasks.PrepareTaskKnowledge(f.ctx, f.caller, f.task.ID, f.prepareRequest(d))
					if err != nil || len(prepared.Contributions) != 1 || prepared.ProtocolVersion != 2 {
						t.Fatalf("v2 preparation fixture failed: %v", err)
					}
				}
				access := mustAccess(t, source, source.ctx)
				reopen := lifecycleParams(t, source, "reopen", "todo", access.Revision, access.LifecycleEpoch)
				consume := func(ctx context.Context) (int, error) {
					if phase == "prepare" {
						out, err := f.tasks.PrepareTaskKnowledge(ctx, f.caller, f.task.ID, f.prepareRequest(d))
						prepared = out
						return len(out.Contributions), err
					}
					_, err := f.tasks.AuthorizeTaskKnowledgeLaunch(ctx, f.caller, f.task.ID, f.authorizeRequest(prepared, d))
					return 1, err
				}
				if order == "reopen_first" {
					if _, err := source.svc.ChangeLifecycle(source.ctx, reopen); err != nil {
						t.Fatal(err)
					}
					count, err := consume(f.ctx)
					if phase == "prepare" && (err != nil || count != 0) || phase == "authorize" && !collaborationErrorCode(err, "knowledge_context_changed") {
						t.Fatalf("reopen committed first but %s still consumed proof: count=%d err=%v", phase, count, err)
					}
					return
				}
				gate := newKnowledgeCommitGate(t, f.pool)
				f.tasks.TxStarter = gate
				ctx, cancel := context.WithTimeout(f.ctx, 10*time.Second)
				defer cancel()
				type result struct {
					count int
					err   error
				}
				done := make(chan result, 1)
				go func() { count, err := consume(ctx); done <- result{count, err} }()
				var holder uint32
				select {
				case holder = <-gate.entered:
				case value := <-done:
					t.Fatalf("consumer failed before commit fence: %v", value.err)
				case <-ctx.Done():
					t.Fatal("consumer never reached commit fence")
				}
				reopened := make(chan error, 1)
				go func() { _, err := source.svc.ChangeLifecycle(ctx, reopen); reopened <- err }()
				knowledgeWaitForBlock(t, f.pool, holder)
				close(gate.release)
				value := <-done
				if value.err != nil || value.count != 1 {
					t.Fatalf("authorized pre-reopen %s was lost: %v", phase, value.err)
				}
				if err := <-reopened; err != nil {
					t.Fatal("reopen failed after source locks released:", err)
				}
				f.tasks.TxStarter = f.pool
				req := f.authorizeRequest(prepared, d)
				if phase == "authorize" {
					row, err := f.tasks.Queries.GetAgentTask(f.ctx, f.task.ID)
					if err != nil {
						t.Fatal(err)
					}
					metadata, err := decodeTaskKnowledge(row.KnowledgeContextVersion, row.KnowledgeDelivery)
					if err != nil {
						t.Fatal(err)
					}
					authorized := metadata.Claims[0].Attempts[0].Authorization
					if err = f.tasks.ObserveTaskKnowledgeLaunch(f.ctx, f.caller, f.task.ID, protocol.TaskKnowledgeObservation{ProtocolVersion: 2, RequestID: uuid.NewString(), PrepareID: prepared.PrepareID,
						ClaimGeneration: d.ClaimGeneration, DispatchedAt: d.DispatchedAt, AttemptSequence: 1, AttemptNonce: authorized.AttemptNonce, Kind: "terminated", TerminationKind: "not_started"}); err != nil {
						t.Fatal("cleanup observation was refused after reopen:", err)
					}
					req.AttemptSequence = 2
				}
				if _, err := f.tasks.AuthorizeTaskKnowledgeLaunch(f.ctx, f.caller, f.task.ID, req); !collaborationErrorCode(err, "knowledge_context_changed") {
					t.Fatal("post-reopen launch inherited old authorization:", err)
				}
			})
		}
	}
}
