package service

import (
	"context"
	"errors"
	"fmt"
	"path"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/material"
	"github.com/multica-ai/multica/server/internal/materialgit"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
)

var errMaterialMergePrepared = errors.New("material merge prepared")

type materialMergeReceipt struct {
	materialgit.Review
	DecisionID string `json:"merge_decision_id,omitempty"`
}

// Run processes already committed intents. A failed or interrupted external
// operation is recovered through the same provider identity, never a new branch.
func (s *MaterialService) Run(ctx context.Context) {
	timer := time.NewTicker(3 * time.Second)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			for i := 0; i < 10; i++ {
				worked, err := s.ProcessNext(ctx)
				if err != nil || !worked {
					break
				}
			}
		}
	}
}
func (s *MaterialService) ProcessNext(ctx context.Context) (bool, error) {
	job, err := s.Queries.ClaimMaterialJob(ctx, dbid.NewV7())
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	stepCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	if job.Kind == "plan" {
		err = s.processMaterialPlan(stepCtx, job)
	} else {
		err = s.processMaterialGit(stepCtx, job)
		if errors.Is(err, errMaterialMergePrepared) {
			err = s.processMaterialGit(stepCtx, job)
		}
	}
	if err != nil {
		// The durable intent survives a transaction/connection failure. Only the
		// lease owner may record this attempt's failure; an expired owner cannot win.
		var domain MaterialError
		state := "pending"
		if errors.As(err, &domain) {
			state = "blocked"
		}
		_ = s.Queries.FinishMaterialJob(ctx, db.FinishMaterialJobParams{ID: job.ID, LeaseToken: job.LeaseToken, State: state, Blocker: materialText(materialCode(err))})
	}
	return true, err
}
func (s *MaterialService) workerTx(ctx context.Context, job db.GitMaterialJob) (*materialTx, db.GitMaterial, db.GitMaterialRevision, materialBinding, error) {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return nil, db.GitMaterial{}, db.GitMaterialRevision{}, materialBinding{}, err
	}
	t := &materialTx{tx: tx, q: s.Queries.WithTx(tx), access: MaterialAccess{WorkspaceID: job.WorkspaceID, ProjectID: job.ProjectID}, kind: "human"}
	fail := func(e error) (*materialTx, db.GitMaterial, db.GitMaterialRevision, materialBinding, error) {
		_ = tx.Rollback(ctx)
		return nil, db.GitMaterial{}, db.GitMaterialRevision{}, materialBinding{}, e
	}
	if _, err = tx.Exec(ctx, "SET TRANSACTION ISOLATION LEVEL READ COMMITTED"); err != nil {
		return fail(err)
	}
	if _, err = t.q.LockWorkspaceForChatSessionCreate(ctx, job.WorkspaceID); err != nil {
		return fail(err)
	}
	if _, err = t.q.LockProjectForExecutionSquad(ctx, db.LockProjectForExecutionSquadParams{ID: job.ProjectID, WorkspaceID: job.WorkspaceID}); err != nil {
		return fail(err)
	}
	if _, err = t.q.LockMaterialJob(ctx, db.LockMaterialJobParams{ID: job.ID, LeaseToken: job.LeaseToken}); err != nil {
		return fail(err)
	}
	config, binding, err := t.binding(ctx)
	if err != nil {
		return fail(err)
	}
	t.actor = config.AuthorizedBy
	t.member, err = currentMaterialManager(ctx, t.q, job.WorkspaceID, t.actor, config.MemberID)
	if err != nil {
		return fail(err)
	}
	row, err := t.get(ctx, util.UUIDToString(job.MaterialID))
	if err != nil {
		return fail(err)
	}
	rev, err := t.q.GetMaterialRevision(ctx, job.RevisionID)
	if err != nil {
		return fail(err)
	}
	return t, row, rev, binding, nil
}
func materialFiles(row db.GitMaterial, rev material.Revision) []materialgit.File {
	if rev.Kind == "requirement" {
		return []materialgit.File{{Path: row.Path, Content: rev.Content}}
	}
	plan := ""
	if rev.PlanContent != nil {
		plan = *rev.PlanContent
	}
	return []materialgit.File{{Path: path.Join(path.Dir(row.Path), "design.md"), Content: rev.Content}, {Path: path.Join(path.Dir(row.Path), "plan.md"), Content: plan}}
}
func materialReviewMatches(row db.GitMaterial, rev material.Revision, b materialBinding, review materialgit.Review) error {
	if review.Branch != "multica/material-"+rev.ID || review.BaseBranch != b.BaseBranch || !material.IsSHA(review.HeadSHA) {
		return materialError(409, "material_review_changed")
	}
	expected := map[string]bool{}
	for _, f := range materialFiles(row, rev) {
		expected[f.Path] = true
	}
	if len(review.Changes) == 0 || len(review.Changes) > len(expected) {
		return materialError(409, "material_unapproved_files")
	}
	for _, c := range review.Changes {
		if !expected[c.Path] || c.PreviousPath != "" && c.PreviousPath != c.Path || c.Mode != "100644" || c.Status == "removed" || c.Status == "deleted" {
			return materialError(409, "material_unapproved_files")
		}
		delete(expected, c.Path)
	}

	return nil
}
func materialVerifyFiles(ctx context.Context, p materialgit.Provider, row db.GitMaterial, rev material.Revision, commit string) error {
	if !material.IsSHA(commit) {
		return materialError(409, "material_commit_unverified")
	}
	for _, f := range materialFiles(row, rev) {
		body, err := p.ReadFile(ctx, commit, f.Path)
		if err != nil {
			return err
		}
		if body != f.Content {
			return materialError(409, "material_content_changed")
		}
	}
	return nil
}
func (s *MaterialService) processMaterialGit(ctx context.Context, job db.GitMaterialJob) error {
	t, row, revision, binding, err := s.workerTx(ctx, job)
	if err != nil {
		return err
	}
	defer t.tx.Rollback(ctx)
	finish := func(state, code string) error {
		if code != "" {
			if _, e := updateMaterial(ctx, t.q, row, "blocked", code, row.CurrentRevisionID, row.Baseline); e != nil {
				return e
			}
		}
		if e := t.q.FinishMaterialJob(ctx, db.FinishMaterialJobParams{ID: job.ID, LeaseToken: job.LeaseToken, State: state, Blocker: materialText(code)}); e != nil {
			return e
		}
		return t.tx.Commit(ctx)
	}
	if row.CurrentRevisionID != revision.ID || revision.PublicationStatus == "merged" {
		return finish("done", "")
	}
	if !binding.Enabled {
		return finish("blocked", "material_disabled")
	}
	frozen, err := materialParse[materialBinding](revision.Settings)
	if err != nil {
		return err
	}
	if binding.Version != revision.ConfigVersion {
		return finish("blocked", "material_configuration_changed")
	}
	if _, err = checkMaterialBinding(ctx, t.q, row.WorkspaceID, row.ProjectID, frozen, false); err != nil {
		return finish("blocked", materialCode(err))
	}
	rev, err := materialParse[material.Revision](revision.Payload)
	if err != nil {
		return err
	}
	if rev.Kind == "plan" {
		base, e := materialParse[material.Baseline](row.Baseline)
		if e != nil || rev.SourceBaselineID == nil || *rev.SourceBaselineID != base.ID {
			return finish("blocked", "material_baseline_changed")
		}
	}
	if s.ResolveProvider == nil {
		return finish("blocked", "material_provider_unavailable")
	}
	provider, cleanup, err := s.ResolveProvider(ctx, t.q, row.WorkspaceID, binding.Settings)
	if err != nil {
		return finish("blocked", "material_connection_unavailable")
	}
	if cleanup != nil {
		defer cleanup()
	}
	if job.Kind == "publish" {
		review, e := provider.Publish(ctx, materialgit.PublishRequest{Key: rev.ID, Branch: "multica/material-" + rev.ID, BaseBranch: binding.BaseBranch, Title: "Multica engineering material " + rev.ID, Body: "Versioned engineering material. Business review and execution are managed in Multica. This change does not close an implementation issue.", Files: materialFiles(row, rev)})
		if e != nil {
			return materialProviderFailure(finish, e, "material_publication_unconfirmed")
		}
		if e = materialReviewMatches(row, rev, binding, review); e != nil {
			return finish("blocked", materialCode(e))
		}
		if review.State != "open" {
			return finish("blocked", "material_review_not_open")
		}
		if e = materialVerifyFiles(ctx, provider, row, rev, review.HeadSHA); e != nil {
			return finish("blocked", materialCode(e))
		}
		if e = t.q.ObserveMaterialRevision(ctx, db.ObserveMaterialRevisionParams{ID: revision.ID, PublicationStatus: "in_review", Review: materialJSON(review)}); e != nil {
			return e
		}
		status := "in_review"
		if rev.Kind == "plan" {
			status = "plan_review"
		}
		if _, e = updateMaterial(ctx, t.q, row, status, "", revision.ID, row.Baseline); e != nil {
			return e
		}
		return finish("done", "")
	}
	decision, err := t.q.LatestMaterialDecision(ctx, revision.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return finish("done", "")
	}
	if err != nil {
		return err
	}
	members, err := LockArtifactUsers(ctx, t.q, row.WorkspaceID, util.UUIDToString(decision.ActorID))
	if err != nil {
		return finish("blocked", "material_reviewer_revoked")
	}
	signer, err := materialParse[stageSignerCredential](decision.Credential)
	if err != nil {
		return err
	}
	if members[util.UUIDToString(decision.ActorID)].ID != decision.MemberID || !slices.Contains(binding.ReviewerIDs, util.UUIDToString(decision.ActorID)) {
		return finish("blocked", "material_reviewer_revoked")
	}
	if err = CheckArtifactCredentialInTx(ctx, t.tx, signer.credential(util.UUIDToString(decision.ActorID))); err != nil {
		return finish("blocked", "material_approval_expired")
	}
	stored, err := materialParse[materialMergeReceipt](revision.Review)
	if err != nil {
		return err
	}
	review, err := provider.Observe(ctx, stored.Number)
	if err != nil {
		return materialProviderFailure(finish, err, "material_review_unconfirmed")
	}
	if err = materialReviewMatches(row, rev, binding, review); err != nil {
		return finish("blocked", materialCode(err))
	}
	if review.Number != stored.Number || review.HeadSHA != decision.HeadSha || review.HeadSHA != stored.HeadSHA {
		return finish("blocked", "material_head_changed")
	}
	note := materialgit.DecisionNote{Key: util.UUIDToString(decision.ID), ActorID: util.UUIDToString(decision.ActorID), RevisionID: rev.ID, HeadSHA: review.HeadSHA, Decision: decision.Decision, Reason: decision.Reason, URL: ""}
	if err = provider.SyncDecision(ctx, review.Number, note); err != nil {
		return materialProviderFailure(finish, err, "material_review_sync_pending")
	}
	if decision.Decision != "approve" {
		return finish("done", "")
	}
	if err = materialVerifyFiles(ctx, provider, row, rev, review.HeadSHA); err != nil {
		return finish("blocked", materialCode(err))
	}
	decisionID := util.UUIDToString(decision.ID)
	if review.State == "merged" && stored.DecisionID != decisionID {
		return finish("blocked", "material_merge_not_authorized")
	}
	if review.State != "merged" {
		if review.State != "open" {
			return finish("blocked", "material_review_not_open")
		}
		if !review.GatesPassed {
			reason := review.BlockReason
			if reason == "" {
				reason = "material_native_gates_pending"
			}
			return finish("pending", reason)
		}
		if stored.DecisionID != decisionID {
			stored.DecisionID = decisionID
			if err = t.q.ObserveMaterialRevision(ctx, db.ObserveMaterialRevisionParams{ID: revision.ID, PublicationStatus: revision.PublicationStatus, Review: materialJSON(stored)}); err != nil {
				return err
			}
			if err = t.tx.Commit(ctx); err != nil {
				return err
			}
			return errMaterialMergePrepared
		}
		review, err = provider.Merge(ctx, materialgit.MergeExpectation{
			Number:     review.Number,
			HeadSHA:    decision.HeadSha,
			Branch:     review.Branch,
			BaseBranch: review.BaseBranch,
			Changes:    review.Changes,
		})
		if err != nil {
			return materialProviderFailure(finish, err, "material_merge_unconfirmed")
		}
	}
	if review.State != "merged" || !material.IsSHA(review.MergeSHA) {
		return finish("pending", "material_merge_unconfirmed")
	}
	if review.Number != stored.Number || review.HeadSHA != decision.HeadSha || review.MergeActor == "" {
		return finish("blocked", "material_review_changed")
	}
	if err = materialReviewMatches(row, rev, binding, review); err != nil {
		return finish("blocked", materialCode(err))
	}
	if err = materialVerifyFiles(ctx, provider, row, rev, review.MergeSHA); err != nil {
		return finish("blocked", materialCode(err))
	}
	if err = t.q.ObserveMaterialRevision(ctx, db.ObserveMaterialRevisionParams{ID: revision.ID, PublicationStatus: "merged", Review: materialJSON(review)}); err != nil {
		return err
	}
	if rev.Kind == "plan" {
		p, e := t.q.GetMaterialPlan(ctx, row.ID)
		if e != nil {
			return e
		}
		p.Status = "approved"
		p.RevisionID = revision.ID
		p.Blocker = pgtype.Text{}
		if e = saveMaterialPlan(ctx, t.q, p); e != nil {
			return e
		}
		if _, e = updateMaterial(ctx, t.q, row, "plan_approved", "", revision.ID, row.Baseline); e != nil {
			return e
		}
	} else {
		base := material.Baseline{ID: rev.ID, RevisionID: rev.ID, Commit: review.MergeSHA, Path: row.Path, ContentSHA256: rev.ContentSHA256, ReviewURL: review.URL, VerifiedAt: time.Now().UTC()}
		row, err = updateMaterial(ctx, t.q, row, "baseline_ready", "", revision.ID, materialJSON(base))
		if err != nil {
			return err
		}
		if err = ensureMaterialJob(ctx, t.q, row, revision.ID, "plan"); err != nil {
			return err
		}
	}
	return finish("done", "")
}

