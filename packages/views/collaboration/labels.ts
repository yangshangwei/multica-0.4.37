import { useT } from "../i18n";

export function useCollaborationLabels() {
  const { t } = useT("collaboration");
  return {
    reason: (code: string) => {
      switch (code) {
        case "target_registered": return t(($) => $.resources.registered);
        case "resource_consented": return t(($) => $.resources.authorized);
        case "grant_revoked": case "grant_already_revoked": return t(($) => $.resources.withdrawn);
        case "switch_proposed": return t(($) => $.resources.proposed);
        case "switch_resource_consented": return t(($) => $.resources.switchConsent);
        case "switch_consent_revoked": return t(($) => $.resources.switchWithdrawn);
        case "target_switched": return t(($) => $.resources.switched);
        case "owner_not_participant": case "resource_owner_not_participant": return t(($) => $.resources.ownerRequired);
        case "resource_owner_unavailable": return t(($) => $.resources.ownerUnavailable);
        case "target_exists": return t(($) => $.resources.alreadyRegistered);
        case "runtime_registration_required": return t(($) => $.resources.registrationRequired);
        case "target_pause_required": return t(($) => $.resources.pauseRequired);
        case "target_configuration_changed": return t(($) => $.execution.config_invalid);
        case "switch_consent_required": return t(($) => $.execution.consentMissing);
        case "stale_switch_proposal": case "stale_switch_consent": case "stale_grant": return t(($) => $.execution.staleRequest);
        case "waiting": return t(($) => $.execution.waiting);
        case "issue_paused": case "queue_paused": return t(($) => $.execution.issuePaused);
        case "target_paused": return t(($) => $.execution.lanePaused);
        case "target_busy": case "channel_busy": return t(($) => $.execution.busy);
        case "runtime_offline": return t(($) => $.execution.offline);
        case "shared_runtime_unavailable": case "collaboration_execution_unavailable": case "command_unavailable": return t(($) => $.execution.unavailable);
        case "runtime_upgrade_required": return t(($) => $.execution.upgrade);
        case "squad_runtime_unavailable": return t(($) => $.execution.squadUnavailable);
        case "not_participant": return t(($) => $.execution.notParticipant);
        case "target_unavailable": case "stale_target": return t(($) => $.execution.targetUnavailable);
        case "issue_closed": return t(($) => $.execution.closed);
        case "stop_unconfirmed": return t(($) => $.execution.proofPending);
        case "grant_required": return t(($) => $.execution.grant_required);
        case "config_invalid": return t(($) => $.execution.config_invalid);
        case "initial": return t(($) => $.execution.initial);
        case "switch_hold": return t(($) => $.execution.switch_hold);
        case "resumed": case "already_active": case "hold_released": return t(($) => $.execution.active);
        case "stop_requested": return t(($) => $.execution.stopRequested);
        case "already_ended": return t(($) => $.execution.proofConfirmed);
        case "command_not_waiting": return t(($) => $.execution.notWaiting);
        case "stale_authority": case "stale_scope": case "stale_control_version": case "stale_execution": case "stale_hold": case "stale_admission": case "stale_revision": return t(($) => $.execution.staleRequest);
        default: return t(($) => $.page.unavailable);
      }
    },
    status: (key: string) => {
      switch (key) {
        case "backlog": return t(($) => $.statuses.backlog);
        case "todo": return t(($) => $.statuses.todo);
        case "in_progress": return t(($) => $.statuses.in_progress);
        case "in_review": return t(($) => $.statuses.in_review);
        case "blocked": return t(($) => $.statuses.blocked);
        case "done": return t(($) => $.statuses.done);
        case "cancelled": return t(($) => $.statuses.cancelled);
        default: return key;
      }
    },
    action: (key: string) => {
      switch (key) {
        case "invite": return t(($) => $.workflow.invite);
        case "transfer": return t(($) => $.workflow.transfer);
        case "request_information": return t(($) => $.workflow.requestInformation);
        case "return_information": return t(($) => $.workflow.returnInformation);
        case "submit_acceptance": return t(($) => $.workflow.acceptance);
        case "change_status": return t(($) => $.workflow.changeStatus);
        case "complete": return t(($) => $.workflow.complete);
        case "cancel": return t(($) => $.workflow.cancelIssue);
        case "reopen": return t(($) => $.workflow.reopen);
        case "leave": return t(($) => $.workflow.leave);
        case "edit": return t(($) => $.workflow.edit);
        case "edit_source_structure": return t(($) => $.fields.editSource);
        case "change_iteration": return t(($) => $.fields.editIteration);
        default: return t(($) => $.workflow.pendingTitle);
      }
    },
    execution: (key: string) => {
      switch (key) {
        case "paused": return t(($) => $.execution.paused);
        case "active": return t(($) => $.execution.active);
        case "closed": return t(($) => $.execution.closed);
        case "waiting": return t(($) => $.execution.waiting);
        case "in_flight": return t(($) => $.execution.in_flight);
        case "succeeded": return t(($) => $.execution.succeeded);
        case "failed": return t(($) => $.execution.failed);
        case "stopped": return t(($) => $.execution.stopped);
        case "cancelled": return t(($) => $.execution.cancelled);
        case "waiting_children": return t(($) => $.execution.waiting_children);
        case "ending": return t(($) => $.execution.ending);
        case "initial": return t(($) => $.execution.initial);
        case "grant_required": return t(($) => $.execution.grant_required);
        case "config_invalid": return t(($) => $.execution.config_invalid);
        case "switch_hold": return t(($) => $.execution.switch_hold);
        case "pending": return t(($) => $.execution.pending);
        case "consent_ready": return t(($) => $.execution.consent_ready);
        case "stale": return t(($) => $.execution.stale);
        case "committed": return t(($) => $.execution.committed);
        default: return t(($) => $.page.unavailable);
      }
    },
  };
}
