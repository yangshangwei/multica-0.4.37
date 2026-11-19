package main

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"log/slog"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/handler"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// These tests replay a saved comment's event through the actual ordinary
// notification listener. They do not repeat HTTP comment creation, register
// subscriber mutation listeners, or claim deduplication for other producers.
type commentReplayFixture struct {
	fx      *testutil.Fixture
	queries *db.Queries
}

func newCommentReplayFixture(t *testing.T) *commentReplayFixture {
	t.Helper()
	base := testutil.New(testPool, testWorkspaceID, testUserID)
	key := uuid.NewString()
	actor := base.User(t, "Comment replay actor", "comment-replay-"+key+"@multica.ai")
	workspace := base.Workspace(t, "Comment replay", "comment-replay-"+key)
	base.Member(t, workspace, actor, "owner")
	return &commentReplayFixture{
		fx:      testutil.New(testPool, workspace, actor),
		queries: db.New(testPool),
	}
}

func (f *commentReplayFixture) member(t *testing.T, name string) string {
	t.Helper()
	id := f.fx.User(t, name, "comment-replay-"+uuid.NewString()+"@multica.ai")
	f.fx.Member(t, f.fx.WorkspaceID, id, "member")
	return id
}

func (f *commentReplayFixture) issue(t *testing.T, title string, over ...testutil.Cols) string {
	t.Helper()
	id := f.fx.Issue(t, title, over...)
	// Generated notifications belong to this test's exact source issue. Register
	// their cleanup after the issue and before any comments/subscribers.
	f.fx.Cleanup(t, `DELETE FROM inbox_item WHERE workspace_id=$1 AND issue_id=$2`, f.fx.WorkspaceID, id)
	return id
}

func (f *commentReplayFixture) subscribe(t *testing.T, issue, userType, user, reason string, over ...testutil.Cols) {
	t.Helper()
	cols := testutil.Cols{"issue_id": issue, "user_type": userType, "user_id": user, "reason": reason}
	for _, values := range over {
		for key, value := range values {
			cols[key] = value
		}
	}
	f.fx.InsertNoID(t, "issue_subscriber", cols, "issue_id=$1 AND user_type=$2 AND user_id=$3", issue, userType, user)
}

func (f *commentReplayFixture) event(t *testing.T, commentID string) events.Event {
	t.Helper()
	comment, err := f.queries.GetComment(context.Background(), util.MustParseUUID(commentID))
	if err != nil {
		t.Fatalf("read persisted comment: %v", err)
	}
	issue, err := f.queries.GetIssue(context.Background(), comment.IssueID)
	if err != nil {
		t.Fatalf("read persisted source issue: %v", err)
	}
	return events.Event{
		Type:        protocol.EventCommentCreated,
		WorkspaceID: util.UUIDToString(comment.WorkspaceID),
		ActorType:   comment.AuthorType,
		ActorID:     util.UUIDToString(comment.AuthorID),
		Payload: map[string]any{
			"comment": handler.CommentResponse{
				ID:         util.UUIDToString(comment.ID),
				IssueID:    util.UUIDToString(comment.IssueID),
				AuthorType: comment.AuthorType,
				AuthorID:   util.UUIDToString(comment.AuthorID),
				Content:    comment.Content,
				Type:       comment.Type,
				CreatedAt:  comment.CreatedAt.Time.UTC().Format(time.RFC3339Nano),
				UpdatedAt:  comment.UpdatedAt.Time.UTC().Format(time.RFC3339Nano),
				Revision:   comment.Revision,
			},
			"issue_title":  issue.Title,
			"issue_status": issue.Status,
		},
	}
}

func commentReplayWithID(e events.Event, id string) events.Event {
	payload := e.Payload.(map[string]any)
	comment := payload["comment"].(handler.CommentResponse)
	comment.ID = id
	e.Payload = map[string]any{
		"comment":      comment,
		"issue_title":  payload["issue_title"],
		"issue_status": payload["issue_status"],
	}
	return e
}

// Read all retained rows, including read/archived rows that ListInboxItems hides.
func (f *commentReplayFixture) items(t *testing.T, issue string) []db.InboxItem {
	t.Helper()
	var raw []byte
	f.fx.QueryRow(t, `SELECT COALESCE(jsonb_agg(id ORDER BY id), '[]'::jsonb)
		FROM inbox_item WHERE workspace_id=$1 AND issue_id=$2`, f.fx.WorkspaceID, issue).Scan(&raw)
	var ids []string
	if err := json.Unmarshal(raw, &ids); err != nil {
		t.Fatalf("decode retained inbox IDs: %v", err)
	}
	items := make([]db.InboxItem, 0, len(ids))
	for _, id := range ids {
		item, err := f.queries.GetInboxItem(context.Background(), util.MustParseUUID(id))
		if err != nil {
			t.Fatalf("read retained inbox item %s: %v", id, err)
		}
		items = append(items, item)
	}
	return items
}

