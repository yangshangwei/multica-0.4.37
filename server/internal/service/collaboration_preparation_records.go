package service

import (
	"context"
	"errors"
	"sort"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// Human records deliberately do not share the runtime manifest type. Its
// digest includes private invocation settings, and neither belongs in this DTO.
type CollaborationRunPreparation struct {
	Version     int                                `json:"version"`
	Attachments []CollaborationRunPreparationFile  `json:"attachments"`
	Git         *CollaborationRunPreparationGit    `json:"git"`
	Stages      []CollaborationRunPreparationStage `json:"stages"`
}

type CollaborationRunPreparationFile struct {
	ID        string `json:"id"`
	Version   string `json:"version"`
	Name      string `json:"name"`
	SizeBytes int64  `json:"size_bytes"`
}

type CollaborationRunPreparationGit struct {
	ObjectFormat string `json:"object_format"`
	CommitOID    string `json:"commit_oid"`
}

type CollaborationRunPreparationStage struct {
	Seq   string `json:"seq"`
	Stage string `json:"stage"`
	State string `json:"state"`
	Code  string `json:"code,omitempty"`
}

func recordedPreparationManifest(issueID pgtype.UUID, run db.CollaborationRun, inputs []protocol.CollaborationRunInput) (protocol.CollaborationPreparation, error) {
	var saved protocol.CollaborationPreparation
	if run.PromptSnapshot == nil || len(run.PreparationSnapshot) > 2<<20 || decodeCollaborationPreparationJSON(run.PreparationSnapshot, &saved) != nil ||
		protocol.ValidateCollaborationPreparation(&saved, collaborationRunIdentity(run, issueID), inputs) != nil ||
		saved.Resource.AgentID != util.UUIDToString(run.AgentID) || saved.Resource.RuntimeID != util.UUIDToString(run.RuntimeID) ||
		saved.Resource.OwnerUserID != util.UUIDToString(run.ResourceOwnerUserID) || saved.Resource.ExecutionWorkspaceID != util.UUIDToString(run.ExecutionWorkspaceID) ||
		saved.Resource.TargetEpoch != strconv.FormatInt(run.TargetEpoch.Int64, 10) || saved.Resource.ConfigVersion != strconv.FormatInt(run.ConfigVersion.Int64, 10) ||
		saved.Resource.RuntimeConfigEpoch != strconv.FormatInt(run.ResourceEpoch, 10) {
		return saved, invalidCollaborationLedger("record preparation invalid")
	}
	return saved, nil
}

// This borrows the same current-human issue transaction as the other records.
// Retained facts do not consult the mutable or possibly deleted live resource.
func readCollaborationPreparationRecords(ctx context.Context, q *db.Queries, issueID pgtype.UUID, run db.CollaborationRun, inputs []protocol.CollaborationRunInput, through int64) (*CollaborationRunPreparation, error) {
	if run.PreparationSnapshot == nil {
		return nil, nil
	}
	saved, err := recordedPreparationManifest(issueID, run, inputs)
	if err != nil {
		return nil, err
	}
	out := &CollaborationRunPreparation{Version: 1, Attachments: make([]CollaborationRunPreparationFile, 0, len(saved.Attachments)), Stages: []CollaborationRunPreparationStage{}}
	for _, file := range saved.Attachments {
		out.Attachments = append(out.Attachments, CollaborationRunPreparationFile{ID: file.ID, Version: file.Version, Name: file.Name, SizeBytes: file.SizeBytes})
	}
	if saved.Git != nil {
		out.Git = &CollaborationRunPreparationGit{ObjectFormat: saved.Git.ObjectFormat, CommitOID: saved.Git.CommitOID}
	}
	rows, err := q.ListCollaborationRunPreparationRecords(ctx, db.ListCollaborationRunPreparationRecordsParams{IssueID: issueID, RunID: run.ID, ThroughSeq: through})
	if err != nil {
		return nil, err
	}
	if len(rows) > 4 {
		return nil, invalidCollaborationLedger("record preparation stages invalid")
	}
	if len(rows) == 0 {
		return out, nil
	}
	attempt, err := q.GetCollaborationAttempt(ctx, db.GetCollaborationAttemptParams{RunID: run.ID, AttemptSeq: 1})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, invalidCollaborationLedger("record preparation attempt missing")
	}
	if err != nil {
		return nil, err
	}
	if run.AuthorizedAttemptSeq != 1 || attempt.RunID != run.ID || attempt.AttemptSeq != 1 || attempt.ClaimGeneration != run.ClaimGeneration {
		return nil, invalidCollaborationLedger("record preparation attempt invalid")
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Seq < rows[j].Seq })
	for _, event := range rows {
		var observation protocol.CollaborationPreparationObservation
		if event.Kind != "preparation" || event.IssueID != issueID || event.RunID != run.ID || event.Seq <= 0 || event.Seq > through ||
			len(event.Payload) > 2<<20 || decodeCollaborationPreparationJSON(event.Payload, &observation) != nil || protocol.ValidateCollaborationPreparationObservation(observation) != nil ||
			observation.Identity != saved.Identity || observation.ManifestDigest != saved.ManifestDigest || observation.EventID != event.ProducerEventID ||
			observation.AttemptNonce != attempt.AttemptNonce || observation.Stage == "git" && saved.Git == nil {
			return nil, invalidCollaborationLedger("record preparation observation invalid")
		}
		out.Stages = append(out.Stages, CollaborationRunPreparationStage{Seq: strconv.FormatInt(event.Seq, 10), Stage: observation.Stage, State: observation.State, Code: observation.Code})
	}
	return out, nil
}
