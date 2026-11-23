package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/llm"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type automationModelInput struct {
	Mode          string                              `json:"mode"`
	MaxCandidates int                                 `json:"max_candidates"`
	Sources       []knowledgeAutomationSourceText     `json:"sources"`
	Predecessors  []knowledgeAutomationPriorText      `json:"predecessors"`
	Candidates    []knowledgeAutomationCandidateText  `json:"candidates"`
	Comparisons   []knowledgeAutomationComparisonText `json:"comparisons"`
}

func automationTestModel(t *testing.T, respond func(automationModelInput) any) (*llm.Client, *atomic.Int32) {
	t.Helper()
	calls := &atomic.Int32{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var wire struct {
			Messages []struct{ Role, Content string } `json:"messages"`
		}
		if json.NewDecoder(r.Body).Decode(&wire) != nil || len(wire.Messages) != 2 {
			t.Error("invalid model request")
			w.WriteHeader(400)
			return
		}
		var input automationModelInput
		if json.Unmarshal([]byte(wire.Messages[1].Content), &input) != nil {
			t.Error("invalid bounded input")
			w.WriteHeader(400)
			return
		}
		if len(input.Candidates) == 0 && len(input.Comparisons) > 0 {
			t.Error("generation received comparison text")
		}
		value := respond(input)
		if status, ok := value.(int); ok {
			w.WriteHeader(status)
			return
		}
		content, err := json.Marshal(value)
		if err != nil {
			t.Error(err)
			w.WriteHeader(500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"model": "fake-accounted-model", "choices": []any{map[string]any{"message": map[string]any{"content": string(content)}, "finish_reason": "stop"}}, "usage": map[string]int{"prompt_tokens": 10, "completion_tokens": 7, "total_tokens": 17}})
	}))
	t.Cleanup(server.Close)
	retries, err := llm.Retries(0)
	if err != nil {
		t.Fatal(err)
	}
	return llm.New(llm.Config{BaseURL: server.URL, APIKey: "fake-test-only", DefaultModel: "fake-request-model", MaxRetries: retries}), calls
}

func automationAcceptFixture(t *testing.T, result string) (*collaborationQueueFixture, pgtype.UUID, db.CollaborationKnowledgeJob) {
	t.Helper()
	f := newCollaborationHandoffFixture(t)
	project := util.MustParseUUID(f.fx.Project(t, "Automation project"))
	f.fx.Exec(t, `UPDATE issue SET project_id=$1 WHERE id=$2`, project, f.issue.Issue.ID)
	p := handoffAcceptance(t, f, 1, result)
	p.Basis = "The submitted project contract is accepted as the current result."
	submitted, err := f.svc.SubmitAcceptance(collaborationContext(f.handler), p)
	if err != nil {
		t.Fatal(err)
	}
	id := submitted.DeliveryConfirmation.SubmissionID
	accept := lifecycleParams(t, f, "complete", "done", 2, 1)
	accept.SubmissionID = &id
	if _, err = f.svc.ChangeLifecycle(f.ctx, accept); err != nil {
		t.Fatal(err)
	}
	return f, project, automationLatestJob(t, f)
}

func automationLatestJob(t *testing.T, f *collaborationQueueFixture) db.CollaborationKnowledgeJob {
	t.Helper()
	rows, err := f.svc.Queries.ListKnowledgeAutomationJobs(f.ctx, db.ListKnowledgeAutomationJobsParams{SourceWorkspaceID: util.MustParseUUID(f.workspace), SourceIssueID: f.issue.Issue.ID, Limit: 1})
	if err != nil || len(rows) != 1 {
		t.Fatalf("accepted job: %+v %v", rows, err)
	}
	return rows[0]
}

