package handler

import (
	"encoding/json"
	"net/http"
	"slices"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestSquadMembershipSummaryIncludesAgentsBeyondPreview(t *testing.T) {
	summary := &squadMemberSummary{}
	want := []string{
		"00000000-0000-0000-0000-000000000001",
		"00000000-0000-0000-0000-000000000002",
		"00000000-0000-0000-0000-000000000003",
		"00000000-0000-0000-0000-000000000004",
	}
	addSquadMemberPreview(summary, "member", parseUUID("00000000-0000-0000-0000-000000000005"), "person")
	for _, id := range want {
		addSquadMemberPreview(summary, "agent", parseUUID(id), "specialist")
	}
	resp := SquadResponse{}
	applySquadMemberSummary(&resp, summary)
	if resp.MemberCount != 5 || len(resp.MemberPreview) != 3 {
		t.Fatalf("summary = %d members, %d previews; want 5 and 3", resp.MemberCount, len(resp.MemberPreview))
	}
	encoded, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}
	var actual struct {
		AgentMemberIDs []string `json:"agent_member_ids"`
	}
	if err := json.Unmarshal(encoded, &actual); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(actual.AgentMemberIDs, want) {
		t.Fatalf("complete agent membership = %v, want %v", actual.AgentMemberIDs, want)
	}
}

func TestListSquadsCompleteAgentMembership(t *testing.T) {
	if testHandler == nil || dbfx == nil {
		t.Skip("database not available")
	}
	leader := dbfx.Agent(t, "discovery-leader", "")
	shared := dbfx.Agent(t, "discovery-shared", "")
	third := dbfx.Agent(t, "discovery-third", "")
	fourth := dbfx.Agent(t, "discovery-fourth", "")
	fifth := dbfx.Agent(t, "discovery-fifth", "")
	makeSquad := func(name, workspace string, archived bool) string {
		cols := testutil.Cols{"workspace_id": workspace, "name": name, "leader_id": leader, "creator_id": testUserID}
		if archived {
			cols["archived_at"] = testutil.Raw("now()")
		}
		return dbfx.Insert(t, "squad", cols)
	}
	one := makeSquad("Discovery one", testWorkspaceID, false)
	two := makeSquad("Discovery two", testWorkspaceID, false)
	empty := makeSquad("Discovery empty", testWorkspaceID, false)
	archived := makeSquad("Discovery archived", testWorkspaceID, true)
	otherWorkspace := dbfx.Workspace(t, "Discovery outside", "discovery-outside")
	outside := makeSquad("Discovery outside", otherWorkspace, false)
	for _, id := range []string{one, two, archived, outside} {
		for _, agentID := range []string{leader, shared} {
			dbfx.Insert(t, "squad_member", testutil.Cols{"squad_id": id, "member_type": "agent", "member_id": agentID, "role": ""})
		}
	}
	for _, agentID := range []string{third, fourth, fifth} {
		dbfx.Insert(t, "squad_member", testutil.Cols{"squad_id": one, "member_type": "agent", "member_id": agentID, "role": ""})
	}
	dbfx.Insert(t, "squad_member", testutil.Cols{"squad_id": one, "member_type": "member", "member_id": testUserID, "role": ""})
	var got []struct {
		ID             string                       `json:"id"`
		MemberCount    int                          `json:"member_count"`
		MemberPreview  []SquadMemberPreviewResponse `json:"member_preview"`
		AgentMemberIDs []string                     `json:"agent_member_ids"`
	}
	testutil.Call(t, testHandler.ListSquads, squadScopeReq("", http.MethodGet, "/api/squads", nil, nil)).Want(http.StatusOK).JSON(&got)
	found := map[string]bool{}
	for _, row := range got {
		found[row.ID] = true
		switch row.ID {
		case one:
			if row.MemberCount != 6 || len(row.MemberPreview) != 3 || len(row.AgentMemberIDs) != 5 {
				t.Fatalf("full squad count/preview/membership = %d/%d/%d, want 6/3/5", row.MemberCount, len(row.MemberPreview), len(row.AgentMemberIDs))
			}
			for _, id := range []string{leader, shared, third, fourth, fifth} {
				if !slices.Contains(row.AgentMemberIDs, id) {
					t.Errorf("squad one omitted agent %s", id)
				}
			}
		case two:
			if !slices.Contains(row.AgentMemberIDs, shared) {
				t.Error("shared specialist missing from second squad")
			}
		case empty:
			if row.AgentMemberIDs == nil || len(row.AgentMemberIDs) != 0 {
				t.Errorf("empty membership must encode as [], got %v", row.AgentMemberIDs)
			}
		}
	}
	if !found[one] || !found[two] || !found[empty] {
		t.Error("missing active workspace squad")
	}
	if found[archived] || found[outside] {
		t.Error("list exposed archived or foreign-workspace squad")
	}
}
