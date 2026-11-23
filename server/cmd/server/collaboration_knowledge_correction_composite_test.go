//go:build knowledgeintegration

package main

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestCollaborationKnowledgeHostCompositeCorrectionReplacesActualInputAndWithdrawalRefusesNextLaunch(t *testing.T) {
	f := newHostCompositeFixture(t)
	first := f.nextRun(t)
	if err := f.runner.Run(t.Context(), first, f.request); err != nil {
		t.Fatal(err)
	}
	starts := f.captures(t)
	if len(starts) != 1 || starts[0].RunID != first || !strings.Contains(hostCompositePrompt(t, starts[0]), f.proposals[0].Content) {
		t.Fatal("original approved version did not reach exactly one physical process")
	}
	var firstPreparation []byte
	f.base.managed.fx.QueryRow(t, `SELECT payload FROM collaboration_run_event WHERE run_id=$1 AND kind='knowledge_preparation'`, first).Scan(&firstPreparation)

	ownerToken := platformRouterJWT(t, f.base.owner, nil)
	prior := f.proposals[0]
	correction := prior
	correction.CandidateID, correction.RequestID = uuid.NewString(), uuid.NewString()
	correction.SupersedesCandidateID = &prior.CandidateID
	correction.Summary = "Human-reviewed corrected project decision"
	correction.Content = "PRIVATE_HOST_CORRECTED_VERSION_TWO_ONLY"
	candidatePath := "/api/collaboration/knowledge-candidates/" + correction.CandidateID
	proposal := f.base.managed.request(t, ownerToken, false, "POST", "/api/collaboration/issues/"+prior.Sources[0].IssueID+"/knowledge-candidates", correction, nil).Want(201)
	var proposed protocol.CollaborationKnowledgeReceipt
	proposal.JSON(&proposed)
	if proposed.Code != "knowledge_proposed" || proposed.Version != "2" || proposed.State != "pending" || proposed.StateRevision != "1" {
		t.Fatal("correction was not a separate pending reviewed version")
	}
	priorRow, err := f.q.GetKnowledgeCandidate(t.Context(), util.MustParseUUID(prior.CandidateID))
	if err != nil || priorRow.State != "approved" {
		t.Fatal("an unapproved correction replaced the effective version")
	}
	approval := protocol.CollaborationKnowledgeApproveRequest{ProtocolVersion: 1, RequestID: uuid.NewString(), ExpectedStateRevision: proposed.StateRevision, ExpectedVersion: proposed.Version, ExpectedContentSHA256: protocol.CollaborationKnowledgeHash(correction.Content)}
	approvedResponse := f.base.managed.request(t, ownerToken, false, "POST", candidatePath+"/approve", approval, nil).Want(200)
	var approved protocol.CollaborationKnowledgeReceipt
	approvedResponse.JSON(&approved)
	if approved.Code != "knowledge_approved" || approved.Version != "2" || approved.State != "approved" || approved.StateRevision != "2" {
		t.Fatal("exact human approval did not publish the correction")
	}
	priorRow, err = f.q.GetKnowledgeCandidate(t.Context(), util.MustParseUUID(prior.CandidateID))
	if err != nil || priorRow.State != "superseded" || priorRow.Content != prior.Content || priorRow.Version != 1 {
		t.Fatal("correction overwrote the original approved history")
	}
	correctedRow, err := f.q.GetKnowledgeCandidate(t.Context(), util.MustParseUUID(correction.CandidateID))
	if err != nil || correctedRow.State != "approved" || correctedRow.Version != 2 || correctedRow.LineageID != priorRow.LineageID || correctedRow.SupersedesCandidateID != priorRow.ID || util.UUIDToString(correctedRow.ProposedByUserID) != f.base.owner || util.UUIDToString(correctedRow.ApprovedByUserID) != f.base.owner {
		t.Fatal("correction lost lineage, exact version or human decision attribution")
	}

	const secondInput = "Use the separately approved corrected project decision."
	f.submit(t, f.owner, secondInput)
	second := f.nextRun(t)
	if second == first {
		t.Fatal("correction reused the completed run")
	}
	if err = f.runner.Run(t.Context(), second, f.request); err != nil {
		t.Fatal(err)
	}
	starts = f.captures(t)
	if len(starts) != 2 || starts[1].RunID != second || starts[1].Cwd == starts[0].Cwd || !starts[1].BusinessCredential || starts[1].BroadCredential || slices.Contains(starts[1].Args, "--resume") || !slices.Contains(starts[1].Args, "--no-session-persistence") {
		t.Fatal("corrected input did not use one separate, freshly authorized physical process")
	}
	prompt := hostCompositePrompt(t, starts[1])
	if !strings.Contains(prompt, correction.Content) || strings.Contains(prompt, prior.Content) || !strings.Contains(prompt, f.proposals[1].Content) || !strings.Contains(prompt, secondInput) || !strings.Contains(prompt, f.base.owner) {
		t.Fatal("actual corrected stdin retained superseded content or lost accepted input/author")
	}
	var prepared protocol.CollaborationRunKnowledgeBundle
	var authorization protocol.CollaborationRunKnowledgeAuthorization
	for _, call := range f.transport.snapshot() {
		if call.Path == "/api/collaboration/runtime/runs/"+second+"/knowledge-context/prepare" {
			if call.Status != 200 || json.Unmarshal(call.Response, &prepared) != nil {
				t.Fatal("corrected preparation did not use the real private route")
			}
		}
		if call.Path == "/api/collaboration/runtime/runs/"+second+"/knowledge-context/authorize" {
			if call.Status != 200 || json.Unmarshal(call.Response, &authorization) != nil {
				t.Fatal("corrected input did not use real physical-launch authorization")
			}
		}
	}
	if len(prepared.Contributions) != 2 || authorization.PromptSHA256 != protocol.CollaborationHandoffHash(prompt) {
		t.Fatal("corrected private preparation was not bound to actual stdin")
	}
	var correctedContribution bool
	for _, contribution := range prepared.Contributions {
		if contribution.CandidateID == prior.CandidateID || strings.Contains(contribution.Content, prior.Content) {
			t.Fatal("superseded contribution remained in the new preparation")
		}
		if contribution.CandidateID == correction.CandidateID {
			correctedContribution = contribution.Version == "2" && contribution.Content == correction.Content && contribution.ContentSHA256 == approval.ExpectedContentSHA256
		}
	}
	if !correctedContribution {
		t.Fatal("actual private preparation omitted the exact approved correction")
	}

	// A fresh third claim freezes the correction. Withdraw it at the real
	// authorization boundary, before the third physical process can start.
	f.submit(t, f.owner, "A later input must not reuse a withdrawn private contribution.")
	third := f.nextRun(t)
	var once sync.Once
	var withdrew bool
	f.transport.mu.Lock()
	f.transport.before = func(path string, _ []byte) {
		if path == "/api/collaboration/runtime/runs/"+third+"/knowledge-context/authorize" {
			once.Do(func() {
				response := f.base.managed.request(t, ownerToken, false, "POST", candidatePath+"/withdraw", protocol.CollaborationKnowledgeDecisionRequest{ProtocolVersion: 1, RequestID: uuid.NewString(), ExpectedStateRevision: approved.StateRevision, Reason: "Human withdrew the corrected decision before a later launch"}, nil).Want(200)
				var receipt protocol.CollaborationKnowledgeReceipt
				response.JSON(&receipt)
				withdrew = receipt.Code == "knowledge_withdrawn" && receipt.Version == "2" && receipt.State == "withdrawn"
			})
		}
	}
	f.transport.mu.Unlock()
	err = f.runner.Run(t.Context(), third, f.request)
	if err == nil || !withdrew || len(f.captures(t)) != 2 {
		t.Fatal("withdrawal did not refuse the stale later authorization before physical start")
	}
	var refused bool
	for _, call := range f.transport.snapshot() {
		if call.Path == "/api/collaboration/runtime/runs/"+third+"/knowledge-context/authorize" {
			refused = call.Status == 409
			if strings.Contains(string(call.Response), correction.Content) || strings.Contains(string(call.Response), prior.Content) {
				t.Fatal("withdrawal refusal redisclosed retained private content")
			}
		}
	}
	if !refused {
		t.Fatal("withdrawal was not observed at the real authorization route")
	}
	thirdState, err := f.q.GetCollaborationRuntimeRun(t.Context(), util.MustParseUUID(third))
	if err != nil || thirdState.ProgressState != "settled" || thirdState.ProofState != "never_launched_proven" {
		t.Fatal("withdrawal refusal failed to preserve the third run's nonlaunch proof")
	}
	for _, run := range []string{first, second} {
		state, err := f.q.GetCollaborationRuntimeRun(t.Context(), util.MustParseUUID(run))
		if err != nil || state.ProgressState != "settled" || state.ProofState != "host_exit_observed" || state.BusinessOutcome.String != "success" {
			t.Fatal("later correction or withdrawal changed an earlier completed run")
		}
	}
	var retainedPreparation []byte
	f.base.managed.fx.QueryRow(t, `SELECT payload FROM collaboration_run_event WHERE run_id=$1 AND kind='knowledge_preparation'`, first).Scan(&retainedPreparation)
	if !bytes.Equal(firstPreparation, retainedPreparation) || len(firstPreparation) == 0 {
		t.Fatal("correction or withdrawal rewrote the original exposure metadata")
	}
	for _, receipt := range []protocol.CollaborationKnowledgeReceipt{proposed, approved} {
		response := f.base.managed.request(t, ownerToken, false, "GET", candidatePath+"/operations/"+receipt.RequestID, nil, nil).Want(200)
		var recovered protocol.CollaborationKnowledgeReceipt
		response.JSON(&recovered)
		original, marshalErr := json.Marshal(receipt)
		actual, recoveredErr := json.Marshal(recovered)
		if marshalErr != nil || recoveredErr != nil || !bytes.Equal(original, actual) {
			t.Fatal("later withdrawal changed an original human operation receipt")
		}
	}
	t.Log("physical_launches=2 corrected_version=2 superseded_stdin_absent=true third_authorization_refused=true third_physical_launches=0 original_exposure_and_receipts_retained=true")
}
