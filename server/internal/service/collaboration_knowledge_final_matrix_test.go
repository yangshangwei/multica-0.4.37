package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func knowledgeApprovalMatrixGuard(t *testing.T) {
	t.Helper()
	u, err := url.Parse(os.Getenv("DATABASE_URL"))
	if err != nil || u.Path == "" {
		t.Fatal("approval matrix requires an explicit isolated DATABASE_URL")
	}
	if expected := os.Getenv("MULTICA_KNOWLEDGE_MATRIX_DATABASE"); expected != "" && strings.TrimPrefix(u.Path, "/") != expected {
		t.Fatal("approval matrix database identity differs from its exclusive marker")
	}
}

type knowledgeApprovalMatrixResult struct {
	receipt protocol.CollaborationKnowledgeReceipt
	err     error
}
type knowledgeApprovalMatrixCall func(TxStarter) knowledgeApprovalMatrixResult
type knowledgeApprovalMatrixSQLState struct{ code, query string }
type knowledgeApprovalMatrixObserver struct {
	TxStarter
	states chan knowledgeApprovalMatrixSQLState
}
type knowledgeApprovalMatrixTx struct {
	pgx.Tx
	states chan knowledgeApprovalMatrixSQLState
}
type knowledgeApprovalMatrixRow struct {
	pgx.Row
	states chan knowledgeApprovalMatrixSQLState
	query  string
}

func (s knowledgeApprovalMatrixObserver) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return knowledgeApprovalMatrixTx{Tx: tx, states: s.states}, nil
}
func knowledgeApprovalMatrixRecord(states chan knowledgeApprovalMatrixSQLState, query string, err error) {
	var pgerr *pgconn.PgError
	if errors.As(err, &pgerr) {
		select {
		case states <- knowledgeApprovalMatrixSQLState{code: pgerr.Code, query: strings.SplitN(query, "\n", 2)[0]}:
		default:
		}
	}
}
func (tx knowledgeApprovalMatrixTx) QueryRow(ctx context.Context, query string, args ...any) pgx.Row {
	return knowledgeApprovalMatrixRow{Row: tx.Tx.QueryRow(ctx, query, args...), states: tx.states, query: query}
}
func (r knowledgeApprovalMatrixRow) Scan(values ...any) error {
	err := r.Row.Scan(values...)
	knowledgeApprovalMatrixRecord(r.states, r.query, err)
	return err
}
func (tx knowledgeApprovalMatrixTx) Exec(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error) {
	tag, err := tx.Tx.Exec(ctx, query, args...)
	knowledgeApprovalMatrixRecord(tx.states, query, err)
	return tag, err
}

func knowledgeApprovalMatrixService(f *collaborationQueueFixture, starter TxStarter) *CollaborationService {
	copy := *f.svc
	copy.TxStarter = starter
	return &copy
}
func knowledgeApprovalMatrixContext(t *testing.T, ctx context.Context) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	t.Cleanup(cancel)
	return ctx
}
func knowledgeApprovalMatrixAwait(t *testing.T, result <-chan knowledgeApprovalMatrixResult) knowledgeApprovalMatrixResult {
	t.Helper()
	select {
	case out := <-result:
		return out
	case <-time.After(8 * time.Second):
		t.Fatal("actual writer failed to finish after its commit barrier was released")
		return knowledgeApprovalMatrixResult{}
	}
}

// The first callback is an actual writer held immediately before commit. The
// follower either reaches an observed PG wait chain or the production NOWAIT
// retry boundary. No business mutation is replaced by a raw lock statement.
func knowledgeApprovalMatrixPair(t *testing.T, f *collaborationQueueFixture, first, second knowledgeApprovalMatrixCall, nowait bool) (knowledgeApprovalMatrixResult, knowledgeApprovalMatrixResult) {
	t.Helper()
	gate := newKnowledgeCommitGate(t, f.pool)
	firstDone, secondDone := make(chan knowledgeApprovalMatrixResult, 1), make(chan knowledgeApprovalMatrixResult, 1)
	go func() { firstDone <- first(gate) }()
	var pid uint32
	select {
	case pid = <-gate.entered:
	case out := <-firstDone:
		t.Fatalf("first writer failed before commit barrier: %v", out.err)
	case <-time.After(8 * time.Second):
		t.Fatal("first actual writer did not reach commit barrier")
	}
	states := make(chan knowledgeApprovalMatrixSQLState, 16)
	go func() { secondDone <- second(knowledgeApprovalMatrixObserver{TxStarter: f.pool, states: states}) }()
	var follower knowledgeApprovalMatrixResult
	if nowait {
		follower = knowledgeApprovalMatrixAwait(t, secondDone)
		if !collaborationErrorCode(follower.err, "knowledge_busy") {
			t.Fatalf("contended actual approval did not refuse boundedly: %v", follower.err)
		}
		observed := false
		for len(states) > 0 {
			state := <-states
			if state.code == "55P03" {
				observed = true
				t.Logf("production NOWAIT boundary: %s SQLSTATE %s", state.query, state.code)
			}
		}
		if !observed {
			t.Fatal("bounded refusal lacked a PostgreSQL NOWAIT observation")
		}
	} else {
		knowledgeWaitForBlock(t, f.pool, pid)
		t.Log("observed actual follower's PostgreSQL wait chain before first commit")
	}
	close(gate.release)
	leader := knowledgeApprovalMatrixAwait(t, firstDone)
	if !nowait {
		follower = knowledgeApprovalMatrixAwait(t, secondDone)
	}
	return leader, follower
}

