package service

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// CollectKnowledgeReadLockHints reads bounded routing/reference metadata only.
// nil manifest discovers the current collection; a supplied manifest discovers
// exactly its historical exposures. Neither path grants body-read authority.
func (s *CollaborationService) CollectKnowledgeReadLockHints(ctx context.Context, q *db.Queries, f KnowledgeConsumerFacts, manifest *KnowledgeManifest) (KnowledgeReadLockHints, error) {
	out := KnowledgeReadLockHints{Targets: []KnowledgeTargetRevision{}, Sources: []KnowledgeSourceLockHint{}, CandidateIDs: []pgtype.UUID{}, Validities: map[pgtype.UUID]knowledgeAutomaticProof{}}
	if q == nil {
		return out, knowledgeFailure(503, "knowledge_unavailable")
	}
	groups := map[pgtype.UUID]*KnowledgeSourceLockHint{}
	add := func(id, ws, issue pgtype.UUID, raw []byte) error {
		var refs []protocol.CollaborationKnowledgeSource
		if json.Unmarshal(raw, &refs) != nil || len(refs) == 0 || len(refs) > 32 {
			return knowledgeFailure(503, "knowledge_unavailable")
		}
		potential, err := knowledgeSourcePotential(ctx, q, issue, ws, refs)
		if err != nil {
			return err
		}
		if !potential {
			if manifest != nil {
				return knowledgeFailure(409, "knowledge_context_changed")
			}
			return nil
		}
		g := groups[issue]
		if g == nil {
			g = &KnowledgeSourceLockHint{IssueID: issue, WorkspaceID: ws, Sources: []protocol.CollaborationKnowledgeSource{}}
			groups[issue] = g
		}
		if g.WorkspaceID != ws {
			return knowledgeFailure(503, "knowledge_unavailable")
		}
		for _, ref := range refs {
			if ref.IssueID != util.UUIDToString(issue) {
				return knowledgeFailure(503, "knowledge_unavailable")
			}
			if !slices.Contains(g.Sources, ref) {
				g.Sources = append(g.Sources, ref)
			}
		}
		if !slices.Contains(out.CandidateIDs, id) {
			out.CandidateIDs = append(out.CandidateIDs, id)
		}
		return nil
	}
	addCandidate := func(id, ws, issue pgtype.UUID, raw []byte, approvalOrigin string) error {
		var refs []protocol.CollaborationKnowledgeSource
		if json.Unmarshal(raw, &refs) != nil {
			return knowledgeFailure(503, "knowledge_unavailable")
		}
		if knowledgeConsumerVersion(f) == 1 {
			if approvalOrigin == "automation" {
				return knowledgeFailure(409, "knowledge_protocol_unsupported")
			}
			for _, ref := range refs {
				if ref.Kind == "delivery" {
					return knowledgeFailure(409, "knowledge_protocol_unsupported")
				}
			}
		}
		if err := add(id, ws, issue, raw); err != nil {
			return err
		}
		if approvalOrigin != "automation" || !slices.Contains(out.CandidateIDs, id) {
			return nil
		}
		heads, err := q.ListKnowledgeValidityLockHints(ctx, []pgtype.UUID{id})
		if err != nil {
			return err
		}
		if len(heads) != 1 {
			if manifest != nil {
				return knowledgeFailure(409, "knowledge_context_changed")
			}
			return nil
		}
		head := heads[0]
		dependencies, valid := knowledgeSupportDependencies(head.SupportDependencies)
		if !valid || head.SourceIssueID != issue {
			if manifest != nil {
				return knowledgeFailure(409, "knowledge_context_changed")
			}
			return nil
		}
		proof := knowledgeAutomaticProof{Binding: protocol.TaskKnowledgeValidity{Revision: strconv.FormatInt(head.Revision, 10), AcceptedEventSeq: strconv.FormatInt(head.AcceptedEventSeq, 10), SubmissionID: util.UUIDToString(head.SubmissionID), ValidatedCloseEpoch: strconv.FormatInt(head.ValidatedCloseEpoch, 10)}, EvaluationID: head.EvaluationID, Dependencies: dependencies}
		for _, ref := range dependencies {
			if ref.IssueID != util.UUIDToString(issue) {
				if manifest != nil {
					return knowledgeFailure(409, "knowledge_context_changed")
				}
				return nil
			}
			encoded, err := json.Marshal([]protocol.CollaborationKnowledgeSource{ref})
			if err != nil {
				return err
			}
			if err = add(id, ws, issue, encoded); err != nil {
				return err
			}
		}
		out.Validities[id] = proof
		return nil
	}
	for _, target := range knowledgeConsumerTargets(f) {
		before, err := knowledgeReadTargetRevision(ctx, q, target)
		if err != nil {
			return out, err
		}
		out.Targets = append(out.Targets, KnowledgeTargetRevision{Target: target, Revision: before})
		if manifest == nil {
			ws, _ := util.ParseUUID(target.WorkspaceID)
			project, _ := util.ParseUUID(target.ProjectID)
			rows, err := q.ListKnowledgeTargetLockHints(ctx, db.ListKnowledgeTargetLockHintsParams{TargetKind: target.Kind, TargetWorkspaceID: ws, ProjectID: project})
			if err != nil {
				return out, err
			}
			if len(rows) > 16 {
				return out, knowledgeFailure(503, "knowledge_unavailable")
			}
			for _, row := range rows {
				if err = addCandidate(row.ID, row.SourceWorkspaceID, row.SourceIssueID, row.Sources, row.ApprovalOrigin); err != nil {
					return out, err
				}
			}
		}
		after, err := knowledgeReadTargetRevision(ctx, q, target)
		if err != nil {
			return out, err
		}
		if before != after {
			return out, knowledgeFailure(409, "knowledge_context_changed")
		}
	}
	if manifest != nil {
		if manifest.ProtocolVersion > knowledgeConsumerVersion(f) || protocol.ValidateTaskKnowledgeManifest(*manifest) != nil {
			return out, knowledgeFailure(409, "knowledge_context_changed")
		}
		for _, entry := range manifest.Entries {
			if !knowledgeTargetApplies(f, entry.Target) {
				return out, knowledgeFailure(409, "knowledge_context_changed")
			}
			id, err := util.ParseUUID(entry.CandidateID)
			if err != nil {
				return out, knowledgeFailure(409, "knowledge_context_changed")
			}
			row, err := q.GetKnowledgeCandidateLockHint(ctx, id)
			if errors.Is(err, pgx.ErrNoRows) {
				return out, knowledgeFailure(409, "knowledge_context_changed")
			}
			if err != nil {
				return out, err
			}
			if err = addCandidate(row.ID, row.SourceWorkspaceID, row.SourceIssueID, row.Sources, row.ApprovalOrigin); err != nil {
				return out, err
			}
			if entry.Validity != nil {
				proof, ok := out.Validities[id]
				if !ok || proof.Binding != *entry.Validity {
					return out, knowledgeFailure(409, "knowledge_context_changed")
				}
			}
		}
	}
	for _, source := range groups {
		out.Sources = append(out.Sources, *source)
	}
	slices.SortFunc(out.Sources, func(a, b KnowledgeSourceLockHint) int { return knowledgeCompareUUID(a.IssueID, b.IssueID) })
	slices.SortFunc(out.CandidateIDs, knowledgeCompareUUID)
	return out, nil
}

