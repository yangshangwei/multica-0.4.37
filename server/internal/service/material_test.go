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
	"github.com/multica-ai/multica/server/internal/materialrepo"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type materialFakeGit struct {
	mu                     sync.Mutex
	reviews                map[int64]materialgit.Review
	files                  map[string]string
	branches               map[string]int64
	notes                  map[int64]protocol.MaterialAuditNote
	publish, merge         int
	lostPublish, lostMerge bool
	gates                  bool
	changeHead             bool
	extra                  bool
	badContent             bool
	extraAfterMerge        bool
}

func newMaterialFake() *materialFakeGit {
	return &materialFakeGit{reviews: map[int64]materialgit.Review{}, files: map[string]string{}, branches: map[string]int64{}, notes: map[int64]protocol.MaterialAuditNote{}, gates: true}
}
func (p *materialFakeGit) Publish(_ context.Context, in materialgit.PublishRequest) (materialgit.Review, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.publish++
	n := p.branches[in.Branch]
	if n == 0 {
		n = int64(len(p.reviews) + 1)
		r := materialgit.Review{Number: n, URL: "https://github.com/example/materials/pull/1", HeadSHA: material.Digest(in.Key)[:40], BaseBranch: in.BaseBranch, Branch: in.Branch, State: "open", GatesPassed: p.gates}
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
func (p *materialFakeGit) SyncDecision(n int64, note protocol.MaterialAuditNote) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	note.Actor = "fixture-runtime"
	p.notes[n] = note
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
	r.MergeSHA = material.Digest(r.HeadSHA + "merge")[:40]
	r.MergeActor = "integration"
	mergedAt := time.Now()
	r.MergedAt = &mergedAt
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
	rules                       materialrepo.Snapshot
	pool                        *pgxpool.Pool
	fx                          *testutil.Fixture
	svc                         *MaterialService
	git                         *materialFakeGit
	access                      MaterialAccess
	settings                    material.SettingsInput
	ws, user, project, resource string
	runtime                     string
	active                      *protocol.MaterialExecution
	claim                       *protocol.MaterialClaimBinding
	task                        db.AgentTaskQueue
	lastErr                     error
}

func newMaterialFixture(t *testing.T, auto bool) *materialFixture {
	return newMaterialRepositoryFixture(t, auto, true)
}
func newMaterialRepositoryFixture(t *testing.T, auto, initialize bool) *materialFixture {
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
	runtime := fx.Runtime(t, "material runtime", testutil.Cols{"daemon_id": "material-" + suffix})
	q := db.New(pool)
	if e = q.SeedIssueStatusEntries(ctx, util.MustParseUUID(ws)); e != nil {
		t.Fatal(e)
	}
	for _, table := range []string{"issue_status", "issue", "project_material_settings", "git_material", "git_material_revision", "git_material_decision", "git_material_comment", "git_material_request", "git_material_job", "git_material_plan", "git_material_operation", "material_repository_decision", "material_repository_revision", "project_material_repository"} {
		fx.Cleanup(t, `DELETE FROM `+table+` WHERE workspace_id=$1`, ws)
	}
	fx.Cleanup(t, `DELETE FROM task_token WHERE workspace_id=$1`, ws)
	fx.Cleanup(t, `DELETE FROM agent_task_queue WHERE agent_id IN (SELECT id FROM agent WHERE workspace_id=$1)`, ws)
	fx.Cleanup(t, `DELETE FROM agent WHERE workspace_id=$1 AND kind='system'`, ws)
	f := &materialFixture{pool: pool, fx: fx, ws: ws, user: user, project: project, resource: resource, git: newMaterialFake()}
	tasks := NewTaskService(q, pool, nil, events.New(), nil)
	issues := NewIssueService(q, pool, nil, nil, tasks)
	f.svc = NewMaterialService(q, pool, issues, tasks)
	f.runtime = runtime
	f.access = MaterialAccess{WorkspaceID: util.MustParseUUID(ws), ProjectID: util.MustParseUUID(project), Credential: stageCredential(user)}
	f.settings = material.SettingsInput{RequestID: uuid.NewString(), Enabled: true, Provider: "github", ResourceID: resource, ExecutionRuntimeID: &runtime, Repository: "example/materials", BaseBranch: "main", RootPath: "engineering", ReviewerIDs: []string{user}, AutoStartPlan: &auto}
	if _, e = f.svc.Configure(ctx, f.access, f.settings); e != nil {
		t.Fatal(e)
	}
	if initialize {
		f.initializeRepository(t)
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

// This service fixture exercises queue admission and receipt validation. The
// executable/daemon integration is a separate test layer; this fake never acts
// as a product server provider or writes final material state directly.
func (f *materialFixture) step(t *testing.T) {
	t.Helper()
	f.lastErr = nil
	if f.active != nil {
		f.lastErr = f.observe(t)
		if f.lastErr == nil {
			f.finish(t)
		}
		return
	}
	f.fx.Exec(t, `UPDATE git_material_job SET next_at=now() WHERE workspace_id=$1`, f.ws)
	worked, e := f.svc.ProcessNext(t.Context())
	if e != nil {
		f.lastErr = e
		return
	}
	if !worked {
		return
	}
	var id pgtype.UUID
	e = f.pool.QueryRow(t.Context(), `SELECT o.task_id FROM git_material_operation o JOIN agent_task_queue t ON t.id=o.task_id WHERE o.workspace_id=$1 AND o.action!='verify_baseline' AND t.status='queued' ORDER BY o.created_at LIMIT 1`, f.ws).Scan(&id)
	if e != nil {
		return
	}
	f.admit(t, id)
	d := *f.active
	switch d.Action {
	case "publish":
		files := []materialgit.File{}
		for _, v := range d.Files {
			files = append(files, materialgit.File{Path: v.Path, Content: v.Content})
		}
		_, e = f.git.Publish(t.Context(), materialgit.PublishRequest{Key: d.OperationKey, Branch: d.SourceBranch, BaseBranch: d.BaseBranch, Files: files})
	case "review_sync":
		e = f.git.SyncDecision(d.ReviewNumber, *d.AuditNote)
	case "merge":
		e = f.observe(t)
		if e == nil {
			_, e = f.svc.OperationPermit(t.Context(), f.task, d.OperationID, protocol.MaterialPermitInput{RequestID: uuid.NewString(), ExpectedDigest: d.ExpectedDigest})
		}
		if e == nil {
			_, e = f.git.Merge(t.Context(), materialgit.MergeExpectation{Number: d.ReviewNumber, HeadSHA: d.ApprovedHead})
		}
	}
	if e != nil {
		f.lastErr = e
		return
	}
	f.lastErr = f.observe(t)
	if f.lastErr != nil {
		return
	}
	f.finish(t)
	if d.Action == "review_sync" {
		f.step(t)
	}
}
func (f *materialFixture) admit(t *testing.T, id pgtype.UUID) {
	t.Helper()
	original, e := f.svc.Queries.GetAgentTask(t.Context(), id)
	if e != nil {
		t.Fatal(e)
	}
	task, e := f.svc.Tasks.ClaimTask(t.Context(), original.AgentID)
	if e != nil || task == nil || task.ID != id {
		t.Fatalf("claim: %+v %v", task, e)
	}
	final, e := f.svc.Tasks.FinalizeTaskClaim(t.Context(), *task, db.CreateTaskTokenParams{TokenHash: uuid.NewString(), TaskID: id, AgentID: task.AgentID, WorkspaceID: f.access.WorkspaceID, UserID: util.MustParseUUID(f.user), ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true}}, nil, false, nil, TaskClaimBudgetProtocolNone)
	if e != nil || final.MaterialClaim == nil {
		t.Fatalf("claim finalization: %+v %v", final, e)
	}
	op, e := MaterialOperationForTask(t.Context(), f.svc.Queries, *task)
	if e != nil {
		t.Fatal(e)
	}
	f.active = &op.Execution
	f.claim = final.MaterialClaim
	f.task = *task
	if op.Execution.RepositoryExecution != nil && op.Execution.TargetKind != "repository_initialization" && op.Execution.Action != "verify_baseline" {
		observation := f.rulesObservation(t, "rules_verified")
		if e = f.svc.ObserveOperation(t.Context(), id, observation); e != nil {
			t.Fatal(e)
		}
	}
	if op.Execution.Action != "verify_baseline" {
		if _, e = f.svc.Tasks.StartTask(t.Context(), id); e != nil {
			t.Fatal(e)
		}
	}
}
func (f *materialFixture) observe(t *testing.T) error {
	t.Helper()
	d := *f.active
	o := protocol.MaterialObservation{ProtocolVersion: 1, OperationID: d.OperationID, TaskID: d.TaskID, RuntimeID: d.RuntimeID, ClaimGeneration: f.claim.ClaimGeneration, ExpectedDigest: d.ExpectedDigest, ObservationID: uuid.NewString(), HostChallenge: f.claim.HostChallenge, ObservedAt: time.Now(), Outcome: "observed", SourceComplete: true, RepositoryIdentity: d.Provider + ":" + d.InstanceURL + ":" + d.Repository}
	o.RulesSnapshot = materialJSON(f.rules)
	commit := d.BaselineCommit
	if d.Action != "verify_baseline" {
		n := d.ReviewNumber
		if n == 0 {
			n = f.git.branches[d.SourceBranch]
		}
		r, e := f.git.Observe(t.Context(), n)
		if e != nil {
			return e
		}
		if r.State == "merged" && f.git.extraAfterMerge {
			r.Changes = append(r.Changes, materialgit.Change{Path: "another.md", Mode: "100644", Status: "added"})
		}
		o.Review = &protocol.MaterialReview{Number: r.Number, URL: r.URL, SourceBranch: r.Branch, BaseBranch: r.BaseBranch, HeadSHA: r.HeadSHA, State: r.State, MergeSHA: r.MergeSHA, MergeActor: r.MergeActor, MergedAt: r.MergedAt, GatesPassed: r.GatesPassed}
		for _, c := range r.Changes {
			o.Review.Changes = append(o.Review.Changes, protocol.MaterialChange{Path: c.Path, Mode: c.Mode, Status: c.Status, PreviousPath: c.PreviousPath})
		}
		commit = r.HeadSHA
		if r.State == "merged" {
			commit = r.MergeSHA
		}
	}
	for _, v := range d.Files {
		body, e := f.git.ReadFile(t.Context(), commit, v.Path)
		if e != nil {
			return e
		}
		o.Files = append(o.Files, protocol.MaterialFile{Path: v.Path, Content: body, SHA256: material.Digest(body), Mode: "100644", Commit: commit})
	}
	if d.AuditNote != nil {
		note := f.git.notes[d.ReviewNumber]
		o.AuditNote = &note
	}
	return f.svc.ObserveOperation(t.Context(), f.task.ID, o)
}
func (f *materialFixture) finish(t *testing.T) {
	t.Helper()
	if _, e := f.svc.Tasks.CompleteTask(t.Context(), f.task.ID, []byte(`{}`), "", "", "", false, "", ""); e != nil {
		t.Fatal(e)
	}
	f.active = nil
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
	if v := f.detail(t, d.Material.ID); v.Baseline != nil || v.Material.Status == "in_review" {
		t.Fatal(v)
	}
	f.step(t)
	d = f.detail(t, d.Material.ID)
	if len(f.git.reviews) != 1 || f.git.publish != 1 || d.Material.Status != "in_review" {
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
func TestMaterialSchedulerLeaseDoesNotRepeatActiveAttempt(t *testing.T) {
	f := newMaterialFixture(t, false)
	d := f.candidate(t)
	if worked, err := f.svc.ProcessNext(t.Context()); err != nil || !worked {
		t.Fatalf("dispatch: %v %v", worked, err)
	}
	f.fx.Exec(t, `UPDATE git_material_job SET state='working',leased_until=now()-interval '3 minutes',next_at=now() WHERE workspace_id=$1`, f.ws)
	if worked, err := f.svc.ProcessNext(t.Context()); err != nil || !worked {
		t.Fatalf("lease recovery: %v %v", worked, err)
	}
	if n := f.fx.Count(t, `SELECT count(*) FROM git_material_operation WHERE material_id=$1`, d.Material.ID); n != 1 {
		t.Fatalf("expired scheduler lease duplicated operation: %d", n)
	}
	if n := f.fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE id IN (SELECT task_id FROM git_material_operation WHERE material_id=$1)`, d.Material.ID); n != 1 {
		t.Fatalf("expired scheduler lease duplicated runtime launch: %d", n)
	}
	if f.detail(t, d.Material.ID).Baseline != nil {
		t.Fatal("queue dispatch established a baseline")
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
	runtime := uuid.NewString()
	binding := materialBinding{Settings: material.Settings{Provider: "gitlab", Repository: "team/repo", ResourceID: uuid.NewString(), ExecutionRuntimeID: &runtime, RootPath: "engineering", BaseBranch: "main"}, ResourceDigest: "resource-digest"}
	revision := db.GitMaterialRevision{Settings: materialJSON(binding)}
	binding.AutoStartPlan = true
	binding.Version++
	if err := checkMaterialBaselineTarget(revision, binding); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"resource", "branch", "repository", "root"} {
		t.Run(field, func(t *testing.T) {
			changed := binding
			switch field {
			case "resource":
				changed.ResourceDigest += "changed"
			case "branch":
				changed.BaseBranch = "next"
			case "repository":
				changed.Repository = "other/repo"
			case "root":
				changed.RootPath = "other"
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
	if detail.Baseline != nil || materialCode(f.lastErr) != "material_unapproved_files" {
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
	if detail.Baseline != nil || materialCode(f.lastErr) != "material_merge_not_authorized" {
		t.Fatalf("preexisting merge counted as a system merge: %+v", detail.Material)
	}
}

func TestMaterialManualRunConsumesExactTaskAndLiveAuthority(t *testing.T) {
	f := newMaterialFixture(t, false)
	ctx := context.Background()
	runtimeID := f.runtime
	f.settings.RequestID, f.settings.ExpectedVersion, f.settings.ExecutionRuntimeID, f.settings.AutoStartPlan = uuid.NewString(), 1, &runtimeID, nil
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
	f.step(t)
	plan, err := f.svc.Queries.GetMaterialPlan(ctx, util.MustParseUUID(id))
	if err != nil {
		t.Fatal(err)
	}
	f.admit(t, plan.TaskID)
	f.git.badContent = true
	if err = f.observe(t); materialCode(err) != "material_content_changed" {
		t.Fatalf("bad baseline accepted: %v", err)
	}
	if _, err = f.svc.Tasks.StartTask(ctx, plan.TaskID); materialCode(err) != "material_baseline_unverified" {
		t.Fatalf("model started before baseline verification: %v", err)
	}
	verified := f.detail(t, id).Baseline.VerifiedAt
	f.git.badContent = false
	if err = f.observe(t); err != nil {
		t.Fatal(err)
	}
	f.active = nil
	if _, err = f.svc.Command(ctx, f.access, id, "start-plan", start); err != nil {
		t.Fatal(err)
	}
	detail = f.detail(t, id)
	if detail.Plan.Status != "queued" || !detail.Baseline.VerifiedAt.Equal(verified) || f.fx.Count(t, `SELECT COUNT(*) FROM agent_task_queue WHERE issue_id=$1`, *detail.Plan.IssueID) != 1 {
		t.Fatalf("verification did not preserve baseline and one run: %+v", detail.Plan)
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
	newRuntime := f.fx.Runtime(t, "replacement", testutil.Cols{"daemon_id": "replacement"})
	f.settings.RequestID, f.settings.ExpectedVersion, f.settings.ExecutionRuntimeID = uuid.NewString(), 2, &newRuntime
	if _, err = f.svc.Configure(ctx, f.access, f.settings); err != nil {
		t.Fatal(err)
	}
	check("material_repository_not_ready")
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
	runtimeID := f.runtime
	f.settings.RequestID, f.settings.ExpectedVersion, f.settings.ExecutionRuntimeID = uuid.NewString(), 1, &runtimeID
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
				f.admit(t, plan.TaskID)
				if err := f.observe(t); err != nil {
					t.Fatal(err)
				}
				if _, err := f.svc.Tasks.StartTask(t.Context(), plan.TaskID); err != nil {
					t.Fatal(err)
				}
				f.active = nil
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
	for _, table := range []string{"project", "member", "agent", "runtime"} {
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
			if table == "runtime" {
				query, id = `SELECT id FROM agent_runtime WHERE id=$1 FOR UPDATE`, util.MustParseUUID(f.runtime)
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

func TestMaterialAbsentPublishRecoveryAndReceiptReplay(t *testing.T) {
	f := newMaterialFixture(t, false)
	d := f.candidate(t)
	if _, err := f.svc.ProcessNext(t.Context()); err != nil {
		t.Fatal(err)
	}
	first, err := f.svc.Queries.CurrentMaterialOperation(t.Context(), util.MustParseUUID(d.Material.ID))
	if err != nil {
		t.Fatal(err)
	}
	f.admit(t, first.TaskID)
	// The runtime exited before publication. Ordinary completion proves no effect neither way.
	f.finish(t)
	retry := func() {
		t.Helper()
		current := f.detail(t, d.Material.ID)
		if _, err := f.svc.Command(t.Context(), f.access, d.Material.ID, "retry", material.CommandInput{RequestID: uuid.NewString(), ExpectedVersion: current.Material.Version}); err != nil {
			t.Fatal(err)
		}
		if _, err := f.svc.ProcessNext(t.Context()); err != nil {
			t.Fatal(err)
		}
	}
	retry()
	recovery, err := f.svc.Queries.CurrentMaterialOperation(t.Context(), util.MustParseUUID(d.Material.ID))
	if err != nil {
		t.Fatal(err)
	}
	f.admit(t, recovery.TaskID)
	if f.active.Action != "observe" || f.active.ReconcileAction != "publish" {
		t.Fatalf("retry launched mutation before reconciliation: %+v", f.active)
	}
	expected := *f.active
	observation := protocol.MaterialObservation{ProtocolVersion: 1, OperationID: expected.OperationID, TaskID: expected.TaskID, RuntimeID: expected.RuntimeID, ClaimGeneration: f.claim.ClaimGeneration, ExpectedDigest: expected.ExpectedDigest, ObservationID: uuid.NewString(), HostChallenge: f.claim.HostChallenge, ObservedAt: time.Now(), Outcome: "not_found", SourceComplete: true, RepositoryIdentity: expected.Provider + ":" + expected.InstanceURL + ":" + expected.Repository}
	observation.RulesSnapshot = materialJSON(f.rules)
	invalid := observation
	invalid.ClaimGeneration++
	if err := f.svc.ObserveOperation(t.Context(), f.task.ID, invalid); materialCode(err) != "material_observation_denied" {
		t.Fatalf("wrong generation accepted: %v", err)
	}
	if err := f.svc.ObserveOperation(t.Context(), f.task.ID, observation); err != nil {
		t.Fatal(err)
	}
	invalid = observation
	invalid.ObservationID = uuid.NewString()
	if err := f.svc.ObserveOperation(t.Context(), f.task.ID, invalid); materialCode(err) != "material_observation_state" {
		t.Fatalf("new receipt changed settled outcome: %v", err)
	}
	before := f.detail(t, d.Material.ID)
	if before.Baseline != nil || before.Operation.Status != "no_effect" {
		t.Fatalf("absence promoted material: %+v", before)
	}
	f.finish(t)
	if err := f.svc.ObserveOperation(t.Context(), recovery.TaskID, observation); err != nil {
		t.Fatalf("lost response exact replay rejected after completion: %v", err)
	}
	invalid = observation
	invalid.SourceComplete = false
	if err := f.svc.ObserveOperation(t.Context(), recovery.TaskID, invalid); materialCode(err) != "material_observation_conflict" {
		t.Fatalf("changed receipt identity accepted: %v", err)
	}
	retry()
	next, err := f.svc.Queries.CurrentMaterialOperation(t.Context(), util.MustParseUUID(d.Material.ID))
	if err != nil {
		t.Fatal(err)
	}
	descriptor, err := materialParse[protocol.MaterialExecution](next.Descriptor)
	if err != nil {
		t.Fatal(err)
	}
	if descriptor.Action != "publish" || descriptor.SourceBranch != expected.SourceBranch || descriptor.OperationKey != expected.OperationKey {
		t.Fatal("recovery lost original remote intent")
	}
	if f.fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE id=ANY($1::uuid[])`, []string{util.UUIDToString(first.TaskID), util.UUIDToString(recovery.TaskID), util.UUIDToString(next.TaskID)}) != 3 {
		t.Fatal("retry discarded attempt history")
	}
}

func TestMaterialRuntimeDeletionRetainsAuditHistory(t *testing.T) {
	f := newMaterialFixture(t, false)
	d := f.candidate(t)
	f.step(t)
	d = f.approve(t, f.detail(t, d.Material.ID))
	f.step(t)
	f.step(t)
	d = f.detail(t, d.Material.ID)
	if d.Baseline == nil || d.Plan == nil || d.Plan.IssueID == nil {
		t.Fatalf("missing retained baseline/plan setup: %+v", d)
	}
	op, err := f.svc.Queries.CurrentMaterialOperation(t.Context(), util.MustParseUUID(d.Material.ID))
	if err != nil {
		t.Fatal(err)
	}
	beforeTasks := f.fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE agent_id=$1`, op.AgentID)
	beforeDecisions := f.fx.Count(t, `SELECT count(*) FROM git_material_decision WHERE material_id=$1`, d.Material.ID)
	tx, err := f.pool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(t.Context())
	q := f.svc.Queries.WithTx(tx)
	if _, err = TeardownRuntime(t.Context(), q, util.MustParseUUID(f.runtime), RuntimeTeardownOptions{CancelNonTerminalTasks: true}); err != nil {
		t.Fatal(err)
	}
	if err = q.DeleteAgentRuntime(t.Context(), util.MustParseUUID(f.runtime)); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(t.Context()); err != nil {
		t.Fatal(err)
	}
	after := f.detail(t, d.Material.ID)
	if after.Baseline == nil || after.Baseline.ID != d.Baseline.ID || after.Plan == nil || *after.Plan.IssueID != *d.Plan.IssueID {
		t.Fatal("runtime deletion removed baseline/plan history")
	}
	if f.fx.Count(t, `SELECT count(*) FROM agent WHERE id=$1 AND runtime_id IS NULL`, op.AgentID) != 1 || f.fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE agent_id=$1 AND runtime_id IS NULL`, op.AgentID) != beforeTasks || f.fx.Count(t, `SELECT count(*) FROM git_material_decision WHERE material_id=$1`, d.Material.ID) != beforeDecisions {
		t.Fatal("runtime deletion erased carrier, tasks or decisions")
	}
	retained, err := f.svc.Queries.GetMaterialOperation(t.Context(), op.ID)
	if err != nil || retained.RuntimeID != op.RuntimeID || string(retained.Descriptor) != string(op.Descriptor) || string(retained.ReceiptHistory) != string(op.ReceiptHistory) {
		t.Fatalf("historical execution identity changed: %v", err)
	}
	if _, err = f.svc.Command(t.Context(), f.access, d.Material.ID, "start-plan", material.CommandInput{RequestID: uuid.NewString(), ExpectedVersion: after.Material.Version}); err == nil {
		t.Fatal("deleted runtime allowed plan restart")
	}
}

func TestMaterialRuntimeSwitchCannotTransferFrozenAttempt(t *testing.T) {
	f := newMaterialFixture(t, false)
	d := f.candidate(t)
	if _, err := f.svc.ProcessNext(t.Context()); err != nil {
		t.Fatal(err)
	}
	old, err := f.svc.Queries.CurrentMaterialOperation(t.Context(), util.MustParseUUID(d.Material.ID))
	if err != nil {
		t.Fatal(err)
	}
	replacement := f.fx.Runtime(t, "Replacement runtime", testutil.Cols{"daemon_id": "replacement-" + uuid.NewString()})
	f.settings.RequestID = uuid.NewString()
	f.settings.ExpectedVersion = 1
	f.settings.ExecutionRuntimeID = &replacement
	if _, err := f.svc.Configure(t.Context(), f.access, f.settings); err != nil {
		t.Fatal(err)
	}
	if task, err := f.svc.Tasks.ClaimTask(t.Context(), old.AgentID); err != nil || task != nil {
		t.Fatalf("old selection disclosed work: %+v %v", task, err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE id=$1 AND status='cancelled' AND runtime_id=$2`, old.TaskID, f.runtime) != 1 {
		t.Fatal("old queued attempt was moved or erased")
	}
	retained, err := f.svc.Queries.GetMaterialOperation(t.Context(), old.ID)
	if err != nil || retained.RuntimeID != old.RuntimeID || string(retained.Descriptor) != string(old.Descriptor) {
		t.Fatal("configuration changed historical operation identity")
	}
	if f.fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE runtime_id=$1`, replacement) != 0 {
		t.Fatal("runtime selection silently relaunched old task")
	}
	if current := f.detail(t, d.Material.ID); current.Operation.Status != "unknown" {
		t.Fatalf("cancelled execution still shown queued: %+v", current.Operation)
	}
}

func TestMaterialDelayedMergeReadbackRecoversOriginalPermit(t *testing.T) {
	f := newMaterialFixture(t, false)
	d := f.candidate(t)
	f.step(t)
	d = f.approve(t, f.detail(t, d.Material.ID))
	f.git.lostMerge = true
	f.step(t)
	if f.active == nil || f.active.Action != "merge" || f.lastErr == nil {
		t.Fatal("fixture did not lose actual merge response")
	}
	// Advance only this fixture's logical merge/permit clock, without waiting a
	// minute in the suite. The later read still traverses normal receipt handling.
	mergedAt := time.Now().Add(-3 * time.Minute)
	review := f.git.reviews[1]
	review.MergedAt = &mergedAt
	f.git.reviews[1] = review
	f.fx.Exec(t, `UPDATE git_material_operation SET permit_expires_at=$2 WHERE id=$1`, f.active.OperationID, mergedAt.Add(30*time.Second))
	if err := f.observe(t); err != nil {
		t.Fatalf("read after permit expiry lost actual authorized merge: %v", err)
	}
	if f.detail(t, d.Material.ID).Baseline == nil || f.git.merge != 1 {
		t.Fatal("delayed receipt lost baseline or repeated merge")
	}
	f.finish(t)
}

func TestMaterialRevokedRuntimeOwnerDoesNotStarveQueue(t *testing.T) {
	f := newMaterialFixture(t, false)
	owner := f.fx.User(t, "Runtime owner", uuid.NewString()+"@example.test")
	member := f.fx.Member(t, f.ws, owner, "member")
	f.fx.Exec(t, `UPDATE agent_runtime SET owner_id=$2,visibility='public' WHERE id=$1`, f.runtime, owner)
	f.settings.RequestID = uuid.NewString()
	f.settings.ExpectedVersion = 1
	if _, err := f.svc.Configure(t.Context(), f.access, f.settings); err != nil {
		t.Fatal(err)
	}
	f.initializeRepository(t)
	d := f.candidate(t)
	if _, err := f.svc.ProcessNext(t.Context()); err != nil {
		t.Fatal(err)
	}
	op, err := f.svc.Queries.CurrentMaterialOperation(t.Context(), util.MustParseUUID(d.Material.ID))
	if err != nil {
		t.Fatal(err)
	}
	f.fx.Exec(t, `DELETE FROM member WHERE id=$1`, member)
	if task, err := f.svc.Tasks.ClaimTask(t.Context(), op.AgentID); err != nil || task != nil {
		t.Fatalf("revocation failed to settle unstarted task: %+v %v", task, err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE id=$1 AND status='cancelled'`, op.TaskID) != 1 {
		t.Fatal("denied task left at queue head")
	}
}

func TestMaterialNativeGateWaitRetainsBlockedOutcome(t *testing.T) {
	f := newMaterialFixture(t, false)
	d := f.candidate(t)
	f.step(t)
	d = f.approve(t, f.detail(t, d.Material.ID))
	f.git.gates = false
	f.step(t)
	if f.active == nil || f.active.Action != "merge" {
		t.Fatal("expected rejected merge preflight")
	}
	f.finish(t)
	detail := f.detail(t, d.Material.ID)
	if detail.Operation.Status != "blocked" || detail.Operation.Blocker == nil || *detail.Operation.Blocker != "material_native_checks_pending" || detail.Baseline != nil {
		t.Fatalf("native wait looks active or merged: %+v", detail)
	}
}
