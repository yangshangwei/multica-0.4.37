package service

import (
	"encoding/json"
	"strconv"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func knowledgeAcceptedSourceFixture(t *testing.T, result, basis string, beforeDelivery ...func(*collaborationQueueFixture)) (*collaborationQueueFixture, protocol.CollaborationKnowledgeSource) {
	t.Helper()
	f := newCollaborationHandoffFixture(t)
	project := f.fx.Project(t, "Accepted knowledge source")
	f.fx.Exec(t, `UPDATE issue SET project_id=$2 WHERE id=$1`, f.issue.Issue.ID, project)
	for _, setup := range beforeDelivery {
		setup(f)
	}
	submit := handoffAcceptance(t, f, 1, result)
	submit.Basis = basis
	receipt, err := f.svc.SubmitAcceptance(collaborationContext(f.handler), submit)
	if err != nil || receipt.DeliveryConfirmation == nil {
		t.Fatal("formal delivery fixture failed:", err)
	}
	accept := lifecycleParams(t, f, "complete", "done", 2, 1)
	accept.SubmissionID = &receipt.DeliveryConfirmation.SubmissionID
	if _, err = f.svc.ChangeLifecycle(f.ctx, accept); err != nil {
		t.Fatal("formal acceptance fixture failed:", err)
	}
	summary := deliverySummaryForTest(t, f, f.ctx)
	if summary.Latest == nil || summary.Latest.Decision == nil || summary.Latest.Decision.Kind != "accepted" {
		t.Fatal("fixture lacks exact accepted delivery")
	}
	canonical, err := json.Marshal(struct {
		Result string `json:"result"`
		Basis  string `json:"basis"`
	}{result, basis})
	if err != nil {
		t.Fatal(err)
	}
	return f, protocol.CollaborationKnowledgeSource{Kind: "delivery", IssueID: util.UUIDToString(f.issue.Issue.ID), SubmissionID: util.UUIDToString(summary.Latest.ID), DeliveryEventSeq: strconv.FormatInt(summary.Latest.EventSeq, 10), AcceptedEventSeq: strconv.FormatInt(summary.Latest.Decision.EventSeq, 10), SHA256: protocol.CollaborationKnowledgeHash(string(canonical))}
}

func TestKnowledgeDeliverySourceExactAcceptedSnapshot(t *testing.T) {
	f, source := knowledgeAcceptedSourceFixture(t, "完整结果 <&>\u2028\u2029\n保留空格  ", "依据\n\"引用\" 与 \\路径")
	available, err := f.svc.KnowledgeSourceAvailability(f.ctx, f.svc.Queries, f.issue.Issue.ID, []protocol.CollaborationKnowledgeSource{source})
	if err != nil || !available {
		t.Fatalf("exact accepted delivery was unavailable: %t %v", available, err)
	}
	for name, change := range map[string]func(*protocol.CollaborationKnowledgeSource){
		"other submission": func(s *protocol.CollaborationKnowledgeSource) { s.SubmissionID = uuid.NewString() },
		"other acceptance": func(s *protocol.CollaborationKnowledgeSource) { s.AcceptedEventSeq = "999999" },
		"other delivery":   func(s *protocol.CollaborationKnowledgeSource) { s.DeliveryEventSeq = "1" },
		"quote-only hash": func(s *protocol.CollaborationKnowledgeSource) {
			s.SHA256 = protocol.CollaborationKnowledgeHash("完整结果")
		},
	} {
		t.Run(name, func(t *testing.T) {
			changed := source
			change(&changed)
			available, err := f.svc.KnowledgeSourceAvailability(f.ctx, f.svc.Queries, f.issue.Issue.ID, []protocol.CollaborationKnowledgeSource{changed})
			if err != nil || available {
				t.Fatalf("changed delivery identity accepted: %t %v", available, err)
			}
		})
	}
	access := mustAccess(t, f, f.ctx)
	reopen := lifecycleParams(t, f, "reopen", "todo", access.Revision, access.LifecycleEpoch)
	if _, err := f.svc.ChangeLifecycle(f.ctx, reopen); err != nil {
		t.Fatal(err)
	}
	available, err = f.svc.KnowledgeSourceAvailability(f.ctx, f.svc.Queries, f.issue.Issue.ID, []protocol.CollaborationKnowledgeSource{source})
	if err != nil || !available {
		t.Fatal("reopen erased an immutable lawful delivery source:", err)
	}
}

func TestKnowledgeDeliveryCanonicalPreservesFullFieldsAndEscaping(t *testing.T) {
	canonical, err := knowledgeDeliveryCanonical("结果<&>\u2028\u2029", "证据")
	if err != nil || canonical != `{"result":"结果\u003c\u0026\u003e\u2028\u2029","basis":"证据"}` {
		t.Fatalf("canonical field order or escaping changed: %q %v", canonical, err)
	}
	swapped, err := knowledgeDeliveryCanonical("证据", "结果<&>\u2028\u2029")
	if err != nil || swapped == canonical {
		t.Fatal("result and basis lost their field identities")
	}
	for _, changed := range []string{"结果<&>\u2028\u2029 ", "结果<&>\u2028\u2029\n不同结论"} {
		next, err := knowledgeDeliveryCanonical(changed, "证据")
		if err != nil || protocol.CollaborationKnowledgeHash(next) == protocol.CollaborationKnowledgeHash(canonical) {
			t.Fatal("complete changed source reused a quote-only hash")
		}
	}
	for _, malformed := range []string{"", " \n", string([]byte{0xff}), "text\x00"} {
		if _, err := knowledgeDeliveryCanonical(malformed, "证据"); err == nil {
			t.Fatal("malformed result accepted")
		}
	}
}
