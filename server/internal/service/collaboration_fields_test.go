package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/iteration"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func fieldJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func fieldProperty(t *testing.T, f *collaborationQueueFixture, kind string) string {
	t.Helper()
	return f.fx.Insert(t, "issue_property", testutil.Cols{"workspace_id": f.workspace, "name": "Field " + uuid.NewString(), "type": kind})
}

func fieldLabel(t *testing.T, f *collaborationQueueFixture) string {
	t.Helper()
	id := f.fx.Insert(t, "issue_label", testutil.Cols{"workspace_id": f.workspace, "resource_type": "issue", "name": "Label " + uuid.NewString(), "color": "#123456"})
	f.fx.Cleanup(t, `DELETE FROM issue_to_label WHERE issue_id=$1`, f.issue.Issue.ID)
	return id
}

func fieldLabelIDs(ids ...string) *[]pgtype.UUID {
	out := make([]pgtype.UUID, len(ids))
	for i, id := range ids {
		out[i] = util.MustParseUUID(id)
	}
	return &out
}

func TestCollaborationFieldsTypedValuesAndHistory(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	label := fieldLabel(t, f)
	actor := f.fx.User(t, "Source actor reference", "field-actor-"+uuid.NewString()+"@example.test")
	f.fx.Member(t, f.workspace, actor, "member")
	first, second := uuid.NewString(), strings.ToUpper(uuid.NewString())
	patch := make(map[string]json.RawMessage)
	expected := make(map[string]json.RawMessage)
	for _, tc := range []struct {
		kind          string
		input, stored any
	}{
		{"text", "A\x00 note", "A note"}, {"number", 12.5, 12.5}, {"checkbox", false, false},
		{"url", " https://example.test/work ", "https://example.test/work"}, {"date", "2026-10-14", "2026-10-14"},
		{"select", second, second}, {"multi_select", []string{second, first, second}, []string{first, second}},
		{"actor", "member:" + strings.ToUpper(actor), "member:" + actor},
		{"multi_actor", []string{"member:" + actor, "member:" + f.creator, "member:" + strings.ToUpper(actor)}, []string{"member:" + actor, "member:" + f.creator}},
	} {
		id := fieldProperty(t, f, tc.kind)
		if tc.kind == "select" || tc.kind == "multi_select" {
			config := util.IssuePropertyConfig{Options: []util.IssuePropertyOption{{ID: first, Name: "First", Color: "#112233"}, {ID: second, Name: "Second", Color: "#334455"}}}
			f.fx.Exec(t, `UPDATE issue_property SET config=$2::jsonb WHERE id=$1`, id, fieldJSON(t, config))
		}
		patch[strings.ToUpper(id)] = fieldJSON(t, tc.input)
		expected[id] = fieldJSON(t, tc.stored)
	}
	untouched := fieldProperty(t, f, "number")
	f.fx.Exec(t, `UPDATE issue SET properties=jsonb_build_object($2::text,9007199254740993::bigint) WHERE id=$1`, f.issue.Issue.ID, untouched)
	expected[untouched] = json.RawMessage(`9007199254740993`)
	command := f.submit(t, f.handler, "immutable field-edit input")
	beforeNotifications := f.fx.Count(t, `SELECT count(*) FROM collaboration_notification WHERE issue_id=$1`, f.issue.Issue.ID)
	p := CollaborationEditParams{CollaborationWorkflowRequest: workflowRequest(f, 1), Priority: editString("urgent"), LabelIDs: fieldLabelIDs(label, label), Properties: patch}
	r, err := f.svc.EditIssue(collaborationContext(f.handler), p)
	if err != nil || r.Code != "issue_edited" || r.CollaborationRevision != 2 {
		t.Fatalf("external handler field edit: %+v %v", r, err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND priority='urgent' AND properties=$2::jsonb AND revision=$3`, f.issue.Issue.ID, fieldJSON(t, expected), f.issue.Issue.Revision+1) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM issue_to_label WHERE issue_id=$1 AND label_id=$2`, f.issue.Issue.ID, label) != 1 {
		t.Fatal("typed fields were not stored canonically as one edit")
	}
	var payload []byte
	f.fx.QueryRow(t, `SELECT payload FROM collaboration_event WHERE issue_id=$1 AND kind='issue_edited' AND actor_user_id=$2`, f.issue.Issue.ID, f.handler).Scan(&payload)
	var event struct {
		Before, After struct {
			Priority   string
			Labels     []CollaborationFieldLabel
			Properties map[string]json.RawMessage
		}
	}
	if err := json.Unmarshal(payload, &event); err != nil {
		t.Fatal(err)
	}
	if event.Before.Priority != f.issue.Issue.Priority || len(event.Before.Labels) != 0 || len(event.Before.Properties) != 1 ||
		event.After.Priority != "urgent" || len(event.After.Labels) != 1 || event.After.Labels[0].ID != label || len(event.After.Properties) != len(expected) {
		t.Fatalf("field history lost its before/after projection: %s", payload)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND content='immutable field-edit input' AND state='waiting' AND author_user_id=$2`, command.CommandID, f.handler) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_notification WHERE issue_id=$1`, f.issue.Issue.ID) != beforeNotifications ||
		f.fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1`, f.issue.Issue.ID) != 0 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_session WHERE id=$1 AND queue_state='paused' AND control_version=1`, f.issue.SessionID) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM member WHERE workspace_id=$1 AND user_id=$2`, f.workspace, f.handler) != 0 {
		t.Fatal("field edit changed execution, notifications, resources or membership")
	}
	if replay, err := f.svc.EditIssue(collaborationContext(f.handler), p); err != nil || replay != r {
		t.Fatalf("same-key field replay: %+v %v", replay, err)
	}
}

func TestCollaborationFieldsNoopClearAndRetainedReferences(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	a, b := fieldLabel(t, f), fieldLabel(t, f)
	number, archived := fieldProperty(t, f, "number"), fieldProperty(t, f, "actor")
	f.fx.InsertNoID(t, "issue_to_label", testutil.Cols{"issue_id": f.issue.Issue.ID, "label_id": a}, "issue_id=$1 AND label_id=$2", f.issue.Issue.ID, a)
	f.fx.InsertNoID(t, "issue_to_label", testutil.Cols{"issue_id": f.issue.Issue.ID, "label_id": b}, "issue_id=$1 AND label_id=$2", f.issue.Issue.ID, b)
	f.fx.Exec(t, `UPDATE issue_property SET archived_at=now() WHERE id=$1`, archived)
	f.fx.Exec(t, `UPDATE issue SET properties=$2::jsonb WHERE id=$1`, f.issue.Issue.ID, fieldJSON(t, map[string]any{number: 1, archived: "member:" + f.outsider}))
	for _, p := range []CollaborationEditParams{
		{CollaborationWorkflowRequest: workflowRequest(f, 1), Properties: map[string]json.RawMessage{}},
		{CollaborationWorkflowRequest: workflowRequest(f, 1), Priority: editString(f.issue.Issue.Priority), LabelIDs: fieldLabelIDs(b, a, a), Properties: map[string]json.RawMessage{number: json.RawMessage(`1.0`)}},
	} {
		if r, err := f.svc.EditIssue(f.ctx, p); err != nil || r.Code != "unchanged" || r.CollaborationRevision != 1 {
			t.Fatalf("equivalent field state advanced revision: %+v %v", r, err)
		}
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1 AND kind='issue_edited'`, f.issue.Issue.ID) != 0 ||
		f.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND revision=$2`, f.issue.Issue.ID, f.issue.Issue.Revision) != 1 {
		t.Fatal("field no-op wrote issue/history")
	}
	access, err := f.svc.ResolveIssueAccess(f.ctx, nil, f.issue.Issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	fields, err := f.svc.IssueFields(f.ctx, access)
	if err != nil || len(fields.PropertyDefinitions) != 2 || len(fields.Properties) != 2 || len(fields.Labels) != 2 {
		t.Fatalf("retained archived/departed actor references are unreadable: %+v %v", fields, err)
	}
	p := CollaborationEditParams{CollaborationWorkflowRequest: workflowRequest(f, 1), LabelIDs: fieldLabelIDs(), Properties: map[string]json.RawMessage{archived: json.RawMessage(`null`)}}
	if r, err := f.svc.EditIssue(f.ctx, p); err != nil || r.CollaborationRevision != 2 {
		t.Fatalf("clear archived value and all labels: %+v %v", r, err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM issue_to_label WHERE issue_id=$1`, f.issue.Issue.ID) != 0 ||
		f.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND properties=jsonb_build_object($2::text,1) AND revision=$3`, f.issue.Issue.ID, number, f.issue.Issue.Revision+1) != 1 {
		t.Fatal("clearing selected fields overwrote an unrelated property")
	}
	// Preserve and clear must have distinct request identities.
	p.LabelIDs = nil
	if _, err := f.svc.EditIssue(f.ctx, p); !collaborationErrorCode(err, "idempotency_conflict") {
		t.Fatalf("clear/preserve label digest collapsed: %v", err)
	}
}

func TestCollaborationFieldsRejectInvalidSourceAndValues(t *testing.T) {
	for _, scenario := range []string{"priority", "invalid_label", "missing_label", "foreign_label", "agent_label", "missing_property", "foreign_property", "foreign_clear", "archived", "bad_property_key", "duplicate_property_identity", "text", "number", "checkbox", "date", "url", "select", "multi_select", "actor", "multi_actor", "foreign_actor", "oversized"} {
		t.Run(scenario, func(t *testing.T) {
			f := newCollaborationQueueFixture(t)
			label := fieldLabel(t, f)
			property := fieldProperty(t, f, "text")
			p := CollaborationEditParams{CollaborationWorkflowRequest: workflowRequest(f, 1), Title: editString("Must roll back"), Priority: editString("urgent"), LabelIDs: fieldLabelIDs(label), Properties: map[string]json.RawMessage{property: fieldJSON(t, "Valid private value")}}
			code := "invalid_property_value"
			switch scenario {
			case "priority":
				p.Priority, code = editString("private-priority"), "invalid_priority"
			case "invalid_label":
				p.LabelIDs, code = &[]pgtype.UUID{{}}, "invalid_labels"
			case "missing_label":
				p.LabelIDs, code = fieldLabelIDs(uuid.NewString()), "invalid_labels"
			case "foreign_label", "agent_label":
				code = "invalid_labels"
				if scenario == "foreign_label" {
					other := f.fx.Workspace(t, "Other label source", "field-label-"+uuid.NewString())
					f.fx.Exec(t, `UPDATE issue_label SET workspace_id=$2 WHERE id=$1`, label, other)
				} else {
					f.fx.Exec(t, `UPDATE issue_label SET resource_type='agent' WHERE id=$1`, label)
				}
			case "missing_property":
				p.Properties, code = map[string]json.RawMessage{uuid.NewString(): json.RawMessage(`true`)}, "invalid_properties"
			case "foreign_property", "foreign_clear":
				f.fx.Exec(t, `UPDATE issue_property SET workspace_id=$2 WHERE id=$1`, property, uuid.NewString())
				code = "invalid_properties"
				if scenario == "foreign_clear" {
					p.Properties[property] = json.RawMessage(`null`)
				}
			case "archived":
				f.fx.Exec(t, `UPDATE issue_property SET archived_at=now() WHERE id=$1`, property)
				code = "property_archived"
			case "bad_property_key":
				p.Properties, code = map[string]json.RawMessage{"private-bad-id": json.RawMessage(`true`)}, "invalid_properties"
			case "duplicate_property_identity":
				p.Properties[strings.ToUpper(property)] = json.RawMessage(`false`)
				code = "invalid_properties"
			case "text":
				p.Properties[property] = json.RawMessage(`"  "`)
			case "number", "checkbox", "date", "url", "actor", "multi_actor":
				f.fx.Exec(t, `UPDATE issue_property SET type=$2 WHERE id=$1`, property, scenario)
				p.Properties[property] = fieldJSON(t, "private-malformed-value")
			case "select", "multi_select":
				f.fx.Exec(t, `UPDATE issue_property SET type=$2,config=$3::jsonb WHERE id=$1`, property, scenario, fieldJSON(t, util.IssuePropertyConfig{Options: []util.IssuePropertyOption{{ID: uuid.NewString(), Name: "Private option", Color: "#123456"}}}))
				p.Properties[property] = fieldJSON(t, "private-invalid-option")
			case "foreign_actor":
				f.fx.Exec(t, `UPDATE issue_property SET type='actor' WHERE id=$1`, property)
				p.Properties[property] = fieldJSON(t, "member:"+f.handler)
			case "oversized":
				for range 10 {
					id := fieldProperty(t, f, "text")
					p.Properties[id] = fieldJSON(t, strings.Repeat("x", 1900))
				}
				code = "properties_too_large"
			}
			r, err := f.svc.EditIssue(f.ctx, p)
			if !collaborationErrorCode(err, code) || r.Code != code || r.CollaborationRevision != 1 {
				t.Fatalf("%s did not refuse with a durable safe code: %+v %v", scenario, r, err)
			}
			if f.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND title=$2 AND priority=$3 AND properties='{}'::jsonb AND revision=$4`, f.issue.Issue.ID, f.issue.Issue.Title, f.issue.Issue.Priority, f.issue.Issue.Revision) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM issue_to_label WHERE issue_id=$1`, f.issue.Issue.ID) != 0 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1 AND kind='issue_edited'`, f.issue.Issue.ID) != 0 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_operation WHERE scope_id=$1 AND request_id=$2 AND completed_at IS NOT NULL AND result_code=$3`, f.issue.Issue.ID, p.RequestID, code) != 1 {
				t.Fatal("invalid fields partially changed metadata, labels, history or lost the refusal")
			}
			encoded := fieldJSON(t, r)
			if strings.Contains(string(encoded), "private-") {
				t.Fatal("private validation values entered a receipt")
			}
		})
	}
}

func TestCollaborationFieldsLabelRequiredWritesRollback(t *testing.T) {
	for _, needle := range []string{"DELETE FROM issue_to_label", "INSERT INTO issue_to_label"} {
		t.Run(needle, func(t *testing.T) {
			f := newCollaborationQueueFixture(t)
			old, next := fieldLabel(t, f), fieldLabel(t, f)
			property := fieldProperty(t, f, "checkbox")
			f.fx.InsertNoID(t, "issue_to_label", testutil.Cols{"issue_id": f.issue.Issue.ID, "label_id": old}, "issue_id=$1 AND label_id=$2", f.issue.Issue.ID, old)
			p := CollaborationEditParams{CollaborationWorkflowRequest: workflowRequest(f, 1), Priority: editString("high"), LabelIDs: fieldLabelIDs(next), Properties: map[string]json.RawMessage{property: json.RawMessage(`true`)}}
			f.svc.TxStarter = workflowFaultStarter{TxStarter: f.pool, needle: needle}
			if _, err := f.svc.EditIssue(f.ctx, p); err == nil {
				t.Fatal("required field write failure reported success")
			}
			if f.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND priority=$2 AND properties='{}'::jsonb AND revision=$3`, f.issue.Issue.ID, f.issue.Issue.Priority, f.issue.Issue.Revision) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM issue_to_label WHERE issue_id=$1 AND label_id=$2`, f.issue.Issue.ID, old) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM issue_to_label WHERE issue_id=$1 AND label_id=$2`, f.issue.Issue.ID, next) != 0 ||
				f.fx.Count(t, `SELECT count(*) FROM issue_collaboration WHERE issue_id=$1 AND revision=1`, f.issue.Issue.ID) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1 AND kind='issue_edited'`, f.issue.Issue.ID) != 0 ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_operation WHERE scope_id=$1 AND request_id=$2`, f.issue.Issue.ID, p.RequestID) != 0 {
				t.Fatal("field failure left a partial value, label replacement, revision or receipt")
			}
		})
	}
}

