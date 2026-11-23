package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/multica-ai/multica/server/internal/analytics"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/realtime"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// The existing real-auth M1 fixture supplies the ordinary child's actual tar
// and original issuer/consumer. These reads prove preflight and current read
// authority; they do not prove stage-enabled completion or external execution.

func collaborationCompletionReadFixture(test *testing.T, kind string) (*completionCollaborationRouterFixture, string) {
	test.Helper()
	fixture := newCompletionCollaborationRouterFixture(test, kind)
	reader := fixture.token
	if kind == "admin" {
		fixture.fx.Member(test, testWorkspaceID, fixture.admin, "admin")
		reader = fixture.adminToken
	}
	return fixture, reader
}

func collaborationCompletionReadPath(fixture *completionCollaborationRouterFixture, kind, suffix string) string {
	if kind == "admin" {
		return "/api/admin/collaboration/issues/" + fixture.parent + "/delivery-evidence/" + suffix
	}
	return fixture.pathFor("/delivery-evidence/" + suffix)
}

func collaborationCompletionReadRows(test *testing.T, fixture *completionCollaborationRouterFixture) string {
	test.Helper()
	var rows string
	fixture.fx.QueryRow(test, `SELECT jsonb_build_object(
		'source',$3::jsonb,
		'issues',(SELECT jsonb_agg(to_jsonb(i) ORDER BY i.id) FROM issue i WHERE i.id IN ($1,$2)),
		'collaboration',(SELECT to_jsonb(c) FROM issue_collaboration c WHERE c.issue_id=$1),
		'session',(SELECT to_jsonb(s) FROM collaboration_session s WHERE s.issue_id=$1),
		'participants',(SELECT COALESCE(jsonb_agg(to_jsonb(p) ORDER BY p.user_id),'[]'::jsonb) FROM issue_participant p WHERE p.issue_id=$1),
		'handoffs',(SELECT COALESCE(jsonb_agg(to_jsonb(h) ORDER BY h.id),'[]'::jsonb) FROM issue_handoff h WHERE h.issue_id=$1),
		'events',(SELECT COALESCE(jsonb_agg(to_jsonb(e) ORDER BY e.id),'[]'::jsonb) FROM collaboration_event e WHERE e.issue_id=$1),
		'operations',(SELECT COALESCE(jsonb_agg(to_jsonb(o) ORDER BY o.id),'[]'::jsonb) FROM collaboration_operation o WHERE o.scope_id=$1),
		'notifications',(SELECT COALESCE(jsonb_agg(to_jsonb(n) ORDER BY n.id),'[]'::jsonb) FROM collaboration_notification n WHERE n.issue_id=$1),
		'grants',(SELECT COALESCE(jsonb_agg(to_jsonb(g) ORDER BY g.id),'[]'::jsonb) FROM collaboration_grant g WHERE g.issue_id=$1),
		'commands',(SELECT COALESCE(jsonb_agg(to_jsonb(c) ORDER BY c.id),'[]'::jsonb) FROM collaboration_command c JOIN collaboration_session s ON s.id=c.session_id WHERE s.issue_id=$1),
		'runs',(SELECT COALESCE(jsonb_agg(to_jsonb(r) ORDER BY r.id),'[]'::jsonb) FROM collaboration_run r JOIN collaboration_session s ON s.id=r.session_id WHERE s.issue_id=$1),
		'run_events',(SELECT COALESCE(jsonb_agg(to_jsonb(e) ORDER BY e.id),'[]'::jsonb) FROM collaboration_run_event e JOIN collaboration_run r ON r.id=e.run_id JOIN collaboration_session s ON s.id=r.session_id WHERE s.issue_id=$1),
		'authorizations',(SELECT COALESCE(jsonb_agg(to_jsonb(a) ORDER BY a.id),'[]'::jsonb) FROM delivery_completion_authorization a WHERE a.issue_id IN ($1,$2)),
		'terminals',(SELECT COALESCE(jsonb_agg(to_jsonb(t) ORDER BY t.authorization_id),'[]'::jsonb) FROM delivery_completion_terminal t JOIN delivery_completion_authorization a ON a.id=t.authorization_id WHERE a.issue_id IN ($1,$2)),
		'attachments',(SELECT COALESCE(jsonb_agg(to_jsonb(a) ORDER BY a.id),'[]'::jsonb) FROM collaboration_attachment a WHERE a.issue_id=$1 OR a.intended_issue_id=$1),
		'attachment_references',(SELECT COALESCE(jsonb_agg(to_jsonb(r) ORDER BY r.attachment_id,r.reference_kind,r.reference_id),'[]'::jsonb) FROM collaboration_attachment_reference r JOIN collaboration_attachment a ON a.id=r.attachment_id WHERE a.issue_id=$1 OR a.intended_issue_id=$1),
		'comments',(SELECT COALESCE(jsonb_agg(to_jsonb(c) ORDER BY c.id),'[]'::jsonb) FROM comment c WHERE c.issue_id=$1),
		'root_tasks',(SELECT COALESCE(jsonb_agg(to_jsonb(t) ORDER BY t.id),'[]'::jsonb) FROM agent_task_queue t WHERE t.issue_id=$1),
		'stage_task',(SELECT to_jsonb(s) FROM collaboration_stage_task s WHERE s.issue_id=$1),
		'stage_roles',(SELECT COALESCE(jsonb_agg(to_jsonb(r) ORDER BY r.revision),'[]'::jsonb) FROM collaboration_stage_role r WHERE r.issue_id=$1),
		'stage_candidates',(SELECT COALESCE(jsonb_agg(to_jsonb(c) ORDER BY c.candidate_id),'[]'::jsonb) FROM collaboration_stage_candidate c WHERE c.issue_id=$1),
		'stage_decisions',(SELECT COALESCE(jsonb_agg(to_jsonb(d) ORDER BY d.decision_id),'[]'::jsonb) FROM collaboration_stage_decision d WHERE d.issue_id=$1),
		'stage_revocations',(SELECT COALESCE(jsonb_agg(to_jsonb(r) ORDER BY r.revocation_id),'[]'::jsonb) FROM collaboration_stage_revocation r WHERE r.issue_id=$1),
		'stage_children',(SELECT COALESCE(jsonb_agg(to_jsonb(c) ORDER BY c.child_issue_id),'[]'::jsonb) FROM collaboration_stage_child c WHERE c.root_issue_id=$1),
		'stage_submissions',(SELECT COALESCE(jsonb_agg(to_jsonb(s) ORDER BY s.submission_id),'[]'::jsonb) FROM collaboration_stage_submission s WHERE s.issue_id=$1),
		'stage_executions',(SELECT COALESCE(jsonb_agg(to_jsonb(i) ORDER BY i.intent_id),'[]'::jsonb) FROM collaboration_stage_execution_intent i WHERE i.issue_id=$1),
		'stage_observations',(SELECT COALESCE(jsonb_agg(to_jsonb(o) ORDER BY o.observation_id),'[]'::jsonb) FROM collaboration_stage_execution_observation o WHERE o.issue_id=$1),
		'stage_rollback',(SELECT COALESCE(jsonb_agg(to_jsonb(o) ORDER BY o.observation_id),'[]'::jsonb) FROM collaboration_stage_rollback_observation o WHERE o.issue_id=$1),
		'stage_policy',(SELECT COALESCE(jsonb_agg(to_jsonb(p) ORDER BY p.revision),'[]'::jsonb) FROM project_stage_policy p WHERE p.project_id=$4),
		'admin_operations',(SELECT COALESCE(jsonb_agg(to_jsonb(o) ORDER BY o.id),'[]'::jsonb) FROM admin_operation o WHERE o.target_id=$1),
		'admin_audit',(SELECT COALESCE(jsonb_agg(to_jsonb(a) ORDER BY a.id),'[]'::jsonb) FROM admin_audit_event a WHERE a.target_id=$1)
	)::text`, fixture.parent, fixture.request.ResourceID, fixture.completionHintSnapshot(test), fixture.project).Scan(&rows)
	return rows
}

