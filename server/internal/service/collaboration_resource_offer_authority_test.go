package service

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func resourceOfferSquad(t *testing.T, f *collaborationOfferFixture) (CollaborationOfferResourceParams, collaboration.TargetResource, CollaborationResourceIdentity) {
	t.Helper()
	f.fx.Member(t, f.execution, f.handler, "member")
	fx := testutil.New(f.pool, f.execution, f.handler)
	runtime := fx.Runtime(t, "Second owner's runtime", testutil.Cols{"owner_id": f.handler, "status": "offline"})
	agent := fx.Agent(t, "Second owner's private agent", runtime, testutil.Cols{"owner_id": f.handler})
	installation := uuid.NewString()
	binding := fx.Insert(t, "collaboration_runtime_enrollment", testutil.Cols{"deployment_id": auth.ManagedDeploymentID(), "owner_user_id": f.handler,
		"execution_workspace_id": f.execution, "runtime_id": runtime, "installation_id": installation, "public_key": make([]byte, 32)})
	squad := fx.Squad(t, "Explicit two-owner squad", f.resource.AgentID)
	fx.SquadMember(t, squad, "agent", agent)
	leader := f.resource
	leader.Role = "leader"
	member := collaboration.TargetResource{Role: "member", AgentID: agent, RuntimeID: runtime, OwnerUserID: f.handler, ExecutionWorkspaceID: f.execution}
	p := f.offerParams(t)
	p.Kind, p.ResourceID, p.Roster = collaboration.TargetSquad, util.MustParseUUID(squad), []collaboration.TargetResource{leader, member}
	identity := CollaborationResourceIdentity{Kind: "enrolled", InstallationID: util.MustParseUUID(installation), BindingID: util.MustParseUUID(binding), BindingEpoch: 1}
	return p, member, identity
}

func TestCollaborationResourceOfferSquadStillRequiresEveryOwnerConsent(t *testing.T) {
	f := newCollaborationOfferFixture(t)
	p, member, identity := resourceOfferSquad(t, f)
	offer, err := f.svc.OfferResource(collaborationContext(f.outsider), p, offerVisible)
	if err != nil {
		t.Fatal(err)
	}
	// Canonical UUIDs and agent ordering belong to the original payload digest.
	reversed := p
	reversed.Roster = append([]collaboration.TargetResource(nil), p.Roster...)
	reversed.Roster[0], reversed.Roster[1] = reversed.Roster[1], reversed.Roster[0]
	for i := range reversed.Roster {
		reversed.Roster[i].AgentID = strings.ToUpper(reversed.Roster[i].AgentID)
		reversed.Roster[i].RuntimeID = strings.ToUpper(reversed.Roster[i].RuntimeID)
		reversed.Roster[i].OwnerUserID = strings.ToUpper(reversed.Roster[i].OwnerUserID)
		reversed.Roster[i].ExecutionWorkspaceID = strings.ToUpper(reversed.Roster[i].ExecutionWorkspaceID)
	}
	replay, err := f.svc.OfferResource(collaborationContext(f.outsider), reversed, offerVisible)
	if err != nil || !reflect.DeepEqual(offer, replay) {
		t.Fatalf("equivalent roster changed the original offer: %+v %v", replay, err)
	}
	adopted, err := f.svc.AdoptResourceOffer(f.ctx, f.adoptParams(t, offer, f.creator))
	if err != nil {
		t.Fatal(err)
	}
	base := &collaborationQueueFixture{collaborationFixture: f.collaborationFixture, issue: f.issue}
	if targets := authorityCatalogue(t, base, f.ctx); len(targets) != 0 {
		t.Fatal("unconsented offered squad entered mentions")
	}
	ownerConsent := CollaborationConsentParams{CollaborationQueueRequest: f.request(t, f.outsider), TargetID: adopted.TargetID,
		AgentID: util.MustParseUUID(f.resource.AgentID), ExpectedTargetEpoch: 1, ExpectedConfigVersion: 1, Identity: f.identity}
	grant, err := f.svc.ConsentTargetResource(collaborationContext(f.outsider), ownerConsent)
	if err != nil {
		t.Fatal(err)
	}
	if len(authorityCatalogue(t, base, f.ctx)) != 0 {
		t.Fatal("supplying and consenting to the leader authorized another owner's member")
	}
	memberConsent := CollaborationConsentParams{CollaborationQueueRequest: f.request(t, f.handler), TargetID: adopted.TargetID,
		AgentID: util.MustParseUUID(member.AgentID), ExpectedTargetEpoch: 1, ExpectedConfigVersion: 1, Identity: identity}
	if _, err = f.svc.ConsentTargetResource(collaborationContext(f.handler), memberConsent); err != nil {
		t.Fatal(err)
	}
	targets := authorityCatalogue(t, base, collaborationContext(f.outsider))
	if len(targets) != 1 || targets[0].Kind != collaboration.TargetSquad || targets[0].TargetID != adopted.TargetID || targets[0].WaitingReason != "issue_paused" {
		t.Fatalf("all owners did not authorize only the original held squad: %+v", targets)
	}
	if f.fx.Count(t, "SELECT count(*) FROM collaboration_target WHERE session_id=$1", f.issue.SessionID) != 1 {
		t.Fatal("squad supply created standalone member targets")
	}
	if _, err = f.svc.WithdrawResourceOffer(collaborationContext(f.outsider), CollaborationWithdrawResourceOfferParams{CollaborationQueueRequest: f.request(t, f.outsider),
		OfferID: util.MustParseUUID(*offer.OfferID), ExpectedOfferVersion: 2}); !collaborationErrorCode(err, "offer_adopted") {
		t.Fatalf("adopted supply was used to bypass grant revocation: %v", err)
	}
	if _, err = f.svc.RevokeGrant(collaborationContext(f.outsider), CollaborationRevokeGrantParams{CollaborationQueueRequest: f.request(t, f.outsider), GrantID: grant.GrantID, ExpectedGrantEpoch: 1}); err != nil {
		t.Fatal(err)
	}
	if len(authorityCatalogue(t, base, f.ctx)) != 0 || f.fx.Count(t, "SELECT count(*) FROM collaboration_resource_offer WHERE id=$1 AND state='adopted' AND version=2", *offer.OfferID) != 1 {
		t.Fatal("grant revoke revived a pending offer or erased adopted history")
	}
}

