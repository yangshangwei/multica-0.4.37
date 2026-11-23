package service

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/storage"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// Pure selection boundaries live in collaboration_runtime_preparation_selection_test.go.
// This matrix composes registration, whole-roster admission, member claims and
// actual private attachment reads. No provider or native process is started.
func TestCollaborationSquadPreparationBudgetMatrix(t *testing.T) {
	for _, dimension := range []string{"file_count", "per_file_bytes", "total_input_bytes", "workspace_bytes"} {
		t.Run(dimension, func(t *testing.T) {
			for _, mandatory := range []bool{false, true} {
				name := "optional_history"
				if mandatory {
					name = "mandatory_refusal"
				}
				t.Run(name, func(t *testing.T) {
					leader := newManagedCollaborationFixture(t)
					limited := ownerSwitchNamespace(t, leader, leader.creator, "Limited member")
					roomy := ownerSwitchNamespace(t, leader, leader.creator, "Roomy member")
					resources := []*managedCollaborationFixture{leader, limited, roomy}
					caps := make(map[pgtype.UUID]protocol.CollaborationRuntimeCapabilities)
					for _, resource := range resources {
						request := preparationRegistrationRequest(resource)
						request.Capabilities.Limits = protocol.CollaborationPreparationLimits{FileCount: 3, PerFileBytes: 10,
							TotalInputBytes: 20, WorkspaceBytes: 30, GitExpandedBytes: 6, EntryCount: 16, TimeoutSeconds: 60}
						if dimension == "workspace_bytes" {
							request.Capabilities.Git = &protocol.CollaborationPreparationGit{ObjectFormat: "sha1", CommitOID: strings.Repeat("b", 40), BundleDigest: strings.Repeat("c", 64)}
						}
						if resource == limited {
							switch dimension {
							case "file_count":
								request.Capabilities.Limits.FileCount = 2
							case "per_file_bytes":
								request.Capabilities.Limits.PerFileBytes = 5
							case "total_input_bytes":
								request.Capabilities.Limits.TotalInputBytes = 8
							case "workspace_bytes":
								request.Capabilities.Limits.TotalInputBytes = 12
								request.Capabilities.Limits.WorkspaceBytes = 12
							}
						}
						if _, err := leader.svc.RegisterRuntimeCapabilities(resource.daemonCtx, request); err != nil {
							t.Fatal(err)
						}
						resource.runtime = mustRuntime(t, leader.svc.Queries, resource.runtime.ID)
						caps[resource.runtime.ID] = request.Capabilities
					}
					// Change roster order too: selection must not borrow the preceding
					// runtime's limits or carry the preceding member's byte count.
					members := []*managedCollaborationFixture{limited, roomy}
					if dimension == "per_file_bytes" || dimension == "workspace_bytes" {
						members[0], members[1] = members[1], members[0]
					}
					squad := leader.fx.Squad(t, "Per-member preparation budgets", util.UUIDToString(leader.grant.AgentID))
					roster := []collaboration.TargetResource{ownerSwitchResource(leader, "leader")}
					for _, member := range members {
						leader.fx.SquadMember(t, squad, "agent", util.UUIDToString(member.grant.AgentID))
						roster = append(roster, ownerSwitchResource(member, "member"))
					}
					target := ownerSwitchRegister(t, leader, collaboration.TargetSquad, util.MustParseUUID(squad), roster)
					for _, resource := range resources {
						resource.targetID = target.ID
						resource.grant = ownerSwitchConsent(t, resource, target)
					}
					leader.fx.Cleanup(t, `DELETE FROM collaboration_delegation WHERE group_id IN (SELECT id FROM collaboration_execution_group WHERE session_id=$1)`, leader.issue.SessionID)

					t.Setenv("LOCAL_UPLOAD_DIR", t.TempDir())
					t.Setenv("LOCAL_UPLOAD_BASE_URL", "")
					leader.svc.Storage = storage.NewLocalStorageFromEnv()
					files := make([]CollaborationAttachmentView, 0, 3)
					for _, body := range []string{"base", "larger", "ok"} {
						files = append(files, squadBudgetDeliveredFile(t, leader, body))
					}
					references := []pgtype.UUID{files[0].ID}
					if mandatory {
						references = append(references, files[1].ID, files[2].ID)
					}
					input, err := leader.svc.SubmitCommand(leader.ctx, CollaborationSubmitCommandParams{CollaborationQueueRequest: leader.request(),
						TargetID: target.ID, InputEventID: util.MustParseUUID(uuid.NewString()), ExpectedTargetEpoch: target.TargetEpoch,
						ExpectedConfigVersion: target.ConfigVersion, Content: "Preserve the referenced source material", AttachmentIDs: references})
					if err != nil {
						t.Fatal(err)
					}
					leader.resumeManaged(t)
					listed, err := leader.svc.ListRuntimeRuns(leader.daemonCtx, leader.runtime.ID)
					if err != nil {
						t.Fatal(err)
					}
					if mandatory {
						if len(listed.RunIDs) != 0 || leader.fx.Count(t, `SELECT count(*) FROM collaboration_run WHERE session_id=$1`, leader.issue.SessionID) != 0 ||
							leader.fx.Count(t, `SELECT count(*) FROM collaboration_execution_group WHERE session_id=$1`, leader.issue.SessionID) != 0 {
							t.Fatal("one member's mandatory budget overflow created a run/group before refusal")
						}
						for _, resource := range resources {
							if leader.fx.Count(t, `SELECT count(*) FROM collaboration_runtime_reservation WHERE runtime_id=$1`, resource.runtime.ID) != 0 {
								t.Fatal("mandatory member overflow reserved runtime capacity")
							}
						}
						if leader.fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE id=$1 AND state='waiting' AND group_id IS NULL`, input.CommandID) != 1 {
							t.Fatal("refusal consumed or lost the original input")
						}
						// The public poll deliberately withholds refused candidates. The
						// transaction owner supplies the precise admission reason.
						_, err := collaborationRuntimeMutation(leader.daemonCtx, leader.svc, func(tx pgx.Tx) (collaborationRuntimePromotion, error) {
							return leader.svc.promoteCollaborationRuntimeBatch(leader.daemonCtx, tx, target.SessionID, target.ID, leader.runtime.ID)
						})
						if !collaborationErrorCode(err, "runtime_preparation_limits") {
							t.Fatalf("mandatory budget refusal reason: %v", err)
						}
						return
					}
					if len(listed.RunIDs) != 1 {
						t.Fatalf("optional files blocked the squad: candidates=%d", len(listed.RunIDs))
					}
					root := util.MustParseUUID(listed.RunIDs[0])
					_, rootClaim := claimPreparedCandidate(t, leader, root)
					_, rootPermit := permitPreparedClaim(t, leader, root, rootClaim)
					body := "[@limited](mention://agent/" + util.UUIDToString(limited.grant.AgentID) + ") [@roomy](mention://agent/" + util.UUIDToString(roomy.grant.AgentID) + ") inspect the sources"
					dispatch, err := leader.svc.CreateRuntimeComment(context.Background(), root, rootPermit.BusinessToken,
						CollaborationRuntimeCommentParams{CommentID: util.MustParseUUID(uuid.NewString()), Content: body})
					if err != nil || len(dispatch.RunIDs) != 2 {
						t.Fatalf("member dispatch: runs=%d err=%v", len(dispatch.RunIDs), err)
					}
					for _, runID := range dispatch.RunIDs {
						run, err := leader.svc.Queries.GetCollaborationRuntimeRun(leader.ctx, runID)
						if err != nil {
							t.Fatal(err)
						}
						member := roomy
						if run.AgentID == limited.grant.AgentID {
							member = limited
						} else if run.AgentID != roomy.grant.AgentID {
							t.Fatal("dispatch produced an unrelated member")
						}
						request, claim := claimPreparedCandidate(t, member, runID)
						wantIDs := map[string]bool{util.UUIDToString(files[0].ID): true, util.UUIDToString(files[2].ID): true}
						if member == roomy {
							wantIDs[util.UUIDToString(files[1].ID)] = true
						}
						if !reflect.DeepEqual(preparationManifestIDs(claim.Preparation), wantIDs) ||
							claim.Preparation.Limits != caps[member.runtime.ID].Limits || claim.Preparation.Resource.RuntimeID != util.UUIDToString(member.runtime.ID) ||
							len(claim.Inputs) != 1 || claim.Inputs[0].CommandID != util.UUIDToString(input.CommandID) ||
							!reflect.DeepEqual(claim.Inputs[0].AttachmentIDs, []string{util.UUIDToString(files[0].ID)}) {
							t.Fatal("member claim lost mandatory material or borrowed another runtime's budget/selection")
						}
						_, permit := permitPreparedClaim(t, member, runID, claim)
						manifest, err := member.svc.RuntimePreparation(context.Background(), runID, permit.BusinessToken)
						if err != nil || !reflect.DeepEqual(manifest, *claim.Preparation) {
							t.Fatalf("member preparation differs from its frozen claim: %v", err)
						}
						for _, file := range files {
							_, err := member.svc.RuntimeAttachment(context.Background(), runID, file.ID, file.Version, permit.BusinessToken)
							if wantIDs[util.UUIDToString(file.ID)] && err != nil || !wantIDs[util.UUIDToString(file.ID)] && !collaborationErrorCode(err, "attachment_not_found") {
								t.Fatalf("member attachment access disagrees with its manifest: %v", err)
							}
						}
						replay, err := member.svc.ClaimRuntimeRun(member.daemonCtx, runID, request)
						if err != nil || !reflect.DeepEqual(replay.Preparation, claim.Preparation) {
							t.Fatalf("claim replay changed the member budget/selection: %v", err)
						}
					}
				})
			}
		})
	}
}

func squadBudgetDeliveredFile(t *testing.T, f *managedCollaborationFixture, body string) CollaborationAttachmentView {
	t.Helper()
	p := attachmentUploadParams(f.collaborationFixture)
	p.SourceWorkspaceID, p.IssueID, p.Data = pgtype.UUID{}, f.issue.Issue.ID, []byte(body)
	file, err := f.svc.UploadAttachmentDraft(f.ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	f.fx.Cleanup(t, `DELETE FROM collaboration_attachment WHERE id=$1`, file.ID)
	f.fx.Cleanup(t, `DELETE FROM collaboration_attachment_reference WHERE attachment_id=$1`, file.ID)
	if _, err = f.svc.DeliverAttachments(f.ctx, CollaborationAttachmentDelivery{IssueID: f.issue.Issue.ID, DraftID: &p.DraftID,
		AttachmentIDs: []pgtype.UUID{file.ID}, RequestID: uuid.NewString()}); err != nil {
		t.Fatal(err)
	}
	return file
}
