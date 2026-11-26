package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path"
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
)

type MaterialError struct {
	Status int
	Code   string
}

func (e MaterialError) Error() string             { return e.Code }
func materialError(status int, code string) error { return MaterialError{status, code} }

type MaterialAccess struct {
	WorkspaceID, ProjectID pgtype.UUID
	Credential             ArtifactCredentialSnapshot
}
type MaterialProviderResolver func(context.Context, *db.Queries, pgtype.UUID, material.Settings) (materialgit.Provider, func(), error)
type MaterialService struct {
	Queries         *db.Queries
	TxStarter       TxStarter
	Issues          *IssueService
	Tasks           *TaskService
	ResolveProvider MaterialProviderResolver
}

func NewMaterialService(q *db.Queries, tx TxStarter, issues *IssueService, tasks *TaskService) *MaterialService {
	return &MaterialService{Queries: q, TxStarter: tx, Issues: issues, Tasks: tasks}
}

type materialBinding struct {
	material.Settings
	ResourceDigest     string `json:"resource_digest"`
	ConnectionRevision int64  `json:"connection_revision"`
}
type materialTx struct {
	tx     pgx.Tx
	q      *db.Queries
	access MaterialAccess
	actor  pgtype.UUID
	kind   string
	member db.Member
	task   db.AgentTaskQueue
}

func materialJSON(v any) []byte         { b, _ := json.Marshal(v); return b }
func materialText(s string) pgtype.Text { return pgtype.Text{String: s, Valid: s != ""} }
func materialPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
func materialUUIDPtr(id pgtype.UUID) *string {
	if !id.Valid {
		return nil
	}
	return materialPtr(util.UUIDToString(id))
}
func materialUUID(s string) (pgtype.UUID, error) {
	id, e := util.ParseUUID(s)
	if e != nil {
		return id, materialError(400, "invalid_material_id")
	}
	return id, nil
}
func materialParse[T any](raw []byte) (T, error) {
	var v T
	err := json.Unmarshal(raw, &v)
	return v, err
}

func (s *MaterialService) begin(ctx context.Context, a MaterialAccess, write bool) (*materialTx, error) {
	kind, id, err := a.Credential.Actor()
	if err != nil {
		return nil, materialError(403, "material_permission_denied")
	}
	actor, err := materialUUID(id)
	if err != nil {
		return nil, err
	}
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return nil, err
	}
	t := &materialTx{tx: tx, q: s.Queries.WithTx(tx), access: a, actor: actor, kind: kind}
	fail := func(e error) (*materialTx, error) { _ = tx.Rollback(ctx); return nil, e }
	if _, err = tx.Exec(ctx, "SET TRANSACTION ISOLATION LEVEL READ COMMITTED"); err != nil {
		return fail(err)
	}
	if _, err = t.q.LockWorkspaceForChatSessionCreate(ctx, a.WorkspaceID); err != nil {
		return fail(materialError(404, "project_not_found"))
	}
	user := a.Credential.Principal.UserID
	members, err := LockArtifactUsers(ctx, t.q, a.WorkspaceID, user)
	if err != nil {
		return fail(materialError(403, "forbidden"))
	}
	t.member = members[user]
	if err = CheckArtifactCredentialInTx(ctx, tx, a.Credential); err != nil {
		return fail(materialError(403, "material_credential_revoked"))
	}
	if write {
		_, err = t.q.LockProjectForExecutionSquad(ctx, db.LockProjectForExecutionSquadParams{ID: a.ProjectID, WorkspaceID: a.WorkspaceID})
	} else {
		_, err = t.q.LockProjectForAssociation(ctx, db.LockProjectForAssociationParams{ID: a.ProjectID, WorkspaceID: a.WorkspaceID})
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return fail(materialError(404, "project_not_found"))
	}
	if err != nil {
		return fail(err)
	}
	if kind == "agent" {
		token := a.Credential.TaskToken
		if token == nil || token.WorkspaceID != a.WorkspaceID {
			return fail(materialError(403, "forbidden"))
		}
		t.task, err = t.q.GetAgentTask(ctx, token.TaskID)
		if err != nil || t.task.AgentID != actor || !t.task.IssueID.Valid || t.task.Status != "running" {
			return fail(materialError(403, "material_task_scope_denied"))
		}
		issue, e := t.q.GetIssue(ctx, t.task.IssueID)
		if e != nil || issue.WorkspaceID != a.WorkspaceID || issue.ProjectID != a.ProjectID {
			return fail(materialError(403, "material_task_scope_denied"))
		}
		if _, e = t.q.GetMaterialPlanForIssue(ctx, issue.ID); e == nil {
			if e = AuthorizeMaterialPlanTask(ctx, t.q, t.task); e != nil {
				return fail(e)
			}
		} else if !errors.Is(e, pgx.ErrNoRows) || t.task.TriggerEvidenceKind.String == "material_baseline" {
			return fail(materialError(403, "material_task_scope_denied"))
		}
	}
	return t, nil
}
func (t *materialTx) human() error {
	if t.kind != "human" {
		return materialError(403, "material_human_required")
	}
	return nil
}
func (t *materialTx) manager() bool {
	return t.kind == "human" && (t.member.Role == "owner" || t.member.Role == "admin")
}
func (t *materialTx) binding(ctx context.Context) (db.ProjectMaterialSetting, materialBinding, error) {
	row, err := t.q.GetMaterialSettings(ctx, db.GetMaterialSettingsParams{ProjectID: t.access.ProjectID, WorkspaceID: t.access.WorkspaceID})
	if err != nil {
		return row, materialBinding{}, err
	}
	b, err := materialParse[materialBinding](row.Settings)
	return row, b, err
}
func (t *materialTx) get(ctx context.Context, id string) (db.GitMaterial, error) {
	uid, err := materialUUID(id)
	if err != nil {
		return db.GitMaterial{}, err
	}
	return t.q.GetGitMaterial(ctx, db.GetGitMaterialParams{WorkspaceID: t.access.WorkspaceID, ProjectID: t.access.ProjectID, ID: uid})
}
func (t *materialTx) request(ctx context.Context, key, action string, body any) ([]byte, string, error) {
	if len(key) < 8 || len(key) > 128 || strings.TrimSpace(key) != key {
		return nil, "", materialError(400, "invalid_request_id")
	}
	hash := material.JSONDigest(struct {
		Action string
		Body   any
	}{action, body})
	row, err := t.q.GetMaterialRequest(ctx, db.GetMaterialRequestParams{WorkspaceID: t.access.WorkspaceID, ProjectID: t.access.ProjectID, ActorID: t.actor, RequestID: key})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, hash, nil
	}
	if err != nil {
		return nil, hash, err
	}
	if row.BodyHash != hash {
		return nil, hash, materialError(409, "material_idempotency_conflict")
	}
	return row.Response, hash, nil
}
func (t *materialTx) saveRequest(ctx context.Context, key, hash string, response any) error {
	return t.q.CreateMaterialRequest(ctx, db.CreateMaterialRequestParams{WorkspaceID: t.access.WorkspaceID, ProjectID: t.access.ProjectID, ActorID: t.actor, RequestID: key, BodyHash: hash, Response: materialJSON(response)})
}

