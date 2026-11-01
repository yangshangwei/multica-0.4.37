package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// knowledgeStatementHook runs before one named sqlc statement on the same
// transaction, so a test can move the database clock between two steps of one
// knowledge write. A non-nil error replaces that statement without running it.
type knowledgeStatementHook struct {
	TxStarter
	statement string
	before    func(ctx context.Context, tx pgx.Tx) error
}

func (s knowledgeStatementHook) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return knowledgeStatementHookTx{Tx: tx, hook: s}, nil
}

type knowledgeStatementHookTx struct {
	pgx.Tx
	hook knowledgeStatementHook
}

type knowledgeErrorRow struct{ err error }

func (r knowledgeErrorRow) Scan(...any) error { return r.err }

func (tx knowledgeStatementHookTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if strings.Contains(sql, "-- name: "+tx.hook.statement+" ") {
		if err := tx.hook.before(ctx, tx.Tx); err != nil {
			return knowledgeErrorRow{err: err}
		}
	}
	return tx.Tx.QueryRow(ctx, sql, args...)
}

// Expiry belongs to PostgreSQL clock_timestamp(), so tests read and wait on it.
func knowledgeDBNow(t *testing.T, f *collaborationQueueFixture) time.Time {
	t.Helper()
	var now time.Time
	f.fx.QueryRow(t, `SELECT clock_timestamp()`).Scan(&now)
	return now
}

type knowledgeDBExec interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

func knowledgeSleepPastDB(ctx context.Context, db knowledgeDBExec, at time.Time) error {
	_, err := db.Exec(ctx, `SELECT pg_sleep(GREATEST(0, EXTRACT(EPOCH FROM ($1::timestamptz - clock_timestamp())))::float8 + 0.05)`, at)
	return err
}

