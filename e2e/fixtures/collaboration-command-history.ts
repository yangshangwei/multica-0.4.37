import { commentMenuItem } from "./collaboration-comment-menu";
import { expect, type Locator, type Page } from "@playwright/test";
import type { CollaborationCommandHistory, CollaborationCommandHistoryEntry } from "../../packages/core/collaboration/command-history-schema";
import { resourceOfferAccountPath, type CollaborationResourceOfferJourney } from "./collaboration-resource-offers";

export interface CommandHistoryJourneyAdapter {
  navigate(page: Page, path: string): Promise<void>;
  capture?(stage: "cancelled" | "deleted", page: Page, focus: Locator): Promise<void>;
  inspectDeletion?(stage: "before-open" | "confirmation" | "failed", page: Page, comment: Locator): Promise<void>;
}

async function readEntry(fixture: CollaborationResourceOfferJourney, commandId: string) {
  const page = await fixture.handler.requestJSON<CollaborationCommandHistory>(`/api/collaboration/issues/${fixture.issueId}/commands?limit=100`);
  expect(page).toMatchObject({ protocol_version: 1, issue_id: fixture.issueId, next_cursor: null });
  const entry = page.items.find((item) => item.id === commandId);
  if (!entry) throw new Error("The exact command is missing from its history");
  return entry;
}

export async function showCancelledCommand(page: Page, fixture: CollaborationResourceOfferJourney, entry: CollaborationCommandHistoryEntry) {
  await expect(page.getByRole("heading", { name: fixture.title, exact: true })).toBeVisible();
  const toggle = page.getByRole("button", { name: "Command history", exact: true });
  if (await toggle.getAttribute("aria-expanded") !== "true") await toggle.click();
  const history = page.getByRole("region", { name: "Command history", exact: true });
  const row = history.locator(`[data-command-id="${entry.id}"]`);
  await expect(row.getByText("Cancelled", { exact: true })).toBeVisible();
  await expect(row.getByText("Cancelled by a participant", { exact: true })).toBeVisible();
  await expect(row).toContainText("Offer handler H");
  await expect(row).toContainText(fixture.agentName);
  await expect(row.locator(`time[datetime="${entry.cancellation!.created_at}"]`)).toBeVisible();
  await expect(row.locator("details")).not.toHaveAttribute("open", "");
  const originalLink = row.getByRole("link", { name: "Open original comment", exact: true });
  const appUrl = await page.evaluate(() => {
    const runtime = (window as unknown as { desktopAPI?: { runtimeConfig: { ok: boolean; config?: { appUrl: string } } } }).desktopAPI?.runtimeConfig;
    return runtime?.ok ? runtime.config!.appUrl : location.origin;
  });
  const href = await originalLink.getAttribute("href");
  expect(href).not.toBeNull();
  expect(new URL(href!, appUrl).href).toBe(new URL(`${resourceOfferAccountPath(fixture)}#comment-${entry.comment_id}`, appUrl).href);
  return row;
}

/** Cancels the real waiting input produced by the resource-offer journey.
 * Comment edits/deletion go through the UI. The entire immutable record is
 * compared through the real API, then read by a second independent account. */
