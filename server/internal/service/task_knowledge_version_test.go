package service

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func knowledgeStorageVersionFixture(t *testing.T, version int) taskKnowledgeDelivery {
	t.Helper()
	scopeID := uuid.NewString()
	session := protocol.TaskKnowledgeSessionBinding{Provider: "claude", RuntimeID: uuid.NewString(), AgentID: uuid.NewString(), ConfigurationID: "version-test", ScopeKind: "issue", ScopeID: scopeID, Generation: uuid.NewString()}
	manifest := KnowledgeManifest{ProtocolVersion: version, Entries: []KnowledgeManifestEntry{{CandidateID: uuid.NewString(), Version: "1", ContentSHA256: strings.Repeat("a", 64), SourceIssueID: uuid.NewString(), Target: protocol.CollaborationKnowledgeTarget{Kind: "project", WorkspaceID: uuid.NewString(), ProjectID: uuid.NewString()}}}}
	if version == 2 {
		manifest.Entries[0].Validity = &protocol.TaskKnowledgeValidity{Revision: "1", AcceptedEventSeq: "15", SubmissionID: uuid.NewString(), ValidatedCloseEpoch: "3"}
	}
	hash, err := protocol.TaskKnowledgeRequestHash(manifest)
	if err != nil {
		t.Fatal(err)
	}
	prepared := &taskKnowledgePreparation{ID: uuid.NewString(), RequestID: uuid.NewString(), RequestHash: strings.Repeat("b", 64), InvocationFingerprint: strings.Repeat("c", 64), Manifest: manifest, ManifestSHA256: hash, Targets: []KnowledgeTargetRevision{}, DisclosedAt: time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC)}
	claim := taskKnowledgeClaim{
		Phase: "disclosed", DaemonID: "version-test-daemon", RuntimeOwnerUserID: uuid.NewString(), ConfigurationDigest: strings.Repeat("d", 64), Prepared: prepared,
		Descriptor: protocol.TaskKnowledgeClaimDescriptor{ProtocolVersion: version, TaskID: uuid.NewString(), RuntimeID: session.RuntimeID, ClaimGeneration: "1", DispatchedAt: "2026-11-14T00:00:00Z", Provider: session.Provider, Mode: "local", ConfigurationID: session.ConfigurationID, PreparationRequired: true, Session: session},
		Attempts:   []taskKnowledgeAttempt{{RequestID: uuid.NewString(), RequestHash: strings.Repeat("e", 64), Authorization: protocol.TaskKnowledgeAuthorization{ProtocolVersion: version, PrepareID: prepared.ID, InvocationFingerprint: prepared.InvocationFingerprint, AttemptSequence: 1, AttemptNonce: uuid.NewString(), ManifestSHA256: hash, PromptSHA256: strings.Repeat("f", 64), Session: session}, Observations: []protocol.TaskKnowledgeObservation{}}},
	}
	return taskKnowledgeDelivery{Version: version, ScopeKind: session.ScopeKind, ScopeID: scopeID, Claims: []taskKnowledgeClaim{claim}}
}

func TestKnowledgeStorageVersionReadersPreserveOriginal(t *testing.T) {
	for _, version := range []int{1, 2} {
		delivery := knowledgeStorageVersionFixture(t, version)
		raw, err := json.Marshal(delivery)
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := decodeTaskKnowledge(pgtype.Int4{Int32: int32(version), Valid: true}, raw)
		if err != nil {
			t.Errorf("retained version %d rejected: %v", version, err)
			continue
		}
		encoded, err := encodeTaskKnowledge(decoded)
		if err != nil || string(encoded) != string(raw) {
			t.Errorf("retained version %d changed bytes or hash inputs: %v", version, err)
		}
	}
}

func TestKnowledgeStorageV2RetainsOriginalV1Claim(t *testing.T) {
	delivery := knowledgeStorageVersionFixture(t, 1)
	original, err := json.Marshal(delivery.Claims[0])
	if err != nil {
		t.Fatal(err)
	}
	delivery.Version = 2
	newClaim := knowledgeStorageVersionFixture(t, 2).Claims[0]
	newClaim.Descriptor.ClaimGeneration = "2"
	newClaim.Descriptor.Session.ScopeID = delivery.ScopeID
	newClaim.Attempts[0].Authorization.Session = newClaim.Descriptor.Session
	delivery.Claims = append(delivery.Claims, newClaim)
	raw, err := json.Marshal(delivery)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeTaskKnowledge(pgtype.Int4{Int32: 2, Valid: true}, raw)
	if err != nil {
		t.Fatal("monotonic outer v2 envelope lost its original v1 claim:", err)
	}
	after, err := json.Marshal(decoded.Claims[0])
	if err != nil || string(original) != string(after) {
		t.Fatal("v1 claim was relabeled or rewritten inside v2 envelope")
	}
	encoded, err := encodeTaskKnowledge(decoded)
	if err != nil || string(raw) != string(encoded) {
		t.Fatal("mixed retained record failed exact round trip:", err)
	}
}

