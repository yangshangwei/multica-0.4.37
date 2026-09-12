package handler

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const instructionFixtureTimestamp = "2026-09-12T03:04:05.123456Z"

type instructionUpdateFixture struct {
	handler *Handler
	kind    string
	id      string
	events  []events.Event
}

type instructionUpdateSnapshot struct {
	Instructions string `json:"instructions"`
	UpdatedAt    string `json:"updated_at"`
	Name         string `json:"name"`
}

func newInstructionUpdateFixture(t *testing.T, kind, instructions string) *instructionUpdateFixture {
	t.Helper()
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	h := *testHandler
	h.Bus = events.New()
	f := &instructionUpdateFixture{handler: &h, kind: kind}
	h.Bus.SubscribeAll(func(e events.Event) { f.events = append(f.events, e) })
	overrides := testutil.Cols{
		"instructions": instructions,
		"updated_at":   time.Date(2026, time.September, 12, 3, 4, 5, 123456000, time.UTC),
	}
	switch kind {
	case "agent":
		f.id = dbfx.Agent(t, t.Name(), "", overrides)
	case "squad":
		leaderID := dbfx.Agent(t, t.Name()+" leader", "")
		f.id = dbfx.Squad(t, t.Name(), leaderID, overrides)
		dbfx.Cleanup(t, `DELETE FROM squad_member WHERE squad_id = $1`, f.id)
	default:
		t.Fatalf("unknown instruction resource %q", kind)
	}
	return f
}

func (f *instructionUpdateFixture) request(method string, body any) *http.Request {
	req := testutil.JSONRequest(method, "/api/"+f.kind+"s/"+f.id, body)
	testutil.WithHeaders(req, "X-User-ID", testUserID, "X-Workspace-ID", testWorkspaceID)
	return testutil.WithURLParams(req, "id", f.id, "workspaceId", testWorkspaceID)
}

func (f *instructionUpdateFixture) get(w http.ResponseWriter, r *http.Request) {
	if f.kind == "agent" {
		f.handler.GetAgent(w, r)
	} else {
		f.handler.GetSquad(w, r)
	}
}

func (f *instructionUpdateFixture) update(w http.ResponseWriter, r *http.Request) {
	if f.kind == "agent" {
		f.handler.UpdateAgent(w, r)
	} else {
		f.handler.UpdateSquad(w, r)
	}
}

func (f *instructionUpdateFixture) read(t *testing.T) instructionUpdateSnapshot {
	t.Helper()
	return testutil.Decode[instructionUpdateSnapshot](t, f.get, f.request(http.MethodGet, nil), http.StatusOK)
}

func (f *instructionUpdateFixture) stored(t *testing.T) instructionUpdateSnapshot {
	t.Helper()
	var result instructionUpdateSnapshot
	var updatedAt time.Time
	dbfx.QueryRow(t, fmt.Sprintf(`SELECT instructions, updated_at, name FROM %s WHERE id = $1`, f.kind), f.id).
		Scan(&result.Instructions, &updatedAt, &result.Name)
	result.UpdatedAt = updatedAt.Format(time.RFC3339Nano)
	return result
}

func TestInstructionsPreconditionCapability(t *testing.T) {
	for _, kind := range []string{"agent", "squad"} {
		t.Run(kind, func(t *testing.T) {
			f := newInstructionUpdateFixture(t, kind, "Original instructions")
			get := testutil.Call(t, f.get, f.request(http.MethodGet, nil)).Want(http.StatusOK)
			if got := get.Header().Get("X-Multica-Instructions-Precondition"); got != "1" {
				t.Errorf("GET capability header = %q, want 1 so migration can refuse older servers", got)
			}
			update := testutil.Call(t, f.update, f.request(http.MethodPut, map[string]any{
				"instructions": "中文指令",
			})).Want(http.StatusOK)
			if got := update.Header().Get("X-Multica-Instructions-Precondition"); got != "1" {
				t.Errorf("PUT capability header = %q, want 1", got)
			}
		})
	}
}