func knowledgeApprovalMatrixRequest(p protocol.CollaborationKnowledgeProposal, version int64) protocol.CollaborationKnowledgeApproveRequest {
	r := knowledgeApproveRequest(p)
	r.ExpectedVersion = strconv.FormatInt(version, 10)
	return r
}
func knowledgeApprovalMatrixAssertReceipt(t *testing.T, f *collaborationQueueFixture, actor, request string, want protocol.CollaborationKnowledgeReceipt) {
	t.Helper()
	op, err := f.svc.Queries.GetCollaborationOperation(f.ctx, db.GetCollaborationOperationParams{ScopeKind: "issue", ScopeID: f.issue.Issue.ID, ActorUserID: util.MustParseUUID(actor), RequestID: request})
	if err != nil {
		t.Fatal(err)
	}
	var got protocol.CollaborationKnowledgeReceipt
	if err = json.Unmarshal(op.Result, &got); err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(got)
	b, _ := json.Marshal(want)
	if string(a) != string(b) {
		t.Fatal("operation did not retain its exact content-free original receipt")
	}
	if strings.Contains(string(a), "PRIVATE_") || strings.Contains(string(a), util.UUIDToString(f.issue.Issue.ID)) {
		t.Fatal("operation receipt disclosed source identity or material")
	}
}