func knowledgeSourcePotential(ctx context.Context, q *db.Queries, issue, workspace pgtype.UUID, refs []protocol.CollaborationKnowledgeSource) (bool, error) {
	available, err := q.KnowledgeSourceIdentityAvailable(ctx, db.KnowledgeSourceIdentityAvailableParams{ID: issue, WorkspaceID: workspace})
	if err != nil || !available {
		return false, err
	}
	for _, ref := range refs {
		if ref.IssueID != util.UUIDToString(issue) {
			return false, knowledgeFailure(503, "knowledge_unavailable")
		}
		if ref.Kind == "delivery" {
			submission, err := util.ParseUUID(ref.SubmissionID)
			if err != nil {
				return false, nil
			}
			delivery, err := strconv.ParseInt(ref.DeliveryEventSeq, 10, 64)
			if err != nil {
				return false, nil
			}
			accepted, err := strconv.ParseInt(ref.AcceptedEventSeq, 10, 64)
			if err != nil {
				return false, nil
			}
			available, err = q.KnowledgeDeliverySourcePotentiallyAvailable(ctx, db.KnowledgeDeliverySourcePotentiallyAvailableParams{IssueID: issue, SubmissionID: submission, DeliveryEventSeq: delivery, AcceptedEventSeq: accepted})
			if err != nil || !available {
				return false, err
			}
			continue
		}
		version, _ := strconv.ParseInt(ref.Version, 10, 64)
		comment, _ := util.ParseUUID(ref.CommentID)
		run, _ := util.ParseUUID(ref.RunID)
		available, err = q.KnowledgeSourceReferencePotentiallyAvailable(ctx, db.KnowledgeSourceReferencePotentiallyAvailableParams{Kind: ref.Kind, IssueID: issue, WorkspaceID: workspace, Version: version, CommentID: comment, RunID: run})
		if err != nil || !available {
			return false, err
		}
	}
	return true, nil
}

