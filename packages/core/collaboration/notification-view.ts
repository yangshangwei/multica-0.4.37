export const COLLABORATION_NOTIFICATION_VIEWS = ["unread", "all", "archived"] as const;

export type CollaborationNotificationView = (typeof COLLABORATION_NOTIFICATION_VIEWS)[number];

export function isCollaborationNotificationView(value: unknown): value is CollaborationNotificationView {
  return COLLABORATION_NOTIFICATION_VIEWS.some((view) => view === value);
}
