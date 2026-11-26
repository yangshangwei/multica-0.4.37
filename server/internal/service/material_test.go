package service

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/material"
	"github.com/multica-ai/multica/server/internal/materialgit"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type materialFakeGit struct {
	mu                     sync.Mutex
	reviews                map[int64]materialgit.Review
	files                  map[string]string
	branches               map[string]int64
	publish, merge         int
	lostPublish, lostMerge bool
	gates                  bool
	changeHead             bool
	extra                  bool
	badContent             bool
	extraAfterMerge        bool
}

func newMaterialFake() *materialFakeGit {
	return &materialFakeGit{reviews: map[int64]materialgit.Review{}, files: map[string]string{}, branches: map[string]int64{}, gates: true}
}
func (p *materialFakeGit) Publish(_ context.Context, in materialgit.PublishRequest) (materialgit.Review, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.publish++
	n := p.branches[in.Branch]
	if n == 0 {
		n = int64(len(p.reviews) + 1)
		r := materialgit.Review{Number: n, URL: "https://git.example/review", HeadSHA: material.Digest(in.Key)[:40], BaseBranch: in.BaseBranch, Branch: in.Branch, State: "open", GatesPassed: p.gates}
		for _, f := range in.Files {
			r.Changes = append(r.Changes, materialgit.Change{Path: f.Path, Mode: "100644", Status: "added"})
			p.files[r.HeadSHA+":"+f.Path] = f.Content
		}
		p.reviews[n] = r
		p.branches[in.Branch] = n
	}
	if p.lostPublish {
		p.lostPublish = false
		return materialgit.Review{}, errors.New("lost publish response")
	}
	return p.reviews[n], nil
}
func (p *materialFakeGit) Observe(_ context.Context, n int64) (materialgit.Review, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	r := p.reviews[n]
	r.GatesPassed = p.gates
	if p.changeHead {
		r.HeadSHA = strings.Repeat("f", 40)
	}
	if p.extra {
		r.Changes = append(r.Changes, materialgit.Change{Path: "AGENTS.md", Mode: "100644"})
	}
	return r, nil
}
func (p *materialFakeGit) SyncDecision(context.Context, int64, materialgit.DecisionNote) error {
	return nil
}
func (p *materialFakeGit) Merge(_ context.Context, expected materialgit.MergeExpectation) (materialgit.Review, error) {
	n, head := expected.Number, expected.HeadSHA
	p.mu.Lock()
	defer p.mu.Unlock()
	r := p.reviews[n]
	if r.HeadSHA != head || !p.gates {
		return r, errors.New("merge rejected")
	}
	p.merge++
	r.State = "merged"
	r.MergeSHA = strings.Repeat("b", 40)
	r.MergeActor = "integration"
	for _, c := range r.Changes {
		p.files[r.MergeSHA+":"+c.Path] = p.files[r.HeadSHA+":"+c.Path]
	}
	p.reviews[n] = r
	if p.lostMerge {
		p.lostMerge = false
		return materialgit.Review{}, errors.New("lost merge response")
	}
	if p.extraAfterMerge {
		r.Changes = append(r.Changes, materialgit.Change{Path: "another.md", Mode: "100644", Status: "added"})
	}
	return r, nil
}
func (p *materialFakeGit) ReadFile(_ context.Context, commit, path string) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.badContent {
		return "changed bytes", nil
	}
	v, ok := p.files[commit+":"+path]
	if !ok {
		return "", errors.New("missing file")
	}
	return v, nil
}

type materialFixture struct {
	pool                        *pgxpool.Pool
	fx                          *testutil.Fixture
	svc                         *MaterialService
	git                         *materialFakeGit
	access                      MaterialAccess
	settings                    material.SettingsInput
	ws, user, project, resource string
}