func automationDrive(t *testing.T, w *KnowledgeAutomationWorker, f *collaborationQueueFixture, job db.CollaborationKnowledgeJob) db.CollaborationKnowledgeJob {
	t.Helper()
	for range 20 {
		current, err := f.svc.Queries.GetKnowledgeAutomationJob(f.ctx, job.ID)
		if err != nil {
			t.Fatal(err)
		}
		if current.Phase == "completed" || current.Phase == "review_pending" || current.Phase == "waiting_configuration" || current.Phase == "skipped" || current.Phase == "cancelled" {
			return current
		}
		// Other suites retain their immutable job history in this package's DB.
		// Select this fixture's due work without deleting or draining that history.
		f.fx.Exec(t, `UPDATE collaboration_knowledge_job SET next_attempt_at='1970-01-01T00:00:00Z' WHERE id=$1`, job.ID)
		worked, err := w.ProcessNext(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if !worked {
			t.Fatalf("expected ready job remained %s", current.Phase)
		}
	}
	t.Fatal("bounded job did not settle")
	return db.CollaborationKnowledgeJob{}
}

func automationSupportedResponse(input automationModelInput) any {
	if len(input.Candidates) > 0 {
		checks := []knowledgeAutomationCheck{}
		for _, candidate := range input.Candidates {
			check := knowledgeAutomationCheck{Ref: candidate.Ref, Support: "supported", Reuse: "reusable", Relation: "novel", RelatedRefs: []string{}, Reason: "The current accepted source supports the statement."}
			for _, comparison := range input.Comparisons {
				if comparison.PredecessorFor == candidate.Ref {
					check.Relation = "duplicate"
					check.RelatedRefs = []string{comparison.Ref}
				}
			}
			checks = append(checks, check)
		}
		return map[string]any{"checks": checks}
	}
	values := []knowledgeAutomationPriorText{}
	if input.Mode == "review" {
		values = input.Predecessors
	} else {
		lines := strings.Split(input.Sources[0].Result, "\n")
		if len(lines) > input.MaxCandidates {
			lines = lines[len(lines)-input.MaxCandidates:]
		}
		for index, line := range lines {
			values = append(values, knowledgeAutomationPriorText{Ref: fmt.Sprintf("k%d", index+1), Content: line, Summary: line})
		}
	}
	candidates := []any{}
	for _, value := range values {
		candidates = append(candidates, map[string]any{"ref": value.Ref, "summary": value.Summary, "statements": []knowledgeAutomationStatement{{Text: value.Content, SourceRef: "s1", Field: "result", Quote: value.Content}}})
	}
	return map[string]any{"candidates": candidates}
}

func automationReaccept(t *testing.T, f *collaborationQueueFixture, result string) db.CollaborationKnowledgeJob {
	t.Helper()
	state, err := f.svc.Queries.GetKnowledgeSourceState(f.ctx, f.issue.Issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.ChangeLifecycle(f.ctx, lifecycleParams(t, f, "reopen", "todo", state.Revision, state.LifecycleEpoch)); err != nil {
		t.Fatal(err)
	}
	state, err = f.svc.Queries.GetKnowledgeSourceState(f.ctx, f.issue.Issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	p := lifecycleParams(t, f, "complete", "done", state.Revision, state.LifecycleEpoch)
	p.Note = result
	p.Basis = "The current complete project rules remain supported by this accepted revision."
	if _, err = f.svc.ChangeLifecycle(f.ctx, p); err != nil {
		t.Fatal(err)
	}
	return automationLatestJob(t, f)
}

func TestKnowledgeAutomationWorkerAcceptedCreateAndCompleteRevalidation(t *testing.T) {
	text := "Amounts are expressed in cents.\nRetries use an idempotency key.\nTimestamps use UTC."
	f, project, job := automationAcceptFixture(t, text)
	model, calls := automationTestModel(t, automationSupportedResponse)
	worker := NewKnowledgeAutomationWorker(f.svc, model)
	if done := automationDrive(t, worker, f, job); done.Phase != "completed" {
		t.Fatalf("first job: %s %s", done.Phase, done.ReasonCode.String)
	}
	if calls.Load() != 2 || f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_candidate WHERE source_issue_id=$1 AND state='approved'`, f.issue.Issue.ID) != 3 {
		t.Fatalf("first acceptance did not save3 in two phases: calls%d", calls.Load())
	}
	view, err := f.svc.ListKnowledgeAutomationJobs(f.ctx, f.issue.Issue.ID)
	if err != nil || view.Items[0].Counts.Saved != 3 || view.Items[0].Attempts[0].Usage.Status != llm.UsageKnown {
		t.Fatalf("truthful status/usage: %+v %v", view, err)
	}
	text += "\nErrors retain a stable code."
	second := automationReaccept(t, f, text)
	if done := automationDrive(t, worker, f, second); done.Phase != "completed" {
		t.Fatalf("second job: %s %s", done.Phase, done.ReasonCode.String)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_candidate WHERE source_issue_id=$1 AND state='approved'`, f.issue.Issue.ID) != 4 {
		t.Fatal("reacceptance did not restore3 and add1")
	}
	third := automationReaccept(t, f, text)
	if done := automationDrive(t, worker, f, third); done.Phase != "completed" {
		t.Fatalf("third job: %s %s", done.Phase, done.ReasonCode.String)
	}
	batches, err := f.svc.Queries.ListKnowledgeAutomationBatches(f.ctx, third.ID)
	if err != nil || len(batches) != 3 || len(batches[0].PredecessorIds)+len(batches[1].PredecessorIds) != 4 {
		t.Fatalf("review truncated at3: %+v %v", batches, err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_validity WHERE source_issue_id=$1 AND project_id=$2 AND accepted_event_seq=$3 AND validated_close_epoch=$4`, f.issue.Issue.ID, project, third.AcceptedEventSeq, third.ValidatedCloseEpoch) != 4 {
		t.Fatal("not all four automatic heads moved to current acceptance")
	}
	rows, err := f.svc.Queries.ListKnowledgeApprovedTargetCandidates(f.ctx, db.ListKnowledgeApprovedTargetCandidatesParams{TargetProjectID: project, TargetWorkspaceID: util.MustParseUUID(f.workspace)})
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		_, active, err := knowledgeAutomaticValidity(f.ctx, f.svc.Queries, row)
		if err != nil || !active {
			t.Fatalf("saved proof not consumable: %v %v", active, err)
		}
	}
}

func TestKnowledgeAutomationWorkerUnconfiguredZeroAndUncertain(t *testing.T) {
	t.Run("unconfigured", func(t *testing.T) {
		f, _, job := automationAcceptFixture(t, "Amounts are expressed in cents.")
		done := automationDrive(t, NewKnowledgeAutomationWorker(f.svc, llm.New(llm.Config{})), f, job)
		if done.Phase != "waiting_configuration" || f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_attempt WHERE job_id=$1`, job.ID) != 0 {
			t.Fatal("unconfigured work called a model or lost wait state")
		}
	})
	t.Run("zero", func(t *testing.T) {
		f, _, job := automationAcceptFixture(t, "A one-time question was answered.")
		model, calls := automationTestModel(t, func(automationModelInput) any { return map[string]any{"candidates": []any{}} })
		done := automationDrive(t, NewKnowledgeAutomationWorker(f.svc, model), f, job)
		if done.Phase != "completed" || done.ReasonCode.String != "no_reusable_knowledge" || calls.Load() != 1 {
			t.Fatalf("zero result: %+v calls%d", done, calls.Load())
		}
	})
	t.Run("uncertain", func(t *testing.T) {
		f, _, job := automationAcceptFixture(t, "Amounts are expressed in cents.")
		model, calls := automationTestModel(t, func(automationModelInput) any { return 502 })
		worker := NewKnowledgeAutomationWorker(f.svc, model)
		done := automationDrive(t, worker, f, job)
		if done.Phase != "review_pending" || calls.Load() != 1 {
			t.Fatalf("failed model did not stop: %s calls%d", done.Phase, calls.Load())
		}
		if _, err := worker.ProcessNext(context.Background()); err != nil {
			t.Fatal(err)
		}
		if calls.Load() != 1 || f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_attempt WHERE job_id=$1 AND state='uncertain'`, job.ID) != 1 {
			t.Fatal("uncertain generation was replayed or fabricated as unused")
		}
	})
}

func TestKnowledgeAutomationWorkerAtomicSettlementAndStageReuse(t *testing.T) {
	f, _, job := automationAcceptFixture(t, "Amounts are expressed in cents.\nTimestamps use UTC.")
	f.fx.Exec(t, `UPDATE collaboration_knowledge_job SET next_attempt_at='1970-01-01T00:00:00Z' WHERE id=$1`, job.ID)
	model, calls := automationTestModel(t, automationSupportedResponse)
	worker := NewKnowledgeAutomationWorker(f.svc, model)
	var approvals atomic.Int32
	f.svc.TxStarter = knowledgeStatementHook{TxStarter: f.pool, statement: "ApproveAutomatedKnowledgeCandidate", before: func(context.Context, pgx.Tx) error {
		if approvals.Add(1) == 2 {
			return errors.New("injected second approval failure")
		}
		return nil
	}}
	if _, err := worker.ProcessNext(context.Background()); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 || f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_candidate WHERE source_issue_id=$1`, f.issue.Issue.ID) != 0 || f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_evaluation WHERE job_id=$1`, job.ID) != 0 {
		t.Fatal("failed batch left partial candidate/evaluation writes")
	}
	f.svc.TxStarter = f.pool
	f.fx.Exec(t, `UPDATE collaboration_knowledge_job SET next_attempt_at=clock_timestamp() WHERE id=$1`, job.ID)
	if done := automationDrive(t, worker, f, job); done.Phase != "completed" {
		t.Fatalf("replayed settlement: %s %s", done.Phase, done.ReasonCode.String)
	}
	if calls.Load() != 2 || f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_candidate WHERE source_issue_id=$1 AND state='approved'`, f.issue.Issue.ID) != 2 {
		t.Fatal("saved model stages were replayed or atomic result did not recover")
	}
}

func TestKnowledgeAutomationWorkerRevisionAndHumanWithdrawal(t *testing.T) {
	for _, mode := range []string{"revise", "withdraw"} {
		t.Run(mode, func(t *testing.T) {
			original := "Amounts are expressed in cents."
			f, project, job := automationAcceptFixture(t, original)
			model, _ := automationTestModel(t, func(input automationModelInput) any {
				if mode == "revise" && input.Mode == "review" {
					return map[string]any{"candidates": []any{map[string]any{"ref": "k1", "summary": input.Sources[0].Result,
						"statements": []knowledgeAutomationStatement{{Text: input.Sources[0].Result, SourceRef: "s1", Field: "result", Quote: input.Sources[0].Result}}}}}
				}
				value := automationSupportedResponse(input)
				if mode == "revise" && len(input.Candidates) > 0 && input.Candidates[0].Intent == "revise" {
					checks := value.(map[string]any)["checks"].([]knowledgeAutomationCheck)
					checks[0].Relation = "conflict"
				}
				return value
			})
			worker := NewKnowledgeAutomationWorker(f.svc, model)
			if done := automationDrive(t, worker, f, job); done.Phase != "completed" {
				t.Fatal(done.Phase, done.ReasonCode.String)
			}
			rows, err := f.svc.Queries.ListKnowledgeApprovedTargetCandidates(f.ctx, db.ListKnowledgeApprovedTargetCandidatesParams{TargetProjectID: project, TargetWorkspaceID: util.MustParseUUID(f.workspace)})
			if err != nil || len(rows) != 1 {
				t.Fatal(rows, err)
			}
			old := rows[0]
			updated := original
			if mode == "revise" {
				updated = "Amounts are expressed in minor currency units."
			}
			next := automationReaccept(t, f, updated)
			if mode == "withdraw" {
				_, err = f.svc.WithdrawKnowledge(WithKnowledgeProtocolVersion(f.ctx, 2), old.ID, protocol.CollaborationKnowledgeDecisionRequest{
					ProtocolVersion: 2, RequestID: uuid.NewString(), ExpectedStateRevision: "2", Reason: "Human withdrawal must not be reversed."})
				if err != nil {
					t.Fatal(err)
				}
			}
			done := automationDrive(t, worker, f, next)
			if mode == "revise" {
				if done.Phase != "completed" {
					t.Fatal(done.Phase, done.ReasonCode.String)
				}
				if f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_candidate WHERE lineage_id=$1 AND version=2 AND state='approved' AND content=$2`, old.LineageID, updated) != 1 ||
					f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_candidate WHERE id=$1 AND state='superseded' AND approval_job_id=$2`, old.ID, job.ID) != 1 {
					t.Fatal("revision did not preserve and replace exact predecessor")
				}
			} else {
				if done.Phase != "review_pending" || f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_candidate WHERE source_issue_id=$1 AND state='approved'`, f.issue.Issue.ID) != 0 ||
					f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_candidate WHERE id=$1 AND state='withdrawn' AND decision_origin='human'`, old.ID) != 1 {
					t.Fatal("fresh candidate ID bypassed human withdrawal")
				}
			}
		})
	}
}