func TestKnowledgeApprovalMatrixSourceWritesBothCommitOrders(t *testing.T) {
	knowledgeApprovalMatrixGuard(t)
	for _, sourceAction := range []string{"memory_purge", "comment_edit", "comment_delete"} {
		for _, approvalFirst := range []bool{true, false} {
			t.Run(sourceAction+"/"+map[bool]string{true: "approval_first", false: "source_first"}[approvalFirst], func(t *testing.T) {
				f := newCollaborationMemoryFixture(t)
				p := knowledgeServiceProposal(t, f)
				f.fx.Member(t, f.workspace, f.handler, "admin")
				ctx := knowledgeApprovalMatrixContext(t, f.ctx)
				approverCtx := knowledgeApprovalMatrixContext(t, collaborationContext(f.handler))
				var commentID pgtype.UUID
				if sourceAction == "memory_purge" {
					// Public purge protects the current version; the required v1
					// must first be an available, noncurrent historical source.
					if _, err := f.svc.UpdateMemory(ctx, CollaborationMemoryUpdateParams{IssueID: f.issue.Issue.ID, RequestID: uuid.NewString(), ExpectedVersion: 1, Content: "current independent memory v2"}); err != nil {
						t.Fatal(err)
					}
				}
				if sourceAction != "memory_purge" {
					created, err := f.svc.CreateComment(ctx, CollaborationCreateCommentParams{CollaborationQueueRequest: f.request(), CollaborationCommentInput: CollaborationCommentInput{Content: "PRIVATE_MATRIX_COMMENT_SOURCE"}})
					if err != nil {
						t.Fatal(err)
					}
					commentID = created.CommentID
					p.Sources = []protocol.CollaborationKnowledgeSource{{Kind: "comment", IssueID: util.UUIDToString(f.issue.Issue.ID), CommentID: util.UUIDToString(commentID), Version: "1", SHA256: protocol.CollaborationKnowledgeHash("PRIVATE_MATRIX_COMMENT_SOURCE")}}
				}
				if _, err := f.svc.ProposeKnowledge(ctx, f.issue.Issue.ID, p); err != nil {
					t.Fatal(err)
				}
				approval := knowledgeApprovalMatrixRequest(p, 1)
				id := util.MustParseUUID(p.CandidateID)
				approve := func(starter TxStarter) knowledgeApprovalMatrixResult {
					r, err := knowledgeApprovalMatrixService(f, starter).ApproveKnowledge(approverCtx, id, approval)
					return knowledgeApprovalMatrixResult{r, err}
				}
				sourceKey := uuid.NewString()
				change := func(starter TxStarter) knowledgeApprovalMatrixResult {
					svc := knowledgeApprovalMatrixService(f, starter)
					var err error
					if sourceAction == "memory_purge" {
						_, err = svc.PurgeMemoryVersion(ctx, CollaborationMemoryPurgeParams{IssueID: f.issue.Issue.ID, RequestID: sourceKey, Version: 1, Reason: "purge required source"})
					} else {
						r := CollaborationEditCommentParams{IssueID: f.issue.Issue.ID, CommentID: commentID, RequestID: sourceKey, ExpectedCommentRevision: 1}
						if sourceAction == "comment_delete" {
							_, err = svc.DeleteComment(ctx, r)
						} else {
							r.Content = "replacement comment text"
							_, err = svc.EditComment(ctx, r)
						}
					}
					return knowledgeApprovalMatrixResult{err: err}
				}
				first, second := approve, change
				if !approvalFirst {
					first, second = change, approve
				}
				leader, follower := knowledgeApprovalMatrixPair(t, f, first, second, false)
				if leader.err != nil {
					t.Fatal(leader.err)
				}
				approvalResult := leader
				if !approvalFirst {
					approvalResult = follower
				}
				allowed := approvalFirst || sourceAction == "comment_edit"
				if allowed {
					if approvalResult.err != nil || approvalResult.receipt.Code != "knowledge_approved" {
						t.Fatalf("legitimate approval did not win: %+v %v", approvalResult.receipt, approvalResult.err)
					}
				} else if !collaborationErrorCode(approvalResult.err, "knowledge_source_unavailable") {
					t.Fatalf("unavailable original source accepted: %v", approvalResult.err)
				}
				if approvalFirst && follower.err != nil {
					t.Fatal(follower.err)
				}
				row, err := f.svc.Queries.GetKnowledgeCandidate(ctx, id)
				if err != nil {
					t.Fatal(err)
				}
				if allowed && (row.State != "approved" || !row.ApprovedAt.Valid || row.PublishedTargetRevision.Int64 != 1) {
					t.Fatal("later source change erased independent approval history")
				}
				if !allowed && (row.State != "pending" || row.ApprovedAt.Valid) {
					t.Fatal("source-first refusal published candidate")
				}
				wantHints := 0
				if allowed {
					wantHints = 1
				}
				if f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_invalidation WHERE kind='target_hint' AND target_workspace_id=$1`, f.workspace) != wantHints {
					t.Fatal("approval did not produce exactly one direct target hint")
				}
				knowledgeApprovalMatrixAssertReceipt(t, f, f.handler, approval.RequestID, approvalResult.receipt)
				if sourceAction == "comment_edit" {
					var refs []protocol.CollaborationKnowledgeSource
					if err = json.Unmarshal(row.Sources, &refs); err != nil || len(refs) != 1 || refs[0] != p.Sources[0] {
						t.Fatal("comment edit rebased immutable required reference")
					}
					available, err := f.svc.KnowledgeSourceAvailability(ctx, f.svc.Queries, f.issue.Issue.ID, refs)
					if err != nil || !available {
						t.Fatal("exact historical comment ceased to be restorable", err)
					}
				}
			})
		}
	}
}

func TestKnowledgeApprovalMatrixActorDisableBothCommitOrders(t *testing.T) {
	knowledgeApprovalMatrixGuard(t)
	for _, approvalFirst := range []bool{true, false} {
		t.Run(map[bool]string{true: "approval_first", false: "disable_first"}[approvalFirst], func(t *testing.T) {
			a := newAdminCollaborationFixture(t)
			f := a.collaborationQueueFixture
			p := knowledgeServiceProposal(t, f)
			ctx := knowledgeApprovalMatrixContext(t, f.ctx)
			adminCtx := knowledgeApprovalMatrixContext(t, a.adminCtx)
			if _, err := f.svc.ProposeKnowledge(ctx, f.issue.Issue.ID, p); err != nil {
				t.Fatal(err)
			}
			request := knowledgeApprovalMatrixRequest(p, 1)
			id := util.MustParseUUID(p.CandidateID)
			approve := func(starter TxStarter) knowledgeApprovalMatrixResult {
				r, err := knowledgeApprovalMatrixService(f, starter).ApproveKnowledge(ctx, id, request)
				return knowledgeApprovalMatrixResult{r, err}
			}
			params := PlatformAccountChangeParams{OrganizationID: a.org, TargetUserID: util.MustParseUUID(f.creator), IdempotencyKey: pgtype.UUID{Bytes: uuid.New(), Valid: true}, Action: "disable", ExpectedAuthVersion: 1, Reason: "approval authority matrix", Password: platformAdminTestPassword, RequestID: uuid.NewString()}
			f.fx.Cleanup(t, `DELETE FROM admin_operation WHERE target_id=$1`, params.TargetUserID)
			f.fx.Cleanup(t, `DELETE FROM admin_audit_event WHERE target_id=$1`, params.TargetUserID)
			disable := func(starter TxStarter) knowledgeApprovalMatrixResult {
				_, err := NewPlatformAdminService(f.svc.Queries, starter).ChangeAccount(adminCtx, params)
				return knowledgeApprovalMatrixResult{err: err}
			}
			first, second := approve, disable
			if !approvalFirst {
				first, second = disable, approve
			}
			leader, follower := knowledgeApprovalMatrixPair(t, f, first, second, false)
			if leader.err != nil {
				t.Fatal(leader.err)
			}
			if approvalFirst && follower.err != nil || !approvalFirst && !collaborationErrorCode(follower.err, "knowledge_permission_denied") {
				t.Fatalf("actor disable ordering: %v", follower.err)
			}
			row, err := f.svc.Queries.GetKnowledgeCandidate(context.Background(), id)
			if err != nil {
				t.Fatal(err)
			}
			if approvalFirst {
				if row.State != "approved" || !row.ApprovedAt.Valid {
					t.Fatal("disable retroactively erased legitimate approval")
				}
				knowledgeApprovalMatrixAssertReceipt(t, f, f.creator, request.RequestID, leader.receipt)
			} else if row.State != "pending" || row.ApprovedAt.Valid {
				t.Fatal("disabled actor published candidate")
			}
			want := 0
			if approvalFirst {
				want = 1
			}
			if f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_invalidation WHERE kind='target_hint' AND target_workspace_id=$1`, f.workspace) != want {
				t.Fatal("disable race duplicated or fabricated approval hint")
			}
		})
	}
}

