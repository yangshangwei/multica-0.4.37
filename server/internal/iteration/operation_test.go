package iteration

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func operationTestKey() OperationKey {
	return OperationKey{WorkspaceID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", ActorUserID: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", RequestID: "cccccccc-cccc-4ccc-8ccc-cccccccccccc", Operation: "delete"}
}

func TestOperationHashBindsIdentityIntentAndPayload(t *testing.T) {
	key := operationTestKey()
	base, err := key.PayloadHash(map[string]any{"name": " name\r\n"})
	if err != nil {
		t.Fatal(err)
	}
	normalized, err := key.PayloadHash(map[string]any{"name": "name"})
	if err != nil || base != normalized {
		t.Fatalf("normalization drift: %s %s %v", base, normalized, err)
	}
	for _, mutate := range []func(*OperationKey){
		func(k *OperationKey) { k.WorkspaceID = uuid.NewString() },
		func(k *OperationKey) { k.ActorUserID = uuid.NewString() },
		func(k *OperationKey) { k.Operation = "create" },
	} {
		changed := key
		mutate(&changed)
		hash, err := changed.PayloadHash(map[string]any{"name": "name"})
		if err != nil || hash == base {
			t.Fatalf("operation identity not bound: %v", err)
		}
	}
	key.RequestID = "bad"
	if _, err := key.PayloadHash(nil); err == nil {
		t.Fatal("invalid request UUID accepted")
	}
}

func TestOperationIdentityAllowsSameUUIDInDifferentRoles(t *testing.T) {
	key := operationTestKey()
	key.ActorUserID = key.WorkspaceID
	key.RequestID = key.WorkspaceID
	params, err := key.queryKey()
	if err != nil || !params.WorkspaceID.Valid || !params.ActorUserID.Valid || !params.RequestID.Valid {
		t.Fatalf("independent identity fields must all be parsed: %+v %v", params, err)
	}
}

func TestOperationReplayPreservesDeletedIdentityAndRejectsDifferentIntent(t *testing.T) {
	key := operationTestKey()
	id := uuid.New()
	result := WriteResult{WorkspaceID: key.WorkspaceID, RequestID: key.RequestID, OperationID: id.String(), Operation: key.Operation, IterationIDs: []string{}, Result: WriteSummary{Deleted: true, SettingsRevision: 1}, CommittedAt: time.Now().UTC()}
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	row := db.IterationOperation{ID: pgtype.UUID{Bytes: id, Valid: true}, WorkspaceID: pgtype.UUID{Bytes: uuid.MustParse(key.WorkspaceID), Valid: true}, ActorUserID: pgtype.UUID{Bytes: uuid.MustParse(key.ActorUserID), Valid: true}, RequestID: pgtype.UUID{Bytes: uuid.MustParse(key.RequestID), Valid: true}, Operation: key.Operation, PayloadHash: "same", Result: raw}
	replayed, err := decodeOperationResult(row, key, "same", true)
	if err != nil || !replayed.Replayed || !replayed.Result.Deleted || replayed.OperationID != result.OperationID {
		t.Fatalf("tombstone replay: %+v %v", replayed, err)
	}
	_, err = decodeOperationResult(row, key, "different", true)
	var conflict *OperationError
	if !errors.As(err, &conflict) || conflict.Code != "idempotency_conflict" {
		t.Fatalf("hash mismatch must conflict: %v", err)
	}
	row.Result = []byte(`{"workspace_id":"another"}`)
	if _, err = decodeOperationResult(row, key, "same", true); err == nil {
		t.Fatal("malformed durable result must never become an empty success")
	}
}

func TestStatisticsWireShapeIsFlatAndRatiosNullable(t *testing.T) {
	raw, err := json.Marshal(Statistics{})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]json.RawMessage
	if err = json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"original", "initial_effective", "net_effective_change", "started", "chart", "calculated_at"} {
		if _, ok := got[key]; !ok {
			t.Errorf("flat statistics missing %s", key)
		}
	}
	for _, key := range []string{"net_effective_change_ratio", "effective_ratio", "original_ratio"} {
		if string(got[key]) != "null" {
			t.Errorf("empty ratio %s must be null, got %s", key, got[key])
		}
	}
	if _, nested := got["ScopeStatistics"]; nested {
		t.Fatal("domain statistics leaked a nested wire object")
	}
}