func TestKnowledgeAutomationWorkerCurrentAuthorityWinsAfterModel(t *testing.T) {
	for _, change := range []string{"policy", "reopen"} {
		t.Run(change, func(t *testing.T) {
			f, project, job := automationAcceptFixture(t, "Amounts are expressed in cents.")
			model, calls := automationTestModel(t, func(input automationModelInput) any {
				if len(input.Candidates) > 0 {
					if change == "policy" {
						if _, err := f.svc.UpdateProjectKnowledgePolicy(f.ctx, project, util.MustParseUUID(f.workspace), ProjectKnowledgePolicyRequest{
							RequestID: uuid.NewString(), ExpectedRevision: "0", IsEnabled: false, MaxCandidates: 3}); err != nil {
							t.Error(err)
						}
					} else {
						state, err := f.svc.Queries.GetKnowledgeSourceState(f.ctx, f.issue.Issue.ID)
						if err != nil {
							t.Error(err)
						} else if _, err = f.svc.ChangeLifecycle(f.ctx, lifecycleParams(t, f, "reopen", "todo", state.Revision, state.LifecycleEpoch)); err != nil {
							t.Error(err)
						}
					}
				}
				return automationSupportedResponse(input)
			})
			done := automationDrive(t, NewKnowledgeAutomationWorker(f.svc, model), f, job)
			want := "skipped"
			if change == "reopen" {
				want = "cancelled"
			}
			if done.Phase != want || calls.Load() != 2 || f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_candidate WHERE source_issue_id=$1`, f.issue.Issue.ID) != 0 {
				t.Fatalf("late model ignored current %s: phase=%s calls=%d", change, done.Phase, calls.Load())
			}
			if f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1 AND event_seq=$2 AND kind='completed'`, f.issue.Issue.ID, job.AcceptedEventSeq) != 1 {
				t.Fatal("background refusal rolled back original business acceptance")
			}
		})
	}
}