func materialView(row db.GitMaterial) material.Material {
	return material.Material{ID: util.UUIDToString(row.ID), WorkspaceID: util.UUIDToString(row.WorkspaceID), ProjectID: util.UUIDToString(row.ProjectID), Title: row.Title, Path: row.Path, Version: row.Version, Status: row.Status, Blocker: materialPtr(row.Blocker.String), CreatedAt: row.CreatedAt.Time, UpdatedAt: row.UpdatedAt.Time}
}
func (t *materialTx) detail(ctx context.Context, row db.GitMaterial) (material.Detail, error) {
	out := material.Detail{Material: materialView(row), Revisions: []material.Revision{}, Decisions: []material.Decision{}, Comments: []material.Comment{}}
	revisions, err := t.q.ListMaterialRevisions(ctx, row.ID)
	if err != nil {
		return out, err
	}
	for _, r := range revisions {
		v, e := materialParse[material.Revision](r.Payload)
		if e != nil {
			return out, e
		}
		v.PublicationStatus = r.PublicationStatus
		if len(r.Review) > 0 {
			review, e := materialParse[materialgit.Review](r.Review)
			if e != nil {
				return out, e
			}
			v.ReviewURL = materialPtr(review.URL)
			v.HeadSHA = materialPtr(review.HeadSHA)
		}
		out.Revisions = append(out.Revisions, v)
	}
	decisions, err := t.q.ListMaterialDecisions(ctx, row.ID)
	if err != nil {
		return out, err
	}
	for _, d := range decisions {
		out.Decisions = append(out.Decisions, material.Decision{ID: util.UUIDToString(d.ID), RevisionID: util.UUIDToString(d.RevisionID), ActorID: util.UUIDToString(d.ActorID), Decision: d.Decision, Reason: d.Reason, HeadSHA: d.HeadSha, CreatedAt: d.CreatedAt.Time})
	}
	comments, err := t.q.ListMaterialComments(ctx, row.ID)
	if err != nil {
		return out, err
	}
	for _, c := range comments {
		out.Comments = append(out.Comments, material.Comment{ID: util.UUIDToString(c.ID), RevisionID: util.UUIDToString(c.RevisionID), ActorID: util.UUIDToString(c.ActorID), Body: c.Body, CreatedAt: c.CreatedAt.Time})
	}
	if len(row.Baseline) > 0 {
		v, e := materialParse[material.Baseline](row.Baseline)
		if e != nil {
			return out, e
		}
		out.Baseline = &v
	}
	plan, err := t.q.GetMaterialPlan(ctx, row.ID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return out, err
	}
	if err == nil {
		status := plan.Status
		canRestart := !plan.TaskID.Valid
		if plan.TaskID.Valid && (status == "queued" || status == "running") {
			task, e := t.q.GetAgentTask(ctx, plan.TaskID)
			if e != nil {
				return out, e
			}
			switch task.Status {
			case "running":
				status = "running"
			case "completed", "failed", "cancelled":
				status = "blocked"
				canRestart = true
			}
		}
		out.Plan = &material.Plan{IssueID: materialUUIDPtr(plan.IssueID), Status: status, Blocker: materialPtr(plan.Blocker.String), BaselineID: util.UUIDToString(plan.BaselineID), RevisionID: materialUUIDPtr(plan.RevisionID)}
		out.Capabilities.CanStartPlan = canRestart
	}
	_, binding, err := t.binding(ctx)
	if err == nil && binding.Enabled {
		out.Capabilities.CanEdit = true
		out.Capabilities.CanReview = t.kind == "human" && slices.Contains(binding.ReviewerIDs, util.UUIDToString(t.actor))
		out.Capabilities.CanRetry = t.kind == "human"
		out.Capabilities.CanStartPlan = out.Capabilities.CanStartPlan && t.kind == "human" && out.Plan != nil && row.CurrentRevisionID == plan.BaselineID && (out.Plan.Status == "waiting_manual" || out.Plan.Status == "blocked")
	} else {
		out.Capabilities.CanStartPlan = false
	}
	return out, nil
}
func (s *MaterialService) Settings(ctx context.Context, a MaterialAccess) (material.SettingsEnvelope, error) {
	out := material.SettingsEnvelope{WorkspaceID: util.UUIDToString(a.WorkspaceID), ProjectID: util.UUIDToString(a.ProjectID)}
	t, err := s.begin(ctx, a, false)
	if err != nil {
		return out, err
	}
	defer t.tx.Rollback(ctx)
	out.CanConfigure = t.manager()
	_, b, err := t.binding(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, t.tx.Commit(ctx)
	}
	if err != nil {
		return out, err
	}
	out.Settings = &b.Settings
	return out, t.tx.Commit(ctx)
}
func (s *MaterialService) List(ctx context.Context, a MaterialAccess) (material.List, error) {
	out := material.List{WorkspaceID: util.UUIDToString(a.WorkspaceID), ProjectID: util.UUIDToString(a.ProjectID), Materials: []material.Material{}}
	t, err := s.begin(ctx, a, false)
	if err != nil {
		return out, err
	}
	defer t.tx.Rollback(ctx)
	rows, err := t.q.ListGitMaterials(ctx, db.ListGitMaterialsParams{WorkspaceID: a.WorkspaceID, ProjectID: a.ProjectID})
	if err != nil {
		return out, err
	}
	for _, r := range rows {
		out.Materials = append(out.Materials, materialView(r))
	}
	return out, t.tx.Commit(ctx)
}
func (s *MaterialService) Detail(ctx context.Context, a MaterialAccess, id string) (material.Detail, error) {
	t, e := s.begin(ctx, a, false)
	if e != nil {
		return material.Detail{}, e
	}
	defer t.tx.Rollback(ctx)
	row, e := t.get(ctx, id)
	if e != nil {
		return material.Detail{}, e
	}
	out, e := t.detail(ctx, row)
	if e != nil {
		return out, e
	}
	return out, t.tx.Commit(ctx)
}

