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

// Stop advances the current delivery generation without changing the identity
// which claimed the manifest or produced the retained receipt. Historical reads
// must bind that original identity, never reinterpret it as the current claim.
func collaborationRecordedRunIdentity(issueID pgtype.UUID, run db.CollaborationRun, termination db.CollaborationTermination) (protocol.CollaborationRunIdentity, error) {
	identity, err := collaborationFrozenIdentity(termination)
	if err != nil {
		return identity, err
	}
	generation, valid := collaborationRuntimeVersion(identity.ClaimGeneration, false)
	expected := collaborationRunIdentity(run, issueID)
	expected.ClaimGeneration = identity.ClaimGeneration
	if !valid || generation > run.ClaimGeneration || expected != identity || termination.RunID != run.ID || termination.IssueID != issueID {
		return identity, invalidCollaborationLedger("recorded run identity mismatch")
	}
	return identity, nil
}

func readRecordedPreparationManifest(ctx context.Context, q *db.Queries, issueID pgtype.UUID, run db.CollaborationRun, inputs []protocol.CollaborationRunInput) (protocol.CollaborationPreparation, error) {
	termination, err := q.GetCollaborationTermination(ctx, run.ID)
	if err != nil {
		return protocol.CollaborationPreparation{}, err
	}
	identity, err := collaborationRecordedRunIdentity(issueID, run, termination)
	if err != nil {
		return protocol.CollaborationPreparation{}, err
	}
	claimed := run
	claimed.ClaimGeneration, _ = collaborationRuntimeVersion(identity.ClaimGeneration, false)
	return recordedPreparationManifest(issueID, claimed, inputs)
}

// This borrows the same current-human issue transaction as the other records.
// Retained facts do not consult the mutable or possibly deleted live resource.
func readCollaborationPreparationRecords(ctx context.Context, q *db.Queries, issueID pgtype.UUID, run db.CollaborationRun, inputs []protocol.CollaborationRunInput, through int64) (*CollaborationRunPreparation, error) {
	if run.PreparationSnapshot == nil {
		return nil, nil
	}
	saved, err := readRecordedPreparationManifest(ctx, q, issueID, run, inputs)
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
	if saved.Version == protocol.CollaborationPreparationVersion && run.AuthorizedAttemptSeq != 1 {
		return nil, invalidCollaborationLedger("record preparation attempt invalid")
	}
	attempts := make(map[int64]db.CollaborationAttempt)
	sort.Slice(rows, func(i, j int) bool { return rows[i].Seq < rows[j].Seq })
	for _, event := range rows {
		var observation protocol.CollaborationPreparationObservation
		if event.Kind != "preparation" || event.IssueID != issueID || event.RunID != run.ID || event.Seq <= 0 || event.Seq > through ||
			len(event.Payload) > 2<<20 || decodeCollaborationPreparationJSON(event.Payload, &observation) != nil || protocol.ValidateCollaborationPreparationObservation(observation) != nil ||
			observation.Identity != saved.Identity || observation.ManifestDigest != saved.ManifestDigest || observation.EventID != event.ProducerEventID ||
			observation.Stage == "git" && saved.Git == nil {
			return nil, invalidCollaborationLedger("record preparation observation invalid")
		}
		seq, _ := collaborationRuntimeVersion(observation.AttemptSeq, false)
		attempt, found := attempts[seq]
		if !found {
			attempt, err = q.GetCollaborationAttempt(ctx, db.GetCollaborationAttemptParams{RunID: run.ID, AttemptSeq: seq})
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, invalidCollaborationLedger("record preparation attempt missing")
			}
			if err != nil {
				return nil, err
			}
			attempts[seq] = attempt
		}
		if seq > run.AuthorizedAttemptSeq || !attempt.AuthorizedAt.Valid || attempt.RunID != run.ID || attempt.AttemptSeq != seq ||
			strconv.FormatInt(attempt.ClaimGeneration, 10) != saved.Identity.ClaimGeneration || observation.AttemptNonce != attempt.AttemptNonce ||
			!collaborationRuntimeMessageMode(observation.ProtocolVersion, observation.Containment, attempt.Containment) {
			return nil, invalidCollaborationLedger("record preparation attempt invalid")
		}
		out.Stages = append(out.Stages, CollaborationRunPreparationStage{Seq: strconv.FormatInt(event.Seq, 10), Stage: observation.Stage, State: observation.State, Code: observation.Code})
	}
	return out, nil
}
