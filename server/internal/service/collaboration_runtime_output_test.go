package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/storage"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type runtimeOutputFixture struct {
	*managedCollaborationFixture
	dir    string
	run    pgtype.UUID
	claim  protocol.CollaborationRunClaimResponse
	permit protocol.CollaborationRunPermitResponse
}

func configureRuntimeOutputStorage(t *testing.T, f *managedCollaborationFixture) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("LOCAL_UPLOAD_DIR", dir)
	t.Setenv("LOCAL_UPLOAD_BASE_URL", "")
	f.svc.Storage = storage.NewLocalStorageFromEnv()
	f.fx.Cleanup(t, `DELETE FROM collaboration_attachment WHERE source_workspace_id=$1`, f.workspace)
	f.fx.Cleanup(t, `DELETE FROM collaboration_attachment_reference WHERE attachment_id IN (SELECT id FROM collaboration_attachment WHERE source_workspace_id=$1)`, f.workspace)
	f.fx.Cleanup(t, `DELETE FROM comment WHERE issue_id=$1`, f.issue.Issue.ID)
	return dir
}

func newRuntimeOutputFixture(t *testing.T) *runtimeOutputFixture {
	t.Helper()
	f := newManagedCollaborationFixture(t)
	dir := configureRuntimeOutputStorage(t, f)
	f.submitManaged(t, f.creator, "produce a private output file")
	f.resumeManaged(t)
	run, _, claim := f.claimManaged(t)
	permit, err := f.svc.PermitRuntimeRun(f.daemonCtx, run, protocol.CollaborationRunPermitRequest{Identity: claim.Identity, AttemptSeq: "1", AttemptNonce: uuid.NewString(), Containment: protocol.CollaborationContainmentLinuxPIDNamespace})
	if err != nil {
		t.Fatal(err)
	}
	return &runtimeOutputFixture{managedCollaborationFixture: f, dir: dir, run: run, claim: claim, permit: permit}
}

func runtimeOutputInput() CollaborationRuntimeCommentAttachmentParams {
	return CollaborationRuntimeCommentAttachmentParams{CollaborationRuntimeCommentParams: CollaborationRuntimeCommentParams{CommentID: util.MustParseUUID(uuid.NewString()), Content: "Original Agent output"}, Name: "result.txt", Data: []byte("private original output sentinel\n")}
}

func (f *runtimeOutputFixture) write(p CollaborationRuntimeCommentAttachmentParams) (CollaborationRuntimeCommentReceipt, error) {
	return f.svc.CreateRuntimeCommentAttachment(context.Background(), f.run, f.permit.BusinessToken, p)
}

func (f *runtimeOutputFixture) outcome() protocol.CollaborationRunOutcome {
	return protocol.CollaborationRunOutcome{Identity: f.claim.Identity, EventID: uuid.NewString(), AttemptSeq: f.permit.AttemptSeq, AttemptNonce: f.permit.AttemptNonce, Outcome: "success", Body: "Original final answer"}
}

func runtimeOutputObjectNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(dir, storage.CollaborationPrivatePrefix))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

func assertRuntimeOutputCount(t *testing.T, f *runtimeOutputFixture, comments, files int) {
	t.Helper()
	if got := f.fx.Count(t, `SELECT count(*) FROM comment WHERE issue_id=$1 AND author_type='agent'`, f.issue.Issue.ID); got != comments {
		t.Fatalf("Agent comments=%d, want %d", got, comments)
	}
	if got := f.fx.Count(t, `SELECT count(*) FROM collaboration_attachment WHERE run_id=$1`, f.run); got != files {
		t.Fatalf("run output rows=%d, want %d", got, files)
	}
	if got := f.fx.Count(t, `SELECT count(*) FROM collaboration_attachment_reference WHERE attachment_id IN (SELECT id FROM collaboration_attachment WHERE run_id=$1)`, f.run); got != files*2 {
		t.Fatalf("permanent issue/comment refs=%d, want %d", got, files*2)
	}
	if got := f.fx.Count(t, `SELECT count(*) FROM collaboration_run_event WHERE run_id=$1 AND kind='comment'`, f.run); got != comments {
		t.Fatalf("original comment events=%d, want %d", got, comments)
	}
}

