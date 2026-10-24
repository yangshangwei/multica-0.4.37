package service

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestCollaborationPreparationRecordsFrozenSummary(t *testing.T) {
	f := newManagedCollaborationFixture(t)
	f.fx.Exec(t, `UPDATE agent SET instructions='PRIVATE-INVOCATION-INSTRUCTIONS' WHERE id=$1`, f.grant.AgentID)
	registration := preparationRegistrationRequest(f)
	registration.Capabilities.Git = &protocol.CollaborationPreparationGit{ObjectFormat: "sha1", CommitOID: strings.Repeat("b", 40), BundleDigest: strings.Repeat("c", 64)}
	if _, err := f.svc.RegisterRuntimeCapabilities(f.daemonCtx, registration); err != nil {
		t.Fatal(err)
	}
	var err error
	f.runtime, err = f.svc.Queries.GetAgentRuntime(f.ctx, f.runtime.ID)
	if err != nil {
		t.Fatal(err)
	}
	refreshPreparationConsent(t, f)
	file := preparationDeliveredFile(t, f)
	runID := preparationCandidate(t, f)
	_, claim := claimPreparedCandidate(t, f, runID)
	page, err := f.svc.RunRecords(f.ctx, f.issue.Issue.ID, runID, CollaborationRunRecordsQuery{Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(page)
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]json.RawMessage
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatal(err)
	}
	if len(wire["preparation"]) == 0 {
		t.Fatal("claimed run omitted its recorded preparation summary")
	}
	expected := &CollaborationRunPreparation{Version: 1,
		Attachments: []CollaborationRunPreparationFile{{ID: util.UUIDToString(file.ID), Version: strconv.FormatInt(file.Version, 10), Name: file.Name, SizeBytes: file.SizeBytes}},
		Git:         &CollaborationRunPreparationGit{ObjectFormat: "sha1", CommitOID: strings.Repeat("b", 40)}, Stages: []CollaborationRunPreparationStage{}}
	if !reflect.DeepEqual(page.Preparation, expected) || page.Run.BusinessOutcome != nil || page.Run.ProofState != "unknown" || page.Run.SettledAt != nil || page.Recording.Status != "not_recorded" {
		t.Fatal("frozen preparation was omitted, changed or promoted into an outcome/proof")
	}
	if claim.Preparation.Invocation.Instructions != "PRIVATE-INVOCATION-INSTRUCTIONS" {
		t.Fatal("private sentinel was not in the frozen source manifest")
	}
	for _, private := range []string{"PRIVATE-INVOCATION-INSTRUCTIONS", "manifest_digest", claim.Preparation.ManifestDigest, "bundle_digest", "destination", "attempt_nonce", "launch_nonce", "installation_id", strings.Repeat("a", 64), strings.Repeat("c", 64)} {
		if strings.Contains(string(raw), private) {
			t.Fatalf("human preparation leaked %s", private)
		}
	}
	// New delivery and later resource edits cannot rewrite the admitted files or
	// make old human records depend on a live runtime.
	preparationDeliveredFile(t, f)
	f.fx.Exec(t, `UPDATE agent SET instructions='PRIVATE-LATER-INSTRUCTIONS' WHERE id=$1`, f.grant.AgentID)
	page, err = f.svc.RunRecords(f.ctx, f.issue.Issue.ID, runID, CollaborationRunRecordsQuery{Limit: 50})
	if err != nil || !reflect.DeepEqual(page.Preparation, expected) {
		t.Fatalf("new delivery/configuration replaced frozen preparation: %v", err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_runtime_reservation WHERE run_id=$1 AND released_at IS NULL`, runID) != 1 {
		t.Fatal("preparation read released unknown runtime capacity")
	}
}

func TestCollaborationPreparationRecordsWatermarkAndBoundedStages(t *testing.T) {
	f := preparedRuntimeFixture(t)
	f.submitManaged(t, f.creator, "first recorded input")
	runID := preparationCandidate(t, f)
	_, claim := claimPreparedCandidate(t, f, runID)
	_, permit := permitPreparedClaim(t, f, runID, claim)
	before, err := f.svc.Queries.GetCollaborationRuntimeRun(f.ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	observe := func(stage, state, code string) {
		t.Helper()
		_, err := f.svc.RecordRuntimePreparation(context.Background(), runID, permit.BusinessToken, protocol.CollaborationPreparationObservation{
			Identity: claim.Identity, EventID: uuid.NewString(), AttemptSeq: "1", AttemptNonce: permit.AttemptNonce,
			ManifestDigest: claim.Preparation.ManifestDigest, Stage: stage, State: state, Code: code})
		if err != nil {
			t.Fatal(err)
		}
	}
	// Repeated factual observations stay bounded in the response, while the
	// original retained events remain intact.
	for range 12 {
		observe("attachments", "prepared", "")
	}
	first, err := f.svc.RunRecords(f.ctx, f.issue.Issue.ID, runID, CollaborationRunRecordsQuery{Limit: 1})
	if err != nil || first.NextCursor == nil || first.Preparation == nil || len(first.Preparation.Stages) != 1 || first.Preparation.Stages[0].State != "prepared" {
		t.Fatalf("first page did not capture one latest preparation observation: %v", err)
	}
	observe("attachments", "failed", "attachment_unavailable")
	observe("workspace", "failed", "input_limit_exceeded")
	next, err := f.svc.RunRecords(f.ctx, f.issue.Issue.ID, runID, CollaborationRunRecordsQuery{Limit: 1, Cursor: *first.NextCursor})
	if err != nil || !reflect.DeepEqual(next.Preparation, first.Preparation) || next.NextCursor != nil || len(next.Items) != 1 {
		t.Fatalf("later observations were spliced into the old page watermark: %v", err)
	}
	fresh, err := f.svc.RunRecords(f.ctx, f.issue.Issue.ID, runID, CollaborationRunRecordsQuery{Limit: 50})
	if err != nil || fresh.Preparation == nil || len(fresh.Preparation.Stages) != 2 || fresh.Preparation.Stages[0].Code != "attachment_unavailable" || fresh.Preparation.Stages[1].Code != "input_limit_exceeded" {
		t.Fatalf("refresh did not show exact later stage failures: %v", err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_run_event WHERE run_id=$1 AND kind='preparation'`, runID) != 14 {
		t.Fatal("summary changed retained preparation history")
	}
	after, err := f.svc.Queries.GetCollaborationRuntimeRun(f.ctx, runID)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("preparation observations or reads mutated business/proof state: %v", err)
	}
}