func TestInstructionsPreconditionValidation(t *testing.T) {
	cases := []struct {
		name string
		body map[string]any
	}{
		{"missing expected time", map[string]any{"instructions": "中文", "expected_instructions": "Original"}},
		{"missing expected text", map[string]any{"instructions": "中文", "expected_updated_at": instructionFixtureTimestamp}},
		{"missing replacement", map[string]any{"expected_instructions": "Original", "expected_updated_at": instructionFixtureTimestamp}},
		{"null replacement", map[string]any{"instructions": nil, "expected_instructions": "Original", "expected_updated_at": instructionFixtureTimestamp}},
		{"null expected text", map[string]any{"instructions": "中文", "expected_instructions": nil, "expected_updated_at": instructionFixtureTimestamp}},
		{"null expected time", map[string]any{"instructions": "中文", "expected_instructions": "Original", "expected_updated_at": nil}},
		{"both expectations null", map[string]any{"instructions": "中文", "expected_instructions": nil, "expected_updated_at": nil}},
		{"empty expected time", map[string]any{"instructions": "中文", "expected_instructions": "Original", "expected_updated_at": ""}},
		{"invalid expected time", map[string]any{"instructions": "中文", "expected_instructions": "Original", "expected_updated_at": "yesterday"}},
		{"date without time", map[string]any{"instructions": "中文", "expected_instructions": "Original", "expected_updated_at": "2026-09-12"}},
		{"time without zone", map[string]any{"instructions": "中文", "expected_instructions": "Original", "expected_updated_at": "2026-09-12T03:04:05"}},
		{"nonstring expected text", map[string]any{"instructions": "中文", "expected_instructions": 1, "expected_updated_at": instructionFixtureTimestamp}},
		{"nonstring expected time", map[string]any{"instructions": "中文", "expected_instructions": "Original", "expected_updated_at": 1}},
		{"capitalized expected text only", map[string]any{"instructions": "中文", "Expected_Instructions": "Original"}},
		{"capitalized null expectations", map[string]any{"instructions": "中文", "Expected_Instructions": nil, "Expected_Updated_At": nil}},
	}
	for _, kind := range []string{"agent", "squad"} {
		for _, tc := range cases {
			t.Run(kind+"/"+tc.name, func(t *testing.T) {
				f := newInstructionUpdateFixture(t, kind, "Original")
				before := f.stored(t)
				testutil.Call(t, f.update, f.request(http.MethodPut, tc.body)).Want(http.StatusBadRequest)
				if got := f.stored(t); got != before {
					t.Errorf("invalid request mutated resource: got %#v, want %#v", got, before)
				}
				if len(f.events) != 0 {
					t.Errorf("invalid request emitted %d events", len(f.events))
				}
			})
		}
	}
}

func TestInstructionsPreconditionRoundTrip(t *testing.T) {
	for _, kind := range []string{"agent", "squad"} {
		for _, tc := range []struct {
			name, before, after string
		}{
			{"translate", "Original instructions", "请先核对需求，再执行任务。\n保留团队补充。"},
			{"empty original", "", "团队补充"},
			{"clear instructions", "Team instructions", ""},
		} {
			t.Run(kind+"/"+tc.name, func(t *testing.T) {
				f := newInstructionUpdateFixture(t, kind, tc.before)
				before := f.read(t)
				if want := f.stored(t).UpdatedAt; before.UpdatedAt != want {
					t.Fatalf("GET updated_at = %q, want full database precision %q", before.UpdatedAt, want)
				}
				var updated instructionUpdateSnapshot
				testutil.Call(t, f.update, f.request(http.MethodPut, map[string]any{
					"instructions":          tc.after,
					"expected_instructions": before.Instructions,
					"expected_updated_at":   before.UpdatedAt,
				})).Want(http.StatusOK).JSON(&updated)
				if updated.Instructions != tc.after || updated.Name != before.Name {
					t.Errorf("conditional update = %#v, want new instructions and unchanged name", updated)
				}
				if got := f.stored(t); got != updated {
					t.Errorf("saved resource = %#v, want response %#v", got, updated)
				}
				if got := f.read(t); got != updated {
					t.Errorf("GET after PUT = %#v, want response %#v", got, updated)
				}
				if len(f.events) != 1 {
					t.Errorf("successful update emitted %d events, want 1", len(f.events))
				}
			})
		}
	}
}

