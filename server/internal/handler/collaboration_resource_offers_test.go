package handler

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
)

type resourceOfferHTTPFixture struct {
	*configHTTPFixture
	owner, handler, execution, ownedRuntime, ownedAgent, hiddenAgent string
}

func newResourceOfferHTTPFixture(t *testing.T) *resourceOfferHTTPFixture {
	t.Helper()
	base := newConfigHTTPFixture(t)
	t.Setenv("MULTICA_DEPLOYMENT_ID", "68150177-594d-45d8-8a84-bc979b498b04")
	owner := dbfx.User(t, "Private resource provider", "offer-owner-"+uuid.NewString()+"@example.test")
	handler := dbfx.User(t, "Outside resource organizer", "offer-organizer-"+uuid.NewString()+"@example.test")
	execution := dbfx.Workspace(t, "Private owner inventory", "offer-inventory-"+uuid.NewString())
	dbfx.Member(t, execution, owner, "owner")
	for _, actor := range []string{owner, handler} {
		dbfx.InsertNoID(t, "collaboration_account_fence", testutil.Cols{"user_id": actor}, "user_id=$1", actor)
		dbfx.InsertNoID(t, "issue_participant", testutil.Cols{"issue_id": base.issue, "user_id": actor}, "issue_id=$1 AND user_id=$2", base.issue, actor)
	}
	dbfx.Exec(t, "UPDATE issue SET assignee_type='member',assignee_id=$2 WHERE id=$1", base.issue, handler)
	fx := testutil.New(testPool, execution, owner)
	runtime := fx.Runtime(t, "Private offered machine", testutil.Cols{"status": "offline"})
	agent := fx.Agent(t, "Explicitly offered agent", runtime, testutil.Cols{"instructions": "NEVER_EXPOSE_OFFER_CONFIGURATION"})
	hidden := fx.Agent(t, "UNSELECTED_PRIVATE_AGENT", runtime)
	fx.Insert(t, "collaboration_runtime_enrollment", testutil.Cols{"deployment_id": auth.ManagedDeploymentID(), "owner_user_id": owner,
		"execution_workspace_id": execution, "runtime_id": runtime, "installation_id": uuid.NewString(), "public_key": make([]byte, 32)})
	base.owned = append(base.owned, runtime, agent, hidden)
	dbfx.Cleanup(t, "DELETE FROM collaboration_resource_offer WHERE issue_id=$1", base.issue)
	dbfx.Cleanup(t, "DELETE FROM collaboration_grant WHERE issue_id=$1", base.issue)
	return &resourceOfferHTTPFixture{configHTTPFixture: base, owner: owner, handler: handler, execution: execution, ownedRuntime: runtime, ownedAgent: agent, hiddenAgent: hidden}
}

func (f *resourceOfferHTTPFixture) body() map[string]any {
	body := switchHTTPBody()
	body["execution_workspace_id"], body["kind"], body["resource_id"] = f.execution, "agent", f.ownedAgent
	body["roster"] = []collaboration.TargetResource{{Role: "agent", AgentID: f.ownedAgent, RuntimeID: f.ownedRuntime, OwnerUserID: f.owner, ExecutionWorkspaceID: f.execution}}
	return body
}

func (f *resourceOfferHTTPFixture) offer(t *testing.T) service.CollaborationQueueReceipt {
	t.Helper()
	var result service.CollaborationQueueReceipt
	testutil.Call(t, testHandler.OfferCollaborationResource, collaborationContentRequest(http.MethodPost, f.issue, "", f.owner, uuid.NewString(), f.body())).Want(http.StatusCreated).JSON(&result)
	return result
}

