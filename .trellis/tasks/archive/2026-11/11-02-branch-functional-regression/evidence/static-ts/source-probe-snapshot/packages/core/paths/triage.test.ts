// @vitest-environment node
import { expect, it } from "vitest";
import { paths } from "./paths";
import { pageForSegment, WORKSPACE_PAGES } from "./route-icons";
it("registers a shared workspace triage destination with a translated tab identity", () => {
  expect(paths.workspace("a b").triage()).toBe("/a%20b/triage");
  expect(pageForSegment("triage")).toBe("triage");
  expect(WORKSPACE_PAGES.triage.navKey).toBe("triage");
});
