package service

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/material"
	"github.com/multica-ai/multica/server/internal/materialgit"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func materialRuntimeDigest(r db.AgentRuntime) string {
	return material.JSONDigest(struct {
		ID, Owner, Profile                 pgtype.UUID
		Daemon, Mode, Provider, Visibility string
		Epoch                              int64
	}{r.ID, r.OwnerID, r.ProfileID, r.DaemonID.String, r.RuntimeMode, r.Provider, r.Visibility, r.CollaborationConfigEpoch})
}
func materialRuntimeConfigurationDigest(ctx context.Context, q *db.Queries, r db.AgentRuntime) (string, error) {
	base := materialRuntimeDigest(r)
	if !r.ProfileID.Valid {
		return base, nil
	}
	p, e := q.LockMaterialRuntimeProfile(ctx, db.LockMaterialRuntimeProfileParams{ID: r.ProfileID, WorkspaceID: r.WorkspaceID})
	if e != nil {
		return "", e
	}
	if !p.Enabled {
		return "", materialError(409, "material_runtime_unavailable")
	}
	return material.JSONDigest(struct {
		Runtime string
		Profile db.RuntimeProfile
	}{base, p}), nil
}
func materialSignerCurrent(ctx context.Context, q *db.Queries, signer stageSignerCredential, user pgtype.UUID) error {
	if signer.Version != 1 {
		return materialError(403, "material_approval_expired")
	}
	now, e := q.ArtifactExecutionDatabaseTime(ctx)
	if e != nil {
		return e
	}
	switch signer.Kind {
	case auth.CredentialPAT:
		if signer.PATHash == "" {
			return materialError(403, "material_approval_expired")
		}
		if _, e = q.LockArtifactPAT(ctx, db.LockArtifactPATParams{TokenHash: signer.PATHash, UserID: user}); e != nil {
			if errors.Is(e, pgx.ErrNoRows) {
				return materialError(403, "material_approval_expired")
			}
			return e
		}
	case auth.CredentialJWT:
		if signer.ExpiresAt == nil || !now.Time.Before(*signer.ExpiresAt) {
			return materialError(403, "material_approval_expired")
		}
	default:
		return materialError(403, "material_approval_expired")
	}
	if signer.PasswordSession != nil {
		if signer.PasswordSession.UserID != util.UUIDToString(user) {
			return materialError(403, "material_approval_expired")
		}
		if _, e = q.LockArtifactPasswordCredential(ctx, user); e != nil {
			return e
		}
		if _, e = auth.CheckPasswordSessionVersion(ctx, q, *signer.PasswordSession); e != nil {
			if errors.Is(e, auth.ErrPasswordSession) {
				return materialError(403, "material_approval_expired")
			}
			return e
		}
		if e = auth.CheckManagedBinding(ctx, q, *signer.PasswordSession); e != nil {
			if errors.Is(e, auth.ErrPasswordSession) {
				return materialError(403, "material_approval_expired")
			}
			return e
		}
	} else if auth.PasswordMode() {
		return materialError(403, "material_approval_expired")
	}
	return nil
}
func materialRuntimeForUser(ctx context.Context, q *db.Queries, ws pgtype.UUID, id *string, user pgtype.UUID) (db.AgentRuntime, error) {
	if id == nil {
		return db.AgentRuntime{}, materialError(409, "material_runtime_required")
	}
	uid, e := materialUUID(*id)
	if e != nil {
		return db.AgentRuntime{}, e
	}
	r, e := q.LockRuntimeForAutopilotDispatch(ctx, db.LockRuntimeForAutopilotDispatchParams{ID: uid, WorkspaceID: ws})
	if e != nil {
		if errors.Is(e, pgx.ErrNoRows) {
			return r, materialError(409, "material_runtime_unavailable")
		}
		return r, e
	}
	if !r.OwnerID.Valid || r.Visibility != "public" && r.OwnerID != user {
		return r, materialError(403, "material_runtime_forbidden")
	}
	if _, e = LockArtifactUsers(ctx, q, ws, util.UUIDToString(user), util.UUIDToString(r.OwnerID)); e != nil {
		var denied ArtifactError
		if errors.As(e, &denied) {
			return r, materialError(403, "material_runtime_forbidden")
		}
		return r, e
	}
	return r, nil
}
func materialOperationDescriptor(job db.GitMaterialJob, row db.GitMaterial, revision db.GitMaterialRevision, b materialBinding, r db.AgentRuntime, taskID pgtype.UUID, action string) (protocol.MaterialExecution, error) {
	rev, e := materialParse[material.Revision](revision.Payload)
	if e != nil {
		return protocol.MaterialExecution{}, e
	}
	d := protocol.MaterialExecution{ProtocolVersion: 1, OperationID: util.UUIDToString(dbid.NewV7()), JobID: util.UUIDToString(job.ID), MaterialID: util.UUIDToString(row.ID), RevisionID: rev.ID, ProjectID: util.UUIDToString(row.ProjectID), WorkspaceID: util.UUIDToString(row.WorkspaceID), TaskID: util.UUIDToString(taskID), Action: action, OperationKey: rev.ID, ConfigVersion: b.Version, ResourceID: b.ResourceID, Provider: b.Provider, InstanceURL: b.InstanceURL, Repository: b.Repository, BaseBranch: b.BaseBranch, SourceBranch: "multica/material-" + rev.ID, RuntimeID: util.UUIDToString(r.ID), DaemonID: r.DaemonID.String, ProfileID: util.UUIDToString(r.ProfileID), ConfigDigest: b.RuntimeDigest, Title: "Multica engineering material " + rev.ID, Body: "Versioned engineering material. Review and execution are managed in Multica."}
	for _, f := range materialFiles(row, rev) {
		d.Files = append(d.Files, protocol.MaterialFile{Path: f.Path, Content: f.Content, Mode: "100644", SHA256: material.Digest(f.Content)})
	}
	if len(revision.Review) > 0 {
		v, e := materialParse[materialgit.Review](revision.Review)
		if e != nil {
			return d, e
		}
		d.ReviewNumber = v.Number
		d.ReviewURL = v.URL
		d.ApprovedHead = v.HeadSHA
	}
	return d, nil
}
func createMaterialOperation(ctx context.Context, q *db.Queries, job db.GitMaterialJob, d protocol.MaterialExecution, agent, user pgtype.UUID, attempt int32) error {
	d.ExpectedDigest = d.Digest()
	id, e := materialUUID(d.OperationID)
	if e != nil {
		return e
	}
	tid, e := materialUUID(d.TaskID)
	if e != nil {
		return e
	}
	rid, e := materialUUID(d.RuntimeID)
	if e != nil {
		return e
	}
	_, e = q.CreateMaterialOperation(ctx, db.CreateMaterialOperationParams{ID: id, JobID: job.ID, TaskID: tid, WorkspaceID: job.WorkspaceID, ProjectID: job.ProjectID, MaterialID: job.MaterialID, RevisionID: job.RevisionID, RuntimeID: rid, AgentID: agent, AuthorizerID: user, AttemptNo: attempt, Action: d.Action, Descriptor: materialJSON(d), ExpectedDigest: d.ExpectedDigest})
	return e
}
func materialDecision(ctx context.Context, t *materialTx, revision db.GitMaterialRevision, b materialBinding) (db.GitMaterialDecision, error) {
	d, e := t.q.LatestMaterialDecision(ctx, revision.ID)
	if e != nil {
		return d, e
	}
	members, e := LockArtifactUsers(ctx, t.q, revision.WorkspaceID, util.UUIDToString(d.ActorID))
	if e != nil {
		return d, e
	}
	if members[util.UUIDToString(d.ActorID)].ID != d.MemberID || !slices.Contains(b.ReviewerIDs, util.UUIDToString(d.ActorID)) {
		return d, materialError(403, "material_reviewer_revoked")
	}
	signer, e := materialParse[stageSignerCredential](d.Credential)
	if e != nil {
		return d, e
	}
	if e = CheckArtifactCredentialInTx(ctx, t.tx, signer.credential(util.UUIDToString(d.ActorID))); e != nil {
		return d, materialError(403, "material_approval_expired")
	}
	return d, nil
}
func (s *MaterialService) processMaterialGit(ctx context.Context, job db.GitMaterialJob) error {
	t, row, rev, b, e := s.workerTx(ctx, job)
	if e != nil {
		return e
	}
	defer t.tx.Rollback(ctx)
	if !b.Enabled || row.CurrentRevisionID != rev.ID || b.Version != rev.ConfigVersion {
		return materialError(409, "material_configuration_changed")
	}
	if _, e = checkMaterialBinding(ctx, t.q, row.WorkspaceID, row.ProjectID, b, false); e != nil {
		return e
	}
	r, e := materialRuntimeForUser(ctx, t.q, row.WorkspaceID, b.ExecutionRuntimeID, t.actor)
	if e != nil {
		return e
	}
	if r.Status != "online" || r.DaemonID.String == "" {
		return materialError(409, "material_runtime_offline")
	}
	prior, e := t.q.LatestMaterialOperation(ctx, job.ID)
	attempt := int32(1)
	if e == nil { // A scheduler lease is never permission to repeat an uncertain remote mutation.
		if prior.State != "succeeded" && prior.State != "recovery_requested" && prior.State != "no_effect" {
			return t.tx.Commit(ctx)
		}
		attempt = prior.AttemptNo + 1
	} else if !errors.Is(e, pgx.ErrNoRows) {
		return e
	}
	action := job.Kind
	if action == "review" {
		action = "review_sync"
	}
	if action == "merge" {
		action = "review_sync"
	}
	if e == nil && prior.State == "succeeded" {
		old, parseErr := materialParse[protocol.MaterialExecution](prior.Descriptor)
		if parseErr != nil {
			return parseErr
		}
		completedAction := old.Action
		if completedAction == "observe" {
			completedAction = old.ReconcileAction
		}
		if action == "review_sync" {
			decision, err := materialDecision(ctx, t, rev, b)
			if err != nil {
				return err
			}
			if old.DecisionID == util.UUIDToString(decision.ID) {
				if completedAction == "review_sync" && decision.Decision == "approve" {
					action = "merge"
				} else {
					if err = t.q.CompleteMaterialOperationJob(ctx, db.CompleteMaterialOperationJobParams{ID: job.ID, State: "done"}); err != nil {
						return err
					}
					return t.tx.Commit(ctx)
				}
			}
		} else {
			if err := t.q.CompleteMaterialOperationJob(ctx, db.CompleteMaterialOperationJobParams{ID: job.ID, State: "done"}); err != nil {
				return err
			}
			return t.tx.Commit(ctx)
		}
	}
	taskID := dbid.NewV7()
	d, e := materialOperationDescriptor(job, row, rev, b, r, taskID, action)
	if e != nil {
		return e
	}
	if prior.State == "recovery_requested" || prior.State == "no_effect" {
		original, err := materialParse[protocol.MaterialExecution](prior.Descriptor)
		if err != nil {
			return err
		}
		original.TaskID = d.TaskID
		original.OperationID = d.OperationID
		if original.Action == "observe" {
			original.Action = original.ReconcileAction
			original.ReconcileAction = ""
		}
		if prior.State == "recovery_requested" {
			original.ReconcileAction = original.Action
			original.Action = "observe"
		}
		d = original
		action = d.Action
	}
	if action != "publish" && action != "observe" {
		decision, e := materialDecision(ctx, t, rev, b)
		if e != nil {
			return e
		}
		d.DecisionID = util.UUIDToString(decision.ID)
		d.ApprovedHead = decision.HeadSha
		d.OperationKey = d.DecisionID
		d.AuditNote = &protocol.MaterialAuditNote{Marker: "multica-decision:" + d.DecisionID, Body: "<!-- multica-decision:" + d.DecisionID + " -->\n" + fmt.Sprintf("Multica human decision %s by %s for revision %s at %s. Reason: %s", decision.Decision, util.UUIDToString(decision.ActorID), d.RevisionID, decision.HeadSha, decision.Reason)}
		if action == "merge" && decision.Decision != "approve" {
			return materialError(409, "material_merge_not_authorized")
		}
	}
	agent, e := materialUUID(b.CarrierID)
	if e != nil {
		return e
	}
	operationID, _ := materialUUID(d.OperationID)
	task, e := t.q.CreateMaterialOperationTask(ctx, db.CreateMaterialOperationTaskParams{ID: taskID, AgentID: agent, RuntimeID: r.ID, AuthorizerID: t.actor, OperationID: operationID, Context: materialJSON(map[string]any{"type": "material_operation", "operation_id": d.OperationID})})
	if e != nil {
		return e
	}
	if e = createMaterialOperation(ctx, t.q, job, d, agent, t.actor, attempt); e != nil {
		return e
	}
	if e = t.q.FinishMaterialJob(ctx, db.FinishMaterialJobParams{ID: job.ID, LeaseToken: job.LeaseToken, State: "dispatched"}); e != nil {
		return e
	}
	if e = t.tx.Commit(ctx); e != nil {
		return e
	}
	if s.Tasks != nil {
		s.Tasks.FinalizeIssueTaskEnqueue(ctx, task)
	}
	return nil
}

