/** Account routes are independent of workspace membership and identifiers. */
export type AccountView = "current" | "participating";
export interface AccountDestination {
  view: AccountView;
  issueId: string | null;
  commentId: string | null;
  runId?: string | null;
  anchor?: "description";
}

const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;

export function parseAccountPath(path: string): AccountDestination | null {
  if (!/^\/account(?:[?#]|$)/.test(path)) return null;
  const url = new URL(path, "https://account.invalid");
  if (url.pathname !== "/account" || url.origin !== "https://account.invalid") return null;
  const { searchParams: query } = url;
  if ([...query.keys()].some((key) => !["view", "issue"].includes(key) || query.getAll(key).length !== 1)) return null;
  const view = query.get("view") ?? "current";
  if (view !== "current" && view !== "participating") return null;
  const issueId = query.get("issue");
  if (issueId !== null && !uuid.test(issueId)) return null;
  if (url.hash === "#description") return issueId ? { view, issueId: issueId.toLowerCase(), commentId: null, anchor: "description" } : null;
  if (url.hash.startsWith("#run-")) {
    const runId = url.hash.slice("#run-".length);
    return issueId && uuid.test(runId) ? { view, issueId: issueId.toLowerCase(), commentId: null, runId: runId.toLowerCase() } : null;
  }
  const commentId = url.hash ? url.hash.slice("#comment-".length) : null;
  if (url.hash && (!issueId || !url.hash.startsWith("#comment-") || !uuid.test(commentId ?? ""))) return null;
  return { view, issueId: issueId?.toLowerCase() ?? null, commentId: commentId?.toLowerCase() ?? null };
}

export function accountPath(input: Partial<AccountDestination> = {}): string {
  if (input.runId && (input.commentId || input.anchor)) throw new TypeError("Invalid account destination");
  const query = new URLSearchParams({ view: input.view ?? "current" });
  if (input.issueId) query.set("issue", input.issueId.toLowerCase());
  const hash = input.runId ? `#run-${input.runId.toLowerCase()}`
    : input.commentId ? `#comment-${input.commentId.toLowerCase()}` : input.anchor === "description" ? "#description" : "";
  const path = `/account?${query}${hash}`;
  if (!parseAccountPath(path)) throw new TypeError("Invalid account destination");
  return path;
}

/** One tab, but independent scroll/view state for every selected issue and list. */
export function accountRouteKey(path: string): string | null {
  const parsed = parseAccountPath(path);
  return parsed ? accountPath({ view: parsed.view, issueId: parsed.issueId }) : null;
}