func TestKnowledgeApprovalMatrixPendingWithdrawalBothCommitOrders(t *testing.T) {
	knowledgeApprovalMatrixGuard(t)
	for _, approvalFirst := range []bool{true, false} {
		t.Run(map[bool]string{true: "approval_first", false: "withdraw_first"}[approvalFirst], func(t *testing.T) {
			f := newCollaborationMemoryFixture(t)
			p := knowledgeServiceProposal(t, f)
			f.fx.Member(t, f.workspace, f.handler, "admin")
			ctx := knowledgeApprovalMatrixContext(t, f.ctx)
			approverCtx := knowledgeApprovalMatrixContext(t, collaborationContext(f.handler))
			if _, err := f.svc.ProposeKnowledge(ctx, f.issue.Issue.ID, p); err != nil {
				t.Fatal(err)
			}
			id := util.MustParseUUID(p.CandidateID)
			approval := knowledgeApprovalMatrixRequest(p, 1)
			withdrawal := protocol.CollaborationKnowledgeDecisionRequest{ProtocolVersion: 1, RequestID: uuid.NewString(), ExpectedStateRevision: "1", Reason: "withdraw originally pending candidate"}
			approve := func(starter TxStarter) knowledgeApprovalMatrixResult {
				r, err := knowledgeApprovalMatrixService(f, starter).ApproveKnowledge(approverCtx, id, approval)
				return knowledgeApprovalMatrixResult{r, err}
			}
			withdraw := func(starter TxStarter) knowledgeApprovalMatrixResult {
				r, err := knowledgeApprovalMatrixService(f, starter).WithdrawKnowledge(ctx, id, withdrawal)
				return knowledgeApprovalMatrixResult{r, err}
			}
			first, second := approve, withdraw
			if !approvalFirst {
				first, second = withdraw, approve
			}
			leader, follower := knowledgeApprovalMatrixPair(t, f, first, second, false)
			if leader.err != nil || !collaborationErrorCode(follower.err, "knowledge_revision_conflict") {
				t.Fatalf("exact original review CAS: leader=%v follower=%v", leader.err, follower.err)
			}
			row, err := f.svc.Queries.GetKnowledgeCandidate(ctx, id)
			if err != nil {
				t.Fatal(err)
			}
			wantState := "withdrawn"
			wantHints := 0
			if approvalFirst {
				wantState = "approved"
				wantHints = 1
			}
			if row.State != wantState || row.StateRevision != 2 || row.ApprovedAt.Valid != approvalFirst {
				t.Fatalf("lost first committed disposition: %+v", row)
			}
			if f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_invalidation WHERE kind='target_hint' AND target_workspace_id=$1`, f.workspace) != wantHints {
				t.Fatal("stale competing action changed effective collection")
			}
			if approvalFirst {
				knowledgeApprovalMatrixAssertReceipt(t, f, f.creator, withdrawal.RequestID, follower.receipt)
			} else {
				knowledgeApprovalMatrixAssertReceipt(t, f, f.handler, approval.RequestID, follower.receipt)
			}
		})
	}
}

func TestKnowledgeApprovalMatrixCompetingCorrectionsBothCommitOrders(t *testing.T) {
	knowledgeApprovalMatrixGuard(t)
	for _, newerFirst := range []bool{false, true} {
		t.Run(map[bool]string{false: "version2_first", true: "version3_first"}[newerFirst], func(t *testing.T) {
			f := newCollaborationMemoryFixture(t)
			p := knowledgeServiceProposal(t, f)
			f.fx.Member(t, f.workspace, f.handler, "admin")
			ctx := knowledgeApprovalMatrixContext(t, f.ctx)
			otherCtx := knowledgeApprovalMatrixContext(t, collaborationContext(f.handler))
			if _, err := f.svc.ProposeKnowledge(ctx, f.issue.Issue.ID, p); err != nil {
				t.Fatal(err)
			}
			if _, err := f.svc.ApproveKnowledge(ctx, util.MustParseUUID(p.CandidateID), knowledgeApprovalMatrixRequest(p, 1)); err != nil {
				t.Fatal(err)
			}
			v2 := p
			v2.CandidateID = uuid.NewString()
			v2.RequestID = uuid.NewString()
			v2.SupersedesCandidateID = &p.CandidateID
			if _, err := f.svc.ProposeKnowledge(ctx, f.issue.Issue.ID, v2); err != nil {
				t.Fatal(err)
			}
			v3 := v2
			v3.CandidateID = uuid.NewString()
			v3.RequestID = uuid.NewString()
			v3.SupersedesCandidateID = &v2.CandidateID
			if _, err := f.svc.ProposeKnowledge(ctx, f.issue.Issue.ID, v3); err != nil {
				t.Fatal(err)
			}
			r2, r3 := knowledgeApprovalMatrixRequest(v2, 2), knowledgeApprovalMatrixRequest(v3, 3)
			a2 := func(starter TxStarter) knowledgeApprovalMatrixResult {
				r, err := knowledgeApprovalMatrixService(f, starter).ApproveKnowledge(ctx, util.MustParseUUID(v2.CandidateID), r2)
				return knowledgeApprovalMatrixResult{r, err}
			}
			a3 := func(starter TxStarter) knowledgeApprovalMatrixResult {
				r, err := knowledgeApprovalMatrixService(f, starter).ApproveKnowledge(otherCtx, util.MustParseUUID(v3.CandidateID), r3)
				return knowledgeApprovalMatrixResult{r, err}
			}
			first, second := a2, a3
			if newerFirst {
				first, second = a3, a2
			}
			leader, _ := knowledgeApprovalMatrixPair(t, f, first, second, true)
			if leader.err != nil {
				t.Fatal(leader.err)
			}
			follower := second(f.pool)
			if newerFirst {
				if !collaborationErrorCode(follower.err, "knowledge_state_conflict") {
					t.Fatalf("older pending version displaced newer approved one: %v", follower.err)
				}
			} else if follower.err != nil {
				t.Fatalf("same original newer approval did not recover after NOWAIT: %v", follower.err)
			}
			if f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_candidate WHERE lineage_id=$1 AND state='approved' AND version=3`, p.CandidateID) != 1 || f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_candidate WHERE lineage_id=$1 AND state='approved'`, p.CandidateID) != 1 {
				t.Fatal("competing correction publication lost unique newest approval")
			}
			want := 3
			if newerFirst {
				want = 2
			}
			if f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_invalidation WHERE kind='target_hint' AND target_workspace_id=$1`, f.workspace) != want {
				t.Fatal("retry duplicated correction hint or revision")
			}
			var revision int64
			f.fx.QueryRow(t, `SELECT knowledge_revision FROM collaboration_knowledge_target WHERE target_kind='workspace' AND target_id=$1`, f.workspace).Scan(&revision)
			if revision != int64(want) {
				t.Fatalf("collection revision=%d want=%d", revision, want)
			}
		})
	}
}