func materialPlanPrompt(row db.GitMaterial, base material.Baseline, rev material.Revision) string {
	return fmt.Sprintf(`Generate a technical design and task breakdown for the approved requirement below. Do not implement code, assign agents, create implementation issues, or change issue status. The requirement approval is not permission to code. Return the full design and full task plan through the material revision API/CLI with kind "plan", source_baseline_id "%s", fixed sources and stable acceptance IDs. Only the configured human reviewer can approve the plan.

Material ID: %s
Project ID: %s
Source revision: %s
Commit: %s
Path: %s
SHA256: %s

Use "multica project material get %s %s" to read the current version. Write a JSON request file matching RevisionInput (request_id, expected_version, kind, content, plan_content, sources, acceptance_criteria, blocking_questions, ready, source_baseline_id) and submit with "multica project material revise %s %s --file <request.json>". Reuse the exact request file after an uncertain response. The plan must remain a proposal; do not self-approve.

%s

--- BEGIN APPROVED PRD ---
%s
--- END APPROVED PRD ---`, base.ID, util.UUIDToString(row.ID), util.UUIDToString(row.ProjectID), base.RevisionID, base.Commit, base.Path, base.ContentSHA256, util.UUIDToString(row.ProjectID), util.UUIDToString(row.ID), util.UUIDToString(row.ProjectID), util.UUIDToString(row.ID), materialSourceNote(rev), rev.Content)
}