// C calls this before disclosure. Fresh discovery may omit already unavailable
// sources, but a later restored source must not become a body without its locks.
// Target revision watermarks are not authority and may remain original on replay.
func ValidateKnowledgeResolutionLocks(resolution KnowledgeResolution, hints KnowledgeReadLockHints) error {
	if len(resolution.Contributions) != len(resolution.Manifest.Entries) {
		return knowledgeFailure(409, "knowledge_context_changed")
	}
	entries := map[string]KnowledgeManifestEntry{}
	for _, entry := range resolution.Manifest.Entries {
		id, err := util.ParseUUID(entry.CandidateID)
		if err != nil || !slices.Contains(hints.CandidateIDs, id) {
			return knowledgeFailure(409, "knowledge_context_changed")
		}
		if _, seen := entries[entry.CandidateID]; seen {
			return knowledgeFailure(409, "knowledge_context_changed")
		}
		entries[entry.CandidateID] = entry
	}
	for _, item := range resolution.Contributions {
		entry, ok := entries[item.CandidateID]
		if !ok || entry.Version != item.Version || entry.ContentSHA256 != item.ContentSHA256 || entry.SourceIssueID != item.SourceIssueID || entry.Target != item.Target {
			return knowledgeFailure(409, "knowledge_context_changed")
		}
		required := append([]protocol.CollaborationKnowledgeSource{}, item.Sources...)
		if entry.Validity != nil {
			id, _ := util.ParseUUID(entry.CandidateID)
			proof, ok := hints.Validities[id]
			if !ok || proof.Binding != *entry.Validity {
				return knowledgeFailure(409, "knowledge_context_changed")
			}
			required = append(required, proof.Dependencies...)
		}
		for _, ref := range required {
			found := false
			for _, source := range hints.Sources {
				if util.UUIDToString(source.IssueID) == ref.IssueID && slices.Contains(source.Sources, ref) {
					found = true
					break
				}
			}
			if !found {
				return knowledgeFailure(409, "knowledge_context_changed")
			}
		}
		delete(entries, item.CandidateID)
	}
	return nil
}

