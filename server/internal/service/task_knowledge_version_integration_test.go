package service

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestKnowledgeRequestsMustMatchRetainedClaimVersion(t *testing.T) {
	for _, stage := range []string{"prepare", "authorize", "observe"} {
		t.Run(stage, func(t *testing.T) {
			f := newTaskKnowledgeFixture(t)
			d := f.finalize(t, "")
			var prepared protocol.TaskKnowledgePrepareResponse
			var authorization protocol.TaskKnowledgeAuthorization
			var err error
			if stage != "prepare" {
				prepared, err = f.tasks.PrepareTaskKnowledge(f.ctx, f.caller, f.task.ID, f.prepareRequest(d))
				if err != nil {
					t.Fatal(err)
				}
			}
			if stage == "observe" {
				authorization, err = f.tasks.AuthorizeTaskKnowledgeLaunch(f.ctx, f.caller, f.task.ID, f.authorizeRequest(prepared, d))
				if err != nil {
					t.Fatal(err)
				}
			}
			before, err := f.tasks.Queries.GetAgentTask(f.ctx, f.task.ID)
			if err != nil {
				t.Fatal(err)
			}
			switch stage {
			case "prepare":
				req := f.prepareRequest(d)
				req.ProtocolVersion = 2
				_, err = f.tasks.PrepareTaskKnowledge(f.ctx, f.caller, f.task.ID, req)
			case "authorize":
				req := f.authorizeRequest(prepared, d)
				req.ProtocolVersion = 2
				_, err = f.tasks.AuthorizeTaskKnowledgeLaunch(f.ctx, f.caller, f.task.ID, req)
			case "observe":
				err = f.tasks.ObserveTaskKnowledgeLaunch(f.ctx, f.caller, f.task.ID, protocol.TaskKnowledgeObservation{ProtocolVersion: 2, RequestID: uuid.NewString(), PrepareID: authorization.PrepareID, ClaimGeneration: d.ClaimGeneration, DispatchedAt: d.DispatchedAt, AttemptSequence: authorization.AttemptSequence, AttemptNonce: authorization.AttemptNonce, Kind: "started", ProcessID: 42, ProcessInstanceID: "version-test-process"})
			}
			if !collaborationErrorCode(err, "knowledge_context_changed") {
				t.Errorf("%s accepted a protocol version not negotiated by the claim: %v", stage, err)
			}
			after, readErr := f.tasks.Queries.GetAgentTask(f.ctx, f.task.ID)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if before.KnowledgeContextVersion != after.KnowledgeContextVersion || !bytes.Equal(before.KnowledgeDelivery, after.KnowledgeDelivery) || before.SessionID != after.SessionID {
				t.Errorf("%s changed retained metadata after a version refusal", stage)
			}
		})
	}
}

func TestKnowledgeV1CannotResumeV2Exposure(t *testing.T) {
	f := newTaskKnowledgeFixture(t)
	f.recordSession(t, "v2-retained-native")
	row, err := f.tasks.Queries.GetAgentTask(f.ctx, f.task.ID)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := decodeTaskKnowledge(row.KnowledgeContextVersion, row.KnowledgeDelivery)
	if err != nil {
		t.Fatal(err)
	}
	// Model a prior v2 writer while leaving the actual source and session facts
	// unchanged. This fixture does not activate a production v2 writer.
	stored.Version = 2
	for i := range stored.Claims {
		claim := &stored.Claims[i]
		claim.Descriptor.ProtocolVersion = 2
		claim.Prepared.Manifest.ProtocolVersion = 2
		claim.Prepared.ManifestSHA256, err = protocol.TaskKnowledgeRequestHash(claim.Prepared.Manifest)
		if err != nil {
			t.Fatal(err)
		}
		for j := range claim.Attempts {
			attempt := &claim.Attempts[j]
			attempt.Authorization.ProtocolVersion = 2
			attempt.Authorization.ManifestSHA256 = claim.Prepared.ManifestSHA256
			for k := range attempt.Observations {
				attempt.Observations[k].ProtocolVersion = 2
			}
		}
	}
	raw, err := json.Marshal(stored)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = decodeTaskKnowledge(pgtype.Int4{Int32: 2, Valid: true}, raw); err != nil {
		t.Fatal("v2 history fixture must be independently valid:", err)
	}
	f.fx.Exec(t, `UPDATE agent_task_queue SET knowledge_context_version=2, knowledge_delivery=$2 WHERE id=$1`, f.task.ID, raw)
	f.nextTurn(t)
	claim, prior, err := f.tasks.BuildTaskKnowledgeClaim(f.ctx, f.task, &f.inv, "v2-retained-native")
	if !collaborationErrorCode(err, "knowledge_context_changed") || claim != nil || prior != "" {
		t.Fatalf("v1 read of v2 exposure resumed or silently created a fresh session: claim=%v prior=%q err=%v", claim != nil, prior, err)
	}
}