// A stored PRD is only usable while its exact baseline remains current.
func materialPlanBaseline(row db.GitMaterial, rev db.GitMaterialRevision) (material.Baseline, material.Revision, error) {
	base, err := materialParse[material.Baseline](row.Baseline)
	if err != nil {
		return base, material.Revision{}, err
	}
	content, err := materialParse[material.Revision](rev.Payload)
	if err != nil {
		return base, content, err
	}
	id := util.UUIDToString(rev.ID)
	if row.CurrentRevisionID != rev.ID || row.ID != rev.MaterialID || rev.Kind != "requirement" || rev.PublicationStatus != "merged" ||
		base.ID != id || base.RevisionID != id || content.ID != id || content.Kind != rev.Kind ||
		base.Path != row.Path || !material.IsSHA(base.Commit) || base.ContentSHA256 != content.ContentSHA256 || base.ContentSHA256 != material.Digest(content.Content) {
		return base, content, materialError(409, "material_baseline_changed")
	}
	return base, content, nil
}
func materialInvoke(ctx context.Context, q *db.Queries, agent db.Agent, actor pgtype.UUID) (bool, error) {
	if agent.ArchivedAt.Valid || auth.TaskCapabilityProfile(agent.TemplateKey, agent.TemplateVersion) != "" {
		return false, nil
	}
	targets, err := q.ListAgentInvocationTargets(ctx, agent.ID)
	if err != nil {
		return false, err
	}
	return auth.AgentInvocationAllowed(agent, targets, util.UUIDToString(actor), true, false), nil
}
func currentMaterialManager(ctx context.Context, q *db.Queries, ws, actor, memberID pgtype.UUID) (db.Member, error) {
	members, err := LockArtifactUsers(ctx, q, ws, util.UUIDToString(actor))
	var denied ArtifactError
	if err != nil && !errors.As(err, &denied) {
		return db.Member{}, err
	}
	member := members[util.UUIDToString(actor)]
	if err != nil || member.ID != memberID || (member.Role != "owner" && member.Role != "admin") {
		return db.Member{}, materialError(403, "material_configuration_authority_revoked")
	}
	return member, nil
}
func materialCurrentMember(ctx context.Context, q *db.Queries, ws, actor, member pgtype.UUID) error {
	members, err := LockArtifactUsers(ctx, q, ws, util.UUIDToString(actor))
	var denied ArtifactError
	if err != nil && !errors.As(err, &denied) {
		return err
	}
	if err != nil || members[util.UUIDToString(actor)].ID != member {
		return materialError(403, "material_plan_authority_revoked")
	}
	return nil
}

func materialProviderFailure(finish func(string, string) error, err error, fallback string) error {
	var e *materialgit.Error
	if errors.As(err, &e) {
		state := "blocked"
		if e.Retryable || e.Uncertain {
			state = "pending"
		}
		return finish(state, e.Code)
	}
	return finish("pending", fallback)
}