type commentReplayCollector struct {
	mu     sync.Mutex
	events []events.Event
}

func (c *commentReplayCollector) bus(queries *db.Queries) *events.Bus {
	bus := events.New()
	registerNotificationListeners(bus, queries)
	bus.Subscribe(protocol.EventInboxNew, func(e events.Event) {
		c.mu.Lock()
		defer c.mu.Unlock()
		c.events = append(c.events, e)
	})
	return bus
}

func (c *commentReplayCollector) snapshot() []events.Event {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]events.Event(nil), c.events...)
}

// Match broadcasts to the explicit persisted row set supplied by each test.
func checkCommentReplayBroadcasts(t *testing.T, c *commentReplayCollector, want []db.InboxItem) {
	t.Helper()
	notices := c.snapshot()
	if len(notices) != len(want) {
		t.Errorf("inbox:new count = %d, want %d for the original persisted deliveries", len(notices), len(want))
	}
	byID := make(map[string]db.InboxItem, len(want))
	for _, row := range want {
		byID[util.UUIDToString(row.ID)] = row
	}
	seen := make(map[string]int, len(notices))
	for _, notice := range notices {
		payload, ok := notice.Payload.(map[string]any)
		if !ok {
			t.Errorf("inbox:new payload has type %T", notice.Payload)
			continue
		}
		item, ok := payload["item"].(map[string]any)
		if !ok {
			t.Errorf("inbox:new item has type %T", payload["item"])
			continue
		}
		id, _ := item["id"].(string)
		row, exists := byID[id]
		if !exists {
			t.Errorf("inbox:new published nonoriginal ID %q", id)
			continue
		}
		seen[id]++
		if notice.WorkspaceID != util.UUIDToString(row.WorkspaceID) ||
			item["workspace_id"] != util.UUIDToString(row.WorkspaceID) ||
			item["recipient_type"] != row.RecipientType ||
			item["recipient_id"] != util.UUIDToString(row.RecipientID) || item["type"] != row.Type {
			t.Errorf("inbox:new tuple does not match persisted row %s", id)
		}
	}
	for id := range byID {
		if seen[id] != 1 {
			t.Errorf("original inbox ID %s was broadcast %d times, want 1", id, seen[id])
		}
	}
}

func TestNotificationCommentReplaySequential(t *testing.T) {
	for _, name := range []string{"identical_event", "fresh_bus", "case_equivalent_uuid", "map_payload"} {
		t.Run(name, func(t *testing.T) {
			f := newCommentReplayFixture(t)
			issue := f.issue(t, "Original sequential source")
			first := f.member(t, "First watcher")
			second := f.member(t, "Second watcher")
			f.subscribe(t, issue, "member", first, "manual")
			f.subscribe(t, issue, "member", second, "assignee")
			comment := f.fx.Comment(t, issue, "One saved plain comment")
			event := f.event(t, comment)
			if name == "map_payload" {
				payload := event.Payload.(map[string]any)
				originalComment := payload["comment"].(handler.CommentResponse)
				event.Payload = map[string]any{
					"comment": map[string]any{
						"id": originalComment.ID, "issue_id": originalComment.IssueID,
						"author_type": originalComment.AuthorType, "author_id": originalComment.AuthorID,
						"content": originalComment.Content, "type": originalComment.Type,
					},
					"issue_title": payload["issue_title"], "issue_status": payload["issue_status"],
				}
			}
			collector := &commentReplayCollector{}
			bus := collector.bus(f.queries)
			bus.Publish(event)
			original := f.items(t, issue)
			if len(original) != 2 {
				t.Fatalf("first delivery created %d rows, want 2 real recipients", len(original))
			}
			byRecipient := map[string]int{}
			for _, row := range original {
				byRecipient[util.UUIDToString(row.RecipientID)]++
				var details map[string]string
				if err := json.Unmarshal(row.Details, &details); err != nil {
					t.Fatalf("decode original comment details: %v", err)
				}
				if row.Type != "new_comment" || details["comment_id"] != comment ||
					row.RecipientType != "member" ||
					(util.UUIDToString(row.RecipientID) != first && util.UUIDToString(row.RecipientID) != second) ||
					row.Title != "Original sequential source" || !row.Body.Valid || row.Body.String != "One saved plain comment" ||
					row.ActorType.String != "member" || util.UUIDToString(row.ActorID) != f.fx.UserID ||
					row.Read || row.Archived {
					t.Fatalf("first delivery has the wrong original comment/recipient/payload/state: %+v", row)
				}
			}
			if byRecipient[first] != 1 || byRecipient[second] != 1 {
				t.Fatalf("first delivery recipient counts = %v, want each original recipient once", byRecipient)
			}
			switch name {
			case "fresh_bus":
				bus = collector.bus(db.New(testPool))
			case "case_equivalent_uuid":
				event = commentReplayWithID(event, strings.ToUpper(comment))
			}
			bus.Publish(event)
			after := f.items(t, issue)
			if !reflect.DeepEqual(after, original) {
				t.Errorf("same original comment replay changed retained inbox rows: before=%d after=%d", len(original), len(after))
			}
			checkCommentReplayBroadcasts(t, collector, original)
		})
	}
}

