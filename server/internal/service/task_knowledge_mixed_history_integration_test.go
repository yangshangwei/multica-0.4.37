package service

import (
	"bytes"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestKnowledgeMixedPreparedHistoryCannotHideAutomaticExposure(t *testing.T) {
	for _, change := range []string{"server_configuration", "client_binding", "native_refused"} {
		t.Run(change, func(t *testing.T) {
			var humanCandidate protocol.CollaborationKnowledgeProposal
			source, ref := knowledgeAcceptedSourceFixture(t, "Accepted project convention", "The delivery supports this convention", func(source *collaborationQueueFixture) {
				source.fx.Cleanup(t, `DELETE FROM collaboration_memory_version WHERE issue_id=$1`, source.issue.Issue.ID)
				humanCandidate = knowledgeServiceProposal(t, source)
				if _, err := source.svc.ProposeKnowledge(source.ctx, source.issue.Issue.ID, humanCandidate); err != nil {
					t.Fatal(err)
				}
				if _, err := source.svc.ApproveKnowledge(source.ctx, util.MustParseUUID(humanCandidate.CandidateID), protocol.CollaborationKnowledgeApproveRequest{ProtocolVersion: 1,
					RequestID: uuid.NewString(), ExpectedStateRevision: "1", ExpectedVersion: "1", ExpectedContentSHA256: protocol.CollaborationKnowledgeHash(humanCandidate.Content)}); err != nil {
					t.Fatal(err)
				}
			})
			issue, err := source.svc.Queries.GetIssue(source.ctx, source.issue.Issue.ID)
			if err != nil {
				t.Fatal(err)
			}
			consumer := automaticConsumerTask(t, source, issue.ProjectID)
			consumer.inv.ProtocolVersion = 1
			const native = "mixed-prepared-native"
			consumer.fx.Exec(t, `UPDATE agent_task_queue SET session_id=$2 WHERE id=$1`, consumer.task.ID, native)
			humanTask := consumer.task.ID
			humanDescriptor := consumer.finalize(t, native)
			if _, err := consumer.tasks.StartTask(consumer.ctx, humanTask); err != nil {
				t.Fatal(err)
			}
			humanPrepared, err := consumer.tasks.PrepareTaskKnowledge(consumer.ctx, consumer.caller, humanTask, consumer.prepareRequest(humanDescriptor))
			if err != nil || humanPrepared.ProtocolVersion != 1 || len(humanPrepared.Contributions) != 1 || humanPrepared.Contributions[0].CandidateID != humanCandidate.CandidateID {
				t.Fatalf("human preparation must precede automatic publication: %v", err)
			}
			humanAuthorization, err := consumer.tasks.AuthorizeTaskKnowledgeLaunch(consumer.ctx, consumer.caller, humanTask, consumer.authorizeRequest(humanPrepared, humanDescriptor))
			if err != nil {
				t.Fatal(err)
			}
			consumer.observe(t, humanDescriptor, humanAuthorization, "started", "")
			consumer.observe(t, humanDescriptor, humanAuthorization, "native_session", native)
			if change != "native_refused" {
				consumer.observe(t, humanDescriptor, humanAuthorization, "terminated", "")
				if _, err := consumer.tasks.CompleteTask(consumer.ctx, humanTask, []byte(`{"output":"human turn"}`), native, "", "", false, "", ""); err != nil {
					t.Fatal(err)
				}
			}

			automaticCandidate := publishConsumptionProof(t, source, ref, nil, 0, nil)
			consumer.inv.ProtocolVersion = 2
			consumer.nextTurn(t)
			automaticTask := consumer.task.ID
			automaticDescriptor := consumer.finalize(t, native)
			if automaticDescriptor.Session != humanDescriptor.Session {
				t.Fatal("the two genuine preparations must share one exact native binding")
			}
			if _, err := consumer.tasks.StartTask(consumer.ctx, automaticTask); err != nil {
				t.Fatal(err)
			}
			automaticPrepared, err := consumer.tasks.PrepareTaskKnowledge(consumer.ctx, consumer.caller, automaticTask, consumer.prepareRequest(automaticDescriptor))
			if err != nil || automaticPrepared.ProtocolVersion != 2 || len(automaticPrepared.Contributions) != 2 {
				t.Fatalf("automatic preparation must include the original human contribution and the new proof: %v", err)
			}
			automaticAuthorization, err := consumer.tasks.AuthorizeTaskKnowledgeLaunch(consumer.ctx, consumer.caller, automaticTask, consumer.authorizeRequest(automaticPrepared, automaticDescriptor))
			if err != nil {
				t.Fatal(err)
			}
			consumer.observe(t, automaticDescriptor, automaticAuthorization, "started", "")
			consumer.observe(t, automaticDescriptor, automaticAuthorization, "native_session", native)
			consumer.observe(t, automaticDescriptor, automaticAuthorization, "terminated", "")
			if _, err := consumer.tasks.CompleteTask(consumer.ctx, automaticTask, []byte(`{"output":"automatic turn"}`), native, "", "", false, "", ""); err != nil {
				t.Fatal(err)
			}

			consumer.task, err = consumer.tasks.Queries.GetAgentTask(consumer.ctx, humanTask)
			if err != nil {
				t.Fatal(err)
			}
			if change == "native_refused" {
				err := consumer.tasks.ObserveTaskKnowledgeLaunch(consumer.ctx, consumer.caller, humanTask, protocol.TaskKnowledgeObservation{ProtocolVersion: 1,
					RequestID: uuid.NewString(), PrepareID: humanAuthorization.PrepareID, ClaimGeneration: humanDescriptor.ClaimGeneration,
					DispatchedAt: humanDescriptor.DispatchedAt, AttemptSequence: humanAuthorization.AttemptSequence, AttemptNonce: humanAuthorization.AttemptNonce,
					Kind: "native_session", NativeSessionID: "contradictory-human-native", Source: "result"})
				var refusal *CollaborationError
				if !errors.As(err, &refusal) || refusal.Status != 409 || refusal.Code != "knowledge_context_changed" {
					t.Fatalf("the human row must retain an actual contradictory-native refusal: %v", err)
				}
				consumer.observe(t, humanDescriptor, humanAuthorization, "terminated", "")
				if _, err := consumer.tasks.FailTask(consumer.ctx, humanTask, "knowledge_context_changed", native, "", "", "knowledge_context_changed", false, "", ""); err != nil {
					t.Fatal(err)
				}
			}
			consumer.insertClaimedHistory(t, 260)
			consumer.fx.Exec(t, `UPDATE agent_task_queue SET created_at=CASE
				WHEN id=$2 THEN '2026-11-14T03:00:00Z'::timestamptz
				WHEN id=$3 THEN '2026-11-14T01:00:00Z'::timestamptz
				ELSE '2026-11-14T02:00:00Z'::timestamptz END
				WHERE agent_id=$1 AND knowledge_context_version IS NOT NULL`, consumer.task.AgentID, humanTask, automaticTask)
			if count := consumer.fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE agent_id=$1 AND knowledge_context_version IS NOT NULL`, consumer.task.AgentID); count != 262 {
				t.Fatalf("mixed history must contain exactly two preparations and 260 claimed-only fillers, got %d", count)
			}
			page := db.ListTaskKnowledgeHistoryPageParams{AgentID: consumer.task.AgentID, IssueID: consumer.task.IssueID, ChatSessionID: consumer.task.ChatSessionID,
				ScopeID: humanDescriptor.Session.ScopeID, BeforeCreatedAt: pgtype.Timestamptz{InfinityModifier: pgtype.Infinity, Valid: true},
				BeforeID: util.MustParseUUID("ffffffff-ffff-ffff-ffff-ffffffffffff"), PageSize: taskKnowledgeHistoryPage}
			for pageNumber, expectedRows := range []int{128, 128, 6} {
				rows, err := consumer.tasks.Queries.ListTaskKnowledgeHistoryPage(consumer.ctx, page)
				if err != nil || len(rows) != expectedRows {
					t.Fatalf("history page %d: rows=%d want=%d err=%v", pageNumber+1, len(rows), expectedRows, err)
				}
				if pageNumber == 0 && rows[0].ID != humanTask || pageNumber == 2 && rows[len(rows)-1].ID != automaticTask {
					t.Fatal("history must encounter the human preparation first and the automatic preparation last on page three")
				}
				for _, row := range rows {
					stored, err := decodeTaskKnowledge(row.KnowledgeContextVersion, row.KnowledgeDelivery)
					if err != nil || stored == nil || len(stored.Claims) != 1 {
						t.Fatalf("every history fixture must be strictly valid before testing resume: %v", err)
					}
					claim := stored.Claims[0]
					if claim.Descriptor.TaskID != util.UUIDToString(row.ID) || claim.Descriptor.Session.AgentID != util.UUIDToString(row.AgentID) || claim.Descriptor.Session.ScopeID != humanDescriptor.Session.ScopeID {
						t.Fatal("history fixture identity does not match its task row")
					}
					switch row.ID {
					case humanTask:
						if stored.Version != 1 || claim.Prepared == nil || claim.Prepared.Manifest.ProtocolVersion != 1 || len(claim.Prepared.Manifest.Entries) != 1 || claim.Prepared.Manifest.Entries[0].CandidateID != humanCandidate.CandidateID || claim.Prepared.Manifest.Entries[0].Validity != nil || len(claim.Attempts) != 1 || claim.Attempts[0].BoundSessionID != native || claim.Attempts[0].Refused != (change == "native_refused") {
							t.Fatal("the first row must be a pure human preparation, with refusal only in its named variant")
						}
					case automaticTask:
						if stored.Version != 2 || claim.Prepared == nil || claim.Prepared.Manifest.ProtocolVersion != 2 || len(claim.Prepared.Manifest.Entries) != 2 || len(claim.Attempts) != 1 || claim.Attempts[0].BoundSessionID != native || claim.Attempts[0].Refused {
							t.Fatal("the deep row must retain a valid automatic preparation and admitted native observation")
						}
						proofs := 0
						for _, entry := range claim.Prepared.Manifest.Entries {
							if entry.Validity != nil {
								proofs++
								if entry.CandidateID != util.UUIDToString(automaticCandidate.ID) {
									t.Fatal("the deep automatic proof must identify the actually published candidate")
								}
							}
						}
						if proofs != 1 {
							t.Fatal("the deep row must contain exactly one automatic validity proof")
						}
					default:
						if stored.Version != 1 || claim.Descriptor.ProtocolVersion != 1 || claim.Phase != "claimed" || claim.Prepared != nil || len(claim.Attempts) != 0 {
							t.Fatal("growth rows must remain valid v1 claimed-only history")
						}
					}
				}
				last := rows[len(rows)-1]
				page.BeforeCreatedAt, page.BeforeID = last.CreatedAt, last.ID
			}

			humanBefore, err := consumer.tasks.Queries.GetAgentTask(consumer.ctx, humanTask)
			if err != nil {
				t.Fatal(err)
			}
			automaticBefore, err := consumer.tasks.Queries.GetAgentTask(consumer.ctx, automaticTask)
			if err != nil {
				t.Fatal(err)
			}
			access := mustAccess(t, source, source.ctx)
			if _, err := source.svc.ChangeLifecycle(source.ctx, lifecycleParams(t, source, "reopen", "todo", access.Revision, access.LifecycleEpoch)); err != nil {
				t.Fatal(err)
			}
			switch change {
			case "server_configuration":
				consumer.fx.Exec(t, `UPDATE agent SET instructions='Changed mixed-history consumer configuration' WHERE id=$1`, consumer.task.AgentID)
			case "client_binding":
				consumer.inv.ConfigurationID = "changed-mixed-history-client"
			}
			consumer.nextTurn(t)
			if consumer.task.KnowledgeContextVersion.Valid || len(consumer.task.KnowledgeDelivery) != 0 {
				t.Fatal("the selected turn must obtain all exposure facts from retained history")
			}
			var historyBefore, historyAfter []byte
			historySQL := `SELECT jsonb_agg(jsonb_build_object('id',id,'version',knowledge_context_version,'delivery',knowledge_delivery,'session_id',session_id,'created_at',created_at) ORDER BY created_at DESC,id DESC)::text
				FROM agent_task_queue WHERE agent_id=$1 AND knowledge_context_version IS NOT NULL AND id<>$2`
			consumer.fx.QueryRow(t, historySQL, consumer.task.AgentID, consumer.task.ID).Scan(&historyBefore)
			descriptor, prior, err := consumer.tasks.BuildTaskKnowledgeClaim(consumer.ctx, consumer.task, &consumer.inv, native)
			var refusal *CollaborationError
			if !errors.As(err, &refusal) || refusal.Status != 409 || refusal.Code != "knowledge_context_changed" || descriptor != nil || prior != "" {
				t.Fatalf("%s hid the automatic exposure behind an earlier human incompatibility: descriptor=%v prior=%q err=%v", change, descriptor != nil, prior, err)
			}
			current, err := consumer.tasks.Queries.GetAgentTask(consumer.ctx, consumer.task.ID)
			if err != nil {
				t.Fatal(err)
			}
			if current.KnowledgeContextVersion != consumer.task.KnowledgeContextVersion || !bytes.Equal(current.KnowledgeDelivery, consumer.task.KnowledgeDelivery) || current.SessionID != consumer.task.SessionID {
				t.Fatal("resume refusal must not finalize a replacement claim or attempt")
			}
			fresh := consumer.finalize(t, "")
			prepared, err := consumer.tasks.PrepareTaskKnowledge(consumer.ctx, consumer.caller, consumer.task.ID, consumer.prepareRequest(fresh))
			if err != nil || fresh.Session.NativeSessionID != "" || fresh.Session.Generation == humanDescriptor.Session.Generation || protocol.ValidateTaskKnowledgeSession(fresh.Session) != nil || len(prepared.Contributions) != 1 || prepared.Contributions[0].CandidateID != humanCandidate.CandidateID {
				t.Fatalf("explicit fresh execution must retain the human contribution and exclude the paused automatic proof: %v", err)
			}
			consumer.fx.QueryRow(t, historySQL, consumer.task.AgentID, consumer.task.ID).Scan(&historyAfter)
			if !bytes.Equal(historyBefore, historyAfter) {
				t.Fatal("refusal or explicit fresh preparation changed retained mixed-history bytes")
			}
			for _, before := range []db.AgentTaskQueue{humanBefore, automaticBefore} {
				after, err := consumer.tasks.Queries.GetAgentTask(consumer.ctx, before.ID)
				if err != nil {
					t.Fatal(err)
				}
				original, err := decodeTaskKnowledge(before.KnowledgeContextVersion, before.KnowledgeDelivery)
				if err != nil {
					t.Fatal(err)
				}
				retained, err := decodeTaskKnowledge(after.KnowledgeContextVersion, after.KnowledgeDelivery)
				if err != nil {
					t.Fatal(err)
				}
				if before.KnowledgeContextVersion != after.KnowledgeContextVersion || !bytes.Equal(before.KnowledgeDelivery, after.KnowledgeDelivery) || before.SessionID != after.SessionID || original.Claims[0].Prepared.ManifestSHA256 != retained.Claims[0].Prepared.ManifestSHA256 || !sameTaskKnowledgeManifest(original.Claims[0].Prepared.Manifest, retained.Claims[0].Prepared.Manifest) || len(original.Claims[0].Attempts) != len(retained.Claims[0].Attempts) {
					t.Fatal("the original manifest, hash, version, pointer and attempt count must remain unchanged")
				}
			}
		})
	}
}

func TestKnowledgeHumanHistoryVersionSurvivesIncompatibleBinding(t *testing.T) {
	for _, change := range []string{"server_configuration", "client_binding"} {
		t.Run(change, func(t *testing.T) {
			consumer := newTaskKnowledgeFixture(t)
			consumer.inv.ProtocolVersion = 2
			const native = "v2-human-incompatible-native"
			originalDescriptor := consumer.recordSession(t, native)
			oldTask := consumer.task.ID
			if _, err := consumer.tasks.CompleteTask(consumer.ctx, oldTask, []byte(`{"output":"human v2 turn"}`), native, "", "", false, "", ""); err != nil {
				t.Fatal(err)
			}
			before, err := consumer.tasks.Queries.GetAgentTask(consumer.ctx, oldTask)
			if err != nil {
				t.Fatal(err)
			}
			original, err := decodeTaskKnowledge(before.KnowledgeContextVersion, before.KnowledgeDelivery)
			if err != nil || original == nil || original.Version != 2 || len(original.Claims) != 1 || original.Claims[0].Prepared == nil || original.Claims[0].Prepared.Manifest.ProtocolVersion != 2 || len(original.Claims[0].Prepared.Manifest.Entries) != 1 || original.Claims[0].Prepared.Manifest.Entries[0].Validity != nil || original.Claims[0].Prepared.Manifest.Entries[0].CandidateID != consumer.candidate.CandidateID {
				t.Fatalf("the retained row must be a genuine pure v2 human preparation: %v", err)
			}
			switch change {
			case "server_configuration":
				consumer.fx.Exec(t, `UPDATE agent SET instructions='Changed pure-human server configuration' WHERE id=$1`, consumer.task.AgentID)
			case "client_binding":
				consumer.inv.ConfigurationID = "changed-pure-human-client"
			}
			consumer.nextTurn(t)
			consumer.inv.ProtocolVersion = 1
			if consumer.task.KnowledgeContextVersion.Valid {
				t.Fatal("the v1 invocation must rely on the earlier prepared history")
			}
			want := originalDescriptor.Session
			want.NativeSessionID, want.ConfigurationID = native, consumer.inv.ConfigurationID
			binding, managed, manifest, err := consumer.tasks.taskKnowledgeExposure(consumer.ctx, consumer.tasks.Queries, consumer.task, want, true)
			if err != nil || binding != nil || !managed || manifest.ProtocolVersion != 2 || len(manifest.Entries) != 0 {
				t.Fatalf("incompatible pure human history must preserve its highest nested manifest version: managed=%v version=%d err=%v", managed, manifest.ProtocolVersion, err)
			}
			descriptor, prior, err := consumer.tasks.BuildTaskKnowledgeClaim(consumer.ctx, consumer.task, &consumer.inv, native)
			var refusal *CollaborationError
			if !errors.As(err, &refusal) || refusal.Status != 409 || refusal.Code != "knowledge_context_changed" || descriptor != nil || prior != "" {
				t.Fatalf("v1 invocation discarded incompatible v2 human exposure: descriptor=%v prior=%q err=%v", descriptor != nil, prior, err)
			}
			current, err := consumer.tasks.Queries.GetAgentTask(consumer.ctx, consumer.task.ID)
			if err != nil {
				t.Fatal(err)
			}
			if current.KnowledgeContextVersion.Valid || len(current.KnowledgeDelivery) != 0 || current.SessionID != consumer.task.SessionID {
				t.Fatal("v1 refusal must leave the new task without a replacement claim or attempt")
			}
			consumer.inv.ProtocolVersion = 2
			fresh := consumer.finalize(t, "")
			prepared, err := consumer.tasks.PrepareTaskKnowledge(consumer.ctx, consumer.caller, consumer.task.ID, consumer.prepareRequest(fresh))
			if err != nil || fresh.Session.NativeSessionID != "" || fresh.Session.Generation == originalDescriptor.Session.Generation || protocol.ValidateTaskKnowledgeSession(fresh.Session) != nil || prepared.ProtocolVersion != 2 || len(prepared.Contributions) != 1 || prepared.Contributions[0].CandidateID != consumer.candidate.CandidateID {
				t.Fatalf("explicit v2 fresh execution of human knowledge must remain supported: %v", err)
			}
			after, err := consumer.tasks.Queries.GetAgentTask(consumer.ctx, oldTask)
			if err != nil {
				t.Fatal(err)
			}
			retained, err := decodeTaskKnowledge(after.KnowledgeContextVersion, after.KnowledgeDelivery)
			if err != nil {
				t.Fatal(err)
			}
			if before.KnowledgeContextVersion != after.KnowledgeContextVersion || !bytes.Equal(before.KnowledgeDelivery, after.KnowledgeDelivery) || before.SessionID != after.SessionID || original.Claims[0].Prepared.ManifestSHA256 != retained.Claims[0].Prepared.ManifestSHA256 || !sameTaskKnowledgeManifest(original.Claims[0].Prepared.Manifest, retained.Claims[0].Prepared.Manifest) || len(original.Claims[0].Attempts) != len(retained.Claims[0].Attempts) {
				t.Fatal("v1 refusal or v2 fresh preparation changed the original human proof or attempts")
			}
		})
	}
}

func TestKnowledgeLegacyHumanHistoryAllowsFreshV2Binding(t *testing.T) {
	for _, envelope := range []string{"v1_envelope", "v2_envelope_with_v1_claim"} {
		t.Run(envelope, func(t *testing.T) {
			consumer := newTaskKnowledgeFixture(t)
			const native = "legacy-human-fresh-v2-native"
			originalDescriptor := consumer.recordSession(t, native)
			oldTask := consumer.task.ID
			if _, err := consumer.tasks.CompleteTask(consumer.ctx, oldTask, []byte(`{"output":"legacy human turn"}`), native, "", "", false, "", ""); err != nil {
				t.Fatal(err)
			}
			row, err := consumer.tasks.Queries.GetAgentTask(consumer.ctx, oldTask)
			if err != nil {
				t.Fatal(err)
			}
			stored, err := decodeTaskKnowledge(row.KnowledgeContextVersion, row.KnowledgeDelivery)
			if err != nil || stored == nil || stored.Version != 1 || len(stored.Claims) != 1 || stored.Claims[0].Prepared == nil || stored.Claims[0].Prepared.Manifest.ProtocolVersion != 1 || len(stored.Claims[0].Prepared.Manifest.Entries) != 1 || stored.Claims[0].Prepared.Manifest.Entries[0].CandidateID != consumer.candidate.CandidateID || stored.Claims[0].Prepared.Manifest.Entries[0].Validity != nil {
				t.Fatalf("legacy control must begin with a genuine v1 human preparation: %v", err)
			}
			if envelope == "v2_envelope_with_v1_claim" {
				stored.Version = 2
				raw, err := encodeTaskKnowledge(stored)
				if err != nil {
					t.Fatalf("a v2 envelope retaining the original v1 nested proof must decode strictly: %v", err)
				}
				consumer.fx.Exec(t, `UPDATE agent_task_queue SET knowledge_context_version=2,knowledge_delivery=$2 WHERE id=$1`, oldTask, raw)
			}
			before, err := consumer.tasks.Queries.GetAgentTask(consumer.ctx, oldTask)
			if err != nil {
				t.Fatal(err)
			}
			original, err := decodeTaskKnowledge(before.KnowledgeContextVersion, before.KnowledgeDelivery)
			if err != nil || original == nil || original.Claims[0].Descriptor.ProtocolVersion != 1 || original.Claims[0].Prepared.Manifest.ProtocolVersion != 1 || len(original.Claims[0].Attempts) != 1 || original.Claims[0].Attempts[0].Authorization.ProtocolVersion != 1 {
				t.Fatalf("envelope migration must preserve the original nested versions and attempts: %v", err)
			}
			for _, observation := range original.Claims[0].Attempts[0].Observations {
				if observation.ProtocolVersion != 1 {
					t.Fatal("envelope migration must preserve each original v1 observation")
				}
			}
			consumer.nextTurn(t)
			consumer.inv.ProtocolVersion = 2
			resumed, prior, err := consumer.tasks.BuildTaskKnowledgeClaim(consumer.ctx, consumer.task, &consumer.inv, native)
			if err != nil || resumed == nil || prior != native || resumed.Session.NativeSessionID != native || resumed.Session.Generation != originalDescriptor.Session.Generation {
				t.Fatalf("v2 invocation must resume an unchanged legacy human binding before the configuration changes: prior=%q err=%v", prior, err)
			}
			consumer.fx.Exec(t, `UPDATE agent SET instructions='Changed legacy human configuration' WHERE id=$1`, consumer.task.AgentID)
			want := originalDescriptor.Session
			want.NativeSessionID = native
			binding, managed, manifest, err := consumer.tasks.taskKnowledgeExposure(consumer.ctx, consumer.tasks.Queries, consumer.task, want, true)
			if err != nil || binding != nil || !managed || manifest.ProtocolVersion != 1 || len(manifest.Entries) != 0 {
				t.Fatalf("pure legacy fallback must use the nested manifest version rather than its outer envelope: managed=%v version=%d err=%v", managed, manifest.ProtocolVersion, err)
			}
			fresh, prior, err := consumer.tasks.BuildTaskKnowledgeClaim(consumer.ctx, consumer.task, &consumer.inv, native)
			if err != nil || fresh == nil || fresh.ProtocolVersion != 2 || prior != "" || fresh.Session.NativeSessionID != "" || fresh.Session.Generation == originalDescriptor.Session.Generation || protocol.ValidateTaskKnowledgeSession(fresh.Session) != nil {
				t.Fatalf("a v2 invocation of incompatible pure legacy human history must select a new generation with empty native: prior=%q err=%v", prior, err)
			}
			after, err := consumer.tasks.Queries.GetAgentTask(consumer.ctx, oldTask)
			if err != nil {
				t.Fatal(err)
			}
			retained, err := decodeTaskKnowledge(after.KnowledgeContextVersion, after.KnowledgeDelivery)
			if err != nil {
				t.Fatal(err)
			}
			if before.KnowledgeContextVersion != after.KnowledgeContextVersion || !bytes.Equal(before.KnowledgeDelivery, after.KnowledgeDelivery) || before.SessionID != after.SessionID || original.Claims[0].Prepared.ManifestSHA256 != retained.Claims[0].Prepared.ManifestSHA256 || !sameTaskKnowledgeManifest(original.Claims[0].Prepared.Manifest, retained.Claims[0].Prepared.Manifest) || len(original.Claims[0].Attempts) != len(retained.Claims[0].Attempts) {
				t.Fatal("legacy resume or fresh selection changed the original manifest, nested versions or attempts")
			}
		})
	}
}
