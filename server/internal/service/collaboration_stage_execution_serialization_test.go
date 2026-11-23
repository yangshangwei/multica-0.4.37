package service

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/util"
)

// These bytes predate the execution extensions. The workflow hashes the Go
// request, so even adding an unused null field would strand an original key.
func TestStageReviewRequestPreservesPreExecutionCanonicalBytes(t *testing.T) {
	const issue = "11111111-1111-4111-8111-111111111111"
	const request = "22222222-2222-4222-8222-222222222222"
	const candidate = "33333333-3333-4333-8333-333333333333"
	const report = "44444444-4444-4444-8444-444444444444"
	for _, step := range []string{"code_review", "qa"} {
		t.Run(step, func(t *testing.T) {
			expires := time.Date(2026, 11, 14, 8, 0, 0, 0, time.UTC)
			p := CollaborationStageReviewParams{CollaborationWorkflowRequest: CollaborationWorkflowRequest{CollaborationQueueRequest: CollaborationQueueRequest{IssueID: util.MustParseUUID(issue), RequestID: request, ExpectedLifecycleEpoch: 1, ExpectedParticipantEpoch: 2, ExpectedAccountEpoch: 3}, ExpectedCollaborationRevision: 4}, ExpectedPolicyRevision: 5, ExpectedRoleRevision: 6, ExpectedStageRevision: 7, Step: step, Decision: "approve", Candidate: CollaborationStageCandidateRef{CandidateID: util.MustParseUUID(candidate), Version: 1, ContentDigest: strings.Repeat("a", 64)}, DependsOn: []string{}, Reason: "Reviewed exact source", ExpiresAt: &expires}
			evidence := `{"ci_digest":"` + strings.Repeat("c", 64) + `","test_report_id":null,"build":null,"deployment_id":null,"rollback_target_digest":null}`
			ci := strings.Repeat("c", 64)
			p.Evidence.CIDigest = &ci
			if step == "qa" {
				id := report
				p.Evidence = CollaborationStageReviewEvidence{TestReportID: &id, Build: &CollaborationStageBuild{ArtifactDigest: strings.Repeat("b", 64), Environment: "synthetic"}}
				evidence = `{"ci_digest":null,"test_report_id":"` + report + `","build":{"artifact_digest":"` + strings.Repeat("b", 64) + `","environment":"synthetic"},"deployment_id":null,"rollback_target_digest":null}`
			}
			want := fmt.Sprintf(`{"IssueID":%q,"RequestID":%q,"ExpectedLifecycleEpoch":1,"ExpectedParticipantEpoch":2,"ExpectedAccountEpoch":3,"ExpectedCollaborationRevision":4,"ExpectedPolicyRevision":5,"ExpectedRoleRevision":6,"ExpectedStageRevision":7,"Step":%q,"Decision":"approve","Candidate":{"CandidateID":%q,"Version":1,"ContentDigest":%q},"Evidence":%s,"DependsOn":[],"Reason":"Reviewed exact source","ExpiresAt":"2026-11-14T08:00:00Z"}`, issue, request, step, candidate, strings.Repeat("a", 64), evidence)
			raw, err := json.Marshal(p)
			if err != nil || string(raw) != want {
				t.Fatalf("original workflow request bytes changed: %s (%v)", raw, err)
			}
		})
	}
}
