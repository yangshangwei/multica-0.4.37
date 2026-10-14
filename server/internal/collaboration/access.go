// Package collaboration holds object-level collaboration contracts shared by
// account routes and workspace adapters. It does not replace execution admission.
package collaboration

import (
	"context"

	"github.com/multica-ai/multica/server/internal/auth"
)

type Principal struct {
	UserID     string
	Kind       auth.CredentialKind
	Restricted bool
	AgentActor bool
}

func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	p, ok := auth.AuthenticatedPrincipalFromContext(ctx)
	return Principal{UserID: p.UserID, Kind: p.Kind, Restricted: p.Restricted, AgentActor: p.AgentActor}, ok
}

func (p Principal) Human() bool {
	return !p.Restricted && !p.AgentActor && (p.Kind == auth.CredentialJWT || p.Kind == auth.CredentialPAT)
}

// AccessState must be loaded from the database for the current request. Creator
// and handler identity alone do not revive a revoked participant relationship.
type AccessState struct {
	AccountActive bool
	SourceMember  bool
	Participant   bool
	Creator       bool
	Handler       bool
	// InformationRecipient is true only when the viewer is the recipient of the
	// open top of the information request chain.
	InformationRecipient bool
	StatusCategory       string
	QueueState           string
}

type Capabilities struct {
	Reference           bool `json:"reference"`
	Read                bool `json:"read"`
	Comment             bool `json:"comment"`
	ControlAI           bool `json:"control_ai"`
	SubmitCommand       bool `json:"submit_command"`
	Invite              bool `json:"invite"`
	Transfer            bool `json:"transfer"`
	RemoveParticipants  bool `json:"remove_participants"`
	Leave               bool `json:"leave"`
	Edit                bool `json:"edit"`
	EditSourceStructure bool `json:"edit_source_structure"`
	Reorder             bool `json:"reorder"`
	ChangeStatus        bool `json:"change_status"`
	Complete            bool `json:"complete"`
	Cancel              bool `json:"cancel"`
	Reopen              bool `json:"reopen"`
	ResolveThread       bool `json:"resolve_thread"`
	React               bool `json:"react"`
	RequestInformation  bool `json:"request_information"`
	ReturnInformation   bool `json:"return_information"`
	SubmitAcceptance    bool `json:"submit_acceptance"`
}

func NonterminalCategory(category string) bool {
	switch category {
	case "backlog", "todo", "in_progress", "in_review", "blocked":
		return true
	default:
		return false
	}
}

func TerminalCategory(category string) bool {
	return category == "done" || category == "cancelled"
}

func CapabilitiesFor(p Principal, s AccessState) Capabilities {
	var c Capabilities
	if p.UserID == "" || p.Restricted || !s.AccountActive || (!s.SourceMember && !s.Participant) {
		return c
	}
	c.Reference = true
	if !p.Human() {
		return c
	}
	c.Read, c.Comment, c.ResolveThread, c.React = true, true, true, true
	c.Reorder = s.SourceMember
	if !s.Participant {
		return c
	}
	c.ControlAI = true // Stop remains allowed while a closed issue has an unsettled run.
	open := NonterminalCategory(s.StatusCategory) && (s.QueueState == "paused" || s.QueueState == "active")
	c.SubmitCommand = open
	manager := s.Creator || s.Handler
	c.Invite, c.Transfer, c.Edit = manager, manager, manager
	c.EditSourceStructure = manager && s.SourceMember
	c.RemoveParticipants = s.Creator
	c.Leave = !s.Creator && !s.Handler
	c.ChangeStatus = manager && open
	c.RequestInformation = manager && open
	// A directed return belongs to whoever holds the current responsibility for
	// that request; acceptance hands a noncreator handler's result to the creator.
	c.ReturnInformation = s.Handler && s.InformationRecipient && open
	c.SubmitAcceptance = s.Handler && !s.Creator && open
	c.Complete, c.Cancel = s.Creator && open, s.Creator && open
	c.Reopen = s.Creator && TerminalCategory(s.StatusCategory) && s.QueueState == "closed"
	return c
}