func TestCollaborationPreparationRecordsManifestCorrelation(t *testing.T) {
	f := preparedRuntimeFixture(t)
	runID := preparationCandidate(t, f)
	_, claim := claimPreparedCandidate(t, f, runID)
	run, err := f.svc.Queries.GetCollaborationRuntimeRun(f.ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := recordedPreparationManifest(f.issue.Issue.ID, run, claim.Inputs); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*protocol.CollaborationPreparation){
		"other run":       func(p *protocol.CollaborationPreparation) { p.Identity.RunID = uuid.NewString() },
		"other issue":     func(p *protocol.CollaborationPreparation) { p.Identity.IssueID = uuid.NewString() },
		"other group":     func(p *protocol.CollaborationPreparation) { p.Identity.GroupID = uuid.NewString() },
		"other runtime":   func(p *protocol.CollaborationPreparation) { p.Resource.RuntimeID = uuid.NewString() },
		"other agent":     func(p *protocol.CollaborationPreparation) { p.Resource.AgentID = uuid.NewString() },
		"other owner":     func(p *protocol.CollaborationPreparation) { p.Resource.OwnerUserID = uuid.NewString() },
		"other workspace": func(p *protocol.CollaborationPreparation) { p.Resource.ExecutionWorkspaceID = uuid.NewString() },
		"other input":     func(p *protocol.CollaborationPreparation) { p.CommandIDs[0] = uuid.NewString() },
		"other epoch":     func(p *protocol.CollaborationPreparation) { p.Resource.RuntimeConfigEpoch = "900" },
	} {
		t.Run(name, func(t *testing.T) {
			var snapshot protocol.CollaborationPreparation
			if err := json.Unmarshal(run.PreparationSnapshot, &snapshot); err != nil {
				t.Fatal(err)
			}
			mutate(&snapshot)
			if err := protocol.SealCollaborationPreparation(&snapshot); err != nil {
				t.Fatal(err)
			}
			changed := run
			changed.PreparationSnapshot, err = json.Marshal(snapshot)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := recordedPreparationManifest(f.issue.Issue.ID, changed, claim.Inputs); !errors.Is(err, errInvalidCollaborationLedger) {
				t.Fatalf("valid but unrelated manifest escaped correlation: %v", err)
			}
		})
	}
	for _, raw := range [][]byte{nil, []byte(`{"version":1,"invocation":"PRIVATE_INVALID"}`), []byte(strings.Repeat("x", 2<<20+1))} {
		changed := run
		changed.PreparationSnapshot = raw
		if _, err := recordedPreparationManifest(f.issue.Issue.ID, changed, claim.Inputs); !errors.Is(err, errInvalidCollaborationLedger) || strings.Contains(err.Error(), "PRIVATE_INVALID") {
			t.Fatal("corrupt manifest was accepted or printed private data")
		}
	}
	var mismatched protocol.CollaborationPreparation
	if err := json.Unmarshal(run.PreparationSnapshot, &mismatched); err != nil {
		t.Fatal(err)
	}
	mismatched.ManifestDigest = strings.Repeat("f", 64)
	changed := run
	changed.PreparationSnapshot, err = json.Marshal(mismatched)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := recordedPreparationManifest(f.issue.Issue.ID, changed, claim.Inputs); !errors.Is(err, errInvalidCollaborationLedger) {
		t.Fatal("a valid-shaped manifest with a mismatched digest was projected")
	}
}