// Pause only the first two real inbox inserts, without replacing their SQL or
// results. Both publishers reach the database write boundary before release.
type commentReplayInsertGate struct {
	db.DBTX
	mu       sync.Mutex
	arrivals int
	ready    chan struct{}
	release  chan struct{}
	once     sync.Once
}

func (g *commentReplayInsertGate) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	block := false
	if strings.Contains(sql, "INSERT INTO inbox_item") {
		g.mu.Lock()
		g.arrivals++
		block = g.arrivals <= 2
		g.mu.Unlock()
	}
	if block {
		g.ready <- struct{}{}
		<-g.release
	}
	return g.DBTX.QueryRow(ctx, sql, args...)
}

func (g *commentReplayInsertGate) open() {
	g.once.Do(func() { close(g.release) })
}

// The bus recovers listener panics, and listeners log query failures. Observe
// errors as well as rows, so a failed competing writer cannot masquerade as
// successful deduplication. No publisher goroutine calls testing.T methods.
type commentReplayErrors struct {
	mu       sync.Mutex
	messages []string
}

type commentReplayLogHandler struct {
	next   slog.Handler
	errors *commentReplayErrors
}

func (h commentReplayLogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= slog.LevelError || h.next.Enabled(ctx, level)
}

func (h commentReplayLogHandler) Handle(ctx context.Context, record slog.Record) error {
	if record.Level >= slog.LevelError {
		h.errors.mu.Lock()
		h.errors.messages = append(h.errors.messages, record.Message)
		h.errors.mu.Unlock()
	}
	return h.next.Handle(ctx, record)
}

func (h commentReplayLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return commentReplayLogHandler{next: h.next.WithAttrs(attrs), errors: h.errors}
}

func (h commentReplayLogHandler) WithGroup(name string) slog.Handler {
	return commentReplayLogHandler{next: h.next.WithGroup(name), errors: h.errors}
}

