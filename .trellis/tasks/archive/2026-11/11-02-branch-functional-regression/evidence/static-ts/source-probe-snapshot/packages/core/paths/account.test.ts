// @vitest-environment node
import { describe, expect, it } from "vitest";
import { accountPath, parseAccountPath, accountRouteKey } from "./account";

const issueId = "00000000-0000-4000-8000-000000000001";
const commentId = "00000000-0000-4000-8000-000000000002";
const runId = "00000000-0000-4000-8000-0000000000ab";

describe("account destinations", () => {
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