// AuthorizeMaterialOperation checks the frozen execution identity at each boundary.
func AuthorizeMaterialOperation(ctx context.Context, q *db.Queries, task db.AgentTaskQueue) (db.GitMaterialOperation, protocol.MaterialExecution, error) {
	op, e := q.GetMaterialOperationForTask(ctx, task.ID)
	if e != nil {
		return op, protocol.MaterialExecution{}, e
	}
	d, e := materialParse[protocol.MaterialExecution](op.Descriptor)
	if e != nil {
		return op, d, e
	}
	if d.Digest() != op.ExpectedDigest || task.ID != op.TaskID || task.AgentID != op.AgentID || task.RuntimeID != op.RuntimeID || task.OriginatorUserID != op.AuthorizerID {
		return op, d, materialError(403, "material_task_changed")
	}
	if _, e = q.LockMaterialProjectForClaim(ctx, db.LockMaterialProjectForClaimParams{ID: op.ProjectID, WorkspaceID: op.WorkspaceID}); e != nil {
		return op, d, e
	}
	setting, e := q.GetMaterialSettings(ctx, db.GetMaterialSettingsParams{ProjectID: op.ProjectID, WorkspaceID: op.WorkspaceID})
	if e != nil {
		return op, d, e
	}
	b, e := materialParse[materialBinding](setting.Settings)
	if e != nil {
		return op, d, e
	}
	if !b.Enabled || b.Version != d.ConfigVersion || b.ExecutionRuntimeID == nil || *b.ExecutionRuntimeID != d.RuntimeID || b.CarrierID != util.UUIDToString(op.AgentID) {
		return op, d, materialError(409, "material_configuration_changed")
	}
	if _, e = currentMaterialManager(ctx, q, op.WorkspaceID, setting.AuthorizedBy, setting.MemberID); e != nil {
		return op, d, e
	}
	if e = materialSignerCurrent(ctx, q, b.Credential, setting.AuthorizedBy); e != nil {
		return op, d, e
	}
	r, e := materialRuntimeForUser(ctx, q, op.WorkspaceID, b.ExecutionRuntimeID, op.AuthorizerID)
	if e != nil {
		return op, d, e
	}
	digest, e := materialRuntimeConfigurationDigest(ctx, q, r)
	if e != nil {
		return op, d, e
	}
	if digest != d.ConfigDigest || r.DaemonID.String != d.DaemonID {
		return op, d, materialError(409, "material_runtime_changed")
	}
	if _, e = checkMaterialBinding(ctx, q, op.WorkspaceID, op.ProjectID, b, false); e != nil {
		return op, d, e
	}
	row, e := q.GetGitMaterial(ctx, db.GetGitMaterialParams{WorkspaceID: op.WorkspaceID, ProjectID: op.ProjectID, ID: op.MaterialID})
	if e != nil {
		return op, d, e
	}
	if row.CurrentRevisionID != op.RevisionID {
		return op, d, materialError(409, "material_revision_changed")
	}
	if d.DecisionID != "" {
		latest, e := q.LatestMaterialDecision(ctx, op.RevisionID)
		if e != nil {
			return op, d, e
		}
		if util.UUIDToString(latest.ID) != d.DecisionID || latest.HeadSha != d.ApprovedHead || d.Action == "merge" && latest.Decision != "approve" {
			return op, d, materialError(409, "material_approval_changed")
		}
		if !slices.Contains(b.ReviewerIDs, util.UUIDToString(latest.ActorID)) {
			return op, d, materialError(403, "material_reviewer_revoked")
		}
		if e = materialCurrentMember(ctx, q, op.WorkspaceID, latest.ActorID, latest.MemberID); e != nil {
			return op, d, e
		}
		signer, e := materialParse[stageSignerCredential](latest.Credential)
		if e != nil {
			return op, d, e
		}
		if e = materialSignerCurrent(ctx, q, signer, latest.ActorID); e != nil {
			return op, d, e
		}
	}
	return op, d, nil
}
func bindMaterialClaim(ctx context.Context, q *db.Queries, task db.AgentTaskQueue) (*protocol.MaterialClaimBinding, error) {
	op, _, e := AuthorizeMaterialOperation(ctx, q, task)
	if errors.Is(e, pgx.ErrNoRows) && task.TriggerEvidenceKind.String != "material_operation" {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	if op.ClaimGeneration >= task.ClaimGeneration {
		return nil, materialError(409, "material_claim_already_finalized")
	}
	secret := make([]byte, 32)
	if _, e = rand.Read(secret); e != nil {
		return nil, e
	}
	challenge := hex.EncodeToString(secret)
	e = q.BindMaterialOperationClaim(ctx, db.BindMaterialOperationClaimParams{TaskID: task.ID, ClaimGeneration: task.ClaimGeneration, ChallengeHash: material.Digest(challenge)})
	return &protocol.MaterialClaimBinding{ClaimGeneration: task.ClaimGeneration, HostChallenge: challenge}, e
}
func MaterialOperationForTask(ctx context.Context, q *db.Queries, task db.AgentTaskQueue) (protocol.MaterialOperationResponse, error) {
	_, d, e := AuthorizeMaterialOperation(ctx, q, task)
	return protocol.MaterialOperationResponse{Execution: d, ClaimGeneration: task.ClaimGeneration}, e
}
func materialReviewFromObservation(r *protocol.MaterialReview) materialgit.Review {
	if r == nil {
		return materialgit.Review{}
	}
	v := materialgit.Review{Number: r.Number, URL: r.URL, HeadSHA: r.HeadSHA, BaseBranch: r.BaseBranch, Branch: r.SourceBranch, State: r.State, MergeSHA: r.MergeSHA, MergeActor: r.MergeActor, MergedAt: r.MergedAt, GatesPassed: r.GatesPassed, BlockReason: r.BlockReason}
	for _, c := range r.Changes {
		v.Changes = append(v.Changes, materialgit.Change{Path: c.Path, PreviousPath: c.PreviousPath, Status: c.Status, Mode: c.Mode})
	}
	return v
}
func validateMaterialObservation(d protocol.MaterialExecution, o protocol.MaterialObservation) error {
	absentPublication := d.Action == "observe" && d.ReconcileAction == "publish" && o.Outcome == "not_found" && o.Review == nil && len(o.Files) == 0 && o.AuditNote == nil
	absentNote := d.Action == "observe" && d.ReconcileAction == "review_sync" && o.Outcome == "not_found" && d.AuditNote != nil && o.AuditNote == nil
	if d.Action == "observe" {
		d.Action = d.ReconcileAction
	}
	if o.ProtocolVersion != 1 || !o.SourceComplete || (o.Outcome != "observed" && !absentPublication && !absentNote) || o.RepositoryIdentity != d.Provider+":"+d.InstanceURL+":"+d.Repository {
		return materialError(409, "material_observation_incomplete")
	}
	if o.ObservedAt.IsZero() || o.ObservedAt.After(time.Now().Add(time.Minute)) || time.Since(o.ObservedAt) > 5*time.Minute {
		return materialError(409, "material_observation_stale")
	}
	// Only the host's complete read-only reconciliation can prove both the
	// deterministic source branch and its review absent. A mutation's own
	// completion or an incomplete lookup never releases a new write attempt.
	if absentPublication {
		return nil
	}
	commit := d.BaselineCommit
	if d.Action != "verify_baseline" {
		r := o.Review
		if r == nil || r.Number <= 0 || r.SourceBranch != d.SourceBranch || r.BaseBranch != d.BaseBranch || !material.IsSHA(r.HeadSHA) || d.ReviewNumber != 0 && d.ReviewNumber != r.Number || d.ApprovedHead != "" && d.ApprovedHead != r.HeadSHA {
			return materialError(409, "material_review_changed")
		}
		if !strings.HasPrefix(r.URL, d.InstanceURL+"/"+d.Repository+"/") {
			return materialError(409, "material_review_changed")
		}
		expected := map[string]bool{}
		for _, f := range d.Files {
			expected[f.Path] = true
		}
		if len(r.Changes) == 0 || len(r.Changes) > len(expected) {
			return materialError(409, "material_unapproved_files")
		}
		for _, c := range r.Changes {
			if !expected[c.Path] || c.Mode != "100644" || c.PreviousPath != "" && c.PreviousPath != c.Path || c.Status != "added" && c.Status != "modified" {
				return materialError(409, "material_unapproved_files")
			}
			delete(expected, c.Path)
		}
		commit = r.HeadSHA
		if r.State == "merged" {
			commit = r.MergeSHA
			if d.Action != "merge" || r.MergeActor == "" || !material.IsSHA(commit) {
				return materialError(409, "material_merge_not_authorized")
			}
		} else if r.State != "open" {
			return materialError(409, "material_review_not_open")
		}
	}
	if !material.IsSHA(commit) || len(o.Files) != len(d.Files) {
		return materialError(409, "material_content_changed")
	}
	seen := map[string]bool{}
	for _, f := range o.Files {
		if seen[f.Path] || f.Commit != commit || f.Mode != "100644" || material.Digest(f.Content) != f.SHA256 {
			return materialError(409, "material_content_changed")
		}
		seen[f.Path] = true
		found := false
		for _, want := range d.Files {
			if want.Path == f.Path && want.Content == f.Content && want.SHA256 == f.SHA256 {
				found = true
			}
		}
		if !found {
			return materialError(409, "material_content_changed")
		}
	}
	if d.Action == "review_sync" && !absentNote && (d.AuditNote == nil || o.AuditNote == nil || o.AuditNote.Marker != d.AuditNote.Marker || o.AuditNote.Body != d.AuditNote.Body || o.AuditNote.Actor == "") {
		return materialError(409, "material_review_sync_unconfirmed")
	}
	return nil
}
func (s *MaterialService) ObserveOperation(ctx context.Context, taskID pgtype.UUID, o protocol.MaterialObservation) error {
	tx, e := s.TxStarter.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	q := s.Queries.WithTx(tx)
	task, e := q.LockAgentTaskForStart(ctx, taskID)
	if e != nil {
		return e
	}
	op, e := q.GetMaterialOperationForTask(ctx, taskID)
	if e != nil {
		return e
	}
	d, e := materialParse[protocol.MaterialExecution](op.Descriptor)
	if e != nil {
		return e
	}

	digest := material.JSONDigest(o)
	history, e := materialParse[map[string]string](op.ReceiptHistory)
	if e != nil {
		return e
	}
	if previous, ok := history[o.ObservationID]; ok {
		if previous != digest {
			return materialError(409, "material_observation_conflict")
		}
		if _, e = LockArtifactUsers(ctx, q, op.WorkspaceID, util.UUIDToString(op.AuthorizerID)); e != nil {
			return e
		}
		return tx.Commit(ctx)
	}
	if o.TaskID != util.UUIDToString(taskID) || o.OperationID != d.OperationID || o.RuntimeID != d.RuntimeID || o.ClaimGeneration != task.ClaimGeneration || op.ClaimGeneration != task.ClaimGeneration || o.ExpectedDigest != op.ExpectedDigest || len(o.HostChallenge) < 32 || subtle.ConstantTimeCompare([]byte(material.Digest(o.HostChallenge)), []byte(op.ChallengeHash)) != 1 {
		return materialError(403, "material_observation_denied")
	}
	op, d, e = AuthorizeMaterialOperation(ctx, q, task)
	if e != nil {
		return e
	}
	if op.State == "succeeded" || op.State == "no_effect" {
		return materialError(409, "material_observation_state")
	}
	if len(o.ObservationID) < 8 || len(o.ObservationID) > 128 || task.Status != "running" && !(d.Action == "verify_baseline" && task.Status == "dispatched") {
		return materialError(409, "material_observation_state")
	}
	if e = validateMaterialObservation(d, o); e != nil {
		return e
	}
	row, e := q.GetGitMaterial(ctx, db.GetGitMaterialParams{WorkspaceID: op.WorkspaceID, ProjectID: op.ProjectID, ID: op.MaterialID})
	if e != nil {
		return e
	}
	revision, e := q.GetMaterialRevision(ctx, op.RevisionID)
	if e != nil {
		return e
	}
	rev, e := materialParse[material.Revision](revision.Payload)
	if e != nil {
		return e
	}
	state := "succeeded"
	blocker := ""
	jobState := "done"
	review := materialReviewFromObservation(o.Review)
	effectiveAction := d.Action
	if effectiveAction == "observe" {
		effectiveAction = d.ReconcileAction
	}
	switch effectiveAction {
	case "publish":
		if o.Outcome == "not_found" {
			state = "no_effect"
			break
		}
		if e = q.ObserveMaterialRevision(ctx, db.ObserveMaterialRevisionParams{ID: revision.ID, PublicationStatus: "in_review", Review: materialJSON(review)}); e != nil {
			return e
		}
		status := "in_review"
		if rev.Kind == "plan" {
			status = "plan_review"
		}
		_, e = updateMaterial(ctx, q, row, status, "", revision.ID, row.Baseline)
	case "review_sync":
		if o.Outcome == "not_found" {
			state = "no_effect"
			break
		}
		decision, e2 := q.LatestMaterialDecision(ctx, revision.ID)
		if e2 != nil {
			return e2
		}
		if decision.Decision == "approve" {
			jobState = "pending"
		}
	case "merge":
		if review.State != "merged" {
			state = "preflight"
			jobState = "dispatched"
			if !review.GatesPassed {
				blocker = "material_native_checks_pending"
			}
			if d.Action == "observe" {
				state = "no_effect"
				jobState = "done"
			}
		} else {
			permitOperation := op
			if d.Action == "observe" {
				var err error
				permitOperation, err = q.GetMaterialPermittedMerge(ctx, op.JobID)
				if err != nil {
					return err
				}
				original, err := materialParse[protocol.MaterialExecution](permitOperation.Descriptor)
				if err != nil || original.DecisionID != d.DecisionID || original.ApprovedHead != d.ApprovedHead {
					return materialError(409, "material_merge_not_authorized")
				}
			}
			preflight, pe := materialParse[protocol.MaterialObservation](permitOperation.Receipt)
			if permitOperation.PermitRequestID == "" || !permitOperation.PermitExpiresAt.Valid || !materialMergeWithinPermit(permitOperation.PermitExpiresAt.Time, o.ObservedAt, o.Review) || pe != nil || preflight.Review == nil || !preflight.Review.GatesPassed || preflight.Review.HeadSHA != review.HeadSHA || preflight.Review.State != "open" || material.JSONDigest(preflight.Review.Changes) != material.JSONDigest(o.Review.Changes) {
				return materialError(409, "material_merge_not_authorized")
			}
			if e = q.ObserveMaterialRevision(ctx, db.ObserveMaterialRevisionParams{ID: revision.ID, PublicationStatus: "merged", Review: materialJSON(review)}); e != nil {
				return e
			}
			if rev.Kind == "plan" {
				p, e2 := q.GetMaterialPlan(ctx, row.ID)
				if e2 != nil {
					return e2
				}
				p.Status = "approved"
				p.RevisionID = revision.ID
				p.Blocker = pgtype.Text{}
				if e = saveMaterialPlan(ctx, q, p); e != nil {
					return e
				}
				_, e = updateMaterial(ctx, q, row, "plan_approved", "", revision.ID, row.Baseline)
			} else {
				base := material.Baseline{ID: rev.ID, RevisionID: rev.ID, Commit: review.MergeSHA, Path: row.Path, ContentSHA256: rev.ContentSHA256, ReviewURL: review.URL, VerifiedAt: o.ObservedAt}
				row, e = updateMaterial(ctx, q, row, "baseline_ready", "", revision.ID, materialJSON(base))
				if e == nil {
					e = ensureMaterialJob(ctx, q, row, revision.ID, "plan")
				}
			}
		}
	case "verify_baseline":
	default:
		return materialError(400, "material_action_invalid")
	}
	if e != nil {
		return e
	}
	o.HostChallenge = "" // Store no bearer secret; digest binds the original exact transport.
	if e = q.SaveMaterialObservation(ctx, db.SaveMaterialObservationParams{ID: op.ID, Receipt: materialJSON(o), ReceiptDigest: digest, ObservationID: o.ObservationID, State: state, Blocker: materialText(blocker)}); e != nil {
		return e
	}
	if d.Action != "verify_baseline" {
		if e = q.CompleteMaterialOperationJob(ctx, db.CompleteMaterialOperationJobParams{ID: op.JobID, State: jobState}); e != nil {
			return e
		}
	}
	return tx.Commit(ctx)
}

// Hosting APIs report merge instants at second precision. Treat that second
// as the represented instant; a delayed readback is allowed, but neither a
// missing time nor an external merge outside the issued permit is accepted.
func materialMergeWithinPermit(expires, observed time.Time, review *protocol.MaterialReview) bool {
	if review == nil || review.MergedAt == nil {
		return false
	}
	merged := *review.MergedAt
	return !merged.IsZero() && !merged.Before(expires.Add(-time.Minute).Truncate(time.Second)) && !merged.After(expires) && !merged.After(observed)
}

func (s *MaterialService) OperationPermit(ctx context.Context, task db.AgentTaskQueue, id string, in protocol.MaterialPermitInput) (protocol.MaterialPermit, error) {
	var out protocol.MaterialPermit
	tx, e := s.TxStarter.Begin(ctx)
	if e != nil {
		return out, e
	}
	defer tx.Rollback(ctx)
	q := s.Queries.WithTx(tx)
	task, e = q.LockAgentTaskForStart(ctx, task.ID)
	if e != nil {
		return out, e
	}
	op, d, e := AuthorizeMaterialOperation(ctx, q, task)
	if e != nil {
		return out, e
	}
	if task.Status != "running" || d.OperationID != id || d.ExpectedDigest != in.ExpectedDigest || len(in.RequestID) < 8 || (d.Action == "verify_baseline" || d.Action == "observe") {
		return out, materialError(403, "material_permit_denied")
	}
	if d.Action == "merge" {
		o, e := materialParse[protocol.MaterialObservation](op.Receipt)
		if e != nil || op.State != "preflight" || o.Review == nil || !o.Review.GatesPassed || time.Since(o.ObservedAt) > time.Minute {
			return out, materialError(409, "material_merge_preflight_required")
		}
	}
	out = protocol.MaterialPermit{OperationID: id, Action: d.Action, ExpectedDigest: d.ExpectedDigest, HeadSHA: d.ApprovedHead, ExpiresAt: time.Now().Add(time.Minute)}
	for _, f := range d.Files {
		out.Paths = append(out.Paths, f.Path)
	}
	if op.PermitRequestID != "" {
		if op.PermitRequestID != in.RequestID {
			return out, materialError(409, "material_permit_conflict")
		}
		out.ExpiresAt = op.PermitExpiresAt.Time
	} else if e = q.SaveMaterialPermit(ctx, db.SaveMaterialPermitParams{ID: op.ID, PermitRequestID: in.RequestID, PermitExpiresAt: pgtype.Timestamptz{Time: out.ExpiresAt, Valid: true}}); e != nil {
		return out, e
	}
	return out, tx.Commit(ctx)
}