func TestInstructionsPreconditionLegacyUpdates(t *testing.T) {
	for _, kind := range []string{"agent", "squad"} {
		t.Run(kind, func(t *testing.T) {
			f := newInstructionUpdateFixture(t, kind, "Original")
			for _, replacement := range []string{"直接保存中文", ""} {
				var updated instructionUpdateSnapshot
				testutil.Call(t, f.update, f.request(http.MethodPut, map[string]any{
					"instructions": replacement,
				})).Want(http.StatusOK).JSON(&updated)
				if updated.Instructions != replacement {
					t.Errorf("unguarded update instructions = %q, want %q", updated.Instructions, replacement)
				}
			}
			before := f.stored(t)
			var updated instructionUpdateSnapshot
			testutil.Call(t, f.update, f.request(http.MethodPut, map[string]any{
				"description": "Metadata-only update",
			})).Want(http.StatusOK).JSON(&updated)
			if updated.Instructions != before.Instructions {
				t.Errorf("metadata update changed instructions to %q", updated.Instructions)
			}
		})
	}
}

func TestInstructionsPreconditionConflicts(t *testing.T) {
	for _, kind := range []string{"agent", "squad"} {
		for _, tc := range []struct {
			name, instructions, updatedAt string
		}{
			{"different text", "Changed by teammate", instructionFixtureTimestamp},
			{"different time", "Original", "2026-09-12T03:04:05.123457Z"},
			{"rounded time", "Original", "2026-09-12T03:04:05Z"},
		} {
			t.Run(kind+"/"+tc.name, func(t *testing.T) {
				f := newInstructionUpdateFixture(t, kind, "Original")
				before := f.stored(t)
				testutil.Call(t, f.update, f.request(http.MethodPut, map[string]any{
					"name":                  "Must not be saved",
					"instructions":          "中文",
					"expected_instructions": tc.instructions,
					"expected_updated_at":   tc.updatedAt,
				})).Want(http.StatusConflict)
				if got := f.stored(t); got != before {
					t.Errorf("conflicting request mutated resource: got %#v, want %#v", got, before)
				}
				if len(f.events) != 0 {
					t.Errorf("conflicting request emitted %d events", len(f.events))
				}
			})
		}
	}
}

func TestInstructionsPreconditionRejectsConcurrentWrite(t *testing.T) {
	for _, kind := range []string{"agent", "squad"} {
		t.Run(kind, func(t *testing.T) {
			f := newInstructionUpdateFixture(t, kind, "Original")
			before := f.read(t)
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			conn, err := testPool.Acquire(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Release()
			var pid int32
			if err := conn.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid); err != nil {
				t.Fatal(err)
			}
			f.handler.Queries = db.New(conn)
			f.handler.TxStarter = conn

			concurrent, err := testPool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer concurrent.Rollback(context.Background())
			// Keep the old instructions identical and change only metadata/time.
			// SELECT sees the old committed row; the write waits for this lock.
			if _, err := concurrent.Exec(ctx, fmt.Sprintf(`UPDATE %s SET name = 'Teammate edit', updated_at = updated_at + interval '1 microsecond' WHERE id = $1`, kind), f.id); err != nil {
				t.Fatal(err)
			}
			response := make(chan *testutil.Response, 1)
			done := make(chan struct{})
			req := f.request(http.MethodPut, map[string]any{
				"name":                  "Must not overwrite teammate edit",
				"instructions":          "中文",
				"expected_instructions": before.Instructions,
				"expected_updated_at":   before.UpdatedAt,
			}).WithContext(ctx)
			// Preserve chi route params while giving the request a deadline.
			req = testutil.WithURLParams(req, "id", f.id, "workspaceId", testWorkspaceID)
			go func() {
				defer close(done)
				response <- testutil.Call(t, f.update, req)
			}()
			defer func() {
				_ = concurrent.Rollback(context.Background())
				cancel()
				<-done
			}()
			waitForBlockedWriter(t, ctx, pid)
			if err := concurrent.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			(<-response).Want(http.StatusConflict)
			got := f.stored(t)
			beforeTime, err := time.Parse(time.RFC3339Nano, before.UpdatedAt)
			if err != nil {
				t.Fatal(err)
			}
			if got.Instructions != before.Instructions || got.Name != "Teammate edit" || got.UpdatedAt != beforeTime.Add(time.Microsecond).Format(time.RFC3339Nano) {
				t.Errorf("conditional write overwrote concurrent edit: %#v", got)
			}
			if len(f.events) != 0 {
				t.Errorf("concurrent conflict emitted %d events", len(f.events))
			}
		})
	}
}