export async function exerciseCollaborationCommandHistory(
  authorPage: Page, readerPage: Page, fixture: CollaborationResourceOfferJourney,
  identities: { command_id: string | null; comment_id: string; target_id: string },
  adapter: CommandHistoryJourneyAdapter,
) {
  if (!identities.command_id) throw new Error("The journey did not produce a command");
  const root = `/api/collaboration/issues/${fixture.issueId}`;
  const original = await readEntry(fixture, identities.command_id);
  expect(original).toMatchObject({ state: "waiting", cancellation: null, comment_id: identities.comment_id,
    target_id: identities.target_id, author_user_id: fixture.handlerId, attachment_ids: [] });
  expect(original.content).toContain("mention://all/all");
  expect(original.content).toContain(`mention://agent/${fixture.agentId}`);

  const responsePending = authorPage.waitForResponse((response) => new URL(response.url()).pathname === `${root}/commands/${original.id}/cancel`
    && response.request().method() === "POST");
  await authorPage.getByRole("region", { name: "AI execution", exact: true })
    .getByRole("button", { name: `Cancel waiting input ${original.seq}`, exact: true }).click();
  const response = await responsePending;
  expect(response.status()).toBe(200);
  const key = response.request().headers()["idempotency-key"];
  expect(key).toMatch(/^[0-9a-f-]{36}$/i);
  const receipt = await response.json();
  expect(receipt).toMatchObject({ issue_id: fixture.issueId, request_id: key, action: "cancel_command",
    code: "command_cancelled", command_id: original.id, target_id: original.target_id, run_id: null, run_settled: false });
  expect(await fixture.handler.requestJSON(`${root}/commands/operations/${key}`)).toEqual(receipt);
  await expect(authorPage.getByText("Waiting input cancelled.", { exact: true })).toBeVisible();
  await expect(authorPage.getByRole("button", { name: `Cancel waiting input ${original.seq}`, exact: true })).toHaveCount(0);
  const cancelled = await readEntry(fixture, original.id);
  expect(cancelled).toMatchObject({ ...original, state: "cancelled", reason: "cancelled_by_participant",
    cancellation: { kind: "manual", actor_user_id: fixture.handlerId, actor: { user_id: fixture.handlerId, name: "Offer handler H" } } });
  expect(cancelled.cancellation!.event_id).not.toBe(cancelled.input_event_id);
  expect(BigInt(cancelled.cancellation!.event_seq!)).toBeGreaterThan(BigInt(cancelled.input_event_seq!));
  const row = await showCancelledCommand(authorPage, fixture, cancelled);
  await adapter.capture?.("cancelled", authorPage, row);

  const comment = authorPage.locator(`[id="comment-${identities.comment_id}"]`);
  const edited = `Edited discussion after cancelling ${original.id}`;
  await (await commentMenuItem(comment, "Edit comment")).click();
  await comment.getByLabel("Edit comment text", { exact: true }).fill(edited);
  const editPending = authorPage.waitForResponse((result) => new URL(result.url()).pathname === `${root}/comments/${identities.comment_id}`
    && result.request().method() === "PATCH");
  await comment.getByRole("button", { name: "Save comment", exact: true }).click();
  expect((await editPending).status()).toBe(200);
  await expect(comment.getByText(edited, { exact: true })).toBeVisible();
  expect(await readEntry(fixture, original.id)).toEqual(cancelled);
  await expect(row).not.toContainText(edited);

  await adapter.inspectDeletion?.("before-open", authorPage, comment);
  let deletionUsedLatest = false;
  try {
    await (await commentMenuItem(comment, "Delete comment")).click();
    await adapter.inspectDeletion?.("confirmation", authorPage, comment);
    const confirm = comment.getByRole("button", { name: "Confirm deletion", exact: true });
    try {
      await confirm.click({ trial: true, timeout: 5_000 });
    } catch (error) {
      // A delayed comment refresh can invalidate the opened confirmation.
      // Review the actual retained text before explicitly adopting its version.
      const latest = comment.getByRole("button", { name: "Use latest version", exact: true });
      if (!await latest.isVisible()) throw error;
      await expect(comment.getByText(edited, { exact: true })).toBeVisible();
      await expect(confirm).toBeDisabled();
      await latest.click();
      deletionUsedLatest = true;
      await expect(latest).toHaveCount(0);
      await expect(confirm).toBeEnabled();
    }
    // Observe the response error immediately so a precondition/click failure
    // cannot become an unhandled timeout that hides the actual UI state.
    const deletePending = authorPage.waitForResponse((result) => new URL(result.url()).pathname === `${root}/comments/${identities.comment_id}`
      && result.request().method() === "DELETE").then((response) => ({ response }), (error: unknown) => ({ error }));
    await confirm.click();
    const deletion = await deletePending;
    if ("error" in deletion) throw deletion.error;
    expect(deletion.response.status()).toBe(200);
  } catch (error) {
    await adapter.inspectDeletion?.("failed", authorPage, comment);
    throw error;
  }
  await expect(comment.getByText("Comment deleted", { exact: true })).toBeVisible();
  expect(await readEntry(fixture, original.id)).toEqual(cancelled);

  await authorPage.reload();
  await expect(authorPage.getByRole("button", { name: "Command history", exact: true })).toHaveAttribute("aria-expanded", "false");
  const restored = await showCancelledCommand(authorPage, fixture, cancelled);
  await adapter.capture?.("deleted", authorPage, restored);
  await adapter.navigate(readerPage, resourceOfferAccountPath(fixture));
  await showCancelledCommand(readerPage, fixture, cancelled);
  const otherAccount = await fixture.owner.requestJSON<CollaborationCommandHistory>(`${root}/commands?limit=100`);
  expect(otherAccount.items.find((item) => item.id === original.id)).toEqual(cancelled);
  const issue = await fixture.handler.requestJSON<{ session: { queue_state: string }; execution: { runs: unknown[]; groups: unknown[] } }>(root);
  expect(issue.session.queue_state).toBe("paused");
  expect(issue.execution).toMatchObject({ runs: [], groups: [] });
  expect(await fixture.owner.countIssueDispatches(fixture.issueId)).toBe(0);
  return { issue_id: fixture.issueId, command_id: original.id, comment_id: identities.comment_id, deletion_used_latest: deletionUsedLatest,
    input_event_id: original.input_event_id, cancellation_event_id: cancelled.cancellation!.event_id,
    cancellation_request_id: key, entry: cancelled };
}
