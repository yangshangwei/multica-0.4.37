package service

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/multica-ai/multica/server/internal/util"
)

type runtimeOutputRaceResult struct {
	receipt CollaborationRuntimeCommentReceipt
	err     error
}

func raceRuntimeOutputs(t *testing.T, f *runtimeOutputFixture, inputs [2]CollaborationRuntimeCommentAttachmentParams) [2]runtimeOutputRaceResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	arrived, release := make(chan struct{}, 2), make(chan struct{})
	var once sync.Once
	finish := func() { once.Do(func() { close(release) }) }
	defer finish()
	f.svc.Storage = collaborationUploadHookStorage{Storage: f.svc.Storage, uploaded: func() { arrived <- struct{}{}; <-release }}
	var results [2]runtimeOutputRaceResult
	var wg sync.WaitGroup
	for i := range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i].receipt, results[i].err = f.svc.CreateRuntimeCommentAttachment(ctx, f.run, f.permit.BusinessToken, inputs[i])
		}()
	}
	reached := 0
	for reached < 2 {
		select {
		case <-arrived:
			reached++
		case <-ctx.Done():
			finish()
			wg.Wait()
			t.Fatalf("only %d/2 requests reached the real storage barrier: %+v", reached, results)
		}
	}
	finish()
	wg.Wait()
	return results
}

func TestCollaborationRuntimeOutputO04ConcurrentChangedFingerprintPreservesWinner(t *testing.T) {
	for _, kind := range []string{"bytes", "name", "content", "parent"} {
		t.Run(kind, func(t *testing.T) {
			f := newRuntimeOutputFixture(t)
			first := runtimeOutputInput()
			first.Content += " [@human](mention://member/" + f.handler + ")"
			second := first
			comments := 1
			switch kind {
			case "bytes":
				second.Data = []byte("another immutable output")
			case "name":
				second.Name = "another.txt"
			case "content":
				second.Content += " changed body"
			case "parent":
				parent, err := f.svc.CreateRuntimeComment(f.ctx, f.run, f.permit.BusinessToken, CollaborationRuntimeCommentParams{CommentID: util.MustParseUUID(uuid.NewString()), Content: "real parent comment"})
				if err != nil {
					t.Fatal(err)
				}
				second.ParentID = parent.CommentID
				comments++
			}
			inputs := [2]CollaborationRuntimeCommentAttachmentParams{first, second}
			results := raceRuntimeOutputs(t, f, inputs)
			winner := 0
			if results[0].err != nil {
				winner = 1
			}
			if results[winner].err != nil || !collaborationErrorCode(results[1-winner].err, "comment_conflict") || results[winner].receipt.Attachment == nil {
				t.Fatalf("different %s inputs did not produce one winner and exact conflict: %+v", kind, results)
			}
			row, err := f.svc.RuntimeAttachment(context.Background(), f.run, results[winner].receipt.Attachment.ID, 1, f.permit.BusinessToken)
			if err != nil {
				t.Fatal(err)
			}
			if got, err := f.svc.ReadAttachmentBytes(f.ctx, row); err != nil || !bytes.Equal(got, inputs[winner].Data) || row.Name != inputs[winner].Name {
				t.Fatalf("losing fingerprint changed winning bytes/name: %v", err)
			}
			if got := runtimeOutputObjectNames(t, f.dir); len(got) != 1 || got[0] != util.UUIDToString(row.ID) {
				t.Fatalf("loser retained an object or deleted winner: %v", got)
			}
			if f.fx.Count(t, `SELECT count(*) FROM collaboration_notification WHERE issue_id=$1 AND source_id=$2 AND recipient_user_id=$3`, f.issue.Issue.ID, first.CommentID, f.handler) != 1 {
				t.Fatal("concurrent changed output duplicated its original human notification")
			}
			assertRuntimeOutputCount(t, f, comments, 1)
		})
	}
}

