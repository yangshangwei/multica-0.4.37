package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// flaggedPrepare marks a request as sent by a daemon whose reachability gate
// dropped the claimed native session. It goes through the wire decoder so the
// test exercises the same strict boundary as the handler.
func flaggedPrepare(t *testing.T, req protocol.TaskKnowledgePrepareRequest) protocol.TaskKnowledgePrepareRequest {
	t.Helper()
	raw, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	fields["resume_unavailable"] = json.RawMessage("true")
	if raw, err = json.Marshal(fields); err != nil {
		t.Fatal(err)
	}
	var out protocol.TaskKnowledgePrepareRequest
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("prepare request cannot carry resume_unavailable: %v", err)
	}
	return out
}

func (f *taskKnowledgeFixture) knowledgeDelivery(t *testing.T, id pgtype.UUID) []byte {
	t.Helper()
	row, err := f.tasks.Queries.GetAgentTask(f.ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	return row.KnowledgeDelivery
}

// staleClaim records one exposed turn under "stale-native", then finalizes and
// starts the next turn bound to that same session.
func (f *taskKnowledgeFixture) staleClaim(t *testing.T) (*protocol.TaskKnowledgeClaimDescriptor, *protocol.TaskKnowledgeClaimDescriptor, pgtype.UUID) {
	t.Helper()
	first := f.recordSession(t, "stale-native")
	if _, err := f.tasks.CompleteTask(f.ctx, f.task.ID, []byte(`{"output":"done"}`), "stale-native", "", "", false, "", ""); err != nil {
		t.Fatal(err)
	}
	staleRow := f.task.ID
	f.nextTurn(t)
	d := f.finalize(t, "stale-native")
	if d.Session.Generation != first.Session.Generation || d.Session.NativeSessionID != "stale-native" {
		t.Fatal("fixture did not bind the next claim to the stale session")
	}
	if _, err := f.tasks.StartTask(f.ctx, f.task.ID); err != nil {
		t.Fatal(err)
	}
	return first, d, staleRow
}

func TestKnowledgeUnavailableResumeRebindsFreshGeneration(t *testing.T) {
	f := newTaskKnowledgeFixture(t)
	first, d, staleRow := f.staleClaim(t)
	staleBytes := f.knowledgeDelivery(t, staleRow)
	req := flaggedPrepare(t, f.prepareRequest(d))
	p, err := f.tasks.PrepareTaskKnowledge(f.ctx, f.caller, f.task.ID, req)
	if err != nil {
		t.Fatalf("flagged first preparation was refused: %v", err)
	}
	if p.Session.NativeSessionID != "" || p.Session.Generation == first.Session.Generation {
		t.Fatalf("unreachable resume kept the stale binding: %+v", p.Session)
	}
	want := d.Session
	want.Generation, want.NativeSessionID = p.Session.Generation, ""
	if p.Session != want {
		t.Fatal("rebind changed a binding field other than generation and native ID")
	}
	if _, err := uuid.Parse(p.Session.Generation); err != nil {
		t.Fatal("rebind did not mint a UUID scope generation")
	}
	found := false
	for _, entry := range p.Manifest.Entries {
		found = found || entry.CandidateID == f.candidate.CandidateID
	}
	if !found {
		t.Fatal("fresh session did not resolve current approved knowledge")
	}
	replay, err := f.tasks.PrepareTaskKnowledge(f.ctx, f.caller, f.task.ID, req)
	if err != nil || replay.PrepareID != p.PrepareID || replay.Session != p.Session {
		t.Fatalf("original flagged receipt did not replay: %v", err)
	}
	rebound := *d
	rebound.Session = p.Session
	a, err := f.tasks.AuthorizeTaskKnowledgeLaunch(f.ctx, f.caller, f.task.ID, f.authorizeRequest(p, &rebound))
	if err != nil {
		t.Fatalf("fresh binding was not authorized: %v", err)
	}
	f.observe(t, &rebound, a, "started", "")
	f.observe(t, &rebound, a, "native_session", "fresh-native")
	f.observe(t, &rebound, a, "terminated", "")
	if _, err := f.tasks.CompleteTask(f.ctx, f.task.ID, []byte(`{"output":"done"}`), "fresh-native", "", "", false, "", ""); err != nil {
		t.Fatalf("fresh native pointer was refused: %v", err)
	}
	if !bytes.Equal(staleBytes, f.knowledgeDelivery(t, staleRow)) {
		t.Fatal("rebind relabeled the stale session's recorded exposure")
	}
	row, err := f.tasks.Queries.GetAgentTask(f.ctx, f.task.ID)
	if err != nil {
		t.Fatal(err)
	}
	meta, err := decodeTaskKnowledge(row.KnowledgeContextVersion, row.KnowledgeDelivery)
	if err != nil {
		t.Fatal(err)
	}
	c := meta.Claims[0]
	if c.Descriptor.Session != p.Session || len(c.Attempts) != 1 || c.Attempts[0].BoundSessionID != "fresh-native" {
		t.Fatal("current claim did not retain the rebound binding and its observed native ID")
	}
	last, err := f.tasks.Queries.GetLastTaskSession(f.ctx, db.GetLastTaskSessionParams{AgentID: f.task.AgentID, IssueID: f.task.IssueID})
	if err != nil || last.SessionID.String != "fresh-native" {
		t.Fatalf("the stale session still wins the next resume choice: %+v %v", last, err)
	}
	f.nextTurn(t)
	next, prior, err := f.tasks.BuildTaskKnowledgeClaim(f.ctx, f.task, &f.inv, "fresh-native")
	if err != nil || prior != "fresh-native" || next.Session.Generation != p.Session.Generation || next.Session.NativeSessionID != "fresh-native" {
		t.Fatalf("the fresh session did not resume under its own generation: %q %v", prior, err)
	}
	old, prior, err := f.tasks.BuildTaskKnowledgeClaim(f.ctx, f.task, &f.inv, "stale-native")
	if err != nil || prior != "stale-native" || old.Session.Generation != first.Session.Generation {
		t.Fatalf("the stale session lost its own generation: %q %v", prior, err)
	}
}

// The dropped session's history is neither locked nor revalidated for a turn
// that will not see it: a source withdrawn after the stale claim was bound
// still refuses the resume, but not the fresh turn, which never sees it.
func TestKnowledgeUnavailableResumeSkipsDroppedSessionRevalidation(t *testing.T) {
	f := newTaskKnowledgeFixture(t)
	_, d, _ := f.staleClaim(t)
	f.withdraw(t, f.candidate.CandidateID)
	if _, err := f.tasks.PrepareTaskKnowledge(f.ctx, f.caller, f.task.ID, f.prepareRequest(d)); !collaborationErrorCode(err, "knowledge_context_changed") {
		t.Fatalf("resume of a session with withdrawn exposure was prepared: %v", err)
	}
	p, err := f.tasks.PrepareTaskKnowledge(f.ctx, f.caller, f.task.ID, flaggedPrepare(t, f.prepareRequest(d)))
	if err != nil {
		t.Fatalf("withdrawn stale-only exposure blocked the fresh turn: %v", err)
	}
	if p.Session.NativeSessionID != "" || p.Session.Generation == d.Session.Generation || len(p.Contributions) != 0 {
		t.Fatal("fresh turn did not resolve current knowledge under a new binding")
	}
}

func TestKnowledgeUnavailableResumeRefusesUnsafeRebind(t *testing.T) {
	t.Run("nothing to drop", func(t *testing.T) {
		f := newTaskKnowledgeFixture(t)
		d := f.finalize(t, "")
		if _, err := f.tasks.StartTask(f.ctx, f.task.ID); err != nil {
			t.Fatal(err)
		}
		before := f.knowledgeDelivery(t, f.task.ID)
		_, err := f.tasks.PrepareTaskKnowledge(f.ctx, f.caller, f.task.ID, flaggedPrepare(t, f.prepareRequest(d)))
		var domain *CollaborationError
		if !errors.As(err, &domain) || domain.Code != "invalid_knowledge_request" || domain.Status != 400 {
			t.Fatalf("flagged fresh claim was not a bad request: %v", err)
		}
		if !bytes.Equal(before, f.knowledgeDelivery(t, f.task.ID)) {
			t.Fatal("refused request changed knowledge delivery")
		}
	})
	t.Run("already disclosed", func(t *testing.T) {
		f := newTaskKnowledgeFixture(t)
		_, d, _ := f.staleClaim(t)
		if _, err := f.tasks.PrepareTaskKnowledge(f.ctx, f.caller, f.task.ID, f.prepareRequest(d)); err != nil {
			t.Fatal(err)
		}
		_, err := f.tasks.PrepareTaskKnowledge(f.ctx, f.caller, f.task.ID, flaggedPrepare(t, f.prepareRequest(d)))
		if !collaborationErrorCode(err, "idempotency_conflict") {
			t.Fatalf("flagged request rebound a disclosed claim: %v", err)
		}
		row, err := f.tasks.Queries.GetAgentTask(f.ctx, f.task.ID)
		if err != nil {
			t.Fatal(err)
		}
		meta, err := decodeTaskKnowledge(row.KnowledgeContextVersion, row.KnowledgeDelivery)
		if err != nil {
			t.Fatal(err)
		}
		if meta.Claims[0].Descriptor.Session != d.Session {
			t.Fatal("disclosed claim lost its stale native binding")
		}
	})
	t.Run("altered binding", func(t *testing.T) {
		f := newTaskKnowledgeFixture(t)
		_, d, _ := f.staleClaim(t)
		before := f.knowledgeDelivery(t, f.task.ID)
		req := f.prepareRequest(d)
		req.Session.Generation = uuid.NewString()
		_, err := f.tasks.PrepareTaskKnowledge(f.ctx, f.caller, f.task.ID, flaggedPrepare(t, req))
		if !collaborationErrorCode(err, "knowledge_context_changed") {
			t.Fatalf("flagged request with another binding was accepted: %v", err)
		}
		if !bytes.Equal(before, f.knowledgeDelivery(t, f.task.ID)) {
			t.Fatal("refused request changed knowledge delivery")
		}
		row, err := f.tasks.Queries.GetAgentTask(f.ctx, f.task.ID)
		if err != nil {
			t.Fatal(err)
		}
		meta, err := decodeTaskKnowledge(row.KnowledgeContextVersion, row.KnowledgeDelivery)
		if err != nil {
			t.Fatal(err)
		}
		if meta.Claims[0].Prepared != nil {
			t.Fatal("refused request disclosed knowledge")
		}
	})
}