func TestNotificationCommentReplayConcurrent(t *testing.T) {
	for _, name := range []string{"shared_bus", "independent_buses"} {
		t.Run(name, func(t *testing.T) {
			f := newCommentReplayFixture(t)
			issue := f.issue(t, "Concurrent original source")
			wantRecipients := map[string]bool{}
			for _, name := range []string{"Concurrent first", "Concurrent second"} {
				recipient := f.member(t, name)
				f.subscribe(t, issue, "member", recipient, "manual")
				wantRecipients[recipient] = true
			}
			comment := f.fx.Comment(t, issue, "Immutable concurrent event")
			event := f.event(t, comment)
			gate := &commentReplayInsertGate{DBTX: testPool, ready: make(chan struct{}, 2), release: make(chan struct{})}
			collector := &commentReplayCollector{}
			first := collector.bus(db.New(gate))
			second := first
			if name == "independent_buses" {
				second = collector.bus(db.New(gate))
			}
			observed := &commentReplayErrors{}
			previous := slog.Default()
			legacyWriter, legacyFlags := log.Writer(), log.Flags()
			slog.SetDefault(slog.New(commentReplayLogHandler{next: slog.NewTextHandler(io.Discard, nil), errors: observed}))
			// SetDefault also rewires legacy log. Restore its exact state after
			// the later-registered cleanup releases and joins both publishers.
			t.Cleanup(func() {
				slog.SetDefault(previous)
				log.SetOutput(legacyWriter)
				log.SetFlags(legacyFlags)
			})
			var publishers sync.WaitGroup
			publishers.Add(2)
			t.Cleanup(func() {
				gate.open()
				publishers.Wait()
			})
			start := make(chan struct{})
			for _, bus := range []*events.Bus{first, second} {
				go func(bus *events.Bus) {
					defer publishers.Done()
					<-start
					bus.Publish(event)
				}(bus)
			}
			close(start)
			timer := time.NewTimer(5 * time.Second)
			defer timer.Stop()
			arrived := 0
			for arrived < 2 {
				select {
				case <-gate.ready:
					arrived++
				case <-timer.C:
					gate.open()
					publishers.Wait()
					t.Fatalf("only %d publishers reached the real inbox insert boundary", arrived)
				}
			}
			gate.open()
			publishers.Wait()
			observed.mu.Lock()
			loggedErrors := append([]string(nil), observed.messages...)
			observed.mu.Unlock()
			if len(loggedErrors) != 0 {
				t.Errorf("concurrent delivery logged swallowed query failures/listener panics: %v", loggedErrors)
			}
			rows := f.items(t, issue)
			if len(rows) != 2 {
				t.Errorf("concurrent replay created %d retained rows, want one per each of 2 recipients", len(rows))
			}
			if count := len(collector.snapshot()); count != 2 {
				t.Errorf("concurrent replay published %d inbox:new events, want exactly 2", count)
			}
			byRecipient := map[string]int{}
			for _, row := range rows {
				recipient := util.UUIDToString(row.RecipientID)
				byRecipient[recipient]++
				var details map[string]string
				if err := json.Unmarshal(row.Details, &details); err != nil || details["comment_id"] != comment ||
					row.Type != "new_comment" || row.RecipientType != "member" || !wantRecipients[recipient] {
					t.Errorf("concurrent delivery has an unexpected source/recipient/type tuple: %+v, error=%v", row, err)
				}
			}
			for recipient := range wantRecipients {
				count := byRecipient[recipient]
				if count != 1 {
					t.Errorf("original comment %s has %d deliveries for recipient %s, want 1", comment, count, recipient)
				}
			}
			checkCommentReplayBroadcasts(t, collector, rows)
		})
	}
}

func TestNotificationCommentReplayRetainedState(t *testing.T) {
	for _, state := range []struct {
		name     string
		read     bool
		archived bool
	}{
		{"read", true, false},
		{"archived", false, true},
		{"read_and_archived", true, true},
	} {
		t.Run(state.name, func(t *testing.T) {
			f := newCommentReplayFixture(t)
			issue := f.issue(t, "Original retained title")
			f.subscribe(t, issue, "member", f.member(t, "Retained watcher"), "manual")
			comment := f.fx.Comment(t, issue, "Original retained content")
			event := f.event(t, comment)
			collector := &commentReplayCollector{}
			bus := collector.bus(f.queries)
			bus.Publish(event)
			rows := f.items(t, issue)
			if len(rows) != 1 {
				t.Fatalf("first retained delivery = %d rows, want 1", len(rows))
			}
			if state.read {
				if _, err := f.queries.MarkInboxRead(context.Background(), rows[0].ID); err != nil {
					t.Fatalf("read original inbox ID: %v", err)
				}
			}
			if state.archived {
				if _, err := f.queries.ArchiveInboxItem(context.Background(), rows[0].ID); err != nil {
					t.Fatalf("archive original inbox ID: %v", err)
				}
			}
			original := f.items(t, issue)
			if original[0].Read != state.read || original[0].Archived != state.archived {
				t.Fatal("original read/archive operation was not persisted")
			}
			// A later saved edit has the same comment UUID, while the original
			// inbox remains an immutable delivery of the first event's payload.
			f.fx.Exec(t, `UPDATE issue SET title=$2 WHERE id=$1`, issue, "Changed retained title")
			f.fx.Exec(t, `UPDATE comment SET content=$2, revision=revision+1 WHERE id=$1`, comment, "Changed retained content")
			fresh := collector.bus(db.New(testPool))
			fresh.Publish(f.event(t, comment))
			fresh.Publish(event)
			after := f.items(t, issue)
			if !reflect.DeepEqual(after, original) {
				t.Errorf("same comment ID rewrote/reopened/duplicated original read/archive/title/body/details/time: before=%d after=%d", len(original), len(after))
			}
			checkCommentReplayBroadcasts(t, collector, original)
		})
	}
}

