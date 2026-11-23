import { Bell } from "lucide-react";
import { useInfiniteQuery, useQueryClient } from "@tanstack/react-query";
import { collaborationNotificationsOptions, type CollaborationScope } from "@multica/core/collaboration";
import { accountPath } from "@multica/core/paths";
import { AppLink } from "../navigation";
import { useT } from "../i18n";

/** Standalone workbench entry; the sidebar owns its own protected count observer. */
export function CollaborationNotifications({ scope }: { scope: CollaborationScope }) {
  const { t } = useT("collaboration");
  const client = useQueryClient();
  const query = useInfiniteQuery({ ...collaborationNotificationsOptions(client, scope), enabled: scope.userId !== "" });
  const unread = query.data?.pages[0]?.unread_count;
  const label = unread !== undefined && unread > 0
    ? `${t(($) => $.notifications.title)} ${t(($) => $.page.unread, { count: unread })}` : t(($) => $.notifications.title);
  return <AppLink href={accountPath({ view: "notifications", notificationView: "unread" })} aria-label={label} title={label}
    className="inline-flex min-h-8 min-w-0 items-center gap-2 rounded-md px-2 text-body text-muted-foreground hover:bg-accent hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring pointer-coarse:min-h-11">
    <Bell aria-hidden="true" className="size-4 shrink-0" /><span>{t(($) => $.page.notifications)}</span>
    {unread !== undefined && unread > 0 && <span aria-hidden="true" className="text-caption tabular-nums">{unread}</span>}
  </AppLink>;
}

/** Keep event labels factual and forward-compatible, including system results. */
export function useCollaborationNotificationLabel() {
  const { t } = useT("collaboration");
  return (kind: string) => {
    switch (kind) {
      case "ai_completed": return t(($) => $.notifications.aiCompleted);
      case "ai_failed": return t(($) => $.notifications.aiFailed);
      case "ai_stopped": return t(($) => $.notifications.aiStopped);
      case "assigned": return t(($) => $.notifications.assigned);
      case "removed": return t(($) => $.notifications.removed);
      case "created": case "participant_added": case "participant_removed": case "participant_left":
      case "transferred": case "information_requested": case "information_returned": case "acceptance_submitted":
      case "comment_created": case "comment_edited": case "comment_deleted": case "issue_edited":
      case "status_changed": case "iteration_changed": case "completed": case "cancelled": case "reopened":
      case "stage_roles_configured": case "stage_roles_unchanged": case "stage_proposal_created": case "stage_proposal_revised":
      case "stage_proposal_accepted": case "stage_proposal_rejected": case "stage_scope_confirmed": case "stage_code_candidate_created":
      case "stage_decision_recorded": case "stage_decision_revoked": case "stage_advanced": case "stage_execution_admitted": case "stage_execution_observed": case "stage_rollback_observed":
        return t(($) => $.activity.kinds[kind]);
      default: return t(($) => $.activity.kinds.other);
    }
  };
}
