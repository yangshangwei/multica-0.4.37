package collaboration

// KnowledgeTargetAuthority contains current, transaction-verified facts. A
// source reference permits proposing only; it does not grant catalog access.
type KnowledgeTargetAuthority struct {
	Kind                 string
	Exists               bool
	SourceWorkspace      bool
	CurrentSourceProject bool
	SourceProjectVisible bool
	NormallyVisible      bool
	Member               bool
	MemberRole           string
	LeadType             string
	LeadIsActor          bool
}

type KnowledgeCapabilities struct {
	Propose  bool `json:"can_propose"`
	Read     bool `json:"can_read"`
	Approve  bool `json:"can_approve"`
	Reject   bool `json:"can_reject"`
	Withdraw bool `json:"can_withdraw"`
}

// KnowledgeCapabilitiesFor is independent of the issue's open/closed execution
// state. target-only withdrawal never confers a source read or approval right.
func KnowledgeCapabilitiesFor(p Principal, source AccessState, target KnowledgeTargetAuthority, state string, sourcesAvailable, expired bool) KnowledgeCapabilities {
	var c KnowledgeCapabilities
	if p.UserID == "" || !p.Human() || !source.AccountActive {
		return c
	}
	read := CapabilitiesFor(p, source).Read
	manager := read && (source.Creator || source.Handler)
	visible := target.NormallyVisible || target.Kind == "workspace" && target.SourceWorkspace || target.Kind == "project" && target.CurrentSourceProject && target.SourceProjectVisible
	validTarget := target.Exists && (target.Kind == "workspace" || target.Kind == "project")
	confirmer := validTarget && target.Member && (target.MemberRole == "owner" || target.MemberRole == "admin" || target.Kind == "project" && target.LeadType == "member" && target.LeadIsActor)
	c.Propose = manager && validTarget && visible
	c.Read = read && sourcesAvailable
	c.Approve = state == "pending" && read && sourcesAvailable && !expired && confirmer
	// Rejection only reduces pending state. It requires the same human source
	// Read and target role as approval, but no usable/unexpired material and
	// never restores access to unavailable body text.
	c.Reject = state == "pending" && read && confirmer
	c.Withdraw = (state == "pending" || state == "approved") && (manager || confirmer)
	return c
}