func resourceRepository(raw []byte) (host, repo string, err error) {
	var ref struct {
		URL string `json:"url"`
	}
	if json.Unmarshal(raw, &ref) != nil {
		return "", "", errors.New("invalid_resource")
	}
	u, err := url.Parse(ref.URL)
	if err != nil || u.Host == "" {
		text := ref.URL
		if i := strings.Index(text, "@"); i >= 0 {
			text = text[i+1:]
		}
		p := strings.SplitN(text, ":", 2)
		if len(p) != 2 {
			return "", "", errors.New("invalid_resource")
		}
		return strings.ToLower(p[0]), strings.TrimSuffix(strings.Trim(p[1], "/"), ".git"), nil
	}
	return strings.ToLower(u.Host), strings.TrimSuffix(strings.Trim(u.Path, "/"), ".git"), nil
}
func checkMaterialBinding(ctx context.Context, q *db.Queries, ws, project pgtype.UUID, b materialBinding, freeze bool) (materialBinding, error) {
	resourceID, e := materialUUID(b.ResourceID)
	if e != nil {
		return b, e
	}
	resource, e := q.GetProjectResource(ctx, resourceID)
	if e != nil && !errors.Is(e, pgx.ErrNoRows) {
		return b, e
	}
	if e != nil || resource.ProjectID != project || resource.WorkspaceID != ws {
		return b, materialError(403, "material_resource_unavailable")
	}
	if resource.ResourceType != "github_repo" {
		return b, materialError(400, "material_git_resource_required")
	}
	host, repo, e := resourceRepository(resource.ResourceRef)
	if e != nil || repo != b.Repository {
		return b, materialError(409, "material_repository_mismatch")
	}
	digest := material.Digest(string(resource.ResourceRef))
	if !freeze && digest != b.ResourceDigest {
		return b, materialError(409, "material_resource_changed")
	}
	b.ResourceDigest = digest
	if b.ConnectionID == nil {
		return b, materialError(400, "material_connection_required")
	}
	connection, e := materialUUID(*b.ConnectionID)
	if e != nil {
		return b, e
	}
	var revision int64
	if b.Provider == "gitlab" {
		c, e := q.GetVCSConnectionByID(ctx, connection)
		if e != nil && !errors.Is(e, pgx.ErrNoRows) {
			return b, e
		}
		if e != nil || c.WorkspaceID != ws || c.Provider != "gitlab" {
			return b, materialError(403, "material_connection_unavailable")
		}
		u, e := url.Parse(c.InstanceUrl)
		if e != nil || !strings.EqualFold(u.Host, host) {
			return b, materialError(409, "material_repository_mismatch")
		}
		revision = c.TrustRevision
	} else {
		c, e := q.GetGitHubInstallationByID(ctx, connection)
		if e != nil && !errors.Is(e, pgx.ErrNoRows) {
			return b, e
		}
		if e != nil || c.WorkspaceID != ws || host != "github.com" || !strings.EqualFold(strings.Split(b.Repository, "/")[0], c.AccountLogin) {
			return b, materialError(403, "material_connection_unavailable")
		}
		revision = c.TrustRevision
	}
	if !freeze && revision != b.ConnectionRevision {
		return b, materialError(409, "material_connection_changed")
	}
	b.ConnectionRevision = revision
	return b, nil
}
func (s *MaterialService) Configure(ctx context.Context, a MaterialAccess, in material.SettingsInput) (material.SettingsEnvelope, error) {
	out := material.SettingsEnvelope{WorkspaceID: util.UUIDToString(a.WorkspaceID), ProjectID: util.UUIDToString(a.ProjectID), CanConfigure: true}
	setting := in.Settings()
	if err := material.ValidateSettings(setting); err != nil {
		return out, materialError(400, err.Error())
	}
	t, err := s.begin(ctx, a, true)
	if err != nil {
		return out, err
	}
	defer t.tx.Rollback(ctx)
	if !t.manager() {
		return out, materialError(403, "project_permission_denied")
	}
	saved, hash, err := t.request(ctx, in.RequestID, "settings", in)
	if err != nil {
		return out, err
	}
	if saved != nil {
		out, err = materialParse[material.SettingsEnvelope](saved)
		return out, err
	}
	row, old, err := t.binding(ctx)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return out, err
	}
	if row.Version != in.ExpectedVersion {
		return out, materialError(409, "material_version_changed")
	}
	if row.Version > 0 && in.AutoStartPlan == nil {
		setting.AutoStartPlan = old.AutoStartPlan
	}
	if row.Version > 0 && (old.RootPath != setting.RootPath || old.ResourceID != setting.ResourceID || old.Repository != setting.Repository || old.Provider != setting.Provider || old.BaseBranch != setting.BaseBranch) {
		has, e := t.q.MaterialProjectHasHistory(ctx, db.MaterialProjectHasHistoryParams{ProjectID: a.ProjectID, WorkspaceID: a.WorkspaceID})
		if e != nil {
			return out, e
		}
		if has {
			return out, materialError(409, "material_target_has_history")
		}
	}
	seen := map[string]bool{}
	for _, id := range setting.ReviewerIDs {
		if seen[id] {
			return out, materialError(400, "duplicate_reviewer")
		}
		seen[id] = true
		if _, e := LockArtifactUsers(ctx, t.q, a.WorkspaceID, id); e != nil {
			return out, materialError(403, "material_reviewer_unavailable")
		}
	}
	if setting.PlanAgentID != nil {
		id, e := materialUUID(*setting.PlanAgentID)
		if e != nil {
			return out, e
		}
		agent, e := t.q.GetAgent(ctx, id)
		if e != nil || agent.WorkspaceID != a.WorkspaceID || agent.ArchivedAt.Valid {
			return out, materialError(403, "material_agent_unavailable")
		}
	}
	binding, err := checkMaterialBinding(ctx, t.q, a.WorkspaceID, a.ProjectID, materialBinding{Settings: setting}, true)
	if err != nil {
		return out, err
	}
	_, err = t.q.SaveMaterialSettings(ctx, db.SaveMaterialSettingsParams{ProjectID: a.ProjectID, WorkspaceID: a.WorkspaceID, Version: setting.Version, Settings: materialJSON(binding), AuthorizedBy: t.actor, MemberID: t.member.ID})
	if err != nil {
		return out, err
	}
	out.Settings = &setting
	if err = t.saveRequest(ctx, in.RequestID, hash, out); err != nil {
		return out, err
	}
	return out, t.tx.Commit(ctx)
}
func (s *MaterialService) Create(ctx context.Context, a MaterialAccess, in material.CreateInput) (material.Detail, error) {
	var out material.Detail
	if len(strings.TrimSpace(in.Title)) == 0 || len(in.Title) > 500 {
		return out, materialError(400, "invalid_material_title")
	}
	t, err := s.begin(ctx, a, true)
	if err != nil {
		return out, err
	}
	defer t.tx.Rollback(ctx)
	saved, hash, err := t.request(ctx, in.RequestID, "create", in)
	if err != nil {
		return out, err
	}
	if saved != nil {
		return materialParse[material.Detail](saved)
	}
	_, binding, err := t.binding(ctx)
	if err != nil {
		return out, materialError(409, "material_not_configured")
	}
	if !binding.Enabled {
		return out, materialError(409, "material_disabled")
	}
	id := dbid.NewV7()
	row, err := t.q.CreateGitMaterial(ctx, db.CreateGitMaterialParams{ID: id, WorkspaceID: a.WorkspaceID, ProjectID: a.ProjectID, Title: strings.TrimSpace(in.Title), Path: path.Join(binding.RootPath, "requirements", util.UUIDToString(id), "prd.md")})
	if err != nil {
		return out, err
	}
	out, err = t.detail(ctx, row)
	if err != nil {
		return out, err
	}
	if err = t.saveRequest(ctx, in.RequestID, hash, out); err != nil {
		return out, err
	}
	return out, t.tx.Commit(ctx)
}
func updateMaterial(ctx context.Context, q *db.Queries, row db.GitMaterial, status, blocker string, revision pgtype.UUID, baseline []byte) (db.GitMaterial, error) {
	return q.UpdateGitMaterial(ctx, db.UpdateGitMaterialParams{WorkspaceID: row.WorkspaceID, ProjectID: row.ProjectID, ID: row.ID, Status: status, Blocker: materialText(blocker), CurrentRevisionID: revision, Baseline: baseline})
}
func ensureMaterialJob(ctx context.Context, q *db.Queries, row db.GitMaterial, rev pgtype.UUID, kind string) error {
	return q.EnsureMaterialJob(ctx, db.EnsureMaterialJobParams{ID: dbid.NewV7(), WorkspaceID: row.WorkspaceID, ProjectID: row.ProjectID, MaterialID: row.ID, RevisionID: rev, Kind: kind})
}

