// @vitest-environment node
import { describe, expect, it } from "vitest";
import { accountPath, parseAccountPath } from "@multica/core/paths/account";
import {
  encodeIssueWindowArgument,
  parseIssueWindowPath,
  parseIssueWindowRequest,
  readDesktopWindowContext,
} from "./issue-window";

describe("issue window request", () => {
  it("keeps a selected notification list in the main account tab and permits its canonical content link", () => {
    const issueId = "11111111-1111-4111-8111-111111111111";
    const notificationId = "22222222-2222-4222-8222-222222222222";
    const path = accountPath({ view: "notifications", notificationView: "unread", notificationId, issueId, anchor: "description" });
    const destination = parseAccountPath(path);
    expect(destination).not.toBeNull();
    expect(parseIssueWindowPath(path)).toBeNull();
    expect(parseIssueWindowRequest({ path, title: "Collaboration notifications" })).toBeNull();
    expect(() => encodeIssueWindowArgument({ path, title: "Collaboration notifications" })).toThrow();
    const contentPath = accountPath({ ...destination!, view: "participating" });
    expect(parseIssueWindowPath(contentPath)).toEqual({ scope: "account", issueId, path: `/account?view=participating&issue=${issueId}#description` });
  });

  it("round-trips an account issue without inventing a workspace", () => {
    const path = "/account?view=participating&issue=11111111-1111-4111-8111-111111111111#description";
    const expected = {
      kind: "issue",
      scope: "account",
      issueId: "11111111-1111-4111-8111-111111111111",
      path,
      title: "Shared task",
    };

    expect(parseIssueWindowRequest({ path, title: "Shared task" })).toEqual(expected);
    expect(readDesktopWindowContext([encodeIssueWindowArgument({ path, title: "Shared task" })]))
      .toEqual(expected);
  });

  it.each([
    "/account",
    "/account?view=participating",
    "/account?issue=bad",
    "/account?issue=11111111-1111-4111-8111-111111111111&token=secret",
  ])("rejects account list or invalid issue popouts: %s", (path) => {
    expect(parseIssueWindowRequest({ path, title: "Shared task" })).toBeNull();
  });

  it("accepts and canonicalizes a workspace-scoped issue detail path", () => {
    expect(
      parseIssueWindowRequest({
        path: "/acme/issues/issue-123?comment=comment-1#activity",
        title: "  MUL-1: Fix tabs  ",
      }),
    ).toEqual({
      kind: "issue",
      scope: "workspace",
      path: "/acme/issues/issue-123?comment=comment-1#activity",
      title: "MUL-1: Fix tabs",
      workspaceSlug: "acme",
      issueId: "issue-123",
    });
  });

  it.each([
    "/acme/issues",
    "/acme/projects/project-1",
    "/acme/issues/issue-1/attachments",
    "https://example.com/acme/issues/issue-1",
    "//example.com/acme/issues/issue-1",
    "/acme/issues/issue%2F1",
    "/UPPERCASE/issues/issue-1",
  ])("rejects non-issue or unsafe paths: %s", (path) => {
    expect(parseIssueWindowPath(path)).toBeNull();
  });

  it("uses a safe fallback title and bounds renderer-controlled titles", () => {
    expect(
      parseIssueWindowRequest({ path: "/acme/issues/issue-1", title: "  " }),
    ).toMatchObject({ title: "Issue" });
    expect(
      parseIssueWindowRequest({
        path: "/acme/issues/issue-1",
        title: "x".repeat(400),
      })?.title,
    ).toHaveLength(256);
  });

  it("round-trips a validated request through Electron additionalArguments", () => {
    const argument = encodeIssueWindowArgument({
      path: "/acme/issues/issue-1",
      title: "MUL-1: Fix tabs",
    });

    expect(readDesktopWindowContext(["electron", argument])).toEqual({
      kind: "issue",
      scope: "workspace",
      path: "/acme/issues/issue-1",
      title: "MUL-1: Fix tabs",
      workspaceSlug: "acme",
      issueId: "issue-1",
    });
  });

  it("falls back to the main window for malformed launch arguments", () => {
    expect(
      readDesktopWindowContext(["electron", "--multica-issue-window=%7Bbad"]),
    ).toEqual({ kind: "main" });
  });
});
