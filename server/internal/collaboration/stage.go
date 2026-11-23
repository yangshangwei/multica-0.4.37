package collaboration

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"unicode/utf8"
)

// StageContract names the fixed business-stage contract. It is not a general
// workflow engine: stages, roles and transitions are closed sets.
const StageContract = "collaboration-stage-v1"

const (
	StageDevelopment  = "development"
	StageCodeReview   = "code_review"
	StageQA           = "qa"
	StageRelease      = "release"
	StageVerification = "verification"
)

const (
	StageProposalMaxChildren         = 20
	StageProposalTitleMaxRunes       = 500
	StageProposalDescriptionMaxBytes = 64 << 10
	StageProposalSummaryMaxBytes     = 16 << 10
	StageProposalMaxBytes            = 512 << 10
	StageReasonMaxRunes              = 2000
	StageScopeMaxChildren            = 200
)

// StageRoles are account identifiers chosen by an authorized configuration.
// An occupational title or display name never grants one of these roles.
type StageRoles struct {
	BusinessOwner   string
	Implementer     string
	ProductApprover string
	CodeReviewer    string
	QAApprover      string
	ReleaseApprover string
}

// Holders returns the distinct configured accounts in a stable order.
func (r StageRoles) Holders() []string {
	seen := map[string]bool{}
	var out []string
	for _, id := range []string{r.BusinessOwner, r.Implementer, r.ProductApprover, r.CodeReviewer, r.QAApprover, r.ReleaseApprover} {
		if id != "" && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}

// ValidateStageRoles returns a refusal code or "". One account may hold several
// roles, except that the implementer cannot review their own code and QA and
// release approval belong to different accounts.
func ValidateStageRoles(r StageRoles) string {
	for _, id := range []string{r.BusinessOwner, r.Implementer, r.ProductApprover, r.CodeReviewer, r.QAApprover, r.ReleaseApprover} {
		if id == "" {
			return "invalid_stage_request"
		}
	}
	if r.Implementer == r.CodeReviewer {
		return "stage_self_review_forbidden"
	}
	if r.QAApprover == r.ReleaseApprover {
		return "stage_qa_release_conflict"
	}
	return ""
}

// StageProposalChild is one ordinary issue a proposal would create. Its array
// position is the fixed ordinal.
type StageProposalChild struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Priority    string `json:"priority"`
}

type StageProposalContent struct {
	Summary  string               `json:"summary"`
	Children []StageProposalChild `json:"children"`
}

var errStageProposal = errors.New("invalid stage proposal")

func stageText(value string, maxBytes int) bool {
	return utf8.ValidString(value) && !strings.ContainsRune(value, 0) && len(value) <= maxBytes
}

// NormalizeStageProposal trims titles and enforces the ordinary issue bounds.
// It never repairs content: an out-of-range proposal is refused as a whole.
func NormalizeStageProposal(c StageProposalContent) (StageProposalContent, error) {
	if !stageText(c.Summary, StageProposalSummaryMaxBytes) || len(c.Children) < 1 || len(c.Children) > StageProposalMaxChildren {
		return StageProposalContent{}, errStageProposal
	}
	out := StageProposalContent{Summary: c.Summary, Children: make([]StageProposalChild, len(c.Children))}
	for i, child := range c.Children {
		title := strings.TrimSpace(child.Title)
		if title == "" || !stageText(title, 4*StageProposalTitleMaxRunes) || utf8.RuneCountInString(title) > StageProposalTitleMaxRunes ||
			!stageText(child.Description, StageProposalDescriptionMaxBytes) {
			return StageProposalContent{}, errStageProposal
		}
		switch child.Priority {
		case "urgent", "high", "medium", "low", "none":
		default:
			return StageProposalContent{}, errStageProposal
		}
		out.Children[i] = StageProposalChild{Title: title, Description: child.Description, Priority: child.Priority}
	}
	canonical, _, err := StageProposalDigest(out)
	if err != nil || len(canonical) > StageProposalMaxBytes {
		return StageProposalContent{}, errStageProposal
	}
	return out, nil
}

func stageCanonicalJSON(value any) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buffer.Bytes(), []byte("\n")), nil
}

// StageProposalDigest is the server's content identity. Clients echo it; they
// never compute it.
func StageProposalDigest(c StageProposalContent) ([]byte, string, error) {
	canonical, err := stageCanonicalJSON(c)
	if err != nil {
		return nil, "", err
	}
	sum := sha256.Sum256(canonical)
	return canonical, hex.EncodeToString(sum[:]), nil
}

// StageChildContentDigest identifies an existing ordinary child's reviewed
// content. Status, assignee and position are deliberately not part of it.
func StageChildContentDigest(title, description string) string {
	canonical, _ := stageCanonicalJSON(struct {
		Title       string `json:"title"`
		Description string `json:"description"`
	}{title, description})
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:])
}

// StageScopeFacts are loaded under the root's locks for the current request.
type StageScopeFacts struct {
	Managed             bool
	RoleRevision        int64
	ScopeDecision       bool
	ScopeLifecycle      int64
	RootLifecycle       int64
	ScopePolicyRevision int64
	// PolicyLastDisabled is the newest disabling policy revision, 0 when none.
	PolicyLastDisabled int64
	// SignerCurrent: the confirming account still holds the product approver
	// role with the participant and account generation it signed with.
	SignerCurrent bool
}

// StageScopeState reports whether the explicit existing-child confirmation
// still applies. Only "current" satisfies it.
func StageScopeState(f StageScopeFacts) string {
	switch {
	case !f.ScopeDecision:
		return "none"
	case f.ScopeLifecycle != f.RootLifecycle:
		return "lifecycle_changed"
	case f.ScopePolicyRevision <= f.PolicyLastDisabled:
		return "policy_changed"
	case !f.SignerCurrent:
		return "signer_changed"
	default:
		return "current"
	}
}

// StageMissing lists what a managed root still needs before new stage progress
// or new automatic decomposition. Enabling a policy never fills these in.
func StageMissing(f StageScopeFacts) []string {
	missing := []string{}
	if !f.Managed {
		return missing
	}
	if f.RoleRevision < 1 {
		missing = append(missing, "roles")
	}
	if StageScopeState(f) != "current" {
		missing = append(missing, "scope")
	}
	return missing
}

// StageChildFacts describe one ordinary issue against its persistent root
// mapping. Removing the parent link does not change them.
type StageChildFacts struct {
	Managed bool
	// MachineOrigin: materialized from an accepted proposal, or created with an
	// agent creator. Human-created children keep their ordinary contract.
	MachineOrigin     bool
	Mapped            bool
	ApprovedLifecycle int64
	RootLifecycle     int64
}

// StageChildExecutionRefusal gates new enqueue and queued-to-started only.
func StageChildExecutionRefusal(f StageChildFacts) string {
	if !f.Managed || !f.MachineOrigin {
		return ""
	}
	if !f.Mapped {
		return "stage_child_unapproved"
	}
	if f.ApprovedLifecycle != f.RootLifecycle {
		return "stage_scope_unconfirmed"
	}
	return ""
}

// StageMachineCreation classifies a creation by the verified credential and the
// server-resolved creator, never by a client-supplied marker. An absent
// principal with a member creator is a trusted internal human-attributed caller.
func StageMachineCreation(creatorType string, principal Principal, authenticated bool) bool {
	return creatorType != "member" || (authenticated && !principal.Human())
}
