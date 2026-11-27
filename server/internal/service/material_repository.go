package service

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/material"
	"github.com/multica-ai/multica/server/internal/materialrepo"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type repositoryState struct {
	Status        string                         `json:"status"`
	Blocker       string                         `json:"blocker,omitempty"`
	RevisionID    string                         `json:"revision_id"`
	Proof         *materialrepo.Proof            `json:"proof,omitempty"`
	BundleVersion string                         `json:"bundle_version,omitempty"`
	VerifiedAt    *time.Time                     `json:"verified_at,omitempty"`
	History       []material.RepositoryReadiness `json:"history,omitempty"`
}
type repositoryRevision struct {
	ConfigVersion int64                              `json:"config_version"`
	Kind          string                             `json:"kind"`
	BundleVersion string                             `json:"bundle_version"`
	Candidate     *materialrepo.Candidate            `json:"candidate,omitempty"`
	Maintenance   *materialrepo.MaintenanceChangeSet `json:"maintenance,omitempty"`
	Inspection    *materialrepo.Inspection           `json:"inspection,omitempty"`
	Snapshot      *materialrepo.Snapshot             `json:"snapshot,omitempty"`
	Review        *protocol.MaterialReview           `json:"review,omitempty"`
}

func repositoryBindingDigest(b materialBinding) string {
	return material.JSONDigest(struct{ Resource, Runtime, Provider, Instance, Repository, Base, Root string }{b.ResourceDigest, b.RuntimeDigest, b.Provider, b.InstanceURL, b.Repository, b.BaseBranch, b.RootPath})
}
func repositoryPayload(row db.ProjectMaterialRepository) (repositoryState, error) {
	return materialParse[repositoryState](row.Payload)
}
func saveRepository(ctx context.Context, q *db.Queries, row db.ProjectMaterialRepository, state repositoryState) (db.ProjectMaterialRepository, error) {
	return q.SaveMaterialRepository(ctx, db.SaveMaterialRepositoryParams{ID: row.ID, Generation: row.Generation, ConfigVersion: row.ConfigVersion, BindingDigest: row.BindingDigest, Payload: materialJSON(state)})
}
func repositoryJob(ctx context.Context, q *db.Queries, row db.ProjectMaterialRepository, revision pgtype.UUID, kind string) error {
	return q.EnsureMaterialRepositoryJob(ctx, db.EnsureMaterialRepositoryJobParams{ID: dbid.NewV7(), WorkspaceID: row.WorkspaceID, ProjectID: row.ProjectID, RepositoryID: row.ID, RepositoryRevisionID: revision, Kind: kind})
}
func newRepositoryInspection(ctx context.Context, q *db.Queries, row db.ProjectMaterialRepository, state repositoryState, kind, bundle string) (db.ProjectMaterialRepository, error) {
	id := dbid.NewV7()
	_, err := q.CreateMaterialRepositoryRevision(ctx, db.CreateMaterialRepositoryRevisionParams{ID: id, RepositoryID: row.ID, WorkspaceID: row.WorkspaceID, ProjectID: row.ProjectID, Generation: row.Generation, Payload: materialJSON(repositoryRevision{Kind: kind, BundleVersion: bundle, ConfigVersion: row.ConfigVersion})})
	if err != nil {
		return row, err
	}
	state.RevisionID = util.UUIDToString(id)
	state.Status = "checking"
	state.Blocker = ""
	row, err = saveRepository(ctx, q, row, state)
	if err != nil {
		return row, err
	}
	return row, repositoryJob(ctx, q, row, id, "inspect_repository")
}
func ensureRepositoryConfiguration(ctx context.Context, q *db.Queries, ws, project pgtype.UUID, b materialBinding) error {
	if !b.Enabled {
		return nil
	}
	row, err := q.GetMaterialRepository(ctx, db.GetMaterialRepositoryParams{ProjectID: project, WorkspaceID: ws})
	digest := repositoryBindingDigest(b)
	if errors.Is(err, pgx.ErrNoRows) {
		row, err = q.CreateMaterialRepository(ctx, db.CreateMaterialRepositoryParams{ID: dbid.NewV7(), WorkspaceID: ws, ProjectID: project, ConfigVersion: b.Version, BindingDigest: digest, Payload: materialJSON(repositoryState{Status: "checking"})})
		if err != nil {
			return err
		}
		_, err = newRepositoryInspection(ctx, q, row, repositoryState{}, "initialize", materialrepo.BundleVersion)
		return err
	}
	if err != nil {
		return err
	}
	state, err := repositoryPayload(row)
	if err != nil {
		return err
	}
	if row.BindingDigest == digest {
		if row.ConfigVersion != b.Version {
			row.ConfigVersion = b.Version
			_, err = saveRepository(ctx, q, row, state)
		}
		return err
	}
	if state.Proof != nil {
		v := materialrepo.SchemaVersion
		state.History = append(state.History, material.RepositoryReadiness{VerifiedCommit: materialPtr(state.Proof.Commit), ManifestSHA256: materialPtr(state.Proof.CatalogDigest), SchemaVersion: &v, BundleVersion: materialPtr(state.BundleVersion), VerifiedAt: state.VerifiedAt})
	}
	row.Generation++
	row.ConfigVersion = b.Version
	row.BindingDigest = digest
	state.Proof = nil
	state.VerifiedAt = nil
	_, err = newRepositoryInspection(ctx, q, row, state, "initialize", materialrepo.BundleVersion)
	return err
}
func requireRepositoryReady(ctx context.Context, q *db.Queries, ws, project pgtype.UUID, b materialBinding) (db.ProjectMaterialRepository, repositoryState, error) {
	row, err := q.GetMaterialRepository(ctx, db.GetMaterialRepositoryParams{ProjectID: project, WorkspaceID: ws})
	if errors.Is(err, pgx.ErrNoRows) {
		return row, repositoryState{}, materialError(409, "material_repository_not_ready")
	}
	if err != nil {
		return row, repositoryState{}, err
	}
	state, err := repositoryPayload(row)
	if err != nil {
		return row, state, err
	}
	if row.BindingDigest != repositoryBindingDigest(b) || state.Proof == nil || state.Blocker == "material_repository_rules_changed" {
		return row, state, materialError(409, "material_repository_not_ready")
	}
	return row, state, nil
}
func repositoryExecutionProof(ctx context.Context, q *db.Queries, ws, project pgtype.UUID, b materialBinding) (*protocol.MaterialRepositoryExecution, error) {
	row, state, err := requireRepositoryReady(ctx, q, ws, project, b)
	if err != nil {
		return nil, err
	}
	return &protocol.MaterialRepositoryExecution{SystemID: util.UUIDToString(row.ID), RootPath: b.RootPath, BundleVersion: state.BundleVersion, CatalogDigest: state.Proof.CatalogDigest, ReadyProof: materialJSON(state.Proof)}, nil
}
func (t *materialTx) repositoryView(ctx context.Context) (material.Initialization, error) {
	out := material.Initialization{WorkspaceID: util.UUIDToString(t.access.WorkspaceID), ProjectID: util.UUIDToString(t.access.ProjectID), ProtocolVersion: 1, Status: "unconfigured", Decisions: []material.InitializationDecision{}, AvailableBundleVersions: materialrepo.AvailableBundleVersions()}
	_, b, err := t.binding(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	out.SettingsVersion = b.Version
	row, err := t.q.GetMaterialRepository(ctx, db.GetMaterialRepositoryParams{ProjectID: t.access.ProjectID, WorkspaceID: t.access.WorkspaceID})
	if errors.Is(err, pgx.ErrNoRows) {
		out.Capabilities.CanCheck = t.manager()
		return out, nil
	}
	if err != nil {
		return out, err
	}
	state, err := repositoryPayload(row)
	if err != nil {
		return out, err
	}
	out.History = state.History
	out.Version = row.Version
	out.BindingGeneration = row.Generation
	out.Status = state.Status
	out.Blocker = materialPtr(state.Blocker)
	if state.Proof != nil {
		v := materialrepo.SchemaVersion
		out.Readiness = material.RepositoryReadiness{PublishAllowed: b.Enabled && row.BindingDigest == repositoryBindingDigest(b) && state.Blocker != "material_repository_rules_changed", VerifiedCommit: materialPtr(state.Proof.Commit), ManifestSHA256: materialPtr(state.Proof.CatalogDigest), SchemaVersion: &v, BundleVersion: materialPtr(state.BundleVersion), VerifiedAt: state.VerifiedAt}
	}
	out.Capabilities = material.InitializationCapabilities{CanCheck: t.manager() && b.Enabled, CanRetry: t.manager() && b.Enabled, CanMaintain: t.manager() && out.Readiness.PublishAllowed}
	if state.RevisionID != "" {
		rev, err := t.q.GetMaterialRepositoryRevision(ctx, util.MustParseUUID(state.RevisionID))
		if err != nil {
			return out, err
		}
		r, err := materialParse[repositoryRevision](rev.Payload)
		if err != nil {
			return out, err
		}
		if r.Candidate != nil || r.Maintenance != nil || r.Inspection != nil {
			c := material.InitializationCandidate{ID: state.RevisionID, Kind: r.Kind, Version: row.Version, BindingGeneration: row.Generation, BundleVersion: r.BundleVersion, PublicationStatus: state.Status, Files: []material.InitializationFile{}}
			if r.Candidate != nil {
				c.ManifestSHA256 = r.Candidate.Digest
				c.BaseCommit = r.Candidate.Before.Commit
				for _, f := range r.Candidate.Files {
					c.Files = append(c.Files, material.InitializationFile{Path: f.Path, Action: "add", AfterSHA256: materialPtr(f.SHA256), AfterContent: materialPtr(f.Content)})
				}
			}
			if r.Maintenance != nil {
				c.ManifestSHA256 = r.Maintenance.Digest
				c.BaseCommit = r.Maintenance.Before.Commit
				for _, f := range r.Maintenance.Changes {
					c.Files = append(c.Files, material.InitializationFile{Path: f.After.Path, Action: "modify", BeforeSHA256: materialPtr(f.Before.SHA256), BeforeContent: f.Before.Content, AfterSHA256: materialPtr(f.After.SHA256), AfterContent: materialPtr(f.After.Content)})
				}
			}
			if r.Snapshot != nil {
				c.BaseCommit = r.Snapshot.Commit
			}
			if r.Inspection != nil {
				for _, finding := range r.Inspection.Findings {
					if finding.State == "missing" {
						continue
					}
					entry := material.InitializationFile{Path: finding.Path, Action: "preserve", Explanation: materialPtr(finding.Reason)}
					if finding.State == "conflict" {
						entry.Action = "conflict"
					}
					if r.Snapshot != nil {
						for _, before := range r.Snapshot.Entries {
							if before.Path == finding.Path {
								entry.BeforeSHA256 = materialPtr(before.SHA256)
								entry.BeforeContent = before.Content
								entry.AfterSHA256 = entry.BeforeSHA256
								entry.AfterContent = before.Content
							}
						}
					}
					c.Files = append(c.Files, entry)
				}
			}
			if r.Review != nil {
				c.HeadSHA = materialPtr(r.Review.HeadSHA)
				c.ReviewURL = materialPtr(r.Review.URL)
			}
			out.Candidate = &c
			out.Capabilities.CanReview = t.manager() && b.Enabled && row.BindingDigest == repositoryBindingDigest(b) && rev.Generation == row.Generation && r.Review != nil && r.Review.State == "open" && state.Status != "ready"
		}
	}
	decisions, err := t.q.ListMaterialRepositoryDecisions(ctx, row.ID)
	if err != nil {
		return out, err
	}
	for _, d := range decisions {
		v, err := materialParse[material.InitializationDecision](d.Payload)
		if err != nil {
			return out, err
		}
		out.Decisions = append(out.Decisions, v)
	}
	if op, err := t.q.CurrentMaterialRepositoryOperation(ctx, row.ID); err == nil {
		status := op.State
		if status == "queued" || status == "dispatched" {
			status = op.TaskStatus
			if status == "failed" || status == "cancelled" || status == "completed" {
				status = "unknown"
			}
		}
		out.Operation = &material.Operation{ID: util.UUIDToString(op.ID), TaskID: util.UUIDToString(op.TaskID), Kind: op.Action, Status: status, ExecutionRuntimeID: util.UUIDToString(op.RuntimeID), Blocker: materialPtr(op.Blocker.String)}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return out, err
	}
	return out, nil
}
func (s *MaterialService) Initialization(ctx context.Context, a MaterialAccess) (material.Initialization, error) {
	t, err := s.begin(ctx, a, false)
	if err != nil {
		return material.Initialization{}, err
	}
	defer t.tx.Rollback(ctx)
	out, err := t.repositoryView(ctx)
	if err != nil {
		return out, err
	}
	return out, t.tx.Commit(ctx)
}
func (s *MaterialService) InitializationCommand(ctx context.Context, a MaterialAccess, action string, in material.InitializationCommand) (material.Initialization, error) {
	var out material.Initialization
	t, err := s.begin(ctx, a, true)
	if err != nil {
		return out, err
	}
	defer t.tx.Rollback(ctx)
	if !t.manager() {
		return out, materialError(403, "material_human_manager_required")
	}
	saved, hash, err := t.request(ctx, in.RequestID, "initialization:"+action, in)
	if err != nil {
		return out, err
	}
	if saved != nil {
		return materialParse[material.Initialization](saved)
	}
	_, b, err := t.binding(ctx)
	if err != nil {
		return out, err
	}
	if b.Version != in.ExpectedSettingsVersion {
		return out, materialError(409, "material_configuration_changed")
	}
	if !b.Enabled {
		return out, materialError(409, "material_disabled")
	}
	row, err := t.q.GetMaterialRepository(ctx, db.GetMaterialRepositoryParams{ProjectID: a.ProjectID, WorkspaceID: a.WorkspaceID})
	if errors.Is(err, pgx.ErrNoRows) && in.ExpectedVersion == 0 {
		if err = ensureRepositoryConfiguration(ctx, t.q, a.WorkspaceID, a.ProjectID, b); err != nil {
			return out, err
		}
	} else if err != nil {
		return out, err
	} else {
		if row.Version != in.ExpectedVersion {
			return out, materialError(409, "material_version_changed")
		}
		state, err := repositoryPayload(row)
		if err != nil {
			return out, err
		}
		switch action {
		case "maintenance":
			if state.Status == "checking" || state.Status == "publishing" || state.Status == "in_review" || state.Status == "approved" || state.Status == "merging" || state.Status == "changes_requested" {
				break // Retain the pending inspection/candidate, including its queued job.
			}
			if op, e := t.q.CurrentMaterialRepositoryOperation(ctx, row.ID); e == nil && op.State != "succeeded" && op.State != "no_effect" {
				return out, materialError(409, "material_repository_recovery_required")
			} else if e != nil && !errors.Is(e, pgx.ErrNoRows) {
				return out, e
			}
			if _, e := materialrepo.BuildCatalogVersion(materialrepo.Binding{SystemID: util.UUIDToString(row.ID), RootPath: b.RootPath}, in.TargetBundleVersion); e != nil {
				return out, materialError(422, "material_repository_bundle_unsupported")
			}
			if _, _, err = requireRepositoryReady(ctx, t.q, a.WorkspaceID, a.ProjectID, b); err != nil {
				return out, err
			}
			if state.BundleVersion != in.TargetBundleVersion {
				_, err = newRepositoryInspection(ctx, t.q, row, state, "maintenance", in.TargetBundleVersion)
			}
		case "retry":
			op, e := t.q.CurrentMaterialRepositoryOperation(ctx, row.ID)
			if e == nil && util.UUIDToString(op.RepositoryRevisionID) != state.RevisionID {
				err = repositoryJob(ctx, t.q, row, util.MustParseUUID(state.RevisionID), "inspect_repository")
				break
			}
			if e == nil {
				if op.TaskStatus == "queued" || op.TaskStatus == "running" || op.TaskStatus == "dispatched" {
					break
				}
				_, err = t.q.RecoverMaterialRepositoryOperation(ctx, row.ID)
				if err == nil {
					err = t.q.CompleteMaterialOperationJob(ctx, db.CompleteMaterialOperationJobParams{ID: op.JobID, State: "pending"})
				}
			} else if errors.Is(e, pgx.ErrNoRows) {
				// Dispatch can fail before creating an operation (offline runtime,
				// revoked authorization). Retry the existing inspection intent.
				err = repositoryJob(ctx, t.q, row, util.MustParseUUID(state.RevisionID), "inspect_repository")
			} else {
				err = e
			}
		case "check":
			if state.Status == "checking" {
				if op, e := t.q.CurrentMaterialRepositoryOperation(ctx, row.ID); errors.Is(e, pgx.ErrNoRows) || e == nil && util.UUIDToString(op.RepositoryRevisionID) != state.RevisionID {
					break
				}
			}
			if state.RevisionID != "" {
				revision, e := t.q.GetMaterialRepositoryRevision(ctx, util.MustParseUUID(state.RevisionID))
				if e != nil {
					return out, e
				}
				candidate, e := materialParse[repositoryRevision](revision.Payload)
				if e != nil {
					return out, e
				}
				// A completed publish/note task does not settle its review. Keep
				// that candidate instead of creating a competing branch or PR.
				if (candidate.Candidate != nil || candidate.Maintenance != nil) && (candidate.Review == nil || candidate.Review.State != "merged") {
					break
				}
			}
			op, e := t.q.CurrentMaterialRepositoryOperation(ctx, row.ID)
			if e != nil && !errors.Is(e, pgx.ErrNoRows) {
				return out, e
			}
			if e == nil {
				if op.TaskStatus == "queued" || op.TaskStatus == "running" || op.TaskStatus == "dispatched" {
					break
				}
				if op.State != "succeeded" && op.State != "no_effect" && op.Action != "inspect_repository" {
					return out, materialError(409, "material_repository_recovery_required")
				}
			}
			_, err = newRepositoryInspection(ctx, t.q, row, state, "initialize", materialrepo.BundleVersion)
		default:
			return out, materialError(400, "material_initialization_action_invalid")
		}
		if err != nil {
			return out, err
		}
	}
	out, err = t.repositoryView(ctx)
	if err != nil {
		return out, err
	}
	if err = t.saveRequest(ctx, in.RequestID, hash, out); err != nil {
		return out, err
	}
	return out, t.tx.Commit(ctx)
}
func (s *MaterialService) InitializationDecide(ctx context.Context, a MaterialAccess, in material.InitializationDecisionInput) (material.Initialization, error) {
	var out material.Initialization
	t, err := s.begin(ctx, a, true)
	if err != nil {
		return out, err
	}
	defer t.tx.Rollback(ctx)
	if !t.manager() {
		return out, materialError(403, "material_human_manager_required")
	}
	if (in.Decision != "approve" && in.Decision != "request_changes" && in.Decision != "withdraw") || !material.IsSHA(in.HeadSHA) || len(in.Reason) > 4000 {
		return out, materialError(400, "invalid_material_decision")
	}
	saved, hash, err := t.request(ctx, in.RequestID, "initialization:decision", in)
	if err != nil {
		return out, err
	}
	if saved != nil {
		return materialParse[material.Initialization](saved)
	}
	out, err = t.repositoryView(ctx)
	if err != nil {
		return out, err
	}
	if out.Version != in.ExpectedVersion || out.SettingsVersion != in.ExpectedSettingsVersion || out.Candidate == nil || out.Candidate.ID != in.CandidateID || out.Candidate.ManifestSHA256 != in.ManifestSHA256 || out.Candidate.HeadSHA == nil || *out.Candidate.HeadSHA != in.HeadSHA || !out.Capabilities.CanReview {
		return out, materialError(409, "material_initialization_changed")
	}
	row, err := t.q.GetMaterialRepository(ctx, db.GetMaterialRepositoryParams{ProjectID: a.ProjectID, WorkspaceID: a.WorkspaceID})
	if err != nil {
		return out, err
	}
	state, err := repositoryPayload(row)
	if err != nil {
		return out, err
	}
	decision := material.InitializationDecision{ID: util.UUIDToString(dbid.NewV7()), CandidateID: in.CandidateID, ActorID: util.UUIDToString(t.actor), Decision: in.Decision, Reason: in.Reason, HeadSHA: in.HeadSHA, ManifestSHA256: in.ManifestSHA256, CreatedAt: time.Now()}
	decision.SettingsVersion = out.SettingsVersion
	decision.BindingGeneration = row.Generation
	credential := stageSignerCredential{Version: 1, Kind: a.Credential.Principal.Kind, PasswordSession: a.Credential.PasswordSession, PATHash: a.Credential.Principal.PATTokenHash, SourceMemberID: util.UUIDToString(t.member.ID)}
	if credential.Kind == auth.CredentialJWT {
		at := a.Credential.Principal.ExpiresAt
		credential.ExpiresAt = &at
	}
	_, err = t.q.CreateMaterialRepositoryDecision(ctx, db.CreateMaterialRepositoryDecisionParams{ID: util.MustParseUUID(decision.ID), RepositoryID: row.ID, RevisionID: util.MustParseUUID(in.CandidateID), WorkspaceID: a.WorkspaceID, ProjectID: a.ProjectID, ActorID: t.actor, MemberID: t.member.ID, Credential: materialJSON(credential), Payload: materialJSON(decision)})
	if err != nil {
		return out, err
	}
	state.Status = "changes_requested"
	if in.Decision == "approve" {
		state.Status = "approved"
	}
	if _, err = saveRepository(ctx, t.q, row, state); err != nil {
		return out, err
	}
	if err = repositoryJob(ctx, t.q, row, util.MustParseUUID(in.CandidateID), "review_sync"); err != nil {
		return out, err
	}
	out, err = t.repositoryView(ctx)
	if err != nil {
		return out, err
	}
	if err = t.saveRequest(ctx, in.RequestID, hash, out); err != nil {
		return out, err
	}
	return out, t.tx.Commit(ctx)
}
