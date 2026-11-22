import { isCollaborationNotificationView, type CollaborationNotificationView } from "../collaboration/notification-view";

/** Account routes are independent of workspace membership and identifiers. */
export type AccountView = "current" | "participating";
export interface AccountTarget {
  issueId: string | null;
  commentId: string | null;
  runId?: string | null;
  /** Exact formal delivery; opens retained submission history when needed. */
  submissionId?: string | null;
  anchor?: "description" | "memory";
  /** Exact shared-context version; implies the memory section. */
  memoryVersion?: string | null;
}

export interface AccountWorkbenchDestination extends AccountTarget {
  view: AccountView;
  administration?: true;
}

export interface AccountNotificationsDestination extends AccountTarget {
  view: "notifications";
  notificationView: CollaborationNotificationView;
  notificationId: string | null;
  administration?: never;
}

export type AccountDestination = AccountWorkbenchDestination | AccountNotificationsDestination;

const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;
// Shared-context versions are positive signed-64-bit decimal strings.
function memoryVersion(value: string): boolean {
  return /^[1-9][0-9]{0,18}$/.test(value) && BigInt(value) <= 9223372036854775807n;
}

export function parseAccountPath(path: string): AccountDestination | null {
  if (!/^\/account(?:[?#]|$)/.test(path)) return null;
  const url = new URL(path, "https://account.invalid");
  if (url.pathname !== "/account" || url.origin !== "https://account.invalid") return null;
  const { searchParams: query } = url;
  if ([...query.keys()].some((key) => !["view", "issue", "administration", "filter", "notification"].includes(key) || query.getAll(key).length !== 1)) return null;
  const view = query.get("view") ?? "current";
  if (view !== "current" && view !== "participating" && view !== "notifications") return null;
  if (view !== "notifications" && (query.has("filter") || query.has("notification"))) return null;
  const notificationView = query.get("filter") ?? "unread";
  if (!isCollaborationNotificationView(notificationView)) return null;
  const notificationId = query.get("notification");
  if (notificationId !== null && !uuid.test(notificationId)) return null;
  const issueId = query.get("issue");
  if (issueId !== null && !uuid.test(issueId)) return null;
  if (query.has("administration")) {
    if (query.get("administration") !== "collaboration" || view !== "current" || url.hash) return null;
    return { view, administration: true, issueId: issueId?.toLowerCase() ?? null, commentId: null };
  }
  const target = { issueId: issueId?.toLowerCase() ?? null, commentId: null };
  const destination: AccountDestination = view === "notifications"
    ? { view, notificationView, notificationId: notificationId?.toLowerCase() ?? null, ...target }
    : { view, ...target };
  if (url.hash === "#description" || url.hash === "#memory") {
    return issueId ? { ...destination, anchor: url.hash === "#memory" ? "memory" : "description" } : null;
  }
  if (url.hash.startsWith("#memory-v")) {
    const version = url.hash.slice("#memory-v".length);
    return issueId && memoryVersion(version) ? { ...destination, memoryVersion: version } : null;
  }
  if (url.hash.startsWith("#run-")) {
    const runId = url.hash.slice("#run-".length);
    return issueId && uuid.test(runId) ? { ...destination, runId: runId.toLowerCase() } : null;
  }
  if (url.hash.startsWith("#delivery-")) {
    const submissionId = url.hash.slice("#delivery-".length);
    return issueId && uuid.test(submissionId) ? { ...destination, submissionId: submissionId.toLowerCase() } : null;
  }
  const commentId = url.hash ? url.hash.slice("#comment-".length) : null;
  if (url.hash && (!issueId || !url.hash.startsWith("#comment-") || !uuid.test(commentId ?? ""))) return null;
  return { ...destination, commentId: commentId?.toLowerCase() ?? null };
}

export function accountPath(input: Partial<AccountDestination> = {}): string {
  if ([input.runId, input.submissionId, input.commentId, input.anchor, input.memoryVersion].filter(Boolean).length > 1) throw new TypeError("Invalid account destination");
  const query = new URLSearchParams({ view: input.view ?? "current" });
  if (input.view === "notifications") {
    query.set("filter", input.notificationView ?? "unread");
    if (input.notificationId) query.set("notification", input.notificationId.toLowerCase());
  }
  if (input.administration) query.set("administration", "collaboration");
  if (input.issueId) query.set("issue", input.issueId.toLowerCase());
  const hash = input.submissionId ? `#delivery-${input.submissionId.toLowerCase()}` : input.runId ? `#run-${input.runId.toLowerCase()}`
    : input.commentId ? `#comment-${input.commentId.toLowerCase()}`
      : input.memoryVersion ? `#memory-v${input.memoryVersion}` : input.anchor ? `#${input.anchor}` : "";
  const path = `/account?${query}${hash}`;
  if (!parseAccountPath(path)) throw new TypeError("Invalid account destination");
  return path;
}

/** One tab, but independent scroll/view state for every selected issue and list. */
export function accountRouteKey(path: string): string | null {
  const parsed = parseAccountPath(path);
  if (!parsed) return null;
  return parsed.view === "notifications"
    ? accountPath({ view: parsed.view, notificationView: parsed.notificationView, notificationId: parsed.notificationId, issueId: parsed.issueId })
    : accountPath({ view: parsed.view, issueId: parsed.issueId, administration: parsed.administration });
}
