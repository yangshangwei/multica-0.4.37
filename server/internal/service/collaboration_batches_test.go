package service

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
)

func TestCollaborationDirectBatchClaimKeepsEnqueueOrderAcrossTransactions(t *testing.T) {
	f := newManagedCollaborationFixture(t)
	// Hold the older transaction before it takes any queue locks. The newer
	// transaction must commit the first input before the older one can enqueue.
	older, err := f.pool.Begin(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer rollbackCollaborationQueue(f.ctx, older)
	var startedAt time.Time
	if err = older.QueryRow(f.ctx, `SELECT transaction_timestamp()`).Scan(&startedAt); err != nil {
		t.Fatal(err)
	}
	first := f.submitManaged(t, f.creator, "first enqueued input")

	// pgx.Tx.Begin uses a savepoint, keeping the real submission service inside
	// the older transaction without replacing its reads, locks, or writes.
	olderService := NewCollaborationService(f.svc.Queries, older, f.svc.Issues)
	second, err := olderService.SubmitCommand(collaborationPasswordContext(util.MustParseUUID(f.handler), 1), CollaborationSubmitCommandParams{
		CollaborationQueueRequest: f.request(), TargetID: f.targetID, InputEventID: util.MustParseUUID(uuid.NewString()),
		ExpectedTargetEpoch: 1, ExpectedConfigVersion: 1, Content: "second enqueued input",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = older.Commit(f.ctx); err != nil {
		t.Fatal(err)
	}
	if first.BatchID != second.BatchID || first.ChannelID != second.ChannelID || first.CommandSequence != 1 || second.CommandSequence != 2 {
		t.Fatalf("expected two ordered inputs in one direct-agent batch: %+v %+v", first, second)
	}
	var firstCreatedAt, secondCreatedAt time.Time
	f.fx.QueryRow(t, `SELECT first.created_at, second.created_at
		FROM collaboration_command first, collaboration_command second
		WHERE first.id=$1 AND second.id=$2`, first.CommandID, second.CommandID).Scan(&firstCreatedAt, &secondCreatedAt)
	if !secondCreatedAt.Equal(startedAt) || !secondCreatedAt.Before(firstCreatedAt) {
		t.Fatalf("transaction barrier did not invert timestamps: started=%s first=%s second=%s", startedAt, firstCreatedAt, secondCreatedAt)
	}

	f.resumeManaged(t)
	runID, _, claim := f.claimManaged(t)
	if len(claim.Inputs) != 2 {
		t.Fatalf("claim has %d inputs, want both queued inputs", len(claim.Inputs))
	}
	for i, want := range []struct {
		command pgtype.UUID
		author  string
		body    string
	}{
		{first.CommandID, f.creator, "first enqueued input"},
		{second.CommandID, f.handler, "second enqueued input"},
	} {
		got := claim.Inputs[i]
		if got.CommandID != util.UUIDToString(want.command) || got.AuthorUserID != want.author || got.Body != want.body {
			t.Errorf("claim input %d: got %q by %s, want %q by %s in enqueue order", i, got.Body, got.AuthorUserID, want.body, want.author)
		}
	}
	run, err := f.svc.Queries.GetCollaborationRuntimeRun(f.ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	if run.CommandID != first.CommandID {
		t.Error("promotion selected a later-enqueued input as the batch root")
	}
}

func TestCollaborationMixedBatchKeepsExistingInputOrder(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	squad, _ := seedQueueSquad(t, f, false)
	first := f.submit(t, f.creator, "first direct input")
	second := f.submit(t, f.handler, "second direct input")
	third := submitTarget(t, f, squad.ID, f.creator, "first squad input", pgtype.UUID{})
	if first.BatchID != second.BatchID || second.BatchID != third.BatchID || second.CommandSequence != 2 || third.CommandSequence != 1 {
		t.Fatal("expected a mixed batch with independent target sequence counters")
	}
	// M09 changes direct-agent batches only. A target's sequence must not sort
	// a mixed batch globally; lane-wide FIFO remains a separate implementation.
	inputs, err := f.svc.Queries.ListCollaborationBatchInputs(f.ctx, first.BatchID)
	if err != nil {
		t.Fatal(err)
	}
	if len(inputs) != 3 {
		t.Fatalf("mixed batch has %d inputs, want 3", len(inputs))
	}
	for i, command := range []pgtype.UUID{first.CommandID, second.CommandID, third.CommandID} {
		if inputs[i].ID != command {
			t.Errorf("mixed input %d reordered across independent target sequences", i)
		}
	}
}

func TestCollaborationWaitingInputsShareOnePendingBatch(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	one := f.submit(t, f.creator, "creator input remains immutable")
	two := f.submit(t, f.handler, "handler input remains immutable")
	if !one.ChannelID.Valid || !one.BatchID.Valid || one.BatchID != two.BatchID || one.ChannelID != two.ChannelID || one.CommandID == two.CommandID {
		t.Fatalf("waiting inputs were not grouped without losing individual identity: %+v %+v", one, two)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_batch WHERE channel_id=$1 AND state='waiting'`, one.ChannelID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE batch_id=$1 AND state='waiting'`, one.BatchID) != 2 {
		t.Fatal("one paused channel must retain one batch of immutable inputs")
	}
	for _, input := range []CollaborationQueueReceipt{one, two} {
		_, err := f.svc.CancelWaitingCommand(f.ctx, CollaborationCancelCommandParams{CollaborationQueueRequest: f.request(), TargetID: f.targetID, CommandID: input.CommandID, ExpectedCollaborationRevision: 1})
		if err != nil {
			t.Fatal(err)
		}
		if input.CommandID == one.CommandID && f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='waiting' AND author_user_id=$2 AND content='handler input remains immutable'`, two.CommandID, f.handler) != 1 {
			t.Fatal("cancelling one input removed the other author's contribution")
		}
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_batch WHERE id=$1 AND state='waiting'`, one.BatchID) != 0 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE batch_id=$1 AND state='cancelled'`, one.BatchID) != 2 {
		t.Fatal("empty batch remained runnable or cancelled input history disappeared")
	}
}

func TestCollaborationBatchCoalescesDirectAgentAndOneSquad(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	squad, _ := seedQueueSquad(t, f, false)
	a := f.submit(t, f.creator, "direct request")
	s := submitTarget(t, f, squad.ID, f.handler, "squad request", pgtype.UUID{})
	if a.ChannelID != s.ChannelID || a.BatchID != s.BatchID || a.CommandID == s.CommandID {
		t.Fatal("the same actual root did not coalesce into one batch")
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_batch WHERE id=$1 AND effective_target_id=$2 AND state='waiting'`, a.BatchID, squad.ID) != 1 {
		t.Fatal("mixed direct/squad batch did not keep squad role")
	}
	other, _ := seedQueueSquad(t, f, false)
	next := submitTarget(t, f, other.ID, f.creator, "other squad stays separate", pgtype.UUID{})
	if next.ChannelID != a.ChannelID || next.BatchID == a.BatchID {
		t.Fatal("a different same-leader squad must use the next batch on the same channel")
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_batch WHERE channel_id=$1 AND state='waiting'`, a.ChannelID) != 2 {
		t.Fatal("different squad context was discarded or merged")
	}
	if f.fx.Count(t, `SELECT count(DISTINCT author_user_id) FROM collaboration_command WHERE batch_id=$1`, a.BatchID) != 2 {
		t.Fatal("batch authorship was replaced by a single operator")
	}
}

func TestCollaborationBatchKeepsOneFollowupBehindRunningWork(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	first := f.submit(t, f.creator, "running batch")
	f.run(t, first, 1)
	second := f.submit(t, f.handler, "followup one")
	third := f.submit(t, f.creator, "followup two")
	if first.BatchID == second.BatchID || second.BatchID != third.BatchID {
		t.Fatal("running input was changed or followups were not coalesced")
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE batch_id=$1 AND state='in_flight'`, first.BatchID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE batch_id=$1 AND state='waiting'`, second.BatchID) != 2 {
		t.Fatal("batch start failed to freeze its input set")
	}
	if first.ChannelID != second.ChannelID || util.UUIDToString(first.ChannelID) == "" {
		t.Fatal("followup escaped the actual root channel")
	}
}
