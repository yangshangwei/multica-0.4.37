// Package material owns versioned engineering-document contracts, not workflow authority.
package material

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"path"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

const MaxContent = 256 * 1024

var shaPattern = regexp.MustCompile(`^(?:[a-f0-9]{40}|[a-f0-9]{64})$`)
var acceptancePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{0,39}[:：\s]+\S`)

type Settings struct {
	Version            int64    `json:"version"`
	Enabled            bool     `json:"enabled"`
	Provider           string   `json:"provider"`
	ResourceID         string   `json:"resource_id"`
	ExecutionRuntimeID *string  `json:"execution_runtime_id"`
	Repository         string   `json:"repository"`
	BaseBranch         string   `json:"base_branch"`
	RootPath           string   `json:"root_path"`
	ReviewerIDs        []string `json:"reviewer_ids"`
	AutoStartPlan      bool     `json:"auto_start_plan"`
}
type SettingsInput struct {
	RequestID          string   `json:"request_id"`
	ExpectedVersion    int64    `json:"expected_version"`
	Enabled            bool     `json:"enabled"`
	Provider           string   `json:"provider"`
	ResourceID         string   `json:"resource_id"`
	ExecutionRuntimeID *string  `json:"execution_runtime_id"`
	Repository         string   `json:"repository"`
	BaseBranch         string   `json:"base_branch"`
	RootPath           string   `json:"root_path"`
	ReviewerIDs        []string `json:"reviewer_ids"`
	AutoStartPlan      *bool    `json:"auto_start_plan"`
}

func (in SettingsInput) Settings() Settings {
	auto := true
	if in.AutoStartPlan != nil {
		auto = *in.AutoStartPlan
	}
	return Settings{Version: in.ExpectedVersion + 1, Enabled: in.Enabled, Provider: in.Provider, ResourceID: in.ResourceID, ExecutionRuntimeID: in.ExecutionRuntimeID, Repository: in.Repository, BaseBranch: in.BaseBranch, RootPath: in.RootPath, ReviewerIDs: in.ReviewerIDs, AutoStartPlan: auto}
}

type SettingsEnvelope struct {
	ExecutionProtocol int       `json:"execution_protocol"`
	WorkspaceID       string    `json:"workspace_id"`
	ProjectID         string    `json:"project_id"`
	CanConfigure      bool      `json:"can_configure"`
	Settings          *Settings `json:"settings"`
}
type Source struct {
	ResourceID string `json:"resource_id"`
	Commit     string `json:"commit"`
	Path       string `json:"path"`
}
type Material struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspace_id"`
	ProjectID   string    `json:"project_id"`
	Title       string    `json:"title"`
	Path        string    `json:"path"`
	Version     int64     `json:"version"`
	Status      string    `json:"status"`
	Blocker     *string   `json:"blocker"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
type CreateInput struct {
	RequestID string `json:"request_id"`
	Title     string `json:"title"`
}
type RevisionInput struct {
	RequestID          string   `json:"request_id"`
	ExpectedVersion    int64    `json:"expected_version"`
	Kind               string   `json:"kind"`
	Content            string   `json:"content"`
	PlanContent        *string  `json:"plan_content,omitempty"`
	Sources            []Source `json:"sources"`
	AcceptanceCriteria []string `json:"acceptance_criteria"`
	BlockingQuestions  []string `json:"blocking_questions"`
	Ready              bool     `json:"ready"`
	SourceBaselineID   *string  `json:"source_baseline_id,omitempty"`
}
type Revision struct {
	ID                 string    `json:"id"`
	Kind               string    `json:"kind"`
	Content            string    `json:"content"`
	PlanContent        *string   `json:"plan_content"`
	ContentSHA256      string    `json:"content_sha256"`
	Sources            []Source  `json:"sources"`
	AcceptanceCriteria []string  `json:"acceptance_criteria"`
	BlockingQuestions  []string  `json:"blocking_questions"`
	SourceBaselineID   *string   `json:"source_baseline_id"`
	PublicationStatus  string    `json:"publication_status"`
	ReviewURL          *string   `json:"review_url"`
	HeadSHA            *string   `json:"head_sha"`
	CreatedAt          time.Time `json:"created_at"`
}
type DecisionInput struct {
	RequestID       string `json:"request_id"`
	ExpectedVersion int64  `json:"expected_version"`
	RevisionID      string `json:"revision_id"`
	HeadSHA         string `json:"head_sha"`
	Decision        string `json:"decision"`
	Reason          string `json:"reason"`
}
type Decision struct {
	ID         string    `json:"id"`
	RevisionID string    `json:"revision_id"`
	ActorID    string    `json:"actor_id"`
	Decision   string    `json:"decision"`
	Reason     string    `json:"reason"`
	HeadSHA    string    `json:"head_sha"`
	CreatedAt  time.Time `json:"created_at"`
}
type CommentInput struct {
	RequestID  string `json:"request_id"`
	RevisionID string `json:"revision_id"`
	Body       string `json:"body"`
}
type Comment struct {
	ID         string    `json:"id"`
	RevisionID string    `json:"revision_id"`
	ActorID    string    `json:"actor_id"`
	Body       string    `json:"body"`
	CreatedAt  time.Time `json:"created_at"`
}
type CommandInput struct {
	RequestID       string `json:"request_id"`
	ExpectedVersion int64  `json:"expected_version"`
}
type Baseline struct {
	ID            string    `json:"id"`
	RevisionID    string    `json:"revision_id"`
	Commit        string    `json:"commit"`
	Path          string    `json:"path"`
	ContentSHA256 string    `json:"content_sha256"`
	ReviewURL     string    `json:"review_url"`
	VerifiedAt    time.Time `json:"verified_at"`
}
type Plan struct {
	IssueID    *string `json:"issue_id"`
	Status     string  `json:"status"`
	Blocker    *string `json:"blocker"`
	BaselineID string  `json:"baseline_id"`
	RevisionID *string `json:"revision_id"`
}
type Capabilities struct {
	CanPublish   bool `json:"can_publish"`
	CanEdit      bool `json:"can_edit"`
	CanReview    bool `json:"can_review"`
	CanStartPlan bool `json:"can_start_plan"`
	CanRetry     bool `json:"can_retry"`
}
type Operation struct {
	ID                 string  `json:"id"`
	Kind               string  `json:"kind"`
	Status             string  `json:"status"`
	TaskID             string  `json:"task_id"`
	ExecutionRuntimeID string  `json:"execution_runtime_id"`
	Blocker            *string `json:"blocker"`
}
type Detail struct {
	Operation    *Operation   `json:"operation,omitempty"`
	Material     Material     `json:"material"`
	Revisions    []Revision   `json:"revisions"`
	Decisions    []Decision   `json:"decisions"`
	Comments     []Comment    `json:"comments"`
	Baseline     *Baseline    `json:"baseline"`
	Plan         *Plan        `json:"plan"`
	Capabilities Capabilities `json:"capabilities"`
}
type List struct {
	WorkspaceID string     `json:"workspace_id"`
	ProjectID   string     `json:"project_id"`
	Materials   []Material `json:"materials"`
}

func Digest(s string) string  { sum := sha256.Sum256([]byte(s)); return hex.EncodeToString(sum[:]) }
func JSONDigest(v any) string { b, _ := json.Marshal(v); return Digest(string(b)) }
func IsSHA(s string) bool     { return shaPattern.MatchString(s) }
func ValidPath(s string, empty bool) bool {
	if s == "" {
		return empty
	}
	if !utf8.ValidString(s) || len(s) > 512 || strings.ContainsAny(s, "\\\x00\r\n") || strings.HasPrefix(s, "/") || path.Clean(s) != s {
		return false
	}
	for _, part := range strings.Split(s, "/") {
		if part == "." || part == ".." || strings.EqualFold(part, ".git") {
			return false
		}
	}
	return true
}
func ValidateSettings(s Settings) error {
	if (s.Provider != "github" && s.Provider != "gitlab") || !ValidPath(s.Repository, false) || !strings.Contains(s.Repository, "/") || strings.HasSuffix(s.Repository, ".git") || !ValidPath(s.RootPath, true) || !ValidPath(s.BaseBranch, false) || strings.ContainsAny(s.BaseBranch, " ~^:?*[\t") || strings.Contains(s.BaseBranch, "..") || strings.Contains(s.BaseBranch, "@{") || strings.HasSuffix(s.BaseBranch, ".lock") || len(s.ReviewerIDs) == 0 || len(s.ReviewerIDs) > 32 || s.Enabled && (s.ExecutionRuntimeID == nil || *s.ExecutionRuntimeID == "") {
		return errors.New("invalid_material_settings")
	}
	return nil
}
func ValidateRevision(in RevisionInput) error {
	if in.Kind != "requirement" && in.Kind != "plan" {
		return errors.New("invalid_revision_kind")
	}
	if !utf8.ValidString(in.Content) || len(in.Content) > MaxContent || strings.ContainsRune(in.Content, 0) || len(in.Sources) > 100 || len(in.AcceptanceCriteria) > 200 || len(in.BlockingQuestions) > 100 {
		return errors.New("invalid_revision")
	}
	if in.PlanContent != nil && (!utf8.ValidString(*in.PlanContent) || len(*in.PlanContent) > MaxContent || strings.ContainsRune(*in.PlanContent, 0)) {
		return errors.New("invalid_plan")
	}
	for _, src := range in.Sources {
		if src.ResourceID == "" || !IsSHA(src.Commit) || !ValidPath(src.Path, false) {
			return errors.New("invalid_source")
		}
	}
	if !in.Ready {
		return nil
	}
	if len(strings.TrimSpace(in.Content)) < 40 || len(in.Sources) == 0 || len(in.AcceptanceCriteria) == 0 || len(in.BlockingQuestions) > 0 {
		return errors.New("material_not_reviewable")
	}
	if in.Kind == "plan" && (in.PlanContent == nil || len(strings.TrimSpace(*in.PlanContent)) < 40 || in.SourceBaselineID == nil) {
		return errors.New("plan_not_reviewable")
	}
	seen := map[string]bool{}
	for _, ac := range in.AcceptanceCriteria {
		if len(ac) > 4000 || !acceptancePattern.MatchString(ac) {
			return errors.New("invalid_acceptance_criterion")
		}
		id := strings.FieldsFunc(ac, func(r rune) bool { return r == ':' || r == '：' || r == ' ' || r == '\t' })[0]
		if seen[id] {
			return errors.New("duplicate_acceptance_id")
		}
		seen[id] = true
	}
	return nil
}