func TestKnowledgeApprovalMatrixConcurrentCorrectionAllocation(t *testing.T) {
	knowledgeApprovalMatrixGuard(t)
	for _, handlerFirst := range []bool{false, true} {
		t.Run(map[bool]string{false: "creator_first", true: "handler_first"}[handlerFirst], func(t *testing.T) {
			f := newCollaborationMemoryFixture(t)
			root := knowledgeServiceProposal(t, f)
			ctx := knowledgeApprovalMatrixContext(t, f.ctx)
			otherCtx := knowledgeApprovalMatrixContext(t, collaborationContext(f.handler))
			if _, err := f.svc.ProposeKnowledge(ctx, f.issue.Issue.ID, root); err != nil {
				t.Fatal(err)
			}
			firstIntent := root
			firstIntent.CandidateID = uuid.NewString()
			firstIntent.RequestID = uuid.NewString()
			firstIntent.SupersedesCandidateID = &root.CandidateID
			secondIntent := firstIntent
			secondIntent.CandidateID = uuid.NewString()
			secondIntent.RequestID = uuid.NewString()
			creator := func(starter TxStarter) knowledgeApprovalMatrixResult {
				r, err := knowledgeApprovalMatrixService(f, starter).ProposeKnowledge(ctx, f.issue.Issue.ID, firstIntent)
				return knowledgeApprovalMatrixResult{r, err}
			}
			handler := func(starter TxStarter) knowledgeApprovalMatrixResult {
				r, err := knowledgeApprovalMatrixService(f, starter).ProposeKnowledge(otherCtx, f.issue.Issue.ID, secondIntent)
				return knowledgeApprovalMatrixResult{r, err}
			}
			first, second := creator, handler
			if handlerFirst {
				first, second = handler, creator
			}
			leader, follower := knowledgeApprovalMatrixPair(t, f, first, second, false)
			if leader.err != nil || leader.receipt.Version != "2" || !collaborationErrorCode(follower.err, "knowledge_state_conflict") {
				t.Fatalf("same predecessor allocation: winner=%+v %v loser=%v", leader.receipt, leader.err, follower.err)
			}
			if f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_candidate WHERE lineage_id=$1`, root.CandidateID) != 2 {
				t.Fatal("concurrent intents silently rebased or duplicated a version")
			}
			fresh := root
			fresh.CandidateID = uuid.NewString()
			fresh.RequestID = uuid.NewString()
			fresh.SupersedesCandidateID = &leader.receipt.CandidateID
			r, err := f.svc.ProposeKnowledge(ctx, f.issue.Issue.ID, fresh)
			if err != nil || r.Version != "3" {
				t.Fatalf("explicit next predecessor did not allocate contiguous version: %+v %v", r, err)
			}
			if f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_invalidation WHERE target_workspace_id=$1`, f.workspace) != 0 {
				t.Fatal("pending correction allocation signaled target")
			}
		})
	}
}

