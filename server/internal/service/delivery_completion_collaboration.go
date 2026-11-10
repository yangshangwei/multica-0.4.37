package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// Only verified middleware values are captured here. Retained issuers are passed
// explicitly; their identity is never installed into the consumer's context.
func completionRequestCredential(ctx context.Context) ArtifactCredentialSnapshot {
	p, _ := auth.AuthenticatedPrincipalFromContext(ctx)
	c := ArtifactCredentialSnapshot{Principal: p}
	if s, ok := auth.PasswordSessionFromContext(ctx); ok {
		c.PasswordSession = &s
	}
	if t, ok := auth.VerifiedTaskTokenFromContext(ctx); ok {
		c.TaskToken = &t
	}
	return c
}
func completionIsCollaboration(kind string) bool {
	return kind == "collaboration_submission" || kind == "collaboration_self" || kind == "collaboration_admin"
}

// This explicit-user projection borrows the writer transaction. It does not use
// an ambient principal or grant source membership to an outside participant.
func completionCollaborationAccess(ctx context.Context, tx pgx.Tx, issue db.Issue, viewer pgtype.UUID) (CollaborationIssueAccess, error) {
	q := db.New(tx)
	users, err := q.LockArtifactUsers(ctx, []pgtype.UUID{viewer})
	if err != nil {
		return CollaborationIssueAccess{}, err
	}
	if len(users) != 1 || users[0].DisabledAt.Valid || auth.IsTemporarilyDisabledUser(util.UUIDToString(viewer), users[0].Email.String) {
		return CollaborationIssueAccess{}, completionDenied()
	}
	for _, statement := range []string{
		`SELECT issue_id FROM issue_collaboration WHERE issue_id=$1 FOR UPDATE NOWAIT`,
		`SELECT id FROM collaboration_session WHERE issue_id=$1 FOR UPDATE NOWAIT`,
	} {
		if _, err = tx.Exec(ctx, statement, issue.ID); err != nil {
			return CollaborationIssueAccess{}, err
		}
	}
	if _, err = tx.Exec(ctx, `SELECT user_id FROM issue_participant WHERE issue_id=$1 AND user_id=$2 FOR SHARE NOWAIT`, issue.ID, viewer); err != nil {
		return CollaborationIssueAccess{}, err
	}
	if _, err = tx.Exec(ctx, `SELECT user_id FROM collaboration_account_fence WHERE user_id=$1 FOR SHARE NOWAIT`, viewer); err != nil {
		return CollaborationIssueAccess{}, err
	}
	row, err := q.GetCollaborationAccess(ctx, db.GetCollaborationAccessParams{ViewerID: viewer, IssueID: issue.ID})
	if err != nil {
		return CollaborationIssueAccess{}, err
	}
	if row.Issue.WorkspaceID != issue.WorkspaceID {
		return CollaborationIssueAccess{}, completionConflict()
	}
	caps := collaboration.CapabilitiesFor(collaboration.Principal{UserID: util.UUIDToString(viewer), Kind: auth.CredentialJWT}, collaboration.AccessState{AccountActive: row.AccountActive, SourceMember: row.SourceMember, Participant: row.Participant, Creator: issue.CreatorType == "member" && issue.CreatorID == viewer, Handler: issue.AssigneeType.String == "member" && issue.AssigneeID == viewer, InformationRecipient: row.InformationRequestID.Valid && row.InformationRecipientID == viewer, StatusCategory: row.StatusCategory, QueueState: row.QueueState, AIPolicy: row.AiPolicy})
	if !caps.Read {
		return CollaborationIssueAccess{}, pgx.ErrNoRows
	}
	return CollaborationIssueAccess{Issue: row.Issue, Revision: row.CollaborationRevision, LifecycleEpoch: row.LifecycleEpoch, AuthorityEpoch: row.AuthorityEpoch, EventSequence: row.EventSeq, SessionID: row.SessionID, QueueState: row.QueueState, ControlVersion: row.ControlVersion, ParticipantEpoch: row.ParticipantEpoch, AccountEpoch: row.AccountEpoch, StatusCategory: row.StatusCategory, SourceMember: row.SourceMember, AIPolicy: row.AiPolicy, InformationRequestID: row.InformationRequestID, Capabilities: caps}, nil
}