func newMaterialFixture(t *testing.T, auto bool) *materialFixture {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("requires task-owned DATABASE_URL")
	}
	ctx := context.Background()
	pool, e := pgxpool.New(ctx, url)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(pool.Close)
	fx := testutil.New(pool, "", "")
	suffix := uuid.NewString()
	user := fx.User(t, "Material owner", "material-"+suffix+"@example.test")
	ws := fx.Workspace(t, "Material project", "material-"+suffix)
	fx.Member(t, ws, user, "owner")
	fx.WorkspaceID, fx.UserID = ws, user
	project := fx.Project(t, "Engineering materials")
	resource := fx.Insert(t, "project_resource", testutil.Cols{"workspace_id": ws, "project_id": project, "resource_type": "github_repo", "resource_ref": `{"url":"https://github.com/example/materials.git"}`, "position": 0})
	conn := fx.Insert(t, "github_installation", testutil.Cols{"workspace_id": ws, "installation_id": time.Now().UnixNano(), "account_login": "example", "account_type": "Organization"})
	q := db.New(pool)
	if e = q.SeedIssueStatusEntries(ctx, util.MustParseUUID(ws)); e != nil {
		t.Fatal(e)
	}
	for _, table := range []string{"issue_status", "issue", "project_material_settings", "git_material", "git_material_revision", "git_material_decision", "git_material_comment", "git_material_request", "git_material_job", "git_material_plan"} {
		fx.Cleanup(t, `DELETE FROM `+table+` WHERE workspace_id=$1`, ws)
	}
	fx.Cleanup(t, `DELETE FROM agent_task_queue WHERE issue_id IN (SELECT id FROM issue WHERE workspace_id=$1)`, ws)
	f := &materialFixture{pool: pool, fx: fx, ws: ws, user: user, project: project, resource: resource, git: newMaterialFake()}
	tasks := NewTaskService(q, pool, nil, events.New(), nil)
	issues := NewIssueService(q, pool, nil, nil, tasks)
	f.svc = NewMaterialService(q, pool, issues, tasks)
	f.svc.ResolveProvider = func(context.Context, *db.Queries, pgtype.UUID, material.Settings) (materialgit.Provider, func(), error) {
		return f.git, nil, nil
	}
	f.access = MaterialAccess{WorkspaceID: util.MustParseUUID(ws), ProjectID: util.MustParseUUID(project), Credential: stageCredential(user)}
	f.settings = material.SettingsInput{RequestID: uuid.NewString(), Enabled: true, Provider: "github", ResourceID: resource, ConnectionID: &conn, Repository: "example/materials", BaseBranch: "main", RootPath: "engineering", ReviewerIDs: []string{user}, AutoStartPlan: &auto}
	if _, e = f.svc.Configure(ctx, f.access, f.settings); e != nil {
		t.Fatal(e)
	}
	return f
}
func (f *materialFixture) candidate(t *testing.T) material.Detail {
	t.Helper()
	ctx := context.Background()
	d, e := f.svc.Create(ctx, f.access, material.CreateInput{RequestID: uuid.NewString(), Title: "Support safe refunds"})
	if e != nil {
		t.Fatal(e)
	}
	d, e = f.svc.Revise(ctx, f.access, d.Material.ID, material.RevisionInput{RequestID: uuid.NewString(), ExpectedVersion: d.Material.Version, Kind: "requirement", Content: strings.Repeat("Precise refund requirements and failure semantics.\n", 4), Sources: []material.Source{{ResourceID: f.resource, Commit: strings.Repeat("a", 40), Path: "README.md"}}, AcceptanceCriteria: []string{"AC1: A rejected refund never changes the balance"}, Ready: true})
	if e != nil {
		t.Fatal(e)
	}
	return d
}
func (f *materialFixture) step(t *testing.T) {
	t.Helper()
	f.fx.Exec(t, `UPDATE git_material_job SET next_at=now() WHERE workspace_id=$1`, f.ws)
	if worked, e := f.svc.ProcessNext(context.Background()); e != nil || !worked {
		t.Fatalf("worker: %v %v", worked, e)
	}
}
func (f *materialFixture) detail(t *testing.T, id string) material.Detail {
	t.Helper()
	v, e := f.svc.Detail(context.Background(), f.access, id)
	if e != nil {
		t.Fatal(e)
	}
	return v
}
func (f *materialFixture) approve(t *testing.T, d material.Detail) material.Detail {
	t.Helper()
	r := d.Revisions[len(d.Revisions)-1]
	v, e := f.svc.Decide(context.Background(), f.access, d.Material.ID, material.DecisionInput{RequestID: uuid.NewString(), ExpectedVersion: d.Material.Version, RevisionID: r.ID, HeadSHA: *r.HeadSHA, Decision: "approve"})
	if e != nil {
		t.Fatal(e)
	}
	return v
}
func TestMaterialLostResponsesRecoverOneBaselineAndManualPlan(t *testing.T) {
	f := newMaterialFixture(t, false)
	f.git.lostPublish = true
	d := f.candidate(t)
	f.step(t)
	if v := f.detail(t, d.Material.ID); v.Material.Status != "blocked" || v.Baseline != nil {
		t.Fatal(v)
	}
	f.step(t)
	d = f.detail(t, d.Material.ID)
	if len(f.git.reviews) != 1 || d.Material.Status != "in_review" {
		t.Fatal(d)
	}
	d = f.approve(t, d)
	f.git.lostMerge = true
	f.step(t)
	if f.detail(t, d.Material.ID).Baseline != nil {
		t.Fatal("uncertain merge created baseline")
	}
	f.step(t)
	d = f.detail(t, d.Material.ID)
	if d.Baseline == nil || f.git.merge != 1 {
		t.Fatalf("baseline: %+v merges=%d", d, f.git.merge)
	}
	f.step(t)
	d = f.detail(t, d.Material.ID)
	if d.Plan == nil || d.Plan.Status != "waiting_manual" || d.Plan.IssueID == nil {
		t.Fatal(d)
	}
	if err := checkMaterialExecution(context.Background(), f.svc.Queries, util.MustParseUUID(*d.Plan.IssueID)); materialCode(err) != "material_plan_waiting_manual" {
		t.Fatal(err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1`, *d.Plan.IssueID) != 0 {
		t.Fatal("disabled plan dispatched")
	}
	enabled := true
	f.settings.RequestID = uuid.NewString()
	f.settings.ExpectedVersion = 1
	f.settings.AutoStartPlan = &enabled
	if _, e := f.svc.Configure(context.Background(), f.access, f.settings); e != nil {
		t.Fatal(e)
	}
	if d = f.detail(t, d.Material.ID); d.Plan.Status != "waiting_manual" {
		t.Fatal("re-enable drained old todo")
	}
}
func TestMaterialCurrentVersionAndNativeGates(t *testing.T) {
	for _, condition := range []string{"head", "extra files", "native checks", "expired signer", "unapproved external merge", "changed bytes"} {
		t.Run(condition, func(t *testing.T) {
			f := newMaterialFixture(t, false)
			d := f.candidate(t)
			f.step(t)
			d = f.detail(t, d.Material.ID)
			if condition != "unapproved external merge" {
				if condition == "expired signer" {
					f.access.Credential.Principal.ExpiresAt = time.Now().Add(time.Millisecond * 30)
				}
				d = f.approve(t, d)
			}
			switch condition {
			case "head":
				f.git.changeHead = true
			case "extra files":
				f.git.extra = true
			case "native checks":
				f.git.gates = false
			case "expired signer":
				time.Sleep(35 * time.Millisecond)
			case "changed bytes":
				f.git.badContent = true
			case "unapproved external merge":
				r := f.git.reviews[1]
				r.State = "merged"
				r.MergeSHA = strings.Repeat("b", 40)
				f.git.reviews[1] = r
				_, e := f.svc.Command(context.Background(), f.access, d.Material.ID, "retry", material.CommandInput{RequestID: uuid.NewString(), ExpectedVersion: d.Material.Version})
				if e != nil {
					t.Fatal(e)
				}
			}
			f.step(t)
			f.access.Credential = stageCredential(f.user)
			d = f.detail(t, d.Material.ID)
			if d.Baseline != nil || f.git.merge != 0 {
				t.Fatalf("unsafe merge: %+v %d", d, f.git.merge)
			}
		})
	}
}
func TestMaterialIdempotencyRevisionAndAuthority(t *testing.T) {
	f := newMaterialFixture(t, false)
	ctx := context.Background()
	in := material.CreateInput{RequestID: uuid.NewString(), Title: "Stable identity"}
	d, e := f.svc.Create(ctx, f.access, in)
	if e != nil {
		t.Fatal(e)
	}
	again, e := f.svc.Create(ctx, f.access, in)
	if e != nil || d.Material.ID != again.Material.ID {
		t.Fatal(e)
	}
	in.Title = "Changed body"
	if _, e = f.svc.Create(ctx, f.access, in); materialCode(e) != "material_idempotency_conflict" {
		t.Fatal(e)
	}
	machine := f.access
	machine.Credential.Principal.Kind = auth.CredentialCloud
	if _, e = f.svc.Configure(ctx, machine, f.settings); materialCode(e) != "material_permission_denied" {
		t.Fatal(e)
	}
	other := f.access
	other.ProjectID = util.MustParseUUID(uuid.NewString())
	if _, e = f.svc.Detail(ctx, other, d.Material.ID); e == nil {
		t.Fatal("cross project read")
	}
	f.fx.Exec(t, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, f.ws, f.user)
	if _, e = f.svc.Detail(ctx, f.access, d.Material.ID); e == nil {
		t.Fatal("revoked member read")
	}
}

func TestMaterialPlanBaselineChecksFrozenIdentity(t *testing.T) {
	revisionID := util.MustParseUUID(uuid.NewString())
	id := util.UUIDToString(revisionID)
	content := "approved requirement"
	digest := material.Digest(content)
	base := material.Baseline{ID: id, RevisionID: id, ContentSHA256: digest, Commit: strings.Repeat("a", 40), Path: "requirements/one/prd.md"}
	row := db.GitMaterial{ID: util.MustParseUUID(uuid.NewString()), Path: base.Path, CurrentRevisionID: revisionID, Baseline: materialJSON(base)}
	payload := material.Revision{ID: id, Kind: "requirement", Content: content, ContentSHA256: digest}
	revision := db.GitMaterialRevision{ID: revisionID, MaterialID: row.ID, Kind: "requirement", PublicationStatus: "merged", Payload: materialJSON(payload)}
	if _, _, err := materialPlanBaseline(row, revision); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"baseline_id", "commit", "path", "payload_id", "kind", "payload_digest", "current_revision", "publication"} {
		t.Run(field, func(t *testing.T) {
			receipt, frozen, materialRow, revisionRow := base, payload, row, revision
			switch field {
			case "baseline_id":
				receipt.ID = uuid.NewString()
			case "commit":
				receipt.Commit = "main"
			case "path":
				receipt.Path = "different/prd.md"
			case "payload_id":
				frozen.ID = uuid.NewString()
			case "kind":
				frozen.Kind = "plan"
			case "payload_digest":
				frozen.ContentSHA256 = material.Digest("other")
			case "current_revision":
				materialRow.CurrentRevisionID = util.MustParseUUID(uuid.NewString())
			case "publication":
				revisionRow.PublicationStatus = "in_review"
			}
			materialRow.Baseline, revisionRow.Payload = materialJSON(receipt), materialJSON(frozen)
			if _, _, err := materialPlanBaseline(materialRow, revisionRow); materialCode(err) != "material_baseline_changed" {
				t.Fatalf("accepted mismatched %s: %v", field, err)
			}
		})
	}
}

func TestMaterialBaselineTargetAllowsPolicyNotRepositoryChanges(t *testing.T) {
	connection := uuid.NewString()
	binding := materialBinding{Settings: material.Settings{Provider: "gitlab", Repository: "team/repo", ResourceID: uuid.NewString(), ConnectionID: &connection, RootPath: "engineering", BaseBranch: "main"}, ResourceDigest: "resource-digest", ConnectionRevision: 2}
	revision := db.GitMaterialRevision{Settings: materialJSON(binding)}
	binding.AutoStartPlan = true
	binding.Version++
	if err := checkMaterialBaselineTarget(revision, binding); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"resource", "branch", "connection", "trust"} {
		t.Run(field, func(t *testing.T) {
			changed := binding
			switch field {
			case "resource":
				changed.ResourceDigest += "changed"
			case "branch":
				changed.BaseBranch = "next"
			case "connection":
				changed.ConnectionID = materialPtr(uuid.NewString())
			case "trust":
				changed.ConnectionRevision++
			}
			if err := checkMaterialBaselineTarget(revision, changed); materialCode(err) != "material_configuration_changed" {
				t.Fatalf("accepted changed target %s: %v", field, err)
			}
		})
	}
}

func TestMaterialFinalMergeReceiptRechecksPaths(t *testing.T) {
	f := newMaterialFixture(t, false)
	detail := f.candidate(t)
	f.step(t)
	f.approve(t, f.detail(t, detail.Material.ID))
	f.git.extraAfterMerge = true
	f.step(t)
	detail = f.detail(t, detail.Material.ID)
	if detail.Baseline != nil || detail.Material.Blocker == nil || *detail.Material.Blocker != "material_unapproved_files" {
		t.Fatalf("unexpected merge receipt accepted: %+v", detail.Material)
	}
}

func TestMaterialPriorExternalMergeIsNotAnUncertainMergeAttempt(t *testing.T) {
	f := newMaterialFixture(t, false)
	detail := f.candidate(t)
	f.step(t)
	review := f.git.reviews[1]
	review.State, review.MergeSHA, review.MergeActor = "merged", strings.Repeat("b", 40), "external"
	f.git.reviews[1] = review
	f.git.files[review.MergeSHA+":"+detail.Material.Path] = detail.Revisions[0].Content
	f.approve(t, f.detail(t, detail.Material.ID))
	f.step(t)
	detail = f.detail(t, detail.Material.ID)
	if detail.Baseline != nil || detail.Material.Blocker == nil || *detail.Material.Blocker != "material_merge_not_authorized" {
		t.Fatalf("preexisting merge counted as a system merge: %+v", detail.Material)
	}
}

func TestMaterialManualRunConsumesExactTaskAndLiveAuthority(t *testing.T) {
	f := newMaterialFixture(t, false)
	ctx := context.Background()
	agentID := f.fx.Agent(t, "material planner", f.fx.Runtime(t, "material runtime"))
	f.settings.RequestID, f.settings.ExpectedVersion, f.settings.PlanAgentID, f.settings.AutoStartPlan = uuid.NewString(), 1, &agentID, nil
	setting, err := f.svc.Configure(ctx, f.access, f.settings)
	if err != nil || setting.Settings.AutoStartPlan {
		t.Fatalf("omitted switch lost explicit false: %+v %v", setting, err)
	}
	detail := f.candidate(t)
	id := detail.Material.ID
	f.step(t)
	f.approve(t, f.detail(t, id))
	f.step(t)
	f.step(t)
	detail = f.detail(t, id)
	if detail.Plan == nil || detail.Plan.Status != "waiting_manual" {
		t.Fatal(detail.Plan)
	}
	start := material.CommandInput{RequestID: uuid.NewString(), ExpectedVersion: detail.Material.Version}
	if _, err = f.svc.Command(ctx, f.access, id, "start-plan", start); err != nil {
		t.Fatal(err)
	}
	f.git.badContent = true
	f.step(t)
	detail = f.detail(t, id)
	if detail.Material.Blocker == nil || *detail.Material.Blocker != "material_baseline_unreadable" {
		t.Fatal(detail.Material)
	}
	verified := detail.Baseline.VerifiedAt
	f.git.badContent = false
	if _, err = f.svc.Command(ctx, f.access, id, "retry", material.CommandInput{RequestID: uuid.NewString(), ExpectedVersion: detail.Material.Version}); err != nil {
		t.Fatal(err)
	}
	f.step(t)
	if _, err = f.svc.Command(ctx, f.access, id, "start-plan", start); err != nil {
		t.Fatal(err)
	}
	detail = f.detail(t, id)
	if detail.Plan.Status != "queued" || !detail.Baseline.VerifiedAt.Equal(verified) || f.fx.Count(t, `SELECT COUNT(*) FROM agent_task_queue WHERE issue_id=$1`, *detail.Plan.IssueID) != 1 {
		t.Fatalf("retry did not preserve baseline and dispatch one run: %+v", detail.Plan)
	}
	plan, err := f.svc.Queries.GetMaterialPlan(ctx, util.MustParseUUID(id))
	if err != nil {
		t.Fatal(err)
	}
	run := context.WithValue(ctx, materialExecutionKey{}, materialExecutionPermit{issueID: plan.IssueID, taskID: plan.TaskID})
	check := func(want string) {
		t.Helper()
		err := checkMaterialExecution(run, f.svc.Queries, plan.IssueID)
		if want == "" && err != nil || want != "" && materialCode(err) != want {
			t.Fatalf("consumption: got %v, want %q", err, want)
		}
	}
	check("")
	f.fx.Exec(t, `UPDATE member SET role='member' WHERE workspace_id=$1 AND user_id=$2`, f.ws, f.user)
	check("material_configuration_authority_revoked")
	f.fx.Exec(t, `UPDATE member SET role='owner' WHERE workspace_id=$1 AND user_id=$2`, f.ws, f.user)
	f.fx.Exec(t, `UPDATE agent_task_queue SET handoff_note='not the approved input' WHERE id=$1`, plan.TaskID)
	check("material_baseline_changed")
	f.settings.RequestID, f.settings.ExpectedVersion, f.settings.PlanAgentID = uuid.NewString(), 2, nil
	if _, err = f.svc.Configure(ctx, f.access, f.settings); err != nil {
		t.Fatal(err)
	}
	check("material_plan_context_changed")
}

func TestMaterialNewRequirementSupersedesPlanUntilNewBaseline(t *testing.T) {
	f := newMaterialFixture(t, false)
	ctx := context.Background()
	detail := f.candidate(t)
	f.step(t)
	f.approve(t, f.detail(t, detail.Material.ID))
	f.step(t)
	f.step(t)
	detail = f.detail(t, detail.Material.ID)
	oldPlan := *detail.Plan.IssueID
	baseID := detail.Baseline.ID
	requirement := material.RevisionInput{RequestID: uuid.NewString(), ExpectedVersion: detail.Material.Version, Kind: "requirement", Content: strings.Repeat("new requirement ", 5), Sources: detail.Revisions[0].Sources, AcceptanceCriteria: detail.Revisions[0].AcceptanceCriteria, Ready: true}
	detail, err := f.svc.Revise(ctx, f.access, detail.Material.ID, requirement)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Capabilities.CanStartPlan {
		t.Fatal("obsolete plan still advertised as startable")
	}
	if _, err = f.svc.Revise(ctx, f.access, detail.Material.ID, material.RevisionInput{RequestID: uuid.NewString(), ExpectedVersion: detail.Material.Version, Kind: "plan", SourceBaselineID: &baseID}); materialCode(err) != "material_baseline_changed" {
		t.Fatalf("stale plan replaced newer requirement: %v", err)
	}
	if _, err = f.svc.Command(ctx, f.access, detail.Material.ID, "start-plan", material.CommandInput{RequestID: uuid.NewString(), ExpectedVersion: detail.Material.Version}); materialCode(err) != "material_baseline_changed" {
		t.Fatalf("stale plan start: %v", err)
	}
	f.step(t)
	f.approve(t, f.detail(t, detail.Material.ID))
	f.step(t)
	f.step(t)
	detail = f.detail(t, detail.Material.ID)
	if detail.Baseline.ID == baseID || detail.Plan.BaselineID != detail.Baseline.ID || *detail.Plan.IssueID == oldPlan || detail.Plan.Status != "waiting_manual" {
		t.Fatalf("new baseline did not get an independent manual plan: %+v", detail.Plan)
	}
}

func (f *materialFixture) queuedPlan(t *testing.T) db.GitMaterialPlan {
	t.Helper()
	agentID := f.fx.Agent(t, "planner", f.fx.Runtime(t, "planner runtime"))
	f.settings.RequestID, f.settings.ExpectedVersion, f.settings.PlanAgentID = uuid.NewString(), 1, &agentID
	if _, err := f.svc.Configure(t.Context(), f.access, f.settings); err != nil {
		t.Fatal(err)
	}
	detail := f.candidate(t)
	f.step(t)
	f.approve(t, f.detail(t, detail.Material.ID))
	f.step(t)
	f.step(t)
	plan, err := f.svc.Queries.GetMaterialPlan(t.Context(), util.MustParseUUID(detail.Material.ID))
	if err != nil || !plan.TaskID.Valid || plan.Status != "queued" {
		t.Fatalf("automatic plan was not queued: %+v %v", plan, err)
	}
	return plan
}

func TestMaterialClaimsRecheckCurrentAuthority(t *testing.T) {
	for _, entry := range []string{"claim", "reclaim", "batch_reclaim", "finalize"} {
		t.Run(entry, func(t *testing.T) {
			f := newMaterialFixture(t, true)
			plan := f.queuedPlan(t)
			task, err := f.svc.Queries.GetAgentTask(t.Context(), plan.TaskID)
			if err != nil {
				t.Fatal(err)
			}
			if entry != "claim" {
				claimed, err := f.svc.Tasks.ClaimTask(t.Context(), plan.AgentID)
				if err != nil || claimed == nil {
					t.Fatalf("positive claim: %v %v", claimed, err)
				}
				task = *claimed
				if entry != "finalize" {
					f.fx.Exec(t, `UPDATE agent_task_queue SET dispatched_at=now()-interval '10 minutes',prepare_lease_expires_at=now()-interval '1 minute' WHERE id=$1`, plan.TaskID)
				}
			}
			f.fx.Exec(t, `UPDATE member SET role='member' WHERE workspace_id=$1 AND user_id=$2`, f.ws, f.user)
			switch entry {
			case "claim":
				_, err = f.svc.Tasks.ClaimTask(t.Context(), plan.AgentID)
			case "reclaim":
				_, err = f.svc.Tasks.ClaimTaskForRuntime(t.Context(), task.RuntimeID)
			case "batch_reclaim":
				_, err = f.svc.Tasks.ClaimTasksForRuntimes(t.Context(), []pgtype.UUID{task.RuntimeID}, 5)
			case "finalize":
				_, err = f.svc.Tasks.FinalizeTaskClaim(t.Context(), task, db.CreateTaskTokenParams{TokenHash: uuid.NewString(), TaskID: task.ID, AgentID: task.AgentID, WorkspaceID: f.access.WorkspaceID, UserID: util.MustParseUUID(f.user), ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true}}, nil, false, nil, TaskClaimBudgetProtocolNone)
			}
			if entry == "finalize" {
				if materialCode(err) != "material_configuration_authority_revoked" {
					t.Fatalf("revoked plan finalized: %v", err)
				}
			} else if err != nil || f.fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE id=$1 AND status='cancelled'`, plan.TaskID) != 1 {
				t.Fatalf("denied plan was not settled at %s: %v", entry, err)
			}
			if entry == "finalize" && f.fx.Count(t, `SELECT count(*) FROM task_token WHERE task_id=$1`, plan.TaskID) != 0 {
				t.Fatal("denied claim minted token")
			}
		})
	}
}