func TestCollaborationPreparationRecordsRetainedAfterCleanup(t *testing.T) {
	f := preparedRuntimeFixture(t)
	preparationDeliveredFile(t, f)
	runID := preparationCandidate(t, f)
	_, claim := claimPreparedCandidate(t, f, runID)
	_, permit := permitPreparedClaim(t, f, runID, claim)
	if _, err := f.svc.RecordRuntimePreparation(context.Background(), runID, permit.BusinessToken, protocol.CollaborationPreparationObservation{
		Identity: claim.Identity, EventID: uuid.NewString(), AttemptSeq: "1", AttemptNonce: permit.AttemptNonce,
		ManifestDigest: claim.Preparation.ManifestDigest, Stage: "workspace", State: "prepared"}); err != nil {
		t.Fatal(err)
	}
	before, err := f.svc.RunRecords(f.ctx, f.issue.Issue.ID, runID, CollaborationRunRecordsQuery{Limit: 50})
	if err != nil || before.Preparation == nil || len(before.Preparation.Stages) != 1 || before.Run.ProofState != "unknown" || before.Run.BusinessOutcome != nil {
		t.Fatalf("preparation acquired unrelated result/proof authority: %v", err)
	}
	if err := f.svc.RecordRuntimeOutcome(context.Background(), runID, permit.BusinessToken, recordedOutcome(claim, permit, 1)); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.AcceptRuntimeProof(context.Background(), runID, claim.TerminationToken, managedProof(claim.Identity, permit.AttemptNonce)); err != nil {
		t.Fatal(err)
	}
	f.fx.Exec(t, `DELETE FROM agent_task_queue WHERE id=(SELECT task_id FROM collaboration_run WHERE id=$1)`, runID)
	f.fx.Exec(t, `UPDATE agent SET runtime_id=NULL WHERE runtime_id=$1`, f.runtime.ID)
	f.fx.Exec(t, `DELETE FROM agent_runtime WHERE id=$1`, f.runtime.ID)
	after, err := f.svc.RunRecords(f.ctx, f.issue.Issue.ID, runID, CollaborationRunRecordsQuery{Limit: 50})
	if err != nil || !reflect.DeepEqual(before.Preparation, after.Preparation) || after.Run.ProofState != "exited_proven" || after.Usage == nil {
		t.Fatalf("actual proof or resource cleanup erased preparation facts: %v", err)
	}
}

func TestCollaborationPreparationRecordsRejectUnrelatedObservations(t *testing.T) {
	f := preparedRuntimeFixture(t)
	runID := preparationCandidate(t, f)
	_, claim := claimPreparedCandidate(t, f, runID)
	_, permit := permitPreparedClaim(t, f, runID, claim)
	for name, mutate := range map[string]func(*protocol.CollaborationPreparationObservation){
		"other run":      func(p *protocol.CollaborationPreparationObservation) { p.Identity.RunID = uuid.NewString() },
		"other issue":    func(p *protocol.CollaborationPreparationObservation) { p.Identity.IssueID = uuid.NewString() },
		"other group":    func(p *protocol.CollaborationPreparationObservation) { p.Identity.GroupID = uuid.NewString() },
		"other attempt":  func(p *protocol.CollaborationPreparationObservation) { p.AttemptNonce = uuid.NewString() },
		"other manifest": func(p *protocol.CollaborationPreparationObservation) { p.ManifestDigest = strings.Repeat("f", 64) },
		"other event":    func(p *protocol.CollaborationPreparationObservation) { p.EventID = uuid.NewString() },
		"unknown stage":  func(p *protocol.CollaborationPreparationObservation) { p.Stage = "PRIVATE_STAGE" },
		"absent Git":     func(p *protocol.CollaborationPreparationObservation) { p.Stage = "git" },
		"private reason": func(p *protocol.CollaborationPreparationObservation) { p.State, p.Code = "failed", "PRIVATE_REASON" },
	} {
		t.Run(name, func(t *testing.T) {
			eventID := uuid.NewString()
			observation := protocol.CollaborationPreparationObservation{Identity: claim.Identity, EventID: eventID, AttemptSeq: "1", AttemptNonce: permit.AttemptNonce,
				ManifestDigest: claim.Preparation.ManifestDigest, Stage: "workspace", State: "prepared"}
			mutate(&observation)
			raw, err := json.Marshal(observation)
			if err != nil {
				t.Fatal(err)
			}
			f.fx.Insert(t, "collaboration_run_event", testutil.Cols{"issue_id": f.issue.Issue.ID, "run_id": runID, "producer_event_id": eventID, "seq": 99, "kind": "preparation", "payload": json.RawMessage(raw)})
			page, err := f.svc.RunRecords(f.ctx, f.issue.Issue.ID, runID, CollaborationRunRecordsQuery{Limit: 50})
			if !errors.Is(err, errInvalidCollaborationLedger) || strings.Contains(err.Error(), "PRIVATE_") || page.Preparation != nil {
				t.Fatal("unrelated/corrupt observation was published or leaked")
			}
		})
	}
	// The query returns at most five rows even for arbitrarily many unknown
	// stage names; the fifth row is only a rejection sentinel.
	for i := range 8 {
		eventID := uuid.NewString()
		f.fx.Insert(t, "collaboration_run_event", testutil.Cols{"issue_id": f.issue.Issue.ID, "run_id": runID, "producer_event_id": eventID, "seq": 100 + i, "kind": "preparation", "payload": json.RawMessage(`{"stage":"unknown` + strconv.Itoa(i) + `"}`)})
	}
	rows, err := f.svc.Queries.ListCollaborationRunPreparationRecords(f.ctx, db.ListCollaborationRunPreparationRecordsParams{IssueID: f.issue.Issue.ID, RunID: runID, ThroughSeq: 200})
	if err != nil || len(rows) != 5 {
		t.Fatalf("preparation query exceeded its fixed result bound: %d %v", len(rows), err)
	}
	if _, err := f.svc.RunRecords(f.ctx, f.issue.Issue.ID, runID, CollaborationRunRecordsQuery{Limit: 50}); !errors.Is(err, errInvalidCollaborationLedger) {
		t.Fatal("unknown-stage sentinel became human preparation evidence")
	}
}