func completionSourceIssue(ctx context.Context, tx pgx.Tx, req DeliveryCompletionAuthorizationRequest, target db.Issue, exact bool) (db.Issue, error) {
	if req.Target.Kind == "ordinary" {
		return target, nil
	}
	if !completionIsCollaboration(req.Target.Kind) {
		return db.Issue{}, completionUnsupported()
	}
	q := db.New(tx)
	child, err := q.LockLifecycleIssue(ctx, db.LockLifecycleIssueParams{ID: completionUUIDPG(req.Evidence.IssueID), WorkspaceID: target.WorkspaceID})
	if err != nil {
		return child, err
	}
	if err = completionOrdinary(ctx, q, child); err != nil {
		return child, err
	}
	if exact && (child.ID == target.ID || child.ParentIssueID != target.ID || child.ProjectID != target.ProjectID || child.Revision != req.Evidence.ExpectedIssueRevision) {
		return child, completionConflict()
	}
	return child, nil
}

// Nomination checks an administrator's present role and source organization;
// consuming additionally requires the actual complete password session.
func completionAdminNomination(ctx context.Context, tx pgx.Tx, issue db.Issue, user, org pgtype.UUID, newAdmission bool) (CollaborationIssueAccess, error) {
	q := db.New(tx)
	var locked bool
	if err := tx.QueryRow(ctx, `SELECT pg_try_advisory_xact_lock(70856394211471801::bigint)`).Scan(&locked); err != nil {
		return CollaborationIssueAccess{}, err
	}
	if !locked {
		return CollaborationIssueAccess{}, completionConflict()
	}
	users, err := q.LockArtifactUsers(ctx, []pgtype.UUID{user, issue.CreatorID})
	if err != nil {
		return CollaborationIssueAccess{}, err
	}
	var creator db.User
	for _, u := range users {
		if u.ID == issue.CreatorID {
			creator = u
		}
		if u.ID == user && (u.DisabledAt.Valid || auth.IsTemporarilyDisabledUser(util.UUIDToString(user), u.Email.String)) {
			return CollaborationIssueAccess{}, completionDenied()
		}
	}
	if !creator.ID.Valid {
		return CollaborationIssueAccess{}, completionDenied()
	}
	if _, err = tx.Exec(ctx, `SELECT user_id FROM platform_role_binding WHERE user_id=$1 FOR SHARE NOWAIT`, user); err != nil {
		return CollaborationIssueAccess{}, err
	}
	if _, err = q.LockArtifactPasswordCredential(ctx, user); err != nil {
		return CollaborationIssueAccess{}, err
	}
	account, err := q.GetPlatformAdminAccount(ctx, user)
	if err != nil {
		return CollaborationIssueAccess{}, err
	}
	if account.Role.String != PlatformRoleSuperAdmin || account.DisabledAt.Valid || account.MustChangePassword || account.SessionVersion <= 0 {
		return CollaborationIssueAccess{}, completionDenied()
	}
	if _, err = tx.Exec(ctx, `SELECT o.id FROM organization o JOIN organization_workspace ow ON ow.organization_id=o.id WHERE ow.workspace_id=$1 AND o.id=$2 AND o.internal AND o.state='active' FOR SHARE OF o,ow NOWAIT`, issue.WorkspaceID, org); err != nil {
		return CollaborationIssueAccess{}, err
	}
	row, err := q.GetAdminCollaborationIssue(ctx, db.GetAdminCollaborationIssueParams{OrganizationID: org, IssueID: issue.ID})
	if err != nil {
		return CollaborationIssueAccess{}, err
	}
	if newAdmission && !adminCollaborationAccountDisabled(creator.ID, creator.Email, creator.DisabledAt) {
		return CollaborationIssueAccess{}, completionDenied()
	}
	state, err := q.LockIssueCollaboration(ctx, issue.ID)
	if err != nil {
		return CollaborationIssueAccess{}, err
	}
	session, err := q.LockCollaborationSession(ctx, issue.ID)
	if err != nil {
		return CollaborationIssueAccess{}, err
	}
	return CollaborationIssueAccess{Issue: issue, Revision: state.Revision, LifecycleEpoch: state.LifecycleEpoch, AuthorityEpoch: state.AuthorityEpoch, EventSequence: session.EventSeq, SessionID: session.ID, QueueState: session.QueueState, ControlVersion: session.ControlVersion, StatusCategory: row.StatusCategory, InformationRequestID: state.ActiveInformationRequestID}, nil
}
func completionCollaborationConsumer(ctx context.Context, tx pgx.Tx, row db.DeliveryCompletionAuthorization, c ArtifactCredentialSnapshot, issue db.Issue, newAdmission bool) error {
	kind, _, err := c.Actor()
	if err != nil || kind != "human" || completionUUIDPG(c.Principal.UserID) != row.ConsumerUserID {
		return pgx.ErrNoRows
	}
	if err = CheckArtifactCredentialInTx(ctx, tx, c); err != nil {
		return err
	}
	if row.TargetKind == "collaboration_admin" {
		if !auth.PasswordMode() || c.Principal.Kind != auth.CredentialJWT || c.PasswordSession == nil || c.PasswordSession.Kind != "jwt" {
			return completionDenied()
		}
		_, err = completionAdminNomination(ctx, tx, issue, row.ConsumerUserID, row.AdminOrganizationID, newAdmission)
		return err
	}
	a, err := completionCollaborationAccess(ctx, tx, issue, row.ConsumerUserID)
	if err != nil {
		return err
	}
	if issue.CreatorType != "member" || issue.CreatorID != row.ConsumerUserID || !a.Capabilities.ControlAI {
		return completionDenied()
	}
	return nil
}

