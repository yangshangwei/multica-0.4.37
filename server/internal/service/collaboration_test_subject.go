package service

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"slices"
	"strconv"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// This private snapshot retains the original approval and immutable code facts.
// It is never projected as an approval or taken from the report producer.
type collaborationBoundTestSubject struct {
	Version          int                                  `json:"version"`
	Reference        protocol.CollaborationTestSubjectRef `json:"reference"`
	Code             stageCodeContent                     `json:"code"`
	ReviewDecisionID string                               `json:"review_decision_id"`
	LifecycleEpoch   int64                                `json:"lifecycle_epoch"`
}

type collaborationFrozenTestSubject struct {
	Version        int                                  `json:"version"`
	Identity       protocol.CollaborationRunIdentity    `json:"identity"`
	Subject        protocol.CollaborationTestSubject    `json:"subject"`
	Binding        collaborationBoundTestSubject        `json:"binding"`
	CommandIDs     []string                             `json:"command_ids"`
	ManifestDigest string                               `json:"manifest_digest"`
	Git            protocol.CollaborationPreparationGit `json:"git"`
	// A random private salt makes the public binding digest opaque even when
	// private manifest instructions have a small, guessable value space.
	Salt string `json:"salt"`
}

func (s *CollaborationService) validateCommentTestSubject(ctx context.Context, t *collaborationContentTx, p CollaborationCreateCommentParams) (*collaborationBoundTestSubject, error) {
	if p.TestSubject == nil {
		return nil, nil
	}
	if !t.access.Capabilities.SubmitCommand || t.participant.RevokedAt.Valid {
		return nil, collaborationError(403, "forbidden")
	}
	if t.access.LifecycleEpoch != p.ExpectedLifecycleEpoch || t.participant.Epoch != p.ExpectedParticipantEpoch || t.account.Epoch != p.ExpectedAccountEpoch {
		return nil, collaborationError(409, "stale_authority")
	}
	if !t.access.SourceMember {
		return nil, collaborationError(409, "stage_evidence_unavailable")
	}
	if len(parseCollaborationMentions(p.Content).Targets) == 0 || len(p.TargetVersions) == 0 {
		return nil, collaborationError(409, "test_subject_target_required")
	}
	selected := false
	for _, mention := range parseCollaborationMentions(p.Content).Targets {
		for _, target := range t.targets {
			if target.ResourceID != mention.ResourceID || target.Kind != string(mention.Kind) || slices.Contains(p.SuppressedTargetIDs, target.ID) {
				continue
			}
			for _, version := range p.TargetVersions {
				if version.TargetID == target.ID {
					selected = true
				}
			}
		}
	}
	if !selected {
		return nil, collaborationError(409, "test_subject_target_required")
	}
	w := &collaborationWorkflowTx{collaborationQueueTx: t.collaborationQueueTx, tx: t.tx}
	if w.stageClosed() {
		return nil, collaborationError(409, "issue_closed")
	}
	policy, err := t.q.GetProjectStagePolicy(ctx, db.GetProjectStagePolicyParams{WorkspaceID: t.access.Issue.WorkspaceID, ProjectID: t.access.Issue.ProjectID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, collaborationError(409, "stage_policy_disabled")
	}
	if err != nil {
		return nil, err
	}
	state, refusal, err := w.loadStage(ctx, policy.Revision)
	if err != nil {
		return nil, err
	}
	if refusal != nil {
		return nil, collaborationError(refusal.Status, refusal.Code)
	}
	if len(collaboration.StageMissing(state.facts)) != 0 {
		return nil, collaborationError(409, "stage_needs_confirmation")
	}
	if state.task.Stage != "qa" {
		return nil, collaborationError(409, "stage_stage_changed")
	}
	version, _ := strconv.ParseInt(p.TestSubject.CandidateVersion, 10, 64)
	c, content, refusal, err := w.codeCandidate(ctx, CollaborationStageCandidateRef{CandidateID: util.MustParseUUID(p.TestSubject.CandidateID), Version: version, ContentDigest: p.TestSubject.CandidateDigest})
	if err != nil {
		return nil, err
	}
	if refusal != nil {
		return nil, collaborationError(refusal.Status, refusal.Code)
	}
	if err = w.lockCodeChild(ctx, content); err != nil {
		return nil, err
	}
	if err = lockStageCodeEvidence(ctx, t.tx); err != nil {
		return nil, err
	}
	now, err := t.q.ArtifactExecutionDatabaseTime(ctx)
	if err != nil {
		return nil, err
	}
	observed, code, err := stageCodeCurrent(ctx, t.q, t.access, state, c, content, now.Time)
	if err != nil {
		return nil, err
	}
	if code != "" {
		return nil, collaborationError(409, code)
	}
	d, err := t.q.GetCollaborationStageReviewHead(ctx, db.GetCollaborationStageReviewHeadParams{IssueID: p.IssueID, CandidateID: c.CandidateID, Step: "code_review"})
	if errors.Is(err, pgx.ErrNoRows) || err == nil && d.Decision != "approved" {
		return nil, collaborationError(409, "stage_dependency_invalid")
	}
	if err != nil {
		return nil, err
	}
	valid, _, err := stageReviewState(ctx, t.tx, t.access, state, c, content, d, observed, code, now.Time)
	if err != nil {
		return nil, err
	}
	if valid != "valid" {
		return nil, collaborationError(409, stageDecisionRefusal(valid))
	}
	code, err = checkStageArtifactCurrent(ctx, t.tx, content)
	if err != nil {
		return nil, err
	}
	if code != "" {
		return nil, collaborationError(409, code)
	}
	final, code, err := checkStageFinalAuthority(ctx, t.tx, t.access, state, []db.CollaborationStageDecision{d})
	if err != nil {
		return nil, err
	}
	if code == "forbidden" {
		return nil, collaborationError(403, code)
	}
	if code != "" {
		return nil, collaborationError(409, code)
	}
	if observed.CI.ObservedAt == nil || !final.Before(observed.CI.ObservedAt.Add(stageCIHorizon)) {
		return nil, collaborationError(409, "stage_ci_stale")
	}
	return &collaborationBoundTestSubject{Version: 1, Reference: *p.TestSubject, Code: content, ReviewDecisionID: util.UUIDToString(d.DecisionID), LifecycleEpoch: t.access.LifecycleEpoch}, nil
}

func bindCommentTestSubject(ctx context.Context, t *collaborationContentTx, command pgtype.UUID, b *collaborationBoundTestSubject) error {
	if b == nil {
		return nil
	}
	raw, err := json.Marshal(b)
	if err != nil {
		return err
	}
	version, _ := strconv.ParseInt(b.Reference.CandidateVersion, 10, 64)
	return t.q.CreateCollaborationTestSubject(ctx, db.CreateCollaborationTestSubjectParams{IssueID: t.access.Issue.ID, WorkspaceID: t.access.Issue.WorkspaceID, CommandID: command,
		CandidateID: util.MustParseUUID(b.Reference.CandidateID), CandidateVersion: version, CandidateDigest: b.Reference.CandidateDigest, Subject: raw, BoundByUserID: t.actor,
		ParticipantEpoch: t.participant.Epoch, AccountEpoch: t.account.Epoch, LifecycleEpoch: t.access.LifecycleEpoch})
}

func decodeBoundTestSubject(row db.CollaborationTestSubject) (collaborationBoundTestSubject, error) {
	var b collaborationBoundTestSubject
	if decodeCollaborationPreparationJSON(row.Subject, &b) != nil || b.Version != 1 || protocol.ValidateCollaborationTestSubjectRef(b.Reference) != nil ||
		b.Reference.CandidateID != util.UUIDToString(row.CandidateID) || b.Reference.CandidateVersion != stageVersion(row.CandidateVersion) || b.Reference.CandidateDigest != row.CandidateDigest ||
		b.LifecycleEpoch != row.LifecycleEpoch || !stageUUID(b.ReviewDecisionID) {
		return b, collaborationError(503, "test_report_unavailable")
	}
	_, digest, err := stageJSONDigest(b.Code)
	if err != nil || digest != row.CandidateDigest {
		return b, collaborationError(503, "test_report_unavailable")
	}
	return b, nil
}

func (s *CollaborationService) freezeRuntimeTestSubject(ctx context.Context, r *collaborationRuntimeState, inputs []protocol.CollaborationRunInput) error {
	ids := make([]pgtype.UUID, 0, len(inputs))
	commands := make(map[string]protocol.CollaborationRunInput, len(inputs))
	for _, input := range inputs {
		ids = append(ids, util.MustParseUUID(input.CommandID))
		commands[input.CommandID] = input
	}
	rows, err := r.q.ListCollaborationCommandTestSubjects(ctx, db.ListCollaborationCommandTestSubjectsParams{IssueID: r.access.Issue.ID, Column2: ids})
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}
	var binding collaborationBoundTestSubject
	for i, row := range rows {
		b, err := decodeBoundTestSubject(row)
		if err != nil {
			return err
		}
		input := commands[util.UUIDToString(row.CommandID)]
		if row.WorkspaceID != r.access.Issue.WorkspaceID || input.AuthorUserID != util.UUIDToString(row.BoundByUserID) || input.ParticipantEpoch != stageVersion(row.ParticipantEpoch) || input.AccountEpoch != stageVersion(row.AccountEpoch) || input.LifecycleEpoch != stageVersion(row.LifecycleEpoch) {
			return collaborationError(409, "test_subject_mismatch")
		}
		if i > 0 && !reflect.DeepEqual(binding, b) {
			return collaborationError(409, "test_subject_ambiguous")
		}
		binding = b
	}
	if r.run.PreparationSnapshot == nil {
		return collaborationError(409, "test_source_unbound")
	}
	preparation, err := s.readRuntimePreparation(ctx, r)
	if err != nil {
		return err
	}
	if preparation.Git == nil {
		return collaborationError(409, "test_source_unbound")
	}
	if preparation.Git.CommitOID != binding.Code.Baseline.PullRequest.HeadSHA {
		return collaborationError(409, "test_subject_mismatch")
	}
	subject := protocol.CollaborationTestSubject{CandidateID: binding.Reference.CandidateID, CandidateVersion: binding.Reference.CandidateVersion, CandidateDigest: binding.Reference.CandidateDigest, SourceRevision: preparation.Git.CommitOID}
	if a := binding.Code.Baseline.Artifact; a != nil {
		subject.ArtifactDigest = &a.ArtifactDigest
	}
	frozen := collaborationFrozenTestSubject{Version: 1, Identity: preparation.Identity, Subject: subject, Binding: binding, CommandIDs: preparation.CommandIDs, ManifestDigest: preparation.ManifestDigest, Git: *preparation.Git, Salt: uuid.NewString()}
	_, digest, err := stageJSONDigest(frozen)
	if err != nil {
		return err
	}
	frozen.Subject.BindingDigest = digest
	raw, err := json.Marshal(frozen)
	if err != nil {
		return err
	}
	_, err = r.q.CreateCollaborationBusinessEvent(ctx, db.CreateCollaborationBusinessEventParams{IssueID: r.access.Issue.ID, RunID: r.run.ID, EventID: "test-subject", Kind: "test_subject", Payload: raw})
	return err
}

