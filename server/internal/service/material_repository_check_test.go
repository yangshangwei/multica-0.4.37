package service

import (
	"path"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/material"
	"github.com/multica-ai/multica/server/internal/materialrepo"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func repositoryCandidateForCheck(t *testing.T) *materialFixture {
	t.Helper()
	f := newMaterialRepositoryFixture(t, false, false)
	f.admitRepository(t)
	o := f.rulesObservation(t, "observed")
	o.RulesSnapshot = nil
	o.RepositorySnapshot = materialJSON(materialrepo.Snapshot{Complete: true, Commit: strings.Repeat("a", 40), Entries: []materialrepo.Entry{}})
	if err := f.svc.ObserveOperation(t.Context(), f.task.ID, o); err != nil {
		t.Fatal(err)
	}
	f.finish(t)
	f.admitRepository(t)
	tree := catalogSnapshot(t, materialrepo.Binding{SystemID: f.active.RepositoryID, RootPath: f.settings.RootPath})
	pub := materialrepo.Publication{Complete: true, BaseCommit: strings.Repeat("a", 40), HeadCommit: tree.Commit, Tree: tree}
	review := &protocol.MaterialReview{Number: 1, URL: "https://github.com/example/materials/pull/1", SourceBranch: f.active.SourceBranch, BaseBranch: f.active.BaseBranch, HeadSHA: tree.Commit, State: "open"}
	for _, file := range f.active.Files {
		pub.Changes = append(pub.Changes, materialrepo.Change{Path: file.Path, Mode: file.Mode, Status: "added"})
		review.Changes = append(review.Changes, protocol.MaterialChange{Path: file.Path, Mode: file.Mode, Status: "added"})
	}
	o = f.rulesObservation(t, "observed")
	o.RulesSnapshot = nil
	o.Review = review
	o.RepositoryPublication = materialJSON(pub)
	if err := f.svc.ObserveOperation(t.Context(), f.task.ID, o); err != nil {
		t.Fatal(err)
	}
	f.finish(t)
	return f
}

func TestMaterialRepositoryMaintenanceDriftRemainsBlocked(t *testing.T) {
	f := newMaterialRepositoryFixture(t, false, false)
	f.admitRepository(t)
	catalog, err := materialrepo.BuildCatalogVersion(materialrepo.Binding{SystemID: f.active.RepositoryID, RootPath: f.settings.RootPath}, "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	snapshot := materialrepo.Snapshot{Complete: true, Commit: strings.Repeat("c", 40)}
	dirs := map[string]bool{}
	for _, file := range catalog.Files {
		body := file.Content
		snapshot.Entries = append(snapshot.Entries, materialrepo.Entry{Path: file.Path, Mode: file.Mode, SHA256: file.SHA256, Content: &body})
		for dir := path.Dir(file.Path); dir != "."; dir = path.Dir(dir) {
			dirs[dir] = true
		}
	}
	for dir := range dirs {
		snapshot.Entries = append(snapshot.Entries, materialrepo.Entry{Path: dir, Mode: "040000"})
	}
	o := f.rulesObservation(t, "observed")
	o.RulesSnapshot = nil
	o.RepositorySnapshot = materialJSON(snapshot)
	if err = f.svc.ObserveOperation(t.Context(), f.task.ID, o); err != nil {
		t.Fatal(err)
	}
	f.finish(t)
	view, err := f.svc.Initialization(t.Context(), f.access)
	if err != nil {
		t.Fatal(err)
	}
	if view.Readiness.BundleVersion == nil || *view.Readiness.BundleVersion != "1.0.0" {
		t.Fatal("old compatible catalog incorrectly upgraded")
	}
	_, err = f.svc.InitializationCommand(t.Context(), f.access, "maintenance", material.InitializationCommand{RequestID: uuid.NewString(), ExpectedVersion: view.Version, ExpectedSettingsVersion: view.SettingsVersion, TargetBundleVersion: materialrepo.BundleVersion})
	if err != nil {
		t.Fatal(err)
	}
	f.admitRepository(t)
	for i := range snapshot.Entries {
		if strings.HasSuffix(snapshot.Entries[i].Path, "AGENTS.md") {
			body := "changed rules"
			snapshot.Entries[i].Content = &body
			snapshot.Entries[i].SHA256 = material.Digest(body)
		}
	}
	o = f.rulesObservation(t, "observed")
	o.RulesSnapshot = nil
	o.RepositorySnapshot = materialJSON(snapshot)
	if err = f.svc.ObserveOperation(t.Context(), f.task.ID, o); err != nil {
		t.Fatalf("negative maintenance result rolled back: %v", err)
	}
	view, err = f.svc.Initialization(t.Context(), f.access)
	if err != nil || view.Readiness.PublishAllowed || view.Status != "blocked" {
		t.Fatalf("maintenance drift not retained: %+v %v", view, err)
	}
}

func TestMaterialPlanTaskCannotAdoptRecheckedRepositoryProof(t *testing.T) {
	f := newMaterialFixture(t, true)
	plan := f.queuedPlan(t)
	f.admit(t, plan.TaskID)
	if err := f.observe(t); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Tasks.StartTask(t.Context(), plan.TaskID); err != nil {
		t.Fatal(err)
	}
	task, err := f.svc.Queries.GetAgentTask(t.Context(), plan.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	root, err := f.svc.Queries.GetMaterialRepository(t.Context(), db.GetMaterialRepositoryParams{ProjectID: f.access.ProjectID, WorkspaceID: f.access.WorkspaceID})
	if err != nil {
		t.Fatal(err)
	}
	// A new accepted compatible commit is a different proof even with the same catalog.
	state, err := repositoryPayload(root)
	if err != nil {
		t.Fatal(err)
	}
	state.Proof.Commit = strings.Repeat("e", 40)
	if _, err = saveRepository(t.Context(), f.svc.Queries, root, state); err != nil {
		t.Fatal(err)
	}
	if err = AuthorizeMaterialPlanTask(t.Context(), f.svc.Queries, task); materialCode(err) != "material_repository_proof_changed" {
		t.Fatalf("running plan adopted newer proof: %v", err)
	}
}

func TestMaterialRepositoryCheckRetainsPublishedCandidate(t *testing.T) {
	f := repositoryCandidateForCheck(t)
	before, err := f.svc.Initialization(t.Context(), f.access)
	if err != nil {
		t.Fatal(err)
	}
	input := material.InitializationCommand{RequestID: uuid.NewString(), ExpectedVersion: before.Version, ExpectedSettingsVersion: before.SettingsVersion}
	after, err := f.svc.InitializationCommand(t.Context(), f.access, "check", input)
	if err != nil {
		t.Fatal(err)
	}
	if after.Candidate == nil || after.Candidate.ID != before.Candidate.ID || after.Status != "in_review" {
		t.Fatal("recheck abandoned published candidate")
	}
	if f.fx.Count(t, `SELECT count(*) FROM material_repository_revision WHERE project_id=$1`, f.project) != 1 {
		t.Fatal("recheck duplicated revision")
	}
}

func TestMaterialRepositoryRepeatedCheckBeforeDispatch(t *testing.T) {
	f := newMaterialRepositoryFixture(t, false, false)
	for i := 0; i < 3; i++ {
		v, e := f.svc.Initialization(t.Context(), f.access)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = f.svc.InitializationCommand(t.Context(), f.access, "check", material.InitializationCommand{RequestID: uuid.NewString(), ExpectedVersion: v.Version, ExpectedSettingsVersion: v.SettingsVersion}); e != nil {
			t.Fatal(e)
		}
	}
	if f.fx.Count(t, `SELECT count(*) FROM material_repository_revision WHERE project_id=$1`, f.project) != 1 || f.fx.Count(t, `SELECT count(*) FROM git_material_job WHERE project_id=$1`, f.project) != 1 {
		t.Fatal("queued inspection duplicated by check clicks")
	}
}

func TestMaterialRepositoryRulesMalformedReadDoesNotRevokeProof(t *testing.T) {
	f := newMaterialFixture(t, false)
	d := f.candidate(t)
	if _, err := f.svc.ProcessNext(t.Context()); err != nil {
		t.Fatal(err)
	}
	op, err := f.svc.Queries.CurrentMaterialOperation(t.Context(), util.MustParseUUID(d.Material.ID))
	if err != nil {
		t.Fatal(err)
	}
	f.admit(t, op.TaskID)
	o := f.rulesObservation(t, "rules_changed")
	malformed := f.rules
	malformed.Entries = append([]materialrepo.Entry{}, f.rules.Entries...)
	malformed.Entries[0].SHA256 = ""
	o.RulesSnapshot = materialJSON(malformed)
	if err := f.svc.ObserveOperation(t.Context(), op.TaskID, o); materialCode(err) != "material_repository_unverified" {
		t.Fatalf("unknown snapshot became proven drift: %v", err)
	}
	v, err := f.svc.Initialization(t.Context(), f.access)
	if err != nil || !v.Readiness.PublishAllowed {
		t.Fatal("invalid evidence revoked valid proof")
	}
}

func TestMaterialRepositoryRetryAfterDispatchFailure(t *testing.T) {
	f := newMaterialRepositoryFixture(t, false, false)
	f.fx.Exec(t, `UPDATE agent_runtime SET status='offline' WHERE id=$1`, f.runtime)
	if _, err := f.svc.ProcessNext(t.Context()); materialCode(err) != "material_runtime_offline" {
		t.Fatalf("expected offline blocker: %v", err)
	}
	f.fx.Exec(t, `UPDATE agent_runtime SET status='online' WHERE id=$1`, f.runtime)
	before, err := f.svc.Initialization(t.Context(), f.access)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.svc.InitializationCommand(t.Context(), f.access, "retry", material.InitializationCommand{RequestID: uuid.NewString(), ExpectedVersion: before.Version, ExpectedSettingsVersion: before.SettingsVersion})
	if err != nil {
		t.Fatalf("pre-dispatch retry stranded: %v", err)
	}
	f.admitRepository(t)
	if f.active.Action != "inspect_repository" {
		t.Fatal("retry did not inspect original root")
	}
	if f.fx.Count(t, `SELECT count(*) FROM material_repository_revision WHERE project_id=$1`, f.project) != 1 {
		t.Fatal("retry duplicated initial revision")
	}
}

func TestMaterialRepositorySameBindingRequiresNewDecision(t *testing.T) {
	for _, disable := range []bool{false, true} {
		t.Run(map[bool]string{false: "policy", true: "disable_enable"}[disable], func(t *testing.T) {
			f := repositoryCandidateForCheck(t)
			decide := func() material.Initialization {
				t.Helper()
				v, e := f.svc.Initialization(t.Context(), f.access)
				if e != nil {
					t.Fatal(e)
				}
				out, e := f.svc.InitializationDecide(t.Context(), f.access, material.InitializationDecisionInput{RequestID: uuid.NewString(), ExpectedVersion: v.Version, ExpectedSettingsVersion: v.SettingsVersion, CandidateID: v.Candidate.ID, ManifestSHA256: v.Candidate.ManifestSHA256, HeadSHA: *v.Candidate.HeadSHA, Decision: "approve"})
				if e != nil {
					t.Fatal(e)
				}
				return out
			}
			before := decide()
			f.admitRepository(t)
			old := *f.active
			oldTask := f.task
			f.settings.RequestID, f.settings.ExpectedVersion = uuid.NewString(), 1
			f.settings.Enabled = !disable
			if _, e := f.svc.Configure(t.Context(), f.access, f.settings); e != nil {
				t.Fatal(e)
			}
			if _, e := f.svc.OperationPermit(t.Context(), oldTask, old.OperationID, protocol.MaterialPermitInput{RequestID: uuid.NewString(), ExpectedDigest: old.ExpectedDigest}); e == nil {
				t.Fatal("old attempt kept permit")
			}
			f.finish(t)
			if disable {
				f.settings.RequestID, f.settings.ExpectedVersion, f.settings.Enabled = uuid.NewString(), 2, true
				if _, e := f.svc.Configure(t.Context(), f.access, f.settings); e != nil {
					t.Fatal(e)
				}
			}
			if _, _, e := repositoryDecisionCurrent(t.Context(), f.svc.Queries, util.MustParseUUID(old.RepositoryRevisionID)); materialCode(e) != "material_approval_changed" {
				t.Fatalf("old approval inherited current authority: %v", e)
			}
			after := decide()
			f.admitRepository(t)
			if f.active.RepositoryRevisionID != old.RepositoryRevisionID || f.active.SourceBranch != old.SourceBranch || f.active.ReviewNumber != old.ReviewNumber || f.active.Action != "review_sync" || f.active.ConfigVersion != after.SettingsVersion || f.active.DecisionID == old.DecisionID {
				t.Fatal("explicit reapproval failed to preserve candidate and replace authority")
			}
			if after.Candidate.ID != before.Candidate.ID || len(after.Decisions) != 2 || after.Decisions[0].SettingsVersion != before.SettingsVersion || f.fx.Count(t, `SELECT count(*) FROM material_repository_revision WHERE project_id=$1`, f.project) != 1 {
				t.Fatal("reauthorization rewrote history or duplicated PR intent")
			}
		})
	}
}