func TestKnowledgeApprovalMatrixApprovedWithdrawalAndCorrectionBothOrders(t *testing.T) {
	knowledgeApprovalMatrixGuard(t)
	for _, correctionFirst := range []bool{true, false} {
		t.Run(map[bool]string{true: "correction_first", false: "approved_withdrawal_first"}[correctionFirst], func(t *testing.T) {
			f := newCollaborationMemoryFixture(t)
			p := knowledgeServiceProposal(t, f)
			f.fx.Member(t, f.workspace, f.handler, "admin")
			ctx := knowledgeApprovalMatrixContext(t, f.ctx)
			otherCtx := knowledgeApprovalMatrixContext(t, collaborationContext(f.handler))
			if _, err := f.svc.ProposeKnowledge(ctx, f.issue.Issue.ID, p); err != nil {
				t.Fatal(err)
			}
			if _, err := f.svc.ApproveKnowledge(ctx, util.MustParseUUID(p.CandidateID), knowledgeApprovalMatrixRequest(p, 1)); err != nil {
				t.Fatal(err)
			}
			next := p
			next.CandidateID, next.RequestID = uuid.NewString(), uuid.NewString()
			next.SupersedesCandidateID = &p.CandidateID
			if _, err := f.svc.ProposeKnowledge(ctx, f.issue.Issue.ID, next); err != nil {
				t.Fatal(err)
			}
			approveRequest := knowledgeApprovalMatrixRequest(next, 2)
			withdrawRequest := protocol.CollaborationKnowledgeDecisionRequest{ProtocolVersion: 1, RequestID: uuid.NewString(), ExpectedStateRevision: "2", Reason: "withdraw exact originally approved version"}
			approve := func(starter TxStarter) knowledgeApprovalMatrixResult {
				r, err := knowledgeApprovalMatrixService(f, starter).ApproveKnowledge(otherCtx, util.MustParseUUID(next.CandidateID), approveRequest)
				return knowledgeApprovalMatrixResult{r, err}
			}
			withdraw := func(starter TxStarter) knowledgeApprovalMatrixResult {
				r, err := knowledgeApprovalMatrixService(f, starter).WithdrawKnowledge(ctx, util.MustParseUUID(p.CandidateID), withdrawRequest)
				return knowledgeApprovalMatrixResult{r, err}
			}
			first, second := approve, withdraw
			if !correctionFirst {
				first, second = withdraw, approve
			}
			leader, _ := knowledgeApprovalMatrixPair(t, f, first, second, true)
			if leader.err != nil {
				t.Fatal(leader.err)
			}
			follower := second(f.pool)
			if correctionFirst {
				if !collaborationErrorCode(follower.err, "knowledge_revision_conflict") {
					t.Fatalf("old-version withdrawal did not retain its stale observation: %v", follower.err)
				}
			} else if follower.err != nil {
				t.Fatalf("independent reviewed correction was implicitly withdrawn: %v", follower.err)
			}
			if f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_candidate WHERE lineage_id=$1 AND state='approved' AND version=2`, p.CandidateID) != 1 {
				t.Fatal("old-version withdrawal cascaded to the independent approved correction")
			}
			old, err := f.svc.Queries.GetKnowledgeCandidate(ctx, util.MustParseUUID(p.CandidateID))
			if err != nil || !old.ApprovedAt.Valid || old.PublishedTargetRevision.Int64 != 1 {
				t.Fatal("original approval history was erased", err)
			}
			wantState, wantHints := "superseded", 2
			if !correctionFirst {
				wantState, wantHints = "withdrawn", 3
			}
			if old.State != wantState || f.fx.Count(t, `SELECT count(*) FROM collaboration_knowledge_invalidation WHERE kind='target_hint' AND target_workspace_id=$1`, f.workspace) != wantHints {
				t.Fatal("correction/withdrawal state or hint cardinality changed")
			}
		})
	}
}

func TestKnowledgeApprovalMatrixRecoveryRejectsOtherCandidateAndKinds(t *testing.T) {
	knowledgeApprovalMatrixGuard(t)
	f := newCollaborationMemoryFixture(t)
	a := knowledgeServiceProposal(t, f)
	ctx := knowledgeApprovalMatrixContext(t, f.ctx)
	if _, err := f.svc.ProposeKnowledge(ctx, f.issue.Issue.ID, a); err != nil {
		t.Fatal(err)
	}
	b := a
	b.CandidateID = uuid.NewString()
	b.RequestID = uuid.NewString()
	if _, err := f.svc.ProposeKnowledge(ctx, f.issue.Issue.ID, b); err != nil {
		t.Fatal(err)
	}
	memoryKey := uuid.NewString()
	if _, err := f.svc.UpdateMemory(ctx, CollaborationMemoryUpdateParams{IssueID: f.issue.Issue.ID, RequestID: memoryKey, ExpectedVersion: 1, Content: "actual second source memory"}); err != nil {
		t.Fatal(err)
	}
	commandRequest := f.request()
	commandRequest.RequestID = uuid.NewString()
	if _, err := f.svc.SubmitCommand(ctx, CollaborationSubmitCommandParams{CollaborationQueueRequest: commandRequest, TargetID: f.targetID, InputEventID: util.MustParseUUID(uuid.NewString()), ExpectedTargetEpoch: 1, ExpectedConfigVersion: 1, Content: "actual command-key namespace fixture"}); err != nil {
		t.Fatal(err)
	}
	snapshot := func() string {
		var state string
		f.fx.QueryRow(t, `SELECT jsonb_build_object('candidates',(SELECT jsonb_agg(to_jsonb(c) ORDER BY id) FROM collaboration_knowledge_candidate c WHERE source_issue_id=$1),'operations',(SELECT jsonb_agg(to_jsonb(o) ORDER BY id) FROM collaboration_operation o WHERE scope_kind='issue' AND scope_id=$1),'hints',(SELECT jsonb_agg(to_jsonb(h) ORDER BY id) FROM collaboration_knowledge_invalidation h WHERE target_workspace_id=$2))::text`, f.issue.Issue.ID, f.workspace).Scan(&state)
		return state
	}
	before := snapshot()
	for _, tc := range []struct{ name, candidate, request string }{{"different_existing_candidate", b.CandidateID, a.RequestID}, {"memory_kind", a.CandidateID, memoryKey}, {"command_kind", a.CandidateID, commandRequest.RequestID}} {
		t.Run(tc.name, func(t *testing.T) {
			r, err := f.svc.GetKnowledgeOperation(ctx, util.MustParseUUID(tc.candidate), tc.request)
			if !collaborationErrorCode(err, "operation_not_found") || r.CandidateID != "" || r.State != "" || r.Code != "" {
				t.Fatalf("negative recovery returned a false receipt: %+v %v", r, err)
			}
		})
	}
	if snapshot() != before {
		t.Fatal("negative recovery changed candidates/operations/hints")
	}
}

func TestKnowledgeApprovalMatrixDeletedCommentSuppressesAllHumanProjections(t *testing.T) {
	knowledgeApprovalMatrixGuard(t)
	f := newCollaborationMemoryFixture(t)
	p := knowledgeServiceProposal(t, f)
	ctx := knowledgeApprovalMatrixContext(t, f.ctx)
	comment, err := f.svc.CreateComment(ctx, CollaborationCreateCommentParams{CollaborationQueueRequest: f.request(), CollaborationCommentInput: CollaborationCommentInput{Content: "PRIVATE_DELETED_REQUIRED_SOURCE"}})
	if err != nil {
		t.Fatal(err)
	}
	ref := protocol.CollaborationKnowledgeSource{Kind: "comment", IssueID: util.UUIDToString(f.issue.Issue.ID), CommentID: util.UUIDToString(comment.CommentID), Version: "1", SHA256: protocol.CollaborationKnowledgeHash("PRIVATE_DELETED_REQUIRED_SOURCE")}
	p.Sources = []protocol.CollaborationKnowledgeSource{ref}
	p.Evidence = []protocol.CollaborationKnowledgeEvidence{{Kind: "test", Source: &ref, Reference: "PRIVATE_DERIVED_EVIDENCE", Verification: "observed"}}
	if _, err = f.svc.ProposeKnowledge(ctx, f.issue.Issue.ID, p); err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.ApproveKnowledge(ctx, util.MustParseUUID(p.CandidateID), knowledgeApprovalMatrixRequest(p, 1)); err != nil {
		t.Fatal(err)
	}
	second := p
	second.CandidateID = uuid.NewString()
	second.RequestID = uuid.NewString()
	second.SupersedesCandidateID = &p.CandidateID
	if _, err = f.svc.ProposeKnowledge(ctx, f.issue.Issue.ID, second); err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.RejectKnowledge(ctx, util.MustParseUUID(second.CandidateID), protocol.CollaborationKnowledgeDecisionRequest{ProtocolVersion: 1, RequestID: uuid.NewString(), ExpectedStateRevision: "1", Reason: "PRIVATE_DERIVED_REASON"}); err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.DeleteComment(ctx, CollaborationEditCommentParams{IssueID: f.issue.Issue.ID, CommentID: comment.CommentID, RequestID: uuid.NewString(), ExpectedCommentRevision: 1}); err != nil {
		t.Fatal(err)
	}
	list, err := f.svc.ListKnowledgeCandidates(ctx, f.issue.Issue.ID, KnowledgePageQuery{})
	if err != nil {
		t.Fatal(err)
	}
	detail, err := f.svc.KnowledgeCandidate(ctx, util.MustParseUUID(second.CandidateID))
	if err != nil {
		t.Fatal(err)
	}
	target, err := f.svc.ListTargetKnowledge(ctx, p.Target, KnowledgePageQuery{})
	if err != nil {
		t.Fatal(err)
	}
	for name, view := range map[string]any{"source_list": list, "detail_history": detail, "target": target} {
		raw, _ := json.Marshal(view)
		if strings.Contains(string(raw), "PRIVATE_") || strings.Contains(string(raw), `"sources"`) || strings.Contains(string(raw), `"content_sha256"`) {
			t.Fatalf("%s exposed deleted-source derivatives: %s", name, raw)
		}
	}
	if detail.Candidate.Availability != "source_unavailable" || len(detail.History) != 2 || len(list.Items) != 2 || len(target.Items) != 1 {
		t.Fatal("deletion removed audit identity instead of suppressing material")
	}
	row, err := f.svc.Queries.GetKnowledgeCandidate(ctx, util.MustParseUUID(p.CandidateID))
	if err != nil || row.State != "approved" || !row.ApprovedAt.Valid {
		t.Fatal("source deletion cascaded independent approval history", err)
	}
}
