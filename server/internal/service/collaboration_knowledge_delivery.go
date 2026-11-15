package service

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type knowledgeDeliverySnapshot struct {
	Source           protocol.CollaborationKnowledgeSource
	Result           string
	Basis            string
	Canonical        string
	DeliveryEpoch    int64
	AcceptedByUserID pgtype.UUID
}

// Field order and JSON escaping define the complete delivery hash. Quotes are
// checked against Result or Basis separately, never against this serialization.
func knowledgeDeliveryCanonical(result, basis string) (string, error) {
	if !collaborationDeliveryTextValid(result) || !collaborationDeliveryTextValid(basis) || strings.ContainsRune(result, 0) || strings.ContainsRune(basis, 0) {
		return "", errCollaborationDeliveryEvidence
	}
	raw, err := json.Marshal(struct {
		Result string `json:"result"`
		Basis  string `json:"basis"`
	}{result, basis})
	return string(raw), err
}

// knowledgeAcceptedDelivery borrows a source-authorized read. It grants no
// authority and resolves one exact immutable delivery through the domain reader.
func knowledgeAcceptedDelivery(ctx context.Context, q *db.Queries, issue db.Issue, submissionID pgtype.UUID, deliverySeq, acceptedSeq int64) (knowledgeDeliverySnapshot, bool, error) {
	var out knowledgeDeliverySnapshot
	if q == nil {
		return out, false, knowledgeFailure(503, "knowledge_unavailable")
	}
	if !issue.ID.Valid || !submissionID.Valid || deliverySeq < 1 || acceptedSeq <= deliverySeq {
		return out, false, nil
	}
	state, err := q.GetKnowledgeSourceState(ctx, issue.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, false, nil
	}
	if err != nil {
		return out, false, err
	}
	events, err := q.ListCollaborationDeliveryEvents(ctx, db.ListCollaborationDeliveryEventsParams{IssueID: issue.ID, ThroughSeq: deliverySeq, RowLimit: 1})
	if err != nil || len(events) != 1 || events[0].EventSeq != deliverySeq {
		return out, false, err
	}
	items, err := projectDeliverySubmissions(ctx, q, CollaborationIssueAccess{Issue: issue, LifecycleEpoch: state.LifecycleEpoch}, events)
	if errors.Is(err, errCollaborationDeliveryEvidence) {
		return out, false, nil
	}
	if err != nil || len(items) != 1 {
		return out, false, err
	}
	item := items[0]
	if item.ID != submissionID || item.EvidenceState != "available" || item.Result == nil || item.Basis == nil || item.LifecycleEpoch == nil || item.Decision == nil || item.Decision.Kind != "accepted" || item.Decision.EventSeq != acceptedSeq || item.Decision.SubmissionID != submissionID {
		return out, false, nil
	}
	canonical, err := knowledgeDeliveryCanonical(*item.Result, *item.Basis)
	if err != nil {
		return out, false, nil
	}
	epoch, err := strconv.ParseInt(*item.LifecycleEpoch, 10, 64)
	if err != nil || epoch < 1 {
		return out, false, nil
	}
	out = knowledgeDeliverySnapshot{Result: *item.Result, Basis: *item.Basis, Canonical: canonical, DeliveryEpoch: epoch, AcceptedByUserID: item.Decision.ActorUserID,
		Source: protocol.CollaborationKnowledgeSource{Kind: "delivery", IssueID: util.UUIDToString(issue.ID), SubmissionID: util.UUIDToString(submissionID), DeliveryEventSeq: strconv.FormatInt(deliverySeq, 10), AcceptedEventSeq: strconv.FormatInt(acceptedSeq, 10), SHA256: protocol.CollaborationKnowledgeHash(canonical)}}
	return out, true, nil
}