func TestInstructionsPreconditionPreservesAuthorization(t *testing.T) {
	for _, kind := range []string{"agent", "squad"} {
		for _, access := range []string{"anonymous", "other member", "other workspace"} {
			t.Run(kind+"/"+access, func(t *testing.T) {
				f := newInstructionUpdateFixture(t, kind, "Original")
				before := f.stored(t)
				req := f.request(http.MethodPut, map[string]any{
					"instructions":          "Must not be saved",
					"expected_instructions": before.Instructions,
					"expected_updated_at":   before.UpdatedAt,
					"expected_workspace_id": testWorkspaceID,
				})
				var want int
				switch access {
				case "anonymous":
					req.Header.Del("X-User-ID")
					want = http.StatusUnauthorized
				case "other member":
					uid := dbfx.User(t, t.Name(), fmt.Sprintf("instructions-%s-%d@multica.test", kind, time.Now().UnixNano()))
					dbfx.Member(t, testWorkspaceID, uid, "member")
					req.Header.Set("X-User-ID", uid)
					want = http.StatusForbidden
				case "other workspace":
					wsID := dbfx.Workspace(t, "Other workspace", fmt.Sprintf("instructions-%s-%d", kind, time.Now().UnixNano()))
					dbfx.Member(t, wsID, testUserID, "owner")
					req.Header.Set("X-Workspace-ID", wsID)
					req = testutil.WithURLParams(req, "id", f.id, "workspaceId", wsID)
					want = http.StatusNotFound
				}
				testutil.Call(t, f.update, req).Want(want)
				if got := f.stored(t); got != before {
					t.Errorf("unauthorized request mutated resource: got %#v, want %#v", got, before)
				}
				if len(f.events) != 0 {
					t.Errorf("unauthorized request emitted %d events", len(f.events))
				}
			})
		}
	}
}

func TestSquadInstructionsPreconditionRollsBackLeaderMembership(t *testing.T) {
	f := newInstructionUpdateFixture(t, "squad", "Original")
	newLeaderID := dbfx.Agent(t, t.Name()+" new leader", "")
	var beforeLeaderID string
	dbfx.QueryRow(t, `SELECT leader_id FROM squad WHERE id = $1`, f.id).Scan(&beforeLeaderID)
	testutil.Call(t, f.update, f.request(http.MethodPut, map[string]any{
		"leader_id":             newLeaderID,
		"instructions":          "中文",
		"expected_instructions": "Outdated instructions",
		"expected_updated_at":   instructionFixtureTimestamp,
	})).Want(http.StatusConflict)
	if got := dbfx.Count(t, `SELECT count(*) FROM squad_member WHERE squad_id = $1 AND member_id = $2`, f.id, newLeaderID); got != 0 {
		t.Errorf("conflict left %d new leader memberships, want 0", got)
	}
	var afterLeaderID string
	dbfx.QueryRow(t, `SELECT leader_id FROM squad WHERE id = $1`, f.id).Scan(&afterLeaderID)
	if afterLeaderID != beforeLeaderID {
		t.Errorf("conflict changed squad leader from %q to %q", beforeLeaderID, afterLeaderID)
	}
	if len(f.events) != 0 {
		t.Errorf("conflict emitted %d events", len(f.events))
	}
}
