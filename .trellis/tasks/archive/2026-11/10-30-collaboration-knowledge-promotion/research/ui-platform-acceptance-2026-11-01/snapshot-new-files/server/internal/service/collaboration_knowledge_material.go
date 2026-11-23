package service

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func knowledgeCandidateSources(row db.CollaborationKnowledgeCandidate) ([]protocol.CollaborationKnowledgeSource, error) {
	var refs []protocol.CollaborationKnowledgeSource
	if json.Unmarshal(row.Sources, &refs) != nil || len(refs) == 0 || len(refs) > protocol.CollaborationKnowledgeMaxReferences {
		return nil, knowledgeFailure(503, "knowledge_unavailable")
	}
	for _, ref := range refs {
		if ref.IssueID != util.UUIDToString(row.SourceIssueID) {
			return nil, knowledgeFailure(503, "knowledge_unavailable")
		}
	}
	return refs, nil
}

func knowledgeContribution(row db.CollaborationKnowledgeCandidate) (protocol.CollaborationKnowledgeContribution, error) {
	item := protocol.CollaborationKnowledgeContribution{CandidateID: util.UUIDToString(row.ID), Version: strconv.FormatInt(row.Version, 10), ContentSHA256: row.ContentSha256, Target: knowledgeTarget(row), SourceIssueID: util.UUIDToString(row.SourceIssueID), Summary: row.Summary, Content: row.Content, ExpiresAt: knowledgeTime(row.ExpiresAt)}
	refs, err := knowledgeCandidateSources(row)
	if err != nil {
		return item, err
	}
	item.Sources = refs
	if json.Unmarshal(row.Evidence, &item.Evidence) != nil || json.Unmarshal(row.Applicability, &item.Applicability) != nil {
		return item, knowledgeFailure(503, "knowledge_unavailable")
	}
	raw, err := json.Marshal(item)
	if err != nil {
		return item, err
	}
	var checked protocol.CollaborationKnowledgeContribution
	if json.Unmarshal(raw, &checked) != nil {
		return item, knowledgeFailure(503, "knowledge_unavailable")
	}
	return checked, nil
}
func knowledgeTime(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	v := value.Time.UTC()
	return &v
}
func knowledgeIDPointer(value pgtype.UUID) *string {
	if !value.Valid {
		return nil
	}
	v := util.UUIDToString(value)
	return &v
}
func knowledgeVersionPointer(value pgtype.Int8) *string {
	if !value.Valid {
		return nil
	}
	v := strconv.FormatInt(value.Int64, 10)
	return &v
}
