"use client";

import { useCallback, useId, useRef } from "react";
import { useIssueStatuses } from "@multica/core/issue-statuses/hooks";
import { StatusIcon } from "../../issues/components";
import {
  IssueAgentActivityIndicator,
} from "../../issues/components/issue-agent-activity-indicator";
import { ActorAvatar } from "../../common/actor-avatar";
import { Archive, ArchiveRestore } from "lucide-react";
import type { InboxItem } from "@multica/core/types";
import type { InboxView } from "./inbox-view";
import { InboxDetailLabel } from "./inbox-detail-label";
import { getInboxDisplayTitle, getInboxDestination } from "./inbox-display";
import { useInboxContextMenu } from "./inbox-context-menu";
import { useStatusLabel } from "../../issues/utils/status-label";
import { InboxRowMenu } from "./inbox-row-menu";
import { useT } from "../../i18n";
import { paths, useWorkspaceSlug } from "@multica/core/paths";
import { resolveClickIntent, useIntentNavigate } from "../../navigation";

// Hook returning a localized relative-time formatter — the i18n equivalent
// of the previous static `timeAgo` function. Returning a function (rather
// than a string) keeps call-site usage identical: `timeAgo(dateStr)`.
export function useTimeAgo() {
  const { t } = useT("inbox");
  return (dateStr: string): string => {
    const diff = Date.now() - new Date(dateStr).getTime();
    const minutes = Math.floor(diff / 60000);
    if (minutes < 1) return t(($) => $.list.time.just_now);
    if (minutes < 60) return t(($) => $.list.time.minutes, { count: minutes });
    const hours = Math.floor(minutes / 60);
    if (hours < 24) return t(($) => $.list.time.hours, { count: hours });
    const days = Math.floor(hours / 24);
    return t(($) => $.list.time.days, { count: days });
  };
}

