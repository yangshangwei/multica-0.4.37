package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/materialgit"
	"github.com/multica-ai/multica/server/internal/materialrepo"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func repositoryObserver(reader materialgit.Observer) (materialgit.RepositoryObserver, error) {
	r, ok := reader.(materialgit.RepositoryObserver)
	if !ok {
		return nil, errors.New("material_repository_observer_required")
	}
	return r, nil
}
func materialRepositoryPaths(e protocol.MaterialExecution) ([]string, error) {
	if e.RepositoryExecution == nil {
		return nil, errors.New("material_repository_descriptor_required")
	}
	catalog, err := materialrepo.BuildCatalog(materialrepo.Binding{SystemID: e.RepositoryExecution.SystemID, RootPath: e.RepositoryExecution.RootPath})
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(catalog.Files))
	for _, f := range catalog.Files {
		paths = append(paths, f.Path)
	}
	return paths, nil
}
func readMaterialRules(ctx context.Context, e protocol.MaterialExecution, reader materialgit.Observer) (json.RawMessage, error) {
	paths, err := materialRepositoryPaths(e)
	if err != nil {
		return nil, err
	}
	ro, err := repositoryObserver(reader)
	if err != nil {
		return nil, err
	}
	commit, err := ro.ResolveBranch(ctx, e.BaseBranch)
	if err != nil {
		return nil, err
	}
	snapshot, err := ro.Snapshot(ctx, commit, paths)
	if err != nil {
		return nil, err
	}
	// Rules receipts need the complete tree/digests but not unrelated file bodies.
	return json.Marshal(snapshot)
}
func readMaterialRulesObservation(ctx context.Context, e protocol.MaterialExecution, b protocol.MaterialClaimBinding, reader materialgit.Observer) (protocol.MaterialObservation, error) {
	o := protocol.MaterialObservation{ProtocolVersion: 1, OperationID: e.OperationID, TaskID: e.TaskID, RuntimeID: e.RuntimeID, ClaimGeneration: b.ClaimGeneration, HostChallenge: b.HostChallenge, ExpectedDigest: e.ExpectedDigest, ObservationID: uuid.NewString(), ObservedAt: time.Now().UTC(), Outcome: "rules_verified", RepositoryIdentity: e.Provider + ":" + e.InstanceURL + ":" + e.Repository, Files: []protocol.MaterialFile{}}
	var err error
	o.RulesSnapshot, err = readMaterialRules(ctx, e, reader)
	o.SourceComplete = err == nil
	if err == nil {
		var proof materialrepo.Proof
		var snapshot materialrepo.Snapshot
		if json.Unmarshal(e.RepositoryExecution.ReadyProof, &proof) != nil || json.Unmarshal(o.RulesSnapshot, &snapshot) != nil {
			return o, errors.New("material_repository_proof_invalid")
		}
		if checkErr := materialrepo.VerifyProtected(proof, snapshot); checkErr != nil {
			var invalid *materialrepo.Error
			if !errors.As(checkErr, &invalid) || invalid.Code != "protected_rule_changed" {
				return o, errors.New("material_repository_unverified")
			}
			o.Outcome = "rules_changed"
			o.Blocker = "material_repository_rules_changed"
		}
	}
	return o, err
}
func readRepositoryObservation(ctx context.Context, e protocol.MaterialExecution, b protocol.MaterialClaimBinding, reader materialgit.Observer) (protocol.MaterialObservation, error) {
	o := protocol.MaterialObservation{ProtocolVersion: 1, OperationID: e.OperationID, TaskID: e.TaskID, RuntimeID: e.RuntimeID, ClaimGeneration: b.ClaimGeneration, HostChallenge: b.HostChallenge, ExpectedDigest: e.ExpectedDigest, ObservationID: uuid.NewString(), ObservedAt: time.Now().UTC(), Outcome: "observed", RepositoryIdentity: e.Provider + ":" + e.InstanceURL + ":" + e.Repository, Files: []protocol.MaterialFile{}}
	paths, err := materialRepositoryPaths(e)
	if err != nil {
		return o, err
	}
	ro, err := repositoryObserver(reader)
	if err != nil {
		return o, err
	}
	if e.Action == "inspect_repository" || e.Action == "observe" && e.ReconcileAction == "inspect_repository" {
		commit, err := ro.ResolveBranch(ctx, e.BaseBranch)
		if err != nil {
			return o, err
		}
		snapshot, err := ro.Snapshot(ctx, commit, paths)
		if err != nil {
			return o, err
		}
		o.RepositorySnapshot, err = json.Marshal(snapshot)
		o.SourceComplete = err == nil
		return o, err
	}
	number := e.ReviewNumber
	if number == 0 {
		number, err = reader.FindReview(ctx, e.SourceBranch, e.BaseBranch)
		if err != nil {
			var remote *materialgit.Error
			if e.Action == "observe" && e.ReconcileAction == "publish" && errors.As(err, &remote) && remote.Code == "candidate_effect_absent" {
				o.Outcome = "not_found"
				o.SourceComplete = true
				return o, nil
			}
			return o, err
		}
	}
	review, err := reader.Observe(ctx, number)
	if err != nil {
		return o, err
	}
	if review.BaseBranch != e.BaseBranch || review.Branch != e.SourceBranch {
		return o, errors.New("material_review_target_changed")
	}
	changes := []protocol.MaterialChange{}
	publicationChanges := []materialrepo.Change{}
	for _, c := range review.Changes {
		previous := c.PreviousPath
		if previous == c.Path {
			previous = ""
		}
		changes = append(changes, protocol.MaterialChange{Path: c.Path, PreviousPath: previous, Status: c.Status, Mode: c.Mode})
		publicationChanges = append(publicationChanges, materialrepo.Change{Path: c.Path, PreviousPath: previous, Status: c.Status, Mode: c.Mode})
	}
	o.Review = &protocol.MaterialReview{Number: review.Number, URL: review.URL, SourceBranch: review.Branch, BaseBranch: review.BaseBranch, HeadSHA: review.HeadSHA, State: review.State, MergeSHA: review.MergeSHA, MergeActor: review.MergeActor, MergedAt: review.MergedAt, GatesPassed: review.GatesPassed, BlockReason: review.BlockReason, Changes: changes}
	commit := review.HeadSHA
	if review.State == "merged" {
		commit = review.MergeSHA
	}
	snapshot, err := ro.Snapshot(ctx, commit, paths)
	if err != nil {
		return o, err
	}
	if review.State == "merged" {
		o.RepositorySnapshot, err = json.Marshal(snapshot)
	} else {
		o.RepositoryPublication, err = json.Marshal(materialrepo.Publication{BaseCommit: review.BaseCommit, HeadCommit: review.HeadSHA, Complete: true, Changes: publicationChanges, Tree: snapshot})
	}
	if err != nil {
		return o, err
	}
	if e.AuditNote != nil {
		actor, err := reader.ObserveDecision(ctx, number, e.AuditNote.Marker, e.AuditNote.Body)
		if err != nil {
			var remote *materialgit.Error
			if e.Action == "observe" && e.ReconcileAction == "review_sync" && errors.As(err, &remote) && remote.Code == "decision_note_missing" {
				o.Outcome = "not_found"
			} else {
				return o, err
			}
		} else {
			o.AuditNote = &protocol.MaterialAuditNote{Marker: e.AuditNote.Marker, Body: e.AuditNote.Body, Actor: actor}
		}
	}
	o.SourceComplete = true
	return o, nil
}