func TestKnowledgeAutomaticHistoryCannotDisappearDuringResumeSelection(t *testing.T) {
	for _, change := range []string{"server_configuration", "client_binding", "native_refused", "v1_after_configuration_change"} {
		t.Run(change, func(t *testing.T) {
			source, ref := knowledgeAcceptedSourceFixture(t, "Accepted project convention", "The delivery supports this convention")
			candidate := publishConsumptionProof(t, source, ref, nil, 0, nil)
			f := automaticConsumerTask(t, source, candidate.TargetProjectID)
			const native = "retained-automatic-native"
			var original *protocol.TaskKnowledgeClaimDescriptor
			if change == "native_refused" {
				original = f.finalize(t, "")
				if _, err := f.tasks.StartTask(f.ctx, f.task.ID); err != nil {
					t.Fatal(err)
				}
				prepared, err := f.tasks.PrepareTaskKnowledge(f.ctx, f.caller, f.task.ID, f.prepareRequest(original))
				if err != nil {
					t.Fatal(err)
				}
				authorization, err := f.tasks.AuthorizeTaskKnowledgeLaunch(f.ctx, f.caller, f.task.ID, f.authorizeRequest(prepared, original))
				if err != nil {
					t.Fatal(err)
				}
				f.observe(t, original, authorization, "started", "")
				f.observe(t, original, authorization, "native_session", native)
				err = f.tasks.ObserveTaskKnowledgeLaunch(f.ctx, f.caller, f.task.ID, protocol.TaskKnowledgeObservation{ProtocolVersion: 2,
					RequestID: uuid.NewString(), PrepareID: authorization.PrepareID, ClaimGeneration: original.ClaimGeneration,
					DispatchedAt: original.DispatchedAt, AttemptSequence: authorization.AttemptSequence, AttemptNonce: authorization.AttemptNonce,
					Kind: "native_session", NativeSessionID: "contradictory-native", Source: "result"})
				if !collaborationErrorCode(err, "knowledge_context_changed") {
					t.Fatal("fixture did not record the contradictory native refusal:", err)
				}
				f.observe(t, original, authorization, "terminated", "")
				if _, err = f.tasks.FailTask(f.ctx, f.task.ID, "knowledge_context_changed", native, "", "", "knowledge_context_changed", false, "", ""); err != nil {
					t.Fatal(err)
				}
			} else {
				original = f.recordSession(t, native)
				if _, err := f.tasks.CompleteTask(f.ctx, f.task.ID, []byte(`{"output":"done"}`), native, "", "", false, "", ""); err != nil {
					t.Fatal(err)
				}
			}
			oldTask, before := f.task.ID, f.knowledgeDelivery(t, f.task.ID)
			access := mustAccess(t, source, source.ctx)
			if _, err := source.svc.ChangeLifecycle(source.ctx, lifecycleParams(t, source, "reopen", "todo", access.Revision, access.LifecycleEpoch)); err != nil {
				t.Fatal(err)
			}
			switch change {
			case "server_configuration", "v1_after_configuration_change":
				f.fx.Exec(t, `UPDATE agent SET instructions='Updated consumer configuration' WHERE id=$1`, f.task.AgentID)
			case "client_binding":
				f.inv.ConfigurationID = "different-client-configuration"
			}
			f.nextTurn(t)
			if f.task.KnowledgeContextVersion.Valid {
				t.Fatal("new task row must obtain its exposure facts from retained history")
			}
			if change == "v1_after_configuration_change" {
				f.inv.ProtocolVersion = 1
			}
			claim, prior, err := f.tasks.BuildTaskKnowledgeClaim(f.ctx, f.task, &f.inv, native)
			if !collaborationErrorCode(err, "knowledge_context_changed") || claim != nil || prior != "" {
				t.Fatalf("%s erased retained automatic history into a fresh fallback: claim=%v prior=%q err=%v", change, claim != nil, prior, err)
			}
			// An explicitly selected new binding remains distinct from silently
			// replacing a requested resume, and cannot reuse the paused proof.
			f.inv.ProtocolVersion = 2
			fresh := f.finalize(t, "")
			prepared, err := f.tasks.PrepareTaskKnowledge(f.ctx, f.caller, f.task.ID, f.prepareRequest(fresh))
			if err != nil || fresh.Session.NativeSessionID != "" || fresh.Session.Generation == original.Session.Generation || len(prepared.Contributions) != 0 {
				t.Fatalf("explicit fresh binding reused paused history or was refused: %v", err)
			}
			if !bytes.Equal(before, f.knowledgeDelivery(t, oldTask)) {
				t.Fatal("resume refusal or fresh preparation rewrote retained exposure")
			}
		})
	}
}