func TestCollaborationRuntimeOutputO01AgentIdentityCiphertextAndReferences(t *testing.T) {
	f := newRuntimeOutputFixture(t)
	p := runtimeOutputInput()
	p.Content += " [@human](mention://member/" + f.handler + ")"
	r, err := f.write(p)
	if err != nil || r.Attachment == nil || r.CommentID != p.CommentID {
		t.Fatalf("combined Agent output receipt: %+v %v", r, err)
	}
	a := *r.Attachment
	if a.UploaderUserID.Valid || a.DraftID.Valid || a.IntendedIssueID.Valid || a.UploaderAgentID != f.grant.AgentID || a.RunID != f.run || a.AttemptSeq == nil || *a.AttemptSeq != "1" || a.IssueID != f.issue.Issue.ID || a.SourceWorkspaceID != util.MustParseUUID(f.workspace) || a.DeliveredAt == nil || a.Name != p.Name || a.ContentType != http.DetectContentType(p.Data) || a.SizeBytes != int64(len(p.Data)) {
		t.Fatalf("output was assigned human or unbound file identity: %+v", a)
	}
	comment, err := f.svc.Queries.GetCollaborationComment(f.ctx, db.GetCollaborationCommentParams{IssueID: f.issue.Issue.ID, CommentID: p.CommentID, WorkspaceID: util.MustParseUUID(f.workspace)})
	if err != nil || comment.AuthorType != "agent" || comment.AuthorID != f.grant.AgentID || comment.Content != p.Content {
		t.Fatalf("actual comment lost admitted Agent identity: %+v %v", comment, err)
	}
	row, err := f.svc.GetAttachment(f.ctx, f.issue.Issue.ID, pgtype.UUID{}, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := os.ReadFile(filepath.Join(f.dir, row.StorageKey))
	if err != nil || bytes.Contains(sealed, p.Data) || len(sealed) != len(p.Data)+28 {
		t.Fatalf("output plaintext escaped encrypted storage: %v", err)
	}
	data, err := f.svc.ReadAttachmentBytes(f.ctx, row)
	if err != nil || !bytes.Equal(data, p.Data) {
		t.Fatalf("delivered output could not be decrypted: %v", err)
	}
	if own, err := f.svc.RuntimeAttachment(context.Background(), f.run, a.ID, a.Version, f.permit.BusinessToken); err != nil || own.ID != a.ID {
		t.Fatalf("original run cannot read its own output: %v", err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_notification WHERE issue_id=$1 AND source_id=$2 AND recipient_user_id=$3`, f.issue.Issue.ID, p.CommentID, f.handler) != 1 {
		t.Fatal("combined writer did not retain its original human mention notification")
	}
	assertRuntimeOutputCount(t, f, 1, 1)
}

func TestCollaborationRuntimeOutputO02UnknownCommitOriginalRecovery(t *testing.T) {
	f := newRuntimeOutputFixture(t)
	p := runtimeOutputInput()
	fault := &runtimeScanFaultStarter{TxStarter: f.pool, commitQuery: "CreateCollaborationRuntimeOutputAttachment", commitError: &pgconn.PgError{Code: "40001", Message: "lost output commit reply"}}
	f.svc.TxStarter = fault
	_, err := f.write(p)
	var commit *collaborationRuntimeCommitError
	if !errors.As(err, &commit) || fault.lostCommits != 1 {
		t.Fatalf("output commit was not retained as unknown: %v", err)
	}
	assertRuntimeOutputCount(t, f, 1, 1)
	recovered, err := f.write(p)
	if err != nil || recovered.Attachment == nil || len(runtimeOutputObjectNames(t, f.dir)) != 1 {
		t.Fatalf("original comment UUID did not recover its committed file: %+v %v", recovered, err)
	}
	row, err := f.svc.RuntimeAttachment(context.Background(), f.run, recovered.Attachment.ID, 1, f.permit.BusinessToken)
	if err != nil {
		t.Fatal(err)
	}
	if data, err := f.svc.ReadAttachmentBytes(f.ctx, row); err != nil || !bytes.Equal(data, p.Data) {
		t.Fatalf("unknown commit recovery erased the winning object: %v", err)
	}
	assertRuntimeOutputCount(t, f, 1, 1)
}

func TestCollaborationRuntimeOutputO03SameIdentityConcurrentConvergence(t *testing.T) {
	f := newRuntimeOutputFixture(t)
	p := runtimeOutputInput()
	arrived, release := make(chan struct{}, 2), make(chan struct{})
	var releaseOnce sync.Once
	finish := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(finish)
	f.svc.Storage = collaborationUploadHookStorage{Storage: f.svc.Storage, uploaded: func() { arrived <- struct{}{}; <-release }}
	type result struct {
		receipt CollaborationRuntimeCommentReceipt
		err     error
	}
	done := make(chan result, 2)
	for range 2 {
		go func() { r, err := f.write(p); done <- result{r, err} }()
	}
	for range 2 {
		select {
		case <-arrived:
		case <-time.After(10 * time.Second):
			finish()
			for range 2 {
				<-done
			}
			t.Fatal("both original uploads did not reach the external-storage barrier")
		}
	}
	finish()
	a, b := <-done, <-done
	if a.err != nil || b.err != nil || !reflect.DeepEqual(a.receipt, b.receipt) || a.receipt.Attachment == nil {
		t.Fatalf("same UUID/bytes did not converge on one original receipt: %+v %+v", a, b)
	}
	if got := runtimeOutputObjectNames(t, f.dir); len(got) != 1 || got[0] != util.UUIDToString(a.receipt.Attachment.ID) {
		t.Fatalf("loser deleted winner or retained a duplicate object: %v", got)
	}
	assertRuntimeOutputCount(t, f, 1, 1)
}

func TestCollaborationRuntimeOutputO04ChangedInputAndJSONAliasConflict(t *testing.T) {
	for _, kind := range []string{"bytes", "name", "content", "parent", "json-first", "multipart-first"} {
		t.Run(kind, func(t *testing.T) {
			f := newRuntimeOutputFixture(t)
			p := runtimeOutputInput()
			if kind == "json-first" {
				if _, err := f.svc.CreateRuntimeComment(context.Background(), f.run, f.permit.BusinessToken, p.CollaborationRuntimeCommentParams); err != nil {
					t.Fatal(err)
				}
				if _, err := f.write(p); !collaborationErrorCode(err, "comment_conflict") {
					t.Fatalf("multipart aliased an old no-file event: %v", err)
				}
				assertRuntimeOutputCount(t, f, 1, 0)
				if len(runtimeOutputObjectNames(t, f.dir)) != 0 {
					t.Fatal("known JSON alias rejection uploaded an orphan")
				}
				return
			}
			winner, err := f.write(p)
			if err != nil {
				t.Fatal(err)
			}
			changed := p
			switch kind {
			case "bytes":
				changed.Data = []byte("changed output")
			case "name":
				changed.Name = "renamed.txt"
			case "content":
				changed.Content = "changed comment"
			case "parent":
				changed.ParentID = util.MustParseUUID(uuid.NewString())
			case "multipart-first":
				if _, err = f.svc.CreateRuntimeComment(context.Background(), f.run, f.permit.BusinessToken, p.CollaborationRuntimeCommentParams); !collaborationErrorCode(err, "comment_conflict") {
					t.Fatalf("JSON aliased the output event: %v", err)
				}
			}
			if kind != "multipart-first" {
				if _, err = f.write(changed); !collaborationErrorCode(err, "comment_conflict") {
					t.Fatalf("changed immutable %s was accepted: %v", kind, err)
				}
			}
			replay, err := f.write(p)
			if err != nil || !reflect.DeepEqual(winner, replay) || len(runtimeOutputObjectNames(t, f.dir)) != 1 {
				t.Fatalf("conflict modified the original winning file/receipt: %v", err)
			}
			assertRuntimeOutputCount(t, f, 1, 1)
		})
	}
}

type runtimeOutputWriteFault struct {
	TxStarter
	query string
	hits  atomic.Int32
	retry bool
}

type runtimeOutputWriteFaultTx struct {
	pgx.Tx
	owner *runtimeOutputWriteFault
}

func (s *runtimeOutputWriteFault) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &runtimeOutputWriteFaultTx{Tx: tx, owner: s}, nil
}

func (t *runtimeOutputWriteFaultTx) fail(sql string) error {
	if strings.Contains(sql, "-- name: "+t.owner.query+" ") {
		t.owner.hits.Add(1)
		if t.owner.retry {
			return &pgconn.PgError{Code: "40001", Message: "rolled back output contention"}
		}
		return errors.New("required output write failed")
	}
	return nil
}

func (t *runtimeOutputWriteFaultTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if err := t.fail(sql); err != nil {
		return queueFailureRow{err: err}
	}
	return t.Tx.QueryRow(ctx, sql, args...)
}

func (t *runtimeOutputWriteFaultTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if err := t.fail(sql); err != nil {
		return pgconn.CommandTag{}, err
	}
	return t.Tx.Exec(ctx, sql, args...)
}

func TestCollaborationRuntimeOutputO05RequiredWritesRollbackAndOwnObjectCleanup(t *testing.T) {
	for _, query := range []string{"CreateCollaborationRuntimeOutputAttachment", "CreateCollaborationAttachmentReference", "CreateCollaborationRuntimeComment", "CreateCollaborationBusinessEvent", "CreateCollaborationEvent", "CreateCollaborationContentNotification", "retry-after-insert"} {
		t.Run(query, func(t *testing.T) {
			f := newRuntimeOutputFixture(t)
			p := runtimeOutputInput()
			p.Content += " [@human](mention://member/" + f.handler + ")"
			fault := &runtimeOutputWriteFault{TxStarter: f.pool, query: query}
			if query == "retry-after-insert" {
				fault.query, fault.retry = "CreateCollaborationEvent", true
			}
			f.svc.TxStarter = fault
			if _, err := f.write(p); err == nil || fault.hits.Load() == 0 {
				t.Fatalf("required write fault did not reach and roll back %s: hits=%d err=%v", query, fault.hits.Load(), err)
			}
			assertRuntimeOutputCount(t, f, 0, 0)
			if len(runtimeOutputObjectNames(t, f.dir)) != 0 || f.fx.Count(t, `SELECT count(*) FROM collaboration_notification WHERE issue_id=$1 AND source_id=$2`, f.issue.Issue.ID, p.CommentID) != 0 {
				t.Fatal("definite rollback retained its object or notification")
			}
		})
	}
}

func TestCollaborationRuntimeOutputO06SlowStorageRevocationAndCurrentReads(t *testing.T) {
	for _, kind := range []string{"source", "participant", "grant"} {
		t.Run(kind, func(t *testing.T) {
			f := newRuntimeOutputFixture(t)
			first, err := f.write(runtimeOutputInput())
			if err != nil {
				t.Fatal(err)
			}
			p := runtimeOutputInput()
			f.svc.Storage = collaborationUploadHookStorage{Storage: f.svc.Storage, uploaded: func() {
				switch kind {
				case "source":
					f.fx.Exec(t, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, f.workspace, f.creator)
				case "participant":
					f.fx.Exec(t, `UPDATE issue_participant SET revoked_at=now(),epoch=epoch+1 WHERE issue_id=$1 AND user_id=$2`, f.issue.Issue.ID, f.creator)
				case "grant":
					f.fx.Exec(t, `UPDATE collaboration_grant SET revoked_at=now(),epoch=epoch+1 WHERE id=$1`, f.grant.ID)
				}
			}}
			if _, err = f.write(p); err == nil {
				t.Fatal("slow storage committed after current authority was withdrawn")
			}
			if _, err = f.svc.RuntimeAttachment(context.Background(), f.run, first.Attachment.ID, 1, f.permit.BusinessToken); err == nil {
				t.Fatal("original runtime credential retained output reads after withdrawal")
			}
			if len(runtimeOutputObjectNames(t, f.dir)) != 1 {
				t.Fatal("failed later upload deleted original output or retained its own orphan")
			}
			assertRuntimeOutputCount(t, f, 1, 1)
		})
	}
}

func TestCollaborationRuntimeOutputO06UnfrozenAndMissingFilesHaveSameRefusal(t *testing.T) {
	f := newRuntimeOutputFixture(t)
	p := attachmentUploadParams(f.collaborationFixture)
	p.SourceWorkspaceID, p.IssueID = pgtype.UUID{}, f.issue.Issue.ID
	human, err := f.svc.UploadAttachmentDraft(f.ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.DeliverAttachments(f.ctx, CollaborationAttachmentDelivery{IssueID: f.issue.Issue.ID, DraftID: &p.DraftID, AttachmentIDs: []pgtype.UUID{human.ID}, RequestID: uuid.NewString()}); err != nil {
		t.Fatal(err)
	}
	for _, id := range []pgtype.UUID{human.ID, util.MustParseUUID(uuid.NewString())} {
		if _, err = f.svc.RuntimeAttachment(context.Background(), f.run, id, 1, f.permit.BusinessToken); !collaborationErrorCode(err, "attachment_not_found") {
			t.Fatalf("unfrozen/missing file identities had different disclosure: %s %v", util.UUIDToString(id), err)
		}
	}
	if output, err := f.write(runtimeOutputInput()); err != nil || output.Attachment == nil {
		t.Fatalf("no-preparation input refusal blocked actual own output: %v", err)
	}
}

func TestCollaborationRuntimeOutputO07BoundOutcomeAndLateWrites(t *testing.T) {
	f := newRuntimeOutputFixture(t)
	originalInput := runtimeOutputInput()
	original, err := f.write(originalInput)
	if err != nil {
		t.Fatal(err)
	}
	for range 31 {
		if _, err := f.write(runtimeOutputInput()); err != nil {
			t.Fatal(err)
		}
	}
	t.Run("full-bound-original-replay", func(t *testing.T) {
		replayed, err := f.write(originalInput)
		if err != nil || !reflect.DeepEqual(original, replayed) || len(runtimeOutputObjectNames(t, f.dir)) != 32 {
			t.Fatalf("original key consumed output capacity or changed its winning receipt: %+v %v", replayed, err)
		}
		assertRuntimeOutputCount(t, f, 32, 32)
		if f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1 AND kind='runtime_comment_created'`, f.issue.Issue.ID) != 32 {
			t.Fatal("full-bound original replay duplicated its public comment event")
		}
	})
	if _, err := f.write(runtimeOutputInput()); !collaborationErrorCode(err, "attachment_limit") || len(runtimeOutputObjectNames(t, f.dir)) != 32 {
		t.Fatalf("run output bound was not exact or left the refused object: %v", err)
	}
	p := f.outcome()
	if err := f.svc.RecordRuntimeOutcome(context.Background(), f.run, f.permit.BusinessToken, p); err != nil {
		t.Fatal(err)
	}
	e, err := f.svc.Queries.GetCollaborationRuntimeEvent(f.ctx, db.GetCollaborationRuntimeEventParams{RunID: f.run, ProducerEventID: p.EventID})
	if err != nil {
		t.Fatal(err)
	}
	saved, err := decodeCollaborationStoredOutcome(e.Payload)
	if err != nil || len(saved.OutputAttachmentIDs) != 32 {
		t.Fatalf("outcome truncated the bounded output inventory: %v", err)
	}
	if _, err := f.write(runtimeOutputInput()); !collaborationErrorCode(err, "outcome_already_recorded") {
		t.Fatalf("new output appeared after frozen outcome: %v", err)
	}
	if err := f.svc.RecordRuntimeOutcome(context.Background(), f.run, f.permit.BusinessToken, p); err != nil {
		t.Fatalf("full original outcome ACK changed after server-owned references: %v", err)
	}
	changed := p
	changed.Body = "changed final answer"
	if err := f.svc.RecordRuntimeOutcome(context.Background(), f.run, f.permit.BusinessToken, changed); !collaborationErrorCode(err, "outcome_conflict") {
		t.Fatalf("frozen result was replaced: %v", err)
	}
	assertRuntimeOutputCount(t, f, 32, 32)
}

