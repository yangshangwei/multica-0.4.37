package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// These are production content/report writers with synthetic PR/CI and managed
// preparation, consent and settlement fixtures. The setup's retired test-only
// ReconfigureTarget is not production Switch, daemon or payment execution.
func TestCollaborationHumanEditRuntimeReplyBothCommitOrders(t *testing.T) {
	if strings.TrimSpace(os.Getenv("DATABASE_URL")) == "" {
		t.Fatal("concurrent content proof requires an explicitly owned DATABASE_URL")
	}
	for _, order := range []string{"human_edit_first", "runtime_reply_first"} {
		t.Run(order, func(t *testing.T) {
			f := newStageRuntimeFixture(t, true)
			f.fx.Cleanup(t, `DELETE FROM comment WHERE issue_id=$1`, f.issue.Issue.ID)
			input := f.input(t)
			input.Content = "Human V1: inspect the frozen candidate. " + input.Content
			created, err := f.svc.CreateComment(f.ctx, input)
			if err != nil || created.Status != http.StatusCreated || created.CommentRevision != 1 || len(created.Targets) != 1 || !created.Targets[0].CommandID.Valid {
				t.Fatalf("source-bound human create: %+v %v", created, err)
			}
			command := created.Targets[0].CommandID
			f.resumeManaged(t)
			list, err := f.svc.ListRuntimeRuns(f.daemonCtx, f.runtime.ID)
			if err != nil || len(list.RunIDs) != 1 {
				t.Fatalf("one admitted runtime: %+v %v", list, err)
			}
			run := util.MustParseUUID(list.RunIDs[0])
			_, claim := claimPreparedCandidate(t, f.managedCollaborationFixture, run)
			_, permit := permitPreparedClaim(t, f.managedCollaborationFixture, run, claim)
			subject, err := f.svc.RuntimeTestSubject(context.Background(), run, permit.BusinessToken)
			if err != nil || subject.IssueID != util.UUIDToString(f.issue.Issue.ID) || subject.RunID != util.UUIDToString(run) ||
				subject.Subject.CandidateID != util.UUIDToString(f.candidate.CandidateID) || subject.Subject.CandidateVersion != stageVersion(f.candidate.Version) ||
				subject.Subject.CandidateDigest != f.candidate.ContentDigest || subject.Subject.SourceRevision != f.stage.head ||
				claim.Preparation.Git == nil || claim.Preparation.Git.CommitOID != f.stage.head ||
				!reflect.DeepEqual(claim.Preparation.CommandIDs, []string{util.UUIDToString(command)}) || len(claim.Inputs) != 1 ||
				claim.Inputs[0].CommandID != util.UUIDToString(command) || claim.Inputs[0].AuthorUserID != f.creator || claim.Inputs[0].Body != input.Content {
				t.Fatalf("admitted input lost its human/source/run identity: %v", err)
			}
			before := tc06ContentFacts(t, f)
			for _, table := range []string{"collaboration_command", "collaboration_group_input", "collaboration_run", "collaboration_attempt", "collaboration_test_subject", "comment"} {
				if len(tc06Rows(t, before[table])) != 1 {
					t.Fatalf("expected one original %s", table)
				}
			}
			for _, table := range []string{"agent_task_queue", "collaboration_delegation", "collaboration_attachment", "collaboration_attachment_reference"} {
				if before[table] != "[]" {
					t.Fatalf("unexpected initial dispatch/output collection %s", table)
				}
			}
			tc06Save(t, order, "before", before)
			edit := CollaborationEditCommentParams{IssueID: f.issue.Issue.ID, CommentID: created.CommentID, RequestID: uuid.NewString(),
				ExpectedCommentRevision: 1, Content: "Human V2: preserve this correction while the admitted run replies."}
			reply := CollaborationRuntimeCommentParams{CommentID: util.MustParseUUID(uuid.NewString()), ParentID: created.CommentID,
				Content: "Agent reply: the admitted V1 input and its frozen candidate remain the source."}
			edited, replied, blocker := tc06ContentCommitOrder(t, f, order, run, permit.BusinessToken, edit, reply)
			if edited.Status != http.StatusOK || edited.Code != "comment_edited" || edited.CommentID != created.CommentID || edited.CommentRevision != 2 ||
				replied.CommentID != reply.CommentID || len(replied.RunIDs) != 0 || replied.ContinuationRunID.Valid || replied.Attachment != nil {
				t.Fatalf("concurrent distinct comments did not commit once: human=%+v runtime=%+v", edited, replied)
			}
			after := tc06ContentFacts(t, f)
			tc06Save(t, order, "after-race", after)
			for table, original := range before {
				switch table {
				case "collaboration_session", "collaboration_operation", "collaboration_event", "collaboration_run_event", "comment":
					// These exact changes are checked below; all other full rows stay equal.
				default:
					if after[table] != original {
						t.Fatalf("content race changed frozen/business collection %s", table)
					}
				}
			}
			oldSession, newSession := tc06Rows(t, before["collaboration_session"])[0], tc06Rows(t, after["collaboration_session"])[0]
			var oldSeq, newSeq int64
			if json.Unmarshal(oldSession["event_seq"], &oldSeq) != nil || json.Unmarshal(newSession["event_seq"], &newSeq) != nil || newSeq != oldSeq+2 {
				t.Fatal("two content writes must advance the original session event sequence exactly twice")
			}
			newSession["event_seq"] = oldSession["event_seq"]
			if !reflect.DeepEqual(oldSession, newSession) {
				t.Fatal("content race changed another session field")
			}
			operations := tc06AddedRows(t, before["collaboration_operation"], after["collaboration_operation"])
			events := tc06AddedRows(t, before["collaboration_event"], after["collaboration_event"])
			runEvents := tc06AddedRows(t, before["collaboration_run_event"], after["collaboration_run_event"])
			if len(operations) != 1 || len(events) != 2 || len(runEvents) != 1 || len(tc06Rows(t, after["comment"])) != 2 {
				t.Fatal("race did not add exactly one edit operation, two timeline events, one runtime event and one reply")
			}
			var operation struct {
				db.CollaborationOperation
				Result json.RawMessage `json:"result"`
			}
			tc06Decode(t, operations[0], &operation)
			if operation.ActorUserID != util.MustParseUUID(f.creator) || operation.RequestID != edit.RequestID || operation.Kind != "edit_comment" ||
				operation.ScopeID != f.issue.Issue.ID || operation.ResultCode.String != "comment_edited" || !operation.CreatedAt.Valid || !operation.CompletedAt.Valid {
				t.Fatal("new edit operation lost its human/original-request identity")
			}
			var storedEdit CollaborationCommentReceipt
			tc06Decode(t, operation.Result, &storedEdit)
			if !tc06SameJSON(t, storedEdit, edited) {
				t.Fatal("edit result differs from its durable original receipt")
			}
			var runtimeEvent struct {
				db.CollaborationRunEvent
				Payload json.RawMessage `json:"payload"`
			}
			tc06Decode(t, runEvents[0], &runtimeEvent)
			var runtimePayload collaborationRuntimeCommentEvent
			tc06Decode(t, runtimeEvent.Payload, &runtimePayload)
			if runtimeEvent.IssueID != f.issue.Issue.ID || runtimeEvent.RunID != run || runtimeEvent.Kind != "comment" ||
				runtimeEvent.ProducerEventID != "comment:"+util.UUIDToString(reply.CommentID) || !runtimeEvent.CreatedAt.Valid ||
				!reflect.DeepEqual(runtimePayload.Input.CollaborationRuntimeCommentParams, reply) || !tc06SameJSON(t, runtimePayload.Receipt, replied) {
				t.Fatal("runtime event lost its exact original reply/run identity")
			}
			type timelineEvent struct {
				db.CollaborationEvent
				Payload json.RawMessage `json:"payload"`
			}
			var humanEvent, replyEvent timelineEvent
			for _, raw := range events {
				var event timelineEvent
				tc06Decode(t, raw, &event)
				switch event.Kind {
				case "comment_edited":
					humanEvent = event
				case "runtime_comment_created":
					replyEvent = event
				default:
					t.Fatalf("unexpected race event %s", event.Kind)
				}
			}
			var historyPayload collaborationCommentHistoryPayload
			tc06Decode(t, humanEvent.Payload, &historyPayload)
			var replyPayload struct {
				CommentID  pgtype.UUID `json:"comment_id"`
				RunID      pgtype.UUID `json:"run_id"`
				GroupID    pgtype.UUID `json:"group_id"`
				AuthorType string      `json:"author_type"`
				AuthorID   pgtype.UUID `json:"author_id"`
			}
			tc06Decode(t, replyEvent.Payload, &replyPayload)
			if humanEvent.ID != edited.EventID || humanEvent.ActorUserID != util.MustParseUUID(f.creator) || humanEvent.IssueID != f.issue.Issue.ID ||
				historyPayload.RequestID != edit.RequestID || historyPayload.CommentID != created.CommentID || historyPayload.Before == nil ||
				*historyPayload.Before != input.Content || historyPayload.After != edit.Content || historyPayload.CommentRevision != 2 ||
				replyEvent.IssueID != f.issue.Issue.ID || replyEvent.ActorUserID.Valid || replyPayload.CommentID != reply.CommentID || replyPayload.RunID != run ||
				replyPayload.GroupID != util.MustParseUUID(claim.Identity.GroupID) || replyPayload.AuthorType != "agent" || replyPayload.AuthorID != f.grant.AgentID {
				t.Fatal("human and runtime timeline events lost their distinct actor/request/source identities")
			}
			firstSeq, secondSeq := humanEvent.EventSeq, replyEvent.EventSeq
			if order == "runtime_reply_first" {
				firstSeq, secondSeq = secondSeq, firstSeq
			}
			if firstSeq != oldSeq+1 || secondSeq != oldSeq+2 {
				t.Fatal("persisted timeline order differs from the observed database commit order")
			}
			oldComment := tc06Rows(t, before["comment"])[0]
			for _, row := range tc06Rows(t, after["comment"]) {
				var id pgtype.UUID
				tc06Decode(t, row["id"], &id)
				if id == created.CommentID {
					for _, field := range []string{"content", "revision", "updated_at"} {
						row[field] = oldComment[field]
					}
					if !reflect.DeepEqual(row, oldComment) {
						t.Fatal("human edit changed original comment identity or another field")
					}
				}
			}
			human, err := f.svc.GetComment(f.ctx, f.issue.Issue.ID, created.CommentID)
			if err != nil || human.Content != edit.Content || human.CommentRevision != 2 || human.AuthorType != "member" || human.Author == nil ||
				human.Author.UserID != util.MustParseUUID(f.creator) || human.AuthorAgent != nil || human.DeletedAt != nil ||
				!human.UpdatedAt.Equal(humanEvent.CreatedAt.Time) {
				t.Fatalf("current human correction was overwritten: %+v %v", human, err)
			}
			agent, err := f.svc.GetComment(f.ctx, f.issue.Issue.ID, reply.CommentID)
			if err != nil || agent.Content != reply.Content || agent.CommentRevision != 1 || agent.ParentID != created.CommentID || agent.AuthorType != "agent" ||
				agent.Author != nil || agent.AuthorAgent == nil || agent.AuthorAgent.AgentID != f.grant.AgentID || agent.DeletedAt != nil ||
				!agent.CreatedAt.Equal(runtimeEvent.CreatedAt.Time) || !agent.CreatedAt.Equal(replyEvent.CreatedAt.Time) {
				t.Fatalf("separate Agent reply lost its identity or parent: %+v %v", agent, err)
			}
			history, err := f.svc.CommentHistory(f.ctx, f.issue.Issue.ID, created.CommentID)
			if err != nil || len(history) != 2 || history[0].EventID != created.EventID || history[0].After != input.Content || history[0].CommentRevision != 1 ||
				history[1].EventID != edited.EventID || history[1].Before == nil || *history[1].Before != input.Content || history[1].After != edit.Content ||
				history[0].Actor == nil || history[0].Actor.UserID != util.MustParseUUID(f.creator) || history[1].Actor == nil ||
				history[1].Actor.UserID != util.MustParseUUID(f.creator) || !history[0].CreatedAt.Equal(human.CreatedAt) || !history[1].CreatedAt.Equal(human.UpdatedAt) {
				t.Fatalf("human V1/V2 history lost exact actor, time or original events: %v", err)
			}
			current, err := f.svc.RuntimeContext(context.Background(), run, permit.BusinessToken)
			if err != nil || current.Identity != claim.Identity || !reflect.DeepEqual(current.Inputs, claim.Inputs) || len(current.Comments) != 2 {
				t.Fatalf("live runtime no longer separates frozen inputs from current discussion: %v", err)
			}
			seenHuman, seenAgent := false, false
			for _, comment := range current.Comments {
				if comment.ID == created.CommentID {
					seenHuman = comment.Content == edit.Content && comment.AuthorType == "member" && comment.AuthorID == util.MustParseUUID(f.creator)
				} else if comment.ID == reply.CommentID {
					seenAgent = comment.Content == reply.Content && comment.AuthorType == "agent" && comment.AuthorID == f.grant.AgentID && comment.ParentID == created.CommentID
				} else {
					t.Fatal("runtime current discussion lost the human correction or separate Agent reply")
				}
			}
			if !seenHuman || !seenAgent {
				t.Fatal("runtime current discussion lost exact human/Agent comment identities")
			}
			currentSubject, err := f.svc.RuntimeTestSubject(context.Background(), run, permit.BusinessToken)
			if err != nil || !reflect.DeepEqual(currentSubject, subject) {
				t.Fatalf("comment edit changed the admitted frozen test subject: %v", err)
			}

			// Human receipts retain the original revision; runtime reply recovery
			// requires a live capability. Neither replay returns the current prose.
			for _, original := range []struct {
				key  string
				want CollaborationCommentReceipt
				call func() (CollaborationCommentReceipt, error)
			}{{input.RequestID, created, func() (CollaborationCommentReceipt, error) { return f.svc.CreateComment(f.ctx, input) }},
				{edit.RequestID, edited, func() (CollaborationCommentReceipt, error) { return f.svc.EditComment(f.ctx, edit) }}} {
				replayed, writeErr := original.call()
				own, readErr := f.svc.GetCommentOperation(f.ctx, f.issue.Issue.ID, original.key)
				if writeErr != nil || readErr != nil || !tc06SameJSON(t, replayed, original.want) || !tc06SameJSON(t, own, original.want) {
					t.Fatalf("original human content receipt changed: write=%v read=%v", writeErr, readErr)
				}
			}
			replayedReply, err := f.svc.CreateRuntimeComment(context.Background(), run, permit.BusinessToken, reply)
			if err != nil || !tc06SameJSON(t, replayedReply, replied) {
				t.Fatalf("live original runtime reply changed: %v", err)
			}
			changedEdit := edit
			changedEdit.Content = "An old key must not replace the human correction."
			var humanConflict *CollaborationError
			if _, err = f.svc.EditComment(f.ctx, changedEdit); !errors.As(err, &humanConflict) || humanConflict.Status != http.StatusConflict || humanConflict.Code != "idempotency_conflict" {
				t.Fatalf("changed original human edit: %v", err)
			}
			changedReply := reply
			changedReply.Content = "An old runtime UUID must not replace the reply."
			var runtimeConflict *CollaborationError
			if _, err = f.svc.CreateRuntimeComment(context.Background(), run, permit.BusinessToken, changedReply); !errors.As(err, &runtimeConflict) || runtimeConflict.Status != http.StatusConflict || runtimeConflict.Code != "comment_conflict" {
				t.Fatalf("changed original runtime reply: %v", err)
			}
			afterReplay := tc06ContentFacts(t, f)
			if !reflect.DeepEqual(after, afterReplay) {
				t.Fatal("original content recovery or conflicting old payload changed business state")
			}
			tc06Save(t, order, "after-live-recovery", afterReplay)
			tc06Save(t, order, "identity-and-race", map[string]any{"order": order, "observed_blocker_pid": blocker,
				"input_request": input, "create_receipt": created, "edit_request": edit, "edit_receipt": edited, "runtime_reply_request": reply, "runtime_reply_receipt": replied,
				"claim_identity": claim.Identity, "claim_inputs": claim.Inputs, "subject": subject, "human_comment": human, "agent_comment": agent,
				"human_history": history, "runtime_context": current, "human_event": humanEvent, "agent_event": replyEvent, "runtime_event": runtimeEvent,
				"changed_human_request_refusal": humanConflict, "changed_runtime_reply_refusal": runtimeConflict})

			report := stageRuntimeReport(subject.Subject, claim.Identity)
			report.Report.SuiteID = "synthetic-concurrent-content-source"
			report.Report.Cases[0].ID, report.Report.Cases[0].Outcome = "source-still-frozen", "passed"
			report.Report.Logs[0].Body = "Synthetic report: the admitted V1 source is retained alongside current human V2 and the distinct Agent reply."
			ack, err := f.svc.RecordRuntimeTestReport(context.Background(), run, permit.BusinessToken, report)
			if err != nil || !ack.Accepted || ack.IssueID != subject.IssueID || ack.RunID != subject.RunID || ack.RequestID != report.RequestID {
				t.Fatalf("source-bound report failed: %+v %v", ack, err)
			}
			finishSquadRun(t, f.managedCollaborationFixture, run, claim, permit.BusinessToken, permit.AttemptNonce, "success")
			detail, err := f.svc.TestReport(f.ctx, f.issue.Issue.ID, run, util.MustParseUUID(ack.ReportID))
			if err != nil || detail.ReportID != ack.ReportID || detail.ReportDigest != ack.ReportDigest || detail.IssueID != subject.IssueID || detail.RunID != subject.RunID ||
				detail.AgentID != util.UUIDToString(f.grant.AgentID) || detail.Attempt != "1" || detail.SourceKind != "runtime_reported" || detail.ReportedResult != "passed" || detail.Applicability != "usable" ||
				!reflect.DeepEqual(detail.Subject, subject.Subject) || !reflect.DeepEqual(detail.Cases, report.Report.Cases) || len(detail.Logs) != 1 || detail.Logs[0].Body != report.Report.Logs[0].Body {
				t.Fatalf("report no longer joins the original candidate/run/source: %+v %v", detail, err)
			}
			settled := tc06ContentFacts(t, f)
			for _, table := range []string{"comment", "collaboration_test_subject", "collaboration_stage_candidate", "collaboration_operation", "collaboration_group_input", "collaboration_attempt"} {
				if after[table] != settled[table] {
					t.Fatalf("report/settlement changed retained content/source collection %s", table)
				}
			}
			tc06AddedRows(t, after["collaboration_event"], settled["collaboration_event"])
			tc06AddedRows(t, after["collaboration_run_event"], settled["collaboration_run_event"])
			if f.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE session_id=$1`, f.issue.SessionID) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE session_id=$1`, f.issue.SessionID) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_run_event WHERE run_id=$1 AND kind='test_report'`, run) != 1 {
				t.Fatal("content/report path dispatched another run or duplicated its report")
			}
			storedRun, err := f.svc.Queries.GetCollaborationRuntimeRun(f.ctx, run)
			var frozenInputs []protocol.CollaborationRunInput
			if err != nil || !storedRun.SettledAt.Valid || storedRun.BusinessOutcome.String != "success" ||
				json.Unmarshal(storedRun.PromptSnapshot, &frozenInputs) != nil || !reflect.DeepEqual(frozenInputs, claim.Inputs) {
				t.Fatalf("settlement lost its original frozen input: %v", err)
			}
			tc06Save(t, order, "after-settlement", settled)
			reportReplay, replayErr := f.svc.RecordRuntimeTestReport(context.Background(), run, permit.BusinessToken, report)
			reportReceipt, readErr := f.svc.RuntimeTestReportReceipt(context.Background(), run, permit.BusinessToken, report.RequestID)
			if replayErr != nil || readErr != nil || !reflect.DeepEqual(reportReplay, ack) || !reflect.DeepEqual(reportReceipt, ack) {
				t.Fatalf("settled original report ACK changed: replay=%v read=%v", replayErr, readErr)
			}
			for _, original := range []struct {
				key     string
				receipt CollaborationCommentReceipt
			}{{input.RequestID, created}, {edit.RequestID, edited}} {
				own, err := f.svc.GetCommentOperation(f.ctx, f.issue.Issue.ID, original.key)
				if err != nil || !tc06SameJSON(t, own, original.receipt) {
					t.Fatalf("settlement rewrote the human original receipt: %v", err)
				}
			}
			final := tc06ContentFacts(t, f)
			if !reflect.DeepEqual(settled, final) {
				t.Fatal("post-settlement original recovery changed retained business history")
			}
			tc06Save(t, order, "after-settled-recovery", final)
			tc06Save(t, order, "report", map[string]any{"request": report, "ack": ack, "replayed_ack": reportReplay, "receipt": reportReceipt, "detail": detail})
		})
	}
}

func tc06ContentCommitOrder(t *testing.T, f *stageRuntimeFixture, order string, run pgtype.UUID, token string, edit CollaborationEditCommentParams, reply CollaborationRuntimeCommentParams) (CollaborationCommentReceipt, CollaborationRuntimeCommentReceipt, int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(f.ctx, 15*time.Second)
	release := make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	var writers sync.WaitGroup
	defer func() { unblock(); cancel(); writers.Wait() }()
	barrier := runtimeSourceCommitStarter{recipientCommitStarter{TxStarter: f.pool, reached: make(chan int, 1), release: release}}
	service := *f.svc
	service.TxStarter = barrier
	var human CollaborationCommentReceipt
	var runtime CollaborationRuntimeCommentReceipt
	editCall := func(s *CollaborationService) error {
		var err error
		human, err = s.EditComment(ctx, edit)
		return err
	}
	replyCall := func(s *CollaborationService) error {
		var err error
		runtime, err = s.CreateRuntimeComment(ctx, run, token, reply)
		return err
	}
	leading, following := editCall, replyCall
	if order == "runtime_reply_first" {
		leading, following = replyCall, editCall
	}
	firstDone := make(chan error, 1)
	writers.Add(1)
	go func() { defer writers.Done(); firstDone <- leading(&service) }()
	var blocker int
	select {
	case blocker = <-barrier.reached:
	case err := <-firstDone:
		t.Fatalf("leading content writer failed before source commit: %v", err)
	case <-ctx.Done():
		t.Fatal("leading content writer did not reach its source commit barrier")
	}
	secondDone := make(chan error, 1)
	writers.Add(1)
	go func() { defer writers.Done(); secondDone <- following(f.svc) }()
	waitFIFOBlocked(t, f.collaborationQueueFixture, blocker, secondDone)
	unblock()
	firstErr, secondErr := <-firstDone, <-secondDone
	writers.Wait()
	if firstErr != nil || secondErr != nil {
		t.Fatalf("both distinct content writers must succeed: first=%v second=%v", firstErr, secondErr)
	}
	return human, runtime, blocker
}

// Use complete rows for equality in memory, including credential metadata. The
// evidence writer below fingerprints sensitive columns rather than exporting them.
func tc06ContentFacts(t *testing.T, f *stageRuntimeFixture) map[string]string {
	t.Helper()
	out := tc13QAFacts(t, f)
	sessions := "SELECT id FROM collaboration_session WHERE issue_id=$1"
	runs := "SELECT id FROM collaboration_run WHERE session_id IN (" + sessions + ")"
	groups := "SELECT id FROM collaboration_execution_group WHERE session_id IN (" + sessions + ")"
	for table, predicate := range map[string]string{
		"issue_participant": "issue_id=$1", "collaboration_grant": "issue_id=$1",
		"collaboration_target": "session_id IN (" + sessions + ")", "collaboration_channel": "session_id IN (" + sessions + ")",
		"collaboration_batch": "session_id IN (" + sessions + ")", "collaboration_group_input": "group_id IN (" + groups + ")",
		"collaboration_group_dependency": "group_id IN (" + groups + ")", "collaboration_delegation": "group_id IN (" + groups + ")",
		"collaboration_attempt": "run_id IN (" + runs + ")", "collaboration_termination": "run_id IN (" + runs + ")",
		"collaboration_business_capability": "run_id IN (" + runs + ")", "collaboration_runtime_reservation": "run_id IN (" + runs + ")",
		"collaboration_attachment": "issue_id=$1", "collaboration_attachment_reference": "attachment_id IN (SELECT id FROM collaboration_attachment WHERE issue_id=$1)",
	} {
		var rows string
		f.fx.QueryRow(t, `SELECT COALESCE(jsonb_agg(to_jsonb(x) ORDER BY to_jsonb(x)::text),'[]'::jsonb)::text FROM `+table+` x WHERE `+predicate, f.issue.Issue.ID).Scan(&rows)
		out[table] = rows
	}
	return out
}

func tc06Rows(t *testing.T, raw string) []map[string]json.RawMessage {
	t.Helper()
	var out []map[string]json.RawMessage
	tc06Decode(t, []byte(raw), &out)
	return out
}

func tc06Decode(t *testing.T, raw []byte, out any) {
	t.Helper()
	if err := json.Unmarshal(raw, out); err != nil {
		t.Fatal(err)
	}
}

func tc06SameJSON(t *testing.T, a, b any) bool {
	t.Helper()
	left, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	right, err := json.Marshal(b)
	if err != nil {
		t.Fatal(err)
	}
	return bytes.Equal(left, right)
}

func tc06AddedRows(t *testing.T, before, after string) []json.RawMessage {
	t.Helper()
	remaining := make(map[string]int)
	for _, row := range tc06Rows(t, before) {
		raw, err := json.Marshal(row)
		if err != nil {
			t.Fatal(err)
		}
		remaining[string(raw)]++
	}
	var added []json.RawMessage
	for _, row := range tc06Rows(t, after) {
		raw, err := json.Marshal(row)
		if err != nil {
			t.Fatal(err)
		}
		if remaining[string(raw)] > 0 {
			remaining[string(raw)]--
		} else {
			added = append(added, raw)
		}
	}
	for _, count := range remaining {
		if count != 0 {
			t.Fatal("append-only comparison lost or rewrote an original complete row")
		}
	}
	return added
}

func tc06Save(t *testing.T, order, label string, value any) {
	t.Helper()
	root := os.Getenv("PAY241_TC06_EVIDENCE_DIR")
	if root == "" {
		return
	}
	if !filepath.IsAbs(root) {
		t.Fatal("TC06 evidence directory must be an explicit absolute path")
	}
	if facts, ok := value.(map[string]string); ok {
		safe := make(map[string]any, len(facts))
		for table, raw := range facts {
			rows := tc06Rows(t, raw)
			for _, row := range rows {
				for _, field := range []string{"token_hash", "token_ciphertext", "encrypted_key", "encryption_key", "object_key"} {
					if secret, found := row[field]; found {
						digest := sha256.Sum256(secret)
						row[field+"_sha256"], _ = json.Marshal(hex.EncodeToString(digest[:]))
						delete(row, field)
					}
				}
			}
			digest := sha256.Sum256([]byte(raw))
			safe[table] = map[string]any{"full_rows_sha256": hex.EncodeToString(digest[:]), "rows": rows}
		}
		value = safe
	}
	dir := filepath.Join(root, order)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(filepath.Join(dir, label+".json"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	_, writeErr := file.Write(append(raw, '\n'))
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		t.Fatalf("retain TC06 evidence: write=%v close=%v", writeErr, closeErr)
	}
}