func TestNotificationCommentReplayLegacyRows(t *testing.T) {
	for _, name := range []string{"random_v4_read", "random_v7_archived", "uppercase_legacy_id", "existing_duplicates_retained"} {
		t.Run(name, func(t *testing.T) {
			f := newCommentReplayFixture(t)
			issue := f.issue(t, "Current legacy source")
			watcher := f.member(t, "Legacy watcher")
			f.subscribe(t, issue, "member", watcher, "manual")
			comment := f.fx.Comment(t, issue, "Current legacy payload")
			legacyComment := comment
			if name == "uppercase_legacy_id" {
				legacyComment = strings.ToUpper(comment)
			}
			count := 1
			if name == "existing_duplicates_retained" {
				count = 2
			}
			for i := 0; i < count; i++ {
				id := uuid.NewString()
				if name == "random_v7_archived" {
					id = util.UUIDToString(dbid.NewV7())
				}
				details, err := json.Marshal(map[string]string{"comment_id": legacyComment, "retained": "old writer"})
				if err != nil {
					t.Fatalf("encode retained legacy details: %v", err)
				}
				f.fx.Insert(t, "inbox_item", testutil.Cols{
					"id": id, "workspace_id": f.fx.WorkspaceID,
					"recipient_type": "member", "recipient_id": watcher,
					"type": "new_comment", "severity": "info", "issue_id": issue,
					"title": "Historical original title", "body": "Historical original content",
					"actor_type": "member", "actor_id": f.fx.UserID,
					"details": string(details), "read": name != "random_v7_archived",
					"archived":   name != "random_v4_read",
					"created_at": time.Date(2026, 10, 1, 2, 3, 4, i*1000, time.UTC),
				})
			}
			original := f.items(t, issue)
			if len(original) != count {
				t.Fatalf("retained legacy fixture has %d rows, want %d", len(original), count)
			}
			collector := &commentReplayCollector{}
			bus := collector.bus(f.queries)
			bus.Publish(f.event(t, comment))
			collector.bus(db.New(testPool)).Publish(f.event(t, comment))
			after := f.items(t, issue)
			if !reflect.DeepEqual(after, original) {
				t.Errorf("replay altered retained random-ID rows or added another delivery: before=%d after=%d", len(original), len(after))
			}
			checkCommentReplayBroadcasts(t, collector, nil)
		})
	}
}