func TestCollaborationRuntimeOutputO08StoredOutcomeDecoderAndRecords(t *testing.T) {
	f := newRuntimeOutputFixture(t)
	p := runtimeOutputInput()
	r, err := f.write(p)
	if err != nil {
		t.Fatal(err)
	}
	outcome := f.outcome()
	if err = f.svc.RecordRuntimeOutcome(context.Background(), f.run, f.permit.BusinessToken, outcome); err != nil {
		t.Fatal(err)
	}
	e, err := f.svc.Queries.GetCollaborationRuntimeEvent(f.ctx, db.GetCollaborationRuntimeEventParams{RunID: f.run, ProducerEventID: outcome.EventID})
	if err != nil {
		t.Fatal(err)
	}
	saved, err := decodeCollaborationStoredOutcome(e.Payload)
	if err != nil || !reflect.DeepEqual(saved.CollaborationRunOutcome, outcome) || !reflect.DeepEqual(saved.OutputAttachmentIDs, []string{util.UUIDToString(r.Attachment.ID)}) {
		t.Fatalf("stored extension changed the original request or frozen file IDs: %+v %v", saved, err)
	}
	if _, err = readCollaborationRecording(f.ctx, f.svc.Queries, f.issue.Issue.ID, f.run, math.MaxInt64); err != nil {
		t.Fatalf("retention reader rejected server-owned output references: %v", err)
	}
	items, err := collaborationRecordEventItems(e)
	if err != nil || len(items) != 1 || items[0].AttachmentIDs == nil || !reflect.DeepEqual(*items[0].AttachmentIDs, saved.OutputAttachmentIDs) {
		t.Fatalf("result record did not project frozen output references: %+v %v", items, err)
	}
	commentEvent, err := f.svc.Queries.GetCollaborationRuntimeEvent(f.ctx, db.GetCollaborationRuntimeEventParams{RunID: f.run, ProducerEventID: "comment:" + util.UUIDToString(p.CommentID)})
	if err != nil {
		t.Fatal(err)
	}
	items, err = collaborationRecordEventItems(commentEvent)
	if err != nil || len(items) != 1 || items[0].AttachmentIDs == nil || !reflect.DeepEqual(*items[0].AttachmentIDs, saved.OutputAttachmentIDs) {
		t.Fatalf("original comment event did not retain its file references: %+v %v", items, err)
	}
	if body, _, err := handoffResultText(e); err != nil || body != outcome.Body {
		t.Fatalf("handoff could not read original result body with its stored extension: %v", err)
	}
	legacy, _ := json.Marshal(outcome)
	if prior, err := decodeCollaborationStoredOutcome(legacy); err != nil || !reflect.DeepEqual(prior.CollaborationRunOutcome, outcome) || len(prior.OutputAttachmentIDs) != 0 {
		t.Fatalf("old absent-extension outcome became invalid: %v", err)
	}
	var fields map[string]json.RawMessage
	if err = json.Unmarshal(e.Payload, &fields); err != nil {
		t.Fatal(err)
	}
	fields["output_attachment_ids"] = json.RawMessage(`["` + util.UUIDToString(r.Attachment.ID) + `","` + util.UUIDToString(r.Attachment.ID) + `"]`)
	invalid, _ := json.Marshal(fields)
	if _, err = decodeCollaborationStoredOutcome(invalid); err == nil {
		t.Fatal("duplicate frozen output IDs were silently accepted")
	}
	t.Run("prepared-output-loses-to-outcome", func(t *testing.T) {
		f := newRuntimeOutputFixture(t)
		firstInput := runtimeOutputInput()
		firstInput.Content += " [@human](mention://member/" + f.handler + ")"
		first, err := f.write(firstInput)
		if err != nil || first.Attachment == nil {
			t.Fatalf("original committed output was not established: %v", err)
		}
		pending := runtimeOutputInput()
		pending.Content += " [@human](mention://member/" + f.handler + ")"
		outcome := f.outcome()
		var frozen db.CollaborationRunEvent
		var outcomeErr error
		preparedObject := ""
		f.svc.Storage = collaborationUploadHookStorage{Storage: f.svc.Storage, uploaded: func() {
			// The new ciphertext exists, but its final runtime transaction has
			// not begun. Commit the outcome before releasing external storage.
			objects := runtimeOutputObjectNames(t, f.dir)
			if len(objects) != 2 {
				t.Fatalf("pending output did not prepare its independent ciphertext: %v", objects)
			}
			for _, name := range objects {
				if name != util.UUIDToString(first.Attachment.ID) {
					preparedObject = name
				}
			}
			assertRuntimeOutputCount(t, f, 1, 1)
			outcomeErr = f.svc.RecordRuntimeOutcome(context.Background(), f.run, f.permit.BusinessToken, outcome)
			if outcomeErr == nil {
				frozen, outcomeErr = f.svc.Queries.GetCollaborationRuntimeEvent(f.ctx, db.GetCollaborationRuntimeEventParams{RunID: f.run, ProducerEventID: outcome.EventID})
			}
		}}
		if _, err = f.write(pending); outcomeErr != nil || preparedObject == "" || !collaborationErrorCode(err, "outcome_already_recorded") {
			t.Fatalf("prepared output did not lose to the committed original outcome: outcome=%v output=%v", outcomeErr, err)
		}
		if _, err = os.Stat(filepath.Join(f.dir, storage.CollaborationPrivatePrefix, preparedObject)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("known losing ciphertext was retained: %v", err)
		}
		objects := runtimeOutputObjectNames(t, f.dir)
		if len(objects) != 1 || objects[0] != util.UUIDToString(first.Attachment.ID) {
			t.Fatalf("loser cleanup changed the winning object: %v", objects)
		}
		after, err := f.svc.Queries.GetCollaborationRuntimeEvent(f.ctx, db.GetCollaborationRuntimeEventParams{RunID: f.run, ProducerEventID: outcome.EventID})
		if err != nil || !reflect.DeepEqual(frozen, after) {
			t.Fatalf("late prepared output changed the original outcome event: %v", err)
		}
		saved, err := decodeCollaborationStoredOutcome(after.Payload)
		if err != nil || !reflect.DeepEqual(saved.OutputAttachmentIDs, []string{util.UUIDToString(first.Attachment.ID)}) {
			t.Fatalf("outcome froze an uncommitted or missing output: %+v %v", saved.OutputAttachmentIDs, err)
		}
		assertRuntimeOutputCount(t, f, 1, 1)
		if f.fx.Count(t, `SELECT count(*) FROM collaboration_notification WHERE issue_id=$1 AND source_id=$2 AND recipient_user_id=$3`, f.issue.Issue.ID, firstInput.CommentID, f.handler) != 1 ||
			f.fx.Count(t, `SELECT count(*) FROM collaboration_notification WHERE issue_id=$1 AND source_id=$2`, f.issue.Issue.ID, pending.CommentID) != 0 ||
			f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1 AND kind='runtime_comment_created'`, f.issue.Issue.ID) != 1 {
			t.Fatal("outcome-first loser changed the original notification or public event effects")
		}
	})
}
