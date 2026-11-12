package realtime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/multica-ai/multica/server/internal/auth"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestKnowledgeEnvelopeRejectsEveryAmbiguousOrPrivateOuterField(t *testing.T) {
	const payload = `{"target_kind":"workspace","target_id":"b736820a-476b-4e30-bbbf-e8f046c54562","knowledge_revision":"1"}`
	const eventID = `"e8f208a2-04c0-47c2-9cd1-ed69d213f8c3:target"`
	for name, raw := range map[string]string{
		"private_outer":        `{"type":"collaboration:knowledge_changed","payload":` + payload + `,"source_id":"PRIVATE_SOURCE"}`,
		"duplicate_payload":    `{"type":"collaboration:knowledge_changed","payload":{"source_id":"PRIVATE_SOURCE"},"payload":` + payload + `,"event_id":` + eventID + `}`,
		"duplicate_type_first": `{"type":"collaboration:knowledge_changed","type":"other","payload":` + payload + `}`,
		"duplicate_type_last":  `{"type":"other","type":"collaboration:knowledge_changed","payload":` + payload + `}`,
		"case_alias":           `{"Type":"collaboration:knowledge_changed","payload":` + payload + `}`,
		"payload_alias":        `{"type":"collaboration:knowledge_changed","Payload":` + payload + `}`,
		"duplicate_event_id":   `{"type":"collaboration:knowledge_changed","payload":` + payload + `,"event_id":` + eventID + `,"event_id":` + eventID + `}`,
		"private_event_id":     `{"type":"collaboration:knowledge_changed","payload":` + payload + `,"event_id":"PRIVATE_SOURCE"}`,
		"null_event_id":        `{"type":"collaboration:knowledge_changed","payload":` + payload + `,"event_id":null}`,
		"missing_payload":      `{"type":"collaboration:knowledge_changed"}`,
		"malformed":            `{"type":"collaboration:knowledge_changed","payload":`,
		"trailing_json":        `{"type":"collaboration:knowledge_changed","payload":` + payload + `}{"source_id":"PRIVATE_SOURCE"}`,
		"invalid_utf8":         "{\"type\":\"collaboration:knowledge_changed\",\"payload\":" + payload + ",\"source_id\":\"\xff\"}",
	} {
		t.Run(name, func(t *testing.T) {
			out, _, err := canonicalKnowledgeFrame([]byte(raw))
			if err == nil || out != nil || strings.Contains(err.Error(), "PRIVATE") {
				t.Fatal("private/ambiguous knowledge envelope was accepted or disclosed in diagnostics")
			}
		})
	}
}

func TestKnowledgeEnvelopeCanonicalizesWithoutMutatingSharedBytes(t *testing.T) {
	raw := []byte(` { "event_id":"e8f208a2-04c0-47c2-9cd1-ed69d213f8c3:target", "payload": {"knowledge_revision":"3","target_id":"b736820a-476b-4e30-bbbf-e8f046c54562","target_kind":"project"}, "type":"collaboration:knowledge_changed" } `)
	original := bytes.Clone(raw)
	out, knowledge, err := canonicalKnowledgeFrame(raw)
	if err != nil || !knowledge || bytes.Equal(raw, out) || !bytes.Equal(raw, original) {
		t.Fatal("knowledge envelope was not canonicalized into separate bytes")
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(out, &fields) != nil || len(fields) != 3 || string(fields["event_id"]) != `"e8f208a2-04c0-47c2-9cd1-ed69d213f8c3:target"` {
		t.Fatal("canonical envelope changed routing identity")
	}
	for _, ordinary := range []string{` {"type":"collaboration:workbench_changed","payload":{}} `, `{"type":"other","payload":{"type":"collaboration:knowledge_changed"},"ordinary_field":"retained"}`} {
		out, knowledge, err = canonicalKnowledgeFrame([]byte(ordinary))
		if err != nil || knowledge || string(out) != ordinary {
			t.Fatal("valid unrelated event changed")
		}
	}
}

type knowledgeAuthFailureDB struct {
	db.DBTX
	err error
}

func (q knowledgeAuthFailureDB) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, q.err
}
func (q knowledgeAuthFailureDB) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, q.err
}
func (q knowledgeAuthFailureDB) QueryRow(context.Context, string, ...any) pgx.Row {
	return knowledgeAuthFailureRow{q.err}
}

type knowledgeAuthFailureRow struct{ err error }

func (r knowledgeAuthFailureRow) Scan(...any) error { return r.err }

func TestKnowledgeOriginalCredentialValidationFailsClosed(t *testing.T) {
	t.Setenv("MULTICA_AUTH_MODE", "legacy")
	hub := NewHub()
	if err := hub.validateKnowledgeCredential(context.Background(), makeTestToken(t), testUserID); err != nil {
		t.Fatal("valid original JWT refused")
	}
	if err := hub.validateKnowledgeCredential(context.Background(), makeTestToken(t), "different-subject"); err == nil {
		t.Fatal("JWT changed its authenticated subject")
	}
	if err := hub.validateKnowledgeCredential(context.Background(), "mul_private_test_token", testUserID); err == nil {
		t.Fatal("PAT allowed without uncached database")
	}
	fault := errors.New("database temporarily unavailable")
	hub.SetPasswordQueries(db.New(knowledgeAuthFailureDB{err: fault}))
	if err := hub.validateKnowledgeCredential(context.Background(), "mul_private_test_token", testUserID); !errors.Is(err, fault) {
		t.Fatal("uncached credential lookup failure was not propagated")
	}
	t.Setenv("MULTICA_AUTH_MODE", "password")
	if err := hub.validateKnowledgeCredential(context.Background(), makeTestToken(t), testUserID); !errors.Is(err, auth.ErrPasswordSession) {
		t.Fatal("legacy JWT bypassed required password session claims")
	}
	if err := hub.validateKnowledgeCredential(context.Background(), "mul_private_test_token", testUserID); !errors.Is(err, fault) {
		t.Fatal("password PAT fell back to stale legacy authorization")
	}
}

func TestKnowledgeMissingFreshnessCapabilityDropsOnlyKnowledge(t *testing.T) {
	const workspace = "b736820a-476b-4e30-bbbf-e8f046c54562"
	hub := NewHub()
	hub.SetAuthorizer(&knowledgeScopeAccess{allowed: true})
	client := &Client{hub: hub, workspaceID: workspace, userID: "viewer", ctx: context.Background()}
	message := outboundMessage{scope: scopeKey{ScopeWorkspace, workspace}, data: []byte(`{"type":"collaboration:knowledge_changed","payload":{"target_kind":"workspace","target_id":"b736820a-476b-4e30-bbbf-e8f046c54562","knowledge_revision":"1"}}`)}
	if _, allowed := client.outboundKnowledgeFrame(message); allowed {
		t.Fatal("missing original credential validator allowed knowledge")
	}
	client.authorizeKnowledge = func(context.Context) error { return errors.New("validator unavailable") }
	if _, allowed := client.outboundKnowledgeFrame(message); allowed {
		t.Fatal("validator failure allowed knowledge")
	}
	message.data = []byte(`{"type":"ordinary","payload":{}}`)
	if out, allowed := client.outboundKnowledgeFrame(message); !allowed || !bytes.Equal(out, message.data) {
		t.Fatal("knowledge freshness failure changed unrelated traffic")
	}
}