func TestNotificationCommentReplayDistinctTuples(t *testing.T) {
	t.Run("distinct_comments_and_recipients", func(t *testing.T) {
		f := newCommentReplayFixture(t)
		issue := f.issue(t, "Distinct original comments")
		first := f.member(t, "Distinct first")
		second := f.member(t, "Distinct second")
		f.subscribe(t, issue, "member", first, "manual")
		f.subscribe(t, issue, "member", second, "manual")
		comments := []string{f.fx.Comment(t, issue, "Equal text"), f.fx.Comment(t, issue, "Equal text")}
		collector := &commentReplayCollector{}
		bus := collector.bus(f.queries)
		for _, comment := range comments {
			event := f.event(t, comment)
			bus.Publish(event)
			bus.Publish(event)
		}
		rows := f.items(t, issue)
		if len(rows) != 4 {
			t.Errorf("2 original comments x 2 recipients created %d rows, want 4", len(rows))
		}
		if count := len(collector.snapshot()); count != 4 {
			t.Errorf("2 original comments x 2 recipients published %d inbox:new events, want 4", count)
		}
		pairs := map[[2]string]int{}
		for _, row := range rows {
			var details map[string]string
			if err := json.Unmarshal(row.Details, &details); err != nil {
				t.Fatalf("decode distinct source details: %v", err)
			}
			pairs[[2]string{details["comment_id"], util.UUIDToString(row.RecipientID)}]++
		}
		for _, comment := range comments {
			for _, recipient := range []string{first, second} {
				if n := pairs[[2]string{comment, recipient}]; n != 1 {
					t.Errorf("comment %s/recipient %s has %d rows, want 1", comment, recipient, n)
				}
			}
		}
		checkCommentReplayBroadcasts(t, collector, rows)
	})

	t.Run("distinct_workspace_sources", func(t *testing.T) {
		first := newCommentReplayFixture(t)
		second := newCommentReplayFixture(t)
		watcher := first.member(t, "Shared account, separate sources")
		second.fx.Member(t, second.fx.WorkspaceID, watcher, "member")
		firstIssue := first.issue(t, "First workspace source")
		secondIssue := second.issue(t, "Second workspace source")
		first.subscribe(t, firstIssue, "member", watcher, "manual")
		second.subscribe(t, secondIssue, "member", watcher, "manual")
		firstComment := first.fx.Comment(t, firstIssue, "Identical text across sources")
		secondComment := second.fx.Comment(t, secondIssue, "Identical text across sources")
		collector := &commentReplayCollector{}
		bus := collector.bus(first.queries)
		firstEvent, secondEvent := first.event(t, firstComment), second.event(t, secondComment)
		bus.Publish(firstEvent)
		bus.Publish(secondEvent)
		bus.Publish(firstEvent)
		bus.Publish(secondEvent)
		firstRows, secondRows := first.items(t, firstIssue), second.items(t, secondIssue)
		if len(firstRows) != 1 || len(secondRows) != 1 {
			t.Errorf("distinct workspace source counts = %d/%d, want one original delivery each", len(firstRows), len(secondRows))
		}
		if len(firstRows) == 1 && len(secondRows) == 1 && firstRows[0].ID == secondRows[0].ID {
			t.Error("distinct source deliveries shared an inbox ID")
		}
		if count := len(collector.snapshot()); count != 2 {
			t.Errorf("distinct workspace sources published %d inbox:new events, want 2", count)
		}
		checkCommentReplayBroadcasts(t, collector, append(firstRows, secondRows...))
	})

	t.Run("unrelated_retained_tuple_fields", func(t *testing.T) {
		f := newCommentReplayFixture(t)
		issue := f.issue(t, "Exact current tuple")
		otherIssue := f.issue(t, "Different issue tuple")
		watcher := f.member(t, "Exact current watcher")
		otherWatcher := f.member(t, "Different recipient tuple")
		f.subscribe(t, issue, "member", watcher, "manual")
		comment := f.fx.Comment(t, issue, "Current exact source")
		other := newCommentReplayFixture(t)
		otherSource := other.issue(t, "Different workspace tuple")
		legacyIDs := []string{}
		for _, change := range []testutil.Cols{
			{"workspace_id": other.fx.WorkspaceID, "issue_id": otherSource},
			{"issue_id": otherIssue},
			{"recipient_id": otherWatcher},
			{"recipient_type": "agent"},
			{"type": "mentioned"},
		} {
			// A retained details field alone is not a matching delivery: every
			// source/recipient/type component must agree with the current event.
			cols := testutil.Cols{
				"workspace_id": f.fx.WorkspaceID, "issue_id": issue,
				"recipient_type": "member", "recipient_id": watcher,
				"type": "new_comment", "title": "Unrelated retained tuple",
				"details": `{"comment_id":"` + comment + `"}`, "read": true, "archived": true,
			}
			for key, value := range change {
				cols[key] = value
			}
			legacyIDs = append(legacyIDs, f.fx.Insert(t, "inbox_item", cols))
		}
		original := map[string]db.InboxItem{}
		for _, id := range legacyIDs {
			row, err := f.queries.GetInboxItem(context.Background(), util.MustParseUUID(id))
			if err != nil {
				t.Fatalf("read unrelated retained tuple: %v", err)
			}
			original[id] = row
		}
		collector := &commentReplayCollector{}
		bus := collector.bus(f.queries)
		event := f.event(t, comment)
		bus.Publish(event)
		bus.Publish(event)
		current := []db.InboxItem{}
		for _, row := range f.items(t, issue) {
			if _, old := original[util.UUIDToString(row.ID)]; !old {
				current = append(current, row)
			}
		}
		if len(current) != 1 || current[0].RecipientType != "member" ||
			util.UUIDToString(current[0].RecipientID) != watcher || current[0].Type != "new_comment" {
			t.Errorf("unrelated retained tuples suppressed/duplicated the exact current delivery: %d new rows", len(current))
		}
		if count := len(collector.snapshot()); count != 1 {
			t.Errorf("exact current delivery published %d inbox:new events, want 1", count)
		}
		for id, before := range original {
			after, err := f.queries.GetInboxItem(context.Background(), util.MustParseUUID(id))
			if err != nil || !reflect.DeepEqual(after, before) {
				t.Errorf("unrelated retained tuple %s changed on replay: %v", id, err)
			}
		}
		checkCommentReplayBroadcasts(t, collector, current)
	})
}