// A sealed expiry refusal is the original outcome: it replays and recovers
// unchanged and leaves the candidate, target revision, hints and events as
// they were before the approval.
func knowledgeAssertSealedExpiry(t *testing.T, f *collaborationQueueFixture, candidate string, approve protocol.CollaborationKnowledgeApproveRequest, r protocol.CollaborationKnowledgeReceipt, targetRevision string, hints, events int) {
	t.Helper()
	id := util.MustParseUUID(candidate)
	if r.CandidateID != candidate || r.State != "pending" || r.StateRevision != "1" || r.TargetKnowledgeRevision == nil || *r.TargetKnowledgeRevision != targetRevision {
		t.Fatalf("expiry receipt does not describe the untouched rows: %+v", r)
	}
	want, _ := json.Marshal(r)
	replay, err := f.svc.ApproveKnowledge(f.ctx, id, approve)
	if got, _ := json.Marshal(replay); !collaborationErrorCode(err, "knowledge_expired") || string(got) != string(want) {
		t.Fatalf("expiry replay changed the original outcome: %s %v", got, err)
	}
	recovered, err := f.svc.GetKnowledgeOperation(f.ctx, id, approve.RequestID)
	if got, _ := json.Marshal(recovered); err != nil || string(got) != string(want) {
		t.Fatalf("expiry receipt was not sealed: %s %v", got, err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_operation WHERE scope_kind='issue' AND scope_id=$1 AND request_id=$2 AND kind='knowledge_approve' AND result_code='knowledge_expired' AND completed_at IS NOT NULL`, f.issue.Issue.ID, approve.RequestID) != 1 {
		t.Fatal("expiry refusal left no completed operation")
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_candidate WHERE id=$1 AND state='pending' AND state_revision=1 AND approved_at IS NULL AND published_target_revision IS NULL`, candidate) != 1 {
		t.Fatal("expired candidate changed state")
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_target WHERE target_kind='workspace' AND target_id=$1 AND knowledge_revision=$2::bigint`, f.workspace, targetRevision) != 1 {
		t.Fatal("expiry refusal advanced the target revision")
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_invalidation WHERE target_workspace_id=$1 AND kind='target_hint'`, f.workspace) != hints {
		t.Fatal("expiry refusal retained a target hint")
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1 AND kind='knowledge_changed'`, f.issue.Issue.ID) != events {
		t.Fatal("expiry refusal recorded a knowledge change")
	}
}

type knowledgeBeforeSourceStarter struct {
	TxStarter
	once   *sync.Once
	before func()
}

func (s knowledgeBeforeSourceStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return knowledgeBeforeSourceTx{Tx: tx, once: s.once, before: s.before}, nil
}

type knowledgeBeforeSourceTx struct {
	pgx.Tx
	once   *sync.Once
	before func()
}

func (tx knowledgeBeforeSourceTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	// Workflow edits lock every affected participant's user row. Interleave
	// before that shared actor fence, where the real editor can actually win.
	if strings.Contains(sql, "-- name: LockCollaborationUsers ") {
		tx.once.Do(tx.before)
	}
	return tx.Tx.Query(ctx, sql, args...)
}

type knowledgeCommitGate struct {
	TxStarter
	entered chan uint32
	release chan struct{}
	once    *sync.Once
}

func (s knowledgeCommitGate) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return knowledgeCommitGateTx{Tx: tx, gate: s}, nil
}

type knowledgeCommitGateTx struct {
	pgx.Tx
	gate knowledgeCommitGate
}

func (tx knowledgeCommitGateTx) Commit(ctx context.Context) error {
	tx.gate.once.Do(func() {
		tx.gate.entered <- tx.Conn().PgConn().PID()
		select {
		case <-tx.gate.release:
		case <-ctx.Done():
		}
	})
	return tx.Tx.Commit(ctx)
}
func newKnowledgeCommitGate(t *testing.T, pool *pgxpool.Pool) knowledgeCommitGate {
	t.Helper()
	g := knowledgeCommitGate{TxStarter: pool, entered: make(chan uint32, 1), release: make(chan struct{}), once: &sync.Once{}}
	t.Cleanup(func() {
		select {
		case <-g.release:
		default:
			close(g.release)
		}
	})
	return g
}
func knowledgeWaitForBlock(t *testing.T, pool *pgxpool.Pool, holder uint32) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		var blocked bool
		err := pool.QueryRow(context.Background(), `SELECT EXISTS (SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND $1::integer=ANY(pg_blocking_pids(pid)))`, int32(holder)).Scan(&blocked)
		if err != nil {
			t.Fatal(err)
		}
		if blocked {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("competing transaction never reached the observed PostgreSQL lock barrier")
}

func TestKnowledgeSourceProjectProposalRevalidatesCurrentRelationship(t *testing.T) {
	f := newCollaborationMemoryFixture(t)
	p := knowledgeServiceProposal(t, f)
	oldProject := util.MustParseUUID(f.fx.Project(t, "Original project"))
	newProject := util.MustParseUUID(f.fx.Project(t, "New project"))
	f.fx.Exec(t, `UPDATE issue SET project_id=$2 WHERE id=$1`, f.issue.Issue.ID, oldProject)
	p.Target.Kind = "project"
	p.Target.ProjectID = util.UUIDToString(oldProject)
	editor := NewCollaborationService(f.svc.Queries, f.pool, f.svc.Issues)
	f.svc.TxStarter = knowledgeBeforeSourceStarter{TxStarter: f.pool, once: &sync.Once{}, before: func() {
		if _, err := editor.EditIssue(f.ctx, CollaborationEditParams{CollaborationWorkflowRequest: workflowRequest(f, 1), ProjectID: &newProject}); err != nil {
			t.Fatalf("actual source edit before proposal actor fence: %v", err)
		}
	}}
	if _, err := f.svc.ProposeKnowledge(collaborationContext(f.handler), f.issue.Issue.ID, p); !collaborationErrorCode(err, "knowledge_target_not_found") {
		t.Fatalf("stale source-project reference was accepted: %v", err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_candidate WHERE id=$1`, p.CandidateID) != 0 {
		t.Fatal("stale relationship persisted a proposal")
	}
}

func TestKnowledgeSourceProjectEditAfterProposalKeepsOriginalTarget(t *testing.T) {
	f := newCollaborationMemoryFixture(t)
	p := knowledgeServiceProposal(t, f)
	oldProject := util.MustParseUUID(f.fx.Project(t, "Original project"))
	newProject := util.MustParseUUID(f.fx.Project(t, "New project"))
	f.fx.Exec(t, `UPDATE issue SET project_id=$2 WHERE id=$1`, f.issue.Issue.ID, oldProject)
	p.Target.Kind = "project"
	p.Target.ProjectID = util.UUIDToString(oldProject)
	gate := newKnowledgeCommitGate(t, f.pool)
	f.svc.TxStarter = gate
	proposed := make(chan error, 1)
	go func() {
		_, err := f.svc.ProposeKnowledge(collaborationContext(f.handler), f.issue.Issue.ID, p)
		proposed <- err
	}()
	var holder uint32
	select {
	case holder = <-gate.entered:
	case err := <-proposed:
		t.Fatalf("proposal before barrier: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("proposal did not reach commit")
	}
	editor := NewCollaborationService(f.svc.Queries, f.pool, f.svc.Issues)
	edited := make(chan error, 1)
	go func() {
		_, err := editor.EditIssue(f.ctx, CollaborationEditParams{CollaborationWorkflowRequest: workflowRequest(f, 1), ProjectID: &newProject})
		edited <- err
	}()
	knowledgeWaitForBlock(t, f.pool, holder)
	close(gate.release)
	if err := <-proposed; err != nil {
		t.Fatal(err)
	}
	if err := <-edited; err != nil {
		t.Fatal(err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_candidate WHERE id=$1 AND target_project_id=$2`, p.CandidateID, oldProject) != 1 {
		t.Fatal("later source edit rewrote the immutable proposal target")
	}
}

func TestKnowledgeApprovalAndParticipantRemovalBothCommitOrders(t *testing.T) {
	for _, approvalFirst := range []bool{true, false} {
		t.Run(map[bool]string{true: "approval_first", false: "removal_first"}[approvalFirst], func(t *testing.T) {
			f := newCollaborationMemoryFixture(t)
			p := knowledgeServiceProposal(t, f)
			target := f.fx.Workspace(t, "Approval target", "knowledge-race-"+uuid.NewString())
			f.fx.Member(t, target, f.creator, "member")
			f.fx.Member(t, target, f.handler, "owner")
			p.Target.WorkspaceID = target
			if _, err := f.svc.ProposeKnowledge(f.ctx, f.issue.Issue.ID, p); err != nil {
				t.Fatal(err)
			}
			gate := newKnowledgeCommitGate(t, f.pool)
			first := NewCollaborationService(f.svc.Queries, gate, f.svc.Issues)
			second := NewCollaborationService(f.svc.Queries, f.pool, f.svc.Issues)
			approve := func(s *CollaborationService) error {
				_, err := s.ApproveKnowledge(collaborationContext(f.handler), util.MustParseUUID(p.CandidateID), knowledgeApproveRequest(p))
				return err
			}
			remove := func(s *CollaborationService) error {
				_, err := s.RemoveParticipant(f.ctx, CollaborationParticipantParams{CollaborationWorkflowRequest: workflowRequest(f, 1), TargetUserID: util.MustParseUUID(f.handler)})
				return err
			}
			firstResult := make(chan error, 1)
			secondResult := make(chan error, 1)
			go func() {
				if approvalFirst {
					firstResult <- approve(first)
				} else {
					firstResult <- remove(first)
				}
			}()
			var holder uint32
			select {
			case holder = <-gate.entered:
			case err := <-firstResult:
				t.Fatalf("first writer before barrier: %v", err)
			case <-time.After(3 * time.Second):
				t.Fatal("first writer did not reach commit")
			}
			go func() {
				if approvalFirst {
					secondResult <- remove(second)
				} else {
					secondResult <- approve(second)
				}
			}()
			knowledgeWaitForBlock(t, f.pool, holder)
			close(gate.release)
			if err := <-firstResult; err != nil {
				t.Fatal(err)
			}
			err := <-secondResult
			if approvalFirst && err != nil || !approvalFirst && !collaborationErrorCode(err, "knowledge_permission_denied") {
				t.Fatalf("second writer: %v", err)
			}
			want := 0
			if approvalFirst {
				want = 1
			}
			if f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_candidate WHERE id=$1 AND state='approved'`, p.CandidateID) != want {
				t.Fatal("approval ignored source authority linearization")
			}
			view, err := second.KnowledgeCandidate(collaborationContext(f.handler), util.MustParseUUID(p.CandidateID))
			if err != nil || view.Candidate.Availability != "restricted" || view.Candidate.Content != nil {
				t.Fatalf("removed actor retained source body: %+v %v", view, err)
			}
		})
	}
}