func TestCollaborationResourceOfferEveryOwnerMustRemainCurrent(t *testing.T) {
	for _, scenario := range []string{"missing_participant", "disabled", "lost_invoke", "participant_epoch", "account_epoch", "binding_epoch"} {
		t.Run(scenario, func(t *testing.T) {
			f := newCollaborationOfferFixture(t)
			p, member, identity := resourceOfferSquad(t, f)
			offer, err := f.svc.OfferResource(collaborationContext(f.outsider), p, offerVisible)
			if err != nil {
				t.Fatal(err)
			}
			expected := "stale_offer"
			switch scenario {
			case "missing_participant":
				f.fx.Exec(t, "UPDATE issue_participant SET revoked_at=now(),epoch=epoch+1 WHERE issue_id=$1 AND user_id=$2", f.issue.Issue.ID, f.handler)
				expected = "resource_owner_not_participant"
			case "disabled":
				f.fx.Exec(t, "UPDATE \"user\" SET disabled_at=now() WHERE id=$1", f.handler)
				expected = "resource_owner_unavailable"
			case "lost_invoke":
				f.fx.Exec(t, "UPDATE agent SET owner_id=$2 WHERE id=$1", member.AgentID, f.outsider)
				expected = "target_configuration_changed"
			case "participant_epoch":
				f.fx.Exec(t, "UPDATE issue_participant SET epoch=epoch+1 WHERE issue_id=$1 AND user_id=$2", f.issue.Issue.ID, f.handler)
			case "account_epoch":
				f.fx.Exec(t, "UPDATE collaboration_account_fence SET epoch=epoch+1 WHERE user_id=$1", f.handler)
			case "binding_epoch":
				f.fx.Exec(t, "UPDATE collaboration_runtime_enrollment SET binding_epoch=binding_epoch+1 WHERE id=$1", identity.BindingID)
			}
			if _, err = f.svc.AdoptResourceOffer(f.ctx, f.adoptParams(t, offer, f.creator)); !collaborationErrorCode(err, expected) {
				t.Fatalf("non-provider owner's %s was not revalidated: %v", scenario, err)
			}
			if f.fx.Count(t, "SELECT count(*) FROM collaboration_target WHERE session_id=$1", f.issue.SessionID) != 0 {
				t.Fatal("invalid member owner left a partial squad target")
			}
		})
	}
}

