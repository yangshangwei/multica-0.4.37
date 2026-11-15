package migrations

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func knowledgeRollbackJSON(t *testing.T, value any) string {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func knowledgeRollbackCandidate(t *testing.T, fx *testutil.Fixture, sources, evidence string) {
	t.Helper()
	id, issue := uuid.NewString(), uuid.NewString()
	fx.Insert(t, "collaboration_knowledge_candidate", testutil.Cols{
		"id": id, "lineage_id": id, "version": 1, "source_workspace_id": uuid.NewString(), "source_issue_id": issue,
		"target_kind": "workspace", "target_workspace_id": uuid.NewString(), "content": "Retained human knowledge", "summary": "Human proposal",
		"content_sha256": protocol.CollaborationKnowledgeHash("Retained human knowledge"), "sources": sources,
		"evidence": evidence, "applicability": `{"surfaces":["issue"]}`, "proposed_by_user_id": uuid.NewString(),
	})
}

func knowledgeRollbackRuntimePayloads(t *testing.T, version int) map[string]map[string]any {
	t.Helper()
	id := uuid.NewString()
	identity := protocol.CollaborationRunIdentity{DeploymentID: id, InstallationID: id, IssueID: id, SessionID: id,
		TargetID: id, ChannelID: id, BatchID: id, GroupID: id, RunID: id, Fence: "1", ClaimGeneration: "1", LaunchNonce: id}
	request := map[string]any{"protocol_version": version, "identity": identity, "request_id": uuid.NewString(),
		"handoff_id": uuid.NewString(), "invocation_fingerprint": strings.Repeat("a", 64)}
	manifest := protocol.TaskKnowledgeManifest{ProtocolVersion: version, Entries: []protocol.TaskKnowledgeManifestEntry{}}
	manifestHash, err := protocol.TaskKnowledgeRequestHash(manifest)
	if err != nil {
		t.Fatal(err)
	}
	authorization := make(map[string]any)
	for key, value := range request {
		authorization[key] = value
	}
	authorization["manifest_sha256"], authorization["prompt_sha256"] = manifestHash, strings.Repeat("b", 64)
	authorization["attempt_seq"], authorization["attempt_nonce"] = "1", uuid.NewString()
	return map[string]map[string]any{
		"knowledge_claim":         {"protocol_version": version, "identity": identity, "configuration_digest": strings.Repeat("c", 64), "namespace": id},
		"knowledge_preparation":   {"request": request, "manifest": manifest, "manifest_sha256": manifestHash, "targets": []any{}},
		"knowledge_authorization": authorization,
	}
}

func knowledgeRollbackRunEvent(t *testing.T, fx *testutil.Fixture, kind string, payload any) {
	t.Helper()
	key := map[string]string{"knowledge_claim": "knowledge:claim:v1", "knowledge_preparation": "knowledge:prepare:v1", "knowledge_authorization": "knowledge:launch:1"}[kind]
	fx.Insert(t, "collaboration_run_event", testutil.Cols{"issue_id": uuid.NewString(), "run_id": uuid.NewString(),
		"producer_event_id": key, "seq": 1, "kind": kind, "payload": knowledgeRollbackJSON(t, payload)})
}

func knowledgeRollbackOperation(t *testing.T, fx *testutil.Fixture, receipt any) {
	t.Helper()
	fx.Insert(t, "collaboration_operation", testutil.Cols{"scope_kind": "issue", "scope_id": uuid.NewString(),
		"actor_user_id": uuid.NewString(), "request_id": uuid.NewString(), "kind": "knowledge_approve",
		"payload_hash": strings.Repeat("d", 64), "result_code": "knowledge_approved", "result": knowledgeRollbackJSON(t, receipt)})
}

func TestKnowledgeAutomationRollbackRejectsRetainedV2(t *testing.T) {
	seed := map[string]func(*testing.T, *testutil.Fixture){
		"human delivery source": func(t *testing.T, fx *testutil.Fixture) {
			knowledgeRollbackCandidate(t, fx, `[{"kind":"delivery"}]`, `[]`)
		},
		"human delivery evidence": func(t *testing.T, fx *testutil.Fixture) {
			knowledgeRollbackCandidate(t, fx, `[{"kind":"memory"}]`, `[{"kind":"test","source":{"kind":"delivery"}}]`)
		},
		"unknown source kind": func(t *testing.T, fx *testutil.Fixture) {
			knowledgeRollbackCandidate(t, fx, `[{"kind":"future_source"}]`, `[]`)
		},
		"v2 human receipt": func(t *testing.T, fx *testutil.Fixture) {
			knowledgeRollbackOperation(t, fx, map[string]any{"protocol_version": 2})
		},
		"missing receipt version": func(t *testing.T, fx *testutil.Fixture) {
			knowledgeRollbackOperation(t, fx, map[string]any{"code": "knowledge_approved"})
		},
		"unknown runtime version": func(t *testing.T, fx *testutil.Fixture) {
			knowledgeRollbackRunEvent(t, fx, "knowledge_claim", knowledgeRollbackRuntimePayloads(t, 3)["knowledge_claim"])
		},
		"missing runtime version": func(t *testing.T, fx *testutil.Fixture) {
			payload := knowledgeRollbackRuntimePayloads(t, 1)["knowledge_claim"]
			delete(payload, "protocol_version")
			knowledgeRollbackRunEvent(t, fx, "knowledge_claim", payload)
		},
		"malformed runtime object": func(t *testing.T, fx *testutil.Fixture) {
			knowledgeRollbackRunEvent(t, fx, "knowledge_claim", []any{})
		},
		"unknown runtime field": func(t *testing.T, fx *testutil.Fixture) {
			payload := knowledgeRollbackRuntimePayloads(t, 1)["knowledge_claim"]
			payload["future_exposure"] = true
			knowledgeRollbackRunEvent(t, fx, "knowledge_claim", payload)
		},
		"noninteger runtime version": func(t *testing.T, fx *testutil.Fixture) {
			payload := knowledgeRollbackRuntimePayloads(t, 1)["knowledge_claim"]
			payload["protocol_version"] = json.RawMessage(`1.0`)
			knowledgeRollbackRunEvent(t, fx, "knowledge_claim", payload)
		},
		"mismatched preparation manifest version": func(t *testing.T, fx *testutil.Fixture) {
			payload := knowledgeRollbackRuntimePayloads(t, 1)["knowledge_preparation"]
			payload["manifest"] = protocol.TaskKnowledgeManifest{ProtocolVersion: 2, Entries: []protocol.TaskKnowledgeManifestEntry{}}
			knowledgeRollbackRunEvent(t, fx, "knowledge_preparation", payload)
		},
		"validity hidden in v1 manifest": func(t *testing.T, fx *testutil.Fixture) {
			payload := knowledgeRollbackRuntimePayloads(t, 1)["knowledge_preparation"]
			payload["manifest"] = map[string]any{"protocol_version": 1, "entries": []any{map[string]any{
				"candidate_id": uuid.NewString(), "version": "1", "content_sha256": strings.Repeat("a", 64),
				"source_issue_id": uuid.NewString(), "target": map[string]string{"kind": "project", "workspace_id": uuid.NewString(), "project_id": uuid.NewString()},
				"validity": map[string]string{"revision": "1", "accepted_event_seq": "4", "submission_id": uuid.NewString(), "validated_close_epoch": "2"},
			}}}
			knowledgeRollbackRunEvent(t, fx, "knowledge_preparation", payload)
		},
	}
	for _, kind := range []string{"knowledge_claim", "knowledge_preparation", "knowledge_authorization"} {
		seed["v2 "+kind] = func(t *testing.T, fx *testutil.Fixture) {
			knowledgeRollbackRunEvent(t, fx, kind, knowledgeRollbackRuntimePayloads(t, 2)[kind])
		}
	}
	for name, insert := range seed {
		t.Run(name, func(t *testing.T) {
			pool, fx := knowledgeAutomationSchema(t)
			insert(t, fx)
			knowledgeError(t, pool, "55000", `SELECT collaboration_assert_knowledge_automation_empty()`)
			for _, index := range []int{0, 15} {
				path := strings.TrimSuffix(knowledgeAutomationFiles(t)[index], ".up.sql") + ".down.sql"
				raw, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				knowledgeError(t, pool, "55000", string(raw))
			}
			if fx.Count(t, `SELECT count(*) FROM pg_class WHERE relnamespace=current_schema()::regnamespace AND relname='collaboration_knowledge_validity'`) != 1 {
				t.Fatal("refused rollback removed the validity schema")
			}
		})
	}
}

func TestKnowledgeAutomationRollbackPreservesLegacyRuntime(t *testing.T) {
	pool, fx := knowledgeAutomationSchema(t)
	knowledgeCandidate(t, fx)
	for kind, payload := range knowledgeRollbackRuntimePayloads(t, 1) {
		knowledgeRollbackRunEvent(t, fx, kind, payload)
	}
	knowledgeRollbackOperation(t, fx, protocol.CollaborationKnowledgeReceipt{ProtocolVersion: 1, RequestID: uuid.NewString(),
		Action: "approve", Code: "knowledge_approved", CandidateID: uuid.NewString(), Version: "1", StateRevision: "2", State: "approved"})
	fx.Insert(t, "collaboration_run_event", testutil.Cols{"issue_id": uuid.NewString(), "run_id": uuid.NewString(),
		"producer_event_id": "unrelated-business-record", "seq": 1, "kind": "business_result", "payload": `{"protocol_version":2}`})
	var before, after string
	snapshot := `SELECT jsonb_build_object('events',(SELECT jsonb_agg(to_jsonb(e) ORDER BY id) FROM collaboration_run_event e),'operations',(SELECT jsonb_agg(to_jsonb(o) ORDER BY id) FROM collaboration_operation o))::text`
	fx.QueryRow(t, snapshot).Scan(&before)
	files := knowledgeAutomationFiles(t)
	for index := len(files) - 1; index >= 0; index-- {
		knowledgeApply(t, pool, strings.TrimSuffix(files[index], ".up.sql")+".down.sql")
	}
	fx.QueryRow(t, snapshot).Scan(&after)
	if before != after {
		t.Fatal("legacy-only downgrade changed retained history")
	}
}