func TestMaterialAutoDisableCancelsOnlyUnstartedPlan(t *testing.T) {
	for _, started := range []bool{false, true} {
		t.Run(map[bool]string{false: "queued", true: "running"}[started], func(t *testing.T) {
			f := newMaterialFixture(t, true)
			plan := f.queuedPlan(t)
			if started {
				if claimed, err := f.svc.Tasks.ClaimTask(t.Context(), plan.AgentID); err != nil || claimed == nil {
					t.Fatalf("running plan claim: %v %v", claimed, err)
				}
				if _, err := f.svc.Tasks.StartTask(t.Context(), plan.TaskID); err != nil {
					t.Fatal(err)
				}
			}
			disabled := false
			f.settings.RequestID, f.settings.ExpectedVersion, f.settings.AutoStartPlan = uuid.NewString(), 2, &disabled
			if _, err := f.svc.Configure(t.Context(), f.access, f.settings); err != nil {
				t.Fatal(err)
			}
			current, err := f.svc.Queries.GetMaterialPlan(t.Context(), plan.MaterialID)
			if err != nil {
				t.Fatal(err)
			}
			task, err := f.svc.Queries.GetAgentTask(t.Context(), plan.TaskID)
			if err != nil {
				t.Fatal(err)
			}
			if started {
				if task.Status != "running" || current.TaskID != plan.TaskID {
					t.Fatal("switch stopped running work")
				}
				return
			}
			if task.Status != "cancelled" || current.Status != "waiting_manual" || current.Requested || current.TaskID.Valid {
				t.Fatalf("queued plan was not held: %+v task=%s", current, task.Status)
			}
			enabled := true
			f.settings.RequestID, f.settings.ExpectedVersion, f.settings.AutoStartPlan = uuid.NewString(), 3, &enabled
			if _, err := f.svc.Configure(t.Context(), f.access, f.settings); err != nil {
				t.Fatal(err)
			}
			current, err = f.svc.Queries.GetMaterialPlan(t.Context(), plan.MaterialID)
			if err != nil || current.Requested || current.IssueID != plan.IssueID {
				t.Fatal("re-enabling drained old plan", err)
			}
			if task, err := f.svc.Tasks.ClaimTask(t.Context(), plan.AgentID); err != nil || task != nil {
				t.Fatalf("held task claimed: %v %v", task, err)
			}
		})
	}
}

