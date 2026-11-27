package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/material"
	"github.com/multica-ai/multica/server/internal/materialrepo"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func repositoryDecisionCurrent(ctx context.Context, q *db.Queries, rev pgtype.UUID) (db.MaterialRepositoryDecision, material.InitializationDecision, error) {
	row, err := q.LatestMaterialRepositoryDecision(ctx, rev)
	if err != nil {
		return row, material.InitializationDecision{}, err
	}
	d, err := materialParse[material.InitializationDecision](row.Payload)
	if err != nil {
		return row, d, err
	}
	if _, err = currentMaterialManager(ctx, q, row.WorkspaceID, row.ActorID, row.MemberID); err != nil {
		return row, d, err
	}
	signer, err := materialParse[stageSignerCredential](row.Credential)
	if err != nil {
		return row, d, err
	}
	settings, err := q.GetMaterialSettings(ctx, db.GetMaterialSettingsParams{ProjectID: row.ProjectID, WorkspaceID: row.WorkspaceID})
	if err != nil {
		return row, d, err
	}
	root, err := q.GetMaterialRepositoryByID(ctx, row.RepositoryID)
	if err != nil {
		return row, d, err
	}
	if d.SettingsVersion != settings.Version || d.BindingGeneration != root.Generation {
		return row, d, materialError(409, "material_approval_changed")
	}
	return row, d, materialSignerCurrent(ctx, q, signer, row.ActorID)
}
func authorizeRepositoryOperation(ctx context.Context, q *db.Queries, op db.GitMaterialOperation, d protocol.MaterialExecution) (db.GitMaterialOperation, protocol.MaterialExecution, error) {
	root, err := q.GetMaterialRepositoryByID(ctx, op.RepositoryID)
	if err != nil {
		return op, d, err
	}
	state, err := repositoryPayload(root)
	if err != nil {
		return op, d, err
	}
	if d.RepositoryID != util.UUIDToString(root.ID) || d.RepositoryGeneration != root.Generation || state.RevisionID != d.RepositoryRevisionID || op.RepositoryRevisionID != util.MustParseUUID(d.RepositoryRevisionID) {
		return op, d, materialError(409, "material_initialization_changed")
	}
	if d.DecisionID != "" {
		_, decision, err := repositoryDecisionCurrent(ctx, q, op.RepositoryRevisionID)
		if err != nil {
			return op, d, err
		}
		if decision.ID != d.DecisionID || decision.HeadSHA != d.ApprovedHead || d.Action == "merge" && decision.Decision != "approve" {
			return op, d, materialError(409, "material_approval_changed")
		}
	}
	return op, d, nil
}
func (s *MaterialService) processRepositoryJob(ctx context.Context, job db.GitMaterialJob) error {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := s.Queries.WithTx(tx)
	if _, err = q.LockProjectForExecutionSquad(ctx, db.LockProjectForExecutionSquadParams{ID: job.ProjectID, WorkspaceID: job.WorkspaceID}); err != nil {
		return err
	}
	if _, err = q.LockMaterialJob(ctx, db.LockMaterialJobParams{ID: job.ID, LeaseToken: job.LeaseToken}); err != nil {
		return err
	}
	setting, err := q.GetMaterialSettings(ctx, db.GetMaterialSettingsParams{ProjectID: job.ProjectID, WorkspaceID: job.WorkspaceID})
	if err != nil {
		return err
	}
	b, err := materialParse[materialBinding](setting.Settings)
	if err != nil {
		return err
	}
	if !b.Enabled {
		return materialError(409, "material_disabled")
	}
	if _, err = currentMaterialManager(ctx, q, job.WorkspaceID, setting.AuthorizedBy, setting.MemberID); err != nil {
		return err
	}
	if err = materialSignerCurrent(ctx, q, b.Credential, setting.AuthorizedBy); err != nil {
		return err
	}
	if _, err = checkMaterialBinding(ctx, q, job.WorkspaceID, job.ProjectID, b, false); err != nil {
		return err
	}
	root, err := q.GetMaterialRepositoryByID(ctx, job.RepositoryID)
	if err != nil {
		return err
	}
	state, err := repositoryPayload(root)
	if err != nil {
		return err
	}
	if state.RevisionID != util.UUIDToString(job.RepositoryRevisionID) || root.BindingDigest != repositoryBindingDigest(b) {
		return materialError(409, "material_initialization_changed")
	}
	rev, err := q.GetMaterialRepositoryRevision(ctx, job.RepositoryRevisionID)
	if err != nil {
		return err
	}
	r, err := materialParse[repositoryRevision](rev.Payload)
	if err != nil {
		return err
	}
	// Candidate geometry stays immutable; execution authority is captured anew
	// per attempt. A same-target policy edit requires a fresh decision below,
	// while a changed repository/runtime binding has a different generation.
	if rev.Generation != root.Generation {
		return materialError(409, "material_configuration_changed")
	}
	runtime, err := materialRuntimeForUser(ctx, q, job.WorkspaceID, b.ExecutionRuntimeID, setting.AuthorizedBy)
	if err != nil {
		return err
	}
	if runtime.Status != "online" {
		return materialError(409, "material_runtime_offline")
	}
	prior, err := q.LatestMaterialOperation(ctx, job.ID)
	attempt := int32(1)
	action := job.Kind
	if err == nil {
		attempt = prior.AttemptNo + 1
		newDecision := false
		if action == "review_sync" || action == "merge" {
			old, e := materialParse[protocol.MaterialExecution](prior.Descriptor)
			if e != nil {
				return e
			}
			_, current, e := repositoryDecisionCurrent(ctx, q, job.RepositoryRevisionID)
			if e != nil {
				return e
			}
			if old.DecisionID != current.ID {
				task, e := q.GetAgentTask(ctx, prior.TaskID)
				if e != nil {
					return e
				}
				newDecision = task.Status == "completed" || task.Status == "failed" || task.Status == "cancelled"
			}
		}
		if !newDecision && prior.State != "succeeded" && prior.State != "recovery_requested" && prior.State != "no_effect" {
			return tx.Commit(ctx)
		}
		if newDecision {
			prior.State = ""
		}
		if prior.State == "succeeded" {
			if action != "review_sync" {
				if err = q.CompleteMaterialOperationJob(ctx, db.CompleteMaterialOperationJobParams{ID: job.ID, State: "done"}); err != nil {
					return err
				}
				return tx.Commit(ctx)
			}
			old, e := materialParse[protocol.MaterialExecution](prior.Descriptor)
			if e != nil {
				return e
			}
			_, current, e := repositoryDecisionCurrent(ctx, q, job.RepositoryRevisionID)
			if e != nil {
				return e
			}
			if old.DecisionID == current.ID {
				if err = q.CompleteMaterialOperationJob(ctx, db.CompleteMaterialOperationJobParams{ID: job.ID, State: "done"}); err != nil {
					return err
				}
				return tx.Commit(ctx)
			}
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	taskID := dbid.NewV7()
	d := protocol.MaterialExecution{ProtocolVersion: 1, TargetKind: "repository_initialization", RepositoryID: util.UUIDToString(root.ID), RepositoryRevisionID: util.UUIDToString(rev.ID), RepositoryGeneration: root.Generation, OperationID: util.UUIDToString(dbid.NewV7()), JobID: util.UUIDToString(job.ID), ProjectID: util.UUIDToString(job.ProjectID), WorkspaceID: util.UUIDToString(job.WorkspaceID), TaskID: util.UUIDToString(taskID), Action: action, OperationKey: util.UUIDToString(rev.ID), ConfigVersion: b.Version, ResourceID: b.ResourceID, Provider: b.Provider, InstanceURL: b.InstanceURL, Repository: b.Repository, BaseBranch: b.BaseBranch, SourceBranch: "multica/initialize-" + util.UUIDToString(rev.ID), RuntimeID: util.UUIDToString(runtime.ID), DaemonID: runtime.DaemonID.String, ProfileID: util.UUIDToString(runtime.ProfileID), ConfigDigest: b.RuntimeDigest, Title: "Initialize material repository", Body: "Independent reviewed material repository rules", RepositoryExecution: &protocol.MaterialRepositoryExecution{SystemID: util.UUIDToString(root.ID), RootPath: b.RootPath, BundleVersion: r.BundleVersion}}
	if r.Candidate != nil {
		d.RepositoryExecution.Candidate = materialJSON(r.Candidate)
		d.RepositoryExecution.CatalogDigest = r.Candidate.CatalogDigest
		d.RepositoryExecution.BaseCommit = r.Candidate.Before.Commit
		for _, f := range r.Candidate.Files {
			d.Files = append(d.Files, protocol.MaterialFile{Path: f.Path, Mode: f.Mode, Content: f.Content, SHA256: f.SHA256})
		}
	}
	if r.Maintenance != nil {
		d.RepositoryExecution.Maintenance = materialJSON(r.Maintenance)
		d.RepositoryExecution.CatalogDigest = r.Maintenance.CatalogDigest
		d.RepositoryExecution.BaseCommit = r.Maintenance.Before.Commit
		for _, f := range r.Maintenance.Changes {
			d.Files = append(d.Files, protocol.MaterialFile{Path: f.After.Path, Mode: f.After.Mode, Content: f.After.Content, SHA256: f.After.SHA256})
		}
	}
	if state.Proof != nil {
		d.RepositoryExecution.ReadyProof = materialJSON(state.Proof)
	}
	if r.Review != nil {
		d.ReviewNumber = r.Review.Number
		d.ReviewURL = r.Review.URL
		d.ApprovedHead = r.Review.HeadSHA
	}
	if action == "review_sync" || action == "merge" {
		_, decision, err := repositoryDecisionCurrent(ctx, q, rev.ID)
		if err != nil {
			return err
		}
		if action == "merge" && decision.Decision != "approve" {
			return materialError(409, "material_merge_not_authorized")
		}
		d.DecisionID = decision.ID
		d.ApprovedHead = decision.HeadSHA
		d.OperationKey = decision.ID
		d.AuditNote = &protocol.MaterialAuditNote{Marker: "multica-decision:" + decision.ID, Body: fmt.Sprintf("<!-- multica-decision:%s -->\nRepository maintenance human decision %s by %s at %s. Reason: %s", decision.ID, decision.Decision, decision.ActorID, decision.HeadSHA, decision.Reason)}
	}
	if prior.State == "recovery_requested" || prior.State == "no_effect" {
		old, err := materialParse[protocol.MaterialExecution](prior.Descriptor)
		if err != nil {
			return err
		}
		old.OperationID = d.OperationID
		old.TaskID = d.TaskID
		if old.Action == "observe" {
			old.Action = old.ReconcileAction
			old.ReconcileAction = ""
		}
		if prior.State == "recovery_requested" {
			old.ReconcileAction = old.Action
			old.Action = "observe"
		}
		if old.DecisionID == "" {
			// Explicit recovery may read the same immutable candidate under a
			// newer policy on this unchanged binding. Never transfer an approval.
			old.ConfigVersion = d.ConfigVersion
			old.ConfigDigest = d.ConfigDigest
		}
		d = old
	}
	agent := util.MustParseUUID(b.CarrierID)
	task, err := q.CreateMaterialOperationTask(ctx, db.CreateMaterialOperationTaskParams{ID: taskID, AgentID: agent, RuntimeID: runtime.ID, AuthorizerID: setting.AuthorizedBy, OperationID: util.MustParseUUID(d.OperationID), Context: materialJSON(map[string]any{"type": "material_operation", "operation_id": d.OperationID})})
	if err != nil {
		return err
	}
	if err = createMaterialOperation(ctx, q, job, d, agent, setting.AuthorizedBy, attempt); err != nil {
		return err
	}
	if err = q.FinishMaterialJob(ctx, db.FinishMaterialJobParams{ID: job.ID, LeaseToken: job.LeaseToken, State: "dispatched"}); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	if s.Tasks != nil {
		s.Tasks.FinalizeIssueTaskEnqueue(ctx, task)
	}
	return nil
}
func repositoryObservation(ctx context.Context, q *db.Queries, op db.GitMaterialOperation, d protocol.MaterialExecution, o protocol.MaterialObservation) (string, error) {
	if o.ProtocolVersion != 1 || !o.SourceComplete || o.RepositoryIdentity != d.Provider+":"+d.InstanceURL+":"+d.Repository || o.ObservedAt.IsZero() || time.Since(o.ObservedAt) > 5*time.Minute || o.ObservedAt.After(time.Now().Add(time.Minute)) {
		return "", materialError(409, "material_observation_incomplete")
	}
	root, err := q.GetMaterialRepositoryByID(ctx, op.RepositoryID)
	if err != nil {
		return "", err
	}
	state, err := repositoryPayload(root)
	if err != nil {
		return "", err
	}
	revision, err := q.GetMaterialRepositoryRevision(ctx, op.RepositoryRevisionID)
	if err != nil {
		return "", err
	}
	r, err := materialParse[repositoryRevision](revision.Payload)
	if err != nil {
		return "", err
	}
	var snapshot materialrepo.Snapshot
	if len(o.RepositorySnapshot) > 0 {
		snapshot, err = materialParse[materialrepo.Snapshot](o.RepositorySnapshot)
		if err != nil {
			return "", err
		}
	}
	action := d.Action
	if action == "observe" {
		action = d.ReconcileAction
	}
	operationState := "succeeded"
	jobState := "done"
	if action == "inspect_repository" {
		if err != nil {
			return "", err
		}
		binding := materialrepo.Binding{SystemID: d.RepositoryExecution.SystemID, RootPath: d.RepositoryExecution.RootPath}
		if r.Kind == "maintenance" {
			if state.Proof == nil {
				return "", materialError(409, "material_repository_not_ready")
			}
			r.Maintenance, err = materialrepo.PlanMaintenance(*state.Proof, snapshot, r.BundleVersion)
			if err != nil {
				var invalid *materialrepo.Error
				if !errors.As(err, &invalid) || invalid.Code != "protected_rule_changed" {
					return "", materialError(409, "material_repository_unverified")
				}
				state.Status = "blocked"
				state.Blocker = "material_repository_rules_changed"
				err = nil // Persist the refused inspection; it is not readiness.
			} else if r.Maintenance == nil {
				state.Status = "ready"
				state.Blocker = ""
			} else {
				state.Status = "publishing"
				err = repositoryJob(ctx, q, root, revision.ID, "publish")
			}
		} else {
			inspection, e := materialrepo.Inspect(binding, snapshot)
			if e != nil {
				return "", materialError(409, "material_repository_unverified")
			} else {
				r.Snapshot = &snapshot
				r.Inspection = &inspection
				r.Candidate = inspection.Candidate
				if r.Candidate != nil {
					catalog, e := materialrepo.SupportedProofCatalog(materialrepo.Proof{Binding: r.Candidate.Binding, CatalogDigest: r.Candidate.CatalogDigest})
					if e != nil {
						return "", e
					}
					r.BundleVersion = catalog.BundleVersion
				}
				if inspection.Compatible {
					proof, e := materialrepo.VerifyExisting(binding, snapshot)
					if e != nil {
						return "", e
					}
					if state.Proof != nil && material.JSONDigest(state.Proof) != material.JSONDigest(proof) {
						v := materialrepo.SchemaVersion
						state.History = append(state.History, material.RepositoryReadiness{VerifiedCommit: materialPtr(state.Proof.Commit), ManifestSHA256: materialPtr(state.Proof.CatalogDigest), SchemaVersion: &v, BundleVersion: materialPtr(state.BundleVersion), VerifiedAt: state.VerifiedAt})
					}
					state.Proof = &proof
					catalog, e := materialrepo.SupportedProofCatalog(proof)
					if e != nil {
						return "", e
					}
					state.BundleVersion = catalog.BundleVersion
					r.BundleVersion = catalog.BundleVersion
					state.VerifiedAt = &o.ObservedAt
					state.Status = "ready"
					state.Blocker = ""
				} else if inspection.Candidate != nil {
					state.Status = "publishing"
					err = repositoryJob(ctx, q, root, revision.ID, "publish")
				} else {
					state.Status = "blocked"
					state.Blocker = "material_repository_conflict"
					state.Proof = nil
				}
			}
		}
	} else {
		if d.Action == "observe" && action == "publish" && o.Outcome == "not_found" && o.Review == nil {
			operationState = "no_effect"
		} else {
			review := o.Review
			if review == nil || review.Number <= 0 || !strings.HasPrefix(review.URL, d.InstanceURL+"/"+d.Repository+"/") || review.SourceBranch != d.SourceBranch || review.BaseBranch != d.BaseBranch || !material.IsSHA(review.HeadSHA) || d.ReviewNumber != 0 && d.ReviewNumber != review.Number || d.ApprovedHead != "" && d.ApprovedHead != review.HeadSHA {
				return "", materialError(409, "material_review_changed")
			}
			var publication materialrepo.Publication
			var e error
			if review.State != "merged" {
				publication, e = materialParse[materialrepo.Publication](o.RepositoryPublication)
				if e != nil {
					return "", e
				}
				if review.State != "open" {
					return "", materialError(409, "material_review_changed")
				}
				if r.Candidate != nil {
					e = materialrepo.ValidatePublication(*r.Candidate, review.HeadSHA, publication)
				} else if r.Maintenance != nil {
					e = materialrepo.ValidateMaintenancePublication(*r.Maintenance, review.HeadSHA, publication)
				} else {
					return "", materialError(409, "material_initialization_changed")
				}
				if e != nil {
					return "", materialError(409, "material_repository_content_changed")
				}
			}
			switch action {
			case "publish":
				if review.State != "open" {
					return "", materialError(409, "material_merge_not_authorized")
				}
				r.Review = review
				state.Status = "in_review"
			case "review_sync":
				if d.Action == "observe" && o.Outcome == "not_found" && o.AuditNote == nil {
					operationState = "no_effect"
					break
				}
				if o.AuditNote == nil || d.AuditNote == nil || *o.AuditNote == (protocol.MaterialAuditNote{}) || o.AuditNote.Marker != d.AuditNote.Marker || o.AuditNote.Body != d.AuditNote.Body || o.AuditNote.Actor == "" {
					return "", materialError(409, "material_review_sync_unconfirmed")
				}
				_, decision, e := repositoryDecisionCurrent(ctx, q, revision.ID)
				if e != nil {
					return "", e
				}
				if decision.Decision == "approve" {
					state.Status = "merging"
					err = repositoryJob(ctx, q, root, revision.ID, "merge")
				}
			case "merge":
				if review.State == "open" {
					operationState = "preflight"
					jobState = "dispatched"
					if d.Action == "observe" {
						operationState = "no_effect"
						jobState = "done"
					}
				} else {
					permit := op
					if d.Action == "observe" {
						permit, err = q.GetMaterialPermittedMerge(ctx, op.JobID)
						if err != nil {
							return "", err
						}
					}
					original, e := materialParse[protocol.MaterialExecution](permit.Descriptor)
					if e != nil || original.DecisionID != d.DecisionID || original.ApprovedHead != d.ApprovedHead || original.RepositoryRevisionID != d.RepositoryRevisionID {
						return "", materialError(409, "material_merge_not_authorized")
					}
					preflight, e := materialParse[protocol.MaterialObservation](permit.Receipt)
					if e != nil || preflight.Review == nil || preflight.Review.State != "open" || preflight.Review.HeadSHA != review.HeadSHA || preflight.Review.Number != review.Number || preflight.Review.URL != review.URL || review.MergeActor == "" || !material.IsSHA(review.MergeSHA) || !permit.PermitExpiresAt.Valid || !preflight.Review.GatesPassed || permit.PermitRequestID == "" || !materialMergeWithinPermit(permit.PermitExpiresAt.Time, o.ObservedAt, review) || material.JSONDigest(preflight.Review.Changes) != material.JSONDigest(review.Changes) {
						return "", materialError(409, "material_merge_not_authorized")
					}
					var proof materialrepo.Proof
					if r.Candidate != nil {
						proof, err = materialrepo.VerifyMerged(*r.Candidate, review.MergeSHA, snapshot)
					} else {
						proof, err = materialrepo.VerifyMaintenanceMerged(*r.Maintenance, review.MergeSHA, snapshot)
					}
					if err != nil {
						return "", materialError(409, "material_repository_content_changed")
					}
					if state.Proof != nil && material.JSONDigest(state.Proof) != material.JSONDigest(proof) {
						v := materialrepo.SchemaVersion
						state.History = append(state.History, material.RepositoryReadiness{VerifiedCommit: materialPtr(state.Proof.Commit), ManifestSHA256: materialPtr(state.Proof.CatalogDigest), SchemaVersion: &v, BundleVersion: materialPtr(state.BundleVersion), VerifiedAt: state.VerifiedAt})
					}
					state.Proof = &proof
					state.BundleVersion = r.BundleVersion
					state.VerifiedAt = &o.ObservedAt
					state.Status = "ready"
					state.Blocker = ""
					r.Review = review
				}
			default:
				return "", materialError(400, "material_action_invalid")
			}
		}
	}
	if err != nil {
		return "", err
	}
	if err = q.SaveMaterialRepositoryRevision(ctx, db.SaveMaterialRepositoryRevisionParams{ID: revision.ID, Payload: materialJSON(r)}); err != nil {
		return "", err
	}
	if _, err = saveRepository(ctx, q, root, state); err != nil {
		return "", err
	}
	return operationState, q.CompleteMaterialOperationJob(ctx, db.CompleteMaterialOperationJobParams{ID: op.JobID, State: jobState, Blocker: materialText(state.Blocker)})
}

func verifyOperationRules(ctx context.Context, q *db.Queries, op db.GitMaterialOperation, d protocol.MaterialExecution, o protocol.MaterialObservation) error {
	if len(o.RulesSnapshot) == 0 || !o.SourceComplete || o.RepositoryIdentity != d.Provider+":"+d.InstanceURL+":"+d.Repository || o.ObservedAt.IsZero() || time.Since(o.ObservedAt) > 5*time.Minute || o.ObservedAt.After(time.Now().Add(time.Minute)) {
		return materialError(409, "material_repository_unverified")
	}
	proof, err := materialParse[materialrepo.Proof](d.RepositoryExecution.ReadyProof)
	if err != nil {
		return err
	}
	snapshot, err := materialParse[materialrepo.Snapshot](o.RulesSnapshot)
	if err != nil || !snapshot.Complete || !material.IsSHA(snapshot.Commit) {
		return materialError(409, "material_repository_unverified")
	}
	setting, err := q.GetMaterialSettings(ctx, db.GetMaterialSettingsParams{ProjectID: op.ProjectID, WorkspaceID: op.WorkspaceID})
	if err != nil {
		return err
	}
	b, err := materialParse[materialBinding](setting.Settings)
	if err != nil {
		return err
	}
	_, state, err := requireRepositoryReady(ctx, q, op.WorkspaceID, op.ProjectID, b)
	if err != nil {
		return err
	}
	if material.JSONDigest(state.Proof) != material.JSONDigest(proof) {
		return materialError(409, "material_repository_proof_changed")
	}
	if err = materialrepo.VerifyProtected(proof, snapshot); err != nil {
		var invalid *materialrepo.Error
		if errors.As(err, &invalid) && invalid.Code == "protected_rule_changed" {
			return materialError(409, "material_repository_rules_changed")
		}
		return materialError(409, "material_repository_unverified")
	}
	return nil
}
