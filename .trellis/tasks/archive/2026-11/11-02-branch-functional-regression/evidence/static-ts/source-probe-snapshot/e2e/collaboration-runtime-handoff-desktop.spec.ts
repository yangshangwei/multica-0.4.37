import { createHash } from "node:crypto";
import { mkdtempSync, readFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { expect, test } from "@playwright/test";
import { deepLink, nativeScreenshot, openDesktop, type NativeHarness } from "./fixtures/collaboration-native";
import { assertRuntimeHandoff, recoverLongComment, seedRuntimeHandoff } from "./fixtures/collaboration-runtime-handoff";

test.use({ actionTimeout: 15_000, trace: "off", screenshot: "off" });

// Web owns the paging/removal matrix. This test owns the actual native entry,
// shared handoff view, unknown recording tail, and one full-text recovery.
// eslint-disable-next-line no-empty-pattern -- Electron owns its browser context.
test("Electron deep link opens retained handoff context and honestly marks an unknown recording tail", async ({}, info) => {
  test.setTimeout(120_000);
  test.skip(process.env.E2E_PASSWORD_AUTH !== "1", "Requires task-owned password production services");
  const renderer = process.env.COLLABORATION_ELECTRON_RENDERER_URL;
  if (!renderer || !["localhost", "127.0.0.1"].includes(new URL(renderer).hostname)) throw new Error("Native acceptance requires the built task-owned renderer");
  const fixture = await seedRuntimeHandoff({ observedThrough: null });
  const profile = mkdtempSync(join(tmpdir(), "multica-runtime-handoff-"));
  let native: Awaited<ReturnType<typeof openDesktop>> | undefined;
  try {
    native = await openDesktop(profile, { token: await fixture.recipient.login() });
    await native.start();
    await deepLink(native.app, fixture.path);
    const { handoff } = await assertRuntimeHandoff(native.page, fixture);
    await handoff.scrollIntoViewIfNeeded();
    const screenshot = info.outputPath("desktop-runtime-handoff.png");
    await nativeScreenshot(native.app, native.page, screenshot);
    await info.attach("desktop-runtime-handoff", { path: screenshot, contentType: "image/png" });
    await recoverLongComment(native.page, fixture);
    expect(native.errors).toEqual([]);
    expect(native.accountHeaders.length).toBeGreaterThan(0);
    expect(native.accountHeaders.every(headers => !headers["x-workspace-id"] && !headers["x-workspace-slug"])).toBe(true);
    const nativeEvidence = await native.app.evaluate(() => (globalThis as unknown as { desktopAccountAcceptance: NativeHarness }).desktopAccountAcceptance.evidence);
    expect(nativeEvidence.daemonStarts).toBe(0);
    const sourceRoot = resolve(process.env.COLLABORATION_ELECTRON_SOURCE_ROOT ?? ".");
    const sources = ["e2e/fixtures/collaboration-account-electron.mjs", "apps/desktop/out/preload/index.js", "apps/desktop/out/renderer/index.html"]
      .map(file => ({ file, sha256: createHash("sha256").update(readFileSync(join(sourceRoot, file))).digest("hex") }));
    await info.attach("runtime-handoff-native-evidence", { contentType: "application/json", body: JSON.stringify({
      issue_id: fixture.issueId, run_id: fixture.runId, platform: process.platform, renderer, sources,
      handoff: fixture.handoff, recording: fixture.records.recording, native: nativeEvidence,
      history_origin: "seeded retained events; ingestion verified separately in Go", provider_account_access: false,
    }, null, 2) });
  } finally {
    try { await native?.app.close(); }
    finally { rmSync(profile, { recursive: true, force: true }); }
  }
});