func TestKnowledgeAutomationWorkerExplicitRetryRefreshesPolicy(t *testing.T) {
	f, project, job := automationAcceptFixture(t, "Amounts are expressed in cents.\nTimestamps use UTC.")
	var first atomic.Bool
	model, calls := automationTestModel(t, func(input automationModelInput) any {
		if !first.Swap(true) {
			return 502
		}
		return automationSupportedResponse(input)
	})
	worker := NewKnowledgeAutomationWorker(f.svc, model)
	if done := automationDrive(t, worker, f, job); done.Phase != "review_pending" {
		t.Fatal(done.Phase)
	}
	if _, err := f.svc.UpdateProjectKnowledgePolicy(f.ctx, project, util.MustParseUUID(f.workspace), ProjectKnowledgePolicyRequest{
		RequestID: uuid.NewString(), ExpectedRevision: "0", IsEnabled: true, MaxCandidates: 1}); err != nil {
		t.Fatal(err)
	}
	request := KnowledgeAutomationRetryRequest{RequestID: uuid.NewString(), ExpectedRevision: "1"}
	receipt, err := f.svc.RetryKnowledgeAutomationJob(f.ctx, f.issue.Issue.ID, job.ID, request)
	if err != nil || receipt.Revision != "2" {
		t.Fatal(receipt, err)
	}
	done := automationDrive(t, worker, f, job)
	if done.Phase != "completed" || done.PolicyRevision != 0 || done.ExecutionPolicyRevision != 1 || done.ExecutionMaxCandidates != 1 || calls.Load() != 3 {
		t.Fatalf("explicit retry lost original policy or failed to use current policy: %+v calls=%d", done, calls.Load())
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_attempt WHERE job_id=$1 AND execution_revision=1 AND state='uncertain'`, job.ID) != 1 {
		t.Fatal("retry erased uncertain original model attempt")
	}
	rows, err := f.svc.Queries.ListKnowledgeApprovedTargetCandidates(f.ctx, db.ListKnowledgeApprovedTargetCandidatesParams{TargetProjectID: project, TargetWorkspaceID: util.MustParseUUID(f.workspace)})
	if err != nil || len(rows) != 1 || rows[0].AutomationPolicyRevision.Int64 != 1 {
		t.Fatal(rows, err)
	}
	if _, valid, err := knowledgeAutomaticValidity(f.ctx, f.svc.Queries, rows[0]); err != nil || !valid {
		t.Fatal("current execution policy proof is not valid", err)
	}
	if replay, err := f.svc.RetryKnowledgeAutomationJob(f.ctx, f.issue.Issue.ID, job.ID, request); err != nil || replay != receipt {
		t.Fatal("retry receipt changed after completion", replay, err)
	}
	if recovered, err := f.svc.GetKnowledgeAutomationRetryOperation(f.ctx, f.issue.Issue.ID, job.ID, request.RequestID); err != nil || recovered != receipt {
		t.Fatal("GET-only original retry recovery failed", recovered, err)
	}
}

func TestKnowledgeAutomationWorkerAcceptanceQuotaCountsPublishedLineages(t *testing.T) {
	for _, scenario := range []struct {
		name           string
		initialSaved   int
		approvePending bool
		withdraw       bool
		revise         bool
		remaining      int
	}{
		{name: "explicit_retry", initialSaved: 1, remaining: 2},
		{name: "human_approved_automatic_pending", initialSaved: 2, approvePending: true},
		{name: "withdrawn_publication", initialSaved: 2, approvePending: true, withdraw: true},
		{name: "human_revised_publication", initialSaved: 2, approvePending: true, revise: true},
		{name: "same_lineage_revision_leaves_one_slot", initialSaved: 2, revise: true, remaining: 1},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			initial := []string{"Amounts are expressed in cents.", "Retries use an idempotency key.", "Timestamps use UTC."}
			fresh := []string{"Errors retain a stable code.", "Page cursors include the final record ID.", "Configuration changes require a revision."}
			correctionText := "Amount fields store integer cents."
			result := strings.Join(initial, "\n") + "\n" + strings.Join(fresh, "\n") + "\n" + correctionText
			f, project, job := automationAcceptFixture(t, result)
			var generations atomic.Int32
			model, calls := automationTestModel(t, func(input automationModelInput) any {
				if len(input.Candidates) > 0 {
					response := automationSupportedResponse(input)
					checks := response.(map[string]any)["checks"].([]knowledgeAutomationCheck)
					for index, candidate := range input.Candidates {
						for _, pending := range initial[scenario.initialSaved:] {
							if candidate.Content == pending {
								checks[index].Support = "uncertain"
							}
						}
					}
					return response
				}
				texts, wantMax := initial, 3
				if generations.Add(1) > 1 {
					texts, wantMax = fresh, scenario.remaining
				}
				if input.Mode != "create" || input.MaxCandidates != wantMax {
					t.Errorf("acceptance publication budget: mode=%s max=%d, want create/%d", input.Mode, input.MaxCandidates, wantMax)
				}
				candidates := []any{}
				for index, text := range texts[:input.MaxCandidates] {
					candidates = append(candidates, map[string]any{"ref": fmt.Sprintf("k%d", index+1), "summary": text,
						"statements": []knowledgeAutomationStatement{{Text: text, SourceRef: "s1", Field: "result", Quote: text}}})
				}
				return map[string]any{"candidates": candidates}
			})
			worker := NewKnowledgeAutomationWorker(f.svc, model)
			if done := automationDrive(t, worker, f, job); done.Phase != "review_pending" || calls.Load() != 2 {
				t.Fatalf("initial partial publication: phase=%s reason=%s calls=%d", done.Phase, done.ReasonCode.String, calls.Load())
			}
			roots := make([]db.CollaborationKnowledgeCandidate, len(initial))
			approved := map[string]string{}
			for index, text := range initial {
				var id string
				f.fx.QueryRow(t, `SELECT id::text FROM collaboration_knowledge_candidate WHERE automation_job_id=$1 AND content=$2`, job.ID, text).Scan(&id)
				row, err := f.svc.Queries.GetKnowledgeCandidate(f.ctx, util.MustParseUUID(id))
				if err != nil {
					t.Fatal(err)
				}
				wantState := "pending"
				if index < scenario.initialSaved {
					wantState = "approved"
					approved[text] = "automation"
				}
				if row.State != wantState || row.ID != row.LineageID || row.Version != 1 || row.ProposalOrigin != "automation" || row.PublishedTargetRevision.Valid != (wantState == "approved") {
					t.Fatalf("initial logical lineage %d: %+v", index, row)
				}
				roots[index] = row
			}
			humanCtx := WithKnowledgeProtocolVersion(f.ctx, 2)
			if scenario.approvePending {
				pending := roots[2]
				receipt, err := f.svc.ApproveKnowledge(humanCtx, pending.ID, protocol.CollaborationKnowledgeApproveRequest{
					ProtocolVersion: 2, RequestID: uuid.NewString(), ExpectedStateRevision: fmt.Sprint(pending.StateRevision),
					ExpectedVersion: fmt.Sprint(pending.Version), ExpectedContentSHA256: pending.ContentSha256})
				if err != nil || receipt.Code != "knowledge_approved" {
					t.Fatal("human approval of original automatic pending candidate", receipt, err)
				}
				row, err := f.svc.Queries.GetKnowledgeCandidate(f.ctx, pending.ID)
				if err != nil || row.ProposalOrigin != "automation" || row.ApprovalOrigin != "human" || row.AutomationJobID != job.ID || !row.PublishedTargetRevision.Valid {
					t.Fatal("human approval lost the acceptance's automatic lineage", row, err)
				}
				approved[pending.Content] = "human"
			}
			if scenario.withdraw {
				receipt, err := f.svc.WithdrawKnowledge(humanCtx, roots[0].ID, protocol.CollaborationKnowledgeDecisionRequest{
					ProtocolVersion: 2, RequestID: uuid.NewString(), ExpectedStateRevision: fmt.Sprint(roots[0].StateRevision), Reason: "Withdraw this published convention."})
				if err != nil || receipt.State != "withdrawn" {
					t.Fatal("human withdrawal of published lineage", receipt, err)
				}
				delete(approved, roots[0].Content)
			}
			if scenario.revise {
				sources, err := knowledgeCandidateSources(roots[0])
				if err != nil {
					t.Fatal(err)
				}
				predecessorID := knowledgeUUIDString(roots[0].ID)
				proposal := protocol.CollaborationKnowledgeProposal{ProtocolVersion: 2, CandidateID: uuid.NewString(), RequestID: uuid.NewString(),
					Target: knowledgeTarget(roots[0]), Content: correctionText, Summary: correctionText, Sources: sources,
					Evidence: []protocol.CollaborationKnowledgeEvidence{}, Applicability: knowledgeAutomationApplicability, SupersedesCandidateID: &predecessorID}
				if receipt, err := f.svc.ProposeKnowledge(humanCtx, f.issue.Issue.ID, proposal); err != nil || receipt.Code != "knowledge_proposed" || receipt.Version != "2" {
					t.Fatal("human correction proposal", receipt, err)
				}
				id := util.MustParseUUID(proposal.CandidateID)
				if receipt, err := f.svc.ApproveKnowledge(humanCtx, id, protocol.CollaborationKnowledgeApproveRequest{
					ProtocolVersion: 2, RequestID: uuid.NewString(), ExpectedStateRevision: "1", ExpectedVersion: "2", ExpectedContentSHA256: protocol.CollaborationKnowledgeHash(correctionText)}); err != nil || receipt.Code != "knowledge_approved" {
					t.Fatal("human correction approval", receipt, err)
				}
				row, err := f.svc.Queries.GetKnowledgeCandidate(f.ctx, id)
				if err != nil || row.LineageID != roots[0].LineageID || row.SupersedesCandidateID != roots[0].ID || row.Version != 2 || row.ApprovalOrigin != "human" || !row.PublishedTargetRevision.Valid {
					t.Fatal("correction created another logical lineage or lost publication", row, err)
				}
				delete(approved, roots[0].Content)
				approved[correctionText] = "human"
			}
			counter, err := f.svc.Queries.GetKnowledgeTarget(f.ctx, db.GetKnowledgeTargetParams{TargetKind: "project", TargetID: project})
			if err != nil {
				t.Fatal(err)
			}
			candidatesBeforeRetry := f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_candidate WHERE source_issue_id=$1`, f.issue.Issue.ID)
			request := KnowledgeAutomationRetryRequest{RequestID: uuid.NewString(), ExpectedRevision: "1"}
			receipt, err := f.svc.RetryKnowledgeAutomationJob(f.ctx, f.issue.Issue.ID, job.ID, request)
			if err != nil || receipt.Code != "knowledge_job_retried" || receipt.Revision != "2" || receipt.JobID != knowledgeUUIDString(job.ID) {
				t.Fatal("explicit retry must retain this accepted job", receipt, err)
			}
			done := automationDrive(t, worker, f, job)
			if done.ID != job.ID || done.SubmissionID != job.SubmissionID || done.AcceptedEventSeq != job.AcceptedEventSeq || done.ExecutionRevision != 2 {
				t.Fatal("retry changed acceptance identity", done)
			}
			if scenario.remaining > 0 {
				if done.Phase != "completed" || generations.Load() != 2 || calls.Load() != 4 {
					t.Fatalf("remaining logical slots were not published: phase=%s reason=%s generations=%d calls=%d", done.Phase, done.ReasonCode.String, generations.Load(), calls.Load())
				}
				for _, text := range fresh[:scenario.remaining] {
					approved[text] = "automation"
				}
			} else {
				if done.Phase != "review_pending" || generations.Load() != 1 || calls.Load() != 2 || f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_attempt WHERE job_id=$1 AND execution_revision=2`, job.ID) != 0 ||
					f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_candidate WHERE source_issue_id=$1`, f.issue.Issue.ID) != candidatesBeforeRetry {
					t.Fatalf("full acceptance quota admitted fresh content: phase=%s generations=%d calls=%d", done.Phase, generations.Load(), calls.Load())
				}
				batches, err := f.svc.Queries.ListKnowledgeAutomationBatches(f.ctx, job.ID)
				if err != nil || len(batches) != 1 {
					t.Fatal(batches, err)
				}
				var result knowledgeAutomationBatchResult
				if err = json.Unmarshal(batches[0].Result, &result); err != nil {
					t.Fatal(err)
				}
				budgetRefusal := false
				for _, outcome := range result.Outcomes {
					budgetRefusal = budgetRefusal || outcome.Outcome == "review_pending" && outcome.ReasonCode == "new_candidate_budget_exceeded"
				}
				if !budgetRefusal {
					t.Fatal("retry stopped without proving the acceptance quota", result)
				}
				current, err := f.svc.Queries.GetKnowledgeTarget(f.ctx, db.GetKnowledgeTargetParams{TargetKind: "project", TargetID: project})
				if err != nil || current.KnowledgeRevision != counter.KnowledgeRevision {
					t.Fatal("quota refusal changed published target revision", current, err)
				}
			}
			secondRequest := KnowledgeAutomationRetryRequest{RequestID: uuid.NewString(), ExpectedRevision: "2"}
			secondReceipt, err := f.svc.RetryKnowledgeAutomationJob(f.ctx, f.issue.Issue.ID, job.ID, secondRequest)
			if scenario.remaining > 0 {
				if !collaborationErrorCode(err, "knowledge_job_revision_conflict") || secondReceipt.Code != "knowledge_job_revision_conflict" || secondReceipt.Revision != "2" {
					t.Fatal("completed acceptance admitted another execution", secondReceipt, err)
				}
			} else {
				if err != nil || secondReceipt.Revision != "3" {
					t.Fatal("second explicit retry", secondReceipt, err)
				}
				if repeated := automationDrive(t, worker, f, job); repeated.Phase != "review_pending" || calls.Load() != 2 {
					t.Fatal("another explicit retry reset the publication quota", repeated.Phase, calls.Load())
				}
			}
			rows, err := f.svc.Queries.ListKnowledgeApprovedTargetCandidates(f.ctx, db.ListKnowledgeApprovedTargetCandidatesParams{TargetProjectID: project, TargetWorkspaceID: util.MustParseUUID(f.workspace)})
			if err != nil || len(rows) != len(approved) {
				t.Fatal("fourth logical publication or lost existing contribution", rows, err)
			}
			for _, row := range rows {
				if origin, exists := approved[row.Content]; !exists || row.ApprovalOrigin != origin {
					t.Fatal("unexpected published content or decision origin", row)
				}
			}
			for _, text := range fresh[scenario.remaining:] {
				if f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_candidate WHERE source_issue_id=$1 AND content=$2`, f.issue.Issue.ID, text) != 0 {
					t.Fatal("retry created content beyond the remaining acceptance quota", text)
				}
			}
			if scenario.withdraw || scenario.revise {
				row, err := f.svc.Queries.GetKnowledgeCandidate(f.ctx, roots[0].ID)
				wantState := "withdrawn"
				if scenario.revise {
					wantState = "superseded"
				}
				if err != nil || row.State != wantState || row.DecisionOrigin != "human" || row.PublishedTargetRevision != roots[0].PublishedTargetRevision || row.ApprovalJobID != job.ID || row.Content != roots[0].Content {
					t.Fatal("retry reset human disposition or immutable publication history", row, err)
				}
			}
		})
	}
}

func TestKnowledgeAutomationWorkerPartialCapacityIsAtomic(t *testing.T) {
	f, project, job := automationAcceptFixture(t, "Amounts are expressed in cents.\nTimestamps use UTC.")
	issue, err := f.svc.Queries.GetIssue(f.ctx, f.issue.Issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	delivery, available, err := knowledgeAcceptedDelivery(f.ctx, f.svc.Queries, issue, job.SubmissionID, job.DeliveryEventSeq, job.AcceptedEventSeq)
	if err != nil || !available {
		t.Fatal("source unavailable", err)
	}
	for index := 0; index < 15; index++ {
		body := fmt.Sprintf("Existing human convention %d.", index)
		proposal := protocol.CollaborationKnowledgeProposal{ProtocolVersion: 2, CandidateID: uuid.NewString(), RequestID: uuid.NewString(),
			Target:  protocol.CollaborationKnowledgeTarget{Kind: "project", WorkspaceID: f.workspace, ProjectID: knowledgeUUIDString(project)},
			Content: body, Summary: body, Sources: []protocol.CollaborationKnowledgeSource{delivery.Source}, Evidence: []protocol.CollaborationKnowledgeEvidence{}, Applicability: knowledgeAutomationApplicability}
		if _, err := f.svc.ProposeKnowledge(WithKnowledgeProtocolVersion(f.ctx, 2), f.issue.Issue.ID, proposal); err != nil {
			t.Fatal(err)
		}
		if _, err := f.svc.ApproveKnowledge(WithKnowledgeProtocolVersion(f.ctx, 2), util.MustParseUUID(proposal.CandidateID), protocol.CollaborationKnowledgeApproveRequest{
			ProtocolVersion: 2, RequestID: uuid.NewString(), ExpectedStateRevision: "1", ExpectedVersion: "1", ExpectedContentSHA256: protocol.CollaborationKnowledgeHash(body)}); err != nil {
			t.Fatal(err)
		}
	}
	model, calls := automationTestModel(t, automationSupportedResponse)
	done := automationDrive(t, NewKnowledgeAutomationWorker(f.svc, model), f, job)
	if done.Phase != "review_pending" || calls.Load() != 2 || f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_candidate WHERE target_project_id=$1 AND state='approved'`, project) != 16 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_candidate WHERE target_project_id=$1 AND proposal_origin='automation' AND state='pending'`, project) != 1 {
		t.Fatal("partial capacity was truncated, evicted old knowledge or misreported")
	}
	view, err := f.svc.ListKnowledgeAutomationJobs(f.ctx, f.issue.Issue.ID)
	if err != nil || view.Items[0].Counts.Saved != 1 || view.Items[0].Counts.Pending != 1 {
		t.Fatal("atomic per-candidate outcomes missing", view, err)
	}
}

func TestKnowledgeAutomationWorkerComparisonClosureAndPrivateReason(t *testing.T) {
	for _, revokeDuringCheck := range []bool{false, true} {
		t.Run(fmt.Sprintf("revoke_during_check_%t", revokeDuringCheck), func(t *testing.T) {
			f, project, job := automationAcceptFixture(t, "Amounts are expressed in cents.")
			other := newCollaborationMemoryFixture(t)
			other.fx.Member(t, other.workspace, f.creator, "member")
			other.fx.Member(t, f.workspace, other.creator, "member")
			secret := "PRIVATE_COMPARISON_REASON_SENTINEL"
			if _, err := other.svc.UpdateMemory(other.ctx, CollaborationMemoryUpdateParams{IssueID: other.issue.Issue.ID, RequestID: uuid.NewString(), Content: secret}); err != nil {
				t.Fatal(err)
			}
			proposal := protocol.CollaborationKnowledgeProposal{ProtocolVersion: 2, CandidateID: uuid.NewString(), RequestID: uuid.NewString(),
				Target:  protocol.CollaborationKnowledgeTarget{Kind: "project", WorkspaceID: f.workspace, ProjectID: knowledgeUUIDString(project)},
				Content: secret, Summary: "Private comparison", Sources: []protocol.CollaborationKnowledgeSource{{Kind: "memory", IssueID: knowledgeUUIDString(other.issue.Issue.ID), Version: "1", SHA256: protocol.CollaborationKnowledgeHash(secret)}},
				Evidence: []protocol.CollaborationKnowledgeEvidence{}, Applicability: knowledgeAutomationApplicability}
			if _, err := other.svc.ProposeKnowledge(WithKnowledgeProtocolVersion(other.ctx, 2), other.issue.Issue.ID, proposal); err != nil {
				t.Fatal(err)
			}
			if _, err := f.svc.ApproveKnowledge(WithKnowledgeProtocolVersion(f.ctx, 2), util.MustParseUUID(proposal.CandidateID), protocol.CollaborationKnowledgeApproveRequest{
				ProtocolVersion: 2, RequestID: uuid.NewString(), ExpectedStateRevision: "1", ExpectedVersion: "1", ExpectedContentSHA256: protocol.CollaborationKnowledgeHash(secret)}); err != nil {
				t.Fatal(err)
			}
			model, _ := automationTestModel(t, func(input automationModelInput) any {
				if len(input.Candidates) == 0 {
					raw, _ := json.Marshal(input)
					if strings.Contains(string(raw), secret) {
						t.Error("generation received another source's private comparison")
					}
					return automationSupportedResponse(input)
				}
				if len(input.Comparisons) != 1 || input.Comparisons[0].Content != secret {
					t.Error("checker lost the full comparison")
				}
				if revokeDuringCheck {
					f.fx.Exec(t, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, other.workspace, f.creator)
				}
				return map[string]any{"checks": []knowledgeAutomationCheck{{Ref: "k1", Support: "supported", Reuse: "reusable", Relation: "novel", RelatedRefs: []string{}, Reason: secret}}}
			})
			done := automationDrive(t, NewKnowledgeAutomationWorker(f.svc, model), f, job)
			want := "completed"
			if revokeDuringCheck {
				want = "review_pending"
			}
			if done.Phase != want {
				t.Fatalf("comparison gate: %s %s", done.Phase, done.ReasonCode.String)
			}
			if !revokeDuringCheck {
				f.fx.Exec(t, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, other.workspace, f.creator)
			}
			view, err := f.svc.ListKnowledgeAutomationJobs(f.ctx, f.issue.Issue.ID)
			if err != nil {
				t.Fatal(err)
			}
			raw, _ := json.Marshal(view)
			if strings.Contains(string(raw), secret) {
				t.Fatal("job projection leaked free-text model comparison reason")
			}
			var closure string
			f.fx.QueryRow(t, `SELECT input_source_closure::text FROM collaboration_knowledge_attempt WHERE job_id=$1 AND stage='checking'`, job.ID).Scan(&closure)
			if !strings.Contains(closure, knowledgeUUIDString(other.issue.Issue.ID)) {
				t.Fatal("model related_refs shrank actual input-source closure")
			}
			if revokeDuringCheck && f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_candidate WHERE source_issue_id=$1`, f.issue.Issue.ID) != 0 {
				t.Fatal("partial readable comparison was used to approve")
			}
		})
	}
}