func TestKnowledgeStorageV2CanContainOnlyLegacyClaims(t *testing.T) {
	delivery := knowledgeStorageVersionFixture(t, 1)
	delivery.Version = 2
	raw, err := json.Marshal(delivery)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeTaskKnowledge(pgtype.Int4{Int32: 2, Valid: true}, raw)
	if err != nil || decoded.Claims[0].Descriptor.ProtocolVersion != 1 || decoded.Claims[0].Prepared.Manifest.ProtocolVersion != 1 {
		t.Fatal("legacy-only contents were relabeled by the outer storage version:", err)
	}
	if _, err := decodeTaskKnowledge(pgtype.Int4{Int32: 1, Valid: true}, raw); !collaborationErrorCode(err, "knowledge_context_changed") {
		t.Fatal("database and JSON outer versions were allowed to disagree:", err)
	}
}

func TestKnowledgeStorageVersionReadersRejectMismatches(t *testing.T) {
	for name, change := range map[string]func(*taskKnowledgeDelivery){
		"outer downgrade": func(d *taskKnowledgeDelivery) { d.Version = 1 },
		"unknown claim":   func(d *taskKnowledgeDelivery) { d.Claims[0].Descriptor.ProtocolVersion = 3 },
		"manifest downgrade": func(d *taskKnowledgeDelivery) {
			d.Claims[0].Prepared.Manifest.ProtocolVersion = 1
			d.Claims[0].Prepared.Manifest.Entries[0].Validity = nil
			hash, _ := protocol.TaskKnowledgeRequestHash(d.Claims[0].Prepared.Manifest)
			d.Claims[0].Prepared.ManifestSHA256 = hash
			d.Claims[0].Attempts[0].Authorization.ManifestSHA256 = hash
		},
		"authorization downgrade": func(d *taskKnowledgeDelivery) { d.Claims[0].Attempts[0].Authorization.ProtocolVersion = 1 },
		"observation downgrade": func(d *taskKnowledgeDelivery) {
			c := &d.Claims[0]
			a := &c.Attempts[0]
			a.Terminated = true
			a.Observations = []protocol.TaskKnowledgeObservation{{ProtocolVersion: 1, RequestID: uuid.NewString(), PrepareID: c.Prepared.ID, ClaimGeneration: c.Descriptor.ClaimGeneration, DispatchedAt: c.Descriptor.DispatchedAt, AttemptSequence: 1, AttemptNonce: a.Authorization.AttemptNonce, Kind: "terminated", TerminationKind: "not_started"}}
		},
	} {
		t.Run(name, func(t *testing.T) {
			delivery := knowledgeStorageVersionFixture(t, 2)
			change(&delivery)
			raw, err := json.Marshal(delivery)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = decodeTaskKnowledge(pgtype.Int4{Int32: int32(delivery.Version), Valid: true}, raw); !collaborationErrorCode(err, "knowledge_context_changed") {
				t.Fatalf("mixed retained versions did not fail closed: %v", err)
			}
		})
	}
}

func TestKnowledgeManifestUnionComparesValidityValues(t *testing.T) {
	a := knowledgeStorageVersionFixture(t, 2).Claims[0].Prepared.Manifest
	raw, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	var b KnowledgeManifest
	if err = json.Unmarshal(raw, &b); err != nil {
		t.Fatal(err)
	}
	merged, err := taskKnowledgeManifestUnion(a, b)
	if err != nil || merged.ProtocolVersion != 2 || len(merged.Entries) != 1 {
		t.Fatalf("equal independently decoded proof was duplicated or downgraded: %#v %v", merged, err)
	}
	b.Entries[0].Validity.Revision = "2"
	if _, err = taskKnowledgeManifestUnion(a, b); !collaborationErrorCode(err, "knowledge_context_changed") {
		t.Fatalf("same candidate's changed proof silently replaced its exposure: %v", err)
	}
}

func TestKnowledgeStorageCollaborationVersions(t *testing.T) {
	if collaborationKnowledgeClaimKey != "knowledge:claim:v1" || collaborationKnowledgePrepareKey != "knowledge:prepare:v1" {
		t.Fatal("retained slots changed, hiding original knowledge exposure")
	}
	for _, version := range []int{1, 2} {
		ordinary := knowledgeStorageVersionFixture(t, version).Claims[0].Prepared
		stored := collaborationKnowledgePreparation{Request: protocol.CollaborationRunKnowledgePrepare{ProtocolVersion: version, Identity: protocol.CollaborationRunIdentity{RunID: uuid.NewString()}, RequestID: uuid.NewString(), HandoffID: uuid.NewString(), InvocationFingerprint: ordinary.InvocationFingerprint}, Manifest: ordinary.Manifest, ManifestSHA256: ordinary.ManifestSHA256, Targets: []KnowledgeTargetRevision{}}
		raw, err := json.Marshal(stored)
		if err != nil {
			t.Fatal(err)
		}
		var decoded collaborationKnowledgePreparation
		if err := decodeCollaborationPreparationJSON(raw, &decoded); err != nil {
			t.Fatalf("retained collaboration v%d rejected: %v", version, err)
		}
		after, err := json.Marshal(decoded)
		if err != nil || string(after) != string(raw) {
			t.Fatalf("retained collaboration v%d changed bytes: %v", version, err)
		}
		stored.Request.ProtocolVersion = 3 - version
		raw, err = json.Marshal(stored)
		if err != nil {
			t.Fatal(err)
		}
		if err := decodeCollaborationPreparationJSON(raw, &decoded); err == nil {
			t.Fatal("retained collaboration request and manifest versions disagree")
		}
	}
}