func TestCollaborationResourceOfferHTTPPrivateOwnerAndOutsideOrganizer(t *testing.T) {
	f := newResourceOfferHTTPFixture(t)
	for _, actor := range []string{testUserID, f.handler} {
		r := resourceHTTPQuery(collaborationContentRequest(http.MethodGet, f.issue, "", actor, "", nil), url.Values{"execution_workspace_id": {f.execution}})
		testutil.Call(t, testHandler.GetCollaborationResourceOptions, r).Want(http.StatusNotFound)
	}
	var options service.CollaborationResourceOptions
	r := resourceHTTPQuery(collaborationContentRequest(http.MethodGet, f.issue, "", f.owner, "", nil), url.Values{"execution_workspace_id": {f.execution}})
	r.Header.Set("X-Workspace-ID", testWorkspaceID)
	r.Header.Set("X-Workspace-Slug", "ambient-source")
	response := testutil.Call(t, testHandler.GetCollaborationResourceOfferOptions, r).Want(http.StatusOK)
	response.JSON(&options)
	if len(options.Items) != 2 || r.Header.Get("X-Workspace-ID") != "" || r.Header.Get("X-Workspace-Slug") != "" || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("owner options failed explicit issue and execution scope")
	}
	if dbfx.Count(t, "SELECT count(*) FROM member WHERE workspace_id=$1 AND user_id=$2", testWorkspaceID, f.owner) != 0 {
		t.Fatal("test does not exercise a source-hidden provider")
	}
	offer := f.offer(t)
	var page service.CollaborationResourceOffersPage
	response = testutil.Call(t, testHandler.ListCollaborationResourceOffers, collaborationContentRequest(http.MethodGet, f.issue, "", f.handler, "", nil)).Want(http.StatusOK)
	response.JSON(&page)
	if len(page.Items) != 1 || !page.Items[0].CanAdopt || page.Items[0].CanWithdraw || page.Items[0].OfferedBy == nil || page.Items[0].OfferedBy.UserID != parseUUID(f.owner) {
		t.Fatalf("organizer did not see the explicit available offer: %+v", page)
	}
	for _, private := range []string{f.hiddenAgent, "UNSELECTED_PRIVATE_AGENT", "NEVER_EXPOSE_OFFER_CONFIGURATION", "configuration_fingerprint", "resource_authority", "grant_id", "binding_id", "installation_id"} {
		if strings.Contains(response.Body.String(), private) {
			t.Fatalf("offer list disclosed %q", private)
		}
	}
	body := switchHTTPBody()
	body["expected_offer_version"], body["expected_target_set_version"] = "1", "1"
	key := uuid.NewString()
	request := withURLParams(collaborationContentRequest(http.MethodPost, f.issue, "", f.handler, key, body), "offerId", *offer.OfferID)
	var adopted service.CollaborationQueueReceipt
	testutil.Call(t, testHandler.AdoptCollaborationResourceOffer, request).Want(http.StatusCreated).JSON(&adopted)
	if !adopted.TargetID.Valid || adopted.OfferVersion != 2 || adopted.OfferID == nil || *adopted.OfferID != *offer.OfferID || adopted.Action != "adopt_resource_offer" {
		t.Fatalf("adoption receipt lost its exact identities: %+v", adopted)
	}
	var recovered service.CollaborationQueueReceipt
	testutil.Call(t, testHandler.GetCollaborationResourceOperation, withURLParams(collaborationContentRequest(http.MethodGet, f.issue, "", f.handler, "", nil), "requestId", key)).Want(http.StatusOK).JSON(&recovered)
	if !reflect.DeepEqual(adopted, recovered) {
		t.Fatal("GET recomputed the adoption receipt")
	}
	if dbfx.Count(t, "SELECT count(*) FROM collaboration_grant WHERE issue_id=$1", f.issue) != 0 ||
		dbfx.Count(t, "SELECT count(*) FROM collaboration_target WHERE id=$1 AND hold_reason='initial' AND queue_state='paused'", adopted.TargetID) != 1 {
		t.Fatal("HTTP adoption bypassed independent owner consent or initial hold")
	}
}

func TestCollaborationResourceOfferHTTPActualInventoryPredicateProtectsSquads(t *testing.T) {
	f := newResourceOfferHTTPFixture(t)
	dbfx.Exec(t, "UPDATE member SET role='member' WHERE workspace_id=$1 AND user_id=$2", f.execution, f.owner)
	dbfx.Member(t, f.execution, f.handler, "member")
	fx := testutil.New(testPool, f.execution, f.handler)
	runtime := fx.Runtime(t, "Hidden other-owner machine")
	agent := fx.Agent(t, "Hidden other-owner agent", runtime)
	squad := fx.Squad(t, "Private mixed squad", f.ownedAgent)
	fx.SquadMember(t, squad, "agent", agent)
	f.owned = append(f.owned, runtime, agent, squad)
	body := f.body()
	body["kind"], body["resource_id"] = "squad", squad
	body["roster"] = []collaboration.TargetResource{
		{Role: "leader", AgentID: f.ownedAgent, RuntimeID: f.ownedRuntime, OwnerUserID: f.owner, ExecutionWorkspaceID: f.execution},
		{Role: "member", AgentID: agent, RuntimeID: runtime, OwnerUserID: f.handler, ExecutionWorkspaceID: f.execution},
	}
	r := resourceHTTPQuery(collaborationContentRequest(http.MethodGet, f.issue, "", f.owner, "", nil), url.Values{"execution_workspace_id": {f.execution}})
	response := testutil.Call(t, testHandler.GetCollaborationResourceOfferOptions, r).Want(http.StatusOK)
	if strings.Contains(response.Body.String(), squad) || strings.Contains(response.Body.String(), agent) || strings.Contains(response.Body.String(), runtime) {
		t.Fatal("owner options revealed a hidden member or partial squad")
	}
	testutil.Call(t, testHandler.OfferCollaborationResource, collaborationContentRequest(http.MethodPost, f.issue, "", f.owner, uuid.NewString(), body)).Want(http.StatusNotFound)
	if dbfx.Count(t, "SELECT count(*) FROM collaboration_resource_offer WHERE issue_id=$1", f.issue) != 0 {
		t.Fatal("a forged direct POST bypassed the selector visibility boundary")
	}
}

