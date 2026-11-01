package service

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type runtimeKnowledgeFixture struct {
	*managedCollaborationFixture
	caps     protocol.CollaborationRuntimeCapabilities
	proposal protocol.CollaborationKnowledgeProposal
	run      pgtype.UUID
	claim    protocol.CollaborationRunClaimResponse
	prepare  protocol.CollaborationRunKnowledgePrepare
	permit   protocol.CollaborationRunPermitRequest
	business string
	hash     string
}

func newRuntimeKnowledgeFixture(t *testing.T, approved, negotiated bool, ttl ...time.Duration) *runtimeKnowledgeFixture {
	t.Helper()
	f, caps := newHostCollaborationFixture(t)
	f.fx.Exec(t, `UPDATE agent_runtime SET provider='claude' WHERE id=$1`, f.runtime.ID)
	var err error
	f.runtime, err = f.svc.Queries.GetAgentRuntime(f.ctx, f.runtime.ID)
	if err != nil {
		t.Fatal(err)
	}
	registration := hostCapabilityRegistration(f)
	if _, err = f.svc.RegisterRuntimeCapabilities(f.daemonCtx, registration); err != nil {
		t.Fatal(err)
	}
	f.runtime, err = f.svc.Queries.GetAgentRuntime(f.ctx, f.runtime.ID)
	if err != nil {
		t.Fatal(err)
	}
	renewHostConsent(t, f)
	caps = registration.Capabilities
	f.svc.KnowledgeContextEnabled = true
	f.fx.Cleanup(t, `DELETE FROM collaboration_memory_version WHERE issue_id=$1`, f.issue.Issue.ID)
	f.fx.Cleanup(t, `DELETE FROM collaboration_knowledge_candidate WHERE source_issue_id=$1`, f.issue.Issue.ID)
	f.fx.Cleanup(t, `DELETE FROM collaboration_knowledge_target WHERE workspace_id=$1`, f.workspace)
	f.fx.Cleanup(t, `DELETE FROM collaboration_knowledge_invalidation WHERE target_workspace_id=$1`, f.workspace)
	p := knowledgeServiceProposal(t, f.collaborationQueueFixture)
	if len(ttl) > 0 {
		expiry := time.Now().UTC().Add(ttl[0])
		p.ExpiresAt = &expiry
	}
	if _, err = f.svc.ProposeKnowledge(f.ctx, f.issue.Issue.ID, p); err != nil {
		t.Fatal(err)
	}
	if approved {
		if _, err = f.svc.ApproveKnowledge(f.ctx, util.MustParseUUID(p.CandidateID), protocol.CollaborationKnowledgeApproveRequest{ProtocolVersion: 1, RequestID: uuid.NewString(), ExpectedStateRevision: "1", ExpectedVersion: "1", ExpectedContentSHA256: protocol.CollaborationKnowledgeHash(p.Content)}); err != nil {
			t.Fatal(err)
		}
	}
	submitHostCommand(t, f, f.creator, "first accepted author input")
	submitHostCommand(t, f, f.handler, "second accepted author input")
	resume := f.issueControl(t)
	resume.ExecutionViewVersion = 6
	if _, err = f.svc.ResumeQueue(f.ctx, CollaborationResumeQueueParams{CollaborationStopQueueParams: resume, ExpectedExecutionEpoch: f.gateEpoch}); err != nil {
		t.Fatal(err)
	}
	listed, err := f.svc.ListRuntimeRuns(f.daemonCtx, f.runtime.ID)
	if err != nil || len(listed.RunIDs) != 1 {
		t.Fatalf("run list: %v %v", listed, err)
	}
	run := util.MustParseUUID(listed.RunIDs[0])
	request := hostClaimRequest(t, f, caps)
	if negotiated {
		request.KnowledgeVersion = 1
	}
	claim, err := f.svc.ClaimRuntimeRun(f.daemonCtx, run, request)
	if err != nil {
		t.Fatal(err)
	}
	hash := auth.HashToken("mdt_" + uuid.NewString())
	f.fx.Insert(t, "daemon_token", testutil.Cols{"token_hash": hash, "workspace_id": f.workspace, "daemon_id": f.runtime.DaemonID.String, "user_id": f.creator, "auth_version": 1, "expires_at": time.Now().Add(time.Hour)})
	permit := hostPermitRequest(claim, 1, "provider")
	authorized, err := f.svc.PermitRuntimeRun(f.daemonCtx, run, permit)
	if err != nil {
		t.Fatal(err)
	}
	handoff, err := f.svc.RuntimeHandoffSnapshot(context.Background(), run, authorized.BusinessToken, protocol.CollaborationHandoffSnapshotRequest{RequestID: uuid.NewString(), MemoryVersion: "1"})
	if err != nil {
		t.Fatal(err)
	}
	return &runtimeKnowledgeFixture{managedCollaborationFixture: f, caps: caps, proposal: p, run: run, claim: claim, permit: permit, business: authorized.BusinessToken, hash: hash, prepare: protocol.CollaborationRunKnowledgePrepare{ProtocolVersion: 1, Identity: claim.Identity, RequestID: uuid.NewString(), HandoffID: handoff.SnapshotID, InvocationFingerprint: strings.Repeat("a", 64)}}
}

