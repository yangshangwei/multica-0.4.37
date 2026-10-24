package service

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type collaborationOfferFixture struct {
	*collaborationFixture
	issue     CollaborationIssueAccess
	execution string
	resource  collaboration.TargetResource
	identity  CollaborationResourceIdentity
}

func newCollaborationOfferFixture(t *testing.T) *collaborationOfferFixture {
	t.Helper()
	f := newCollaborationFixture(t)
	t.Setenv("MULTICA_DEPLOYMENT_ID", "68150177-594d-45d8-8a84-bc979b498b04")
	issue, err := f.svc.Create(f.ctx, f.params(t))
	if err != nil {
		t.Fatal(err)
	}
	execution := f.fx.Workspace(t, "Private offered resources", "offers-"+uuid.NewString())
	f.fx.Member(t, execution, f.outsider, "owner")
	fx := testutil.New(f.pool, execution, f.outsider)
	runtime := fx.Runtime(t, "Offered offline runtime", testutil.Cols{"owner_id": f.outsider, "status": "offline"})
	agent := fx.Agent(t, "Explicit offered agent", runtime, testutil.Cols{"owner_id": f.outsider, "instructions": "PRIVATE_OFFER_CONFIGURATION"})
	installation := uuid.NewString()
	binding := fx.Insert(t, "collaboration_runtime_enrollment", testutil.Cols{
		"deployment_id": auth.ManagedDeploymentID(), "owner_user_id": f.outsider, "execution_workspace_id": execution,
		"runtime_id": runtime, "installation_id": installation, "public_key": make([]byte, 32),
	})
	for _, table := range []string{"collaboration_resource_offer", "collaboration_grant"} {
		f.fx.Cleanup(t, "DELETE FROM "+table+" WHERE issue_id=$1", issue.Issue.ID)
	}
	for _, table := range []string{"collaboration_target", "collaboration_channel", "collaboration_command"} {
		f.fx.Cleanup(t, "DELETE FROM "+table+" WHERE session_id=$1", issue.SessionID)
	}
	f.fx.Cleanup(t, "DELETE FROM collaboration_operation WHERE scope_kind='issue' AND scope_id=$1", issue.Issue.ID)
	p := CollaborationParticipantParams{CollaborationWorkflowRequest: CollaborationWorkflowRequest{
		CollaborationQueueRequest:     CollaborationQueueRequest{IssueID: issue.Issue.ID, RequestID: uuid.NewString(), ExpectedLifecycleEpoch: 1, ExpectedParticipantEpoch: 1, ExpectedAccountEpoch: 1},
		ExpectedCollaborationRevision: 1}, TargetUserID: util.MustParseUUID(f.outsider)}
	if _, err = f.svc.InviteParticipant(f.ctx, p); err != nil {
		t.Fatal(err)
	}
	return &collaborationOfferFixture{collaborationFixture: f, issue: issue, execution: execution,
		resource: collaboration.TargetResource{Role: "agent", AgentID: agent, RuntimeID: runtime, OwnerUserID: f.outsider, ExecutionWorkspaceID: execution},
		identity: CollaborationResourceIdentity{Kind: "enrolled", InstallationID: util.MustParseUUID(installation), BindingID: util.MustParseUUID(binding), BindingEpoch: 1}}
}