func (s *MaterialService) Revise(ctx context.Context, a MaterialAccess, id string, in material.RevisionInput) (material.Detail, error) {
	var out material.Detail
	if err := material.ValidateRevision(in); err != nil {
		return out, materialError(422, err.Error())
	}
	t, err := s.begin(ctx, a, true)
	if err != nil {
		return out, err
	}
	defer t.tx.Rollback(ctx)
	saved, hash, err := t.request(ctx, in.RequestID, "revise:"+id, in)
	if err != nil {
		return out, err
	}
	if saved != nil {
		return materialParse[material.Detail](saved)
	}
	row, err := t.get(ctx, id)
	if err != nil {
		return out, err
	}
	if row.Version != in.ExpectedVersion {
		return out, materialError(409, "material_version_changed")
	}
	_, binding, err := t.binding(ctx)
	if err != nil || !binding.Enabled {
		return out, materialError(409, "material_disabled")
	}
	if _, err = checkMaterialBinding(ctx, t.q, a.WorkspaceID, a.ProjectID, binding, false); err != nil {
		return out, err
	}
	for _, src := range in.Sources {
		sid, e := materialUUID(src.ResourceID)
		if e != nil {
			return out, e
		}
		source, e := t.q.GetProjectResource(ctx, sid)
		if e != nil || source.ProjectID != a.ProjectID || source.WorkspaceID != a.WorkspaceID {
			return out, materialError(403, "material_source_unavailable")
		}
	}
	if in.Kind == "plan" {
		base, e := materialParse[material.Baseline](row.Baseline)
		if e != nil || in.SourceBaselineID == nil || *in.SourceBaselineID != base.ID {
			return out, materialError(409, "material_baseline_changed")
		}
		plan, e := t.q.GetMaterialPlan(ctx, row.ID)
		if e != nil || util.UUIDToString(plan.BaselineID) != base.ID {
			return out, materialError(409, "material_plan_unavailable")
		}
		if row.CurrentRevisionID != plan.BaselineID && row.CurrentRevisionID != plan.RevisionID {
			return out, materialError(409, "material_baseline_changed")
		}
		if t.kind == "agent" && (!plan.TaskID.Valid || plan.TaskID != t.task.ID) {
			return out, materialError(403, "material_plan_task_denied")
		}
		if t.kind == "agent" && row.CurrentRevisionID != plan.BaselineID {
			current, e := t.q.GetMaterialRevision(ctx, row.CurrentRevisionID)
			if e != nil || current.CreatedBy != t.actor || current.PublicationStatus != "draft" {
				return out, materialError(403, "material_plan_task_denied")
			}
		}
	} else if t.kind == "agent" {
		if _, e := t.q.GetMaterialPlanForIssue(ctx, t.task.IssueID); e == nil {
			return out, materialError(403, "material_plan_task_denied")
		} else if !errors.Is(e, pgx.ErrNoRows) {
			return out, e
		}
	}
	revID := dbid.NewV7()
	state := "draft"
	status := "draft"
	if in.Ready {
		state = "publishing"
		status = "publishing"
	}
	rev := material.Revision{ID: util.UUIDToString(revID), Kind: in.Kind, Content: in.Content, PlanContent: in.PlanContent, ContentSHA256: material.Digest(in.Content), Sources: in.Sources, AcceptanceCriteria: in.AcceptanceCriteria, BlockingQuestions: in.BlockingQuestions, SourceBaselineID: in.SourceBaselineID, PublicationStatus: state, CreatedAt: time.Now().UTC()}
	if rev.Sources == nil {
		rev.Sources = []material.Source{}
	}
	if rev.AcceptanceCriteria == nil {
		rev.AcceptanceCriteria = []string{}
	}
	if rev.BlockingQuestions == nil {
		rev.BlockingQuestions = []string{}
	}
	_, err = t.q.CreateMaterialRevision(ctx, db.CreateMaterialRevisionParams{ID: revID, MaterialID: row.ID, WorkspaceID: row.WorkspaceID, ProjectID: row.ProjectID, Kind: in.Kind, Payload: materialJSON(rev), Settings: materialJSON(binding), ConfigVersion: binding.Version, CreatedBy: t.actor, PublicationStatus: state})
	if err != nil {
		return out, err
	}
	row, err = updateMaterial(ctx, t.q, row, status, "", revID, row.Baseline)
	if err != nil {
		return out, err
	}
	if plan, e := t.q.GetMaterialPlan(ctx, row.ID); e == nil {
		plan.Status = "blocked"
		plan.Blocker = materialText("material_revision_changed")
		if in.Kind == "plan" {
			plan.Status = "awaiting_review"
			plan.RevisionID = revID
			plan.Blocker = pgtype.Text{}
		}
		if err = saveMaterialPlan(ctx, t.q, plan); err != nil {
			return out, err
		}
	} else if !errors.Is(e, pgx.ErrNoRows) {
		return out, e
	}
	if in.Ready {
		if err = ensureMaterialJob(ctx, t.q, row, revID, "publish"); err != nil {
			return out, err
		}
	}
	out, err = t.detail(ctx, row)
	if err != nil {
		return out, err
	}
	if err = t.saveRequest(ctx, in.RequestID, hash, out); err != nil {
		return out, err
	}
	return out, t.tx.Commit(ctx)
}
func saveMaterialPlan(ctx context.Context, q *db.Queries, p db.GitMaterialPlan) error {
	return q.UpdateMaterialPlan(ctx, db.UpdateMaterialPlanParams{MaterialID: p.MaterialID, BaselineID: p.BaselineID, Status: p.Status, Requested: p.Requested, AuthorizedBy: p.AuthorizedBy, MemberID: p.MemberID, AgentID: p.AgentID, TaskID: p.TaskID, RevisionID: p.RevisionID, Blocker: p.Blocker})
}

