package service

import (
	"context"
	"encoding/json"
	"time"

	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type knowledgeProtocolContextKey struct{}

// WithKnowledgeProtocolVersion carries the explicitly negotiated human API
// version. Runtime consumers bind their version to their own retained claims.
func WithKnowledgeProtocolVersion(ctx context.Context, version int) context.Context {
	return context.WithValue(ctx, knowledgeProtocolContextKey{}, version)
}

func knowledgePublicVersion(ctx context.Context) int {
	if version, ok := ctx.Value(knowledgeProtocolContextKey{}).(int); ok {
		return version
	}
	return protocol.CollaborationKnowledgeVersion
}

type knowledgeAutomationAudit struct {
	JobID          string `json:"job_id"`
	PolicyRevision string `json:"policy_revision"`
	RuleVersion    string `json:"rule_version"`
}

func knowledgeMaterialRequiresV2(material knowledgeCandidateMaterial) bool {
	for _, source := range material.Sources {
		if source.Kind == "delivery" {
			return true
		}
	}
	for _, evidence := range material.Evidence {
		if evidence.Source != nil && evidence.Source.Kind == "delivery" {
			return true
		}
	}
	return false
}

func knowledgeRequireLegacy(requiresV2 bool, err error) error {
	if err != nil {
		return err
	}
	if requiresV2 {
		return knowledgeFailure(409, "knowledge_protocol_unsupported")
	}
	return nil
}

func knowledgePayloadVersion(payload any) int {
	switch value := payload.(type) {
	case protocol.CollaborationKnowledgeProposal:
		return value.ProtocolVersion
	case protocol.CollaborationKnowledgeApproveRequest:
		return value.ProtocolVersion
	case protocol.CollaborationKnowledgeDecisionRequest:
		return value.ProtocolVersion
	default:
		return 0
	}
}

func (v KnowledgeCandidateView) marshalV2() ([]byte, error) {
	type metadata struct {
		knowledgeCandidateMetadata
		ProposedByUserID *string                   `json:"proposed_by_user_id"`
		ProposalOrigin   string                    `json:"proposal_origin"`
		DecisionOrigin   string                    `json:"decision_origin"`
		Automation       *knowledgeAutomationAudit `json:"automation"`
		UsageStatus      string                    `json:"usage_status"`
	}
	var proposedBy *string
	if v.ProposalOrigin == "human" {
		proposedBy = &v.Metadata.ProposedByUserID
	}
	audit := metadata{v.Metadata, proposedBy, v.ProposalOrigin, v.DecisionOrigin, v.Automation, v.UsageStatus}
	type decisionMetadata struct {
		ActorUserID *string   `json:"actor_user_id"`
		At          time.Time `json:"at"`
	}
	var decision *decisionMetadata
	if v.Decision != nil {
		var actor *string
		if v.DecisionOrigin == "human" {
			actor = &v.Decision.ActorUserID
		}
		decision = &decisionMetadata{actor, v.Decision.At}
	}
	if v.Availability == "source_unavailable" {
		return json.Marshal(struct {
			knowledgeCandidateControl
			metadata
			Decision     *decisionMetadata                   `json:"decision"`
			Capabilities collaboration.KnowledgeCapabilities `json:"capabilities"`
		}{v.knowledgeCandidateControl, audit, decision, v.Capabilities})
	}
	if v.Availability != "available" {
		return nil, knowledgeFailure(503, "knowledge_unavailable")
	}
	type decisionWithReason struct {
		decisionMetadata
		Reason *string `json:"reason"`
	}
	var detail *decisionWithReason
	if decision != nil {
		var reason *string
		if v.DecisionOrigin == "human" {
			reason = v.Decision.Reason
		}
		detail = &decisionWithReason{*decision, reason}
	}
	return json.Marshal(struct {
		knowledgeCandidateControl
		metadata
		knowledgeCandidateMaterial
		Decision     *decisionWithReason                 `json:"decision"`
		Capabilities collaboration.KnowledgeCapabilities `json:"capabilities"`
		Content      *string                             `json:"content,omitempty"`
	}{v.knowledgeCandidateControl, audit, v.Material, detail, v.Capabilities, v.Content})
}
