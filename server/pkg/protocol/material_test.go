package protocol

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
)

func TestMaterialRepositoryDigestSurvivesJSONB(t *testing.T) {
	e := MaterialExecution{ProtocolVersion: 1, RepositoryExecution: &MaterialRepositoryExecution{Candidate: json.RawMessage(`{"z":1,"a":{"b":9007199254740993,"a":2}}`)}}
	digest := e.Digest()
	e.RepositoryExecution.Candidate = json.RawMessage(`{ "a": {"a":2,"b":9007199254740993},"z":1}`)
	if e.Digest() != digest {
		t.Fatal("jsonb normalization changed descriptor")
	}
	e.RepositoryExecution.Candidate = json.RawMessage(`{"a":{"a":2,"b":9007199254740992},"z":1}`)
	if e.Digest() == digest {
		t.Fatal("integer precision lost")
	}
}
func TestMaterialLegacyDigestRemainsByteCompatible(t *testing.T) {
	e := MaterialExecution{ProtocolVersion: 1, TaskID: "legacy", Files: []MaterialFile{}}
	raw, _ := json.Marshal(e)
	sum := sha256.Sum256(raw)
	if e.Digest() != hex.EncodeToString(sum[:]) {
		t.Fatal("legacy digest changed")
	}
}
