package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"

	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const (
	TaskCapabilityReporter = "project_reporter"
	TaskCapabilityReviewer = "issue_reviewer"
)

// TaskCapabilityProfile uses immutable, server-issued template provenance.
// Names, prompt text, skill attachments and mutable autonomy never grant scope.
func TaskCapabilityProfile(key string, version int32) string {
	switch key {
	case "progress-reporter":
		if version >= 2 {
			return TaskCapabilityReporter
		}
	case "code-reviewer", "security-reviewer":
		if version >= 2 {
			return TaskCapabilityReviewer
		}
	case "migration-reviewer":
		if version >= 3 {
			return TaskCapabilityReviewer
		}
	}
	return ""
}

// TaskCapability is a server-only snapshot bound to one claim generation.
// It is deliberately not an API request or response type.
type TaskCapability struct {
	Version         int    `json:"version"`
	Profile         string `json:"profile"`
	IssueID         string `json:"issue_id"`
	ProjectID       string `json:"project_id"`
	OriginatorID    string `json:"originator_id"`
	RuntimeID       string `json:"runtime_id"`
	ClaimGeneration int64  `json:"claim_generation"`
}

func ParseTaskCapability(raw []byte) (TaskCapability, error) {
	var c TaskCapability
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		return c, errors.New("invalid task capability")
	}
	if dec.Decode(new(any)) != io.EOF || c.Version != 1 || (c.Profile != TaskCapabilityReporter && c.Profile != TaskCapabilityReviewer) || c.ClaimGeneration < 1 {
		return c, errors.New("invalid task capability")
	}
	for _, s := range []string{c.IssueID, c.OriginatorID, c.RuntimeID} {
		if _, err := util.ParseUUID(s); err != nil {
			return c, errors.New("invalid task capability identity")
		}
	}
	if c.ProjectID != "" {
		if _, err := util.ParseUUID(c.ProjectID); err != nil {
			return c, errors.New("invalid task capability project")
		}
	}
	return c, nil
}

func (c TaskCapability) AllowsIssue(issue db.Issue, write bool) bool {
	if util.UUIDToString(issue.ID) == c.IssueID {
		return util.UUIDToString(issue.ProjectID) == c.ProjectID
	}
	return !write && c.Profile == TaskCapabilityReporter && c.ProjectID != "" && util.UUIDToString(issue.ProjectID) == c.ProjectID
}

type verifiedTaskTokenKey struct{}

// WithVerifiedTaskToken is only called after credential verification. Keeping the
// whole binding in a private context prevents client headers from supplying it.
func WithVerifiedTaskToken(ctx context.Context, token db.TaskToken) context.Context {
	return context.WithValue(ctx, verifiedTaskTokenKey{}, token)
}
func VerifiedTaskTokenFromContext(ctx context.Context) (db.TaskToken, bool) {
	t, ok := ctx.Value(verifiedTaskTokenKey{}).(db.TaskToken)
	return t, ok && t.ID.Valid
}

var ErrTaskCapability = errors.New("task capability does not authorize this request")

// AgentInvocationAllowed is the existing owner/private/public-to predicate.
// Callers supply current membership; it never infers membership from a token.
func AgentInvocationAllowed(agent db.Agent, targets []db.AgentInvocationTarget, effectiveUser string, isWorkspaceMember, workspaceBroad bool) bool {
	if effectiveUser != "" && util.UUIDToString(agent.OwnerID) == effectiveUser {
		return true
	}
	if agent.PermissionMode != "public_to" {
		return false
	}
	for _, target := range targets {
		switch target.TargetType {
		case "workspace":
			if isWorkspaceMember || workspaceBroad {
				return true
			}
		case "member":
			if effectiveUser != "" && util.UUIDToString(target.TargetID) == effectiveUser {
				return true
			}
		}
	}
	return false
}

// CurrentTaskCapability resolves authority from one caller-owned DB snapshot.
// No fallback to the runtime owner is allowed when originator attribution is absent.
func CurrentTaskCapability(ctx context.Context, q *db.Queries, task db.AgentTaskQueue, agent db.Agent, ownerID, workspaceID string) (TaskCapability, error) {
	c := TaskCapability{Version: 1, Profile: TaskCapabilityProfile(agent.TemplateKey, agent.TemplateVersion), IssueID: util.UUIDToString(task.IssueID), OriginatorID: util.UUIDToString(task.OriginatorUserID), RuntimeID: util.UUIDToString(task.RuntimeID), ClaimGeneration: task.ClaimGeneration}
	if c.Profile == "" || !task.IssueID.Valid || !task.OriginatorUserID.Valid || !task.RuntimeID.Valid || task.AgentID != agent.ID || agent.ArchivedAt.Valid || util.UUIDToString(agent.WorkspaceID) != workspaceID || (task.Status != "dispatched" && task.Status != "running") {
		return c, ErrTaskCapability
	}
	runtime, err := q.GetAgentRuntime(ctx, task.RuntimeID)
	if err != nil {
		return c, err
	}
	if util.UUIDToString(runtime.OwnerID) != ownerID || util.UUIDToString(runtime.WorkspaceID) != workspaceID || agent.RuntimeID != runtime.ID {
		return c, ErrTaskCapability
	}
	issue, err := q.GetIssue(ctx, task.IssueID)
	if err != nil {
		return c, err
	}
	if util.UUIDToString(issue.WorkspaceID) != workspaceID {
		return c, ErrTaskCapability
	}
	c.ProjectID = util.UUIDToString(issue.ProjectID)
	for _, id := range []string{ownerID, c.OriginatorID} {
		uid, err := util.ParseUUID(id)
		if err != nil {
			return c, ErrTaskCapability
		}
		if _, err = q.GetMemberByUserAndWorkspace(ctx, db.GetMemberByUserAndWorkspaceParams{UserID: uid, WorkspaceID: issue.WorkspaceID}); err != nil {
			return c, err
		}
		user, err := q.GetUser(ctx, uid)
		if err != nil {
			return c, err
		}
		if user.DisabledAt.Valid || IsTemporarilyDisabledUser(id, user.Email.String) {
			return c, ErrTaskCapability
		}
	}
	targets, err := q.ListAgentInvocationTargets(ctx, agent.ID)
	if err != nil {
		return c, err
	}
	if !AgentInvocationAllowed(agent, targets, c.OriginatorID, true, false) {
		return c, ErrTaskCapability
	}
	raw, err := json.Marshal(c)
	if err != nil {
		return c, err
	}
	return ParseTaskCapability(raw)
}