func TestCollaborationPreparationRecordsCurrentAccessAndLegacy(t *testing.T) {
	f := preparedRuntimeFixture(t)
	f.submitManaged(t, f.creator, "first recorded input")
	runID := preparationCandidate(t, f)
	_, claim := claimPreparedCandidate(t, f, runID)
	viewer := collaborationPasswordContext(util.MustParseUUID(f.handler), 1)
	page, err := f.svc.RunRecords(viewer, f.issue.Issue.ID, runID, CollaborationRunRecordsQuery{Limit: 1})
	if err != nil || page.Preparation == nil || page.NextCursor == nil || claim.Preparation == nil {
		t.Fatalf("current outside participant lost the public summary: %v", err)
	}
	machine := auth.WithAuthenticatedPrincipal(viewer, auth.AuthenticatedPrincipal{UserID: f.handler, Kind: auth.CredentialTask})
	if denied, err := f.svc.RunRecords(machine, f.issue.Issue.ID, runID, CollaborationRunRecordsQuery{Limit: 1}); !collaborationErrorCode(err, "not_found") || denied.Preparation != nil {
		t.Fatal("machine received human preparation records")
	}
	f.fx.Exec(t, `UPDATE issue_participant SET revoked_at=now() WHERE issue_id=$1 AND user_id=$2`, f.issue.Issue.ID, f.handler)
	if denied, err := f.svc.RunRecords(viewer, f.issue.Issue.ID, runID, CollaborationRunRecordsQuery{Limit: 1, Cursor: *page.NextCursor}); !collaborationErrorCode(err, "not_found") || denied.Preparation != nil {
		t.Fatal("old cursor retained revoked preparation content")
	}
	f.fx.Member(t, f.workspace, f.handler, "member")
	if source, err := f.svc.RunRecords(viewer, f.issue.Issue.ID, runID, CollaborationRunRecordsQuery{Limit: 1}); err != nil || !reflect.DeepEqual(source.Preparation, page.Preparation) {
		t.Fatalf("current source membership lost independent historical access: %v", err)
	}

	legacy := newManagedCollaborationFixture(t)
	legacyRun := preparationCandidate(t, legacy)
	for _, claim := range []bool{false, true} {
		if claim {
			if _, err := legacy.svc.ClaimRuntimeRun(legacy.daemonCtx, legacyRun, preparationClaimRequest(legacy)); err != nil {
				t.Fatal(err)
			}
		}
		page, err := legacy.svc.RunRecords(legacy.ctx, legacy.issue.Issue.ID, legacyRun, CollaborationRunRecordsQuery{Limit: 50})
		if err != nil || page.Preparation != nil || page.Run.ProofState != "unknown" {
			t.Fatalf("unclaimed or legacy run invented preparation/termination facts: %v", err)
		}
		raw, err := json.Marshal(page)
		if err != nil || strings.Contains(string(raw), `"preparation"`) {
			t.Fatal("legacy page did not preserve its existing wire shape")
		}
	}
}
