package projecthealth

import (
	"context"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// Collect only uses q. The caller owns authorization and the RR transaction;
// this function must never borrow a pool connection or start a transaction.
func Collect(ctx context.Context, q *db.Queries, p db.Project, now time.Time) (Collection, error) {
	ws, err := q.GetWorkspace(ctx, p.WorkspaceID)
	if err != nil {
		return Collection{}, err
	}
	in := Input{Project: Project{WorkspaceID: uuid(p.WorkspaceID), ID: uuid(p.ID), Revision: p.Revision, Status: p.Status, DueDate: date(p.DueDate), LeadType: p.LeadType.String, LeadID: uuid(p.LeadID), InProgressSince: timestamp(p.InProgressSince), InProgressSinceSource: p.InProgressSinceSource.String}, Now: now, Timezone: text(ws.PlanningTimezone)}
	statuses, err := q.ListIssueStatusEntries(ctx, db.ListIssueStatusEntriesParams{WorkspaceID: p.WorkspaceID, IncludeArchived: true})
	if err != nil {
		return unavailable(in, "status_catalog_unavailable", err)
	}
	rows, err := q.ListProjectHealthIssues(ctx, db.ListProjectHealthIssuesParams{WorkspaceID: p.WorkspaceID, ProjectID: p.ID})
	if err != nil {
		return unavailable(in, "issues_unavailable", err)
	}
	members, err := q.ListMembers(ctx, p.WorkspaceID)
	if err != nil {
		return unavailable(in, "members_unavailable", err)
	}
	agents, err := q.ListAllAgentsAnyKind(ctx, p.WorkspaceID)
	if err != nil {
		return unavailable(in, "agents_unavailable", err)
	}
	squads, err := q.ListAllSquads(ctx, p.WorkspaceID)
	if err != nil {
		return unavailable(in, "squads_unavailable", err)
	}
	roster, err := q.ListSquadMemberPreviewRows(ctx, p.WorkspaceID)
	if err != nil {
		return unavailable(in, "squad_members_unavailable", err)
	}
	runtimes, err := q.ListAgentRuntimes(ctx, p.WorkspaceID)
	if err != nil {
		return unavailable(in, "runtimes_unavailable", err)
	}
	latest, err := q.GetProjectHealthLatestUpdate(ctx, db.GetProjectHealthLatestUpdateParams{WorkspaceID: p.WorkspaceID, ProjectID: p.ID})
	if err != nil {
		return unavailable(in, "updates_unavailable", err)
	}
	in.LatestUpdateAt = timestamp(latest)
	for _, s := range statuses {
		in.Statuses = append(in.Statuses, Status{Key: s.Key, Category: s.Category})
	}
	for _, i := range rows {
		in.Issues = append(in.Issues, Issue{ID: uuid(i.ID), Status: i.Status, DueDate: date(i.DueDate), AssigneeType: i.AssigneeType.String, AssigneeID: uuid(i.AssigneeID)})
	}
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
	for _, r := range wanted {
		in.References = append(in.References, r)
	}
	out, err := Compute(in)
	if err != nil {
		return Collection{}, err
	}
	out.StatusNames = map[string]string{}
	for _, status := range statuses {
		if !status.IsSystem {
			out.StatusNames[status.Key] = status.Name
		}
	}
	return out, nil
}

func uuid(v pgtype.UUID) string {
	if !v.Valid {
		return ""
	}
	return v.String()
}
func date(v pgtype.Date) *string {
	if !v.Valid {
		return nil
	}
	return ptr(v.Time.Format(time.DateOnly))
}
func timestamp(v pgtype.Timestamptz) *time.Time {
	if !v.Valid {
		return nil
	}
	return ptr(v.Time.UTC())
}
func text(v pgtype.Text) *string {
	if !v.Valid {
		return nil
	}
	return ptr(v.String)
}