func TestKnowledgeAutomationWorkerBoundsConcurrentBatches(t *testing.T) {
	f, _, first := automationAcceptFixture(t, "Amounts are expressed in cents.")
	_, _, second := automationAcceptFixture(t, "Timestamps use UTC.")
	thirdFixture, _, third := automationAcceptFixture(t, "Retries use an idempotency key.")
	f.fx.Exec(t, `UPDATE collaboration_knowledge_job SET next_attempt_at='1970-01-01T00:00:00Z' WHERE id=ANY($1::uuid[])`, []pgtype.UUID{first.ID, second.ID})
	entered, release := make(chan struct{}, 3), make(chan struct{})
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
	})
	model, calls := automationTestModel(t, func(input automationModelInput) any {
		if len(input.Candidates) == 0 {
			entered <- struct{}{}
			<-release
		}
		return automationSupportedResponse(input)
	})
	worker := NewKnowledgeAutomationWorker(f.svc, model)
	results := make(chan error, 2)
	for range 2 {
		go func() { _, err := worker.ProcessNext(context.Background()); results <- err }()
	}
	for range 2 {
		select {
		case <-entered:
		case <-time.After(10 * time.Second):
			t.Fatal("two admitted batches did not reach their model stage")
		}
	}
	if worked, err := worker.ProcessNext(context.Background()); err != nil || worked || calls.Load() != 2 {
		t.Fatal("more than two batches admitted", worked, err, calls.Load())
	}
	close(release)
	for range 2 {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	if calls.Load() != 4 {
		t.Fatal("two batches did not execute exactly two phases", calls.Load())
	}
	for _, job := range []db.CollaborationKnowledgeJob{first, second} {
		row, err := f.svc.Queries.GetKnowledgeAutomationJob(f.ctx, job.ID)
		if err != nil || row.Phase != "completed" {
			t.Fatal("admitted batch failed", row.Phase, err)
		}
	}
	if done := automationDrive(t, worker, thirdFixture, third); done.Phase != "completed" {
		t.Fatal("queued third batch was lost", done.Phase)
	}
}