func knowledgeReadTargetRevision(ctx context.Context, q *db.Queries, target protocol.CollaborationKnowledgeTarget) (string, error) {
	row, err := q.GetKnowledgeTarget(ctx, db.GetKnowledgeTargetParams{TargetKind: target.Kind, TargetID: knowledgeTargetID(target)})
	if err == nil {
		return strconv.FormatInt(row.KnowledgeRevision, 10), nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	ws, _ := util.ParseUUID(target.WorkspaceID)
	project, _ := util.ParseUUID(target.ProjectID)
	_, err = q.GetKnowledgeTargetHead(ctx, db.GetKnowledgeTargetHeadParams{TargetKind: target.Kind, WorkspaceID: ws, ProjectID: project, TargetConfirmer: true})
	if !errors.Is(err, pgx.ErrNoRows) {
		if err != nil {
			return "", err
		}
		return "", knowledgeFailure(503, "knowledge_unavailable")
	}
	return "0", nil
}

// C calls this after its sorted user, workspace and membership fences and
// before Chat/source locks. Parent workspace locks must already be held.
func (s *CollaborationService) LockKnowledgeReadTargets(ctx context.Context, q *db.Queries, hints KnowledgeReadLockHints) error {
	if len(hints.Targets) > 2 {
		return knowledgeFailure(409, "knowledge_context_changed")
	}
	for _, hint := range hints.Targets {
		target := hint.Target
		if target.Kind == "project" {
			ws, _ := util.ParseUUID(target.WorkspaceID)
			if _, err := q.LockKnowledgeProject(ctx, db.LockKnowledgeProjectParams{ID: knowledgeTargetID(target), WorkspaceID: ws}); err != nil {
				return err
			}
		}
		row, err := lockKnowledgeCounter(ctx, q, target, true)
		if err != nil {
			return err
		}
		if strconv.FormatInt(row.KnowledgeRevision, 10) != hint.Revision {
			return knowledgeFailure(409, "knowledge_context_changed")
		}
	}
	return nil
}

// C calls this after Chat and before late runtime/task NOWAIT locks. It never
// locks back into parents/users or starts a nested transaction.
func (s *CollaborationService) LockKnowledgeReadSources(ctx context.Context, q *db.Queries, hints KnowledgeReadLockHints) error {
	if len(hints.Sources) > 32 || len(hints.CandidateIDs) > 32 {
		return knowledgeFailure(409, "knowledge_context_changed")
	}
	for _, source := range hints.Sources {
		issue, err := q.LockKnowledgeSourceIssue(ctx, source.IssueID)
		if errors.Is(err, pgx.ErrNoRows) {
			return knowledgeFailure(409, "knowledge_context_changed")
		}
		if err != nil {
			return err
		}
		if issue.WorkspaceID != source.WorkspaceID {
			return knowledgeFailure(409, "knowledge_context_changed")
		}
		if _, err = q.LockKnowledgeSourceCollaboration(ctx, source.IssueID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return knowledgeFailure(409, "knowledge_context_changed")
			}
			return err
		}
		if _, err = q.LockKnowledgeSourceSession(ctx, source.IssueID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return knowledgeFailure(409, "knowledge_context_changed")
			}
			return err
		}
	}
	rows, err := q.LockKnowledgeReadCandidates(ctx, hints.CandidateIDs)
	if err != nil {
		return err
	}
	if len(rows) != len(hints.CandidateIDs) {
		return knowledgeFailure(409, "knowledge_context_changed")
	}
	validityIDs := make([]pgtype.UUID, 0, len(hints.Validities))
	for id := range hints.Validities {
		validityIDs = append(validityIDs, id)
	}
	slices.SortFunc(validityIDs, knowledgeCompareUUID)
	if len(validityIDs) > 0 {
		heads, err := q.LockKnowledgeValiditiesForRead(ctx, validityIDs)
		if err != nil {
			return err
		}
		if len(heads) != len(validityIDs) {
			return knowledgeFailure(409, "knowledge_context_changed")
		}
		for _, head := range heads {
			proof := hints.Validities[head.CandidateID]
			if head.EvaluationID != proof.EvaluationID || knowledgeValidityBinding(head) != proof.Binding {
				return knowledgeFailure(409, "knowledge_context_changed")
			}
		}
	}
	for _, source := range hints.Sources {
		if len(source.Sources) > 1024 {
			return knowledgeFailure(409, "knowledge_context_changed")
		}
		var versions, deliveryEvents []int64
		var comments, runs, deliveries []pgtype.UUID
		for _, ref := range source.Sources {
			switch ref.Kind {
			case "memory":
				v, err := strconv.ParseInt(ref.Version, 10, 64)
				if err != nil {
					return knowledgeFailure(409, "knowledge_context_changed")
				}
				if !slices.Contains(versions, v) {
					versions = append(versions, v)
				}
			case "comment":
				id, err := util.ParseUUID(ref.CommentID)
				if err != nil {
					return knowledgeFailure(409, "knowledge_context_changed")
				}
				if !slices.Contains(comments, id) {
					comments = append(comments, id)
				}
			case "run_record":
				id, err := util.ParseUUID(ref.RunID)
				if err != nil {
					return knowledgeFailure(409, "knowledge_context_changed")
				}
				if !slices.Contains(runs, id) {
					runs = append(runs, id)
				}
			case "delivery":
				id, err := util.ParseUUID(ref.SubmissionID)
				if err != nil {
					return knowledgeFailure(409, "knowledge_context_changed")
				}
				if !slices.Contains(deliveries, id) {
					deliveries = append(deliveries, id)
				}
				for _, text := range []string{ref.DeliveryEventSeq, ref.AcceptedEventSeq} {
					seq, err := strconv.ParseInt(text, 10, 64)
					if err != nil {
						return knowledgeFailure(409, "knowledge_context_changed")
					}
					if !slices.Contains(deliveryEvents, seq) {
						deliveryEvents = append(deliveryEvents, seq)
					}
				}
			default:
				return knowledgeFailure(409, "knowledge_context_changed")
			}
		}
		if len(versions) > 0 {
			rows, err := q.LockKnowledgeMemorySources(ctx, db.LockKnowledgeMemorySourcesParams{IssueID: source.IssueID, Versions: versions})
			if err != nil {
				return err
			}
			if len(rows) != len(versions) {
				return knowledgeFailure(409, "knowledge_context_changed")
			}
		}
		if len(comments) > 0 {
			rows, err := q.LockKnowledgeCommentSources(ctx, db.LockKnowledgeCommentSourcesParams{IssueID: source.IssueID, Ids: comments})
			if err != nil {
				return err
			}
			if len(rows) != len(comments) {
				return knowledgeFailure(409, "knowledge_context_changed")
			}
		}
		if len(runs) > 0 {
			rows, err := q.LockKnowledgeRunSources(ctx, db.LockKnowledgeRunSourcesParams{IssueID: source.IssueID, Ids: runs})
			if err != nil {
				return err
			}
			if len(rows) != len(runs) {
				return knowledgeFailure(409, "knowledge_context_changed")
			}
		}
		if len(deliveries) > 0 {
			rows, err := q.LockKnowledgeDeliverySources(ctx, db.LockKnowledgeDeliverySourcesParams{IssueID: source.IssueID, Ids: deliveries})
			if err != nil {
				return err
			}
			if len(rows) != len(deliveries) {
				return knowledgeFailure(409, "knowledge_context_changed")
			}
			seqs, err := q.LockKnowledgeDeliveryEvents(ctx, db.LockKnowledgeDeliveryEventsParams{IssueID: source.IssueID, EventSeqs: deliveryEvents})
			if err != nil {
				return err
			}
			if len(seqs) != len(deliveryEvents) {
				return knowledgeFailure(409, "knowledge_context_changed")
			}
		}
	}
	return nil
}
