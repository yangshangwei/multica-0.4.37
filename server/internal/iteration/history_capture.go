package iteration

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/issuestatus"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// CaptureHistoricalIssues freezes identifiers and display summaries from the
// caller's resolved issue rows. Writers hold catalog/I1/issue locks first and
// request reference locks; readers supply a coherent database snapshot instead.
func CaptureHistoricalIssues(ctx context.Context, tx pgx.Tx, ws pgtype.UUID, issues []db.Issue, lockRefs bool) ([]HistoricalIssue, error) {
	references, err := CaptureIssueReferences(ctx, tx, ws, issues, lockRefs)
	if err != nil {
		return nil, err
	}
	result := make([]HistoricalIssue, len(references))
	for i, reference := range references {
		result[i] = reference.Issue
	}
	return result, nil
}

// IssueReferenceFacts keeps current usability separate from historical names.
// Archived entities keep their display identity but invalidate a live preview.
type IssueReferenceFacts struct {
	Issue             HistoricalIssue
	ProjectAvailable  bool
	AssigneeAvailable bool
}

func CaptureIssueReferences(ctx context.Context, tx pgx.Tx, ws pgtype.UUID, issues []db.Issue, lockRefs bool) ([]IssueReferenceFacts, error) {
	result := make([]IssueReferenceFacts, 0, len(issues))
	if len(issues) == 0 {
		return result, nil
	}
	ids := make([]pgtype.UUID, 0, len(issues))
	seen := map[pgtype.UUID]bool{}
	for _, issue := range issues {
		if issue.WorkspaceID != ws || !issue.ID.Valid || seen[issue.ID] {
			return nil, errors.New("historical capture requires unique same-workspace issues")
		}
		seen[issue.ID] = true
		ids = append(ids, issue.ID)
	}
	q := db.New(tx)
	if lockRefs {
		if err := lockHistoricalReferences(ctx, q, ws, issues); err != nil {
			return nil, err
		}
	}
	references, err := q.ListIterationHistoryIssueReferences(ctx, db.ListIterationHistoryIssueReferencesParams{WorkspaceID: ws, IssueIds: ids})
	if err != nil {
		return nil, err
	}
	byID := make(map[pgtype.UUID]db.ListIterationHistoryIssueReferencesRow, len(references))
	for _, ref := range references {
		byID[ref.IssueID] = ref
	}
	for _, issue := range issues {
		ref, ok := byID[issue.ID]
		if !ok {
			return nil, errors.New("historical issue disappeared during capture")
		}
		category := issue.Status
		if !issuestatus.IsBuiltIn(category) {
			category = ref.StatusCategory.String
		}
		if !issuestatus.IsCategory(category) {
			return nil, fmt.Errorf("unknown historical issue status %q", issue.Status)
		}
		assigneeName := pgtype.Text{}
		assigneeAvailable := !issue.AssigneeID.Valid
		switch issue.AssigneeType.String {
		case "member":
			assigneeName = ref.MemberName
			assigneeAvailable = ref.MemberName.Valid
		case "agent":
			assigneeName = ref.AgentName
			assigneeAvailable = ref.AgentName.Valid && !ref.AgentArchivedAt.Valid
		case "squad":
			assigneeName = ref.SquadName
			assigneeAvailable = ref.SquadName.Valid && !ref.SquadArchivedAt.Valid && ref.SquadLeaderID.Valid && !ref.SquadLeaderArchivedAt.Valid
		}
		item := HistoricalIssue{IssueID: historyUUID(issue.ID), Identifier: util.ResolveIssuePrefix(ref.IssuePrefix, ref.WorkspaceName) + "-" + strconv.Itoa(int(issue.Number)), Title: issue.Title, ProjectID: historyNullableUUID(issue.ProjectID), ProjectName: historyText(ref.ProjectName), AssigneeType: historyText(issue.AssigneeType), AssigneeID: historyNullableUUID(issue.AssigneeID), AssigneeName: historyText(assigneeName), StatusKey: issue.Status, StatusCategory: category, WasCompletedAtStart: category == "done", RolloverCount: int(issue.IterationRolloverCount)}
		if err = validateHistoricalIssue(item); err != nil {
			return nil, err
		}
		result = append(result, IssueReferenceFacts{Issue: item, ProjectAvailable: !issue.ProjectID.Valid || ref.ProjectName.Valid, AssigneeAvailable: assigneeAvailable})
	}
	return result, nil
}

func lockHistoricalReferences(ctx context.Context, q *db.Queries, ws pgtype.UUID, issues []db.Issue) error {
	if _, err := q.LockIterationHistoryWorkspace(ctx, ws); err != nil {
		return err
	}
	projects := map[pgtype.UUID]bool{}
	agents := map[pgtype.UUID]bool{}
	squads := map[pgtype.UUID]bool{}
	members := map[pgtype.UUID]bool{}
	for _, issue := range issues {
		if issue.ProjectID.Valid {
			projects[issue.ProjectID] = true
		}
		if !issue.AssigneeID.Valid {
			continue
		}
		switch issue.AssigneeType.String {
		case "member":
			members[issue.AssigneeID] = true
		case "agent":
			agents[issue.AssigneeID] = true
		case "squad":
			squads[issue.AssigneeID] = true
		default:
			return errors.New("unknown historical assignee")
		}
	}
	for _, id := range sortedHistoryIDs(projects) {
		if _, err := q.LockProjectForAssociationNowait(ctx, db.LockProjectForAssociationNowaitParams{ID: id, WorkspaceID: ws}); err != nil {
			return err
		}
	}
	if len(members) > 0 {
		if _, err := q.LockIterationHistoryMembers(ctx, db.LockIterationHistoryMembersParams{WorkspaceID: ws, UserIds: sortedHistoryIDs(members)}); err != nil {
			return err
		}
	}
	leaders := map[pgtype.UUID]bool{}
	for _, id := range sortedHistoryIDs(squads) {
		squad, err := q.LockLifecycleSquad(ctx, db.LockLifecycleSquadParams{ID: id, WorkspaceID: ws})
		if err != nil {
			return err
		}
		if squad.LeaderID.Valid && !agents[squad.LeaderID] {
			leaders[squad.LeaderID] = true
		}
	}
	for id := range leaders {
		agents[id] = true
	}
	for _, id := range sortedHistoryIDs(agents) {
		if _, err := q.LockLifecycleAgent(ctx, db.LockLifecycleAgentParams{ID: id, WorkspaceID: ws}); err != nil {
			if leaders[id] && errors.Is(err, pgx.ErrNoRows) {
				continue
			}
			return err
		}
	}
	return nil
}
func sortedHistoryIDs(set map[pgtype.UUID]bool) []pgtype.UUID {
	ids := make([]pgtype.UUID, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return historyUUID(ids[i]) < historyUUID(ids[j]) })
	return ids
}
