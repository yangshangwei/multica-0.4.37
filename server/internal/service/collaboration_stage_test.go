package service

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
)

// collaborationStageFixture is one collaboration root inside a project. The
// stage policy is switched on only through the real policy revision service.
type collaborationStageFixture struct {
	*collaborationQueueFixture
	project                    string
	reviewer, qa, release      string
	policyRevision             int64
	reviewerCtx, handlerCtx    context.Context
	qaCtx, releaseCtx, machine context.Context
}

func stageCredential(user string) ArtifactCredentialSnapshot {
	return ArtifactCredentialSnapshot{Principal: auth.AuthenticatedPrincipal{UserID: user, Kind: auth.CredentialJWT, ExpiresAt: time.Now().Add(time.Hour)}}
}

func newCollaborationStageFixture(t *testing.T) *collaborationStageFixture {
	t.Helper()
	base := newCollaborationFixture(t)
	base.ctx = auth.WithAuthenticatedPrincipal(base.ctx, stageCredential(base.creator).Principal)
	if err := base.svc.Queries.SeedIssueStatusEntries(base.ctx, util.MustParseUUID(base.workspace)); err != nil {
		t.Fatal(err)
	}
	base.fx.Cleanup(t, `DELETE FROM issue_status WHERE workspace_id=$1`, base.workspace)
	project := base.fx.Project(t, "Stage governed project")
	p := base.params(t)
	p.ProjectID = util.MustParseUUID(project)
	issue, err := base.svc.Create(base.ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	fx := base.fx
	for _, statement := range []string{
		`DELETE FROM collaboration_operation WHERE scope_kind='issue' AND scope_id=$1`,
		`DELETE FROM issue_handoff WHERE issue_id=$1`,
	} {
		fx.Cleanup(t, statement, issue.Issue.ID)
	}
	for _, table := range []string{"collaboration_stage_child", "collaboration_stage_revocation", "collaboration_stage_decision", "collaboration_stage_candidate", "collaboration_stage_role", "collaboration_stage_task", "project_stage_policy"} {
		fx.Cleanup(t, `DELETE FROM `+table+` WHERE workspace_id=$1`, base.workspace)
	}
	fx.Cleanup(t, `DELETE FROM agent_task_queue WHERE issue_id IN (SELECT id FROM issue WHERE workspace_id=$1)`, base.workspace)
	f := &collaborationStageFixture{collaborationQueueFixture: &collaborationQueueFixture{collaborationFixture: base, issue: issue}, project: project}
	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")
	f.reviewer = fx.User(t, "Code reviewer", "reviewer-"+suffix+"@example.test")
	f.qa = fx.User(t, "QA approver", "qa-"+suffix+"@example.test")
	f.release = fx.User(t, "Release approver", "release-"+suffix+"@example.test")
	fx.Cleanup(t, `DELETE FROM collaboration_account_fence WHERE user_id=ANY($1::uuid[])`, []string{f.reviewer, f.qa, f.release})
	for _, user := range []string{f.reviewer, f.qa, f.release} {
		if _, err := f.svc.InviteParticipant(f.ctx, CollaborationParticipantParams{CollaborationWorkflowRequest: f.req(t, f.ctx), TargetUserID: util.MustParseUUID(user)}); err != nil {
			t.Fatal(err)
		}
	}
	f.handlerCtx = auth.WithAuthenticatedPrincipal(context.Background(), stageCredential(f.handler).Principal)
	f.reviewerCtx = auth.WithAuthenticatedPrincipal(context.Background(), stageCredential(f.reviewer).Principal)
	f.qaCtx = auth.WithAuthenticatedPrincipal(context.Background(), stageCredential(f.qa).Principal)
	f.releaseCtx = auth.WithAuthenticatedPrincipal(context.Background(), stageCredential(f.release).Principal)
	f.machine = auth.WithAuthenticatedPrincipal(context.Background(), auth.AuthenticatedPrincipal{UserID: f.creator, Kind: auth.CredentialTask})
	return f
}

// req reads the actor's current observations, as a client would before a write.
func (f *collaborationStageFixture) req(t *testing.T, ctx context.Context) CollaborationWorkflowRequest {
	t.Helper()
	access, err := f.svc.ResolveIssueAccess(ctx, nil, f.issue.Issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	return CollaborationWorkflowRequest{CollaborationQueueRequest: CollaborationQueueRequest{IssueID: f.issue.Issue.ID, RequestID: uuid.NewString(),
		ExpectedLifecycleEpoch: access.LifecycleEpoch, ExpectedParticipantEpoch: access.ParticipantEpoch.Int64, ExpectedAccountEpoch: access.AccountEpoch.Int64},
		ExpectedCollaborationRevision: access.Revision}
}

func (f *collaborationStageFixture) revise(t *testing.T, enabled bool) {
	t.Helper()
	receipt, err := f.svc.ReviseStagePolicy(f.ctx, StagePolicyRevisionParams{WorkspaceID: util.MustParseUUID(f.workspace), ProjectID: util.MustParseUUID(f.project),
		RequestID: uuid.NewString(), ExpectedRevision: f.policyRevision, Enabled: enabled, Reason: "test policy revision", Credential: stageCredential(f.creator)})
	if err != nil {
		t.Fatal(err)
	}
	f.policyRevision++
	if receipt.StageConfirmation.PolicyRevision != stageVersion(f.policyRevision) {
		t.Fatalf("policy revision receipt: %+v", receipt)
	}
}

func (f *collaborationStageFixture) roleSet() CollaborationStageRoleSet {
	id := util.MustParseUUID
	return CollaborationStageRoleSet{BusinessOwner: id(f.creator), Implementer: id(f.handler), ProductApprover: id(f.creator),
		CodeReviewer: id(f.reviewer), QAApprover: id(f.qa), ReleaseApprover: id(f.release)}
}

func (f *collaborationStageFixture) view(t *testing.T) CollaborationStageView {
	t.Helper()
	view, err := f.svc.StageView(f.ctx, f.issue.Issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	return view
}

func (f *collaborationStageFixture) configure(t *testing.T) int64 {
	t.Helper()
	current := f.stageInt(t, `SELECT COALESCE((SELECT role_revision FROM collaboration_stage_task WHERE issue_id=$1),0)`)
	receipt, err := f.svc.ConfigureStageRoles(f.ctx, CollaborationStageRolesParams{CollaborationWorkflowRequest: f.req(t, f.ctx),
		ExpectedPolicyRevision: f.policyRevision, ExpectedRoleRevision: current, Roles: f.roleSet()})
	if err != nil || receipt.Code != "stage_roles_configured" {
		t.Fatalf("configure roles: %+v %v", receipt, err)
	}
	return current + 1
}

func (f *collaborationStageFixture) stageInt(t *testing.T, query string) int64 {
	t.Helper()
	var value int64
	f.fx.QueryRow(t, query, f.issue.Issue.ID).Scan(&value)
	return value
}

func (f *collaborationStageFixture) confirmScope(t *testing.T, children ...CollaborationStageScopeChild) CollaborationWorkflowReceipt {
	t.Helper()
	receipt, err := f.svc.ConfirmStageScope(f.ctx, CollaborationStageScopeParams{CollaborationWorkflowRequest: f.req(t, f.ctx),
		ExpectedPolicyRevision: f.policyRevision, ExpectedRoleRevision: f.stageInt(t, `SELECT role_revision FROM collaboration_stage_task WHERE issue_id=$1`),
		ExpectedScopeRevision: f.stageInt(t, `SELECT scope_revision FROM collaboration_stage_task WHERE issue_id=$1`), Children: children})
	if err != nil || receipt.Code != "stage_scope_confirmed" {
		t.Fatalf("confirm scope: %+v %v", receipt, err)
	}
	return receipt
}

func stageContent(titles ...string) collaboration.StageProposalContent {
	content := collaboration.StageProposalContent{Summary: "Split the delivery"}
	for _, title := range titles {
		content.Children = append(content.Children, collaboration.StageProposalChild{Title: title, Description: "Scope of " + title, Priority: "medium"})
	}
	return content
}

func (f *collaborationStageFixture) propose(t *testing.T, ctx context.Context, titles ...string) CollaborationStageConfirmation {
	t.Helper()
	receipt, err := f.svc.CreateStageProposal(ctx, CollaborationStageProposalParams{CollaborationWorkflowRequest: f.req(t, ctx),
		ExpectedPolicyRevision: f.policyRevision, Content: stageContent(titles...), Source: CollaborationStageProposalSource{Kind: "manual"}})
	if err != nil || receipt.Code != "stage_proposal_created" || receipt.StageConfirmation == nil || !receipt.StageConfirmation.valid(stageKindCreateProposal) {
		t.Fatalf("create proposal: %+v %v", receipt, err)
	}
	return *receipt.StageConfirmation
}

func stageRef(c CollaborationStageConfirmation) CollaborationStageCandidateRef {
	version := int64(0)
	for _, digit := range c.Version {
		version = version*10 + int64(digit-'0')
	}
	return CollaborationStageCandidateRef{CandidateID: util.MustParseUUID(c.CandidateID), Version: version, ContentDigest: c.ContentDigest}
}

func (f *collaborationStageFixture) decision(t *testing.T, ctx context.Context, c CollaborationStageConfirmation, decision, reason string) CollaborationStageDecisionParams {
	t.Helper()
	return CollaborationStageDecisionParams{CollaborationWorkflowRequest: f.req(t, ctx), ExpectedPolicyRevision: f.policyRevision,
		ExpectedRoleRevision: f.stageInt(t, `SELECT role_revision FROM collaboration_stage_task WHERE issue_id=$1`),
		ProposalID:           util.MustParseUUID(c.ProposalID), Candidate: stageRef(c), Decision: decision, Reason: reason}
}

func (f *collaborationStageFixture) children(t *testing.T) int {
	t.Helper()
	return f.fx.Count(t, `SELECT count(*) FROM issue WHERE parent_issue_id=$1`, f.issue.Issue.ID)
}

func (f *collaborationStageFixture) stageRows(t *testing.T) string {
	t.Helper()
	var snapshot string
	f.fx.QueryRow(t, `SELECT jsonb_build_object(
 'task',(SELECT jsonb_agg(to_jsonb(x)-'updated_at') FROM collaboration_stage_task x WHERE issue_id=$1),
 'roles',(SELECT count(*) FROM collaboration_stage_role WHERE issue_id=$1),
 'candidates',(SELECT count(*) FROM collaboration_stage_candidate WHERE issue_id=$1),
 'decisions',(SELECT count(*) FROM collaboration_stage_decision WHERE issue_id=$1),
 'children',(SELECT jsonb_agg(to_jsonb(x)-'updated_at' ORDER BY child_issue_id) FROM collaboration_stage_child x WHERE root_issue_id=$1),
 'issues',(SELECT count(*) FROM issue WHERE parent_issue_id=$1),
 'tasks',(SELECT count(*) FROM agent_task_queue WHERE issue_id IN (SELECT id FROM issue WHERE parent_issue_id=$1)),
 'events',(SELECT count(*) FROM collaboration_event WHERE issue_id=$1))::text`, f.issue.Issue.ID).Scan(&snapshot)
	return snapshot
}

func TestStagePolicyRevisionCASAuthorityAndOriginalKey(t *testing.T) {
	f := newCollaborationStageFixture(t)
	ws, project := util.MustParseUUID(f.workspace), util.MustParseUUID(f.project)
	params := func(user string, expected int64, enabled bool) StagePolicyRevisionParams {
		return StagePolicyRevisionParams{WorkspaceID: ws, ProjectID: project, RequestID: uuid.NewString(), ExpectedRevision: expected, Enabled: enabled,
			Reason: "enable the fixed stage contract", Credential: stageCredential(user)}
	}
	policyRows := func() int {
		return f.fx.Count(t, `SELECT count(*) FROM project_stage_policy WHERE project_id=$1`, f.project)
	}
	initial, err := f.svc.GetStagePolicy(f.ctx, ws, project, stageCredential(f.creator))
	if err != nil || initial.PolicyRevision != "0" || initial.Enabled || initial.CreatedBy != nil {
		t.Fatalf("absent policy is not virtual revision 0/disabled: %+v %v", initial, err)
	}
	member := f.fx.User(t, "Plain member", "member-"+strings.ReplaceAll(uuid.NewString(), "-", "")+"@example.test")
	f.fx.Member(t, f.workspace, member, "member")
	task := stageCredential(f.creator)
	task.Principal.Kind = auth.CredentialTask
	agentActor := stageCredential(f.creator)
	agentActor.Principal.AgentActor = true
	expired := stageCredential(f.creator)
	expired.Principal.ExpiresAt = time.Now().Add(-time.Minute)
	for name, tc := range map[string]struct {
		credential ArtifactCredentialSnapshot
		code       string
	}{
		"member": {stageCredential(member), "stage_policy_management_denied"}, "outsider": {stageCredential(f.outsider), "stage_policy_denied"},
		"external_handler": {stageCredential(f.handler), "stage_policy_denied"}, "task_token": {task, "stage_policy_denied"},
		"agent_actor": {agentActor, "stage_policy_denied"}, "expired": {expired, "stage_policy_denied"},
	} {
		p := params(f.creator, 0, true)
		p.Credential = tc.credential
		if _, err := f.svc.ReviseStagePolicy(f.ctx, p); !collaborationErrorCode(err, tc.code) || policyRows() != 0 {
			t.Fatalf("%s wrote or was not refused with %s: %v", name, tc.code, err)
		}
	}
	blank := params(f.creator, 0, true)
	blank.Reason = "  "
	if _, err = f.svc.ReviseStagePolicy(f.ctx, blank); !collaborationErrorCode(err, "invalid_stage_policy_request") {
		t.Fatalf("blank reason accepted: %v", err)
	}
	before := f.stageRows(t)
	enable := params(f.creator, 0, true)
	first, err := f.svc.ReviseStagePolicy(f.ctx, enable)
	if err != nil || first.Code != "stage_policy_revised" || first.StageConfirmation.Action != "revise_policy" || first.StageConfirmation.PolicyRevision != "1" ||
		first.StageConfirmation.Enabled == nil || !*first.StageConfirmation.Enabled || first.StageConfirmation.IssueID != "" || first.StageConfirmation.ProjectID != f.project {
		t.Fatalf("enable revision: %+v %v", first, err)
	}
	if f.stageRows(t) != before {
		t.Fatal("enabling assigned roles, approved children or started work")
	}
	replay, err := f.svc.ReviseStagePolicy(f.ctx, enable)
	if err != nil || replay != first && (replay.Code != first.Code || replay.StageConfirmation.PolicyRevision != "1") || policyRows() != 1 {
		t.Fatalf("original key created another revision: %+v %v", replay, err)
	}
	changed := enable
	changed.Reason = "a different intent"
	if _, err = f.svc.ReviseStagePolicy(f.ctx, changed); !collaborationErrorCode(err, "idempotency_conflict") {
		t.Fatalf("changed body reused the original key: %v", err)
	}
	if _, err = f.svc.ReviseStagePolicy(f.ctx, params(f.creator, 0, false)); !collaborationErrorCode(err, "stage_policy_revision_conflict") || policyRows() != 1 {
		t.Fatalf("stale revision wrote: %v", err)
	}
	if _, err = f.fx.Pool.Exec(f.ctx, `UPDATE project_stage_policy SET enabled=false WHERE project_id=$1`, f.project); err == nil {
		t.Fatal("a policy revision was rewritten in place")
	}
	recovered, err := f.svc.GetStagePolicyOperation(f.ctx, ws, project, stageCredential(f.creator), enable.RequestID)
	if err != nil || recovered.StageConfirmation.PolicyRevision != "1" {
		t.Fatalf("original key recovery: %+v %v", recovered, err)
	}
	if _, err = f.svc.GetStagePolicyOperation(f.ctx, ws, project, stageCredential(f.creator), uuid.NewString()); !collaborationErrorCode(err, "stage_policy_operation_not_found") {
		t.Fatalf("unknown key inferred a result: %v", err)
	}
	disable := params(f.creator, 1, false)
	second, err := f.svc.ReviseStagePolicy(f.ctx, disable)
	if err != nil || second.StageConfirmation.PolicyRevision != "2" || *second.StageConfirmation.Enabled {
		t.Fatalf("explicit disable: %+v %v", second, err)
	}
	// The first key still describes its own revision, not the current value.
	recovered, err = f.svc.GetStagePolicyOperation(f.ctx, ws, project, stageCredential(f.creator), enable.RequestID)
	if err != nil || recovered.StageConfirmation.PolicyRevision != "1" || !*recovered.StageConfirmation.Enabled {
		t.Fatalf("recovery inferred from the current policy: %+v %v", recovered, err)
	}
	current, err := f.svc.GetStagePolicy(f.ctx, ws, project, stageCredential(member))
	if err != nil || current.PolicyRevision != "2" || current.Enabled || current.Reason == "" {
		t.Fatalf("member read: %+v %v", current, err)
	}
	// Losing administration keeps the original receipt but grants no new write.
	f.fx.Exec(t, `UPDATE member SET role='member' WHERE workspace_id=$1 AND user_id=$2`, f.workspace, f.creator)
	if replay, err = f.svc.ReviseStagePolicy(f.ctx, disable); err != nil || replay.StageConfirmation.PolicyRevision != "2" {
		t.Fatalf("former admin lost the original receipt: %+v %v", replay, err)
	}
	if _, err = f.svc.ReviseStagePolicy(f.ctx, params(f.creator, 2, true)); !collaborationErrorCode(err, "stage_policy_management_denied") || policyRows() != 2 {
		t.Fatalf("former admin wrote a revision: %v", err)
	}
	other := f.fx.Insert(t, "project", testutil.Cols{"workspace_id": f.fx.Workspace(t, "Elsewhere", "elsewhere-"+strings.ReplaceAll(uuid.NewString(), "-", "")), "title": "Foreign", "status": "planned", "priority": "none"})
	if _, err = f.svc.GetStagePolicy(f.ctx, ws, util.MustParseUUID(other), stageCredential(member)); !collaborationErrorCode(err, "stage_policy_project_not_found") {
		t.Fatalf("foreign project resolved: %v", err)
	}
}

func TestStageRolesRequireSourceAdminParticipantsAndSeparation(t *testing.T) {
	f := newCollaborationStageFixture(t)
	roles := func(ctx context.Context, expectedRole int64, set CollaborationStageRoleSet) CollaborationStageRolesParams {
		return CollaborationStageRolesParams{CollaborationWorkflowRequest: f.req(t, ctx), ExpectedPolicyRevision: f.policyRevision, ExpectedRoleRevision: expectedRole, Roles: set}
	}
	disabled := roles(f.ctx, 0, f.roleSet())
	disabled.ExpectedPolicyRevision = 1
	if _, err := f.svc.ConfigureStageRoles(f.ctx, disabled); !collaborationErrorCode(err, "stage_policy_disabled") {
		t.Fatalf("roles configured without an enabled policy: %v", err)
	}
	if _, err := f.svc.ConfigureStageRoles(f.ctx, disabled); !collaborationErrorCode(err, "stage_policy_disabled") {
		t.Fatalf("stored refusal did not replay: %v", err)
	}
	f.revise(t, true)
	before := f.stageRows(t)
	machine := roles(f.ctx, 0, f.roleSet())
	if _, err := f.svc.ConfigureStageRoles(f.machine, machine); !collaborationErrorCode(err, "forbidden") {
		t.Fatalf("machine credential configured roles: %v", err)
	}
	if _, err := f.svc.ConfigureStageRoles(collaborationContext(f.outsider), machine); !collaborationErrorCode(err, "not_found") {
		t.Fatalf("outsider reached the stage configuration: %v", err)
	}
	if _, err := f.svc.ConfigureStageRoles(f.handlerCtx, roles(f.handlerCtx, 0, f.roleSet())); !collaborationErrorCode(err, "stage_configuration_denied") {
		t.Fatalf("external handler configured roles: %v", err)
	}
	// An ordinary source member who participates is still not an administrator.
	f.fx.Member(t, f.workspace, f.reviewer, "member")
	if _, err := f.svc.ConfigureStageRoles(f.reviewerCtx, roles(f.reviewerCtx, 0, f.roleSet())); !collaborationErrorCode(err, "stage_configuration_denied") {
		t.Fatalf("plain source member configured roles: %v", err)
	}
	self := f.roleSet()
	self.CodeReviewer = self.Implementer
	if _, err := f.svc.ConfigureStageRoles(f.ctx, roles(f.ctx, 0, self)); !collaborationErrorCode(err, "stage_self_review_forbidden") {
		t.Fatalf("implementer may review their own code: %v", err)
	}
	merged := f.roleSet()
	merged.ReleaseApprover = merged.QAApprover
	if _, err := f.svc.ConfigureStageRoles(f.ctx, roles(f.ctx, 0, merged)); !collaborationErrorCode(err, "stage_qa_release_conflict") {
		t.Fatalf("QA and release were merged: %v", err)
	}
	stranger := f.roleSet()
	stranger.QAApprover = util.MustParseUUID(f.outsider)
	if _, err := f.svc.ConfigureStageRoles(f.ctx, roles(f.ctx, 0, stranger)); !collaborationErrorCode(err, "stage_role_target_unavailable") {
		t.Fatalf("a non-participant became a signer: %v", err)
	}
	stale := roles(f.ctx, 0, f.roleSet())
	stale.ExpectedPolicyRevision = 7
	if _, err := f.svc.ConfigureStageRoles(f.ctx, stale); !collaborationErrorCode(err, "stage_policy_changed") {
		t.Fatalf("stale policy observation accepted: %v", err)
	}
	if after := f.stageRows(t); after != before {
		t.Fatalf("a refused configuration wrote stage facts or events:\n%s\n%s", before, after)
	}
	first := roles(f.ctx, 0, f.roleSet())
	configured, err := f.svc.ConfigureStageRoles(f.ctx, first)
	if err != nil || configured.Code != "stage_roles_configured" || configured.StageConfirmation == nil || !configured.StageConfirmation.valid(stageKindConfigureRoles) ||
		configured.StageConfirmation.RoleRevision != "1" || configured.StageConfirmation.IssueID != util.UUIDToString(f.issue.Issue.ID) {
		t.Fatalf("configure roles: %+v %v", configured, err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1 AND kind='stage_roles_configured'`, f.issue.Issue.ID) != 1 {
		t.Fatal("role configuration left no event")
	}
	replay, err := f.svc.ConfigureStageRoles(f.ctx, first)
	if err != nil || replay.Code != configured.Code || *replay.StageConfirmation != *configured.StageConfirmation ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_stage_role WHERE issue_id=$1`, f.issue.Issue.ID) != 1 {
		t.Fatalf("original key created another role revision: %+v %v", replay, err)
	}
	recovered, err := f.svc.GetWorkflowOperation(f.ctx, f.issue.Issue.ID, first.RequestID)
	if err != nil || recovered.StageConfirmation == nil || *recovered.StageConfirmation != *configured.StageConfirmation {
		t.Fatalf("original operation recovery lost the stage confirmation: %+v %v", recovered, err)
	}
	if _, err = f.svc.ConfigureStageRoles(f.ctx, roles(f.ctx, 0, f.roleSet())); !collaborationErrorCode(err, "stage_roles_changed") {
		t.Fatalf("stale role observation accepted: %v", err)
	}
	same, err := f.svc.ConfigureStageRoles(f.ctx, roles(f.ctx, 1, f.roleSet()))
	if err != nil || same.Code != "stage_roles_unchanged" || same.StageConfirmation.RoleRevision != "1" {
		t.Fatalf("identical configuration: %+v %v", same, err)
	}
	next := f.roleSet()
	next.BusinessOwner = util.MustParseUUID(f.handler)
	changed, err := f.svc.ConfigureStageRoles(f.ctx, roles(f.ctx, 1, next))
	if err != nil || changed.StageConfirmation.RoleRevision != "2" {
		t.Fatalf("role change: %+v %v", changed, err)
	}
	if _, err = f.fx.Pool.Exec(f.ctx, `UPDATE collaboration_stage_role SET qa_approver_user_id=release_approver_user_id WHERE issue_id=$1`, f.issue.Issue.ID); err == nil {
		t.Fatal("a role revision was rewritten in place")
	}
	view := f.view(t)
	if !view.Managed || view.RoleRevision != "2" || view.Roles == nil || view.Roles.BusinessOwner == nil || util.UUIDToString(view.Roles.BusinessOwner.UserID) != f.handler ||
		util.UUIDToString(view.Roles.ProductApprover.UserID) != f.creator || !view.NeedsConfirmation || strings.Join(view.Missing, ",") != "scope" ||
		view.Stage == nil || view.Stage.Current != "development" || !view.Capabilities.ConfigureRoles || !view.Capabilities.ConfirmScope {
		t.Fatalf("stage view after configuration: %+v", view)
	}
	handlerView, err := f.svc.StageView(f.handlerCtx, f.issue.Issue.ID)
	if err != nil || handlerView.Capabilities.ConfigureRoles || handlerView.Capabilities.ConfirmScope || handlerView.Capabilities.DecideProposal || !handlerView.Capabilities.CreateProposal {
		t.Fatalf("handler capabilities: %+v %v", handlerView.Capabilities, err)
	}
	if _, err = f.svc.StageView(collaborationContext(f.outsider), f.issue.Issue.ID); !collaborationErrorCode(err, "not_found") {
		t.Fatalf("outsider read the stage view: %v", err)
	}
	if _, err = f.svc.StageView(f.machine, f.issue.Issue.ID); !collaborationErrorCode(err, "not_found") {
		t.Fatalf("machine read the stage view: %v", err)
	}
}

func TestStageProposalVersionsDecisionsAndSingleMaterialization(t *testing.T) {
	f := newCollaborationStageFixture(t)
	f.revise(t, true)
	// Roles are not needed to file a proposal; nothing becomes executable.
	created := f.propose(t, f.handlerCtx, "Build the callback", "Verify the callback")
	if f.children(t) != 0 {
		t.Fatal("an unapproved proposal created issues")
	}
	if _, err := f.svc.CreateStageProposal(f.machine, CollaborationStageProposalParams{CollaborationWorkflowRequest: f.req(t, f.ctx),
		ExpectedPolicyRevision: f.policyRevision, Content: stageContent("machine"), Source: CollaborationStageProposalSource{Kind: "manual"}}); !collaborationErrorCode(err, "forbidden") {
		t.Fatalf("machine credential filed a proposal: %v", err)
	}
	invalid := CollaborationStageProposalParams{CollaborationWorkflowRequest: f.req(t, f.ctx), ExpectedPolicyRevision: f.policyRevision,
		Content: collaboration.StageProposalContent{}, Source: CollaborationStageProposalSource{Kind: "manual"}}
	if _, err := f.svc.CreateStageProposal(f.ctx, invalid); !collaborationErrorCode(err, "invalid_stage_request") {
		t.Fatalf("empty proposal accepted: %v", err)
	}
	if _, err := f.svc.CreateStageProposal(f.machine, invalid); !collaborationErrorCode(err, "forbidden") {
		t.Fatalf("a malformed machine request was validated instead of refused: %v", err)
	}
	if _, err := f.svc.DecideStageProposal(f.ctx, f.decisionRaw(t, f.ctx, created, "accept", "", 0)); !collaborationErrorCode(err, "stage_roles_unconfigured") || f.children(t) != 0 {
		t.Fatalf("accept without a role configuration: %v", err)
	}
	f.configure(t)
	if _, err := f.svc.DecideStageProposal(f.ctx, f.decision(t, f.ctx, created, "accept", "")); !collaborationErrorCode(err, "stage_needs_confirmation") || f.children(t) != 0 {
		t.Fatalf("accept before the existing-child scope was confirmed: %v", err)
	}
	f.confirmScope(t)
	if _, err := f.svc.DecideStageProposal(f.handlerCtx, f.decision(t, f.handlerCtx, created, "accept", "")); !collaborationErrorCode(err, "stage_role_required") || f.children(t) != 0 {
		t.Fatalf("a participant who is not the product approver accepted: %v", err)
	}
	if _, err := f.svc.DecideStageProposal(f.machine, f.decision(t, f.ctx, created, "accept", "")); !collaborationErrorCode(err, "forbidden") || f.children(t) != 0 {
		t.Fatalf("a machine credential accepted: %v", err)
	}
	if _, err := f.svc.DecideStageProposal(f.ctx, f.decision(t, f.ctx, created, "reject", " ")); !collaborationErrorCode(err, "invalid_stage_request") {
		t.Fatalf("rejection without a reason: %v", err)
	}
	rejected, err := f.svc.DecideStageProposal(f.ctx, f.decision(t, f.ctx, created, "reject", "Verification scope is missing"))
	if err != nil || rejected.Code != "stage_proposal_rejected" || rejected.StageConfirmation == nil || !rejected.StageConfirmation.valid(stageKindDecideProposal) ||
		rejected.StageConfirmation.Decision != "rejected" || f.children(t) != 0 {
		t.Fatalf("reject: %+v %v", rejected, err)
	}
	if _, err = f.svc.DecideStageProposal(f.ctx, f.decision(t, f.ctx, created, "accept", "")); !collaborationErrorCode(err, "stage_candidate_not_pending") || f.children(t) != 0 {
		t.Fatalf("a rejected version was accepted: %v", err)
	}
	revise := func(ctx context.Context, expected CollaborationStageConfirmation, titles ...string) (CollaborationWorkflowReceipt, error) {
		ref := stageRef(expected)
		return f.svc.ReviseStageProposal(ctx, CollaborationStageProposalParams{CollaborationWorkflowRequest: f.req(t, ctx), ExpectedPolicyRevision: f.policyRevision,
			ProposalID: util.MustParseUUID(expected.ProposalID), ExpectedCandidate: &ref, Content: stageContent(titles...), Source: CollaborationStageProposalSource{Kind: "manual"}})
	}
	second, err := revise(f.handlerCtx, created, "Build the callback", "Verify the callback", "Replay the callback")
	if err != nil || second.Code != "stage_proposal_revised" || !second.StageConfirmation.valid(stageKindReviseProposal) || second.StageConfirmation.Version != "2" ||
		second.StageConfirmation.ParentCandidateID != created.CandidateID || second.StageConfirmation.ProposalID != created.ProposalID {
		t.Fatalf("revise: %+v %v", second, err)
	}
	v2 := *second.StageConfirmation
	if _, err = revise(f.handlerCtx, created, "Something else"); !collaborationErrorCode(err, "stage_candidate_changed") {
		t.Fatalf("a superseded version was revised: %v", err)
	}
	if _, err = revise(f.handlerCtx, v2, "Build the callback", "Verify the callback", "Replay the callback"); !collaborationErrorCode(err, "stage_proposal_unchanged") {
		t.Fatalf("identical content created a version: %v", err)
	}
	if _, err = f.fx.Pool.Exec(f.ctx, `UPDATE collaboration_stage_candidate SET content='{}' WHERE candidate_id=$1`, v2.CandidateID); err == nil {
		t.Fatal("a candidate version was rewritten in place")
	}
	// Modify-accept: the approver saves a visible new version, then accepts it.
	third, err := revise(f.ctx, v2, "Build the callback", "Verify and replay the callback")
	if err != nil {
		t.Fatal(err)
	}
	v3 := *third.StageConfirmation
	if _, err = f.svc.DecideStageProposal(f.ctx, f.decision(t, f.ctx, v2, "accept", "")); !collaborationErrorCode(err, "stage_candidate_changed") || f.children(t) != 0 {
		t.Fatalf("accept was rebound from a superseded version: %v", err)
	}
	wrongDigest := f.decision(t, f.ctx, v3, "accept", "")
	wrongDigest.Candidate.ContentDigest = v2.ContentDigest
	if _, err = f.svc.DecideStageProposal(f.ctx, wrongDigest); !collaborationErrorCode(err, "stage_candidate_changed") || f.children(t) != 0 {
		t.Fatalf("accept ignored the content digest: %v", err)
	}
	beforeEvents := f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1`, f.issue.Issue.ID)
	accept := f.decision(t, f.ctx, v3, "accept", "Approved as modified")
	accepted, err := f.svc.DecideStageProposal(f.ctx, accept)
	if err != nil || accepted.Code != "stage_proposal_accepted" || !accepted.StageConfirmation.valid(stageKindDecideProposal) || accepted.StageConfirmation.Decision != "accepted" ||
		accepted.StageConfirmation.CandidateID != v3.CandidateID || accepted.StageConfirmation.RoleRevision != "1" {
		t.Fatalf("accept: %+v %v", accepted, err)
	}
	if f.children(t) != 2 ||
		f.fx.Count(t, `SELECT count(*) FROM issue WHERE parent_issue_id=$1 AND project_id=$2 AND creator_type='member' AND creator_id=$3 AND assignee_id IS NULL AND status='todo'`, f.issue.Issue.ID, f.project, f.creator) != 2 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_stage_child WHERE root_issue_id=$1 AND origin='materialized' AND proposal_id=$2 AND candidate_id=$3 AND decision_id=$4 AND lifecycle_epoch=1`,
			f.issue.Issue.ID, v3.ProposalID, v3.CandidateID, accepted.StageConfirmation.DecisionID) != 2 ||
		f.fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id IN (SELECT id FROM issue WHERE parent_issue_id=$1)`, f.issue.Issue.ID) != 0 ||
		f.fx.Count(t, `SELECT count(*) FROM issue_collaboration WHERE issue_id IN (SELECT id FROM issue WHERE parent_issue_id=$1)`, f.issue.Issue.ID) != 0 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1`, f.issue.Issue.ID) != beforeEvents+1 {
		t.Fatal("acceptance did not atomically create exactly the ordinary children and mapping, or it dispatched work")
	}
	replay, err := f.svc.DecideStageProposal(f.ctx, accept)
	if err != nil || *replay.StageConfirmation != *accepted.StageConfirmation || f.children(t) != 2 {
		t.Fatalf("original key materialized again: %+v %v", replay, err)
	}
	if _, err = f.svc.DecideStageProposal(f.ctx, f.decision(t, f.ctx, v3, "accept", "")); !collaborationErrorCode(err, "stage_proposal_already_materialized") || f.children(t) != 2 {
		t.Fatalf("a new key materialized the proposal again: %v", err)
	}
	if _, err = revise(f.ctx, v3, "A later idea"); !collaborationErrorCode(err, "stage_proposal_already_materialized") {
		t.Fatalf("a materialized proposal was revised: %v", err)
	}
	detail, err := f.svc.GetStageProposal(f.handlerCtx, f.issue.Issue.ID, util.MustParseUUID(v3.ProposalID))
	if err != nil || detail.State != "accepted" || len(detail.Versions) != 3 || detail.Versions[0].Version != "3" || detail.Versions[0].State != "accepted" ||
		detail.Versions[1].State != "superseded" || detail.Versions[2].State != "rejected" || len(detail.Decisions) != 2 || len(detail.Children) != 2 ||
		detail.Children[0].Ordinal != 1 || detail.Children[0].Title != "Build the callback" || detail.Children[0].CanOpen || !detail.Children[0].Attached {
		t.Fatalf("proposal detail: %+v %v", detail, err)
	}
	modified := false
	for _, decision := range detail.Decisions {
		modified = modified || (decision.Decision == "accepted" && decision.AcceptedWithModification && decision.ActorUserID == f.creator && decision.Reason == "Approved as modified")
	}
	if !modified {
		t.Fatalf("modify-accept lost its person, reason or modification fact: %+v", detail.Decisions)
	}
	page, err := f.svc.ListStageProposals(f.ctx, f.issue.Issue.ID, CollaborationStageProposalQuery{Limit: 1})
	if err != nil || len(page.Items) != 1 || page.Items[0].State != "accepted" || page.Items[0].VersionCount != "3" || page.Items[0].Current.ChildCount != 2 || page.NextCursor != nil {
		t.Fatalf("proposal page: %+v %v", page, err)
	}
	view := f.view(t)
	if len(view.Children.Items) != 2 || view.Children.Items[0].Origin != "materialized" || !view.Children.Items[0].ApprovalCurrent || view.Children.Items[0].ExecutionGated {
		t.Fatalf("materialized children in the stage view: %+v", view.Children)
	}
	// Activity preserves each persisted stage kind and only its safe metadata.
	// Reasons, proposal payloads and child contents stay out of the projection.
	activity, err := f.svc.ListActivity(f.handlerCtx, f.issue.Issue.ID, CollaborationActivityQuery{Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	stageEntries := make(map[pgtype.UUID]CollaborationActivityEntry)
	for _, entry := range activity.Items {
		if strings.HasPrefix(entry.Kind, "stage_") {
			if _, duplicate := stageEntries[entry.ID]; duplicate {
				t.Fatalf("a stage event appeared twice in activity: %+v", entry)
			}
			stageEntries[entry.ID] = entry
		}
	}
	safeFields := map[string][]string{
		"stage_roles_configured":  {"role_revision"},
		"stage_proposal_created":  {"proposal_id", "candidate_id", "version"},
		"stage_proposal_revised":  {"proposal_id", "candidate_id", "version", "parent_candidate_id"},
		"stage_proposal_accepted": {"proposal_id", "candidate_id", "version", "decision_id"},
		"stage_proposal_rejected": {"proposal_id", "candidate_id", "version", "decision_id"},
		"stage_scope_confirmed":   {"decision_id", "scope_revision", "lifecycle_epoch"},
	}
	rows, err := f.fx.Pool.Query(f.ctx, `SELECT id, event_seq, kind, payload FROM collaboration_event WHERE issue_id=$1 AND kind LIKE 'stage_%'`, f.issue.Issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	stageEvents := 0
	for rows.Next() {
		var eventID pgtype.UUID
		var eventSeq int64
		var kind string
		var payload []byte
		if err := rows.Scan(&eventID, &eventSeq, &kind, &payload); err != nil {
			t.Fatal(err)
		}
		stageEvents++
		entry, present := stageEntries[eventID]
		if !present || entry.IssueID != f.issue.Issue.ID || entry.EventSeq != eventSeq || entry.Kind != kind || !entry.DetailsAvailable {
			t.Fatalf("a persisted stage event lost its typed activity projection: id=%v seq=%d kind=%s entry=%+v", eventID, eventSeq, kind, entry)
		}
		delete(stageEntries, eventID)
		fields, supported := safeFields[kind]
		if !supported {
			t.Fatalf("unexpected stage kind in the proposal journey: %s", kind)
		}
		var stored struct {
			Stage map[string]json.RawMessage `json:"stage"`
		}
		if err := json.Unmarshal(payload, &stored); err != nil {
			t.Fatal(err)
		}
		var metadata map[string]string
		if err := json.Unmarshal(entry.Stage, &metadata); err != nil || len(metadata) != len(fields) {
			t.Fatalf("stage activity did not retain exactly its safe metadata: kind=%s stage=%s err=%v", kind, entry.Stage, err)
		}
		for _, field := range fields {
			var expected string
			if err := json.Unmarshal(stored.Stage[field], &expected); err != nil {
				t.Fatalf("persisted stage metadata is invalid: kind=%s field=%s err=%v", kind, field, err)
			}
			if actual, present := metadata[field]; !present || actual != expected {
				t.Fatalf("stage activity changed safe metadata: kind=%s field=%s got=%q want=%q", kind, field, actual, expected)
			}
		}
		if entry.Note != "" || len(entry.Changes) != 0 || entry.Target != nil || entry.Handoff != nil || entry.CommentID.Valid {
			t.Fatalf("a stage event leaked payload, reason or child data through activity: %+v", entry)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if stageEvents == 0 || len(stageEntries) != 0 {
		t.Fatalf("stage activity did not match all persisted events: events=%d unmatched=%d", stageEvents, len(stageEntries))
	}
}

// decisionRaw builds a decision for a root that has no role configuration yet.
func (f *collaborationStageFixture) decisionRaw(t *testing.T, ctx context.Context, c CollaborationStageConfirmation, decision, reason string, role int64) CollaborationStageDecisionParams {
	t.Helper()
	return CollaborationStageDecisionParams{CollaborationWorkflowRequest: f.req(t, ctx), ExpectedPolicyRevision: f.policyRevision, ExpectedRoleRevision: role,
		ProposalID: util.MustParseUUID(c.ProposalID), Candidate: stageRef(c), Decision: decision, Reason: reason}
}

func TestStageProposalConcurrentAcceptCreatesOneChildSet(t *testing.T) {
	f := newCollaborationStageFixture(t)
	f.revise(t, true)
	f.configure(t)
	f.confirmScope(t)
	created := f.propose(t, f.handlerCtx, "First", "Second", "Third")
	const callers = 4
	params := make([]CollaborationStageDecisionParams, callers)
	for i := range params {
		params[i] = f.decision(t, f.ctx, created, "accept", "")
	}
	start := make(chan struct{})
	results := make(chan error, callers)
	var wg sync.WaitGroup
	for i := range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := f.svc.DecideStageProposal(f.ctx, params[i])
			results <- err
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	accepted := 0
	for err := range results {
		switch {
		case err == nil:
			accepted++
		case collaborationErrorCode(err, "stale_revision"), collaborationErrorCode(err, "stage_proposal_already_materialized"), collaborationErrorCode(err, "collaboration_busy"):
		default:
			t.Fatalf("unexpected concurrent result: %v", err)
		}
	}
	if accepted != 1 || f.children(t) != 3 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_stage_decision WHERE issue_id=$1 AND step='proposal' AND decision='accepted'`, f.issue.Issue.ID) != 1 ||
		f.fx.Count(t, `SELECT count(DISTINCT ordinal) FROM collaboration_stage_child WHERE root_issue_id=$1`, f.issue.Issue.ID) != 3 {
		t.Fatalf("concurrent acceptance created %d successes and %d children", accepted, f.children(t))
	}
}

func TestStageProposalAcceptRollsBackEveryChildOnRefusal(t *testing.T) {
	f := newCollaborationStageFixture(t)
	f.revise(t, true)
	f.configure(t)
	// A human's manual child stays an ordinary issue and blocks a duplicate title.
	manual := f.fx.Issue(t, "Duplicate title", testutil.Cols{"parent_issue_id": f.issue.Issue.ID, "project_id": f.project})
	f.syncCounter(t)
	f.confirmScope(t)
	created := f.propose(t, f.handlerCtx, "Fresh one", "Fresh two", "Duplicate title")
	var counter int64
	f.fx.QueryRow(t, `SELECT issue_counter FROM workspace WHERE id=$1`, f.workspace).Scan(&counter)
	before := f.stageRows(t)
	refused := f.decision(t, f.ctx, created, "accept", "")
	if _, err := f.svc.DecideStageProposal(f.ctx, refused); !collaborationErrorCode(err, "stage_child_creation_refused") {
		t.Fatalf("a refused child did not refuse the whole acceptance: %v", err)
	}
	var after int64
	f.fx.QueryRow(t, `SELECT issue_counter FROM workspace WHERE id=$1`, f.workspace).Scan(&after)
	if f.stageRows(t) != before || after != counter || f.children(t) != 1 {
		t.Fatalf("a refused acceptance kept a child prefix, mapping, decision or number:\n%s\n%s", before, f.stageRows(t))
	}
	if _, err := f.svc.DecideStageProposal(f.ctx, refused); !collaborationErrorCode(err, "stage_child_creation_refused") || f.children(t) != 1 {
		t.Fatalf("the stored refusal did not replay: %v", err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_stage_child WHERE child_issue_id=$1`, manual) != 0 {
		t.Fatal("a manual child acquired a proposal mapping")
	}
	// The proposal stays pending; a corrected version can still be accepted.
	ref := stageRef(created)
	revised, err := f.svc.ReviseStageProposal(f.ctx, CollaborationStageProposalParams{CollaborationWorkflowRequest: f.req(t, f.ctx), ExpectedPolicyRevision: f.policyRevision,
		ProposalID: util.MustParseUUID(created.ProposalID), ExpectedCandidate: &ref, Content: stageContent("Fresh one", "Fresh two"), Source: CollaborationStageProposalSource{Kind: "manual"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.DecideStageProposal(f.ctx, f.decision(t, f.ctx, *revised.StageConfirmation, "accept", "")); err != nil || f.children(t) != 3 {
		t.Fatalf("corrected proposal: %v children=%d", err, f.children(t))
	}
	// The approver needs the ordinary source creation right.
	second := f.propose(t, f.handlerCtx, "Needs a member")
	f.fx.Exec(t, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, f.workspace, f.creator)
	if _, err = f.svc.DecideStageProposal(f.ctx, f.decision(t, f.ctx, second, "accept", "")); !collaborationErrorCode(err, "stage_source_creation_denied") || f.children(t) != 3 {
		t.Fatalf("an approver without source membership materialized children: %v", err)
	}
}

func TestStageProposalSourceIsVerifiedByTheServer(t *testing.T) {
	f := newCollaborationStageFixture(t)
	f.revise(t, true)
	comment := f.fx.Comment(t, util.UUIDToString(f.issue.Issue.ID), "Agent suggested a split", testutil.Cols{"author_type": "agent", "author_id": uuid.NewString()})
	f.fx.Cleanup(t, `DELETE FROM comment WHERE id=$1`, comment)
	source := func(kind string, id string, revision int64) CollaborationStageProposalParams {
		p := CollaborationStageProposalParams{CollaborationWorkflowRequest: f.req(t, f.handlerCtx), ExpectedPolicyRevision: f.policyRevision, Content: stageContent("From a comment")}
		p.Source = CollaborationStageProposalSource{Kind: kind}
		if kind == "comment" {
			p.Source.CommentID, p.Source.CommentRevision = util.MustParseUUID(id), revision
		} else if kind == "run" {
			p.Source.RunID = util.MustParseUUID(id)
		}
		return p
	}
	var revision int64
	f.fx.QueryRow(t, `SELECT revision FROM comment WHERE id=$1`, comment).Scan(&revision)
	for name, p := range map[string]CollaborationStageProposalParams{
		"missing_comment": source("comment", uuid.NewString(), 1), "stale_comment": source("comment", comment, revision+1), "missing_run": source("run", uuid.NewString(), 0),
	} {
		if _, err := f.svc.CreateStageProposal(f.handlerCtx, p); !collaborationErrorCode(err, "stage_source_unavailable") {
			t.Fatalf("%s accepted: %v", name, err)
		}
	}
	bad := source("manual", "", 0)
	bad.Source.Kind = "agent"
	if _, err := f.svc.CreateStageProposal(f.handlerCtx, bad); !collaborationErrorCode(err, "invalid_stage_request") {
		t.Fatalf("a client-stated agent source was accepted: %v", err)
	}
	created, err := f.svc.CreateStageProposal(f.handlerCtx, source("comment", comment, revision))
	if err != nil {
		t.Fatal(err)
	}
	detail, err := f.svc.GetStageProposal(f.ctx, f.issue.Issue.ID, util.MustParseUUID(created.StageConfirmation.ProposalID))
	if err != nil || len(detail.Versions) != 1 {
		t.Fatalf("detail: %+v %v", detail, err)
	}
	got := detail.Versions[0].Source
	if got.Kind != "comment" || got.CommentID == nil || *got.CommentID != comment || got.AuthorType == nil || *got.AuthorType != "agent" || got.Digest == nil || len(*got.Digest) != 64 ||
		detail.Versions[0].SubmittedBy != f.handler {
		t.Fatalf("server did not record its own comment facts and the human submitter: %+v", got)
	}
}

func TestStageScopeConfirmationFirstEnableAndReopen(t *testing.T) {
	f := newCollaborationStageFixture(t)
	root := f.issue.Issue.ID
	human := f.fx.Issue(t, "Existing human child", testutil.Cols{"parent_issue_id": root, "project_id": f.project, "description": "manual scope"})
	machine := f.fx.Issue(t, "Existing agent child", testutil.Cols{"parent_issue_id": root, "project_id": f.project, "creator_type": "agent", "creator_id": uuid.NewString()})
	elsewhere := f.fx.Issue(t, "Unrelated issue")
	f.revise(t, true)
	view := f.view(t)
	if !view.Managed || !view.NeedsConfirmation || strings.Join(view.Missing, ",") != "roles,scope" || view.ScopeState != "none" || len(view.Children.Items) != 2 {
		t.Fatalf("first enablement did not require explicit confirmation: %+v", view)
	}
	digests := map[string]string{}
	for _, child := range view.Children.Items {
		digests[child.IssueID] = stageReadableDigest(t, child)
		if child.IssueID == machine && (!child.ExecutionGated || child.Origin != "none" || child.CreatorType != "agent") {
			t.Fatalf("an unconfirmed agent child is not shown as gated: %+v", child)
		}
		if child.IssueID == human && child.ExecutionGated {
			t.Fatalf("a human child lost its ordinary contract: %+v", child)
		}
	}
	scope := func(ctx context.Context, children ...CollaborationStageScopeChild) CollaborationStageScopeParams {
		return CollaborationStageScopeParams{CollaborationWorkflowRequest: f.req(t, ctx), ExpectedPolicyRevision: f.policyRevision,
			ExpectedRoleRevision:  f.stageInt(t, `SELECT COALESCE((SELECT role_revision FROM collaboration_stage_task WHERE issue_id=$1),0)`),
			ExpectedScopeRevision: f.stageInt(t, `SELECT COALESCE((SELECT scope_revision FROM collaboration_stage_task WHERE issue_id=$1),0)`), Children: children}
	}
	child := func(id string) CollaborationStageScopeChild {
		return CollaborationStageScopeChild{IssueID: util.MustParseUUID(id), ContentDigest: digests[id]}
	}
	if _, err := f.svc.ConfirmStageScope(f.ctx, scope(f.ctx, child(human))); !collaborationErrorCode(err, "stage_roles_unconfigured") {
		t.Fatalf("scope confirmed without a product approver: %v", err)
	}
	f.configure(t)
	before := f.stageRows(t)
	if _, err := f.svc.ConfirmStageScope(f.handlerCtx, scope(f.handlerCtx, child(human))); !collaborationErrorCode(err, "stage_role_required") {
		t.Fatalf("a non-approver confirmed scope: %v", err)
	}
	if _, err := f.svc.ConfirmStageScope(f.machine, scope(f.ctx, child(human))); !collaborationErrorCode(err, "forbidden") {
		t.Fatalf("a machine confirmed scope: %v", err)
	}
	wrong := child(machine)
	wrong.ContentDigest = strings.Repeat("0", 64)
	if _, err := f.svc.ConfirmStageScope(f.ctx, scope(f.ctx, child(human), wrong)); !collaborationErrorCode(err, "stage_scope_changed") {
		t.Fatalf("a changed child was confirmed: %v", err)
	}
	foreign := CollaborationStageScopeChild{IssueID: util.MustParseUUID(elsewhere), ContentDigest: collaboration.StageChildContentDigest("Unrelated issue", "")}
	if _, err := f.svc.ConfirmStageScope(f.ctx, scope(f.ctx, foreign)); !collaborationErrorCode(err, "stage_scope_child_unavailable") {
		t.Fatalf("an issue that is not a child was confirmed: %v", err)
	}
	staleScope := scope(f.ctx, child(human))
	staleScope.ExpectedScopeRevision = 5
	if _, err := f.svc.ConfirmStageScope(f.ctx, staleScope); !collaborationErrorCode(err, "stage_scope_changed") {
		t.Fatalf("a stale scope observation was accepted: %v", err)
	}
	if after := f.stageRows(t); after != before {
		t.Fatalf("a refused confirmation wrote stage facts:\n%s\n%s", before, after)
	}
	confirm := scope(f.ctx, child(human), child(machine))
	confirmed, err := f.svc.ConfirmStageScope(f.ctx, confirm)
	if err != nil || confirmed.Code != "stage_scope_confirmed" || !confirmed.StageConfirmation.valid(stageKindConfirmScope) || confirmed.StageConfirmation.ScopeRevision != "1" ||
		confirmed.StageConfirmation.LifecycleEpoch != "1" {
		t.Fatalf("confirm scope: %+v %v", confirmed, err)
	}
	if f.children(t) != 2 || f.fx.Count(t, `SELECT count(*) FROM collaboration_stage_child WHERE root_issue_id=$1 AND origin='confirmed' AND lifecycle_epoch=1 AND scope_decision_id=$2`, root, confirmed.StageConfirmation.DecisionID) != 2 ||
		f.fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=ANY($1::uuid[])`, []string{human, machine}) != 0 {
		t.Fatal("confirmation created, re-dispatched or failed to map the existing children")
	}
	replay, err := f.svc.ConfirmStageScope(f.ctx, confirm)
	if err != nil || *replay.StageConfirmation != *confirmed.StageConfirmation || f.fx.Count(t, `SELECT count(*) FROM collaboration_stage_decision WHERE issue_id=$1 AND step='scope'`, root) != 1 {
		t.Fatalf("original key confirmed again: %+v %v", replay, err)
	}
	view = f.view(t)
	if view.NeedsConfirmation || view.ScopeState != "current" || view.Scope == nil || len(view.Scope.ChildIssueIDs) != 2 || view.ScopeRevision != "1" {
		t.Fatalf("confirmed view: %+v", view)
	}
	for _, item := range view.Children.Items {
		if item.Origin != "confirmed" || !item.ApprovalCurrent || item.ExecutionGated {
			t.Fatalf("confirmed child: %+v", item)
		}
	}
	// A reopen starts a new lifecycle: the earlier confirmation cannot be consumed.
	created := f.propose(t, f.handlerCtx, "Before reopen")
	cancel := lifecycleParams(t, f.collaborationQueueFixture, "cancel", "cancelled", 0, 1)
	cancel.CollaborationWorkflowRequest = f.req(t, f.ctx)
	if _, err = f.svc.ChangeLifecycle(f.ctx, cancel); err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.ConfirmStageScope(f.ctx, scope(f.ctx)); !collaborationErrorCode(err, "issue_closed") {
		t.Fatalf("a closed root accepted a stage fact: %v", err)
	}
	reopen := lifecycleParams(t, f.collaborationQueueFixture, "reopen", "todo", 0, 1)
	reopen.CollaborationWorkflowRequest = f.req(t, f.ctx)
	if _, err = f.svc.ChangeLifecycle(f.ctx, reopen); err != nil {
		t.Fatal(err)
	}
	view = f.view(t)
	if !view.NeedsConfirmation || view.ScopeState != "lifecycle_changed" || strings.Join(view.Missing, ",") != "scope" || view.LifecycleEpoch != "3" {
		t.Fatalf("reopen kept the earlier confirmation: %+v", view)
	}
	for _, item := range view.Children.Items {
		if item.ApprovalCurrent || (item.IssueID == machine) != item.ExecutionGated {
			t.Fatalf("child after reopen: %+v", item)
		}
	}
	if _, err = f.svc.DecideStageProposal(f.ctx, f.decision(t, f.ctx, created, "accept", "")); !collaborationErrorCode(err, "stage_needs_confirmation") {
		t.Fatalf("an earlier-lifecycle confirmation unlocked materialization: %v", err)
	}
	again := f.confirmScope(t, child(human))
	if again.StageConfirmation.ScopeRevision != "2" || again.StageConfirmation.LifecycleEpoch != "3" ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_stage_child WHERE child_issue_id=$1 AND lifecycle_epoch=3`, human) != 1 ||
		f.fx.Count(t, `SELECT count(*) FROM collaboration_stage_child WHERE child_issue_id=$1 AND lifecycle_epoch=1`, machine) != 1 || f.children(t) != 2 {
		t.Fatalf("reconfirmation did not keep identities and refresh only the listed child: %+v", again)
	}
	if _, err = f.svc.DecideStageProposal(f.ctx, f.decision(t, f.ctx, created, "accept", "")); !collaborationErrorCode(err, "stage_candidate_stale") || f.children(t) != 2 {
		t.Fatalf("a candidate of the earlier lifecycle was accepted: %v", err)
	}
	// Moving the product approver role makes the confirmation unusable.
	next := f.roleSet()
	next.ProductApprover = util.MustParseUUID(f.qa)
	if _, err = f.svc.ConfigureStageRoles(f.ctx, CollaborationStageRolesParams{CollaborationWorkflowRequest: f.req(t, f.ctx), ExpectedPolicyRevision: f.policyRevision,
		ExpectedRoleRevision: 1, Roles: next}); err != nil {
		t.Fatal(err)
	}
	if view = f.view(t); view.ScopeState != "signer_changed" || !view.NeedsConfirmation {
		t.Fatalf("a moved approver role kept the confirmation: %+v", view)
	}
	if _, err = f.svc.ConfigureStageRoles(f.ctx, CollaborationStageRolesParams{CollaborationWorkflowRequest: f.req(t, f.ctx), ExpectedPolicyRevision: f.policyRevision,
		ExpectedRoleRevision: 2, Roles: f.roleSet()}); err != nil {
		t.Fatal(err)
	}
	if view = f.view(t); view.ScopeState != "signer_changed" {
		t.Fatalf("restoring the same approver account revived old authority: %+v", view)
	}
	// A new explicit confirmation, not role restoration, renews its authority.
	f.confirmScope(t, child(human))
	// Removing and re-adding the signer changes their participant generation.
	f.fx.Exec(t, `UPDATE issue_participant SET epoch=epoch+2 WHERE issue_id=$1 AND user_id=$2`, root, f.creator)
	if view = f.view(t); view.ScopeState != "signer_changed" {
		t.Fatalf("a new participant generation revived the confirmation: %+v", view)
	}
	f.fx.Exec(t, `UPDATE issue_participant SET epoch=epoch-2 WHERE issue_id=$1 AND user_id=$2`, root, f.creator)
	// Disable and re-enable: a new enabled period needs a new confirmation.
	f.revise(t, false)
	if view = f.view(t); view.Managed || view.NeedsConfirmation || view.Stage != nil || view.Capabilities.ConfirmScope || view.Scope == nil {
		t.Fatalf("disabled policy view: %+v", view)
	}
	f.revise(t, true)
	if view = f.view(t); view.ScopeState != "policy_changed" || !view.NeedsConfirmation {
		t.Fatalf("re-enabled policy kept the earlier confirmation: %+v", view)
	}
}

func stagePgCode(err error) string {
	var pgerr *pgconn.PgError
	if errors.As(err, &pgerr) {
		return pgerr.Code
	}
	return ""
}

var _ = pgtype.UUID{}
var _ = stagePgCode

// A required write that fails after children were created must leave nothing:
// no child prefix, mapping, decision, number or operation row.
func TestStageAcceptRequiredWriteFailuresAndLostCommit(t *testing.T) {
	ready := func(t *testing.T) (*collaborationStageFixture, CollaborationStageDecisionParams) {
		f := newCollaborationStageFixture(t)
		f.revise(t, true)
		f.configure(t)
		f.confirmScope(t)
		created := f.propose(t, f.handlerCtx, "Recoverable one", "Recoverable two")
		return f, f.decision(t, f.ctx, created, "accept", "")
	}
	for _, needle := range []string{"INSERT INTO collaboration_stage_child", "INSERT INTO collaboration_stage_decision", "UPDATE issue_collaboration SET revision",
		"INSERT INTO collaboration_event", "INSERT INTO collaboration_notification", "UPDATE collaboration_operation"} {
		t.Run(needle, func(t *testing.T) {
			f, accept := ready(t)
			var counter, after int64
			f.fx.QueryRow(t, `SELECT issue_counter FROM workspace WHERE id=$1`, f.workspace).Scan(&counter)
			before := f.stageRows(t)
			f.svc.TxStarter = workflowFaultStarter{TxStarter: f.pool, needle: needle}
			_, err := f.svc.DecideStageProposal(f.ctx, accept)
			f.svc.TxStarter = f.pool
			f.fx.QueryRow(t, `SELECT issue_counter FROM workspace WHERE id=$1`, f.workspace).Scan(&after)
			if err == nil || f.stageRows(t) != before || after != counter ||
				f.fx.Count(t, `SELECT count(*) FROM collaboration_operation WHERE scope_id=$1 AND request_id=$2`, f.issue.Issue.ID, accept.RequestID) != 0 {
				t.Fatalf("a failed required write reported success or kept partial acceptance state: %v\n%s\n%s", err, before, f.stageRows(t))
			}
			// The same original intent succeeds once the fault is gone.
			if _, err = f.svc.DecideStageProposal(f.ctx, accept); err != nil || f.children(t) != 2 {
				t.Fatalf("original intent after a rolled-back failure: %v children=%d", err, f.children(t))
			}
		})
	}
	t.Run("lost_commit", func(t *testing.T) {
		f, accept := ready(t)
		fault := &queueFaultStarter{TxStarter: f.pool, lostCommit: true}
		f.svc.TxStarter = fault
		_, err := f.svc.DecideStageProposal(f.ctx, accept)
		f.svc.TxStarter = f.pool
		if err == nil || fault.begins.Load() != 1 {
			t.Fatalf("an uncertain commit was reported as success or retried: %v begins=%d", err, fault.begins.Load())
		}
		recovered, err := f.svc.GetWorkflowOperation(f.ctx, f.issue.Issue.ID, accept.RequestID)
		if err != nil || recovered.Code != "stage_proposal_accepted" || recovered.StageConfirmation == nil || recovered.StageConfirmation.Decision != "accepted" ||
			recovered.StageConfirmation.CandidateID != util.UUIDToString(accept.Candidate.CandidateID) {
			t.Fatalf("original key did not recover the committed acceptance: %+v %v", recovered, err)
		}
		replay, err := f.svc.DecideStageProposal(f.ctx, accept)
		if err != nil || *replay.StageConfirmation != *recovered.StageConfirmation || f.children(t) != 2 ||
			f.fx.Count(t, `SELECT count(*) FROM collaboration_stage_decision WHERE issue_id=$1 AND step='proposal'`, f.issue.Issue.ID) != 1 {
			t.Fatalf("recovery or replay materialized again: %+v %v children=%d", replay, err, f.children(t))
		}
		// A revoked participant keeps only the minimal original receipt.
		f.fx.Exec(t, `UPDATE issue_participant SET revoked_at=now(), epoch=epoch+1 WHERE issue_id=$1 AND user_id=$2`, f.issue.Issue.ID, f.creator)
		f.fx.Exec(t, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, f.workspace, f.creator)
		if recovered, err = f.svc.GetWorkflowOperation(f.ctx, f.issue.Issue.ID, accept.RequestID); err != nil || recovered.StageConfirmation == nil {
			t.Fatalf("the original actor lost their own minimal receipt: %+v %v", recovered, err)
		}
		if _, err = f.svc.GetStageProposal(f.ctx, f.issue.Issue.ID, accept.ProposalID); !collaborationErrorCode(err, "not_found") {
			t.Fatalf("a revoked actor still read the child mapping: %v", err)
		}
		if _, err = f.svc.StageView(f.ctx, f.issue.Issue.ID); !collaborationErrorCode(err, "not_found") {
			t.Fatalf("a revoked actor still read the stage view: %v", err)
		}
	})
}

// Activation, every stage write and migration 819 must check the same exact
// index and trigger shape; a drifted copy would let one of them pass alone.
func TestStageReadinessPredicateIsSharedWithMigration(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "..", "migrations", "819_collaboration_stage_ready.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), collaborationStageReadinessSQL) {
		t.Fatal("migration 819 and the runtime readiness predicate differ")
	}
	f := newCollaborationStageFixture(t)
	var ready bool
	if err = f.pool.QueryRow(f.ctx, collaborationStageReadinessSQL).Scan(&ready); err != nil || !ready {
		t.Fatalf("the migrated task database is not stage-ready: %v %v", ready, err)
	}
}
