package service

import (
	"path"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/material"
	"github.com/multica-ai/multica/server/internal/materialrepo"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func catalogSnapshot(t *testing.T, b materialrepo.Binding) materialrepo.Snapshot {
	t.Helper()
	catalog, err := materialrepo.BuildCatalog(b)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := materialrepo.Snapshot{Commit: strings.Repeat("c", 40), Complete: true}
	dirs := map[string]bool{}
	for _, f := range catalog.Files {
		body := f.Content
		snapshot.Entries = append(snapshot.Entries, materialrepo.Entry{Path: f.Path, Mode: f.Mode, SHA256: f.SHA256, Content: &body})
		for dir := path.Dir(f.Path); dir != "."; dir = path.Dir(dir) {
			dirs[dir] = true
		}
	}
	for dir := range dirs {
		snapshot.Entries = append(snapshot.Entries, materialrepo.Entry{Path: dir, Mode: "040000"})
	}
	return snapshot
}
func (f *materialFixture) rulesObservation(t *testing.T, outcome string) protocol.MaterialObservation {
	t.Helper()
	d := f.active
	return protocol.MaterialObservation{ProtocolVersion: 1, OperationID: d.OperationID, TaskID: d.TaskID, RuntimeID: d.RuntimeID, ClaimGeneration: f.claim.ClaimGeneration, ExpectedDigest: d.ExpectedDigest, ObservationID: uuid.NewString(), HostChallenge: f.claim.HostChallenge, ObservedAt: time.Now(), Outcome: outcome, SourceComplete: true, RepositoryIdentity: d.Provider + ":" + d.InstanceURL + ":" + d.Repository, RulesSnapshot: materialJSON(f.rules)}
}
func (f *materialFixture) initializeRepository(t *testing.T) {
	t.Helper()
	if _, err := f.svc.ProcessNext(t.Context()); err != nil {
		t.Fatal(err)
	}
	root, err := f.svc.Queries.GetMaterialRepository(t.Context(), db.GetMaterialRepositoryParams{ProjectID: f.access.ProjectID, WorkspaceID: f.access.WorkspaceID})
	if err != nil {
		t.Fatal(err)
	}
	op, err := f.svc.Queries.CurrentMaterialRepositoryOperation(t.Context(), root.ID)
	if err != nil {
		t.Fatal(err)
	}
	f.admit(t, op.TaskID)
	if f.active.Action != "inspect_repository" {
		t.Fatal("expected one initialization inspection")
	}
	f.rules = catalogSnapshot(t, materialrepo.Binding{SystemID: util.UUIDToString(root.ID), RootPath: f.settings.RootPath})
	o := f.rulesObservation(t, "observed")
	o.RulesSnapshot = nil
	o.RepositorySnapshot = materialJSON(f.rules)
	if err = f.svc.ObserveOperation(t.Context(), op.TaskID, o); err != nil {
		t.Fatal(err)
	}
	f.finish(t)
}
func TestMaterialRepositoryInitialCheckAndDraftGate(t *testing.T) {
	f := newMaterialRepositoryFixture(t, false, false)
	view, err := f.svc.Initialization(t.Context(), f.access)
	if err != nil {
		t.Fatal(err)
	}
	if view.Readiness.PublishAllowed || view.Status != "checking" {
		t.Fatalf("unverified initialization: %+v", view)
	}
	created, err := f.svc.Create(t.Context(), f.access, material.CreateInput{RequestID: uuid.NewString(), Title: "draft remains available"})
	if err != nil {
		t.Fatal(err)
	}
	revision := material.RevisionInput{RequestID: uuid.NewString(), ExpectedVersion: created.Material.Version, Kind: "requirement", Content: strings.Repeat("Accepted requirement body. ", 4), Sources: []material.Source{{ResourceID: f.resource, Commit: strings.Repeat("a", 40), Path: "README.md"}}, AcceptanceCriteria: []string{"AC1: initialized publication only"}, Ready: true}
	if _, err = f.svc.Revise(t.Context(), f.access, created.Material.ID, revision); materialCode(err) != "material_repository_not_ready" {
		t.Fatalf("publication bypassed initialization: %v", err)
	}
	revision.Ready = false
	if _, err = f.svc.Revise(t.Context(), f.access, created.Material.ID, revision); err != nil {
		t.Fatal(err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM git_material_job WHERE project_id=$1 AND target_kind='repository_initialization'`, f.project) != 1 {
		t.Fatal("configuration duplicated initialization")
	}
	f.initializeRepository(t)
	view, err = f.svc.Initialization(t.Context(), f.access)
	if err != nil || !view.Readiness.PublishAllowed || view.Readiness.VerifiedCommit == nil {
		t.Fatalf("compatible readback not adopted: %+v %v", view, err)
	}
	before := f.fx.Count(t, `SELECT count(*) FROM git_material_job WHERE project_id=$1`, f.project)
	command := material.InitializationCommand{RequestID: uuid.NewString(), ExpectedVersion: view.Version, ExpectedSettingsVersion: view.SettingsVersion, TargetBundleVersion: materialrepo.BundleVersion}
	result, err := f.svc.InitializationCommand(t.Context(), f.access, "maintenance", command)
	if err != nil || result.Version != view.Version {
		t.Fatalf("same bundle maintenance not no-op: %+v %v", result, err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM git_material_job WHERE project_id=$1`, f.project) != before {
		t.Fatal("same bundle created remote work")
	}
	command.TargetBundleVersion = "future"
	if _, err = f.svc.InitializationCommand(t.Context(), f.access, "maintenance", command); materialCode(err) != "material_idempotency_conflict" {
		t.Fatalf("request mutation accepted: %v", err)
	}
}
func TestMaterialRepositoryRulesDriftPersistsBlockedState(t *testing.T) {
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
	for i := range f.rules.Entries {
		e := &f.rules.Entries[i]
		if strings.HasSuffix(e.Path, "AGENTS.md") {
			body := "Unauthorized changed rules"
			e.Content = &body
			e.SHA256 = material.Digest(body)
		}
	}
	o := f.rulesObservation(t, "rules_changed")
	if err = f.svc.ObserveOperation(t.Context(), op.TaskID, o); err != nil {
		t.Fatalf("negative receipt was not acknowledged: %v", err)
	}
	if err = f.svc.ObserveOperation(t.Context(), op.TaskID, o); err != nil {
		t.Fatalf("negative receipt replay failed after invalidation: %v", err)
	}
	view, err := f.svc.Initialization(t.Context(), f.access)
	if err != nil || view.Readiness.PublishAllowed || view.Blocker == nil {
		t.Fatalf("drift was not durably blocked: %+v %v", view, err)
	}
	if _, err = f.svc.OperationPermit(t.Context(), f.task, f.active.OperationID, protocol.MaterialPermitInput{RequestID: uuid.NewString(), ExpectedDigest: f.active.ExpectedDigest}); materialCode(err) != "material_repository_not_ready" {
		t.Fatalf("drift still authorized mutation: %v", err)
	}
}

func (f *materialFixture) admitRepository(t *testing.T) {
	t.Helper()
	f.fx.Exec(t, `UPDATE git_material_job SET next_at=now() WHERE project_id=$1`, f.project)
	if worked, err := f.svc.ProcessNext(t.Context()); err != nil || !worked {
		t.Fatalf("repository dispatch: %v %v", worked, err)
	}
	root, err := f.svc.Queries.GetMaterialRepository(t.Context(), db.GetMaterialRepositoryParams{ProjectID: f.access.ProjectID, WorkspaceID: f.access.WorkspaceID})
	if err != nil {
		t.Fatal(err)
	}
	op, err := f.svc.Queries.CurrentMaterialRepositoryOperation(t.Context(), root.ID)
	if err != nil {
		t.Fatal(err)
	}
	f.admit(t, op.TaskID)
}
func TestMaterialRepositoryReviewedInitializationAndReapproval(t *testing.T) {
	f := newMaterialRepositoryFixture(t, false, false)
	f.admitRepository(t)
	rootID := f.active.RepositoryID
	before := materialrepo.Snapshot{Complete: true, Commit: strings.Repeat("a", 40), Entries: []materialrepo.Entry{}}
	o := f.rulesObservation(t, "observed")
	o.RulesSnapshot = nil
	o.RepositorySnapshot = materialJSON(before)
	if err := f.svc.ObserveOperation(t.Context(), f.task.ID, o); err != nil {
		t.Fatal(err)
	}
	f.finish(t)
	f.admitRepository(t)
	if f.active.Action != "publish" {
		t.Fatal("missing initialization publication")
	}
	after := catalogSnapshot(t, materialrepo.Binding{SystemID: rootID, RootPath: f.settings.RootPath})
	review := protocol.MaterialReview{Number: 1, URL: "https://github.com/example/materials/pull/1", SourceBranch: f.active.SourceBranch, BaseBranch: "main", HeadSHA: after.Commit, State: "open", GatesPassed: true}
	pub := materialrepo.Publication{BaseCommit: before.Commit, HeadCommit: after.Commit, Complete: true, Tree: after}
	for _, file := range f.active.Files {
		review.Changes = append(review.Changes, protocol.MaterialChange{Path: file.Path, Mode: file.Mode, Status: "added"})
		pub.Changes = append(pub.Changes, materialrepo.Change{Path: file.Path, Mode: file.Mode, Status: "added"})
	}
	observe := func(merged bool) {
		t.Helper()
		o := f.rulesObservation(t, "observed")
		o.RulesSnapshot = nil
		o.Review = &review
		if merged {
			o.RepositorySnapshot = materialJSON(after)
		} else {
			o.RepositoryPublication = materialJSON(pub)
		}
		if f.active.AuditNote != nil {
			note := *f.active.AuditNote
			note.Actor = "runtime"
			o.AuditNote = &note
		}
		if err := f.svc.ObserveOperation(t.Context(), f.task.ID, o); err != nil {
			t.Fatal(err)
		}
		if err := f.svc.ObserveOperation(t.Context(), f.task.ID, o); err != nil {
			t.Fatalf("lost acknowledgement could not replay: %v", err)
		}
	}
	observe(false)
	f.finish(t)
	view, err := f.svc.Initialization(t.Context(), f.access)
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range []string{"head", "manifest", "version", "settings"} {
		input := material.InitializationDecisionInput{RequestID: uuid.NewString(), ExpectedVersion: view.Version, ExpectedSettingsVersion: view.SettingsVersion, CandidateID: view.Candidate.ID, ManifestSHA256: view.Candidate.ManifestSHA256, HeadSHA: review.HeadSHA, Decision: "approve"}
		switch change {
		case "head":
			input.HeadSHA = strings.Repeat("f", 40)
		case "manifest":
			input.ManifestSHA256 = strings.Repeat("f", 64)
		case "version":
			input.ExpectedVersion--
		case "settings":
			input.ExpectedSettingsVersion--
		}
		if _, err = f.svc.InitializationDecide(t.Context(), f.access, input); materialCode(err) != "material_initialization_changed" {
			t.Fatalf("stale %s accepted: %v", change, err)
		}
	}
	decide := func(decision string) {
		t.Helper()
		view, err := f.svc.Initialization(t.Context(), f.access)
		if err != nil {
			t.Fatal(err)
		}
		if view.Candidate == nil || view.Readiness.PublishAllowed {
			t.Fatal("candidate bypassed review")
		}
		_, err = f.svc.InitializationDecide(t.Context(), f.access, material.InitializationDecisionInput{RequestID: uuid.NewString(), ExpectedVersion: view.Version, ExpectedSettingsVersion: view.SettingsVersion, CandidateID: view.Candidate.ID, ManifestSHA256: view.Candidate.ManifestSHA256, HeadSHA: review.HeadSHA, Decision: decision})
		if err != nil {
			t.Fatal(err)
		}
	}
	decide("request_changes")
	f.admitRepository(t)
	observe(false)
	f.finish(t)
	decide("approve")
	f.admitRepository(t)
	if f.active.Action != "review_sync" {
		t.Fatal("new approval skipped audit note")
	}
	observe(false)
	f.finish(t)
	f.admitRepository(t)
	if f.active.Action != "merge" {
		t.Fatal("approval did not queue merge")
	}
	observe(false)
	if _, err := f.svc.OperationPermit(t.Context(), f.task, f.active.OperationID, protocol.MaterialPermitInput{RequestID: uuid.NewString(), ExpectedDigest: f.active.ExpectedDigest}); err != nil {
		t.Fatal(err)
	}
	review.State = "merged"
	review.MergeSHA = strings.Repeat("d", 40)
	at := time.Now()
	review.MergedAt = &at
	review.MergeActor = "runtime"
	after.Commit = review.MergeSHA
	observe(true)
	f.finish(t)
	view, err = f.svc.Initialization(t.Context(), f.access)
	if err != nil || !view.Readiness.PublishAllowed || view.Status != "ready" || len(view.Decisions) != 2 {
		t.Fatalf("reviewed initialization not ready with retained history: %+v %v", view, err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM git_material WHERE project_id=$1`, f.project) != 0 {
		t.Fatal("initialization invented a material row")
	}
}

func TestMaterialRepositoryInspectionAuthorityAndCompleteness(t *testing.T) {
	for _, scenario := range []string{"incomplete", "wrong repository", "symlink", "stale generation", "missing id"} {
		t.Run(scenario, func(t *testing.T) {
			f := newMaterialRepositoryFixture(t, false, false)
			f.admitRepository(t)
			snapshot := materialrepo.Snapshot{Complete: true, Commit: strings.Repeat("a", 40), Entries: []materialrepo.Entry{}}
			o := f.rulesObservation(t, "observed")
			o.RulesSnapshot = nil
			switch scenario {
			case "incomplete":
				snapshot.Complete = false
			case "wrong repository":
				o.RepositoryIdentity = "github:https://github.com:other/repo"
			case "symlink":
				snapshot.Entries = []materialrepo.Entry{{Path: f.settings.RootPath, Mode: "120000", SHA256: strings.Repeat("a", 64)}}
			case "stale generation":
				o.ClaimGeneration--
			case "missing id":
				o.ObservationID = ""
			}
			o.RepositorySnapshot = materialJSON(snapshot)
			err := f.svc.ObserveOperation(t.Context(), f.task.ID, o)
			view, readErr := f.svc.Initialization(t.Context(), f.access)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if view.Readiness.PublishAllowed || view.Candidate != nil && view.Candidate.HeadSHA != nil {
				t.Fatal("invalid evidence established publication authority")
			}
			if err == nil && view.Status != "blocked" {
				t.Fatalf("invalid snapshot accepted: %+v", view)
			}
		})
	}
}
func TestMaterialRepositoryManagerAndHistoryGates(t *testing.T) {
	f := newMaterialFixture(t, false)
	view, err := f.svc.Initialization(t.Context(), f.access)
	if err != nil {
		t.Fatal(err)
	}
	outsider := f.fx.User(t, "PRD reviewer", uuid.NewString()+"@example.test")
	f.fx.Member(t, f.ws, outsider, "member")
	access := f.access
	access.Credential = stageCredential(outsider)
	cmd := material.InitializationCommand{RequestID: uuid.NewString(), ExpectedVersion: view.Version, ExpectedSettingsVersion: view.SettingsVersion}
	if _, err = f.svc.InitializationCommand(t.Context(), access, "check", cmd); materialCode(err) != "material_human_manager_required" {
		t.Fatalf("nonmaintainer check authority: %v", err)
	}
	access = f.access
	access.Credential.Principal.AgentActor = true
	if _, err = f.svc.InitializationDecide(t.Context(), access, material.InitializationDecisionInput{}); err == nil {
		t.Fatal("machine self approval accepted")
	}
	if retained, err := f.svc.Queries.MaterialProjectHasHistory(t.Context(), db.MaterialProjectHasHistoryParams{ProjectID: f.access.ProjectID, WorkspaceID: f.access.WorkspaceID}); err != nil || !retained {
		t.Fatal("initialization-only project deletion unguarded")
	}
	if retained, err := f.svc.Queries.MaterialWorkspaceHasHistory(t.Context(), f.access.WorkspaceID); err != nil || !retained {
		t.Fatal("initialization-only workspace deletion unguarded")
	}
	replacement := f.fx.Runtime(t, "new initialization host", testutil.Cols{"daemon_id": "new-init-" + uuid.NewString()})
	f.settings.ExpectedVersion = 1
	f.settings.RequestID = uuid.NewString()
	f.settings.ExecutionRuntimeID = &replacement
	if _, err = f.svc.Configure(t.Context(), f.access, f.settings); err != nil {
		t.Fatal(err)
	}
	next, err := f.svc.Initialization(t.Context(), f.access)
	if err != nil || next.Readiness.PublishAllowed || len(next.History) != 1 || next.History[0].VerifiedCommit == nil || *next.History[0].VerifiedCommit != *view.Readiness.VerifiedCommit {
		t.Fatalf("binding change erased old proof or reused authority: %+v %v", next, err)
	}
}