func TestCollaborationFieldsPriorMetadataReceiptCompatibility(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	p := editParams(f, 1)
	p.Priority = nil
	// This is the exact pre-R42 digest shape, not a marshal of the new type.
	legacy := struct {
		CollaborationWorkflowRequest
		Title, Description, StartDate, DueDate *string
		SetStartDate, SetDueDate               bool
	}{CollaborationWorkflowRequest: p.CollaborationWorkflowRequest, Title: p.Title}
	digest := sha256.Sum256(fieldJSON(t, legacy))
	receipt := CollaborationWorkflowReceipt{Status: http.StatusOK, Code: "issue_edited", CollaborationRevision: 2, QueueState: "paused", ControlVersion: 1}
	f.fx.Insert(t, "collaboration_operation", testutil.Cols{"scope_kind": "issue", "scope_id": f.issue.Issue.ID, "actor_user_id": f.creator,
		"request_id": p.RequestID, "kind": "edit_issue", "payload_hash": hex.EncodeToString(digest[:]), "result_code": receipt.Code,
		"result": fieldJSON(t, receipt), "resolved_issue_id": f.issue.Issue.ID, "completed_at": time.Now()})
	got, err := f.svc.EditIssue(f.ctx, p)
	if err != nil || got != receipt || f.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND title=$2`, f.issue.Issue.ID, f.issue.Issue.Title) != 1 {
		t.Fatalf("new optional fields invalidated or reapplied an original metadata key: %+v %v", got, err)
	}
}

func TestCollaborationFieldsOriginalKeyRecoveryAndDurableRefusal(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	property := fieldProperty(t, f, "checkbox")
	p := CollaborationEditParams{CollaborationWorkflowRequest: workflowRequest(f, 1), Priority: editString("low"), Properties: map[string]json.RawMessage{property: json.RawMessage(`true`)}}
	fault := &queueFaultStarter{TxStarter: f.pool, lostCommit: true}
	f.svc.TxStarter = fault
	if _, err := f.svc.EditIssue(f.ctx, p); err == nil || fault.begins.Load() != 1 {
		t.Fatalf("unknown field commit was retried or claimed successful: %v", err)
	}
	f.svc.TxStarter = f.pool
	replayed, err := f.svc.EditIssue(f.ctx, p)
	if err != nil || replayed.CollaborationRevision != 2 || f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1 AND kind='issue_edited'`, f.issue.Issue.ID) != 1 {
		t.Fatalf("original key did not recover exactly one field change: %+v %v", replayed, err)
	}
	f.fx.Exec(t, `UPDATE issue_property SET archived_at=now() WHERE id=$1`, property)
	p.CollaborationWorkflowRequest = workflowRequest(f, 2)
	p.Properties[property] = json.RawMessage(`false`)
	refused, err := f.svc.EditIssue(f.ctx, p)
	if !collaborationErrorCode(err, "property_archived") {
		t.Fatalf("archived write: %+v %v", refused, err)
	}
	f.fx.Exec(t, `UPDATE issue_property SET archived_at=NULL WHERE id=$1`, property)
	got, err := f.svc.EditIssue(f.ctx, p)
	if !collaborationErrorCode(err, "property_archived") || got != refused ||
		f.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND properties=jsonb_build_object($2::text,true)`, f.issue.Issue.ID, property) != 1 {
		t.Fatalf("definition restoration revived refused intent: %+v %v", got, err)
	}
}

// Pause only after a real failed attempt has fully rolled back. The competing
// writer can then commit without any source locks held by the retrying edit.
type fieldRetryGate struct {
	TxStarter
	ready, release chan struct{}
	once           sync.Once
	begins         atomic.Int32
}

type fieldRetryTx struct {
	pgx.Tx
	gate *fieldRetryGate
}

func (s *fieldRetryGate) Begin(ctx context.Context) (pgx.Tx, error) {
	s.begins.Add(1)
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &fieldRetryTx{Tx: tx, gate: s}, nil
}

func (t *fieldRetryTx) Rollback(ctx context.Context) error {
	err := t.Tx.Rollback(ctx)
	t.gate.once.Do(func() {
		close(t.gate.ready)
		select {
		case <-t.gate.release:
		case <-ctx.Done():
		}
	})
	return err
}

func TestCollaborationFieldsRecheckAfterSourceDefinitionContention(t *testing.T) {
	for _, scenario := range []string{"option_advisory", "definition_row", "member_row", "label_row"} {
		t.Run(scenario, func(t *testing.T) {
			f := newCollaborationQueueFixture(t)
			property := fieldProperty(t, f, "text")
			label := fieldLabel(t, f)
			p := CollaborationEditParams{CollaborationWorkflowRequest: workflowRequest(f, 1), Priority: editString("urgent"), LabelIDs: fieldLabelIDs(label), Properties: map[string]json.RawMessage{property: fieldJSON(t, "value")}}
			ctx, cancel := context.WithTimeout(f.ctx, 10*time.Second)
			defer cancel()
			tx, err := f.pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(context.Background())
			want := "property_archived"
			switch scenario {
			case "option_advisory":
				option := uuid.NewString()
				config := util.IssuePropertyConfig{Options: []util.IssuePropertyOption{{ID: option, Name: "Before", Color: "#112233"}}}
				f.fx.Exec(t, `UPDATE issue_property SET type='select',config=$2::jsonb WHERE id=$1`, property, fieldJSON(t, config))
				p.Properties[property] = fieldJSON(t, option)
				_, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "props:"+f.workspace)
				if err == nil {
					_, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "prop:"+property)
				}
				config.Options[0].ID = uuid.NewString()
				if err == nil {
					_, err = tx.Exec(ctx, `UPDATE issue_property SET config=$2::jsonb WHERE id=$1`, property, fieldJSON(t, config))
				}
				want = "invalid_property_value"
			case "definition_row":
				_, err = tx.Exec(ctx, `UPDATE issue_property SET archived_at=now() WHERE id=$1`, property)
			case "member_row":
				f.fx.Member(t, f.workspace, f.outsider, "member")
				f.fx.Exec(t, `UPDATE issue_property SET type='actor' WHERE id=$1`, property)
				p.Properties[property] = fieldJSON(t, "member:"+f.outsider)
				_, err = tx.Exec(ctx, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, f.workspace, f.outsider)
				want = "invalid_property_value"
			case "label_row":
				_, err = tx.Exec(ctx, `DELETE FROM issue_label WHERE id=$1`, label)
				want = "invalid_labels"
			}
			if err != nil {
				t.Fatal(err)
			}
			gate := &fieldRetryGate{TxStarter: f.pool, ready: make(chan struct{}), release: make(chan struct{})}
			f.svc.TxStarter = gate
			done := make(chan error, 1)
			go func() { _, err := f.svc.EditIssue(ctx, p); done <- err }()
			select {
			case <-gate.ready:
			case <-ctx.Done():
				t.Fatal("field lock did not fail without waiting")
			}
			if f.fx.Count(t, `SELECT count(*) FROM collaboration_operation WHERE scope_id=$1 AND request_id=$2`, f.issue.Issue.ID, p.RequestID) != 0 {
				t.Fatal("contention left a committed partial receipt")
			}
			if err = tx.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			close(gate.release)
			if err = <-done; !collaborationErrorCode(err, want) || gate.begins.Load() != 2 {
				t.Fatalf("retry did not revalidate changed source state: %v (%d attempts)", err, gate.begins.Load())
			}
			if f.fx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND priority=$2 AND properties='{}'::jsonb`, f.issue.Issue.ID, f.issue.Issue.Priority) != 1 ||
				f.fx.Count(t, `SELECT count(*) FROM issue_to_label WHERE issue_id=$1`, f.issue.Issue.ID) != 0 {
				t.Fatal("source race committed stale field values")
			}
		})
	}
}