func TestCollaborationRuntimeOutputO07ConcurrentThirtySecondOutputBound(t *testing.T) {
	f := newRuntimeOutputFixture(t)
	for range 31 {
		if _, err := f.write(runtimeOutputInput()); err != nil {
			t.Fatal(err)
		}
	}
	results := raceRuntimeOutputs(t, f, [2]CollaborationRuntimeCommentAttachmentParams{runtimeOutputInput(), runtimeOutputInput()})
	winner := 0
	if results[0].err != nil {
		winner = 1
	}
	if results[winner].err != nil || !collaborationErrorCode(results[1-winner].err, "attachment_limit") || len(runtimeOutputObjectNames(t, f.dir)) != 32 {
		t.Fatalf("concurrent output capacity was not serialized at exactly 32: %+v", results)
	}
	assertRuntimeOutputCount(t, f, 32, 32)
}

type runtimeOutputRetryReplayStarter struct {
	TxStarter
	other     CollaborationService
	fixture   *runtimeOutputFixture
	input     CollaborationRuntimeCommentAttachmentParams
	failed    bool
	recovered bool
	otherErr  error
}

type runtimeOutputRetryReplayTx struct {
	pgx.Tx
	owner    *runtimeOutputRetryReplayStarter
	faulted  bool
	replayed bool
}

func (s *runtimeOutputRetryReplayStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &runtimeOutputRetryReplayTx{Tx: tx, owner: s}, nil
}

func (t *runtimeOutputRetryReplayTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if strings.Contains(sql, "-- name: CreateCollaborationEvent ") && !t.owner.failed {
		t.owner.failed, t.faulted = true, true
		return queueFailureRow{err: &pgconn.PgError{Code: "40001", Message: "output callback rolled back after insert"}}
	}
	if t.owner.recovered && strings.Contains(sql, "-- name: GetCollaborationRuntimeEvent ") {
		t.replayed = true
	}
	return t.Tx.QueryRow(ctx, sql, args...)
}

func (t *runtimeOutputRetryReplayTx) Rollback(ctx context.Context) error {
	err := t.Tx.Rollback(ctx)
	if t.faulted && !t.owner.recovered {
		t.owner.recovered = true
		_, t.owner.otherErr = t.owner.other.CreateRuntimeCommentAttachment(ctx, t.owner.fixture.run, t.owner.fixture.permit.BusinessToken, t.owner.input)
	}
	return err
}

func (t *runtimeOutputRetryReplayTx) Commit(ctx context.Context) error {
	if err := t.Tx.Commit(ctx); err != nil {
		return err
	}
	if t.replayed {
		return errors.New("lost read-only replay commit reply")
	}
	return nil
}

func TestCollaborationRuntimeOutputO05RolledBackCallbackNeverOwnsReplayedObject(t *testing.T) {
	f := newRuntimeOutputFixture(t)
	p := runtimeOutputInput()
	other := *f.svc
	other.TxStarter = f.pool
	fault := &runtimeOutputRetryReplayStarter{TxStarter: f.pool, other: other, fixture: f, input: p}
	f.svc.TxStarter = fault
	_, err := f.write(p)
	var unknown *collaborationRuntimeCommitError
	if !errors.As(err, &unknown) || !fault.failed || !fault.recovered || fault.otherErr != nil {
		t.Fatalf("actual insert/rollback/other-winner/unknown-replay sequence did not occur: %v other=%v", err, fault.otherErr)
	}
	f.svc.TxStarter = f.pool
	receipt, err := f.write(p)
	if err != nil || receipt.Attachment == nil {
		t.Fatalf("original winner was not recoverable after callback retry: %v", err)
	}
	if objects := runtimeOutputObjectNames(t, f.dir); len(objects) != 1 || objects[0] != util.UUIDToString(receipt.Attachment.ID) {
		t.Fatalf("rolled-back callback retained its losing object on an unknown replay commit: %v", objects)
	}
	assertRuntimeOutputCount(t, f, 1, 1)
}