func (f *collaborationOfferFixture) request(t *testing.T, actor string) CollaborationQueueRequest {
	t.Helper()
	access, err := f.svc.ResolveIssueAccess(collaborationContext(actor), nil, f.issue.Issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	participant, err := f.svc.Queries.GetCollaborationParticipant(f.ctx, db.GetCollaborationParticipantParams{IssueID: f.issue.Issue.ID, UserID: util.MustParseUUID(actor)})
	if err != nil {
		t.Fatal(err)
	}
	account, err := f.svc.Queries.GetCollaborationAccountFence(f.ctx, util.MustParseUUID(actor))
	if err != nil {
		t.Fatal(err)
	}
	return CollaborationQueueRequest{IssueID: f.issue.Issue.ID, RequestID: uuid.NewString(), ExpectedLifecycleEpoch: access.LifecycleEpoch,
		ExpectedParticipantEpoch: participant.Epoch, ExpectedAccountEpoch: account.Epoch}
}

// Handler tests exercise the real directory predicate. Service tests supply a
// deliberately permissive adapter so actual ownership must be checked here.
func offerVisible(db.Member, db.Agent, db.AgentRuntime, []db.AgentInvocationTarget) bool { return true }

func (f *collaborationOfferFixture) offerParams(t *testing.T) CollaborationOfferResourceParams {
	t.Helper()
	return CollaborationOfferResourceParams{CollaborationQueueRequest: f.request(t, f.outsider), ExecutionWorkspaceID: util.MustParseUUID(f.execution),
		Kind: collaboration.TargetAgent, ResourceID: util.MustParseUUID(f.resource.AgentID), Roster: []collaboration.TargetResource{f.resource}}
}

func (f *collaborationOfferFixture) offer(t *testing.T) CollaborationQueueReceipt {
	t.Helper()
	r, err := f.svc.OfferResource(collaborationContext(f.outsider), f.offerParams(t), offerVisible)
	if err != nil || r.OfferID == nil || r.OfferVersion != 1 {
		t.Fatalf("provide an owned resource: %+v %v", r, err)
	}
	return r
}

func (f *collaborationOfferFixture) adoptParams(t *testing.T, offer CollaborationQueueReceipt, actor string) CollaborationAdoptResourceOfferParams {
	t.Helper()
	session, err := f.svc.Queries.GetCollaborationSession(f.ctx, f.issue.Issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	return CollaborationAdoptResourceOfferParams{CollaborationQueueRequest: f.request(t, actor), OfferID: util.MustParseUUID(*offer.OfferID),
		ExpectedOfferVersion: offer.OfferVersion, ExpectedTargetSetVersion: session.TargetSetVersion}
}

func TestCollaborationResourceOfferSeparatesPrivateOwnerAndOrganizer(t *testing.T) {
	f := newCollaborationOfferFixture(t)
	ownerCtx := collaborationContext(f.outsider)
	for _, actor := range []string{f.creator, f.handler} {
		if _, err := f.svc.ResourceOptions(collaborationContext(actor), f.issue.Issue.ID, util.MustParseUUID(f.execution), offerVisible); !collaborationErrorCode(err, "resource_workspace_unavailable") {
			t.Fatalf("organizer gained private directory membership: %v", err)
		}
	}
	options, err := f.svc.ResourceOfferOptions(ownerCtx, f.issue.Issue.ID, util.MustParseUUID(f.execution), offerVisible)
	if err != nil || len(options.Items) != 1 || !options.Items[0].Available {
		t.Fatalf("source-hidden ordinary participant cannot offer own resource: %+v %v", options, err)
	}
	params := f.offerParams(t)
	offered, err := f.svc.OfferResource(ownerCtx, params, offerVisible)
	if err != nil || offered.Status != http.StatusCreated || offered.Code != "resource_offered" || offered.OfferID == nil || offered.Action != "offer_resource" {
		t.Fatalf("offer: %+v %v", offered, err)
	}
	replayed, err := f.svc.OfferResource(ownerCtx, params, offerVisible)
	if err != nil || !reflect.DeepEqual(replayed, offered) {
		t.Fatalf("offer replay changed the original result: %+v %v", replayed, err)
	}
	if f.fx.Count(t, "SELECT count(*) FROM collaboration_target WHERE session_id=$1", f.issue.SessionID) != 0 ||
		f.fx.Count(t, "SELECT count(*) FROM collaboration_grant WHERE issue_id=$1", f.issue.Issue.ID) != 0 {
		t.Fatal("offering implicitly registered a target or consented")
	}
	unauthorized := f.adoptParams(t, offered, f.outsider)
	if _, err = f.svc.AdoptResourceOffer(ownerCtx, unauthorized); !collaborationErrorCode(err, "forbidden") {
		t.Fatalf("ordinary owner acquired organizer authority: %v", err)
	}
	adopt := f.adoptParams(t, offered, f.handler)
	adopted, err := f.svc.AdoptResourceOffer(collaborationContext(f.handler), adopt)
	if err != nil || adopted.Code != "resource_offer_adopted" || !adopted.TargetID.Valid || adopted.OfferVersion != 2 {
		t.Fatalf("outside organizer could not adopt selected resource: %+v %v", adopted, err)
	}
	if f.fx.Count(t, "SELECT count(*) FROM collaboration_target WHERE id=$1 AND queue_state='paused' AND hold_reason='initial'", adopted.TargetID) != 1 ||
		f.fx.Count(t, "SELECT count(*) FROM collaboration_grant WHERE issue_id=$1", f.issue.Issue.ID) != 0 ||
		f.fx.Count(t, "SELECT count(*) FROM collaboration_command WHERE session_id=$1", f.issue.SessionID) != 0 {
		t.Fatal("adoption changed consent, queue or initial hold")
	}
	consent := CollaborationConsentParams{CollaborationQueueRequest: f.request(t, f.creator), TargetID: adopted.TargetID,
		AgentID: util.MustParseUUID(f.resource.AgentID), ExpectedTargetEpoch: 1, ExpectedConfigVersion: 1, Identity: f.identity}
	if _, err = f.svc.ConsentTargetResource(f.ctx, consent); !collaborationErrorCode(err, "forbidden") {
		t.Fatalf("creator consented for another owner: %v", err)
	}
	consent.CollaborationQueueRequest = f.request(t, f.outsider)
	if _, err = f.svc.ConsentTargetResource(ownerCtx, consent); err != nil {
		t.Fatal(err)
	}
	view, err := f.svc.Resources(collaborationContext(f.handler), f.issue.Issue.ID)
	if err != nil || len(view.Targets) != 1 || !view.Targets[0].Authorized {
		t.Fatalf("independent owner consent did not authorize target: %+v %v", view, err)
	}
	for _, actor := range []string{f.outsider, f.creator, f.handler} {
		item, err := f.svc.GetResourceOffer(collaborationContext(actor), f.issue.Issue.ID, util.MustParseUUID(*offered.OfferID))
		if err != nil || item.State != "adopted" || item.TargetID != adopted.TargetID || item.CanWithdraw || item.CanAdopt {
			t.Fatalf("retained adopted fact: %+v %v", item, err)
		}
		body, err := json.Marshal(item)
		if err != nil {
			t.Fatal(err)
		}
		for _, private := range []string{"configuration_fingerprint", "resource_authority", "grant_id", "installation_id", "binding_id", "PRIVATE_OFFER_CONFIGURATION", util.UUIDToString(f.identity.InstallationID)} {
			if strings.Contains(string(body), private) {
				t.Fatalf("offer projection disclosed %q", private)
			}
		}
	}
}

func TestCollaborationResourceOfferRequiresActualOwnerEvenForVisibleResources(t *testing.T) {
	f := newCollaborationOfferFixture(t)
	f.fx.Member(t, f.execution, f.handler, "admin")
	p := f.offerParams(t)
	p.CollaborationQueueRequest = f.request(t, f.handler)
	if _, err := f.svc.OfferResource(collaborationContext(f.handler), p, offerVisible); !collaborationErrorCode(err, "resource_not_owned") {
		t.Fatalf("visible workspace admin offered another owner's resource: %v", err)
	}
	f.fx.Exec(t, "UPDATE agent SET owner_id=$2 WHERE id=$1", f.resource.AgentID, f.handler)
	p.CollaborationQueueRequest = f.request(t, f.handler)
	if _, err := f.svc.OfferResource(collaborationContext(f.handler), p, offerVisible); !collaborationErrorCode(err, "resource_not_owned") {
		t.Fatalf("agent ownership substituted for runtime ownership: %v", err)
	}
	if f.fx.Count(t, "SELECT count(*) FROM collaboration_resource_offer WHERE issue_id=$1", f.issue.Issue.ID) != 0 {
		t.Fatal("non-owner supplied a resource")
	}
}

func (f *collaborationOfferFixture) workflowRequest(t *testing.T, actor string) CollaborationWorkflowRequest {
	t.Helper()
	access, err := f.svc.ResolveIssueAccess(collaborationContext(actor), nil, f.issue.Issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	return CollaborationWorkflowRequest{CollaborationQueueRequest: f.request(t, actor), ExpectedCollaborationRevision: access.Revision}
}

func (f *collaborationOfferFixture) changeLifecycle(t *testing.T, action, status string) {
	t.Helper()
	options, err := f.svc.StatusOptions(f.ctx, f.issue.Issue.ID, action)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.ChangeLifecycle(f.ctx, CollaborationLifecycleParams{CollaborationWorkflowRequest: f.workflowRequest(t, f.creator),
		Action: action, TargetStatus: status, StatusOptionsVersion: options.StatusOptionsVersion}); err != nil {
		t.Fatal(err)
	}
}

func TestCollaborationResourceOfferFrozenAuthorityCannotRevive(t *testing.T) {
	for _, scenario := range []string{"participant_rejoin", "account_restore", "binding_epoch", "configuration_restore", "runtime_owner_restore", "close_reopen"} {
		t.Run(scenario, func(t *testing.T) {
			f := newCollaborationOfferFixture(t)
			offer := f.offer(t)
			switch scenario {
			case "participant_rejoin":
				if _, err := f.svc.Leave(collaborationContext(f.outsider), f.workflowRequest(t, f.outsider)); err != nil {
					t.Fatal(err)
				}
				if _, err := f.svc.InviteParticipant(f.ctx, CollaborationParticipantParams{CollaborationWorkflowRequest: f.workflowRequest(t, f.creator), TargetUserID: util.MustParseUUID(f.outsider)}); err != nil {
					t.Fatal(err)
				}
			case "account_restore":
				f.fx.Exec(t, "UPDATE collaboration_account_fence SET epoch=epoch+1 WHERE user_id=$1", f.outsider)
			case "binding_epoch":
				f.fx.Exec(t, "UPDATE collaboration_runtime_enrollment SET binding_epoch=binding_epoch+1 WHERE id=$1", f.identity.BindingID)
			case "configuration_restore":
				f.fx.Exec(t, "UPDATE agent SET instructions='Changed execution configuration' WHERE id=$1", f.resource.AgentID)
				f.fx.Exec(t, "UPDATE agent SET instructions='PRIVATE_OFFER_CONFIGURATION' WHERE id=$1", f.resource.AgentID)
			case "runtime_owner_restore":
				f.fx.Member(t, f.execution, f.handler, "member")
				f.fx.Exec(t, "UPDATE agent_runtime SET owner_id=$2 WHERE id=$1", f.resource.RuntimeID, f.handler)
				f.fx.Exec(t, "UPDATE agent_runtime SET owner_id=$2 WHERE id=$1", f.resource.RuntimeID, f.outsider)
			case "close_reopen":
				if err := f.svc.Queries.SeedIssueStatusEntries(f.ctx, util.MustParseUUID(f.workspace)); err != nil {
					t.Fatal(err)
				}
				f.fx.Cleanup(t, "DELETE FROM issue_status WHERE workspace_id=$1", f.workspace)
				f.changeLifecycle(t, "complete", "done")
				f.changeLifecycle(t, "reopen", "todo")
			}
			p := f.adoptParams(t, offer, f.handler)
			if _, err := f.svc.AdoptResourceOffer(collaborationContext(f.handler), p); !collaborationErrorCode(err, "stale_offer") {
				t.Fatalf("old resource offer revived after %s: %v", scenario, err)
			}
			view, err := f.svc.GetResourceOffer(f.ctx, f.issue.Issue.ID, p.OfferID)
			if err != nil || view.Available || view.Reason != "stale_offer" || view.State != "offered" || view.Version != 1 {
				t.Fatalf("stale read did not preserve immutable offer: %+v %v", view, err)
			}
			withdrawn, err := f.svc.WithdrawResourceOffer(collaborationContext(f.outsider), CollaborationWithdrawResourceOfferParams{
				CollaborationQueueRequest: f.request(t, f.outsider), OfferID: p.OfferID, ExpectedOfferVersion: 1})
			if err != nil || withdrawn.OfferVersion != 2 || withdrawn.Code != "resource_offer_withdrawn" {
				t.Fatalf("current owner could not withdraw obsolete disclosure: %+v %v", withdrawn, err)
			}
			if f.fx.Count(t, "SELECT count(*) FROM collaboration_target WHERE session_id=$1", f.issue.SessionID) != 0 {
				t.Fatal("stale offer left a partial target")
			}
		})
	}
}

func TestCollaborationResourceOfferReadsApplyViewerPredicateBeforeLimit(t *testing.T) {
	f := newCollaborationOfferFixture(t)
	owned := f.offer(t)
	f.fx.Member(t, f.execution, f.handler, "member")
	fx := testutil.New(f.pool, f.execution, f.handler)
	runtime := fx.Runtime(t, "Another participant runtime", testutil.Cols{"owner_id": f.handler})
	agent := fx.Agent(t, "Another participant agent", runtime, testutil.Cols{"owner_id": f.handler})
	fx.Insert(t, "collaboration_runtime_enrollment", testutil.Cols{"deployment_id": auth.ManagedDeploymentID(), "owner_user_id": f.handler,
		"execution_workspace_id": f.execution, "runtime_id": runtime, "installation_id": uuid.NewString(), "public_key": make([]byte, 32)})
	for i := 0; i < 3; i++ {
		p := CollaborationOfferResourceParams{CollaborationQueueRequest: f.request(t, f.handler), ExecutionWorkspaceID: util.MustParseUUID(f.execution), Kind: collaboration.TargetAgent,
			ResourceID: util.MustParseUUID(agent), Roster: []collaboration.TargetResource{{Role: "agent", AgentID: agent, RuntimeID: runtime, OwnerUserID: f.handler, ExecutionWorkspaceID: f.execution}}}
		if _, err := f.svc.OfferResource(collaborationContext(f.handler), p, offerVisible); err != nil {
			t.Fatal(err)
		}
	}
	page, err := f.svc.ListResourceOffers(collaborationContext(f.outsider), f.issue.Issue.ID, CollaborationResourceOfferQuery{Limit: 1})
	if err != nil || len(page.Items) != 1 || page.HasMore || page.Items[0].OfferID != util.MustParseUUID(*owned.OfferID) || !page.Items[0].CanWithdraw || page.Items[0].CanAdopt {
		t.Fatalf("private newer offers consumed the ordinary owner's page: %+v %v", page, err)
	}
	page, err = f.svc.ListResourceOffers(f.ctx, f.issue.Issue.ID, CollaborationResourceOfferQuery{Limit: 2})
	if err != nil || len(page.Items) != 2 || !page.HasMore {
		t.Fatalf("organizer page: %+v %v", page, err)
	}
	last := page.Items[1]
	next, err := f.svc.ListResourceOffers(f.ctx, f.issue.Issue.ID, CollaborationResourceOfferQuery{Limit: 2,
		BeforeCreatedAt: pgtype.Timestamptz{Time: last.CreatedAt, Valid: true}, BeforeID: last.OfferID})
	if err != nil || len(next.Items) != 2 || next.HasMore || next.Items[1].OfferID != util.MustParseUUID(*owned.OfferID) {
		t.Fatalf("keyset page lost or repeated offers: %+v %v", next, err)
	}
	f.fx.Cleanup(t, "DELETE FROM issue_handoff WHERE issue_id=$1", f.issue.Issue.ID)
	if _, err = f.svc.Transfer(collaborationContext(f.handler), CollaborationTransferParams{CollaborationParticipantParams: CollaborationParticipantParams{
		CollaborationWorkflowRequest: f.workflowRequest(t, f.handler), TargetUserID: util.MustParseUUID(f.creator)}, Note: "Return organization to creator"}); err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.GetResourceOffer(collaborationContext(f.handler), f.issue.Issue.ID, util.MustParseUUID(*owned.OfferID)); !collaborationErrorCode(err, "resource_offer_not_found") {
		t.Fatalf("former handler retained another owner's pending offer: %v", err)
	}
	page, err = f.svc.ListResourceOffers(collaborationContext(f.handler), f.issue.Issue.ID, CollaborationResourceOfferQuery{Limit: 100})
	if err != nil || len(page.Items) != 3 {
		t.Fatalf("former handler lost own offers or retained organizer visibility: %+v %v", page, err)
	}
}

func TestCollaborationResourceOfferAdoptAndWithdrawSerialize(t *testing.T) {
	for _, first := range []string{"adopt", "withdraw", "second_adopt"} {
		t.Run(first, func(t *testing.T) {
			f := newCollaborationOfferFixture(t)
			offer := f.offer(t)
			adopt := f.adoptParams(t, offer, f.handler)
			withdraw := CollaborationWithdrawResourceOfferParams{CollaborationQueueRequest: f.request(t, f.outsider), OfferID: adopt.OfferID, ExpectedOfferVersion: 1}
			actorCtx, cancel := context.WithTimeout(collaborationContext(f.handler), 15*time.Second)
			defer cancel()
			ownerCtx, cancelOwner := context.WithTimeout(collaborationContext(f.outsider), 15*time.Second)
			defer cancelOwner()
			apply := func(s *CollaborationService) error { _, err := s.AdoptResourceOffer(actorCtx, adopt); return err }
			retract := func(s *CollaborationService) error { _, err := s.WithdrawResourceOffer(ownerCtx, withdraw); return err }
			leading, following := apply, retract
			if first == "withdraw" {
				leading, following = retract, apply
			} else if first == "second_adopt" {
				other := f.adoptParams(t, offer, f.creator)
				following = func(s *CollaborationService) error { _, err := s.AdoptResourceOffer(f.ctx, other); return err }
			}
			release := make(chan struct{})
			var once sync.Once
			unblock := func() { once.Do(func() { close(release) }) }
			defer unblock()
			barrier := recipientCommitStarter{TxStarter: f.pool, reached: make(chan int, 1), release: release}
			svc := *f.svc
			svc.TxStarter = barrier
			firstDone := make(chan error, 1)
			go func() { firstDone <- leading(&svc) }()
			var blocker int
			select {
			case blocker = <-barrier.reached:
			case err := <-firstDone:
				t.Fatalf("first writer did not reach its commit fence: %v", err)
			case <-actorCtx.Done():
				t.Fatal("first writer timed out")
			}
			secondDone := make(chan error, 1)
			go func() { secondDone <- following(f.svc) }()
			waitFIFOBlocked(t, &collaborationQueueFixture{collaborationFixture: f.collaborationFixture, issue: f.issue}, blocker, secondDone)
			unblock()
			firstErr, secondErr := <-firstDone, <-secondDone
			if firstErr != nil || !collaborationErrorCode(secondErr, "stale_offer") {
				t.Fatalf("competing offer transitions did not serialize: %v / %v", firstErr, secondErr)
			}
			state, count := "adopted", 1
			if first == "withdraw" {
				state, count = "withdrawn", 0
			}
			if f.fx.Count(t, "SELECT count(*) FROM collaboration_resource_offer WHERE id=$1 AND state=$2 AND version=2", adopt.OfferID, state) != 1 ||
				f.fx.Count(t, "SELECT count(*) FROM collaboration_target WHERE session_id=$1", f.issue.SessionID) != count ||
				f.fx.Count(t, "SELECT count(*) FROM collaboration_grant WHERE issue_id=$1", f.issue.Issue.ID) != 0 {
				t.Fatal("competing transitions duplicated or partially adopted the resource")
			}
		})
	}
}