func TestKnowledgeCorrectionApprovalCannotReplaceNewerApprovedVersion(t *testing.T) {
	f := newCollaborationMemoryFixture(t)
	p := knowledgeServiceProposal(t, f)
	if _, err := f.svc.ProposeKnowledge(f.ctx, f.issue.Issue.ID, p); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.ApproveKnowledge(f.ctx, util.MustParseUUID(p.CandidateID), knowledgeApproveRequest(p)); err != nil {
		t.Fatal(err)
	}
	second := p
	second.CandidateID = uuid.NewString()
	second.RequestID = uuid.NewString()
	second.SupersedesCandidateID = &p.CandidateID
	if _, err := f.svc.ProposeKnowledge(f.ctx, f.issue.Issue.ID, second); err != nil {
		t.Fatal(err)
	}
	third := second
	third.CandidateID = uuid.NewString()
	third.RequestID = uuid.NewString()
	third.SupersedesCandidateID = &second.CandidateID
	if _, err := f.svc.ProposeKnowledge(f.ctx, f.issue.Issue.ID, third); err != nil {
		t.Fatal(err)
	}
	request := knowledgeApproveRequest(third)
	request.ExpectedVersion = "3"
	if _, err := f.svc.ApproveKnowledge(f.ctx, util.MustParseUUID(third.CandidateID), request); err != nil {
		t.Fatal(err)
	}
	request = knowledgeApproveRequest(second)
	request.ExpectedVersion = "2"
	if _, err := f.svc.ApproveKnowledge(f.ctx, util.MustParseUUID(second.CandidateID), request); !collaborationErrorCode(err, "knowledge_state_conflict") {
		t.Fatalf("older pending correction replaced the newer approved version: %v", err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_candidate WHERE lineage_id=$1 AND state='approved' AND version=3`, p.CandidateID) != 1 {
		t.Fatal("lineage approval regressed")
	}
}

func TestKnowledgeOwnerApprovalDoesNotContendWithProjectDescriptionUpdate(t *testing.T) {
	f := newCollaborationMemoryFixture(t)
	p := knowledgeServiceProposal(t, f)
	project := f.fx.Project(t, "Independent project edit")
	p.Target.Kind = "project"
	p.Target.ProjectID = project
	if _, err := f.svc.ProposeKnowledge(f.ctx, f.issue.Issue.ID, p); err != nil {
		t.Fatal(err)
	}
	gate := newKnowledgeCommitGate(t, f.pool)
	f.svc.TxStarter = gate
	approved := make(chan error, 1)
	go func() {
		_, err := f.svc.ApproveKnowledge(f.ctx, util.MustParseUUID(p.CandidateID), knowledgeApproveRequest(p))
		approved <- err
	}()
	select {
	case <-gate.entered:
	case err := <-approved:
		t.Fatalf("approval before commit gate: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("approval missed gate")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, editErr := f.pool.Exec(ctx, `UPDATE project SET description='ordinary concurrent edit',revision=revision+1 WHERE id=$1`, project)
	close(gate.release)
	if err := <-approved; err != nil {
		t.Fatal(err)
	}
	if editErr != nil {
		t.Fatalf("owner approval held an unnecessary lead/description lock: %v", editErr)
	}
}

// L1: the approval UPDATE compares expires_at with clock_timestamp(). An
// application clock that lags PostgreSQL must not let an expired candidate
// reach that UPDATE and come back as a missing candidate.
func TestKnowledgeApprovalExpiryUsesDatabaseClockNotApplicationClock(t *testing.T) {
	f := newCollaborationMemoryFixture(t)
	p := knowledgeServiceProposal(t, f)
	expires := knowledgeDBNow(t, f).Add(1500 * time.Millisecond).UTC()
	p.ExpiresAt = &expires
	if _, err := f.svc.ProposeKnowledge(f.ctx, f.issue.Issue.ID, p); err != nil {
		t.Fatal(err)
	}
	if err := knowledgeSleepPastDB(context.Background(), f.pool, expires); err != nil {
		t.Fatal(err)
	}
	f.svc.knowledgeClock = func() time.Time { return time.Now().Add(-time.Hour) }
	approve := knowledgeApproveRequest(p)
	r, err := f.svc.ApproveKnowledge(f.ctx, util.MustParseUUID(p.CandidateID), approve)
	var refusal *CollaborationError
	if !errors.As(err, &refusal) || refusal.Status != http.StatusConflict || refusal.Code != "knowledge_expired" || r.Code != "knowledge_expired" {
		t.Fatalf("approval after database expiry with a lagging application clock: %+v %v", r, err)
	}
	knowledgeAssertSealedExpiry(t, f, p.CandidateID, approve, r, "0", 0, 1)
}

// L1: expiry can pass after the precheck while the correction approval has
// already advanced the target and superseded the old version. That attempt
// must roll back and retry into the sealed 409, not report 404 or commit.
func TestKnowledgeCorrectionApprovalExpiringMidTransactionRetriesIntoSealedExpiry(t *testing.T) {
	f := newCollaborationMemoryFixture(t)
	p := knowledgeServiceProposal(t, f)
	if _, err := f.svc.ProposeKnowledge(f.ctx, f.issue.Issue.ID, p); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.ApproveKnowledge(f.ctx, util.MustParseUUID(p.CandidateID), knowledgeApproveRequest(p)); err != nil {
		t.Fatal(err)
	}
	correction := p
	correction.CandidateID = uuid.NewString()
	correction.RequestID = uuid.NewString()
	correction.SupersedesCandidateID = &p.CandidateID
	correction.Content = "PRIVATE_KNOWLEDGE_BODY corrected"
	expires := knowledgeDBNow(t, f).Add(2 * time.Second).UTC()
	correction.ExpiresAt = &expires
	if _, err := f.svc.ProposeKnowledge(f.ctx, f.issue.Issue.ID, correction); err != nil {
		t.Fatal(err)
	}
	hooked := 0
	f.svc.TxStarter = knowledgeStatementHook{TxStarter: f.pool, statement: "ApproveKnowledgeCandidate", before: func(ctx context.Context, tx pgx.Tx) error {
		hooked++
		return knowledgeSleepPastDB(ctx, tx, expires)
	}}
	approve := knowledgeApproveRequest(correction)
	approve.ExpectedVersion = "2"
	r, err := f.svc.ApproveKnowledge(f.ctx, util.MustParseUUID(correction.CandidateID), approve)
	if hooked != 1 {
		t.Fatalf("approval reached the expiring UPDATE %d times; want exactly the first attempt", hooked)
	}
	var refusal *CollaborationError
	if !errors.As(err, &refusal) || refusal.Status != http.StatusConflict || refusal.Code != "knowledge_expired" || r.Code != "knowledge_expired" {
		t.Fatalf("approval expiring between precheck and UPDATE: %+v %v", r, err)
	}
	f.svc.TxStarter = f.pool
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_candidate WHERE id=$1 AND state='approved' AND state_revision=2 AND published_target_revision=1`, p.CandidateID) != 1 {
		t.Fatal("rolled-back approval left the previous version superseded")
	}
	knowledgeAssertSealedExpiry(t, f, correction.CandidateID, approve, r, "1", 1, 3)
}
