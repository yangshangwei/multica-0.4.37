import type { ProjectStatus, ProjectPriority } from "../types";

export const PROJECT_STATUS_ORDER: ProjectStatus[] = [
  "planned",
  "in_progress",
  "paused",
  "completed",
  "cancelled",
];

export const PROJECT_STATUS_CONFIG: Record<
  ProjectStatus,
  { label: string; color: string; dotColor: string; badgeBg: string; badgeText: string }
> = {
  planned: { label: "Planned", color: "text-muted-foreground", dotColor: "bg-muted-foreground", badgeBg: "bg-muted", badgeText: "text-muted-foreground" },
  in_progress: { label: "In Progress", color: "text-warning-foreground", dotColor: "bg-warning", badgeBg: "bg-warning/10", badgeText: "text-warning-foreground" },
  paused: { label: "Paused", color: "text-muted-foreground", dotColor: "bg-muted-foreground", badgeBg: "bg-muted", badgeText: "text-muted-foreground" },
  completed: { label: "Completed", color: "text-info-foreground", dotColor: "bg-info", badgeBg: "bg-info/10", badgeText: "text-info-foreground" },
  cancelled: { label: "Cancelled", color: "text-destructive-foreground", dotColor: "bg-destructive", badgeBg: "bg-muted", badgeText: "text-muted-foreground" },
};

export const PROJECT_PRIORITY_ORDER: ProjectPriority[] = [
  "urgent",
  "high",
  "medium",
  "low",
  "none",
];

export const PROJECT_PRIORITY_CONFIG: Record<
  ProjectPriority,
  { label: string; bars: number; color: string; badgeBg: string; badgeText: string }
> = {
  urgent: { label: "Urgent", bars: 4, color: "text-destructive-foreground", badgeBg: "bg-destructive/10", badgeText: "text-destructive-foreground" },
  high: { label: "High", bars: 3, color: "text-warning-foreground", badgeBg: "bg-warning/10", badgeText: "text-warning-foreground" },
  medium: { label: "Medium", bars: 2, color: "text-warning-foreground", badgeBg: "bg-warning/10", badgeText: "text-warning-foreground" },
  low: { label: "Low", bars: 1, color: "text-info-foreground", badgeBg: "bg-info/10", badgeText: "text-info-foreground" },
  none: { label: "No priority", bars: 0, color: "text-muted-foreground", badgeBg: "bg-muted", badgeText: "text-muted-foreground" },
};
