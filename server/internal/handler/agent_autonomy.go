package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/multica-ai/multica/server/internal/logger"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/util"
)

// Autonomy enforcement for agent actors.
//
// An agent acts on the workspace through the same HTTP API a person does, holding
// a task-scoped token (middleware/auth.go's mat_ branch), so the API boundary is
// where a role's declared limits can actually be applied rather than merely
// described. The matching prose the agent is shown lives in
// service.AutonomyBriefing — the two must agree, and a test pins that they do.
//
// Two rules make this safe to add to established endpoints:
//
//   - It only ever applies to agent actors. A human request is untouched.
//   - An agent with no declared level passes everything (service.AutonomyAtLeast).
//     Every agent that existed before this feature is in that state, so no
//     workspace's behaviour changes until it opts into a role.
//
// What this does NOT do: sandbox the agent's shell. A task runs with the daemon
// user's permissions, so an agent that ignores its policy can still touch the
// filesystem. That gap is why high-risk operations go through a recorded human
// approval (agent_approval.go) instead of a permission bit.

// agentActorAutonomy returns the acting agent's declared autonomy level and
// whether this request came from an agent at all.
//
// A lookup failure reports "not an agent actor" and logs: the level is unknown, so
// there is nothing to enforce, and failing closed on a transient database error
// would stall every agent in the workspace — including the overwhelming majority
// that have no declared level and therefore no policy to violate. The request's
// own queries will fail on the same outage a moment later.
func (h *Handler) agentActorAutonomy(r *http.Request, workspaceID string) (string, bool) {
	actorType, actorID := h.resolveActor(r, requestUserID(r), workspaceID)
	if actorType != "agent" || actorID == "" {
		return "", false
	}
	agentUUID, err := util.ParseUUID(actorID)
	if err != nil {
		slog.Warn("autonomy: agent actor id is not a uuid", append(logger.RequestAttrs(r), "agent_id", actorID)...)
		return "", false
	}
	agent, err := h.Queries.GetAgent(r.Context(), agentUUID)
	if err != nil {
		slog.Warn("autonomy: failed to load acting agent; skipping policy check",
			append(logger.RequestAttrs(r), "error", err, "agent_id", actorID)...)
		return "", false
	}
	return agent.AutonomyLevel, true
}

// requireAgentAutonomy allows the request unless an agent actor's declared level
// is below want, in which case it writes 403 and returns false.
//
// `action` names the thing being attempted, in the agent's own vocabulary, and is
// the only part of the message worth reading — an agent that is told "observer
// agents cannot change issue status" can hand the work back with a reason, while
// one told "forbidden" retries.
func (h *Handler) requireAgentAutonomy(w http.ResponseWriter, r *http.Request, workspaceID string, want service.AutonomyLevel, action string) bool {
	level, isAgent := h.agentActorAutonomy(r, workspaceID)
	if !isAgent {
		return true
	}
	if service.AutonomyAtLeast(level, want) {
		return true
	}
	slog.Info("autonomy: denied agent action",
		append(logger.RequestAttrs(r), "autonomy_level", level, "required", string(want), "action", action)...)
	writeError(w, http.StatusForbidden, autonomyDenialMessage(level, want, action))
	return false
}

// autonomyDenialMessage explains the denial in terms of what the agent should do
// instead. It is read by a model, so it says the level, the requirement, and the
// sanctioned alternative in one line.
func autonomyDenialMessage(level string, want service.AutonomyLevel, action string) string {
	return "your autonomy level is " + level + ", which cannot " + action +
		" — that requires " + string(want) +
		". Report what is needed and hand it to a human or an agent at that level instead."
}

// requireAgentMayGrantAutonomy allows the request unless an agent actor is trying
// to create an agent at a level above its own.
//
// UpdateAgent already refuses to let a machine credential move autonomy_level
// (agent.go), because an agent's task token carries its owner's user id and would
// otherwise pass canManageAgent. A route that hands out a level on creation is the
// same escalation by another door: an Observer that cannot promote itself can
// instead mint an Operator and route the work through that. This closes it.
//
// An actor with no declared level has no ceiling, which matches
// service.AutonomyAtLeast: it is already unrestricted, so nothing it creates can
// widen what it is able to do.
func (h *Handler) requireAgentMayGrantAutonomy(w http.ResponseWriter, r *http.Request, workspaceID string, granted service.AutonomyLevel) bool {
	level, isAgent := h.agentActorAutonomy(r, workspaceID)
	if !isAgent || !service.IsKnownAutonomyLevel(level) {
		return true
	}
	if service.AutonomyAtLeast(level, granted) {
		return true
	}
	slog.Info("autonomy: denied agent granting a level above its own",
		append(logger.RequestAttrs(r), "autonomy_level", level, "granted", string(granted))...)
	writeError(w, http.StatusForbidden,
		"your autonomy level is "+level+", so you cannot create an agent at "+string(granted)+
			". Only a person can create an agent above your own level.")
	return false
}

// issueDirectionFields are the issue fields whose presence turns a write from an
// edit into a decision about what the workspace does next. They are the fields the
// Observer level is defined against, and every endpoint that can write them has to
// consult the same list — the first version of this feature gated
// PUT /api/issues/{id} and left POST /api/issues/batch-update open, which let an
// Observer do the forbidden thing by sending a batch of one.
var issueDirectionFields = []string{"status", "assignee_type", "assignee_id"}

// requestTouchesIssueDirection reports whether a request body explicitly carried
// any direction-setting field.
//
// Keyed on PRESENCE in the raw field map, never on a non-nil pointer: the write
// paths treat `{"assignee_id": null}` as "unassign" and read that from this same
// map, so a pointer-based gate misses it entirely and the request goes through.
//
// Matched case-INSENSITIVELY, because encoding/json is: `{"Status":"done"}`
// populates a field tagged `json:"status"`, so the write happens, while an exact
// map lookup for "status" finds nothing and the gate never fires. That divergence
// let an Observer set status and was reachable on both write routes. Callers that
// can also see the decoded struct should OR this with a pointer check — see
// requestSetsIssueDirection.
func requestTouchesIssueDirection(rawFields map[string]json.RawMessage) bool {
	for key := range rawFields {
		lowered := strings.ToLower(key)
		for _, field := range issueDirectionFields {
			if lowered == field {
				return true
			}
		}
	}
	return false
}

// requestSetsIssueDirection is the gate the issue write paths use: the folded
// presence check OR the decoded pointers.
//
// Both halves are load-bearing. Presence catches an explicit null (`assignee_id:
// null` means unassign, and decodes to a nil pointer). Pointers catch anything a
// future key-name transform would hide from the raw map. Either one alone has a
// hole; together they do not.
func requestSetsIssueDirection(rawFields map[string]json.RawMessage, status, assigneeType, assigneeID *string) bool {
	return requestTouchesIssueDirection(rawFields) ||
		status != nil || assigneeType != nil || assigneeID != nil
}