func TestCollaborationResourceOfferHidesIncompleteSquadAndRetainsOriginalRoster(t *testing.T) {
	f := newCollaborationOfferFixture(t)
	p, hidden, _ := resourceOfferSquad(t, f)
	visible := func(_ db.Member, agent db.Agent, _ db.AgentRuntime, _ []db.AgentInvocationTarget) bool {
		return util.UUIDToString(agent.ID) != hidden.AgentID
	}
	options, err := f.svc.ResourceOfferOptions(collaborationContext(f.outsider), f.issue.Issue.ID, util.MustParseUUID(f.execution), visible)
	if err != nil || len(options.Items) != 1 || options.Items[0].Kind != collaboration.TargetAgent {
		t.Fatalf("hidden member disclosed a partial squad: %+v %v", options, err)
	}
	if _, err = f.svc.OfferResource(collaborationContext(f.outsider), p, visible); !collaborationErrorCode(err, "resource_options_unavailable") {
		t.Fatalf("POST bypassed complete roster visibility: %v", err)
	}
	p.RequestID = uuid.NewString()
	offer, err := f.svc.OfferResource(collaborationContext(f.outsider), p, offerVisible)
	if err != nil {
		t.Fatal(err)
	}
	fx := testutil.New(f.pool, f.execution, f.handler)
	runtime := fx.Runtime(t, "New undisclosed runtime", testutil.Cols{"owner_id": f.handler})
	agent := fx.Agent(t, "UNDISCLOSED_NEW_SQUAD_MEMBER", runtime, testutil.Cols{"owner_id": f.handler})
	fx.SquadMember(t, util.UUIDToString(p.ResourceID), "agent", agent)
	view, err := f.svc.GetResourceOffer(f.ctx, f.issue.Issue.ID, util.MustParseUUID(*offer.OfferID))
	if err != nil || view.Available || len(view.Resources) != 2 {
		t.Fatalf("stale offer did not retain its original roster: %+v %v", view, err)
	}
	encoded, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), agent) || strings.Contains(string(encoded), runtime) || strings.Contains(string(encoded), "UNDISCLOSED_NEW_SQUAD_MEMBER") {
		t.Fatal("current squad roster expanded an old offer disclosure")
	}
}

func TestCollaborationResourceOfferIdentityAvailabilityAndPresentation(t *testing.T) {
	for _, scenario := range []string{"missing", "binding_replaced", "display_only"} {
		t.Run(scenario, func(t *testing.T) {
			f := newCollaborationOfferFixture(t)
			offer := f.offer(t)
			switch scenario {
			case "missing":
				f.fx.Exec(t, "UPDATE collaboration_runtime_enrollment SET revoked_at=now() WHERE id=$1", f.identity.BindingID)
			case "binding_replaced":
				// The active-runtime unique index excludes two simultaneous
				// enrollments. A real replacement must revoke the old identity.
				f.fx.Exec(t, "UPDATE collaboration_runtime_enrollment SET revoked_at=now() WHERE id=$1", f.identity.BindingID)
				f.fx.Insert(t, "collaboration_runtime_enrollment", testutil.Cols{"deployment_id": auth.ManagedDeploymentID(), "owner_user_id": f.outsider,
					"execution_workspace_id": f.execution, "runtime_id": f.resource.RuntimeID, "installation_id": uuid.NewString(), "public_key": make([]byte, 32)})
			case "display_only":
				f.fx.Exec(t, "UPDATE agent SET name='New display name',max_concurrent_tasks=3 WHERE id=$1", f.resource.AgentID)
				f.fx.Exec(t, "UPDATE agent_runtime SET name='New machine name',status='offline',last_seen_at=now() WHERE id=$1", f.resource.RuntimeID)
			}
			options, err := f.svc.ResourceOfferOptions(collaborationContext(f.outsider), f.issue.Issue.ID, util.MustParseUUID(f.execution), offerVisible)
			if err != nil || len(options.Items) != 1 {
				t.Fatalf("candidate availability: %+v %v", options, err)
			}
			if scenario == "display_only" {
				if !options.Items[0].Available {
					t.Fatal("presentation or presence invalidated supply authority")
				}
				if _, err = f.svc.AdoptResourceOffer(f.ctx, f.adoptParams(t, offer, f.creator)); err != nil {
					t.Fatalf("unchanged offered authority could not be adopted offline: %v", err)
				}
				return
			}
			if scenario == "binding_replaced" {
				if !options.Items[0].Available {
					t.Fatal("new binding was not available for a fresh explicit offer")
				}
				if _, err = f.svc.AdoptResourceOffer(f.ctx, f.adoptParams(t, offer, f.creator)); !collaborationErrorCode(err, "stale_offer") {
					t.Fatalf("replacement binding revived an old offer: %v", err)
				}
				return
			}
			if options.Items[0].Available || options.Items[0].Reason != "runtime_registration_required" {
				t.Fatalf("missing enrollment was guessed: %+v", options.Items[0])
			}
			if _, err = f.svc.OfferResource(collaborationContext(f.outsider), f.offerParams(t), offerVisible); !collaborationErrorCode(err, "runtime_registration_required") {
				t.Fatalf("write accepted unavailable identity: %v", err)
			}
			if _, err = f.svc.AdoptResourceOffer(f.ctx, f.adoptParams(t, offer, f.creator)); !collaborationErrorCode(err, "runtime_registration_required") {
				t.Fatalf("adoption reused a now unavailable identity: %v", err)
			}
		})
	}
}

