package service

import (
	"context"
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const (
	CollaborationAIPolicyAssisted = "assisted"
	CollaborationAIPolicyManual   = "manual"
)

type CollaborationAIPolicyParams struct {
	CollaborationWorkflowRequest
	AIPolicy string `json:"ai_policy"`
}

// collaborationAIPolicyRefusal is shared by human input and runtime admission.
// The persisted policy is independent of grants, queue intent and launch proof.
func collaborationAIPolicyRefusal(policy string) string {
	if policy == CollaborationAIPolicyManual {
		return "task_manual"
	}
	return ""
}

func collaborationMutationRequiresAssistedPolicy(kind string) bool {
	switch kind {
	case "submit_command", "resume_queue", "register_target", "offer_resource", "adopt_resource_offer", "consent_resource",
		"propose_target_switch", "consent_target_switch", "commit_target_switch":
		return true
	default:
		return false
	}
}

// ChangeAIPolicy borrows the ordinary workflow owner, including its current
// initiator/account/version checks and exact original-key receipt. Only the
// transition to manual requests Stop; assisted never changes execution intent.
func (s *CollaborationService) ChangeAIPolicy(ctx context.Context, p CollaborationAIPolicyParams) (CollaborationWorkflowReceipt, error) {
	return s.mutateCollaborationWorkflow(ctx, p.CollaborationWorkflowRequest, pgtype.UUID{}, "change_ai_policy", p, func(t *collaborationWorkflowTx) (CollaborationWorkflowReceipt, error) {
		if p.AIPolicy != CollaborationAIPolicyManual && p.AIPolicy != CollaborationAIPolicyAssisted {
			return t.result("invalid_ai_policy", http.StatusBadRequest), nil
		}
		if p.AIPolicy == t.access.AIPolicy {
			return t.result("ai_policy_unchanged", http.StatusOK), nil
		}
		previous := t.access.AIPolicy
		if err := t.q.UpdateCollaborationAIPolicy(ctx, db.UpdateCollaborationAIPolicyParams{IssueID: t.access.Issue.ID, AiPolicy: p.AIPolicy}); err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		t.access.AIPolicy = p.AIPolicy
		var stopped *CollaborationQueueReceipt
		if p.AIPolicy == CollaborationAIPolicyManual {
			// A closed lifecycle keeps its closed queue. Its unsettled native
			// executions still need the same retained Stop/proof machinery.
			if t.session.QueueState != "closed" {
				var err error
				t.session, err = t.q.PauseCollaborationSession(ctx, db.PauseCollaborationSessionParams{SessionID: t.session.ID, PauseReason: "task_manual"})
				if err != nil {
					return CollaborationWorkflowReceipt{}, err
				}
			}
			selected := make(map[pgtype.UUID]string, len(t.targets))
			for _, target := range t.targets {
				selected[target.ID] = "stopped"
			}
			receipt, err := t.stopTargets(ctx, selected, true, "task_manual", true)
			if err != nil {
				return CollaborationWorkflowReceipt{}, err
			}
			stopped = &receipt
		}
		return t.finishWith(ctx, "ai_policy_changed", pgtype.UUID{}, stopped, map[string]any{"ai_policy": p.AIPolicy, "previous_ai_policy": previous})
	})
}
