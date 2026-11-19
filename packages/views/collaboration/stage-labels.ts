import { CollaborationStagePrepareError, CollaborationStageIntentStorageError, type CollaborationStageOutcome, type CollaborationStageRequestAction, type CollaborationStageBlocker } from "@multica/core/collaboration";
import { useT } from "../i18n";

export type StageFeedback = { text: string; detail?: string; tone: "success" | "neutral" | "failed"; blockers?: CollaborationStageBlocker[]; blockersTruncated?: boolean };

/** Every server enum goes through a `default` branch: an unfamiliar value is shown as unknown, never as a known state. */
export function useCollaborationStageLabels() {
  const { t } = useT("collaboration");
  const refusal = (code: string | null): string => {
    switch (code) {
      case "forbidden": return t(($) => $.stage.refusals.forbidden);
      case "stale_authority": return t(($) => $.stage.refusals.stale_authority);
      case "stale_revision": return t(($) => $.stage.refusals.stale_revision);
      case "stage_policy_disabled": return t(($) => $.stage.refusals.stage_policy_disabled);
      case "stage_policy_changed": return t(($) => $.stage.refusals.stage_policy_changed);
      case "stage_roles_unconfigured": return t(($) => $.stage.refusals.stage_roles_unconfigured);
      case "stage_roles_changed": return t(($) => $.stage.refusals.stage_roles_changed);
      case "stage_role_required": return t(($) => $.stage.refusals.stage_role_required);
      case "issue_closed": return t(($) => $.stage.refusals.issue_closed);
      case "stage_configuration_denied": return t(($) => $.stage.refusals.stage_configuration_denied);
      case "stage_self_review_forbidden": return t(($) => $.stage.refusals.stage_self_review_forbidden);
      case "stage_qa_release_conflict": return t(($) => $.stage.refusals.stage_qa_release_conflict);
      case "stage_role_target_unavailable": return t(($) => $.stage.refusals.stage_role_target_unavailable);
      case "stage_source_unavailable": return t(($) => $.stage.refusals.stage_source_unavailable);
      case "stage_proposal_not_found": return t(($) => $.stage.refusals.stage_proposal_not_found);
      case "stage_candidate_changed": return t(($) => $.stage.refusals.stage_candidate_changed);
      case "stage_proposal_already_materialized": return t(($) => $.stage.refusals.stage_proposal_already_materialized);
      case "stage_proposal_unchanged": return t(($) => $.stage.refusals.stage_proposal_unchanged);
      case "stage_candidate_not_pending": return t(($) => $.stage.refusals.stage_candidate_not_pending);
      case "stage_candidate_stale": return t(($) => $.stage.refusals.stage_candidate_stale);
      case "stage_needs_confirmation": return t(($) => $.stage.refusals.stage_needs_confirmation);
      case "stage_source_creation_denied": return t(($) => $.stage.refusals.stage_source_creation_denied);
      case "stage_child_creation_refused": return t(($) => $.stage.refusals.stage_child_creation_refused);
      case "stage_scope_changed": return t(($) => $.stage.refusals.stage_scope_changed);
      case "stage_scope_child_unavailable": return t(($) => $.stage.refusals.stage_scope_child_unavailable);
      case "stage_child_unconfirmed": return t(($) => $.stage.refusals.stage_child_unconfirmed);
      case "stage_pull_request_unavailable": return t(($) => $.stage.refusals.stage_pull_request_unavailable);
      case "stage_head_changed": return t(($) => $.stage.refusals.stage_head_changed);
      case "stage_trust_changed": return t(($) => $.stage.refusals.stage_trust_changed);
      case "stage_artifact_unavailable": return t(($) => $.stage.refusals.stage_artifact_unavailable);
      case "stage_ci_missing": return t(($) => $.stage.refusals.stage_ci_missing);
      case "stage_ci_failed": return t(($) => $.stage.refusals.stage_ci_failed);
      case "stage_ci_stale": return t(($) => $.stage.refusals.stage_ci_stale);
      case "stage_ci_unsupported": return t(($) => $.stage.refusals.stage_ci_unsupported);
      case "stage_evidence_changed": return t(($) => $.stage.refusals.stage_evidence_changed);
      case "stage_evidence_unavailable": return t(($) => $.stage.refusals.stage_evidence_unavailable);
      case "stage_dependency_invalid": return t(($) => $.stage.refusals.stage_dependency_invalid);
      case "stage_test_report_unusable": return t(($) => $.stage.refusals.stage_test_report_unusable);
      case "stage_execution_fact_missing": return t(($) => $.stage.refusals.stage_execution_fact_missing);
      case "stage_execution_unavailable": return t(($) => $.stage.refusals.stage_execution_unavailable);
      case "stage_execution_target_changed": return t(($) => $.stage.refusals.stage_execution_target_changed);
      case "stage_execution_pending": return t(($) => $.stage.refusals.stage_execution_pending);
      case "stage_execution_conflict": return t(($) => $.stage.refusals.stage_execution_conflict);
      case "stage_rollback_observation_unavailable": return t(($) => $.stage.refusals.stage_rollback_observation_unavailable);
      case "stage_rollback_observation_changed": return t(($) => $.stage.refusals.stage_rollback_observation_changed);
      case "stage_stage_changed": return t(($) => $.stage.refusals.stage_stage_changed);
      case "stage_decision_expired": return t(($) => $.stage.refusals.stage_decision_expired);
      case "stage_signer_changed": return t(($) => $.stage.refusals.stage_signer_changed);
      case "stage_decision_not_found": return t(($) => $.stage.refusals.stage_decision_not_found);
      case "stage_decision_not_revocable": return t(($) => $.stage.refusals.stage_decision_not_revocable);
      case "stage_transition_invalid": return t(($) => $.stage.refusals.stage_transition_invalid);
      case "stage_expiry_exceeds_limit": return t(($) => $.stage.refusals.stage_expiry_exceeds_limit);
      case "stage_incomplete": return t(($) => $.stage.refusals.stage_incomplete);
      case "stage_policy_exit_required": return t(($) => $.stage.refusals.stage_policy_exit_required);
      case "stage_scope_unconfirmed": return t(($) => $.stage.refusals.stage_scope_unconfirmed);
      case "stage_not_reached": return t(($) => $.stage.refusals.stage_not_reached);
      case "stage_code_candidate_missing": return t(($) => $.stage.refusals.stage_code_candidate_missing);
      case "stage_code_review_missing": return t(($) => $.stage.refusals.stage_code_review_missing);
      case "stage_qa_missing": return t(($) => $.stage.refusals.stage_qa_missing);
      case "stage_release_approval_missing": return t(($) => $.stage.refusals.stage_release_approval_missing);
      case "stage_decision_revoked": return t(($) => $.stage.refusals.stage_decision_revoked);
      case "stage_submission_unbound": return t(($) => $.stage.refusals.stage_submission_unbound);
      case "stage_scope_code_unavailable": return t(($) => $.stage.refusals.stage_scope_code_unavailable);
      case "stage_policy_unavailable": return t(($) => $.stage.refusals.stage_policy_unavailable);
      case "stage_policy_denied": return t(($) => $.stage.refusals.stage_policy_denied);
      case "stage_policy_management_denied": return t(($) => $.stage.refusals.stage_policy_management_denied);
      case "stage_policy_project_not_found": return t(($) => $.stage.refusals.stage_policy_project_not_found);
      case "stage_policy_revision_conflict": return t(($) => $.stage.refusals.stage_policy_revision_conflict);
      case "stage_policy_busy": return t(($) => $.stage.refusals.stage_policy_busy);
      case "stage_schema_unavailable": return t(($) => $.stage.refusals.stage_schema_unavailable);
      case "stage_route_unavailable": return t(($) => $.stage.refusals.stage_route_unavailable);
      case "collaboration_busy": return t(($) => $.stage.refusals.collaboration_busy);
      case "idempotency_conflict": return t(($) => $.stage.refusals.idempotency_conflict);
      case "invalid_request": case "invalid_version": case "invalid_request_id": case "invalid_stage_request": case "invalid_stage_policy_request": case "json_required":
        return t(($) => $.stage.refusals.invalid);
      case "request_too_large": return t(($) => $.stage.refusals.request_too_large);
      default: return t(($) => $.stage.refusals.unknown);
    }
  };
  const confirmed = (action: CollaborationStageRequestAction, outcome: Extract<CollaborationStageOutcome, { kind: "confirmed" }>): string => {
    switch (action) {
      case "configure_roles": return outcome.code === "stage_roles_unchanged" ? t(($) => $.stage.outcome.confirmed.roles_unchanged) : t(($) => $.stage.outcome.confirmed.configure_roles);
      case "create_proposal": return t(($) => $.stage.outcome.confirmed.create_proposal);
      case "revise_proposal": return t(($) => $.stage.outcome.confirmed.revise_proposal);
      case "decide_proposal": return outcome.code === "stage_proposal_rejected" ? t(($) => $.stage.outcome.confirmed.rejected) : t(($) => $.stage.outcome.confirmed.accepted);
      case "confirm_scope": return t(($) => $.stage.outcome.confirmed.confirm_scope);
      case "create_code_candidate": return t(($) => $.stage.code.savedCandidate);
      case "decide_stage": return t(($) => $.stage.code.savedDecision);
      case "revoke_decision": return t(($) => $.stage.code.savedRevocation);
      case "advance_stage": return t(($) => $.stage.code.savedAdvance);
      case "revise_policy": return t(($) => $.stage.outcome.confirmed.revise_policy);
      default: return t(($) => $.stage.outcome.confirmed.other);
    }
  };
  return {
    refusal,
    codeCandidateState: (value: string) => {
      switch (value) {
        case "current": return t(($) => $.stage.code.candidateStates.current);
        case "superseded": return t(($) => $.stage.code.candidateStates.superseded);
        case "stale": return t(($) => $.stage.code.candidateStates.stale);
        default: return t(($) => $.stage.code.unknown);
      }
    },
    pullRequestState: (value: string) => {
      switch (value) {
        case "current": return t(($) => $.stage.code.pullRequestStates.current);
        case "head_changed": return t(($) => $.stage.code.pullRequestStates.head_changed);
        case "unlinked": return t(($) => $.stage.code.pullRequestStates.unlinked);
        case "trust_changed": return t(($) => $.stage.code.pullRequestStates.trust_changed);
        case "unavailable": return t(($) => $.stage.code.pullRequestStates.unavailable);
        default: return t(($) => $.stage.code.unknown);
      }
    },
    ciState: (value: string) => {
      switch (value) {
        case "passed": return t(($) => $.stage.code.ciStates.passed);
        case "missing": return t(($) => $.stage.code.ciStates.missing);
        case "failed": return t(($) => $.stage.code.ciStates.failed);
        case "stale": return t(($) => $.stage.code.ciStates.stale);
        case "unsupported": return t(($) => $.stage.code.ciStates.unsupported);
        case "unavailable": return t(($) => $.stage.code.ciStates.unavailable);
        default: return t(($) => $.stage.code.unknown);
      }
    },
    codeStep: (value: string) => {
      switch (value) {
        case "code_review": return t(($) => $.stage.code.steps.code_review);
        case "qa": return t(($) => $.stage.code.steps.qa);
        case "release": return t(($) => $.stage.code.steps.release);
        case "rollback": return t(($) => $.stage.code.steps.rollback);
        default: return t(($) => $.stage.code.unknown);
      }
    },
    codeDecision: (value: string) => {
      switch (value) {
        case "approved": return t(($) => $.stage.code.decisionKinds.approved);
        case "returned": return t(($) => $.stage.code.decisionKinds.returned);
        default: return t(($) => $.stage.code.unknown);
      }
    },
    codeDecisionState: (value: string) => {
      switch (value) {
        case "valid": return t(($) => $.stage.code.decisionStates.valid);
        case "expired": return t(($) => $.stage.code.decisionStates.expired);
        case "revoked": return t(($) => $.stage.code.decisionStates.revoked);
        case "signer_changed": return t(($) => $.stage.code.decisionStates.signer_changed);
        case "roles_changed": return t(($) => $.stage.code.decisionStates.roles_changed);
        case "policy_changed": return t(($) => $.stage.code.decisionStates.policy_changed);
        case "evidence_changed": return t(($) => $.stage.code.decisionStates.evidence_changed);
        case "superseded": return t(($) => $.stage.code.decisionStates.superseded);
        default: return t(($) => $.stage.code.unknown);
      }
    },
    reportResult: (value: string) => {
      switch (value) {
        case "passed": return t(($) => $.stage.code.reportResults.passed);
        case "failed": return t(($) => $.stage.code.reportResults.failed);
        case "incomplete": return t(($) => $.stage.code.reportResults.incomplete);
        default: return t(($) => $.stage.code.unknown);
      }
    },
    reportApplicability: (value: string) => {
      switch (value) {
        case "usable": return t(($) => $.stage.code.reportApplicability.usable);
        case "stale": return t(($) => $.stage.code.reportApplicability.stale);
        case "execution_failed": return t(($) => $.stage.code.reportApplicability.execution_failed);
        case "execution_unsettled": return t(($) => $.stage.code.reportApplicability.execution_unsettled);
        default: return t(($) => $.stage.code.unknown);
      }
    },
    executionKind: (value: string) => {
      switch (value) {
        case "deployment": return t(($) => $.stage.code.executionKinds.deployment);
        case "rollback": return t(($) => $.stage.code.executionKinds.rollback);
        default: return t(($) => $.stage.code.unknown);
      }
    },
    executionResult: (value: string) => {
      switch (value) {
        case "succeeded": return t(($) => $.stage.code.executionResults.succeeded);
        case "failed": return t(($) => $.stage.code.executionResults.failed);
        default: return t(($) => $.stage.code.unknown);
      }
    },
    executionOperationState: (value: string) => {
      switch (value) {
        case "pending": return t(($) => $.stage.execution.states.pending);
        case "unknown": return t(($) => $.stage.execution.states.unknown);
        case "not_started": return t(($) => $.stage.execution.states.not_started);
        case "succeeded": return t(($) => $.stage.execution.states.succeeded);
        case "failed": return t(($) => $.stage.execution.states.failed);
        default: return t(($) => $.stage.code.unknown);
      }
    },
    metricCondition: (value: string) => {
      switch (value) {
        case "anomaly": return t(($) => $.stage.execution.conditions.anomaly);
        case "healthy": return t(($) => $.stage.execution.conditions.healthy);
        default: return t(($) => $.stage.execution.conditions.unknown);
      }
    },
    metricState: (value: string) => {
      switch (value) {
        case "current": return t(($) => $.stage.execution.metricStates.current);
        case "ambiguous": return t(($) => $.stage.execution.metricStates.ambiguous);
        case "superseded": return t(($) => $.stage.execution.metricStates.superseded);
        case "expired": return t(($) => $.stage.execution.metricStates.expired);
        case "unavailable": return t(($) => $.stage.execution.metricStates.unavailable);
        default: return t(($) => $.stage.code.unknown);
      }
    },
    stage: (value: string | null) => {
      switch (value) {
        case "development": return t(($) => $.stage.stages.development);
        case "code_review": return t(($) => $.stage.stages.code_review);
        case "qa": return t(($) => $.stage.stages.qa);
        case "release": return t(($) => $.stage.stages.release);
        case "verification": return t(($) => $.stage.stages.verification);
        default: return t(($) => $.stage.stages.unknown);
      }
    },
    scopeState: (value: string) => {
      switch (value) {
        case "none": return t(($) => $.stage.scope.states.none);
        case "current": return t(($) => $.stage.scope.states.current);
        case "lifecycle_changed": return t(($) => $.stage.scope.states.lifecycle_changed);
        case "policy_changed": return t(($) => $.stage.scope.states.policy_changed);
        case "signer_changed": return t(($) => $.stage.scope.states.signer_changed);
        default: return t(($) => $.stage.scope.states.unknown);
      }
    },
    origin: (value: string) => {
      switch (value) {
        case "materialized": return t(($) => $.stage.scope.origins.materialized);
        case "confirmed": return t(($) => $.stage.scope.origins.confirmed);
        case "none": return t(($) => $.stage.scope.origins.none);
        default: return t(($) => $.stage.scope.origins.unknown);
      }
    },
    codeState: (value: string) => {
      switch (value) {
        case "none": return t(($) => $.stage.scope.code.states.none);
        case "current": return t(($) => $.stage.scope.code.states.current);
        case "changed": return t(($) => $.stage.scope.code.states.changed);
        case "unavailable": return t(($) => $.stage.scope.code.states.unavailable);
        default: return t(($) => $.stage.scope.code.states.unknown);
      }
    },
    proposalState: (value: string) => {
      switch (value) {
        case "pending": return t(($) => $.stage.proposals.states.pending);
        case "rejected": return t(($) => $.stage.proposals.states.rejected);
        case "accepted": return t(($) => $.stage.proposals.states.accepted);
        case "superseded": return t(($) => $.stage.proposals.states.superseded);
        default: return t(($) => $.stage.proposals.states.unknown);
      }
    },
    priority: (value: string) => {
      switch (value) {
        case "urgent": return t(($) => $.priorities.urgent);
        case "high": return t(($) => $.priorities.high);
        case "medium": return t(($) => $.priorities.medium);
        case "low": return t(($) => $.priorities.low);
        case "none": return t(($) => $.priorities.none);
        default: return value;
      }
    },
    pendingAction: (action: string) => {
      switch (action) {
        case "configure_roles": return t(($) => $.stage.pending.actions.configure_roles);
        case "create_proposal": return t(($) => $.stage.pending.actions.create_proposal);
        case "revise_proposal": return t(($) => $.stage.pending.actions.revise_proposal);
        case "decide_proposal": return t(($) => $.stage.pending.actions.decide_proposal);
        case "confirm_scope": return t(($) => $.stage.pending.actions.confirm_scope);
        case "create_code_candidate": return t(($) => $.stage.code.create);
        case "decide_stage": return t(($) => $.stage.code.decisions);
        case "revoke_decision": return t(($) => $.stage.code.revoke);
        case "advance_stage": return t(($) => $.stage.code.confirmAdvance);
        case "revise_policy": return t(($) => $.stage.pending.actions.revise_policy);
        default: return t(($) => $.stage.pending.title);
      }
    },
    /** A stored refusal keeps its original status and code visible; an unknown result never reads as a failure of the request. */
    outcome: (action: CollaborationStageRequestAction, outcome: CollaborationStageOutcome, recovery: boolean): StageFeedback => {
      switch (outcome.kind) {
        case "confirmed":
          return { text: confirmed(action, outcome), detail: outcome.cleanupFailed ? t(($) => $.stage.outcome.cleanupFailed) : undefined, tone: "success" };
        case "refused":
          return { text: t(($) => $.stage.outcome.refused, { reason: refusal(outcome.code), status: outcome.status, code: outcome.code }), detail: t(($) => $.stage.outcome.refusedHint), tone: "failed", blockers: outcome.blockers, blockersTruncated: outcome.blockersTruncated };
        case "not_written":
          // "Not available yet" is a state of the server, not an error of the request.
          return { text: t(($) => $.stage.outcome.notWritten, { reason: refusal(outcome.code), status: outcome.status, code: outcome.code }),
            tone: outcome.code === "stage_policy_unavailable" || outcome.code === "stage_route_unavailable" || outcome.code === "stage_schema_unavailable" ? "neutral" : "failed" };
        default:
          return { text: outcome.reason === "incomplete" ? t(($) => $.stage.outcome.incomplete) : recovery ? t(($) => $.stage.outcome.stillUnknown) : t(($) => $.stage.outcome.unknown), tone: "neutral" };
      }
    },
    failure: (error: unknown): StageFeedback => {
      if (error instanceof CollaborationStagePrepareError) {
        switch (error.reason) {
          case "stale_view": return { text: t(($) => $.stage.outcome.prepare.stale_view), tone: "neutral" };
          case "role_conflict": return { text: t(($) => $.stage.outcome.prepare.role_conflict), tone: "failed" };
          case "invalid": return { text: t(($) => $.stage.outcome.prepare.invalid), tone: "failed" };
          case "too_large": return { text: t(($) => $.stage.outcome.prepare.too_large), tone: "failed" };
          case "activation_unavailable": return { text: t(($) => $.stage.outcome.prepare.activation_unavailable), tone: "neutral" };
          default: return { text: t(($) => $.stage.outcome.prepare.unavailable), tone: "failed" };
        }
      }
      if (error instanceof CollaborationStageIntentStorageError) return { text: t(($) => $.stage.outcome.storage), tone: "failed" };
      return { text: t(($) => $.stage.outcome.failed), tone: "failed" };
    },
  };
}

export type CollaborationStageLabels = ReturnType<typeof useCollaborationStageLabels>;