func TestCollaborationResourceOfferSupplyFencesDirectoryRoleUntilCommit(t *testing.T) {
	f := newCollaborationOfferFixture(t)
	p, _, _ := resourceOfferSquad(t, f)
	ctx, cancel := context.WithTimeout(collaborationContext(f.outsider), 15*time.Second)
	defer cancel()
	release := make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	defer unblock()
	barrier := recipientCommitStarter{TxStarter: f.pool, reached: make(chan int, 1), release: release}
	svc := *f.svc
	svc.TxStarter = barrier
	firstDone := make(chan error, 1)
	go func() { _, err := svc.OfferResource(ctx, p, offerVisible); firstDone <- err }()
	var blocker int
	select {
	case blocker = <-barrier.reached:
	case err := <-firstDone:
		t.Fatalf("supply did not reach commit: %v", err)
	case <-ctx.Done():
		t.Fatal("supply timed out")
	}
	secondDone := make(chan error, 1)
	go func() {
		_, err := f.pool.Exec(ctx, "UPDATE member SET role='member' WHERE workspace_id=$1 AND user_id=$2", f.execution, f.outsider)
		secondDone <- err
	}()
	waitFIFOBlocked(t, &collaborationQueueFixture{collaborationFixture: f.collaborationFixture, issue: f.issue}, blocker, secondDone)
	unblock()
	if firstErr, secondErr := <-firstDone, <-secondDone; firstErr != nil || secondErr != nil {
		t.Fatalf("role change and exact supply did not serialize: %v / %v", firstErr, secondErr)
	}
}

func TestCollaborationResourceOfferAllOwnersAreEligibleBeforeSupply(t *testing.T) {
	for _, scenario := range []string{"missing_participant", "disabled_owner", "missing_invoke"} {
		t.Run(scenario, func(t *testing.T) {
			f := newCollaborationOfferFixture(t)
			p, member, _ := resourceOfferSquad(t, f)
			code := "resource_owner_not_participant"
			switch scenario {
			case "missing_participant":
				f.fx.Exec(t, "UPDATE issue_participant SET revoked_at=now(),epoch=epoch+1 WHERE issue_id=$1 AND user_id=$2", f.issue.Issue.ID, f.handler)
			case "disabled_owner":
				f.fx.Exec(t, "UPDATE \"user\" SET disabled_at=now() WHERE id=$1", f.handler)
				code = "resource_owner_unavailable"
			case "missing_invoke":
				f.fx.Exec(t, "UPDATE agent SET owner_id=$2 WHERE id=$1", member.AgentID, f.outsider)
				code = "target_configuration_changed"
			}
			if _, err := f.svc.OfferResource(collaborationContext(f.outsider), p, offerVisible); !collaborationErrorCode(err, code) {
				t.Fatalf("ineligible other owner was allowed into an offer: %v", err)
			}
			if f.fx.Count(t, "SELECT count(*) FROM collaboration_resource_offer WHERE issue_id=$1", f.issue.Issue.ID) != 0 {
				t.Fatal("supply deferred owner eligibility until adoption")
			}
		})
	}
}

