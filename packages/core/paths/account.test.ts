// @vitest-environment node
import { describe, expect, it } from "vitest";
import { accountPath, parseAccountPath, accountRouteKey } from "./account";

const issueId = "00000000-0000-4000-8000-000000000001";
const commentId = "00000000-0000-4000-8000-000000000002";
const runId = "00000000-0000-4000-8000-0000000000ab";
const notificationId = "00000000-0000-4000-8000-0000000000cd";

describe("account destinations", () => {
  it("defaults a notifications destination to the unread list without changing the workbench default", () => {
    expect(parseAccountPath("/account?view=notifications")).toEqual({
      view: "notifications", notificationView: "unread", notificationId: null, issueId: null, commentId: null,
    });
    expect(accountPath({ view: "notifications" })).toBe("/account?view=notifications&filter=unread");
    expect(accountPath()).toBe("/account?view=current");
  });

  it.each(["unread", "all", "archived"] as const)("round-trips the %s notification view and a content-free selection", (notificationView) => {
    const path = accountPath({ view: "notifications", notificationView, notificationId });
    expect(path).toBe(`/account?view=notifications&filter=${notificationView}&notification=${notificationId}`);
    const destination = parseAccountPath(path);
    expect(destination).toEqual({ view: "notifications", notificationView, notificationId, issueId: null, commentId: null });
    expect(accountPath(destination!)).toBe(path);
  });

  it.each([
    [{ commentId }, `#comment-${commentId}`],
    [{ runId }, `#run-${runId}`],
    [{ submissionId: runId }, `#delivery-${runId}`],
    [{ anchor: "description" as const }, "#description"],
    [{ anchor: "memory" as const }, "#memory"],
    [{ memoryVersion: "9007199254740993" }, "#memory-v9007199254740993"],
  ] as const)("preserves an exact notification content target %j", (target, hash) => {
    const path = accountPath({ view: "notifications", notificationView: "all", notificationId, issueId, ...target });
    expect(path).toBe(`/account?view=notifications&filter=all&notification=${notificationId}&issue=${issueId}${hash}`);
    const destination = parseAccountPath(path);
    expect(destination).toEqual({ view: "notifications", notificationView: "all", notificationId, issueId, commentId: null, ...target });
    expect(accountPath(destination!)).toBe(path);
    expect(accountRouteKey(path)).toBe(`/account?view=notifications&filter=all&notification=${notificationId}&issue=${issueId}`);
    // Copy/open content leaves the page deliberately, preserving its exact target.
    expect(accountPath({ ...destination!, view: "participating" })).toBe(`/account?view=participating&issue=${issueId}${hash}`);
  });

  it("normalizes notification UUIDs and retains separate filter/selection/issue mementos", () => {
    const path = `/account?view=notifications&filter=archived&notification=${notificationId.toUpperCase()}&issue=${issueId}#run-${runId.toUpperCase()}`;
    const canonical = `/account?view=notifications&filter=archived&notification=${notificationId}&issue=${issueId}#run-${runId}`;
    expect(accountPath(parseAccountPath(path)!)).toBe(canonical);
    const key = accountRouteKey(canonical);
    expect(key).not.toBe(accountRouteKey(canonical.replace("archived", "all")));
    expect(key).not.toBe(accountRouteKey(canonical.replace(notificationId, commentId)));
    expect(key).not.toBe(accountRouteKey(canonical.replace(issueId, commentId)));
    expect(key).toBe(accountRouteKey(canonical.replace(`#run-${runId}`, `#comment-${commentId}`)));
  });

  it.each([
    "view=notifications&filter=unknown", "view=notifications&filter=",
    "view=notifications&filter=unread&filter=all", "view=notifications&view=notifications",
    `view=notifications&notification=${notificationId}&notification=${notificationId}`,
    "view=notifications&notification=SRC-1", "view=notifications&notification=",
    `view=notifications&issue=${issueId}&issue=${issueId}`, "view=notifications&issue=SRC-1",
    "view=notifications&administration=collaboration", "view=notifications&other=unread",
    "view=current&filter=unread", `view=participating&notification=${notificationId}`,
    "filter=unread", `notification=${notificationId}`,
    `view=notifications#comment-${commentId}`, `view=notifications&issue=${issueId}#run-invalid`,
    `view=notifications&issue=${issueId}#run-${runId}#comment-${commentId}`,
  ])("rejects malformed or misplaced notification parameters: %s", (query) => {
    expect(parseAccountPath(`/account?${query}`)).toBeNull();
  });

  it("rejects malformed or ambiguous notification targets when building links", () => {
    expect(() => accountPath({ view: "notifications", notificationId: "SRC-1" })).toThrow(TypeError);
    expect(() => accountPath({ view: "notifications", notificationId, runId })).toThrow(TypeError);
    expect(() => accountPath({ view: "notifications", notificationId, issueId, runId, commentId })).toThrow(TypeError);
  });

  it("round-trips an exact delivery submission in the existing account route", () => {
    const path = accountPath({ view: "participating", issueId, submissionId: runId });
    expect(path).toBe(`/account?view=participating&issue=${issueId}#delivery-${runId}`);
    expect(parseAccountPath(path)?.submissionId).toBe(runId);
    expect(accountPath(parseAccountPath(path)!)).toBe(path);
    expect(accountRouteKey(path)).toBe(accountPath({ view: "participating", issueId }));
    expect(parseAccountPath(`/account#delivery-${runId}`)).toBeNull();
    expect(parseAccountPath(`/account?issue=${issueId}#delivery-not-a-uuid`)).toBeNull();
    expect(() => accountPath({ issueId, runId, submissionId: runId })).toThrow();
  });
  it("preserves the view, exact issue identity and comment anchor across platforms", () => {
    const path = accountPath({ view: "participating", issueId, commentId });
    expect(path).toBe(`/account?view=participating&issue=${issueId}#comment-${commentId}`);
    expect(parseAccountPath(path)).toEqual({ view: "participating", issueId, commentId });
  });

  it("normalizes a bare account page and UUID casing without accepting workspace or foreign URLs", () => {
    expect(parseAccountPath("/account")).toEqual({ view: "current", issueId: null, commentId: null });
    expect(parseAccountPath(accountPath({ issueId: issueId.toUpperCase() }))?.issueId).toBe(issueId);
    for (const invalid of ["https://example.org/account", "//example.org/account", "/account/extra", "/space/account", "/account?view=archived", "/account?issue=SRC-1", `/account?issue=${issueId}&issue=${commentId}`, `/account#comment-${commentId}`]) {
      expect(parseAccountPath(invalid), invalid).toBeNull();
    }
  });

  it("keeps mementos separate for views and selected issues, excluding the comment anchor", () => {
    expect(accountRouteKey(accountPath({ issueId, commentId }))).toBe(accountPath({ issueId }));
    expect(accountRouteKey(accountPath({ issueId, view: "participating" }))).not.toBe(accountRouteKey(accountPath({ issueId })));
    expect(accountRouteKey(`/account?view=current&issue=${commentId}`)).not.toBe(accountRouteKey(accountPath({ issueId })));
  });

  it("locates body mentions without turning their edit-event UUID into a comment", () => {
    const path = accountPath({ view: "participating", issueId, anchor: "description" });
    expect(path).toBe(`/account?view=participating&issue=${issueId}#description`);
    expect(parseAccountPath(path)?.anchor).toBe("description");
    expect(accountRouteKey(path)).toBe(accountPath({ view: "participating", issueId }));
    expect(parseAccountPath("/account#description")).toBeNull();
  });

  it("retains an exact recorded-run anchor through account parsing and native sanitization", () => {
    const path = `/account?view=participating&issue=${issueId}#run-${runId}`;
    expect(accountPath({ view: "participating", issueId, runId: runId.toUpperCase() })).toBe(path);
    const parsed = parseAccountPath(path);
    expect(parsed).toEqual({ view: "participating", issueId, commentId: null, runId });
    expect(accountPath(parsed!)).toBe(path);
    expect(accountRouteKey(path)).toBe(accountPath({ view: "participating", issueId }));
    expect(parseAccountPath(path.replace(runId, runId.toUpperCase()))?.runId).toBe(runId);
  });

  it("rejects invalid or ambiguous run destinations", () => {
    for (const invalid of [`/account#run-${runId}`, `/account?issue=${issueId}#run-unknown`,
      `/account?issue=${issueId}#run-${runId}#comment-${commentId}`, `/account?issue=${issueId}#run-${runId}%2Fother`]) {
      expect(parseAccountPath(invalid), invalid).toBeNull();
    }
    expect(() => accountPath({ runId })).toThrow();
    expect(() => accountPath({ issueId, runId, commentId })).toThrow();
    expect(() => accountPath({ issueId, runId, anchor: "description" })).toThrow();
  });

  it("round-trips exact shared-context versions and keeps their scroll state under the issue", () => {
    const root = accountPath({ view: "participating", issueId, anchor: "memory" });
    expect(parseAccountPath(root)).toEqual({ view: "participating", issueId, commentId: null, anchor: "memory" });
    const version = "9007199254740993";
    const path = accountPath({ issueId, memoryVersion: version });
    expect(path).toBe(`/account?view=current&issue=${issueId}#memory-v${version}`);
    expect(parseAccountPath(path)?.memoryVersion).toBe(version);
    expect(accountRouteKey(path)).toBe(accountPath({ issueId }));
    for (const suffix of ["0", "01", "-1", "1e3", "9223372036854775808", "1%2Fother"]) {
      expect(parseAccountPath(`/account?issue=${issueId}#memory-v${suffix}`)).toBeNull();
    }
    expect(parseAccountPath("/account#memory")).toBeNull();
    expect(() => accountPath({ issueId, runId, memoryVersion: "1" })).toThrow();
    expect(() => accountPath({ issueId, commentId, anchor: "memory" })).toThrow();
    expect(() => accountPath({ administration: true, issueId, memoryVersion: "1" })).toThrow();
  });

  it("retains the limited administration destination as its own account-tab view", () => {
    const destination = { administration: true as const, issueId };
    const path = `/account?view=current&administration=collaboration&issue=${issueId}`;
    expect(accountPath(destination)).toBe(path);
    expect(parseAccountPath(path)).toEqual({ view: "current", administration: true, issueId, commentId: null });
    expect(accountRouteKey(path)).toBe(path);
    expect(accountRouteKey(path)).not.toBe(accountRouteKey(accountPath({ issueId })));
    expect(accountPath({ administration: true })).toBe("/account?view=current&administration=collaboration");
  });

  it("keeps administrative metadata links separate from ordinary content anchors and views", () => {
    for (const query of ["administration=all", "administration=collaboration&view=participating",
      "administration=collaboration&administration=collaboration", `administration=collaboration&issue=${issueId}#run-${runId}`,
      `administration=collaboration&issue=${issueId}#comment-${commentId}`, `administration=collaboration&issue=${issueId}#description`]) {
      expect(parseAccountPath(`/account?${query}`)).toBeNull();
    }
  });
});