func (s *MaterialService) Decide(ctx context.Context, a MaterialAccess, id string, in material.DecisionInput) (material.Detail, error) {
	var out material.Detail
	if (in.Decision != "approve" && in.Decision != "request_changes" && in.Decision != "withdraw") || len(in.Reason) > 4000 || !material.IsSHA(in.HeadSHA) {
		return out, materialError(400, "invalid_material_decision")
	}
	t, err := s.begin(ctx, a, true)
	if err != nil {
		return out, err
	}
	defer t.tx.Rollback(ctx)
	if err = t.human(); err != nil {
		return out, err
	}
	saved, hash, err := t.request(ctx, in.RequestID, "decide:"+id, in)
	if err != nil {
		return out, err
	}
	if saved != nil {
		return materialParse[material.Detail](saved)
	}
	row, err := t.get(ctx, id)
	if err != nil {
		return out, err
	}
	if row.Version != in.ExpectedVersion || util.UUIDToString(row.CurrentRevisionID) != in.RevisionID {
		return out, materialError(409, "material_version_changed")
	}
	_, binding, err := t.binding(ctx)
	if err != nil || !binding.Enabled {
		return out, materialError(409, "material_disabled")
	}
	if !slices.Contains(binding.ReviewerIDs, util.UUIDToString(t.actor)) {
		return out, materialError(403, "project_permission_denied")
	}
	rev, err := t.q.GetMaterialRevision(ctx, row.CurrentRevisionID)
	if err != nil {
		return out, err
	}
	if rev.ConfigVersion != binding.Version {
		return out, materialError(409, "material_configuration_changed")
	}
	review, err := materialParse[materialgit.Review](rev.Review)
	if err != nil || review.HeadSHA != in.HeadSHA || review.State != "open" {
		return out, materialError(409, "material_review_changed")
	}
	if rev.Kind == "plan" {
		payload, e := materialParse[material.Revision](rev.Payload)
		base, e2 := materialParse[material.Baseline](row.Baseline)
		if e != nil || e2 != nil || payload.SourceBaselineID == nil || *payload.SourceBaselineID != base.ID {
			return out, materialError(409, "material_baseline_changed")
		}
	}
	if prior, e := t.q.LatestMaterialDecision(ctx, row.CurrentRevisionID); e == nil && in.Decision == "approve" && prior.Decision != "approve" {
		return out, materialError(409, "material_revision_required")
	} else if e != nil && !errors.Is(e, pgx.ErrNoRows) {
		return out, e
	}
	// Retain credential metadata, never bearer tokens. Final merge consumes this original signer.
	credential := stageSignerCredential{Version: 1, Kind: a.Credential.Principal.Kind, PasswordSession: a.Credential.PasswordSession, PATHash: a.Credential.Principal.PATTokenHash, SourceMemberID: util.UUIDToString(t.member.ID)}
	if a.Credential.Principal.Kind == auth.CredentialJWT {
		at := a.Credential.Principal.ExpiresAt
		credential.ExpiresAt = &at
	}
	_, err = t.q.CreateMaterialDecision(ctx, db.CreateMaterialDecisionParams{ID: dbid.NewV7(), MaterialID: row.ID, RevisionID: row.CurrentRevisionID, WorkspaceID: row.WorkspaceID, ProjectID: row.ProjectID, ActorID: t.actor, MemberID: t.member.ID, Credential: materialJSON(credential), Decision: in.Decision, Reason: strings.TrimSpace(in.Reason), HeadSha: in.HeadSHA})
	if err != nil {
		return out, err
	}
	status := "changes_requested"
	if in.Decision == "approve" {
		status = "approved"
	}
	row, err = updateMaterial(ctx, t.q, row, status, "", row.CurrentRevisionID, row.Baseline)
	if err != nil {
		return out, err
	}
	if err = ensureMaterialJob(ctx, t.q, row, row.CurrentRevisionID, "review"); err != nil {
		return out, err
	}
	out, err = t.detail(ctx, row)
	if err != nil {
		return out, err
	}
	if err = t.saveRequest(ctx, in.RequestID, hash, out); err != nil {
		return out, err
	}
	return out, t.tx.Commit(ctx)
}
func (s *MaterialService) Comment(ctx context.Context, a MaterialAccess, id string, in material.CommentInput) (material.Detail, error) {
	var out material.Detail
	if strings.TrimSpace(in.Body) == "" || len(in.Body) > 16000 {
		return out, materialError(400, "invalid_material_comment")
	}
	t, err := s.begin(ctx, a, true)
	if err != nil {
		return out, err
	}
	defer t.tx.Rollback(ctx)
	if err = t.human(); err != nil {
		return out, err
	}
	saved, hash, err := t.request(ctx, in.RequestID, "comment:"+id, in)
	if err != nil {
		return out, err
	}
	if saved != nil {
		return materialParse[material.Detail](saved)
	}
	row, err := t.get(ctx, id)
	if err != nil {
		return out, err
	}
	revID, err := materialUUID(in.RevisionID)
	if err != nil {
		return out, err
	}
	rev, err := t.q.GetMaterialRevision(ctx, revID)
	if err != nil || rev.MaterialID != row.ID {
		return out, materialError(404, "material_revision_not_found")
	}
	_, err = t.q.CreateMaterialComment(ctx, db.CreateMaterialCommentParams{ID: dbid.NewV7(), MaterialID: row.ID, RevisionID: revID, WorkspaceID: row.WorkspaceID, ProjectID: row.ProjectID, ActorID: t.actor, Body: strings.TrimSpace(in.Body)})
	if err != nil {
		return out, err
	}
	out, err = t.detail(ctx, row)
	if err != nil {
		return out, err
	}
	if err = t.saveRequest(ctx, in.RequestID, hash, out); err != nil {
		return out, err
	}
	return out, t.tx.Commit(ctx)
}
func (s *MaterialService) Command(ctx context.Context, a MaterialAccess, id, command string, in material.CommandInput) (material.Detail, error) {
	var out material.Detail
	t, err := s.begin(ctx, a, true)
	if err != nil {
		return out, err
	}
	defer t.tx.Rollback(ctx)
	if err = t.human(); err != nil {
		return out, err
	}
	saved, hash, err := t.request(ctx, in.RequestID, command+":"+id, in)
	if err != nil {
		return out, err
	}
	if saved != nil {
		return materialParse[material.Detail](saved)
	}
	row, err := t.get(ctx, id)
	if err != nil {
		return out, err
	}
	if row.Version != in.ExpectedVersion {
		return out, materialError(409, "material_version_changed")
	}
	_, binding, err := t.binding(ctx)
	if err != nil || !binding.Enabled {
		return out, materialError(409, "material_disabled")
	}
	kind := "publish"
	if command == "start-plan" {
		p, e := t.q.GetMaterialPlan(ctx, row.ID)
		if e != nil {
			return out, materialError(409, "material_plan_unavailable")
		}
		base, e := materialParse[material.Baseline](row.Baseline)
		if e != nil || util.UUIDToString(p.BaselineID) != base.ID || row.CurrentRevisionID != p.BaselineID {
			return out, materialError(409, "material_baseline_changed")
		}
		if p.TaskID.Valid {
			prior, e := t.q.LockAgentTaskForStart(ctx, p.TaskID)
			if e != nil {
				return out, e
			}
			if prior.Status != "failed" && prior.Status != "cancelled" && prior.Status != "completed" {
				return out, materialError(409, "material_plan_already_started")
			}
			// Keep the old queue row as run history. Only this explicit human
			// command may release the plan's pointer for one new attempt.
			p.TaskID = pgtype.UUID{}
		}
		p.Requested = true
		if err = t.q.MarkMaterialPlanManualStart(ctx, db.MarkMaterialPlanManualStartParams{MaterialID: row.ID, BaselineID: p.BaselineID}); err != nil {
			return out, err
		}
		p.Status = "blocked"
		p.Blocker = materialText("material_plan_start_pending")
		p.AuthorizedBy = t.actor
		p.MemberID = t.member.ID
		if binding.PlanAgentID == nil {
			return out, materialError(409, "material_plan_agent_unavailable")
		}
		p.AgentID, err = materialUUID(*binding.PlanAgentID)
		if err != nil {
			return out, err
		}
		agent, e := t.q.GetAgent(ctx, p.AgentID)
		if e != nil && !errors.Is(e, pgx.ErrNoRows) {
			return out, e
		}
		allowed, invokeErr := materialInvoke(ctx, t.q, agent, t.actor)
		if invokeErr != nil {
			return out, invokeErr
		}
		if e != nil || agent.WorkspaceID != row.WorkspaceID || !allowed {
			return out, materialError(403, "material_plan_invocation_denied")
		}
		if err = saveMaterialPlan(ctx, t.q, p); err != nil {
			return out, err
		}
		kind = "plan"
	} else {
		rev, e := t.q.GetMaterialRevision(ctx, row.CurrentRevisionID)
		if e != nil {
			return out, e
		}
		if rev.PublicationStatus == "draft" {
			return out, materialError(409, "material_not_reviewable")
		}
		if len(rev.Review) > 0 {
			kind = "review"
		}
		if rev.PublicationStatus == "merged" && rev.Kind == "requirement" {
			kind = "plan"
		} else if rev.PublicationStatus == "merged" {
			kind = ""
		}
	}
	if kind != "" {
		if err = ensureMaterialJob(ctx, t.q, row, row.CurrentRevisionID, kind); err != nil {
			return out, err
		}
		row, err = updateMaterial(ctx, t.q, row, row.Status, "", row.CurrentRevisionID, row.Baseline)
		if err != nil {
			return out, err
		}
	}
	out, err = t.detail(ctx, row)
	if err != nil {
		return out, err
	}
	if err = t.saveRequest(ctx, in.RequestID, hash, out); err != nil {
		return out, err
	}
	return out, t.tx.Commit(ctx)
}

func materialCode(err error) string {
	var e MaterialError
	if errors.As(err, &e) {
		return e.Code
	}
	return "material_operation_unavailable"
}
func materialSourceNote(rev material.Revision) string {
	return fmt.Sprintf("Approved requirement source manifest:\n%s\nAcceptance criteria:\n%s", materialJSON(rev.Sources), strings.Join(rev.AcceptanceCriteria, "\n"))
}
