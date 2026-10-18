// @vitest-environment node
import { describe, expect, it } from "vitest";
import { accountPath, parseAccountPath, accountRouteKey } from "./account";

const issueId = "00000000-0000-4000-8000-000000000001";
const commentId = "00000000-0000-4000-8000-000000000002";

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
});