func TestNotificationCommentReplayRecipientControls(t *testing.T) {
	t.Run("ordinary_delivery_tiers", func(t *testing.T) {
		f := newCommentReplayFixture(t)
		parent := f.issue(t, "Parent-only watcher source")
		issue := f.issue(t, "Ordinary child recipient controls", testutil.Cols{"parent_issue_id": parent})
		f.subscribe(t, issue, "member", f.fx.UserID, "commenter")
		want := map[string]bool{}
		for _, reason := range []string{"creator", "assignee", "commenter", "mentioned", "manual", "autopilot"} {
			member := f.member(t, "Direct "+reason)
			f.subscribe(t, issue, "member", member, reason)
			want[member] = true
		}
		f.subscribe(t, issue, "member", f.member(t, "Delegated churn control"), "delegated")
		f.subscribe(t, issue, "member", f.member(t, "Unsubscribed control"), "manual", testutil.Cols{
			"unsubscribed_at": testutil.Raw("now()"), "opt_out_scope": "issue",
		})
		f.member(t, "Never-subscribed control")
		f.subscribe(t, parent, "member", f.member(t, "Parent-only control"), "manual")
		agent := f.fx.Agent(t, "Agent subscriber control", "")
		f.subscribe(t, issue, "agent", agent, "manual")
		muted := f.member(t, "Muted comment control")
		f.subscribe(t, issue, "member", muted, "manual")
		f.fx.Insert(t, "notification_preference", testutil.Cols{
			"workspace_id": f.fx.WorkspaceID, "user_id": muted, "preferences": `{"comments":"muted"}`,
		})
		mentionsMuted := f.member(t, "Muted mentions still receives plain comments")
		f.subscribe(t, issue, "member", mentionsMuted, "manual")
		f.fx.Insert(t, "notification_preference", testutil.Cols{
			"workspace_id": f.fx.WorkspaceID, "user_id": mentionsMuted, "preferences": `{"mentions":"muted"}`,
		})
		want[mentionsMuted] = true
		var subscribersBefore []byte
		f.fx.QueryRow(t, `SELECT jsonb_agg(to_jsonb(s) ORDER BY user_type,user_id)
			FROM issue_subscriber s WHERE issue_id=$1`, issue).Scan(&subscribersBefore)
		comment := f.fx.Comment(t, issue, "Plain comment without mentions")
		collector := &commentReplayCollector{}
		collector.bus(f.queries).Publish(f.event(t, comment))
		rows := f.items(t, issue)
		if len(rows) != len(want) {
			t.Errorf("ordinary recipient controls delivered %d rows, want %d direct unmuted members", len(rows), len(want))
		}
		if count := len(collector.snapshot()); count != len(want) {
			t.Errorf("ordinary recipient controls published %d inbox:new events, want %d", count, len(want))
		}
		seen := map[string]int{}
		for _, row := range rows {
			recipient := util.UUIDToString(row.RecipientID)
			seen[recipient]++
			if !want[recipient] || row.RecipientType != "member" || row.Type != "new_comment" {
				t.Errorf("comment reached an excluded actor/unsubscribed/agent/delegated/nonwatcher/mute tier: %+v", row)
			}
		}
		for recipient := range want {
			if seen[recipient] != 1 {
				t.Errorf("direct unmuted recipient %s has %d rows, want 1", recipient, seen[recipient])
			}
		}
		var subscribersAfter []byte
		f.fx.QueryRow(t, `SELECT jsonb_agg(to_jsonb(s) ORDER BY user_type,user_id)
			FROM issue_subscriber s WHERE issue_id=$1`, issue).Scan(&subscribersAfter)
		if !reflect.DeepEqual(subscribersAfter, subscribersBefore) {
			t.Error("notification delivery changed the original subscriber/tombstone rows")
		}
		if n := f.fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1`, issue); n != 0 {
			t.Errorf("notification-only delivery created %d agent tasks", n)
		}
		checkCommentReplayBroadcasts(t, collector, rows)
	})

	t.Run("current_late_subscriber_and_unsubscribe", func(t *testing.T) {
		f := newCommentReplayFixture(t)
		issue := f.issue(t, "Current subscriber lookup")
		first := f.member(t, "Original subscriber")
		late := f.member(t, "Late subscriber")
		f.subscribe(t, issue, "member", first, "manual")
		comment := f.fx.Comment(t, issue, "Original current-subscriber event")
		event := f.event(t, comment)
		collector := &commentReplayCollector{}
		collector.bus(f.queries).Publish(event)
		original := f.items(t, issue)
		if len(original) != 1 || util.UUIDToString(original[0].RecipientID) != first {
			t.Fatal("first event did not deliver to its original active subscriber")
		}
		if err := f.queries.RemoveIssueSubscriber(context.Background(), db.RemoveIssueSubscriberParams{
			IssueID: util.MustParseUUID(issue), UserType: "member", UserID: util.MustParseUUID(first),
		}); err != nil {
			t.Fatalf("persist original unsubscribe: %v", err)
		}
		f.subscribe(t, issue, "member", late, "manual")
		fresh := collector.bus(db.New(testPool))
		fresh.Publish(event)
		fresh.Publish(event)
		rows := f.items(t, issue)
		if len(rows) != 2 {
			t.Errorf("current late subscriber replay has %d rows, want 2 retained original/late deliveries", len(rows))
		}
		if count := len(collector.snapshot()); count != 2 {
			t.Errorf("original/late subscriber delivery published %d inbox:new events, want 2", count)
		}
		counts := map[string]int{}
		for _, row := range rows {
			counts[util.UUIDToString(row.RecipientID)]++
			if row.ID == original[0].ID && !reflect.DeepEqual(row, original[0]) {
				t.Error("unsubscribe/replay changed the original retained delivery")
			}
		}
		if counts[first] != 1 || counts[late] != 1 {
			t.Errorf("current subscriber delivery counts = %v, want original=1 late=1", counts)
		}
		if err := f.queries.RemoveIssueSubscriber(context.Background(), db.RemoveIssueSubscriberParams{
			IssueID: util.MustParseUUID(issue), UserType: "member", UserID: util.MustParseUUID(late),
		}); err != nil {
			t.Fatalf("persist late unsubscribe: %v", err)
		}
		newComment := f.fx.Comment(t, issue, "Distinct comment after both unsubscribe")
		fresh.Publish(f.event(t, newComment))
		if after := f.items(t, issue); !reflect.DeepEqual(after, rows) {
			t.Error("a distinct event delivered to unsubscribed recipients or changed their retained rows")
		}
		checkCommentReplayBroadcasts(t, collector, rows)
	})
}

func TestNotificationCommentReplayLegacyFallback(t *testing.T) {
	// Absent/nonvalid original identity stays on the existing random-v7 path;
	// these synthetic inputs intentionally have no event replay guarantee.
	for _, tc := range []struct{ name, id string }{
		{"empty_identity", ""},
		{"invalid_identity", "not-a-comment-uuid"},
		{"zero_identity", "00000000-0000-0000-0000-000000000000"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newCommentReplayFixture(t)
			issue := f.issue(t, "Legacy synthetic identity scope")
			f.subscribe(t, issue, "member", f.member(t, "Synthetic identity watcher"), "manual")
			comment := f.fx.Comment(t, issue, "Saved comment behind synthetic control")
			event := commentReplayWithID(f.event(t, comment), tc.id)
			collector := &commentReplayCollector{}
			bus := collector.bus(f.queries)
			bus.Publish(event)
			bus.Publish(event)
			rows := f.items(t, issue)
			if len(rows) != 2 {
				t.Fatalf("legacy identity %q produced %d rows, want 2 ordinary independent writes", tc.id, len(rows))
			}
			for _, row := range rows {
				if uuid.UUID(row.ID.Bytes).Version() != 7 {
					t.Errorf("legacy synthetic inbox ID %s is not an ordinary UUIDv7", util.UUIDToString(row.ID))
				}
				var details map[string]string
				if err := json.Unmarshal(row.Details, &details); err != nil || details["comment_id"] != tc.id {
					t.Errorf("synthetic details changed: %s, error=%v", row.Details, err)
				}
			}
			checkCommentReplayBroadcasts(t, collector, rows)
		})
	}

	t.Run("generic_status_writer_keeps_random_v7", func(t *testing.T) {
		f := newCommentReplayFixture(t)
		issue := f.issue(t, "Generic status source", testutil.Cols{"status": "in_progress"})
		f.subscribe(t, issue, "member", f.member(t, "Generic writer watcher"), "manual")
		event := events.Event{
			Type: protocol.EventIssueUpdated, WorkspaceID: f.fx.WorkspaceID,
			ActorType: "member", ActorID: f.fx.UserID,
			Payload: map[string]any{
				"issue": handler.IssueResponse{ID: issue, WorkspaceID: f.fx.WorkspaceID,
					Title: "Generic status source", Status: "in_progress", CreatorType: "member", CreatorID: f.fx.UserID},
				"status_changed": true, "prev_status": "todo",
			},
		}
		collector := &commentReplayCollector{}
		bus := collector.bus(f.queries)
		bus.Publish(event)
		bus.Publish(event)
		rows := f.items(t, issue)
		if len(rows) != 2 {
			t.Fatalf("generic status writer produced %d rows, want 2 independent writes", len(rows))
		}
		for _, row := range rows {
			if row.Type != "status_changed" || uuid.UUID(row.ID.Bytes).Version() != 7 {
				t.Errorf("generic status notification changed type/UUIDv7 behavior: %+v", row)
			}
		}
		checkCommentReplayBroadcasts(t, collector, rows)
	})
}