func TestMaterialTerminalPlanRequiresHumanRestart(t *testing.T) {
	for _, state := range []string{"queued", "running", "failed", "cancelled", "completed"} {
		t.Run(state, func(t *testing.T) {
			f := newMaterialFixture(t, true)
			plan := f.queuedPlan(t)
			id := util.UUIDToString(plan.MaterialID)
			f.fx.Exec(t, `UPDATE agent_task_queue SET status=$2 WHERE id=$1`, plan.TaskID, state)
			terminal := state == "failed" || state == "cancelled" || state == "completed"
			detail := f.detail(t, id)
			if detail.Capabilities.CanStartPlan != terminal {
				t.Fatalf("restart capability for %s = %v", state, detail.Capabilities.CanStartPlan)
			}
			if _, err := f.svc.Command(t.Context(), f.access, id, "retry", material.CommandInput{RequestID: uuid.NewString(), ExpectedVersion: detail.Material.Version}); err != nil {
				t.Fatal(err)
			}
			f.step(t)
			if f.fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1`, plan.IssueID) != 1 {
				t.Fatal("ordinary recovery reran plan")
			}
			detail = f.detail(t, id)
			start := material.CommandInput{RequestID: uuid.NewString(), ExpectedVersion: detail.Material.Version}
			_, err := f.svc.Command(t.Context(), f.access, id, "start-plan", start)
			if !terminal {
				if materialCode(err) != "material_plan_already_started" {
					t.Fatalf("active run restart: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err = f.svc.Command(t.Context(), f.access, id, "start-plan", start); err != nil {
				t.Fatal("lost-response restart", err)
			}
			f.step(t)
			current, err := f.svc.Queries.GetMaterialPlan(t.Context(), plan.MaterialID)
			if err != nil || current.IssueID != plan.IssueID || current.TaskID == plan.TaskID || current.Status != "queued" {
				t.Fatalf("restart binding: %+v %v", current, err)
			}
			if f.fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1`, plan.IssueID) != 2 || f.fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE id=$1 AND status=$2`, plan.TaskID, state) != 1 {
				t.Fatal("restart lost history or duplicated task")
			}
			detail = f.detail(t, id)
			if _, err = f.svc.Command(t.Context(), f.access, id, "start-plan", material.CommandInput{RequestID: uuid.NewString(), ExpectedVersion: detail.Material.Version}); materialCode(err) != "material_plan_already_started" {
				t.Fatalf("double click reran active task: %v", err)
			}
		})
	}
}

func TestMaterialDeniedClaimDoesNotStarveOrdinaryWork(t *testing.T) {
	for _, reclaim := range []bool{false, true} {
		t.Run(map[bool]string{false: "queued", true: "batch_reclaim"}[reclaim], func(t *testing.T) {
			f := newMaterialFixture(t, true)
			plan := f.queuedPlan(t)
			task, err := f.svc.Queries.GetAgentTask(t.Context(), plan.TaskID)
			if err != nil {
				t.Fatal(err)
			}
			issue := f.fx.Issue(t, "unrelated ordinary work", testutil.Cols{"project_id": f.project})
			ordinary := f.fx.Task(t, util.UUIDToString(plan.AgentID), testutil.Cols{"issue_id": issue, "runtime_id": task.RuntimeID, "priority": -1})
			f.fx.Exec(t, `UPDATE member SET role='member' WHERE workspace_id=$1 AND user_id=$2`, f.ws, f.user)
			if reclaim {
				f.fx.Exec(t, `UPDATE agent_task_queue SET status='dispatched',dispatched_at=now()-interval '10 minutes',prepare_lease_expires_at=now()-interval '1 minute' WHERE id=$1 OR id=$2`, plan.TaskID, ordinary)
				claimed, err := f.svc.Tasks.ClaimTasksForRuntimes(t.Context(), []pgtype.UUID{task.RuntimeID}, 5)
				if err != nil || len(claimed) != 1 || util.UUIDToString(claimed[0].ID) != ordinary {
					t.Fatalf("mixed reclaim lost healthy work: %+v %v", claimed, err)
				}
			} else {
				if claimed, err := f.svc.Tasks.ClaimTask(t.Context(), plan.AgentID); err != nil || claimed != nil {
					t.Fatalf("denied claim: %v %v", claimed, err)
				}
				claimed, err := f.svc.Tasks.ClaimTask(t.Context(), plan.AgentID)
				if err != nil || claimed == nil || util.UUIDToString(claimed.ID) != ordinary {
					t.Fatalf("ordinary task starved: %v %v", claimed, err)
				}
			}
		})
	}
}

func TestMaterialClaimContentionDoesNotCancel(t *testing.T) {
	for _, table := range []string{"project", "member", "agent"} {
		t.Run(table, func(t *testing.T) {
			f := newMaterialFixture(t, true)
			plan := f.queuedPlan(t)
			task, err := f.svc.Tasks.ClaimTask(t.Context(), plan.AgentID)
			if err != nil || task == nil {
				t.Fatalf("claim: %v %v", task, err)
			}
			blocker, err := f.pool.Begin(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			defer blocker.Rollback(t.Context())
			query, id := `SELECT id FROM project WHERE id=$1 FOR UPDATE`, plan.ProjectID
			if table == "member" {
				query, id = `SELECT id FROM member WHERE id=$1 FOR UPDATE`, plan.MemberID
			}
			if table == "agent" {
				query, id = `SELECT id FROM agent WHERE id=$1 FOR UPDATE`, plan.AgentID
			}
			if _, err = blocker.Exec(t.Context(), query, id); err != nil {
				t.Fatal(err)
			}
			tx, err := f.pool.Begin(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(t.Context())
			allowed, err := settleMaterialClaim(t.Context(), f.svc.Queries.WithTx(tx), *task)
			if allowed || err == nil || materialCode(err) != "material_operation_unavailable" {
				t.Fatalf("contention misclassified: %v %v", allowed, err)
			}
			if f.fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE id=$1 AND status='dispatched'`, task.ID) != 1 {
				t.Fatal("contention cancelled task")
			}
		})
	}
}