func TestCollaborationStageCompletionPreviewRouterReadOnlyAndConsumedBinding(test *testing.T) {
	for _, kind := range []string{"self", "submission", "admin"} {
		test.Run(kind, func(test *testing.T) {
			fixture, reader := collaborationCompletionReadFixture(test, kind)
			childRequest := completionAuthorizationRouterBody(test, fixture.deliveryPolicyRouterFixture, 2, map[string]any{"kind": "human", "user_id": fixture.user})
			var childGrant service.DeliveryCompletionAuthorizationView
			fixture.call(test, fixture.token, "POST", completionAuthorizationRouterPath, childRequest).Want(201).JSON(&childGrant)
			fixture.call(test, fixture.token, "PUT", "/api/issues/"+fixture.request.ResourceID, map[string]any{
				"status": "done", "completion": service.DeliveryCompletionBinding{AuthorizationID: childGrant.AuthorizationID, OperationID: childRequest["operation_id"].(string), ExpectedIssueRevision: fixture.body["expected_issue_revision"].(int64)},
			}).Want(200)
			var childRevision int64
			fixture.fx.QueryRow(test, `SELECT revision FROM issue WHERE id=$1`, fixture.request.ResourceID).Scan(&childRevision)
			fixture.body["expected_issue_revision"] = childRevision
			request, completionBody := fixture.authorization(test, kind)
			if kind == "self" {
				draft, attachment := fixture.upload(test)
				request.Target.DraftID, request.Target.AttachmentIDs = &draft, []string{attachment}
				completionBody["draft_id"], completionBody["attachment_ids"] = draft, []string{attachment}
			}
			before, archive := collaborationCompletionReadRows(test, fixture), fixture.archive(test)
			var candidates service.CollaborationDeliveryCandidates
			candidateResponse := fixture.call(test, reader, "GET", collaborationCompletionReadPath(fixture, kind, "candidates")+"?source_issue_id="+fixture.request.ResourceID, nil, "X-Workspace-ID", uuid.NewString(), "X-Workspace-Slug", "unrelated-ambient-workspace").Want(200).JSON(&candidates)
			if candidateResponse.Header().Get("Cache-Control") != "no-store" || candidates.ProtocolVersion != 1 || candidates.IssueID != fixture.parent || candidates.IssueRevision != request.Target.ExpectedIssueRevision || candidates.SourceIssueID != fixture.request.ResourceID || candidates.SourceIssueRevision != childRevision || candidates.ProjectID == nil || *candidates.ProjectID != fixture.project || len(candidates.Items) != 1 || candidates.Items[0].ApprovalID != fixture.approval.Approval.ID || !candidates.Items[0].Selectable || candidates.HasMore || candidates.NextCursor != nil {
				test.Fatal("candidate route did not retain current numeric root/done-child identities and source authority")
			}
			if kind == "admin" {
				if candidates.TargetContext == nil || candidates.TargetContext.ExpectedLifecycleEpoch != request.Target.ExpectedLifecycleEpoch || candidates.TargetContext.SubmissionID != nil {
					test.Fatal("R38 candidates lost their exact disabled-creator lifecycle context")
				}
			} else if _, present := candidateResponse.Map()["target_context"]; present {
				test.Fatal("ordinary collaboration candidate route fabricated R38 context")
			}
			previewBody := service.CollaborationCompletionPreviewRequest{SourceIssueID: fixture.request.ResourceID, ApprovalID: fixture.approval.Approval.ID, Target: request.Target}
			var preview service.CollaborationCompletionPreview
			previewResponse := fixture.call(test, reader, "POST", collaborationCompletionReadPath(fixture, kind, "completion-preview"), previewBody, "X-Workspace-ID", uuid.NewString(), "X-Workspace-Slug", "unrelated-ambient-workspace").Want(200).JSON(&preview)
			if previewResponse.Header().Get("Cache-Control") != "no-store" || preview.ProtocolVersion != 1 || preview.IssueID != fixture.parent || preview.IssueRevision != request.Target.ExpectedIssueRevision || preview.SourceIssueID != fixture.request.ResourceID || preview.SourceIssueRevision != childRevision || preview.Reason != "authorization_available" || preview.Observation == nil || preview.Observation.Verdict != "pass" || preview.AuthorizationBinding == nil || preview.MaxExpiresAt == nil || preview.StageBlockers == nil || len(preview.StageBlockers) != 0 || preview.StageBlockersTruncated {
				test.Fatal("root preview did not observe the actual done child's archive without completing the root")
			}
			wire := previewResponse.Map()
			if wire["enforced"] != false || wire["usable_for_completion"] != false || wire["skip_supported"] != false || wire["stage_blockers_truncated"] != false {
				test.Fatal("preview omitted its explicit non-permission fields")
			}
			binding := preview.AuthorizationBinding
			if !reflect.DeepEqual(binding.Target, request.Target) || binding.Evidence.IssueID != fixture.request.ResourceID || binding.Evidence.ExpectedIssueRevision != childRevision || binding.Evidence.ApprovalID != fixture.approval.Approval.ID || binding.Evidence.ExpectedArtifactDigest != request.Evidence.ExpectedArtifactDigest || binding.Project != request.Project || binding.Consumer != request.Consumer {
				test.Fatal("preview substituted the source issuer, child target or a different completion intent")
			}
			fixture.call(test, reader, "POST", collaborationCompletionReadPath(fixture, kind, "completion-preview"), previewBody).Want(200)
			if collaborationCompletionReadRows(test, fixture) != before || !bytes.Equal(archive, fixture.archive(test)) {
				test.Fatal("candidate/preview reads changed authoritative rows, history, grants, attachment links or archive bytes")
			}
			test.Logf("F06_READONLY kind=%s before_sha256=%x after_sha256=%x root_grants=%d source_grants=%d root_events=%d root_operations=%d root_attachments=%d", kind, sha256.Sum256([]byte(before)), sha256.Sum256([]byte(collaborationCompletionReadRows(test, fixture))), fixture.fx.Count(test, `SELECT count(*) FROM delivery_completion_authorization WHERE issue_id=$1`, fixture.parent), fixture.fx.Count(test, `SELECT count(*) FROM delivery_completion_authorization WHERE issue_id=$1`, fixture.request.ResourceID), fixture.fx.Count(test, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1`, fixture.parent), fixture.fx.Count(test, `SELECT count(*) FROM collaboration_operation WHERE scope_id=$1`, fixture.parent), fixture.fx.Count(test, `SELECT count(*) FROM collaboration_attachment WHERE intended_issue_id=$1 OR issue_id=$1`, fixture.parent))
			request.Target, request.Project, request.Evidence, request.Consumer = binding.Target, binding.Project, binding.Evidence, binding.Consumer
			grant := fixture.grant(test, request)
			fixture.complete(test, request, completionBody, &grant).Want(200)
			if fixture.fx.Count(test, `SELECT count(*) FROM delivery_completion_terminal WHERE authorization_id=$1 AND kind='consumed' AND actor_user_id=$2 AND operation_id=$3 AND issue_revision_before=$4 AND issue_revision_after>issue_revision_before`, grant.AuthorizationID, request.Consumer.UserID, request.OperationID, request.Target.ExpectedIssueRevision) != 1 {
				test.Fatal("preview-derived binding was not consumed by its real exact current consumer")
			}
			if kind != "admin" && (fixture.user == fixture.initiator || fixture.fx.Count(test, `SELECT count(*) FROM member WHERE workspace_id=$1 AND user_id=$2`, testWorkspaceID, fixture.initiator) != 0) {
				test.Fatal("normal completion did not preserve the outside initiator/source issuer distinction")
			}
			if kind == "admin" && fixture.fx.Count(test, `SELECT count(*) FROM collaboration_event WHERE issue_id=$1 AND payload->'delivery_decision'->>'kind'='accepted'`, fixture.parent) != 0 {
				test.Fatal("R38 manufactured initiator acceptance")
			}
			afterCompletion := collaborationCompletionReadRows(test, fixture)
			consumerToken := fixture.initiatorToken
			if kind == "admin" {
				consumerToken = fixture.adminToken
			}
			var recovered service.DeliveryCompletionAuthorizationView
			recovery := fixture.call(test, consumerToken, "GET", completionAuthorizationRouterPath+"/"+grant.AuthorizationID, nil).Want(200).JSON(&recovered)
			if recovery.Header().Get("Cache-Control") != "no-store" || recovered.State != "consumed" || recovered.CompletionReceipt == nil || recovered.OperationID != request.OperationID {
				test.Fatal("actual consumer could not recover its original minimal consumed receipt")
			}
			for _, private := range []string{fixture.evaluationKey, fixture.request.ResourceID, request.Evidence.ExpectedArtifactDigest, "candidate.txt", "observation", "storage_key"} {
				if strings.Contains(recovery.Text(), private) {
					test.Fatal("minimal completion recovery leaked protected source evidence")
				}
			}
			if collaborationCompletionReadRows(test, fixture) != afterCompletion || !bytes.Equal(archive, fixture.archive(test)) {
				test.Fatal("consumer receipt read changed retained completion or artifact facts")
			}
		})
	}
}

func TestCollaborationStageCompletionPreviewRouterCurrentConsumerObservations(test *testing.T) {
	for _, kind := range []string{"self", "submission"} {
		test.Run(kind, func(test *testing.T) {
			fixture := newCompletionCollaborationRouterFixture(test, kind)
			fixture.fx.InsertNoID(test, "issue_participant", testutil.Cols{"issue_id": fixture.parent, "user_id": fixture.user}, "issue_id=$1 AND user_id=$2", fixture.parent, fixture.user)
			fixture.fx.InsertNoID(test, "collaboration_account_fence", testutil.Cols{"user_id": fixture.user}, "user_id=$1", fixture.user)
			fixture.fx.Exec(test, `UPDATE issue_participant SET epoch=7 WHERE issue_id=$1 AND user_id=$2`, fixture.parent, fixture.initiator)
			fixture.fx.Exec(test, `UPDATE collaboration_account_fence SET epoch=9 WHERE user_id=$1`, fixture.initiator)
			request, _ := fixture.authorization(test, kind)
			var issuer struct {
				Write struct {
					Participant string `json:"participant_epoch"`
					Account     string `json:"account_epoch"`
				} `json:"write_context"`
			}
			fixture.call(test, fixture.token, "GET", fixture.pathFor(""), nil).Want(200).JSON(&issuer)
			if request.Target.ExpectedParticipantEpoch != "7" || request.Target.ExpectedAccountEpoch != "9" || issuer.Write.Participant == "7" || issuer.Write.Account == "9" {
				test.Fatal("fixture did not establish distinct current issuer and initiator observations")
			}
			before := collaborationCompletionReadRows(test, fixture)
			body := service.CollaborationCompletionPreviewRequest{SourceIssueID: fixture.request.ResourceID, ApprovalID: fixture.approval.Approval.ID, Target: request.Target}
			var preview service.CollaborationCompletionPreview
			fixture.call(test, fixture.token, "POST", collaborationCompletionReadPath(fixture, kind, "completion-preview"), body).Want(200).JSON(&preview)
			if preview.AuthorizationBinding == nil || preview.AuthorizationBinding.Consumer.UserID != fixture.initiator || preview.AuthorizationBinding.Target.ExpectedParticipantEpoch != "7" || preview.AuthorizationBinding.Target.ExpectedAccountEpoch != "9" {
				test.Fatal("source reader's observations replaced the exact current initiator consumer")
			}
			expires := time.Now().Add(time.Minute).Truncate(time.Second)
			finiteIssuer := controlledArtifactJWT(test, fixture.user, 0, expires)
			fixture.call(test, finiteIssuer, "POST", collaborationCompletionReadPath(fixture, kind, "completion-preview"), body).Want(200).JSON(&preview)
			if preview.MaxExpiresAt == nil || preview.MaxExpiresAt.After(expires) || !preview.MaxExpiresAt.After(preview.CheckedAt) || preview.AuthorizationBinding == nil || preview.AuthorizationBinding.Consumer.UserID != fixture.initiator {
				test.Fatal("preview exceeded its actual current issuer credential horizon or replaced the outside consumer")
			}
			body.Target.ExpectedParticipantEpoch, body.Target.ExpectedAccountEpoch = issuer.Write.Participant, issuer.Write.Account
			fixture.call(test, fixture.token, "POST", collaborationCompletionReadPath(fixture, kind, "completion-preview"), body).Want(409)
			if collaborationCompletionReadRows(test, fixture) != before {
				test.Fatal("consumer-observation preview or refusal wrote an operation or business fact")
			}
		})
	}
}

func TestCollaborationStageCompletionPreviewRouterStageIncomplete(test *testing.T) {
	for _, kind := range []string{"self", "submission", "admin"} {
		test.Run(kind, func(test *testing.T) {
			fixture, reader := collaborationCompletionReadFixture(test, kind)
			request, _ := fixture.authorization(test, kind)
			fixture.fx.InsertNoID(test, "project_stage_policy", testutil.Cols{"workspace_id": testWorkspaceID, "project_id": fixture.project, "revision": 1, "contract": collaboration.StageContract, "enabled": true, "reason": "Isolated incomplete-stage refusal fixture, not public activation", "created_by": fixture.user, "request_id": uuid.NewString(), "request_digest": strings.Repeat("b", 64)}, "workspace_id=$1 AND project_id=$2", testWorkspaceID, fixture.project)
			policyRevision := int64(2)
			for _, mode := range []string{"enforce", "observe", "off"} {
				test.Run(mode, func(test *testing.T) {
					if mode != "enforce" {
						fixture.call(test, fixture.token, "PUT", fixture.policyPath(), fixture.policyBody(policyRevision, mode)).Want(200)
						policyRevision++
					}
					before, archive := collaborationCompletionReadRows(test, fixture), fixture.archive(test)
					var preview service.CollaborationCompletionPreview
					response := fixture.call(test, reader, "POST", collaborationCompletionReadPath(fixture, kind, "completion-preview"), service.CollaborationCompletionPreviewRequest{SourceIssueID: fixture.request.ResourceID, ApprovalID: fixture.approval.Approval.ID, Target: request.Target}).Want(200).JSON(&preview)
					if response.Header().Get("Cache-Control") != "no-store" || preview.Reason != "stage_incomplete" || preview.PolicyMode != mode || preview.AuthorizationBinding != nil || preview.MaxExpiresAt != nil || preview.Enforced || preview.UsableForCompletion || preview.SkipSupported || len(preview.StageBlockers) != 1 || preview.StageBlockers[0].Code != "stage_roles_unconfigured" || preview.StageBlockersTruncated {
						test.Fatal("incomplete stage became a passing or authorizing preview under this M1 mode")
					}
					if collaborationCompletionReadRows(test, fixture) != before || !bytes.Equal(archive, fixture.archive(test)) {
						test.Fatal("incomplete-stage preview changed task/role rows, evidence, grants or history")
					}
				})
			}
		})
	}
}

func TestCollaborationStageCompletionPreviewRouterAuthentication(test *testing.T) {
	for _, kind := range []string{"self", "admin"} {
		test.Run(kind, func(test *testing.T) {
			fixture, reader := collaborationCompletionReadFixture(test, kind)
			request, _ := fixture.authorization(test, kind)
			body := service.CollaborationCompletionPreviewRequest{SourceIssueID: fixture.request.ResourceID, ApprovalID: fixture.approval.Approval.ID, Target: request.Target}
			for _, suffix := range []string{"candidates", "completion-preview"} {
				test.Run(suffix, func(test *testing.T) {
					method, endpoint := "POST", collaborationCompletionReadPath(fixture, kind, suffix)
					var payload any = body
					if suffix == "candidates" {
						method, payload = "GET", nil
						endpoint += "?source_issue_id=" + fixture.request.ResourceID
					}
					before := collaborationCompletionReadRows(test, fixture)
					spoofed := testutil.WithHeaders(testutil.JSONRequest(method, endpoint, payload), "X-User-ID", request.Consumer.UserID, "X-Actor-Source", "member", "X-Workspace-ID", testWorkspaceID)
					testutil.Call(test, fixture.router.ServeHTTP, spoofed).Want(http.StatusUnauthorized)
					fixture.call(test, fixture.taskToken, method, endpoint, payload, "X-User-ID", request.Consumer.UserID, "X-Actor-Source", "member").Want(http.StatusForbidden)
					fixture.call(test, reader, method, endpoint, payload, "X-Agent-ID", fixture.request.AgentID).Want(http.StatusForbidden)
					if kind == "self" {
						denied := fixture.call(test, fixture.initiatorToken, method, endpoint, payload).Want(403)
						if denied.Header().Get("Cache-Control") != "no-store" || !reflect.DeepEqual(denied.Map(), map[string]any{"code": "delivery_completion_denied", "error": "delivery_completion_denied", "message": "delivery_completion_denied"}) {
							test.Fatal("outside initiator denial revealed source metadata or a candidate count")
						}
					}
					if collaborationCompletionReadRows(test, fixture) != before {
						test.Fatal("failed route authentication wrote business rows or consumed a grant")
					}
				})
			}
		})
	}
}

func TestCollaborationStageCompletionPreviewRouterStrictTransport(test *testing.T) {
	for _, kind := range []string{"self", "admin"} {
		test.Run(kind, func(test *testing.T) {
			fixture, reader := collaborationCompletionReadFixture(test, kind)
			request, _ := fixture.authorization(test, kind)
			body := service.CollaborationCompletionPreviewRequest{SourceIssueID: fixture.request.ResourceID, ApprovalID: fixture.approval.Approval.ID, Target: request.Target}
			raw, err := json.Marshal(body)
			if err != nil {
				test.Fatal(err)
			}
			encoded := string(raw)
			test.Run("body_exactly_64_kib_is_a_read", func(test *testing.T) {
				before := collaborationCompletionReadRows(test, fixture)
				var preview service.CollaborationCompletionPreview
				response := fixture.call(test, reader, "POST", collaborationCompletionReadPath(fixture, kind, "completion-preview"), encoded+strings.Repeat(" ", 65536-len(raw))).Want(200).JSON(&preview)
				if response.Header().Get("Cache-Control") != "no-store" || preview.AuthorizationBinding == nil || preview.UsableForCompletion || collaborationCompletionReadRows(test, fixture) != before {
					test.Fatal("maximum-size valid preview changed state or implied completion permission")
				}
			})
			cases := []struct {
				name, raw string
			}{
				{"unknown_shape_array", "[" + encoded + "]"},
				{"unknown_shape_future_field", encoded[:len(encoded)-1] + `,"consumer":{"kind":"human"}}`},
				{"top_level_case_alias", strings.Replace(encoded, `"source_issue_id"`, `"Source_issue_id"`, 1)},
				{"top_level_duplicate", encoded[:len(encoded)-1] + `,"source_issue_id":"` + fixture.request.ResourceID + `"}`},
				{"nested_case_alias", strings.Replace(encoded, `"kind"`, `"Kind"`, 1)},
				{"nested_duplicate", strings.Replace(encoded, `"kind":`, `"kind":"`+request.Target.Kind+`","kind":`, 1)},
				{"numeric_consumer_epoch", strings.Replace(encoded, `"expected_lifecycle_epoch":"`+request.Target.ExpectedLifecycleEpoch+`"`, `"expected_lifecycle_epoch":1`, 1)},
				{"null_required_source", strings.Replace(encoded, `"source_issue_id":"`+fixture.request.ResourceID+`"`, `"source_issue_id":null`, 1)},
				{"trailing_json", encoded + `{}`},
				{"body_over_64_kib", encoded + strings.Repeat(" ", 65536)},
			}
			ordinary := body
			ordinary.Target.Kind = "ordinary"
			ordinaryRaw, err := json.Marshal(ordinary)
			if err != nil {
				test.Fatal(err)
			}
			cases = append(cases, struct{ name, raw string }{"ordinary_target_on_collaboration_route", string(ordinaryRaw)})
			otherRoute := body
			if kind == "self" {
				otherRoute.Target = service.DeliveryCompletionTarget{Kind: "collaboration_admin", IssueID: request.Target.IssueID, ExpectedIssueRevision: request.Target.ExpectedIssueRevision, ExpectedStatus: request.Target.ExpectedStatus, TargetStatus: request.Target.TargetStatus, ExpectedCollaborationRevision: request.Target.ExpectedCollaborationRevision, ExpectedLifecycleEpoch: request.Target.ExpectedLifecycleEpoch, StatusOptionsVersion: request.Target.StatusOptionsVersion, AdministrationReason: "Explicit administrative variant on the normal route"}
			} else {
				otherRoute.Target = service.DeliveryCompletionTarget{Kind: "collaboration_self", IssueID: request.Target.IssueID, ExpectedIssueRevision: request.Target.ExpectedIssueRevision, ExpectedStatus: request.Target.ExpectedStatus, TargetStatus: request.Target.TargetStatus, ExpectedCollaborationRevision: request.Target.ExpectedCollaborationRevision, ExpectedLifecycleEpoch: request.Target.ExpectedLifecycleEpoch, ExpectedParticipantEpoch: "1", ExpectedAccountEpoch: "1", StatusOptionsVersion: request.Target.StatusOptionsVersion, Result: "Self-result on the administrative route", Basis: "Explicit variant mismatch", AttachmentIDs: []string{}}
			}
			if otherRoute.Target.Validate() != nil {
				test.Fatal("cross-route target fixture was not a valid closed M1 variant")
			}
			otherRaw, err := json.Marshal(otherRoute)
			if err != nil {
				test.Fatal(err)
			}
			cases = append(cases, struct{ name, raw string }{"valid_target_for_the_other_route", string(otherRaw)})
			mismatch := body
			mismatch.Target.IssueID = uuid.NewString()
			mismatchRaw, err := json.Marshal(mismatch)
			if err != nil {
				test.Fatal(err)
			}
			cases = append(cases, struct{ name, raw string }{"target_path_identity_mismatch", string(mismatchRaw)})
			for _, scenario := range cases {
				test.Run(scenario.name, func(test *testing.T) {
					before := collaborationCompletionReadRows(test, fixture)
					response := fixture.call(test, reader, "POST", collaborationCompletionReadPath(fixture, kind, "completion-preview"), scenario.raw).Want(400)
					if response.Header().Get("Cache-Control") != "no-store" || response.Map()["code"] != "invalid_delivery_completion_request" || collaborationCompletionReadRows(test, fixture) != before {
						test.Fatal("strict preview refusal changed rows or used an unclassified wire envelope")
					}
				})
			}
			for _, scenario := range []struct{ name, query string }{
				{"missing_source", "limit=1"},
				{"source_case_alias", "Source_issue_id=" + fixture.request.ResourceID},
				{"duplicate_source", "source_issue_id=" + fixture.request.ResourceID + "&source_issue_id=" + fixture.request.ResourceID},
				{"unknown_query", "source_issue_id=" + fixture.request.ResourceID + "&consumer=" + fixture.initiator},
				{"limit_zero", "source_issue_id=" + fixture.request.ResourceID + "&limit=0"},
				{"limit_above_50", "source_issue_id=" + fixture.request.ResourceID + "&limit=51"},
				{"limit_noncanonical", "source_issue_id=" + fixture.request.ResourceID + "&limit=01"},
				{"duplicate_limit", "source_issue_id=" + fixture.request.ResourceID + "&limit=1&limit=2"},
			} {
				test.Run(scenario.name, func(test *testing.T) {
					before := collaborationCompletionReadRows(test, fixture)
					fixture.call(test, reader, "GET", collaborationCompletionReadPath(fixture, kind, "candidates")+"?"+scenario.query, nil).Want(400)
					if collaborationCompletionReadRows(test, fixture) != before {
						test.Fatal("invalid candidate query changed authoritative rows")
					}
				})
			}
			before := collaborationCompletionReadRows(test, fixture)
			fixture.call(test, reader, "POST", collaborationCompletionReadPath(fixture, kind, "completion-preview")+"?limit=1", body).Want(400)
			if collaborationCompletionReadRows(test, fixture) != before {
				test.Fatal("preview accepted a query override or wrote on refusal")
			}
		})
	}
}

func TestCollaborationStageCompletionPreviewRouterCurrentSourceAndVersions(test *testing.T) {
	for _, change := range []string{"source_revoked", "consumer_revoked", "root_revision_changed", "source_detached", "stale_source_head"} {
		test.Run(change, func(test *testing.T) {
			fixture := newCompletionCollaborationRouterFixture(test, "self")
			request, _ := fixture.authorization(test, "self")
			body := service.CollaborationCompletionPreviewRequest{SourceIssueID: fixture.request.ResourceID, ApprovalID: fixture.approval.Approval.ID, Target: request.Target}
			previewPath := collaborationCompletionReadPath(fixture, "self", "completion-preview")
			fixture.call(test, fixture.token, "POST", previewPath, body).Want(200)
			want := 409
			switch change {
			case "source_revoked":
				fixture.fx.Exec(test, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, testWorkspaceID, fixture.user)
				want = 404
			case "consumer_revoked":
				fixture.fx.Exec(test, `UPDATE issue_participant SET revoked_at=clock_timestamp(),epoch=epoch+1 WHERE issue_id=$1 AND user_id=$2`, fixture.parent, fixture.initiator)
				want = 404
			case "root_revision_changed":
				fixture.call(test, fixture.initiatorToken, "PATCH", fixture.pathFor(""), map[string]any{"title": "Explicit current root revision", "expected_collaboration_revision": request.Target.ExpectedCollaborationRevision, "expected_lifecycle_epoch": request.Target.ExpectedLifecycleEpoch, "expected_participant_epoch": request.Target.ExpectedParticipantEpoch, "expected_account_epoch": request.Target.ExpectedAccountEpoch}).Want(200)
			case "source_detached":
				fixture.call(test, fixture.token, "PUT", "/api/issues/"+fixture.request.ResourceID, map[string]any{"parent_issue_id": nil}).Want(200)
				want = 404
			case "stale_source_head":
				fixture.fx.Exec(test, `UPDATE github_pull_request SET head_sha=$2 WHERE id=$1`, fixture.pr, strings.Repeat("c", 40))
				want = 200
			}
			before, archive := collaborationCompletionReadRows(test, fixture), fixture.archive(test)
			response := fixture.call(test, fixture.token, "POST", previewPath, body).Want(want)
			if want == 404 && !reflect.DeepEqual(response.Map(), map[string]any{"code": "not_found", "error": "not_found"}) {
				test.Fatal("current source denial leaked an evidence or count envelope")
			}
			if change == "stale_source_head" {
				var preview service.CollaborationCompletionPreview
				response.JSON(&preview)
				if preview.AuthorizationBinding != nil || preview.MaxExpiresAt != nil || preview.Observation == nil || preview.Observation.Verdict == "pass" || preview.UsableForCompletion {
					test.Fatal("stale selected artifact/PR head became a passing authorization preview")
				}
			}
			if change == "source_revoked" || change == "source_detached" {
				fixture.call(test, fixture.token, "GET", collaborationCompletionReadPath(fixture, "self", "candidates")+"?source_issue_id="+fixture.request.ResourceID, nil).Want(404)
			}
			if collaborationCompletionReadRows(test, fixture) != before || !bytes.Equal(archive, fixture.archive(test)) {
				test.Fatal("stale/current-authority read changed rows or source archive bytes")
			}
		})
	}
}

func TestCollaborationStageCompletionPreviewRouterPaginationAndOldRoute(test *testing.T) {
	fixture := newCompletionCollaborationRouterFixture(test, "self")
	archive := fixture.archive(test)
	readArchive := func() []byte {
		data, err := os.ReadFile(filepath.Join(fixture.output, fixture.approval.ExecutionID, "artifact.tar"))
		if err != nil {
			test.Fatal(err)
		}
		return data
	}
	additional := fixture.create(test, fixture.taskToken)
	approvalPath := "/api/agent-approvals/" + additional.Approval.ID
	fixture.call(test, fixture.token, "POST", approvalPath+"/decision", map[string]string{"decision": "approve", "expected_spec_digest": additional.SpecDigest}).Want(200)
	fixture.call(test, fixture.taskToken, "POST", approvalPath+"/artifact-package/execution", map[string]string{"expected_spec_digest": additional.SpecDigest}).Want(200).JSON(&additional)
	if additional.Status != "succeeded" || additional.ReceiptDigest == "" || additional.Approval.ID == fixture.approval.Approval.ID {
		test.Fatal("pagination fixture did not create a second actual retained package")
	}
	additionalArchive, err := os.ReadFile(filepath.Join(fixture.output, additional.ExecutionID, "artifact.tar"))
	if err != nil {
		test.Fatal(err)
	}
	var additionalDigest string
	fixture.fx.QueryRow(test, `SELECT output_artifact_digest FROM agent_artifact_execution WHERE execution_id=$1`, additional.ExecutionID).Scan(&additionalDigest)
	if fmt.Sprintf("%x", sha256.Sum256(additionalArchive)) != additionalDigest {
		test.Fatal("second actual package archive did not match its retained digest")
	}
	endpoint := collaborationCompletionReadPath(fixture, "self", "candidates") + "?source_issue_id=" + fixture.request.ResourceID
	before := collaborationCompletionReadRows(test, fixture)
	var first, second, maximum service.CollaborationDeliveryCandidates
	fixture.call(test, fixture.token, "GET", endpoint+"&limit=1", nil).Want(200).JSON(&first)
	if len(first.Items) != 1 || !first.HasMore || first.NextCursor == nil || first.Items[0].ApprovalID != additional.Approval.ID {
		test.Fatal("first visible candidate page did not expose one exact package and a progressing cursor")
	}
	fixture.call(test, fixture.token, "GET", endpoint+"&limit=1&cursor="+url.QueryEscape(*first.NextCursor), nil).Want(200).JSON(&second)
	fixture.call(test, fixture.token, "GET", endpoint+"&limit=50", nil).Want(200).JSON(&maximum)
	if len(second.Items) != 1 || second.Items[0].ApprovalID != fixture.approval.Approval.ID || second.HasMore || second.NextCursor != nil || len(maximum.Items) != 2 || maximum.HasMore {
		test.Fatal("candidate pagination repeated/omitted a retained package or inferred a hidden total")
	}
	if collaborationCompletionReadRows(test, fixture) != before || !bytes.Equal(archive, readArchive()) {
		test.Fatal("candidate pages mutated evidence, grants or authoritative root/source rows")
	}
	decoded, err := base64.RawURLEncoding.DecodeString(*first.NextCursor)
	if err != nil {
		test.Fatal(err)
	}
	for _, malformed := range []struct{ name, raw string }{
		{"unknown_field", string(decoded[:len(decoded)-1]) + `,"consumer":"untrusted"}`},
		{"duplicate_viewer", string(decoded[:len(decoded)-1]) + `,"viewer":"` + fixture.user + `"}`},
		{"case_alias", strings.Replace(string(decoded), `"viewer"`, `"Viewer"`, 1)},
		{"trailing_json", string(decoded) + `{}`},
	} {
		test.Run("cursor_strict_"+malformed.name, func(test *testing.T) {
			response := fixture.call(test, fixture.token, "GET", endpoint+"&limit=1&cursor="+base64.RawURLEncoding.EncodeToString([]byte(malformed.raw)), nil).Want(400)
			if response.Header().Get("Cache-Control") != "no-store" || response.Map()["code"] != "invalid_delivery_completion_request" || collaborationCompletionReadRows(test, fixture) != before {
				test.Fatal("malformed cursor disclosed evidence or changed authoritative rows")
			}
		})
	}
	for _, key := range []string{"root", "source", "viewer", "root_revision", "source_revision"} {
		test.Run("cursor_"+key+"_binding", func(test *testing.T) {
			var cursor map[string]any
			if err := json.Unmarshal(decoded, &cursor); err != nil {
				test.Fatal(err)
			}
			if strings.HasSuffix(key, "revision") {
				cursor[key] = cursor[key].(float64) + 1
			} else {
				cursor[key] = uuid.NewString()
			}
			raw, err := json.Marshal(cursor)
			if err != nil {
				test.Fatal(err)
			}
			fixture.call(test, fixture.token, "GET", endpoint+"&limit=1&cursor="+base64.RawURLEncoding.EncodeToString(raw), nil).Want(409)
			if collaborationCompletionReadRows(test, fixture) != before {
				test.Fatal("mismatched cursor binding created a new read authority or business fact")
			}
		})
	}
	test.Run("known_old_route_404_is_not_object_denial", func(test *testing.T) {
		missingRoute := fixture.call(test, fixture.token, "GET", fixture.pathFor("/delivery-evidence/not-a-served-contract"), nil).Want(404)
		missingRoot := fixture.call(test, fixture.token, "GET", "/api/collaboration/issues/"+uuid.NewString()+"/delivery-evidence/candidates?source_issue_id="+fixture.request.ResourceID, nil).Want(404)
		if strings.Contains(missingRoute.Text(), `"code":"not_found"`) || !reflect.DeepEqual(missingRoot.Map(), map[string]any{"code": "not_found", "error": "not_found"}) {
			test.Fatal("absent old route was indistinguishable from authoritative structured object denial")
		}
		fixture.call(test, fixture.token, "GET", fixture.pathFor(""), nil).Want(200)
		if collaborationCompletionReadRows(test, fixture) != before {
			test.Fatal("optional-route or missing-object probe changed retained business rows")
		}
	})
	test.Run("source_revision_stales_saved_cursor", func(test *testing.T) {
		fixture.call(test, fixture.token, "PUT", "/api/issues/"+fixture.request.ResourceID, map[string]any{"title": "Explicit changed ordinary child revision"}).Want(200)
		changed := collaborationCompletionReadRows(test, fixture)
		fixture.call(test, fixture.token, "GET", endpoint+"&limit=1&cursor="+url.QueryEscape(*first.NextCursor), nil).Want(409)
		var current service.CollaborationDeliveryCandidates
		fixture.call(test, fixture.token, "GET", endpoint+"&limit=1", nil).Want(200).JSON(&current)
		if current.SourceIssueRevision <= first.SourceIssueRevision || current.NextCursor == nil || collaborationCompletionReadRows(test, fixture) != changed {
			test.Fatal("old source cursor was retained as current or the fresh page wrote business state")
		}
		first = current
	})
	test.Run("revocation_rejects_cached_cursor_without_private_page", func(test *testing.T) {
		fixture.fx.Exec(test, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, testWorkspaceID, fixture.user)
		changed := collaborationCompletionReadRows(test, fixture)
		response := fixture.call(test, fixture.token, "GET", endpoint+"&limit=1&cursor="+url.QueryEscape(*first.NextCursor), nil).Want(404)
		if !reflect.DeepEqual(response.Map(), map[string]any{"code": "not_found", "error": "not_found"}) || collaborationCompletionReadRows(test, fixture) != changed {
			test.Fatal("cached cursor restored source permission or disclosed a private candidate count")
		}
	})
}

func TestCollaborationStageCompletionPreviewRouterR38Scope(test *testing.T) {
	fixture := newCompletionCollaborationRouterFixture(test, "admin")
	request, _ := fixture.authorization(test, "admin")
	body := service.CollaborationCompletionPreviewRequest{SourceIssueID: fixture.request.ResourceID, ApprovalID: fixture.approval.Approval.ID, Target: request.Target}
	before := collaborationCompletionReadRows(test, fixture)
	for _, suffix := range []string{"candidates", "completion-preview"} {
		method, endpoint := "POST", collaborationCompletionReadPath(fixture, "admin", suffix)
		var payload any = body
		if suffix == "candidates" {
			method, payload = "GET", nil
			endpoint += "?source_issue_id=" + fixture.request.ResourceID
		}
		fixture.call(test, fixture.token, method, endpoint, payload).Want(403)
		denied := fixture.call(test, fixture.adminToken, method, endpoint, payload).Want(403)
		if denied.Header().Get("Cache-Control") != "no-store" || !reflect.DeepEqual(denied.Map(), map[string]any{"code": "delivery_completion_denied", "error": "delivery_completion_denied", "message": "delivery_completion_denied"}) {
			test.Fatal("actual administrator without source authority acquired private evidence")
		}
	}
	if collaborationCompletionReadRows(test, fixture) != before {
		test.Fatal("R38/source authority refusal wrote target history or grants")
	}
	fixture.fx.Member(test, testWorkspaceID, fixture.admin, "admin")
	fixture.call(test, fixture.adminToken, "POST", collaborationCompletionReadPath(fixture, "admin", "completion-preview"), body).Want(200)
	test.Run("enabled_initiator_is_not_R38", func(test *testing.T) {
		fixture.fx.Exec(test, `UPDATE "user" SET disabled_at=NULL WHERE id=$1`, fixture.initiator)
		changed := collaborationCompletionReadRows(test, fixture)
		fixture.call(test, fixture.adminToken, "POST", collaborationCompletionReadPath(fixture, "admin", "completion-preview"), body).Want(403)
		if collaborationCompletionReadRows(test, fixture) != changed {
			test.Fatal("R38 exception bypassed the current disabled-initiator requirement")
		}
	})
	test.Run("workspace_organization_link_mismatch", func(test *testing.T) {
		fixture.fx.Exec(test, `UPDATE "user" SET disabled_at=clock_timestamp() WHERE id=$1`, fixture.initiator)
		// The schema permits exactly one internal organization. This is an
		// association mismatch, not a second tenant or an organization claim.
		other := uuid.NewString()
		fixture.fx.Exec(test, `UPDATE organization_workspace SET organization_id=$2 WHERE workspace_id=$1`, testWorkspaceID, other)
		fixture.fx.Cleanup(test, `UPDATE organization_workspace SET organization_id=$2 WHERE workspace_id=$1`, testWorkspaceID, fixture.organization)
		identity := fixture.call(test, fixture.adminToken, "GET", "/api/admin/me", nil).Want(200).Map()
		if identity["user_id"] != fixture.admin || identity["organization_id"] != fixture.organization || fixture.fx.Count(test, `SELECT count(*) FROM organization_workspace WHERE workspace_id=$1 AND organization_id=$2`, testWorkspaceID, other) != 1 {
			test.Fatal("organization mismatch did not preserve the real administrator and change the current workspace association")
		}
		changed := collaborationCompletionReadRows(test, fixture)
		for _, suffix := range []string{"candidates", "completion-preview"} {
			method, endpoint := "POST", collaborationCompletionReadPath(fixture, "admin", suffix)
			var payload any = body
			if suffix == "candidates" {
				method, payload = "GET", nil
				endpoint += "?source_issue_id=" + fixture.request.ResourceID
			}
			response := fixture.call(test, fixture.adminToken, method, endpoint, payload).Want(404)
			if response.Header().Get("Cache-Control") != "no-store" || !reflect.DeepEqual(response.Map(), map[string]any{"code": "not_found", "error": "not_found"}) {
				test.Fatal("organization-link mismatch disclosed target or private source facts")
			}
		}
		if collaborationCompletionReadRows(test, fixture) != changed {
			test.Fatal("organization-link mismatch acquired R38 target authority or changed completion facts")
		}
	})
}

// These callbacks bracket the real borrowed transaction, after actual router
// authentication and before its actual commit. Every SQL statement is forwarded;
// the competing canonical DeleteMember runs on a different PostgreSQL backend.
type collaborationCompletionReadStarter struct {
	service.TxStarter
	onBegin, onCommit func()
	backend           uint32
}

func (s *collaborationCompletionReadStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return nil, err
	}
	s.backend = tx.Conn().PgConn().PID()
	if s.onBegin != nil {
		s.onBegin()
	}
	return &collaborationCompletionReadTx{Tx: tx, starter: s}, nil
}

type collaborationCompletionReadTx struct {
	pgx.Tx
	starter *collaborationCompletionReadStarter
}

func (tx *collaborationCompletionReadTx) Commit(ctx context.Context) error {
	if tx.starter.onCommit != nil {
		tx.starter.onCommit()
	}
	return tx.Tx.Commit(ctx)
}

func collaborationCompletionBarrierRouter(test *testing.T, starter *collaborationCompletionReadStarter) http.Handler {
	test.Helper()
	router, h := NewRouterWithOptions(testPool, realtime.NewHub(), events.New(), analytics.NoopClient{}, nil, RouterOptions{})
	starter.TxStarter = h.TxStarter
	h.TxStarter = starter
	test.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if h.ChannelRouter != nil && !h.ChannelRouter.Drain(ctx) {
			test.Error("completion read router did not drain")
		}
	})
	return router
}

func collaborationCompletionTryDeleteMember(test *testing.T, fixture *completionCollaborationRouterFixture, user string) (uint32, error) {
	test.Helper()
	ctx, cancel := context.WithTimeout(test.Context(), time.Second)
	defer cancel()
	tx, err := testPool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(context.Background())
	backend := tx.Conn().PgConn().PID()
	if _, err = tx.Exec(ctx, "SET LOCAL lock_timeout='50ms'"); err != nil {
		return backend, err
	}
	q := db.New(tx)
	member, err := q.GetMemberByUserAndWorkspace(ctx, db.GetMemberByUserAndWorkspaceParams{UserID: util.MustParseUUID(user), WorkspaceID: util.MustParseUUID(fixture.fx.WorkspaceID)})
	if err != nil {
		return backend, err
	}
	if err = q.DeleteMember(ctx, member.ID); err != nil {
		return backend, err
	}
	return backend, tx.Commit(ctx)
}

func TestCollaborationStageCompletionPreviewRouterReadRevocationOrders(test *testing.T) {
	for _, shape := range []string{"list", "empty", "cursor", "preview"} {
		for _, order := range []string{"read_first", "revocation_first"} {
			test.Run(shape+"/"+order, func(test *testing.T) {
				fixture := newCompletionCollaborationRouterFixture(test, "self")
				archive := fixture.archive(test)
				reader, token := fixture.user, fixture.token
				method, endpoint := "GET", collaborationCompletionReadPath(fixture, "self", "candidates")+"?source_issue_id="+fixture.request.ResourceID
				var payload any
				if shape == "empty" {
					reader = fixture.fx.User(test, "Source member without artifact authority", "completion-empty-"+uuid.NewString()+"@example.invalid")
					fixture.fx.Member(test, testWorkspaceID, reader, "member")
					token = controlledArtifactJWT(test, reader, 0, time.Now().Add(time.Hour))
				}
				if shape == "cursor" {
					additional := fixture.create(test, fixture.taskToken)
					path := "/api/agent-approvals/" + additional.Approval.ID
					fixture.call(test, fixture.token, "POST", path+"/decision", map[string]string{"decision": "approve", "expected_spec_digest": additional.SpecDigest}).Want(200)
					fixture.call(test, fixture.taskToken, "POST", path+"/artifact-package/execution", map[string]string{"expected_spec_digest": additional.SpecDigest}).Want(200).JSON(&additional)
					var first service.CollaborationDeliveryCandidates
					fixture.call(test, token, "GET", endpoint+"&limit=1", nil).Want(200).JSON(&first)
					if additional.Status != "succeeded" || len(first.Items) != 1 || !first.HasMore || first.NextCursor == nil {
						test.Fatal("cursor race did not use two actual retained packages and a current visible page")
					}
					endpoint += "&limit=1&cursor=" + url.QueryEscape(*first.NextCursor)
				}
				if shape == "preview" {
					request, _ := fixture.authorization(test, "self")
					method, endpoint = "POST", collaborationCompletionReadPath(fixture, "self", "completion-preview")
					payload = service.CollaborationCompletionPreviewRequest{SourceIssueID: fixture.request.ResourceID, ApprovalID: fixture.approval.Approval.ID, Target: request.Target}
				}
				before := collaborationCompletionReadRows(test, fixture)
				starter := &collaborationCompletionReadStarter{}
				hits, writerBackend := 0, uint32(0)
				var removal error
				compete := func() {
					hits++
					writerBackend, removal = collaborationCompletionTryDeleteMember(test, fixture, reader)
				}
				if order == "read_first" {
					starter.onCommit = compete
				} else {
					starter.onBegin = compete
				}
				fixture.router = collaborationCompletionBarrierRouter(test, starter)
				response := fixture.call(test, token, method, endpoint, payload)
				readerBackend := starter.backend
				starter.onBegin, starter.onCommit = nil, nil
				if hits != 1 || readerBackend == 0 || writerBackend == 0 || readerBackend == writerBackend {
					test.Fatal("read/revocation barrier did not reach two real distinct database transactions exactly once")
				}
				if order == "read_first" {
					var locked *pgconn.PgError
					if !errors.As(removal, &locked) || locked.Code != "55P03" {
						test.Fatalf("read released current source membership before its actual commit: deletion=%v", removal)
					}
					response.Want(200)
					if shape == "preview" {
						var preview service.CollaborationCompletionPreview
						response.JSON(&preview)
						if preview.AuthorizationBinding == nil || preview.UsableForCompletion {
							test.Fatal("current read lost its exact non-authorizing preview")
						}
					} else {
						var page service.CollaborationDeliveryCandidates
						response.JSON(&page)
						if shape == "empty" {
							if len(page.Items) != 0 || page.HasMore || page.NextCursor != nil || strings.Contains(response.Text(), fixture.approval.Approval.ID) || strings.Contains(response.Text(), fixture.approval.ExecutionID) || strings.Contains(response.Text(), fixture.body["expected_artifact_digest"].(string)) {
								test.Fatal("authorized empty page revealed a hidden artifact or private count")
							}
						} else if len(page.Items) != 1 || page.Items[0].ApprovalID != fixture.approval.Approval.ID {
							test.Fatal("read-first page lost its actual selected ordinary-child artifact")
						}
					}
					if _, err := collaborationCompletionTryDeleteMember(test, fixture, reader); err != nil {
						test.Fatalf("source deletion did not succeed after the read committed: %v", err)
					}
				} else {
					if removal != nil {
						test.Fatalf("revocation-first transaction did not commit: %v", removal)
					}
					response.Want(404)
					if !reflect.DeepEqual(response.Map(), map[string]any{"code": "not_found", "error": "not_found"}) {
						test.Fatal("revocation-first read released private evidence, bindings or counts")
					}
				}
				denied := fixture.call(test, token, method, endpoint, payload).Want(404)
				afterArchive, err := os.ReadFile(filepath.Join(fixture.output, fixture.approval.ExecutionID, "artifact.tar"))
				if err != nil {
					test.Fatal(err)
				}
				if response.Header().Get("Cache-Control") != "no-store" || denied.Header().Get("Cache-Control") != "no-store" || !reflect.DeepEqual(denied.Map(), map[string]any{"code": "not_found", "error": "not_found"}) || collaborationCompletionReadRows(test, fixture) != before || !bytes.Equal(archive, afterArchive) {
					test.Fatal("read/cached retry changed business facts or restored revoked source authority")
				}
				test.Logf("F06_AUTHORITY_ORDER shape=%s order=%s reader_backend=%d writer_backend=%d read_status=%d before_sha256=%x after_sha256=%x", shape, order, readerBackend, writerBackend, response.Code, sha256.Sum256([]byte(before)), sha256.Sum256([]byte(collaborationCompletionReadRows(test, fixture))))
			})
		}
	}
}

func TestCollaborationStageCompletionPreviewRouterCurrentPasswordAndOrganization(test *testing.T) {
	for _, change := range []string{"password_session_changed", "administrator_role_removed", "organization_became_inactive"} {
		for _, suffix := range []string{"candidates", "completion-preview"} {
			test.Run(change+"/"+suffix, func(test *testing.T) {
				fixture, reader := collaborationCompletionReadFixture(test, "admin")
				request, _ := fixture.authorization(test, "admin")
				method, endpoint := "GET", collaborationCompletionReadPath(fixture, "admin", suffix)+"?source_issue_id="+fixture.request.ResourceID
				var payload any
				if suffix == "completion-preview" {
					method, endpoint = "POST", collaborationCompletionReadPath(fixture, "admin", suffix)
					payload = service.CollaborationCompletionPreviewRequest{SourceIssueID: fixture.request.ResourceID, ApprovalID: fixture.approval.Approval.ID, Target: request.Target}
				}
				before, archive := collaborationCompletionReadRows(test, fixture), fixture.archive(test)
				hits, want := 0, 403
				starter := &collaborationCompletionReadStarter{onBegin: func() {
					hits++
					switch change {
					case "password_session_changed":
						fixture.fx.Exec(test, `UPDATE user_password_credential SET session_version=session_version+1 WHERE user_id=$1`, fixture.admin)
					case "administrator_role_removed":
						fixture.fx.Exec(test, `DELETE FROM platform_role_binding WHERE user_id=$1`, fixture.admin)
					case "organization_became_inactive":
						fixture.fx.Exec(test, `UPDATE organization SET state='inactive' WHERE id=$1`, fixture.organization)
						fixture.fx.Cleanup(test, `UPDATE organization SET state='active' WHERE id=$1`, fixture.organization)
						want = 404
					}
				}}
				fixture.router = collaborationCompletionBarrierRouter(test, starter)
				response := fixture.call(test, reader, method, endpoint, payload).Want(want)
				starter.onBegin = nil
				code := "delivery_completion_denied"
				if want == 404 {
					code = "not_found"
				}
				if hits != 1 || response.Header().Get("Cache-Control") != "no-store" || response.Map()["code"] != code || strings.Contains(response.Text(), fixture.approval.Approval.ID) || strings.Contains(response.Text(), "observation") || collaborationCompletionReadRows(test, fixture) != before || !bytes.Equal(archive, fixture.archive(test)) {
					test.Fatal("authenticated administrator's cached credential/role/organization restored source evidence or changed business facts")
				}
			})
		}
	}
}
