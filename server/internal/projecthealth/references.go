package projecthealth

import (
	"context"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"sort"
)

type referenceInputs struct {
	members  []db.Member
	agents   []db.Agent
	squads   []db.Squad
	roster   []db.ListSquadMemberPreviewRowsRow
	runtimes []db.AgentRuntime
}

// CollectReferences reads the same directory facts as Collect, using only q.
// It returns only the associated references requested by the provided input.
func CollectReferences(ctx context.Context, q *db.Queries, p db.Project, in Input) ([]Reference, error) {
	sources, err := collectReferenceInputs(ctx, q, p, in)
	if err != nil {
		return nil, err
	}
	return resolveReferences(in, sources), nil
}

func collectReferenceInputs(ctx context.Context, q *db.Queries, p db.Project, in Input) (referenceInputs, error) {
	members, err := q.ListMembers(ctx, p.WorkspaceID)
	if err != nil {
		return referenceCollectionFailure(in, "members_unavailable", err)
	}
	agents, err := q.ListAllAgentsAnyKind(ctx, p.WorkspaceID)
	if err != nil {
		return referenceCollectionFailure(in, "agents_unavailable", err)
	}
	squads, err := q.ListAllSquads(ctx, p.WorkspaceID)
	if err != nil {
		return referenceCollectionFailure(in, "squads_unavailable", err)
	}
	roster, err := q.ListSquadMemberPreviewRows(ctx, p.WorkspaceID)
	if err != nil {
		return referenceCollectionFailure(in, "squad_members_unavailable", err)
	}
	runtimes, err := q.ListAgentRuntimes(ctx, p.WorkspaceID)
	if err != nil {
		return referenceCollectionFailure(in, "runtimes_unavailable", err)
	}
	return referenceInputs{members: members, agents: agents, squads: squads, roster: roster, runtimes: runtimes}, nil
}

func referenceCollectionFailure(in Input, reason string, cause error) (referenceInputs, error) {
	_, err := unavailable(in, reason, cause)
	return referenceInputs{}, err
}

func resolveReferences(in Input, sources referenceInputs) []Reference {
	members, agents, squads, roster, runtimes := sources.members, sources.agents, sources.squads, sources.roster, sources.runtimes
	memberMap := map[string]bool{}
	for _, m := range members {
		memberMap[uuid(m.UserID)] = true
	}
	agentMap := map[string]db.Agent{}
	for _, a := range agents {
		agentMap[uuid(a.ID)] = a
	}
	runtimeMap := map[string]db.AgentRuntime{}
	for _, r := range runtimes {
		runtimeMap[uuid(r.ID)] = r
	}
	squadMap := map[string]db.Squad{}
	for _, s := range squads {
		squadMap[uuid(s.ID)] = s
	}
	squadAgents := map[string][]string{}
	for _, r := range roster {
		if r.MemberType == "agent" {
			squadAgents[uuid(r.SquadID)] = append(squadAgents[uuid(r.SquadID)], uuid(r.MemberID))
		}
	}
	agentReference := func(id string) Reference {
		a, exists := agentMap[id]
		runtime, runtimeExists := runtimeMap[uuid(a.RuntimeID)]
		valid := exists && !a.ArchivedAt.Valid
		ready := runtimeExists && runtime.Status == "online"
		facts := map[string]any{"exists": exists, "archived": a.ArchivedAt.Valid, "runtime_id": uuid(a.RuntimeID), "runtime_exists": runtimeExists, "runtime_status": runtime.Status}
		return Reference{Kind: "agent", ID: id, Valid: valid, EnvironmentUnavailable: !ready, Facts: facts}
	}
	wanted := map[string]Reference{}
	add := func(kind, id string) {
		if kind == "" || id == "" {
			return
		}
		key := kind + ":" + id
		if _, ok := wanted[key]; ok {
			return
		}
		r := Reference{Kind: kind, ID: id}
		switch kind {
		case "member":
			r.Valid = memberMap[id]
			r.Facts = map[string]any{"member_exists": r.Valid}
		case "agent":
			r = agentReference(id)
		case "squad":
			s, exists := squadMap[id]
			leader := agentReference(uuid(s.LeaderID))
			r.Valid = exists && !s.ArchivedAt.Valid && leader.Valid
			r.EnvironmentUnavailable = !leader.Valid || leader.EnvironmentUnavailable
			ids := append([]string(nil), squadAgents[id]...)
			sort.Strings(ids)
			facts := make([]Reference, 0, len(ids))
			for _, agentID := range ids {
				member := agentReference(agentID)
				facts = append(facts, member)
				if !member.Valid || member.EnvironmentUnavailable {
					r.EnvironmentUnavailable = true
				}
			}
			r.Facts = map[string]any{"exists": exists, "archived": s.ArchivedAt.Valid, "leader": leader, "agents": facts}
		}
		wanted[key] = r
	}
	add(in.Project.LeadType, in.Project.LeadID)
	for _, i := range in.Issues {
		add(i.AssigneeType, i.AssigneeID)
	}
	references := []Reference{}
	for _, r := range wanted {
		references = append(references, r)
	}
	return references
}