func TestKnowledgeAutomationWorkerExpiredLeasePreservesUncertainty(t *testing.T) {
	f, _, job := automationAcceptFixture(t, "Amounts are expressed in cents.")
	f.fx.Exec(t, `UPDATE collaboration_knowledge_job SET next_attempt_at='1970-01-01T00:00:00Z' WHERE id=$1`, job.ID)
	entered, release := make(chan struct{}, 1), make(chan struct{})
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
	})
	model, calls := automationTestModel(t, func(input automationModelInput) any {
		entered <- struct{}{}
		<-release
		return automationSupportedResponse(input)
	})
	oldWorker := NewKnowledgeAutomationWorker(f.svc, model)
	finished := make(chan error, 1)
	go func() { _, err := oldWorker.ProcessNext(context.Background()); finished <- err }()
	select {
	case <-entered:
	case <-time.After(10 * time.Second):
		t.Fatal("original owner did not persist and start its attempt")
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_attempt WHERE job_id=$1 AND state='running'`, job.ID) != 1 {
		t.Fatal("model request began before its durable attempt")
	}
	f.fx.Exec(t, `UPDATE collaboration_knowledge_job SET lease_expires_at=clock_timestamp()-interval '1 second' WHERE id=$1`, job.ID)
	newWorker := NewKnowledgeAutomationWorker(f.svc, model)
	if worked, err := newWorker.ProcessNext(context.Background()); err != nil || !worked {
		t.Fatal("expired lease was not recoverable", worked, err)
	}
	if calls.Load() != 1 || f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_attempt WHERE job_id=$1 AND state='uncertain' AND usage->>'status'='unknown'`, job.ID) != 1 {
		t.Fatal("takeover replayed or erased the unknown model attempt")
	}
	close(release)
	if err := <-finished; err == nil {
		t.Fatal("late previous lease owner was allowed to settle")
	}
	current, err := f.svc.Queries.GetKnowledgeAutomationJob(f.ctx, job.ID)
	if err != nil || current.Phase != "review_pending" || current.LeaseGeneration != 2 || calls.Load() != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_candidate WHERE source_issue_id=$1`, f.issue.Issue.ID) != 0 {
		t.Fatal("late model result restored old lease authority", current, err)
	}
}

func TestKnowledgeAutomationWorkerLaterAcceptancePassesStaleAutomaticProposal(t *testing.T) {
	original, conflicting, revised := "Amounts are expressed in cents.", "Amounts are expressed in major units.", "Amounts use minor currency units."
	f, project, job := automationAcceptFixture(t, original)
	model, _ := automationTestModel(t, func(input automationModelInput) any {
		if input.Mode == "create" && input.Sources[0].Result != original {
			return map[string]any{"candidates": []any{}}
		}
		if input.Mode == "review" {
			text := input.Sources[0].Result
			return map[string]any{"candidates": []any{map[string]any{"ref": "k1", "summary": text,
				"statements": []knowledgeAutomationStatement{{Text: text, SourceRef: "s1", Field: "result", Quote: text}}}}}
		}
		response := automationSupportedResponse(input)
		if len(input.Candidates) > 0 && input.Candidates[0].Intent == "revise" {
			checks := response.(map[string]any)["checks"].([]knowledgeAutomationCheck)
			checks[0].Relation = "conflict"
			if input.Candidates[0].Content == conflicting {
				for _, comparison := range input.Comparisons {
					if comparison.PredecessorFor == "" {
						checks[0].RelatedRefs = append(checks[0].RelatedRefs, comparison.Ref)
					}
				}
			}
		}
		return response
	})
	worker := NewKnowledgeAutomationWorker(f.svc, model)
	if done := automationDrive(t, worker, f, job); done.Phase != "completed" {
		t.Fatal(done.Phase, done.ReasonCode.String)
	}
	rows, err := f.svc.Queries.ListKnowledgeApprovedTargetCandidates(f.ctx, db.ListKnowledgeApprovedTargetCandidatesParams{TargetProjectID: project, TargetWorkspaceID: util.MustParseUUID(f.workspace)})
	if err != nil || len(rows) != 1 {
		t.Fatal(rows, err)
	}
	root := rows[0]
	issue, err := f.svc.Queries.GetIssue(f.ctx, f.issue.Issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	delivery, ok, err := knowledgeAcceptedDelivery(f.ctx, f.svc.Queries, issue, job.SubmissionID, job.DeliveryEventSeq, job.AcceptedEventSeq)
	if err != nil || !ok {
		t.Fatal(ok, err)
	}
	manual := protocol.CollaborationKnowledgeProposal{ProtocolVersion: 2, CandidateID: uuid.NewString(), RequestID: uuid.NewString(),
		Target: knowledgeTarget(root), Content: "Balances use integer storage.", Summary: "Integer storage", Sources: []protocol.CollaborationKnowledgeSource{delivery.Source},
		Evidence: []protocol.CollaborationKnowledgeEvidence{}, Applicability: knowledgeAutomationApplicability}
	if _, err = f.svc.ProposeKnowledge(WithKnowledgeProtocolVersion(f.ctx, 2), f.issue.Issue.ID, manual); err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.ApproveKnowledge(WithKnowledgeProtocolVersion(f.ctx, 2), util.MustParseUUID(manual.CandidateID), protocol.CollaborationKnowledgeApproveRequest{
		ProtocolVersion: 2, RequestID: uuid.NewString(), ExpectedStateRevision: "1", ExpectedVersion: "1", ExpectedContentSHA256: protocol.CollaborationKnowledgeHash(manual.Content)}); err != nil {
		t.Fatal(err)
	}
	conflictJob := automationReaccept(t, f, conflicting)
	if done := automationDrive(t, worker, f, conflictJob); done.Phase != "review_pending" {
		t.Fatal("conflict did not retain pending revision", done.Phase, done.ReasonCode.String)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_candidate WHERE lineage_id=$1 AND version=2 AND state='pending' AND proposal_origin='automation'`, root.LineageID) != 1 {
		t.Fatal("expected retained automatic pending v2")
	}
	restoreJob := automationReaccept(t, f, original)
	if done := automationDrive(t, worker, f, restoreJob); done.Phase != "completed" {
		t.Fatal("stale automatic proposal blocked restoring v1", done.Phase, done.ReasonCode.String)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_validity WHERE candidate_id=$1 AND accepted_event_seq=$2`, root.ID, restoreJob.AcceptedEventSeq) != 1 {
		t.Fatal("original version did not receive new proof")
	}
	revisionJob := automationReaccept(t, f, revised)
	if done := automationDrive(t, worker, f, revisionJob); done.Phase != "completed" {
		t.Fatal("stale automatic proposal blocked later revision", done.Phase, done.ReasonCode.String)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_candidate WHERE lineage_id=$1 AND version=3 AND state='approved' AND supersedes_candidate_id=$2`, root.LineageID, root.ID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_candidate WHERE lineage_id=$1 AND version=2 AND state='pending'`, root.LineageID) != 1 {
		t.Fatal("later allocation collided with or erased pending v2")
	}
	var currentID string
	f.fx.QueryRow(t, `SELECT id::text FROM collaboration_knowledge_candidate WHERE lineage_id=$1 AND state='approved'`, root.LineageID).Scan(&currentID)
	correction := manual
	correction.CandidateID, correction.RequestID, correction.SupersedesCandidateID = uuid.NewString(), uuid.NewString(), &currentID
	correction.Content, correction.Summary = "Human correction awaits review.", "Human correction"
	if _, err = f.svc.ProposeKnowledge(WithKnowledgeProtocolVersion(f.ctx, 2), f.issue.Issue.ID, correction); err != nil {
		t.Fatal(err)
	}
	humanJob := automationReaccept(t, f, revised)
	if done := automationDrive(t, worker, f, humanJob); done.Phase != "review_pending" {
		t.Fatal("pending human correction was not authoritative", done.Phase)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_candidate WHERE id=$1 AND state='pending' AND proposal_origin='human'`, correction.CandidateID) != 1 {
		t.Fatal("background work changed pending human correction")
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_validity WHERE candidate_id=$1 AND accepted_event_seq=$2`, currentID, humanJob.AcceptedEventSeq) != 0 {
		t.Fatal("new proof bypassed a human correction")
	}
}

func TestKnowledgeAutomationWorkerLateComparisonIsAuthorizedBeforeAnyModel(t *testing.T) {
	f, project, job := automationAcceptFixture(t, "Amounts are expressed in cents.")
	other := newCollaborationMemoryFixture(t)
	other.fx.Member(t, f.workspace, other.creator, "admin")
	secret := "LATE_PRIVATE_COMPARISON_SENTINEL"
	if _, err := other.svc.UpdateMemory(other.ctx, CollaborationMemoryUpdateParams{IssueID: other.issue.Issue.ID, RequestID: uuid.NewString(), Content: secret}); err != nil {
		t.Fatal(err)
	}
	proposal := protocol.CollaborationKnowledgeProposal{ProtocolVersion: 2, CandidateID: uuid.NewString(), RequestID: uuid.NewString(),
		Target:  protocol.CollaborationKnowledgeTarget{Kind: "project", WorkspaceID: f.workspace, ProjectID: knowledgeUUIDString(project)},
		Content: secret, Summary: "Late private comparison", Sources: []protocol.CollaborationKnowledgeSource{{Kind: "memory", IssueID: knowledgeUUIDString(other.issue.Issue.ID), Version: "1", SHA256: protocol.CollaborationKnowledgeHash(secret)}},
		Evidence: []protocol.CollaborationKnowledgeEvidence{}, Applicability: knowledgeAutomationApplicability}
	if _, err := other.svc.ProposeKnowledge(WithKnowledgeProtocolVersion(other.ctx, 2), other.issue.Issue.ID, proposal); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Queries.InitializeKnowledgeTarget(f.ctx, db.InitializeKnowledgeTargetParams{TargetKind: "project", TargetID: project, WorkspaceID: util.MustParseUUID(f.workspace)}); err != nil {
		t.Fatal(err)
	}
	var published atomic.Bool
	f.svc.TxStarter = knowledgeStatementHook{TxStarter: f.pool, statement: "LockKnowledgeTargetForWrite", before: func(context.Context, pgx.Tx) error {
		if published.Swap(true) {
			return nil
		}
		_, err := other.svc.ApproveKnowledge(WithKnowledgeProtocolVersion(other.ctx, 2), util.MustParseUUID(proposal.CandidateID), protocol.CollaborationKnowledgeApproveRequest{
			ProtocolVersion: 2, RequestID: uuid.NewString(), ExpectedStateRevision: "1", ExpectedVersion: "1", ExpectedContentSHA256: protocol.CollaborationKnowledgeHash(secret)})
		return err
	}}
	model, calls := automationTestModel(t, automationSupportedResponse)
	done := automationDrive(t, NewKnowledgeAutomationWorker(f.svc, model), f, job)
	if !published.Load() || done.Phase != "review_pending" || calls.Load() != 0 || f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_attempt WHERE job_id=$1`, job.ID) != 0 {
		t.Fatalf("late source bypassed lock/read coverage: published=%t phase=%s calls=%d", published.Load(), done.Phase, calls.Load())
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_candidate WHERE id=$1 AND state='approved'`, proposal.CandidateID) != 1 {
		t.Fatal("race did not exercise an actual committed publication")
	}
}
