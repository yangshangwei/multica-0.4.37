// @vitest-environment node
import { describe, expect, it } from "vitest";
import { parseAccountDeepLink, parseDesktopAccountPath } from "./account-navigation";

describe("native account destinations", () => {
  const path = "/account?view=participating&issue=11111111-1111-4111-8111-111111111111#description";
  it("preserves account issue/view/anchor from native deep links", () => {
    expect(parseAccountDeepLink(`multica://account${path.slice(8)}`)).toBe(path);
    expect(parseDesktopAccountPath(path)).toBe(path);
  });
  it.each(["multica://account?issue=bad", "multica://account/settings", "https://account?view=current", "multica://account?view=current&view=participating"])("rejects unsupported account deep links: %s", (url) => {
    expect(parseAccountDeepLink(url)).toBeNull();
  });
  it.each(["/acme/issues/1", "//account", "/account?issue=bad", "/account?view=current&token=secret"])("rejects non-account IPC paths: %s", (path) => {
    expect(parseDesktopAccountPath(path)).toBeNull();
  });
});