func TestCollaborationResourceOfferTargetSetConfirmationAndExistingTarget(t *testing.T) {
	f := newCollaborationOfferFixture(t)
	offer := f.offer(t)
	old := f.adoptParams(t, offer, f.creator)
	fx := testutil.New(f.pool, f.execution, f.outsider)
	agent := fx.Agent(t, "Independently registered target", f.resource.RuntimeID)
	resource := f.resource
	resource.AgentID = agent
	if _, err := f.svc.RegisterTarget(f.ctx, CollaborationRegisterTargetParams{CollaborationQueueRequest: f.request(t, f.creator),
		Kind: collaboration.TargetAgent, ResourceID: util.MustParseUUID(agent), ExpectedTargetSetVersion: 1, Roster: []collaboration.TargetResource{resource}}); err != nil {
		t.Fatal(err)
	}
	view, err := f.svc.GetResourceOffer(f.ctx, f.issue.Issue.ID, old.OfferID)
	if err != nil || !view.Available {
		t.Fatalf("unrelated target registration invalidated an offer: %+v %v", view, err)
	}
	if _, err = f.svc.AdoptResourceOffer(f.ctx, old); !collaborationErrorCode(err, "stale_scope") {
		t.Fatalf("adoption ignored its observed target set: %v", err)
	}
	adopted, err := f.svc.AdoptResourceOffer(f.ctx, f.adoptParams(t, offer, f.creator))
	if err != nil {
		t.Fatal(err)
	}
	refused, err := f.svc.GetResourceOperation(f.ctx, f.issue.Issue.ID, old.RequestID)
	if err != nil || refused.Code != "stale_scope" || refused.Status != 409 {
		t.Fatalf("later adoption rewrote an earlier refused operation: %+v %v", refused, err)
	}
	other := f.offer(t)
	if _, err = f.svc.AdoptResourceOffer(f.ctx, f.adoptParams(t, other, f.creator)); !collaborationErrorCode(err, "target_exists") {
		t.Fatalf("another offer replaced an existing typed target: %v", err)
	}
	if f.fx.Count(t, "SELECT count(*) FROM collaboration_resource_offer WHERE id=$1 AND state='offered' AND version=1", *other.OfferID) != 1 ||
		f.fx.Count(t, "SELECT count(*) FROM collaboration_target WHERE id=$1 AND target_epoch=1 AND config_version=1", adopted.TargetID) != 1 {
		t.Fatal("duplicate typed target turned adoption into implicit Switch")
	}
}

func TestCollaborationResourceOfferClosedIssueAllowsOnlyOwnWithdrawal(t *testing.T) {
	f := newCollaborationOfferFixture(t)
	offer := f.offer(t)
	if err := f.svc.Queries.SeedIssueStatusEntries(f.ctx, util.MustParseUUID(f.workspace)); err != nil {
		t.Fatal(err)
	}
	f.fx.Cleanup(t, "DELETE FROM issue_status WHERE workspace_id=$1", f.workspace)
	f.changeLifecycle(t, "complete", "done")
	view, err := f.svc.GetResourceOffer(collaborationContext(f.outsider), f.issue.Issue.ID, util.MustParseUUID(*offer.OfferID))
	if err != nil || !view.CanWithdraw || view.CanAdopt || view.Available || view.Reason != "issue_closed" {
		t.Fatalf("closed issue erased current owner's withdrawal: %+v %v", view, err)
	}
	if _, err = f.svc.OfferResource(collaborationContext(f.outsider), f.offerParams(t), offerVisible); !collaborationErrorCode(err, "issue_closed") {
		t.Fatalf("closed issue accepted new supply: %v", err)
	}
	if _, err = f.svc.AdoptResourceOffer(f.ctx, f.adoptParams(t, offer, f.creator)); !collaborationErrorCode(err, "issue_closed") {
		t.Fatalf("closed issue accepted adoption: %v", err)
	}
	withdraw := CollaborationWithdrawResourceOfferParams{CollaborationQueueRequest: f.request(t, f.creator), OfferID: view.OfferID, ExpectedOfferVersion: 1}
	if _, err = f.svc.WithdrawResourceOffer(f.ctx, withdraw); !collaborationErrorCode(err, "resource_offer_not_found") {
		t.Fatalf("creator withdrew another owner's unadopted offer: %v", err)
	}
	withdraw.CollaborationQueueRequest = f.request(t, f.outsider)
	if _, err = f.svc.WithdrawResourceOffer(collaborationContext(f.outsider), withdraw); err != nil {
		t.Fatalf("closed issue's stale offer could not be withdrawn: %v", err)
	}
}