func completionCollaborationIntent(ctx context.Context, tx pgx.Tx, issue db.Issue, target DeliveryCompletionTarget, consumer DeliveryCompletionConsumer, checkVersions bool) (CollaborationIssueAccess, *CollaborationSubmission, string, error) {
	var a CollaborationIssueAccess
	var err error
	actor := completionUUIDPG(consumer.UserID)
	if target.Kind == "collaboration_admin" {
		a, err = completionAdminNomination(ctx, tx, issue, actor, completionUUIDPG(consumer.OrganizationID), true)
	} else {
		a, err = completionCollaborationAccess(ctx, tx, issue, actor)
	}
	if err != nil {
		return a, nil, "", err
	}
	if !collaboration.NonterminalCategory(a.StatusCategory) || a.QueueState == "closed" {
		return a, nil, "", completionConflict()
	}
	if checkVersions && (target.ExpectedCollaborationRevision != strconv.FormatInt(a.Revision, 10) || target.ExpectedLifecycleEpoch != strconv.FormatInt(a.LifecycleEpoch, 10)) {
		return a, nil, "", completionConflict()
	}
	if target.Kind != "collaboration_admin" {
		if issue.CreatorType != "member" || issue.CreatorID != actor || issue.AssigneeID != actor || !a.Capabilities.ControlAI || a.InformationRequestID.Valid {
			return a, nil, "", completionDenied()
		}
		if checkVersions && (target.ExpectedParticipantEpoch != strconv.FormatInt(a.ParticipantEpoch.Int64, 10) || target.ExpectedAccountEpoch != strconv.FormatInt(a.AccountEpoch.Int64, 10)) {
			return a, nil, "", completionConflict()
		}
	}
	q := db.New(tx)
	catalog, err := q.LockCollaborationStatusEntries(ctx, issue.WorkspaceID)
	if err != nil {
		return a, nil, "", err
	}
	options, err := collaborationStatusOptions(issue, "complete", catalog, db.IssueHandoff{})
	if err != nil {
		return a, nil, "", err
	}
	if options.StatusOptionsVersion != target.StatusOptionsVersion {
		return a, nil, "", completionConflict()
	}
	summary, err := collaborationDeliverySummary(ctx, q, a)
	if err != nil {
		return a, nil, "", err
	}
	current := summary.Current
	switch target.Kind {
	case "collaboration_submission":
		if current == nil || target.SubmissionID == nil || current.ID != completionUUIDPG(*target.SubmissionID) || current.EvidenceState != "available" || a.StatusCategory != "in_review" {
			return a, nil, "", completionConflict()
		}
	case "collaboration_self":
		if current != nil {
			return a, nil, "", completionConflict()
		}
		t := &collaborationWorkflowTx{collaborationQueueTx: &collaborationQueueTx{q: q, actor: actor, access: a}, tx: tx}
		input := completionSelfInput(target)
		refusal, e := t.prepareDeliveryMode(ctx, input, false)
		if e != nil {
			return a, nil, "", e
		}
		if refusal != nil {
			return a, nil, "", completionConflict()
		}
	case "collaboration_admin":
		if (target.SubmissionID != nil) != (current != nil) || current != nil && current.ID != completionUUIDPG(*target.SubmissionID) {
			return a, nil, "", completionConflict()
		}
	default:
		return a, nil, "", completionUnsupported()
	}
	digest, err := completionIntentDigest(ctx, tx, target, current)
	return a, current, digest, err
}
func completionSelfInput(t DeliveryCompletionTarget) collaborationDeliveryInput {
	in := collaborationDeliveryInput{note: t.Result, basis: t.Basis}
	if t.DraftID != nil {
		x := completionUUIDPG(*t.DraftID)
		in.draft = &x
	}
	if t.SourceCommentID != nil {
		x := completionUUIDPG(*t.SourceCommentID)
		in.sourceComment = &x
	}
	for _, id := range t.AttachmentIDs {
		in.attachments = append(in.attachments, completionUUIDPG(id))
	}
	return in
}
func completionIntentDigest(ctx context.Context, tx pgx.Tx, target DeliveryCompletionTarget, current *CollaborationSubmission) (string, error) {
	type fact struct {
		ID, Author    string
		Event         int64
		Lifecycle     *string
		Result, Basis *string
		Attachments   []string
		SourceComment string
	}
	var snapshot *fact
	if current != nil {
		snapshot = &fact{ID: util.UUIDToString(current.ID), Author: util.UUIDToString(current.AuthorUserID), Event: current.EventSeq, Lifecycle: current.LifecycleEpoch, Result: current.Result, Basis: current.Basis, SourceComment: util.UUIDToString(current.SourceCommentID), Attachments: []string{}}
		for _, id := range current.AttachmentIDs {
			snapshot.Attachments = append(snapshot.Attachments, util.UUIDToString(id))
		}
		slices.Sort(snapshot.Attachments)
	}
	type attachmentFact struct {
		ID      string
		Version int64
		Digest  string
		Size    int64
		Source  string
	}
	ids := []pgtype.UUID{}
	if current != nil {
		ids = append(ids, current.AttachmentIDs...)
	} else {
		for _, id := range target.AttachmentIDs {
			ids = append(ids, completionUUIDPG(id))
		}
	}
	files := []attachmentFact{}
	if len(ids) > 0 {
		rows, err := db.New(tx).LockCollaborationAttachments(ctx, ids)
		if err != nil {
			return "", err
		}
		if len(rows) != len(ids) {
			return "", completionConflict()
		}
		for _, row := range rows {
			if err = validateCollaborationStoredAttachment(row); err != nil {
				return "", err
			}
			if row.DiscardedAt.Valid {
				return "", completionConflict()
			}
			files = append(files, attachmentFact{util.UUIDToString(row.ID), row.Version, row.ContentSha256.String, row.SizeBytes, util.UUIDToString(row.SourceWorkspaceID)})
		}
		slices.SortFunc(files, func(a, b attachmentFact) int { return strings.Compare(a.ID, b.ID) })
	}
	target.AttachmentIDs = slices.Clone(target.AttachmentIDs)
	slices.Sort(target.AttachmentIDs)
	raw, err := json.Marshal(struct {
		Target     DeliveryCompletionTarget
		Submission *fact
		Files      []attachmentFact
	}{target, snapshot, files})
	if err != nil {
		return "", err
	}
	return completionDigest("collaboration_intent", raw), nil
}
func completionFreezeCollaboration(ctx context.Context, tx pgx.Tx, issuer ArtifactAccess, req DeliveryCompletionAuthorizationRequest, issue db.Issue, p *db.CreateDeliveryCompletionAuthorizationParams) (db.Issue, error) {
	if !completionIsCollaboration(req.Target.Kind) {
		return db.Issue{}, completionUnsupported()
	}
	if _, err := completionCollaborationAccess(ctx, tx, issue, completionUUIDPG(issuer.Credential.Principal.UserID)); err != nil {
		return db.Issue{}, err
	}
	a, submission, digest, err := completionCollaborationIntent(ctx, tx, issue, req.Target, req.Consumer, true)
	if err != nil {
		return db.Issue{}, err
	}
	child, err := completionSourceIssue(ctx, tx, req, issue, true)
	if err != nil {
		return child, err
	}
	p.ConsumerKind = req.Consumer.Kind
	p.ConsumerUserID = completionUUIDPG(req.Consumer.UserID)
	p.CollaborationRevision = completionInt(a.Revision)
	p.LifecycleEpoch = completionInt(a.LifecycleEpoch)
	p.StatusOptionsVersion = completionText(req.Target.StatusOptionsVersion)
	p.DeliveryIntentDigest = completionText(digest)
	if req.Target.Kind == "collaboration_admin" {
		p.AdminOrganizationID = completionUUIDPG(req.Consumer.OrganizationID)
	} else {
		p.ParticipantEpoch = a.ParticipantEpoch
		p.AccountEpoch = a.AccountEpoch
	}
	if submission != nil {
		p.SubmissionID = submission.ID
		p.SubmissionEventSeq = completionInt(submission.EventSeq)
	}
	return child, nil
}
func completionValidateStoredCollaboration(row db.DeliveryCompletionAuthorization, req DeliveryCompletionAuthorizationRequest) error {
	if req.Target.Kind == "ordinary" {
		return nil
	}
	if req.Target.ExpectedCollaborationRevision != strconv.FormatInt(row.CollaborationRevision.Int64, 10) || req.Target.ExpectedLifecycleEpoch != strconv.FormatInt(row.LifecycleEpoch.Int64, 10) || req.Target.StatusOptionsVersion != row.StatusOptionsVersion.String || !row.DeliveryIntentDigest.Valid {
		return completionConflict()
	}
	if req.Target.Kind == "collaboration_admin" {
		if completionUUIDPG(req.Consumer.OrganizationID) != row.AdminOrganizationID {
			return completionConflict()
		}
	} else if req.Target.ExpectedParticipantEpoch != strconv.FormatInt(row.ParticipantEpoch.Int64, 10) || req.Target.ExpectedAccountEpoch != strconv.FormatInt(row.AccountEpoch.Int64, 10) {
		return completionConflict()
	}
	if (req.Target.SubmissionID != nil) != row.SubmissionID.Valid || req.Target.SubmissionID != nil && completionUUIDPG(*req.Target.SubmissionID) != row.SubmissionID {
		return completionConflict()
	}
	return nil
}
func completionRecheckCollaborationIntent(ctx context.Context, tx pgx.Tx, row db.DeliveryCompletionAuthorization, req DeliveryCompletionAuthorizationRequest, issue db.Issue) error {
	_, _, digest, err := completionCollaborationIntent(ctx, tx, issue, req.Target, req.Consumer, true)
	if err != nil {
		return err
	}
	if digest != row.DeliveryIntentDigest.String {
		return completionConflict()
	}
	return nil
}
func completionCollaborationFinished(ctx context.Context, tx pgx.Tx, row db.DeliveryCompletionAuthorization, after db.Issue, result DeliveryWriterResult) error {
	q := db.New(tx)
	c, err := q.LockIssueCollaboration(ctx, after.ID)
	if err != nil {
		return err
	}
	if c.Revision != row.CollaborationRevision.Int64+1 || c.LifecycleEpoch != row.LifecycleEpoch.Int64+1 || result.CollaborationRevision != c.Revision || !result.CollaborationOperationID.Valid {
		return completionConflict()
	}
	op, err := q.GetCollaborationOperation(ctx, db.GetCollaborationOperationParams{ScopeKind: "issue", ScopeID: after.ID, ActorUserID: row.ConsumerUserID, RequestID: util.UUIDToString(row.OperationID)})
	if err != nil {
		return err
	}
	if op.ID != result.CollaborationOperationID {
		return completionConflict()
	}
	if row.TargetKind == "collaboration_admin" {
		if op.Kind != "admin_complete" || !result.AdminOperationID.Valid || result.SubmissionID.Valid {
			return completionConflict()
		}
		admin, err := q.GetAdminOperationForActor(ctx, db.GetAdminOperationForActorParams{OrganizationID: row.AdminOrganizationID, ActorID: row.ConsumerUserID, ID: result.AdminOperationID})
		if err != nil {
			return err
		}
		if admin.TargetID != after.ID || admin.Kind != "collaboration.complete" || admin.IdempotencyKey != row.OperationID {
			return completionConflict()
		}
	} else {
		if op.Kind != "complete" || result.AdminOperationID.Valid || !result.SubmissionID.Valid {
			return completionConflict()
		}
		a, err := completionCollaborationAccess(ctx, tx, after, row.ConsumerUserID)
		if err != nil {
			return err
		}
		summary, err := collaborationDeliverySummary(ctx, q, a)
		if err != nil {
			return err
		}
		latest := summary.Latest
		if latest == nil || latest.ID != result.SubmissionID || latest.Decision == nil || latest.Decision.Kind != "accepted" || latest.Decision.ActorUserID != row.ConsumerUserID {
			return completionConflict()
		}
		req, err := completionStoredRequest(row)
		if err != nil {
			return err
		}
		if row.TargetKind == "collaboration_submission" {
			if result.SubmissionID != row.SubmissionID {
				return completionConflict()
			}
			digest, err := completionIntentDigest(ctx, tx, req.Target, latest)
			if err != nil {
				return err
			}
			if digest != row.DeliveryIntentDigest.String {
				return completionConflict()
			}
		} else {
			got := []string{}
			for _, id := range latest.AttachmentIDs {
				got = append(got, util.UUIDToString(id))
			}
			want := slices.Clone(req.Target.AttachmentIDs)
			slices.Sort(got)
			slices.Sort(want)
			if latest.Result == nil || *latest.Result != req.Target.Result || latest.Basis == nil || *latest.Basis != req.Target.Basis || !slices.Equal(got, want) {
				return completionConflict()
			}
			if (req.Target.SourceCommentID != nil) != latest.SourceCommentID.Valid || req.Target.SourceCommentID != nil && completionUUIDPG(*req.Target.SourceCommentID) != latest.SourceCommentID {
				return completionConflict()
			}
			digest, err := completionIntentDigest(ctx, tx, req.Target, nil)
			if err != nil {
				return err
			}
			if digest != row.DeliveryIntentDigest.String {
				return completionConflict()
			}
		}
	}
	return nil
}
func completionCollaborationTargetFromWorkflow(t *collaborationWorkflowTx, p CollaborationLifecycleParams) DeliveryCompletionTarget {
	out := DeliveryCompletionTarget{Kind: "collaboration_self", IssueID: util.UUIDToString(t.access.Issue.ID), ExpectedIssueRevision: t.access.Issue.Revision, ExpectedStatus: t.access.Issue.Status, TargetStatus: p.TargetStatus, ExpectedCollaborationRevision: strconv.FormatInt(p.ExpectedCollaborationRevision, 10), ExpectedLifecycleEpoch: strconv.FormatInt(p.ExpectedLifecycleEpoch, 10), ExpectedParticipantEpoch: strconv.FormatInt(p.ExpectedParticipantEpoch, 10), ExpectedAccountEpoch: strconv.FormatInt(p.ExpectedAccountEpoch, 10), StatusOptionsVersion: p.StatusOptionsVersion}
	if p.SubmissionID != nil {
		id := util.UUIDToString(*p.SubmissionID)
		out.Kind = "collaboration_submission"
		out.SubmissionID = &id
		out.AcceptanceNote = p.Note
	} else {
		out.Result = p.Note
		out.Basis = p.Basis
		out.AttachmentIDs = []string{}
		for _, id := range p.AttachmentIDs {
			out.AttachmentIDs = append(out.AttachmentIDs, util.UUIDToString(id))
		}
		slices.Sort(out.AttachmentIDs)
		if p.DraftID != nil {
			id := util.UUIDToString(*p.DraftID)
			out.DraftID = &id
		}
		if p.SourceCommentID != nil {
			id := util.UUIDToString(*p.SourceCommentID)
			out.SourceCommentID = &id
		}
	}
	return out
}
func completionTargetEqual(a, b DeliveryCompletionTarget) bool {
	a.AttachmentIDs = slices.Clone(a.AttachmentIDs)
	b.AttachmentIDs = slices.Clone(b.AttachmentIDs)
	slices.Sort(a.AttachmentIDs)
	slices.Sort(b.AttachmentIDs)
	ra, e1 := json.Marshal(a)
	rb, e2 := json.Marshal(b)
	return e1 == nil && e2 == nil && reflect.DeepEqual(ra, rb)
}
func completionWorkflowRefusal(t *collaborationWorkflowTx, err error) (CollaborationWorkflowReceipt, error) {
	var e DeliveryCompletionError
	if errors.As(err, &e) {
		return t.result(e.Code, e.Status), nil
	}
	return CollaborationWorkflowReceipt{}, err
}
func completionAdminTarget(t *collaborationWorkflowTx, p AdminCollaborationLifecycleParams, current *CollaborationSubmission) DeliveryCompletionTarget {
	out := DeliveryCompletionTarget{Kind: "collaboration_admin", IssueID: util.UUIDToString(t.access.Issue.ID), ExpectedIssueRevision: t.access.Issue.Revision, ExpectedStatus: p.ExpectedStatus, TargetStatus: p.TargetStatus, ExpectedCollaborationRevision: fmt.Sprint(p.ExpectedCollaborationRevision), ExpectedLifecycleEpoch: fmt.Sprint(t.access.LifecycleEpoch), StatusOptionsVersion: p.StatusOptionsVersion, AdministrationReason: p.Reason}
	if current != nil {
		id := util.UUIDToString(current.ID)
		out.SubmissionID = &id
	}
	return out
}
