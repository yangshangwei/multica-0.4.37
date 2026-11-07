// @vitest-environment node
import { describe, expect, it } from "vitest";
import {
  customRuntimeDocsHref,
  daemonRuntimesDocsHref,
} from "./runtime-docs";

// These links used to point at https://multica.ai/docs/<lang>/daemon-runtimes,
// which is unreachable on an intranet deployment. They now address the docs the
// connected server itself publishes.
//
// That the anchor below matches a real heading is NOT asserted here — a string
// compare against a string cannot know. See
// apps/docs/lib/docs-bundle/docs-anchor-parity.test.ts, which checks
// DOCS_ANCHORS against the generated bundle.
describe("runtime docs links", () => {
  it("addresses the daemon guide inside the current workspace", () => {
    expect(daemonRuntimesDocsHref("acme")).toBe("/acme/docs/daemon-runtimes");
  });

  it("adds the custom runtime section as a percent-encoded anchor", () => {
    expect(customRuntimeDocsHref("acme")).toBe(
      `/acme/docs/daemon-runtimes#${encodeURIComponent("自定义运行时配置")}`,
    );
  });

  it("no longer emits a public docs-site URL", () => {
    expect(daemonRuntimesDocsHref("acme")).not.toContain("multica.ai");
    expect(customRuntimeDocsHref("acme")).not.toContain("multica.ai");
  });

  // The runtime profiles dialog also opens from surfaces that are not under a
  // workspace route. Returning null there lets the caller drop the link rather
  // than throw and take the whole dialog down.
  it("returns null with no workspace in scope", () => {
    expect(daemonRuntimesDocsHref(null)).toBeNull();
    expect(customRuntimeDocsHref(null)).toBeNull();
  });
});