func TestCollaborationResourceOfferHTTPPaginationIsViewerBoundAndReauthorized(t *testing.T) {
	f := newResourceOfferHTTPFixture(t)
	for i := 0; i < 3; i++ {
		f.offer(t)
	}
	var first service.CollaborationResourceOffersPage
	r := resourceHTTPQuery(collaborationContentRequest(http.MethodGet, f.issue, "", f.owner, "", nil), url.Values{"limit": {"2"}})
	testutil.Call(t, testHandler.ListCollaborationResourceOffers, r).Want(http.StatusOK).JSON(&first)
	if first.NextCursor == nil || len(first.Items) != 2 {
		t.Fatal("expected bounded first page and cursor")
	}
	var next service.CollaborationResourceOffersPage
	r = resourceHTTPQuery(collaborationContentRequest(http.MethodGet, f.issue, "", f.owner, "", nil), url.Values{"limit": {"2"}, "cursor": {*first.NextCursor}})
	testutil.Call(t, testHandler.ListCollaborationResourceOffers, r).Want(http.StatusOK).JSON(&next)
	if len(next.Items) != 1 || next.NextCursor != nil || next.Items[0].OfferID == first.Items[1].OfferID {
		t.Fatal("next page repeated or lost the original offer")
	}
	r = resourceHTTPQuery(collaborationContentRequest(http.MethodGet, f.issue, "", f.handler, "", nil), url.Values{"cursor": {*first.NextCursor}})
	testutil.Call(t, testHandler.ListCollaborationResourceOffers, r).Want(http.StatusBadRequest)
	dbfx.Exec(t, "UPDATE issue_participant SET revoked_at=now(),epoch=epoch+1 WHERE issue_id=$1 AND user_id=$2", f.issue, f.owner)
	r = resourceHTTPQuery(collaborationContentRequest(http.MethodGet, f.issue, "", f.owner, "", nil), url.Values{"cursor": {*first.NextCursor}})
	testutil.Call(t, testHandler.ListCollaborationResourceOffers, r).Want(http.StatusNotFound)
	otherIssue := collaborationBoundaryIssue(t, testWorkspaceID)
	testutil.Call(t, testHandler.GetCollaborationResourceOffer, withURLParams(collaborationContentRequest(http.MethodGet, otherIssue, "", testUserID, "", nil), "offerId", uuidToString(first.Items[0].OfferID))).Want(http.StatusNotFound)
}

func TestCollaborationResourceOfferHTTPRecoveryIsExactlyRedactedAfterDeparture(t *testing.T) {
	for _, action := range []string{"offer", "withdraw", "adopt"} {
		t.Run(action, func(t *testing.T) {
			f := newResourceOfferHTTPFixture(t)
			actor, key := f.owner, uuid.NewString()
			var result service.CollaborationQueueReceipt
			if action == "offer" {
				testutil.Call(t, testHandler.OfferCollaborationResource, collaborationContentRequest(http.MethodPost, f.issue, "", actor, key, f.body())).Want(http.StatusCreated).JSON(&result)
			} else {
				offer := f.offer(t)
				body := switchHTTPBody()
				body["expected_offer_version"] = "1"
				call, status := testHandler.WithdrawCollaborationResourceOffer, http.StatusOK
				if action == "adopt" {
					actor, call, status = f.handler, testHandler.AdoptCollaborationResourceOffer, http.StatusCreated
					body["expected_target_set_version"] = "1"
				}
				testutil.Call(t, call, withURLParams(collaborationContentRequest(http.MethodPost, f.issue, "", actor, key, body), "offerId", *offer.OfferID)).Want(status).JSON(&result)
			}
			dbfx.Exec(t, "UPDATE issue_participant SET revoked_at=now(),epoch=epoch+1 WHERE issue_id=$1 AND user_id=$2", f.issue, actor)
			var wire map[string]json.RawMessage
			response := testutil.Call(t, testHandler.GetCollaborationResourceOperation, withURLParams(collaborationContentRequest(http.MethodGet, f.issue, "", actor, "", nil), "requestId", key)).Want(http.StatusOK)
			response.JSON(&wire)
			if len(wire) != 7 || string(wire["redacted"]) != "true" || string(wire["action"]) != `"`+result.Action+`"` ||
				strings.Contains(response.Body.String(), *result.OfferID) || response.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("new resource recovery leaked data or lost action: %s", response.Body.String())
			}
		})
	}
}

