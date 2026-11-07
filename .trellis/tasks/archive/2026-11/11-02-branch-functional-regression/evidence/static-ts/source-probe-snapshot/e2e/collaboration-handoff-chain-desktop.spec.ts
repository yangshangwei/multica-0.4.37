import { createHash } from "node:crypto";
import { mkdtempSync, readFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { test, expect } from "@playwright/test";
import type { CollaborationIssue } from "@multica/core/collaboration";
import { deepLink, openDesktop, nativeScreenshot, type NativeHarness } from "./fixtures/collaboration-native";
import { seedOfflineHandoff, uiWorkflow, type HandoffActor, type HandoffSession } from "./fixtures/collaboration-offline-handoff";
import { directedFormThroughUI } from "./fixtures/collaboration-handoff-chain";

test.use({ actionTimeout: 15_000, trace: "off", screenshot: "off" });

// The Web directed-chain suite owns the full business transition matrix. This
// native regression owns requester/note/status visibility in the receiving form.
// eslint-disable-next-line no-empty-pattern -- Electron owns its browser contexts.
test("Electron information return shows the original requester and note before restoring responsibility", async ({}, info) => {
  test.setTimeout(120_000);
  test.skip(process.env.E2E_PASSWORD_AUTH !== "1" || !process.env.COLLABORATION_ELECTRON_RENDERER_URL,
    "Requires task-owned password services and the built desktop renderer");
  const fixture = await seedOfflineHandoff();
  const natives: Awaited<ReturnType<typeof openDesktop>>[] = [];
  const profiles: string[] = [];
  const open = async (actor: HandoffActor) => {
    const token = await actor.login();
    const profile = mkdtempSync(join(tmpdir(), "multica-directed-handoff-"));
    profiles.push(profile);
    const native = await openDesktop(profile, { token, deepLink: `multica://account${fixture.accountPath.slice("/account".length)}` });
    natives.push(native);
    // The shared directed-form helper independently reads the exact response
    // URL through page.request. Authenticate that browser-owned API context too.
    await actor.authenticateWeb(native.page);
    await native.start();
    await expect(native.page.getByRole("heading", { name: fixture.title, exact: true })).toBeVisible();
    const session: HandoffSession = { page: native.page, navigate: (next) => deepLink(native.app, next), close: () => native.app.close() };
    return { native, session };
  };
  const path = `/api/collaboration/issues/${fixture.issueId}`;
  try {
    const creator = await open(fixture.creator);
    const transferred = await uiWorkflow(creator.session, fixture, fixture.creator, "transfer", fixture.recipient,
      "Please implement the configured retry provider.");
    const handler = await open(fixture.recipient);
    const requestNote = "Which configured provider should the desktop retry use?";
    const requested = await directedFormThroughUI(handler.native.page, fixture.issueId, "request_information", requestNote,
      { id: fixture.creator.id, name: fixture.creator.name });
    const returned = await directedFormThroughUI(creator.native.page, fixture.issueId, "return_information", "Use the existing configured provider.");
    expect(returned.baseline.current_information_request).toMatchObject({
      requester: { user_id: fixture.recipient.id, name: fixture.recipient.name },
      recipient: { user_id: fixture.creator.id }, note: requestNote,
    });
    expect(returned.receipt.code).toBe("information_returned");
    expect(returned.selectedStatus).toBe(transferred.current_issue.status);
    const current = await fixture.creator.client.requestJSON<CollaborationIssue>(path);
    expect(current).toMatchObject({ handler: { user_id: fixture.recipient.id },
      current_information_request: null, status: returned.selectedStatus });
    expect(await fixture.creator.client.countIssueDispatches(fixture.issueId)).toBe(0);
    for (const native of natives) {
      expect(native.errors).toEqual([]);
      expect(native.accountHeaders.every((headers) => !headers["x-workspace-id"] && !headers["x-workspace-slug"])).toBe(true);
      expect(await native.app.evaluate(() => (globalThis as unknown as { desktopAccountAcceptance: NativeHarness })
        .desktopAccountAcceptance.evidence.daemonStarts)).toBe(0);
    }
    const nativeRoot = resolve(process.env.COLLABORATION_ELECTRON_SOURCE_ROOT ?? ".");
    const sources = ["e2e/fixtures/collaboration-account-electron.mjs", "apps/desktop/out/preload/index.js", "apps/desktop/out/renderer/index.html"]
      .map((file) => ({ file, sha256: createHash("sha256").update(readFileSync(join(nativeRoot, file))).digest("hex") }));
    await nativeScreenshot(creator.native.app, creator.native.page, info.outputPath("desktop-information-return.png"));
    await info.attach("native-information-return-evidence", { contentType: "application/json", body: JSON.stringify({
      issue_id: fixture.issueId, requester: fixture.recipient.id, recipient: fixture.creator.id,
      request_note: requestNote, requested: requested.receipt, returned: returned.receipt,
      selected_status: returned.selectedStatus, final_handler: current.handler.user_id,
      native_platform: process.platform, renderer: process.env.COLLABORATION_ELECTRON_RENDERER_URL,
      provider_account_access: false, native_sources: sources,
    }, null, 2) });
  } finally {
    for (const native of natives) await native.app.close().catch(() => undefined);
    for (const profile of profiles) rmSync(profile, { recursive: true, force: true });
  }
});