export function InboxListItem({
  item,
  view,
  isSelected,
  onClick,
  onAction,
  position,
  totalCount,
  onSelectionMount,
}: {
  item: InboxItem;
  view: InboxView;
  isSelected: boolean;
  onClick: () => void;
  // Archive in the main list, unarchive in the archived one — the row action is
  // always the reversal of the current view, so the two lists share this row.
  onAction: () => void;
  position?: number;
  totalCount?: number;
  onSelectionMount?: (id: string, button: HTMLButtonElement | null) => void;
}) {
  const { t } = useT("inbox");
  const timeAgo = useTimeAgo();
  // Inbox is a cross-workspace surface, so the catalog is read against the
  // item's OWN workspace rather than the route's. (MUL-6243)
  const { categoryOf: statusCategoryOf, colorOf: statusColorOf } =
    useIssueStatuses(item.workspace_id);
  const statusLabelOf = useStatusLabel(item.workspace_id);
  const openContextMenu = useInboxContextMenu();
  // Null-safe slug (not useWorkspacePaths, which throws): the row renders in
  // tests and could render outside a workspace route; without a slug the
  // modifier-click affordance simply stays off.
  const slug = useWorkspaceSlug();
  const issueHref = slug ? getInboxDestination(item, paths.workspace(slug)) : null;
  const intentNavigate = useIntentNavigate();
  const displayTitle = getInboxDisplayTitle(item);
  const detailId = useId();
  const timeId = useId();
  const selectionButton = useRef<HTMLButtonElement | null>(null);
  const attachSelection = useCallback((button: HTMLButtonElement | null) => {
    selectionButton.current = button;
    onSelectionMount?.(item.id, button);
  }, [item.id, onSelectionMount]);
  const isArchivedView = view === "archived";
  // Archiving deliberately leaves `read` untouched so unarchiving restores the
  // real unread state, so archived rows would otherwise keep an unread marker
  // the user cannot clear from this view. Suppress the affordance here only.
  const showUnread = item.read !== true && !isArchivedView;
  const ActionIcon = isArchivedView ? ArchiveRestore : Archive;
  const actionLabel = isArchivedView
    ? t(($) => $.list.unarchive_tooltip)
    : t(($) => $.list.archive_tooltip);
  const actorType = item.actor_type ?? item.recipient_type;
  // The glyph is per CATEGORY, so it alone cannot tell "In Review" from a
  // custom "Human Review" — moving between two statuses of the same category
  // left this row pixel-identical and read as "the inbox never updated"
  // (MUL-6395). Colour is what carries a custom status's own identity, exactly
  // as the status-changed detail label already renders it. `colorOf` returns
  // null for a built-in, which keeps it on its semantic token.
  const statusColor = item.issue_status ? statusColorOf(item.issue_status) : null;

  return (
    // The title selects this notification. Other row controls remain siblings
    // rather than descendants of a button/listbox option, where assistive
    // technology can lose their independent semantics.
    <div
      role="listitem"
      aria-posinset={position}
      aria-setsize={totalCount}
      onClick={(e) => {
        if (e.defaultPrevented) return;
        const control = e.target instanceof Element ? e.target.closest("button, a, input, [role=button]") : null;
        if (control && control !== selectionButton.current) return;
        // Plain click keeps the master-detail selection; a modifier click on
        // a row that references an issue opens that issue as its own tab.
        if (issueHref) {
          const intent = resolveClickIntent(e);
          if (intent !== "push") {
            intentNavigate(issueHref, intent);
            return;
          }
        }
        // Safari does not focus a button on pointer activation.
        selectionButton.current?.focus({ preventScroll: true });
        onClick();
      }}
      onAuxClick={(e) => {
        if (e.defaultPrevented || e.button !== 1 || !issueHref) return;
        e.preventDefault();
        intentNavigate(issueHref, "background-tab");
      }}
      // Right-click opens the list's shared menu (mark read/unread, archive).
      // `select-none` mirrors what Base UI's own trigger used to merge in, so
      // right-clicking a row never starts a text selection.
      onContextMenu={
        openContextMenu ? (e) => openContextMenu(item, e) : undefined
      }
      className={`group flex w-full cursor-default select-none items-center gap-3 rounded-md px-2 py-2.5 text-left outline-none transition-colors has-[:focus-visible]:ring-1 has-[:focus-visible]:ring-ring ${
        isSelected
          ? "bg-accent"
          : "hover:bg-accent/50 data-[popup-open]:bg-accent/50"
      }`}
    >
      <ActorAvatar
        actorType={actorType}
        actorId={item.actor_id ?? item.recipient_id}
        size="lg"
        enableHoverCard
      />
      <div className="min-w-0 flex-1">
        <div className="flex items-center justify-between gap-2">
          <div className="flex min-w-0 items-center gap-1.5">
            {showUnread && (
              <span className="h-1.5 w-1.5 shrink-0 rounded-full bg-brand" />
            )}
            <button
              ref={attachSelection}
              type="button"
              data-inbox-selection={item.id}
              aria-current={isSelected ? "true" : undefined}
              aria-describedby={`${detailId} ${timeId}`}
              className={`truncate text-left text-body outline-none ${showUnread ? "font-medium" : "text-muted-foreground"}`}
            >
              {displayTitle}
            </button>
          </div>
          <div className="flex shrink-0 items-center gap-1">
            {/* Pointer-only affordance: revealed on hover, and on keyboard
                focus anywhere in the row so it is reachable by Tab. Touch has
                neither, so it stays hidden on a pointer that cannot hover and
                the compact menu below carries the same action. */}
            <button
              type="button"
              title={actionLabel}
              aria-label={actionLabel}
              onClick={(e) => {
                e.stopPropagation();
                onAction();
              }}
              className="hidden rounded p-0.5 text-muted-foreground outline-none hover:bg-accent hover:text-foreground focus-visible:ring-1 focus-visible:ring-ring [@media(hover:hover)]:group-hover:inline-flex [@media(hover:hover)]:group-focus-within:inline-flex"
            >
              <ActionIcon className="h-3.5 w-3.5" />
            </button>
            <InboxRowMenu item={item} view={view} />
            {item.issue_status && (
              // Icon-only, like every other issue row — but a colour is not a
              // name, and this row has no space for the CustomStatusChip the
              // board card and list row carry. `title` is that name, and it is
              // the same affordance the archive button above already uses.
              <span
                title={statusLabelOf(item.issue_status)}
                className="flex shrink-0 items-center"
              >
                <StatusIcon
                  status={item.issue_status}
                  category={statusCategoryOf(item.issue_status)}
                  color={statusColor}
                  className="h-3.5 w-3.5 shrink-0"
                />
              </span>
            )}
          </div>
        </div>
        <div className="mt-0.5 flex items-center justify-between gap-2">
          <p id={detailId} className="min-w-0 overflow-hidden text-ellipsis whitespace-nowrap text-caption text-muted-foreground">
            <InboxDetailLabel item={item} />
          </p>
          <div className="flex shrink-0 items-center gap-1.5">
            {/* Badge only, no hover card (MUL-5189). "An agent is on this"
                is worth showing while triaging; the card behind it adds only
                elapsed time, which does not change whether you open the row.
                The row already carries the ActorAvatar hover card on the
                left, so a second popup here was mostly noise. */}
            {item.issue_id && (
              <IssueAgentActivityIndicator
                issueId={item.issue_id}
                hoverCard={false}
              />
            )}
            <span id={timeId} className="text-caption text-muted-foreground">
              {timeAgo(item.created_at)}
            </span>
          </div>
        </div>
      </div>
    </div>
  );
}