func TestCollaborationResourceOfferHTTPRejectsMachineBeforeDatabase(t *testing.T) {
	bare := &Handler{}
	for _, h := range []http.HandlerFunc{bare.GetCollaborationResourceOfferOptions, bare.ListCollaborationResourceOffers, bare.GetCollaborationResourceOffer,
		bare.OfferCollaborationResource, bare.WithdrawCollaborationResourceOffer, bare.AdoptCollaborationResourceOffer} {
		for _, kind := range []auth.CredentialKind{auth.CredentialTask, auth.CredentialCloud} {
			r := withURLParams(collaborationContentRequest(http.MethodPost, uuid.NewString(), "", testUserID, uuid.NewString(), switchHTTPBody()), "offerId", uuid.NewString())
			r.Header.Set("X-Workspace-ID", uuid.NewString())
			r = r.WithContext(auth.WithAuthenticatedPrincipal(r.Context(), auth.AuthenticatedPrincipal{UserID: testUserID, Kind: kind}))
			response := testutil.Call(t, h, r)
			if response.Code != http.StatusNotFound && response.Code != http.StatusForbidden {
				t.Fatalf("machine request was not denied: %d", response.Code)
			}
			if response.Header().Get("Cache-Control") != "no-store" || r.Header.Get("X-Workspace-ID") != "" {
				t.Fatal("machine denial kept ambient workspace or cacheability")
			}
		}
	}
}

func TestCollaborationResourceOfferHTTPStrictQueries(t *testing.T) {
	bare := &Handler{}
	issue, offer := uuid.NewString(), uuid.NewString()
	cursor := collaborationResourceOfferCursor{IssueID: parseUUID(issue), ViewerID: parseUUID(testUserID), BeforeID: parseUUID(offer), BeforeCreatedAt: time.Now().UTC()}
	encoded, err := json.Marshal(cursor)
	if err != nil {
		t.Fatal(err)
	}
	badCursor := func(raw string) url.Values {
		return url.Values{"cursor": {base64.RawURLEncoding.EncodeToString([]byte(raw))}}
	}
	for _, query := range []url.Values{
		{"limit": {"0"}}, {"limit": {"101"}}, {"limit": {"1", "2"}}, {"cursor": {""}}, {"cursor": {strings.Repeat("a", 513)}},
		{"cursor": {"not-base64!"}}, {"actor": {testUserID}}, badCursor(strings.Replace(string(encoded), testUserID, uuid.NewString(), 1)),
		badCursor(strings.Replace(string(encoded), issue, uuid.NewString(), 1)), badCursor(strings.TrimSuffix(string(encoded), "}") + `,"extra":true}`),
		badCursor(strings.TrimSuffix(string(encoded), "}") + `,"before_id":"` + offer + `"}`),
	} {
		r := resourceHTTPQuery(collaborationContentRequest(http.MethodGet, issue, "", testUserID, "", nil), query)
		testutil.Call(t, bare.ListCollaborationResourceOffers, r).Want(http.StatusBadRequest)
	}
	for _, query := range []url.Values{{}, {"execution_workspace_id": {"invalid"}}, {"execution_workspace_id": {uuid.NewString(), uuid.NewString()}}, {"execution_workspace_id": {uuid.NewString()}, "owner_id": {testUserID}}} {
		r := resourceHTTPQuery(collaborationContentRequest(http.MethodGet, issue, "", testUserID, "", nil), query)
		testutil.Call(t, bare.GetCollaborationResourceOfferOptions, r).Want(http.StatusBadRequest)
	}
	r := withURLParams(collaborationContentRequest(http.MethodGet, issue, "", testUserID, "", nil), "offerId", "bad-uuid")
	testutil.Call(t, bare.GetCollaborationResourceOffer, r).Want(http.StatusBadRequest)
}

