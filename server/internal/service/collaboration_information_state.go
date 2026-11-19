package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const informationReadTimeout = 5 * time.Second

var informationEstimatePattern = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\.[0-9]{1,6})?(Z|[+-][0-9]{2}:[0-9]{2})$`)

type informationBlockValue struct {
	Condition string
	Estimate  time.Time
}

func informationText(value string) bool {
	return utf8.ValidString(value) && len(value) <= 4096 && strings.TrimSpace(value) != "" && !strings.ContainsRune(value, 0)
}

func validInformationEstimate(value time.Time) bool {
	year := value.UTC().Year()
	return year >= 1 && year <= 9999 && value.Equal(value.Truncate(time.Microsecond))
}

func parseInformationEstimate(raw string) (time.Time, bool) {
	if !informationEstimatePattern.MatchString(raw) {
		return time.Time{}, false
	}
	if !strings.HasSuffix(raw, "Z") {
		offset := raw[len(raw)-5:]
		hours, e1 := strconv.Atoi(offset[:2])
		minutes, e2 := strconv.Atoi(offset[3:])
		if e1 != nil || e2 != nil || hours > 23 || minutes > 59 {
			return time.Time{}, false
		}
	}
	value, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil || value.Year() < 1 || !validInformationEstimate(value) {
		return time.Time{}, false
	}
	return value.UTC(), true
}

// Missing legacy fields retain their old digest. The caller checks the final
// database clock only for a new write after original-operation recovery.
func informationMetadata(note string, condition, estimate *string, managed bool) (*informationBlockValue, string) {
	if condition == nil && estimate == nil {
		if managed {
			return nil, "information_metadata_required"
		}
		return nil, ""
	}
	if condition == nil || estimate == nil || !informationText(note) || !informationText(*condition) {
		return nil, "information_metadata_invalid"
	}
	instant, valid := parseInformationEstimate(*estimate)
	if !valid {
		return nil, "information_metadata_invalid"
	}
	return &informationBlockValue{Condition: *condition, Estimate: instant}, ""
}

func informationTime(value time.Time) string {
	return value.UTC().Format("2006-01-02T15:04:05.000000Z")
}

type CollaborationInformationBlock struct {
	ProtocolVersion       int     `json:"protocol_version"`
	State                 string  `json:"state"`
	UnblockCondition      *string `json:"unblock_condition,omitempty"`
	EstimatedResolutionAt *string `json:"estimated_resolution_at,omitempty"`
	ReasonCode            *string `json:"reason_code,omitempty"`
}

func informationBlock(row db.IssueHandoff) CollaborationInformationBlock {
	out := CollaborationInformationBlock{ProtocolVersion: 1, State: "legacy"}
	if !row.UnblockCondition.Valid && !row.EstimatedResolutionAt.Valid {
		return out
	}
	if row.UnblockCondition.Valid && row.EstimatedResolutionAt.Valid && informationText(row.Note) && informationText(row.UnblockCondition.String) &&
		row.ToUserID.Valid && row.ToUserID.Bytes != [16]byte{} && row.EstimatedResolutionAt.InfinityModifier == pgtype.Finite &&
		validInformationEstimate(row.EstimatedResolutionAt.Time) {
		estimate := informationTime(row.EstimatedResolutionAt.Time)
		out.State, out.UnblockCondition, out.EstimatedResolutionAt = "complete", &row.UnblockCondition.String, &estimate
		return out
	}
	code := "information_metadata_invalid"
	out.State, out.ReasonCode = "unavailable", &code
	return out
}

type CollaborationInformationAction struct {
	Available bool    `json:"available"`
	Code      *string `json:"code"`
}

type CollaborationInformationActions struct {
	Request CollaborationInformationAction `json:"request"`
	Return  CollaborationInformationAction `json:"return"`
	Repair  CollaborationInformationAction `json:"repair"`
}

type CollaborationInformationWorkflow struct {
	ProtocolVersion int                             `json:"protocol_version"`
	Managed         bool                            `json:"managed"`
	ContextVersion  string                          `json:"context_version"`
	ActiveRequestID pgtype.UUID                     `json:"active_request_id"`
	RecordState     string                          `json:"record_state"`
	Actions         CollaborationInformationActions `json:"actions"`
}

type informationPolicy struct {
	SourceWorkspaceID pgtype.UUID
	ProjectID         pgtype.UUID
	Contract          string
	Revision          int64
	Managed           bool
}

func informationSchemaReady(ctx context.Context, q *db.Queries) error {
	ready, err := q.CollaborationInformationSchemaReady(ctx)
	if err != nil {
		return err
	}
	if !ready.Valid || !ready.Bool {
		return collaborationError(http.StatusServiceUnavailable, "information_context_unavailable")
	}
	return nil
}

// Applicability borrows the established project fence; it does not prepare or
// consume a stage completion, create a stage head, or require source membership.
func informationPolicyFor(ctx context.Context, q *db.Queries, issue db.Issue, lock bool) (informationPolicy, error) {
	out := informationPolicy{SourceWorkspaceID: issue.WorkspaceID, ProjectID: issue.ProjectID, Contract: collaboration.StageContract}
	if !issue.ProjectID.Valid {
		return out, nil
	}
	var err error
	if lock {
		_, err = q.LockProjectForAssociationNowait(ctx, db.LockProjectForAssociationNowaitParams{ID: issue.ProjectID, WorkspaceID: issue.WorkspaceID})
	} else {
		_, err = q.GetProjectInWorkspace(ctx, db.GetProjectInWorkspaceParams{ID: issue.ProjectID, WorkspaceID: issue.WorkspaceID})
	}
	if err != nil {
		return out, err
	}
	if err = stageReady(ctx, q); err != nil {
		return out, err
	}
	row, found, err := currentStagePolicy(ctx, q, issue.WorkspaceID, issue.ProjectID)
	if err != nil {
		return out, err
	}
	if found {
		if row.Contract != collaboration.StageContract || row.Revision < 1 {
			return out, collaborationError(http.StatusServiceUnavailable, "information_context_unavailable")
		}
		out.Revision, out.Managed = row.Revision, row.Enabled
	}
	return out, nil
}

type informationState struct {
	Access           CollaborationIssueAccess
	Policy           informationPolicy
	Actor            pgtype.UUID
	RawActive        pgtype.UUID
	Request          *db.IssueHandoff
	Block            *CollaborationInformationBlock
	Pending          *CollaborationSubmission
	PendingEvent     *db.CollaborationEvent
	PendingUnknown   bool
	ChainCode        string
	Origin           *CollaborationInformationOrigin
	ContextVersion   string
	AuthorityVersion string
}

func informationEligible(ctx context.Context, q *db.Queries, source, user pgtype.UUID, participant bool) (bool, error) {
	if !user.Valid || user.Bytes == [16]byte{} {
		return false, nil
	}
	if participant {
		row, err := q.GetCollaborationParticipant(ctx, db.GetCollaborationParticipantParams{IssueID: source, UserID: user})
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		if err != nil || row.RevokedAt.Valid {
			return false, err
		}
	}
	u, err := q.GetUser(ctx, user)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil || u.DisabledAt.Valid || auth.IsTemporarilyDisabledUser(util.UUIDToString(user), u.Email.String) {
		return false, err
	}
	return true, nil
}

func informationReturnEligible(ctx context.Context, q *db.Queries, access CollaborationIssueAccess, user pgtype.UUID) (bool, error) {
	active, err := informationEligible(ctx, q, access.Issue.ID, user, true)
	if err != nil || !active {
		return false, err
	}
	return collaborationRecipientEligible(ctx, q, user, access.Issue.WorkspaceID)
}

func informationEvent(ctx context.Context, q *db.Queries, row db.IssueHandoff, kind string) (*db.CollaborationEvent, string, error) {
	events, err := q.ListCollaborationInformationEvents(ctx, db.ListCollaborationInformationEventsParams{IssueID: row.IssueID, HandoffID: row.ID})
	if err != nil {
		return nil, "", err
	}
	if len(events) == 0 {
		return nil, "legacy_unlinked", nil
	}
	if len(events) != 1 || events[0].Kind != kind || events[0].ActorUserID != row.ActorUserID {
		return nil, "unavailable", nil
	}
	var payload struct {
		HandoffID    pgtype.UUID `json:"handoff_id"`
		TargetUserID pgtype.UUID `json:"target_user_id"`
	}
	if json.Unmarshal(events[0].Payload, &payload) != nil || payload.HandoffID != row.ID || payload.TargetUserID != row.ToUserID {
		return nil, "unavailable", nil
	}
	return &events[0], "linked", nil
}

func informationChain(ctx context.Context, q *db.Queries, state *informationState, catalog []db.IssueStatus) (string, *CollaborationInformationOrigin, error) {
	if state.Request == nil || state.PendingUnknown {
		return "information_context_unavailable", nil, nil
	}
	row := *state.Request
	expectedRecipient := state.Access.Issue.AssigneeID
	seen := map[pgtype.UUID]bool{}
	currentCode := ""
	for {
		if ctx.Err() != nil {
			return "", nil, ctx.Err()
		}
		if seen[row.ID] || row.IssueID != state.Access.Issue.ID || row.Purpose != "information" || row.Resolution != "open" || row.ToUserID != expectedRecipient || row.ActorUserID != row.RequesterUserID || row.SupersededByRepairHandoffID.Valid {
			return "information_context_unavailable", nil, nil
		}
		seen[row.ID] = true
		if informationBlock(row).State != "complete" {
			return "information_repair_required", nil, nil
		}
		eligible, err := informationReturnEligible(ctx, q, state.Access, row.RequesterUserID)
		if err != nil {
			return "", nil, err
		}
		if !eligible {
			currentCode = "return_target_unavailable"
		}
		event, provenance, err := informationEvent(ctx, q, row, "information_requested")
		if err != nil {
			return "", nil, err
		}
		if provenance != "linked" {
			return "information_context_unavailable", nil, nil
		}
		if row.ParentRequestID.Valid {
			saved, ok := collaborationCatalogEntry(catalog, row.PreviousStatus)
			if !ok || saved.Category != "blocked" || saved.ArchivedAt.Valid {
				return "information_repair_required", nil, nil
			}
			expectedRecipient = row.RequesterUserID
			parent, err := q.GetCollaborationInformationRequest(ctx, db.GetCollaborationInformationRequestParams{IssueID: row.IssueID, RequestID: row.ParentRequestID})
			if errors.Is(err, pgx.ErrNoRows) {
				return "information_context_unavailable", nil, nil
			}
			if err != nil {
				return "", nil, err
			}
			row = parent
			continue
		}
		status, fallback, available := collaborationReturnStatus(catalog, row.PreviousStatus)
		entry, found := collaborationCatalogEntry(catalog, status)
		if !available || !found || entry.Category == "blocked" {
			return "information_repair_required", nil, nil
		}
		if state.Pending != nil && (entry.Category != "in_review" || fallback || row.RequesterUserID != state.Access.Issue.CreatorID) {
			return "information_repair_required", nil, nil
		}
		if !fallback {
			return currentCode, &CollaborationInformationOrigin{OriginRequestID: row.ID, OriginEventID: event.ID, Status: row.PreviousStatus}, nil
		}
		return currentCode, nil, nil
	}
}

func informationAuthorityVersion(ctx context.Context, q *db.Queries, access CollaborationIssueAccess, actor pgtype.UUID) (string, error) {
	var member *db.Member
	if access.SourceMember {
		row, err := q.GetMemberByUserAndWorkspace(ctx, db.GetMemberByUserAndWorkspaceParams{UserID: actor, WorkspaceID: access.Issue.WorkspaceID})
		if err != nil {
			return "", err
		}
		member = &row
	}
	credential := completionRequestCredential(ctx)
	// Only a private fingerprint is emitted. No credential or token hash enters
	// a response/cursor; source-only readers retain actual nullable fence values.
	var passwordVersion *int64
	if credential.PasswordSession != nil {
		passwordVersion = &credential.PasswordSession.Version
	}
	patGeneration := ""
	if credential.Principal.Kind == auth.CredentialPAT {
		generation := sha256.Sum256([]byte(credential.Principal.PATTokenHash))
		patGeneration = hex.EncodeToString(generation[:])
	}
	authority, err := json.Marshal(struct {
		Actor            pgtype.UUID
		AuthorityEpoch   int64
		ParticipantEpoch pgtype.Int8
		AccountEpoch     pgtype.Int8
		Member           *db.Member
		CredentialKind   auth.CredentialKind
		CredentialExpiry time.Time
		PasswordVersion  *int64
		PATGeneration    string
	}{actor, access.AuthorityEpoch, access.ParticipantEpoch, access.AccountEpoch, member,
		credential.Principal.Kind, credential.Principal.ExpiresAt, passwordVersion, patGeneration})
	if err != nil {
		return "", err
	}
	fingerprint := sha256.Sum256(authority)
	return hex.EncodeToString(fingerprint[:]), nil
}

func loadInformationState(ctx context.Context, q *db.Queries, access CollaborationIssueAccess, catalog []db.IssueStatus, lock bool) (*informationState, error) {
	if err := informationSchemaReady(ctx, q); err != nil {
		return nil, err
	}
	principal, ok := collaboration.PrincipalFromContext(ctx)
	if !ok || !principal.Human() {
		return nil, collaborationError(http.StatusNotFound, "not_found")
	}
	actor, err := util.ParseUUID(principal.UserID)
	if err != nil {
		return nil, collaborationError(http.StatusNotFound, "not_found")
	}
	state := &informationState{Access: access, Actor: actor}
	if state.Policy, err = informationPolicyFor(ctx, q, access.Issue, lock); err != nil {
		return nil, err
	}
	if state.RawActive, err = q.GetCollaborationInformationPointer(ctx, access.Issue.ID); err != nil {
		return nil, err
	}
	if state.RawActive.Valid {
		row, e := q.GetCollaborationInformationRequest(ctx, db.GetCollaborationInformationRequestParams{IssueID: access.Issue.ID, RequestID: state.RawActive})
		if e != nil && !errors.Is(e, pgx.ErrNoRows) {
			return nil, e
		}
		if e == nil && row.Resolution == "open" {
			state.Request = &row
			block := informationBlock(row)
			state.Block = &block
		}
	}
	summary, err := collaborationDeliverySummary(ctx, q, access)
	if err != nil && !errors.Is(err, errCollaborationDeliveryEvidence) {
		return nil, err
	}
	state.Pending, state.PendingUnknown = summary.Current, errors.Is(err, errCollaborationDeliveryEvidence)
	if state.Pending != nil {
		if state.Pending.EvidenceState != "available" {
			state.PendingUnknown = true
		} else {
			event, e := q.GetCollaborationInformationSubmissionEvent(ctx, db.GetCollaborationInformationSubmissionEventParams{IssueID: access.Issue.ID, EventSeq: state.Pending.EventSeq})
			if e != nil {
				return nil, e
			}
			var payload collaborationDeliveryEventPayload
			if json.Unmarshal(event.Payload, &payload) != nil || payload.HandoffID != state.Pending.ID || event.ActorUserID != state.Pending.AuthorUserID {
				state.PendingUnknown = true
			} else {
				state.PendingEvent = &event
			}
		}
	}
	if state.Policy.Managed && state.RawActive.Valid {
		state.ChainCode, state.Origin, err = informationChain(ctx, q, state, catalog)
		if err != nil {
			return nil, err
		}
	}
	facts, err := q.ListCollaborationInformationAuthorityFacts(ctx, access.Issue.ID)
	if err != nil {
		return nil, err
	}
	state.AuthorityVersion, err = informationAuthorityVersion(ctx, q, access, actor)
	if err != nil {
		return nil, err
	}
	contextBytes, err := json.Marshal(struct {
		Policy         informationPolicy
		Status         string
		Handler        pgtype.UUID
		Lifecycle      int64
		Revision       int64
		RawActive      pgtype.UUID
		Request        *db.IssueHandoff
		PendingID      pgtype.UUID
		PendingEventID pgtype.UUID
		PendingUnknown bool
		Authority      string
		Accounts       []db.ListCollaborationInformationAuthorityFactsRow
	}{Policy: state.Policy, Status: access.Issue.Status, Handler: access.Issue.AssigneeID, Lifecycle: access.LifecycleEpoch,
		Revision: access.Revision, RawActive: state.RawActive, Request: state.Request, PendingUnknown: state.PendingUnknown,
		Authority: state.AuthorityVersion, Accounts: facts,
		PendingID: func() pgtype.UUID {
			if state.Pending != nil {
				return state.Pending.ID
			}
			return pgtype.UUID{}
		}(),
		PendingEventID: func() pgtype.UUID {
			if state.PendingEvent != nil {
				return state.PendingEvent.ID
			}
			return pgtype.UUID{}
		}()})
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(contextBytes)
	state.ContextVersion = hex.EncodeToString(digest[:])
	return state, nil
}

func informationAction(available bool, code string) CollaborationInformationAction {
	out := CollaborationInformationAction{Available: available}
	if !available {
		out.Code = &code
	}
	return out
}

func (state *informationState) workflow(ctx context.Context, q *db.Queries, catalog []db.IssueStatus) (CollaborationInformationWorkflow, error) {
	out := CollaborationInformationWorkflow{ProtocolVersion: 1, Managed: state.Policy.Managed, ContextVersion: state.ContextVersion,
		ActiveRequestID: state.RawActive, RecordState: "none"}
	if state.RawActive.Valid {
		out.RecordState = "unavailable"
		if state.Block != nil {
			out.RecordState = state.Block.State
		}
	}
	open := collaboration.NonterminalCategory(state.Access.StatusCategory) && state.Access.QueueState != "closed"
	request := state.Access.Capabilities.Transfer && open
	requestCode := "forbidden"
	if state.Policy.Managed && state.Access.StatusCategory == "blocked" &&
		(state.Request == nil || state.Request.ToUserID != state.Actor || state.Access.Issue.AssigneeID != state.Actor || state.ChainCode != "") {
		request, requestCode = false, "information_repair_required"
	}
	canReturn := state.Access.Capabilities.ReturnInformation && open && state.Request != nil
	returnCode := "information_request_not_active"
	if canReturn {
		eligible, err := informationReturnEligible(ctx, q, state.Access, state.Request.RequesterUserID)
		if err != nil {
			return out, err
		}
		if !eligible {
			canReturn, returnCode = false, "return_target_unavailable"
		} else if state.Policy.Managed && state.ChainCode != "" {
			canReturn, returnCode = false, state.ChainCode
		}
	}
	repair := state.Policy.Managed && open && state.Access.StatusCategory == "blocked" &&
		state.Access.Capabilities.Transfer && state.Access.Capabilities.ChangeStatus
	repairCode := "information_repair_not_applicable"
	if repair {
		_, err := informationRepairOptions(ctx, q, state, catalog)
		if err != nil {
			var domain *CollaborationError
			if !errors.As(err, &domain) {
				return out, err
			}
			repair, repairCode = false, domain.Code
		}
	}
	out.Actions = CollaborationInformationActions{Request: informationAction(request, requestCode), Return: informationAction(canReturn, returnCode), Repair: informationAction(repair, repairCode)}
	return out, nil
}

// InformationWorkflowView borrows the detail's already authorized snapshot.
func (s *CollaborationService) InformationWorkflowView(ctx context.Context, access CollaborationIssueAccess) (*CollaborationInformationWorkflow, error) {
	catalog, err := s.Queries.ListIssueStatusEntries(ctx, db.ListIssueStatusEntriesParams{WorkspaceID: access.Issue.WorkspaceID, IncludeArchived: true})
	if err != nil {
		return nil, err
	}
	state, err := loadInformationState(ctx, s.Queries, access, catalog, false)
	if err != nil {
		return nil, err
	}
	view, err := state.workflow(ctx, s.Queries, catalog)
	return &view, err
}