func loadFrozenTestSubject(ctx context.Context, q *db.Queries, run, issue pgtype.UUID, identity protocol.CollaborationRunIdentity) (collaborationFrozenTestSubject, error) {
	var f collaborationFrozenTestSubject
	e, err := q.GetCollaborationRuntimeEvent(ctx, db.GetCollaborationRuntimeEventParams{RunID: run, ProducerEventID: "test-subject"})
	if errors.Is(err, pgx.ErrNoRows) {
		return f, collaborationError(409, "test_subject_unavailable")
	}
	if err != nil {
		return f, err
	}
	if e.Kind != "test_subject" || e.IssueID != issue || decodeCollaborationPreparationJSON(e.Payload, &f) != nil || f.Version != 1 || f.Identity != identity || protocol.ValidateCollaborationTestSubject(f.Subject) != nil || !stageUUID(f.Salt) {
		return f, collaborationError(503, "test_report_unavailable")
	}
	digest := f.Subject.BindingDigest
	f.Subject.BindingDigest = ""
	_, expected, err := stageJSONDigest(f)
	f.Subject.BindingDigest = digest
	if err != nil || expected != digest {
		return f, collaborationError(503, "test_report_unavailable")
	}
	return f, nil
}

// Runtime content authority includes the humans who selected the private code.
// Current participant/command authority is already checked by lockRuntimeBusiness.
func checkRuntimeTestSource(ctx context.Context, r *collaborationRuntimeState, f collaborationFrozenTestSubject) error {
	ids := make([]pgtype.UUID, 0, len(f.CommandIDs))
	for _, id := range f.CommandIDs {
		ids = append(ids, util.MustParseUUID(id))
	}
	rows, err := r.q.ListCollaborationCommandTestSubjects(ctx, db.ListCollaborationCommandTestSubjectsParams{IssueID: r.access.Issue.ID, Column2: ids})
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return collaborationError(409, "test_subject_unavailable")
	}
	for _, row := range rows {
		members, err := r.q.LockArtifactMembers(ctx, db.LockArtifactMembersParams{WorkspaceID: r.access.Issue.WorkspaceID, UserIds: []pgtype.UUID{row.BoundByUserID}})
		if err != nil {
			return err
		}
		if len(members) != 1 {
			return collaborationError(403, "forbidden")
		}
	}
	return nil
}