func TestCollaborationResourceOfferHTTPStrictWriteShapes(t *testing.T) {
	bare := &Handler{}
	issue, offer, resource, runtime, workspace := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	valid := func() map[string]any {
		body := switchHTTPBody()
		body["execution_workspace_id"], body["kind"], body["resource_id"] = workspace, "agent", resource
		body["roster"] = []collaboration.TargetResource{{Role: "agent", AgentID: resource, RuntimeID: runtime, ExecutionWorkspaceID: workspace, OwnerUserID: testUserID}}
		return body
	}
	for _, mutate := range []func(map[string]any){
		func(p map[string]any) { p["configuration_fingerprint"] = strings.Repeat("0", 64) },
		func(p map[string]any) { p["resource_id"] = "wrong" },
		func(p map[string]any) { p["execution_workspace_id"] = "wrong" },
		func(p map[string]any) { p["expected_lifecycle_epoch"] = "0" },
		func(p map[string]any) { p["expected_account_epoch"] = 1 },
		func(p map[string]any) {
			p["roster"] = []map[string]any{{"role": "agent", "agent_id": "wrong", "runtime_id": runtime, "owner_user_id": testUserID, "execution_workspace_id": workspace}}
		},
	} {
		body := valid()
		mutate(body)
		testutil.Call(t, bare.OfferCollaborationResource, collaborationContentRequest(http.MethodPost, issue, "", testUserID, uuid.NewString(), body)).Want(http.StatusBadRequest)
	}
	for _, call := range []http.HandlerFunc{bare.WithdrawCollaborationResourceOffer, bare.AdoptCollaborationResourceOffer} {
		body := switchHTTPBody()
		body["expected_offer_version"], body["roster"] = "1", []any{}
		testutil.Call(t, call, withURLParams(collaborationContentRequest(http.MethodPost, issue, "", testUserID, uuid.NewString(), body), "offerId", offer)).Want(http.StatusBadRequest)
		delete(body, "roster")
		body["expected_offer_version"] = "0"
		testutil.Call(t, call, withURLParams(collaborationContentRequest(http.MethodPost, issue, "", testUserID, uuid.NewString(), body), "offerId", offer)).Want(http.StatusBadRequest)
	}
	r := collaborationContentRequest(http.MethodPost, issue, "", testUserID, uuid.NewString(), valid())
	r.Header.Add("Idempotency-Key", uuid.NewString())
	testutil.Call(t, bare.OfferCollaborationResource, r).Want(http.StatusBadRequest)
}

func TestCollaborationResourceOfferHTTPRequiresParticipationBeforeDisclosure(t *testing.T) {
	f := newResourceOfferHTTPFixture(t)
	offer := f.offer(t)
	viewer := dbfx.User(t, "Source member without participation", "offer-source-reader-"+uuid.NewString()+"@example.test")
	dbfx.Member(t, testWorkspaceID, viewer, "member")
	for _, actor := range []string{viewer, f.handler} {
		if actor == f.handler {
			dbfx.Exec(t, "UPDATE issue SET assignee_id=$2 WHERE id=$1", f.issue, testUserID)
		}
		r := withURLParams(collaborationContentRequest(http.MethodGet, f.issue, "", actor, "", nil), "offerId", *offer.OfferID)
		testutil.Call(t, testHandler.GetCollaborationResourceOffer, r).Want(http.StatusNotFound)
	}
	var page service.CollaborationResourceOffersPage
	testutil.Call(t, testHandler.ListCollaborationResourceOffers, collaborationContentRequest(http.MethodGet, f.issue, "", f.handler, "", nil)).Want(http.StatusOK).JSON(&page)
	if len(page.Items) != 0 {
		t.Fatal("ordinary participant retained another owner's offers")
	}
	dbfx.Exec(t, "UPDATE issue_participant SET revoked_at=now(),epoch=epoch+1 WHERE issue_id=$1 AND user_id=$2", f.issue, f.owner)
	key := uuid.NewString()
	testutil.Call(t, testHandler.OfferCollaborationResource, collaborationContentRequest(http.MethodPost, f.issue, "", f.owner, key, f.body())).Want(http.StatusNotFound)
	if dbfx.Count(t, "SELECT count(*) FROM collaboration_operation WHERE scope_id=$1 AND request_id=$2", f.issue, key) != 0 ||
		dbfx.Count(t, "SELECT count(*) FROM collaboration_resource_offer WHERE issue_id=$1", f.issue) != 1 {
		t.Fatal("departed provider wrote an offer or operation")
	}
}