func TestCollaborationFieldsBusyIsBoundedAndLabelFenceIsCurrent(t *testing.T) {
	t.Run("property_busy", func(t *testing.T) {
		f := newCollaborationQueueFixture(t)
		property := fieldProperty(t, f, "checkbox")
		tx, err := f.pool.Begin(f.ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(f.ctx)
		if _, err = tx.Exec(f.ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "prop:"+property); err != nil {
			t.Fatal(err)
		}
		counted := &queueFaultStarter{TxStarter: f.pool}
		f.svc.TxStarter = counted
		p := CollaborationEditParams{CollaborationWorkflowRequest: workflowRequest(f, 1), Properties: map[string]json.RawMessage{property: json.RawMessage(`true`)}}
		ctx, cancel := context.WithTimeout(f.ctx, 3*time.Second)
		defer cancel()
		if _, err = f.svc.EditIssue(ctx, p); !collaborationErrorCode(err, "collaboration_busy") || counted.begins.Load() != 3 {
			t.Fatalf("property contention was not bounded to three rolled-back attempts: %v (%d)", err, counted.begins.Load())
		}
		if f.fx.Count(t, `SELECT count(*) FROM collaboration_operation WHERE scope_id=$1 AND request_id=$2`, f.issue.Issue.ID, p.RequestID) != 0 {
			t.Fatal("busy operation left a partial receipt")
		}
	})
	t.Run("source_label_fence", func(t *testing.T) {
		f := newCollaborationQueueFixture(t)
		label := fieldLabel(t, f)
		tx, err := f.pool.Begin(f.ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(f.ctx)
		if err = iteration.LockWorkspace(f.ctx, tx, util.MustParseUUID(f.workspace)); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(f.ctx, 10*time.Second)
		defer cancel()
		done := make(chan error, 1)
		p := CollaborationEditParams{CollaborationWorkflowRequest: workflowRequest(f, 1), LabelIDs: fieldLabelIDs(label)}
		go func() { _, err := f.svc.EditIssue(ctx, p); done <- err }()
		waitWorkflowBlocked(t, f, int(tx.Conn().PgConn().PID()), done)
		if _, err = db.New(tx).DeleteLabel(ctx, db.DeleteLabelParams{ID: util.MustParseUUID(label), WorkspaceID: util.MustParseUUID(f.workspace)}); err != nil {
			t.Fatal(err)
		}
		if err = tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		if err = <-done; !collaborationErrorCode(err, "invalid_labels") {
			t.Fatalf("label edit ignored catalog deletion under I1 fence: %v", err)
		}
	})
}

func TestCollaborationFieldOptionsAuthorityAndProjection(t *testing.T) {
	f := newCollaborationQueueFixture(t)
	label := fieldLabel(t, f)
	property, archived := fieldProperty(t, f, "text"), fieldProperty(t, f, "checkbox")
	f.fx.Exec(t, `UPDATE issue_property SET archived_at=now() WHERE id=$1`, archived)
	f.fx.Insert(t, "issue_label", testutil.Cols{"workspace_id": f.workspace, "resource_type": "agent", "name": "Private agent label", "color": "#112233"})
	f.fx.Insert(t, "issue_property", testutil.Cols{"workspace_id": uuid.NewString(), "type": "text", "name": "Private foreign definition"})
	for _, actor := range []string{f.creator, f.handler} {
		got, err := f.svc.FieldOptions(collaborationContext(actor), f.issue.Issue.ID)
		if err != nil || got.IssueID != util.UUIDToString(f.issue.Issue.ID) || got.SourceWorkspaceID != f.workspace || got.CollaborationRevision != 1 ||
			len(got.Labels) != 1 || got.Labels[0].ID != label || len(got.Properties) != 1 || got.Properties[0].ID != property || got.Properties[0].Archived {
			t.Fatalf("source editing options: %+v %v", got, err)
		}
	}
	f.fx.Member(t, f.workspace, f.outsider, "owner")
	if _, err := f.svc.FieldOptions(collaborationContext(f.outsider), f.issue.Issue.ID); !collaborationErrorCode(err, "forbidden") {
		t.Fatalf("source owner browsed editing options: %v", err)
	}
	f.fx.Cleanup(t, `DELETE FROM issue_handoff WHERE issue_id=$1`, f.issue.Issue.ID)
	transfer := CollaborationTransferParams{CollaborationParticipantParams: CollaborationParticipantParams{CollaborationWorkflowRequest: workflowRequest(f, 1), TargetUserID: util.MustParseUUID(f.creator)}}
	if _, err := f.svc.Transfer(f.ctx, transfer); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.FieldOptions(collaborationContext(f.handler), f.issue.Issue.ID); !collaborationErrorCode(err, "forbidden") {
		t.Fatalf("former handler retained editing catalog access: %v", err)
	}
	f.fx.Exec(t, `UPDATE issue_participant SET revoked_at=now() WHERE issue_id=$1 AND user_id=$2`, f.issue.Issue.ID, f.handler)
	if _, err := f.svc.FieldOptions(collaborationContext(f.handler), f.issue.Issue.ID); !collaborationErrorCode(err, "not_found") {
		t.Fatalf("revoked external actor read editing catalog: %v", err)
	}
	for _, principal := range []auth.AuthenticatedPrincipal{{UserID: f.creator, Kind: auth.CredentialTask}, {UserID: f.creator, Kind: auth.CredentialCloud}, {UserID: f.creator}, {UserID: f.creator, Kind: auth.CredentialJWT, AgentActor: true}, {UserID: f.creator, Kind: auth.CredentialJWT, Restricted: true}} {
		ctx := auth.WithAuthenticatedPrincipal(f.ctx, principal)
		if _, err := (&CollaborationService{}).FieldOptions(ctx, f.issue.Issue.ID); !collaborationErrorCode(err, "not_found") {
			t.Fatalf("nonhuman/restricted field options: %v", err)
		}
	}
	// Response projections cannot accidentally grow into the ordinary catalog DTO.
	got, err := f.svc.FieldOptions(f.ctx, f.issue.Issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	var encoded map[string]any
	if err := json.Unmarshal(fieldJSON(t, got), &encoded); err != nil {
		t.Fatal(err)
	}
	if len(encoded) != 5 || !reflect.DeepEqual(got.Properties[0].Config, util.IssuePropertyConfig{}) {
		t.Fatalf("options projection grew management metadata: %+v", encoded)
	}
}