func (f *runtimeKnowledgeFixture) authorize(t *testing.T, bundle protocol.CollaborationRunKnowledgeBundle) (protocol.CollaborationRunKnowledgeAuthorization, error) {
	t.Helper()
	p := protocol.CollaborationRunKnowledgeAuthorization{CollaborationRunKnowledgePrepare: f.prepare, ManifestSHA256: bundle.ManifestSHA256, PromptSHA256: strings.Repeat("b", 64), AttemptSeq: f.permit.AttemptSeq, AttemptNonce: f.permit.AttemptNonce}
	_, err := f.svc.BindRuntimeHandoff(context.Background(), f.run, f.business, protocol.CollaborationHandoffBinding{SnapshotID: p.HandoffID, PromptSHA256: p.PromptSHA256, AttemptSeq: p.AttemptSeq, AttemptNonce: p.AttemptNonce})
	if err != nil {
		return p, err
	}
	return f.svc.AuthorizeRuntimeKnowledge(f.daemonCtx, f.run, f.hash, p)
}

func TestCollaborationKnowledgeRuntimeApprovedExactManifestAndOriginalInputs(t *testing.T) {
	f := newRuntimeKnowledgeFixture(t, true, true)
	if f.claim.KnowledgeContext == nil || f.claim.KnowledgeContext.Namespace != f.claim.Identity.RunID || len(f.claim.Inputs) != 2 {
		t.Fatal("claim omitted fresh namespace or original authors")
	}
	raw, _ := json.Marshal(f.claim)
	if strings.Contains(string(raw), f.proposal.Content) || strings.Contains(string(raw), f.proposal.CandidateID) {
		t.Fatal("claim disclosed knowledge")
	}
	bundle, err := f.svc.PrepareRuntimeKnowledge(f.daemonCtx, f.run, f.hash, f.prepare)
	if err != nil || len(bundle.Contributions) != 1 || bundle.Contributions[0].Content != f.proposal.Content {
		t.Fatalf("prepared bundle: %v %d", err, len(bundle.Contributions))
	}
	replay, err := f.svc.PrepareRuntimeKnowledge(f.daemonCtx, f.run, f.hash, f.prepare)
	if err != nil || !reflect.DeepEqual(replay, bundle) {
		t.Fatalf("original replay: %v", err)
	}
	if _, err = f.authorize(t, bundle); err != nil {
		t.Fatalf("actual attempt binding: %v", err)
	}
	if _, err = f.authorize(t, bundle); err != nil {
		t.Fatalf("authorization lost reply recovery: %v", err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_run_event WHERE run_id=$1 AND kind='knowledge_preparation'`, f.run) != 1 || f.fx.Count(t, `SELECT count(*) FROM collaboration_run_event WHERE run_id=$1 AND kind='knowledge_authorization'`, f.run) != 1 {
		t.Fatal("replay duplicated metadata")
	}
	var metadata string
	f.fx.QueryRow(t, `SELECT string_agg(payload::text,'') FROM collaboration_run_event WHERE run_id=$1 AND kind LIKE 'knowledge_%'`, f.run).Scan(&metadata)
	if strings.Contains(metadata, f.proposal.Content) || strings.Contains(metadata, f.proposal.Summary) {
		t.Fatal("private material copied into metadata")
	}
	page, err := f.svc.RunRecords(WithCollaborationExecutionViewVersion(f.ctx, 6), f.issue.Issue.ID, f.run, CollaborationRunRecordsQuery{Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ = json.Marshal(page)
	if strings.Contains(string(raw), f.proposal.CandidateID) || strings.Contains(string(raw), f.proposal.Content) || len(page.Items) != 2 {
		t.Fatal("generic records exposed private metadata or lost inputs")
	}
}

func TestCollaborationKnowledgeRuntimeWithdrawExpiryPurgeStopFutureDisclosure(t *testing.T) {
	for _, state := range []string{"pending", "withdrawn", "expired", "purged"} {
		t.Run(state, func(t *testing.T) {
			var ttl []time.Duration
			if state == "expired" {
				ttl = []time.Duration{2 * time.Second}
			}
			f := newRuntimeKnowledgeFixture(t, state != "pending", true, ttl...)
			if state != "pending" {
				bundle, err := f.svc.PrepareRuntimeKnowledge(f.daemonCtx, f.run, f.hash, f.prepare)
				if err != nil {
					t.Fatal(err)
				}
				authorization, err := f.authorize(t, bundle)
				if err != nil {
					t.Fatal(err)
				}
				switch state {
				case "withdrawn":
					_, err = f.svc.WithdrawKnowledge(f.ctx, util.MustParseUUID(f.proposal.CandidateID), protocol.CollaborationKnowledgeDecisionRequest{ProtocolVersion: 1, RequestID: uuid.NewString(), ExpectedStateRevision: "2", Reason: "withdraw current contribution"})
					if err != nil {
						t.Fatal(err)
					}
				case "expired":
					time.Sleep(time.Until(*f.proposal.ExpiresAt) + 10*time.Millisecond)
				case "purged":
					f.fx.Exec(t, `UPDATE collaboration_memory_version SET content='',purged_at=now(),purged_by_user_id=$2,purge_reason='remove source' WHERE issue_id=$1`, f.issue.Issue.ID, f.creator)
				}
				if _, err = f.svc.PrepareRuntimeKnowledge(f.daemonCtx, f.run, f.hash, f.prepare); err == nil {
					t.Fatal("lost response replay redisclosed invalid knowledge")
				}
				if _, err = f.svc.AuthorizeRuntimeKnowledge(f.daemonCtx, f.run, f.hash, authorization); err == nil {
					t.Fatal("stale knowledge was launchable")
				}
				f.svc.KnowledgeContextEnabled = false
				status, err := f.svc.RuntimeKnowledgeStatus(context.Background(), f.run, f.claim.TerminationToken)
				if err != nil || !status.Prepared || status.RequestID != f.prepare.RequestID {
					t.Fatalf("metadata recovery after revoke: %v", err)
				}
				raw, _ := json.Marshal(status)
				if strings.Contains(string(raw), f.proposal.CandidateID) || strings.Contains(string(raw), "PRIVATE_KNOWLEDGE") {
					t.Fatal("status leaked private material")
				}
				ended := hostLaunchEnded(f.permit, false)
				if _, err = f.svc.AcceptRuntimeLaunchTermination(context.Background(), f.run, f.claim.TerminationToken, ended); err != nil {
					t.Fatal(err)
				}
				if _, err = f.svc.AcceptRuntimeProof(context.Background(), f.run, f.claim.TerminationToken, hostFinalProof(t, f.claim, ended)); err != nil {
					t.Fatal(err)
				}
			} else {
				bundle, err := f.svc.PrepareRuntimeKnowledge(f.daemonCtx, f.run, f.hash, f.prepare)
				if err != nil || len(bundle.Contributions) != 0 {
					t.Fatalf("pending affected input: %v", err)
				}
			}
		})
	}
}

func TestCollaborationKnowledgeRuntimeNegotiationCannotUpgradeOrDowngrade(t *testing.T) {
	for _, negotiated := range []bool{false, true} {
		t.Run(map[bool]string{false: "legacy", true: "knowledge"}[negotiated], func(t *testing.T) {
			f := newRuntimeKnowledgeFixture(t, true, negotiated)
			request := hostClaimRequest(t, f.managedCollaborationFixture, f.caps)
			if !negotiated {
				request.KnowledgeVersion = 1
			}
			claim, err := f.svc.ClaimRuntimeRun(f.daemonCtx, f.run, request)
			if negotiated && err == nil {
				t.Fatal("admitted claim downgraded")
			}
			if !negotiated && (err != nil || claim.KnowledgeContext != nil) {
				t.Fatalf("legacy claim upgraded: %v", err)
			}
			if !negotiated {
				if _, err = f.svc.PrepareRuntimeKnowledge(f.daemonCtx, f.run, f.hash, f.prepare); err == nil {
					t.Fatal("unnegotiated run obtained private knowledge")
				}
			}
		})
	}
}

func TestCollaborationKnowledgeRuntimeEveryAuthorAndCurrentOwnerMustRemainAuthorized(t *testing.T) {
	for _, person := range []string{"second_author", "runtime_owner"} {
		t.Run(person, func(t *testing.T) {
			f := newRuntimeKnowledgeFixture(t, true, true)
			bundle, err := f.svc.PrepareRuntimeKnowledge(f.daemonCtx, f.run, f.hash, f.prepare)
			if err != nil {
				t.Fatal(err)
			}
			if person == "second_author" {
				f.fx.Exec(t, `UPDATE issue_participant SET revoked_at=now(),epoch=epoch+1 WHERE issue_id=$1 AND user_id=$2`, f.issue.Issue.ID, f.handler)
			} else {
				f.fx.Exec(t, `UPDATE agent_runtime SET owner_id=$2 WHERE id=$1`, f.runtime.ID, f.outsider)
			}
			if _, err = f.svc.PrepareRuntimeKnowledge(f.daemonCtx, f.run, f.hash, f.prepare); err == nil {
				t.Fatal("changed consumer authority redisclosed knowledge")
			}
			if _, err = f.authorize(t, bundle); err == nil {
				t.Fatal("changed authority launched knowledge")
			}
		})
	}
}

func TestCollaborationKnowledgeRuntimeLateRunLockIsNowait(t *testing.T) {
	f := newRuntimeKnowledgeFixture(t, true, true)
	tx, err := f.pool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(t.Context())
	if _, err = tx.Exec(t.Context(), `SELECT id FROM collaboration_run WHERE id=$1 FOR UPDATE`, f.run); err != nil {
		t.Fatal(err)
	}
	// The group-run lock stays NOWAIT. The bounded busy retry re-runs the graph
	// but never waits on the holder: a blocking wait would last until this
	// deadline, while the retry refuses within the server's busy budget.
	ctx, cancel := context.WithTimeout(f.daemonCtx, 5*time.Second)
	defer cancel()
	started := time.Now()
	_, err = f.svc.PrepareRuntimeKnowledge(ctx, f.run, f.hash, f.prepare)
	if elapsed := time.Since(started); !knowledgeBusy(err) || ctx.Err() != nil || elapsed > taskKnowledgeBusyBudget+time.Second {
		t.Fatalf("late run lock waited: %v %s", err, elapsed)
	}
}

func TestCollaborationKnowledgeRuntimeAudienceCannotReadAnotherSourcesPrivateBody(t *testing.T) {
	f := newRuntimeKnowledgeFixture(t, false, true)
	p := f.params(t)
	p.HandlerID = util.MustParseUUID(f.creator)
	source, err := f.svc.Create(f.ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	f.fx.Cleanup(t, `DELETE FROM collaboration_memory_version WHERE issue_id=$1`, source.Issue.ID)
	f.fx.Cleanup(t, `DELETE FROM collaboration_knowledge_candidate WHERE source_issue_id=$1`, source.Issue.ID)
	f.fx.Cleanup(t, `DELETE FROM collaboration_operation WHERE scope_kind='issue' AND scope_id=$1`, source.Issue.ID)
	sourceFixture := *f.collaborationQueueFixture
	sourceFixture.issue = source
	proposal := knowledgeServiceProposal(t, &sourceFixture)
	proposal.Content = "PRIVATE_OTHER_SOURCE_NOT_SHARED_WITH_SECOND_AUTHOR"
	if _, err = f.svc.ProposeKnowledge(f.ctx, source.Issue.ID, proposal); err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.ApproveKnowledge(f.ctx, util.MustParseUUID(proposal.CandidateID), knowledgeApproveRequest(proposal)); err != nil {
		t.Fatal(err)
	}
	bundle, err := f.svc.PrepareRuntimeKnowledge(f.daemonCtx, f.run, f.hash, f.prepare)
	if err != nil || len(bundle.Contributions) != 0 {
		t.Fatalf("collaboration audience widened source: %v", err)
	}
	raw, _ := json.Marshal(bundle)
	if strings.Contains(string(raw), proposal.CandidateID) || strings.Contains(string(raw), proposal.Content) {
		t.Fatal("omitted source leaked identifying material")
	}
}

func TestCollaborationKnowledgeRuntimeChangedRequestCannotReplacePreparedManifest(t *testing.T) {
	f := newRuntimeKnowledgeFixture(t, true, true)
	bundle, err := f.svc.PrepareRuntimeKnowledge(f.daemonCtx, f.run, f.hash, f.prepare)
	if err != nil {
		t.Fatal(err)
	}
	changed := f.prepare
	changed.RequestID = uuid.NewString()
	if _, err = f.svc.PrepareRuntimeKnowledge(f.daemonCtx, f.run, f.hash, changed); !collaborationErrorCode(err, "knowledge_context_changed") {
		t.Fatalf("new key replaced preparation: %v", err)
	}
	if _, err = f.authorize(t, bundle); err != nil {
		t.Fatal(err)
	}
	changed = f.prepare
	changed.InvocationFingerprint = strings.Repeat("c", 64)
	if _, err = f.svc.PrepareRuntimeKnowledge(f.daemonCtx, f.run, f.hash, changed); !collaborationErrorCode(err, "knowledge_context_changed") {
		t.Fatalf("configuration replaced original snapshot: %v", err)
	}
}

// A managed claim under the daemon credential holds the runtime owner FOR UPDATE
// and then the installation namespace; the knowledge fence meets the first as a
// NOWAIT conflict and the second as a late try-lock. Neither may end the run
// with a busy refusal: the whole locked graph is rolled back and re-run.
func TestCollaborationKnowledgePrepareWaitsOutSiblingNamespaceHolder(t *testing.T) {
	for _, tc := range []struct{ step, holder string }{{"prepare", "managed_claim"}, {"prepare", "installation_namespace"}, {"authorize", "managed_claim"}, {"authorize", "installation_namespace"}} {
		step := tc.step
		t.Run(tc.step+"/"+tc.holder, func(t *testing.T) {
			f := newRuntimeKnowledgeFixture(t, true, true)
			var authorization protocol.CollaborationRunKnowledgeAuthorization
			if step == "authorize" {
				bundle, err := f.svc.PrepareRuntimeKnowledge(f.daemonCtx, f.run, f.hash, f.prepare)
				if err != nil {
					t.Fatal(err)
				}
				authorization = protocol.CollaborationRunKnowledgeAuthorization{CollaborationRunKnowledgePrepare: f.prepare, ManifestSHA256: bundle.ManifestSHA256, PromptSHA256: strings.Repeat("b", 64), AttemptSeq: f.permit.AttemptSeq, AttemptNonce: f.permit.AttemptNonce}
				if _, err = f.svc.BindRuntimeHandoff(context.Background(), f.run, f.business, protocol.CollaborationHandoffBinding{SnapshotID: authorization.HandoffID, PromptSHA256: authorization.PromptSHA256, AttemptSeq: authorization.AttemptSeq, AttemptNonce: authorization.AttemptNonce}); err != nil {
					t.Fatal(err)
				}
			}
			held, release := holdKnowledgeSiblingLock(t, f.pool, func(tx pgx.Tx) error {
				q := f.svc.Queries.WithTx(tx)
				if tc.holder == "installation_namespace" {
					return q.LockInstallationNamespace(f.ctx, installationNamespace(f.runtime.WorkspaceID, f.runtime.DaemonID.String))
				}
				_, err := LockManagedRuntime(f.daemonCtx, RuntimeLookup{Queries: q}, f.runtime)
				return err
			})
			<-held
			type outcome struct {
				bundle   protocol.CollaborationRunKnowledgeBundle
				err      error
				returned time.Time
			}
			done := make(chan outcome, 1)
			ctx, cancel := context.WithTimeout(f.daemonCtx, 5*time.Second)
			defer cancel()
			go func() {
				var got outcome
				if step == "prepare" {
					got.bundle, got.err = f.svc.PrepareRuntimeKnowledge(ctx, f.run, f.hash, f.prepare)
				} else {
					_, got.err = f.svc.AuthorizeRuntimeKnowledge(ctx, f.run, f.hash, authorization)
				}
				got.returned = time.Now()
				done <- got
			}()
			select {
			case got := <-done:
				release()
				t.Fatalf("%s returned while the sibling held the namespace: %v", step, got.err)
			case <-time.After(300 * time.Millisecond):
			}
			released := release()
			var got outcome
			select {
			case got = <-done:
			case <-time.After(3 * time.Second):
				t.Fatalf("%s did not finish after the sibling released the namespace", step)
			}
			if got.err != nil || !got.returned.After(released) {
				t.Fatalf("lock contention ended the collaboration %s: %v", step, got.err)
			}
			if step == "prepare" && (len(got.bundle.Contributions) != 1 || got.bundle.Contributions[0].Content != f.proposal.Content) {
				t.Fatal("retried preparation lost the approved bundle")
			}
			if step == "authorize" && f.fx.Count(t, `SELECT count(*) FROM collaboration_run_event WHERE run_id=$1 AND kind='knowledge_authorization'`, f.run) != 1 {
				t.Fatal("retried authorization did not store exactly one receipt")
			}
		})
	}
}
