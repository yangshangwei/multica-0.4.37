package collaboration

const ExecutionProtocolVersion = 2

type TargetKind string

const (
	TargetAgent TargetKind = "agent"
	TargetSquad TargetKind = "squad"
)

type ControlScope string

const (
	ControlTarget ControlScope = "target"
	ControlIssue  ControlScope = "issue"
)

// TargetResource is an intended resource binding, not consent or runtime
// admission. Each entry needs its actual owner's current grant for this target
// configuration. A squad's member entries never create direct-agent targets.
type TargetResource struct {
	Role                 string `json:"role"`
	AgentID              string `json:"agent_id"`
	RuntimeID            string `json:"runtime_id"`
	ExecutionWorkspaceID string `json:"execution_workspace_id"`
	OwnerUserID          string `json:"owner_user_id"`
}

// GroupResource freezes the grant used by one member of a root execution group.
// Runtime admission must still check every identity and epoch before any permit.
type GroupResource struct {
	TargetResource
	GrantID    string `json:"grant_id"`
	GrantEpoch int64  `json:"grant_epoch,string"`
}
