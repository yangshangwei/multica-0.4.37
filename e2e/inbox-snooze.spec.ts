import { expect, test, type Locator } from "@playwright/test";
import { InboxSnoozeReceiptSchema } from "../packages/core/api/schemas";
import {
  SnoozeEvidence, chooseSnoozePreset, closeSnoozeJourney, createSnoozeActors, createSnoozeFixture,
  inboxRow, listSnoozeNotices, makeSnoozeDue, noticeFact, observeNativeTabs, openRowSnooze, openSnoozeSession,
  postSnoozeComment, requireSnoozeAllocation, seedSnoozeIssue, selectApiWorkspace, setSnoozeTheme, snoozePicker,
  snoozeStateFact, uiSnoozeResponse, type SnoozeFixture, type SnoozeIssue, type SnoozeSession,
} from "./fixtures/inbox-snooze";

// Core owns the time/schema/cache matrices; Go owns source replay and transaction races.
// These journeys prove the compiled shared controls, real persistence and personal transport.
test.use({ trace: "off", screenshot: "only-on-failure" });

let allocation: Awaited<ReturnType<typeof requireSnoozeAllocation>>;
let actors: Awaited<ReturnType<typeof createSnoozeActors>>;

const mainList = (session: SnoozeSession) => session.page.locator("[data-inbox-page]");
const snoozedEntry = (session: SnoozeSession) => mainList(session).getByRole("button", { name: /^Snoozed\s*\d*$/ });
const currentUndo = (session: SnoozeSession) => session.page.locator('[data-sonner-toast][data-front="true"]').getByRole("button", { name: "Undo", exact: true });

async function customTime(session: SnoozeSession, issue: SnoozeIssue, change: boolean, days = 2) {
  await openRowSnooze(session, issue, change);
  await snoozePicker(session.page).getByRole("button", { name: "Choose date and time...", exact: true }).click();
  const value = await session.page.evaluate((offset) => {
    const date = new Date(); date.setDate(date.getDate() + offset); date.setHours(10, 30, 0, 0);
    const two = (number: number) => String(number).padStart(2, "0");
    return { value: `${date.getFullYear()}-${two(date.getMonth() + 1)}-${two(date.getDate())}T10:30`, instant: date.toISOString() };
  }, days);
  const input = snoozePicker(session.page).getByLabel("Reminder date and time", { exact: true });
  await input.fill(value.value);
  return { input, ...value };
}

async function remindNow(session: SnoozeSession, fixture: SnoozeFixture, issue: SnoozeIssue, id: string) {
  await inboxRow(session.page, issue).click({ button: "right" });
  const response = await uiSnoozeResponse(session, fixture, `/api/inbox/${id}/unsnooze`, () =>
    session.page.getByRole("menuitem", { name: "Remind now", exact: true }).click());
  const receipt = InboxSnoozeReceiptSchema.parse(await response.json());
  expect(receipt).toMatchObject({ workspace_id: fixture.workspace.id, group_id: issue.id, snoozed_until: null });
  return receipt;
}

async function switchWorkspace(session: SnoozeSession, from: SnoozeFixture["workspace"], to: SnoozeFixture["workspace"]) {
  const navigation = session.page.getByRole("navigation", { name: "Application navigation", exact: true });
  await expect(navigation.getByRole("button", { name: from.name, exact: true })).toBeVisible();
  await navigation.getByRole("button", { name: "Switch workspace", exact: true }).click();
  await session.page.getByRole("menuitem").filter({ hasText: to.name }).click();
  const inbox = navigation.getByRole("link", { name: /^Inbox(?:\s+\d+)?$/ });
  await expect(inbox).toHaveAttribute("href", new RegExp(`/${to.slug}/inbox$`));
  await inbox.click();
  await expect(session.page).toHaveURL(new RegExp(`/${to.slug}/inbox`));
}

async function touchTarget(control: Locator, evidence: SnoozeEvidence) {
  await expect(control).toBeVisible();
  const samples: ({ width: number; height: number; x: number; y: number } | null)[] = [];
  await expect.poll(async () => {
    await control.evaluate(() => new Promise<void>((done) => requestAnimationFrame(() => requestAnimationFrame(() => done()))));
    const box = await control.boundingBox(), previous = samples.at(-1);
    samples.push(box);
    const stable = box && previous && (["width", "height", "x", "y"] as const).every((key) => Math.abs(box[key] - previous[key]) < 0.01);
    return Boolean(stable && box!.width >= 44 && box!.height >= 44);
  }, { message: "A settled touch target must retain the 44px minimum after its popup animation" }).toBe(true);
  const box = samples.at(-1)!;
  expect(box).not.toBeNull(); expect(box!.height).toBeGreaterThanOrEqual(44); expect(box!.width).toBeGreaterThanOrEqual(44);
  evidence.record("touch_target_geometry", { accessible_label: await control.getAttribute("aria-label"), samples, settled: box });
}

test.describe("Inbox snooze on owned production clients", () => {
  test.skip(process.env.INBOX_SNOOZE_E2E_RUNTIME_READY !== "1", "Requires the explicit owned inbox-snooze runtime permit");
  test.beforeAll(async () => { allocation = await requireSnoozeAllocation(); actors = await createSnoozeActors(allocation); });

  test("Web group snooze survives reload, changes time, undoes and reminds the original unread group", async ({ browser }, info) => {
    const evidence = new SnoozeEvidence(info, allocation);
    const fixture = await createSnoozeFixture(allocation, actors, evidence);
    const group = await seedSnoozeIssue(fixture, "lifecycle", 2), neighbor = await seedSnoozeIssue(fixture, "neighbor");
    const sessions: SnoozeSession[] = [];
    try {
      const session = await openSnoozeSession(browser, fixture, "web"); sessions.push(session);
      const row = inboxRow(session.page, group.issue);
      await expect(row).toHaveCount(1);
      const id = await row.getByRole("button", { name: group.issue.title, exact: true }).getAttribute("data-inbox-selection");
      expect(group.notices.map((item) => item.id)).toContain(id);
      await uiSnoozeResponse(session, fixture, `/api/inbox/${id}/read`, () => row.getByRole("button", { name: group.issue.title, exact: true }).click());
      const readSnapshot = (await listSnoozeNotices(fixture, "inbox")).filter((item) => item.issue_id === group.issue.id)
        .map((item) => ({ id: item.id, read: item.read })).sort((a, b) => a.id.localeCompare(b.id));
      expect(readSnapshot.find((item) => item.id === id)?.read).toBe(true);
      await openRowSnooze(session, group.issue, false);
      const picker = snoozePicker(session.page);
      await expect(picker.getByRole("group", { name: "Snooze notification", exact: true }).getByRole("button")).toHaveCount(6);
      await expect(picker.getByRole("button", { name: /^In 1 hour/ })).toBeFocused();
      await session.capture("en-wide-light-presets");
      const first = await uiSnoozeResponse(session, fixture, `/api/inbox/${id}/snooze`, () => picker.getByRole("button", { name: /^In 1 hour/ }).click());
      const firstReceipt = InboxSnoozeReceiptSchema.parse(await first.json());
      expect(firstReceipt.group_id).toBe(group.issue.id);
      await expect(row).toHaveCount(0);
      await expect(inboxRow(session.page, neighbor.issue).getByRole("button", { name: neighbor.issue.title, exact: true })).toHaveAttribute("aria-current", "true");
      await expect(snoozedEntry(session)).toHaveAccessibleName("Snoozed 1");
      const snoozed = (await listSnoozeNotices(fixture, "snoozed")).filter((item) => item.issue_id === group.issue.id);
      expect(snoozed.map((item) => item.id).sort()).toEqual(group.notices.map((item) => item.id).sort());
      expect(snoozed.every((item) => item.snoozed_until === firstReceipt.snoozed_until)).toBe(true);
      expect(snoozed.map((item) => ({ id: item.id, read: item.read })).sort((a, b) => a.id.localeCompare(b.id))).toEqual(readSnapshot);
      await snoozedEntry(session).click(); await expect(session.page).toHaveURL(/view=snoozed/);
      await session.traffic.settle();
      await session.page.reload({ waitUntil: "domcontentloaded" });
      await expect(inboxRow(session.page, group.issue)).toHaveCount(1);
      await session.capture("en-wide-light-snoozed-reload");
      await inboxRow(session.page, group.issue).getByRole("button", { name: group.issue.title, exact: true }).click();
      await expect(session.page.locator(`#comment-body-${group.sources.at(-1)!.id}`)).toBeVisible();
      expect((await listSnoozeNotices(fixture, "snoozed")).filter((item) => item.issue_id === group.issue.id)).toHaveLength(2);
      const changed = await customTime(session, group.issue, true);
      await expect(changed.input).toBeFocused();
      await session.capture("en-wide-light-custom");
      await snoozePicker(session.page).getByRole("button", { name: "Cancel", exact: true }).click();
      await setSnoozeTheme(session, evidence, "dark");
      await openRowSnooze(session, group.issue, true);
      await snoozePicker(session.page).getByRole("button", { name: "Choose date and time...", exact: true }).click();
      await changed.input.fill(changed.value);
      await session.capture("en-wide-dark-custom");
      const edit = await uiSnoozeResponse(session, fixture, `/api/inbox/${id}/snooze`, () =>
        snoozePicker(session.page).getByRole("button", { name: "Save time", exact: true }).click());
      expect(new Date(InboxSnoozeReceiptSchema.parse(await edit.json()).snoozed_until!).toISOString()).toBe(changed.instant);
      await uiSnoozeResponse(session, fixture, `/api/inbox/${id}/snooze/undo`, () => currentUndo(session).click());
      await expect.poll(async () => (await listSnoozeNotices(fixture, "snoozed")).find((item) => item.id === id)?.snoozed_until).toBe(firstReceipt.snoozed_until);
      await remindNow(session, fixture, group.issue, id!);
      await expect.poll(async () => (await listSnoozeNotices(fixture, "inbox")).filter((item) => item.issue_id === group.issue.id).every((item) => !item.read && !item.snoozed_until)).toBe(true);
      await uiSnoozeResponse(session, fixture, `/api/inbox/${id}/snooze/undo`, () => currentUndo(session).click());
      await expect(inboxRow(session.page, group.issue)).toHaveCount(1);
      await remindNow(session, fixture, group.issue, id!);
      await snoozedEntry(session).click();
      const returned = inboxRow(session.page, group.issue).getByRole("button", { name: group.issue.title, exact: true });
      await expect(returned).toHaveAttribute("data-inbox-selection", id!);
      await expect(returned).not.toHaveAttribute("aria-current", "true");
      await expect(inboxRow(session.page, group.issue)).toContainText("Snoozed reminder");
      const final = (await listSnoozeNotices(fixture, "inbox")).filter((item) => item.issue_id === group.issue.id);
      expect(final.map((item) => item.id).sort()).toEqual(group.notices.map((item) => item.id).sort());
      expect(final.every((item) => !item.read)).toBe(true);
      evidence.record("group_lifecycle_confirmed", { issue_id: group.issue.id, notification_ids: final.map((item) => item.id),
        original_comment_ids: group.sources.map((comment) => comment.id), returned: final.map(noticeFact), selection_advanced_to: neighbor.issue.id });
    } finally { await closeSnoozeJourney(fixture, sessions); }
  });

  test("Web failed save retains selection and custom input; another device invalidates the visible undo", async ({ browser }, info) => {
    const evidence = new SnoozeEvidence(info, allocation);
    const fixture = await createSnoozeFixture(allocation, actors, evidence);
    const group = await seedSnoozeIssue(fixture, "failure retention"), neighbor = await seedSnoozeIssue(fixture, "failure neighbor");
    const sessions: SnoozeSession[] = [];
    try {
      const first = await openSnoozeSession(browser, fixture, "web"); sessions.push(first);
      const second = await openSnoozeSession(browser, fixture, "web"); sessions.push(second);
      const id = group.notices[0]!.id;
      await snoozedEntry(second).click();
      await uiSnoozeResponse(first, fixture, `/api/inbox/${id}/read`, () => inboxRow(first.page, group.issue).getByRole("button", { name: group.issue.title, exact: true }).click());
      const before = (await listSnoozeNotices(fixture, "inbox")).map(noticeFact);
      const custom = await customTime(first, group.issue, false);
      await first.page.route(`**/api/inbox/${id}/snooze`, (route) => route.fulfill({ status: 500, contentType: "application/json", body: JSON.stringify({ error: "Synthetic owned save failure" }) }), { times: 1 });
      evidence.record("injected_save_response_failure", { path: `/api/inbox/${id}/snooze`, status: 500, kind: "test_owned_network_fault" });
      await uiSnoozeResponse(first, fixture, `/api/inbox/${id}/snooze`, () => snoozePicker(first.page).getByRole("button", { name: "Snooze", exact: true }).click(), 500);
      await expect(snoozePicker(first.page).getByRole("alert")).toContainText("Could not snooze this notification. Try again.");
      await expect(custom.input).toHaveValue(custom.value);
      await expect(inboxRow(first.page, group.issue).getByRole("button", { name: group.issue.title, exact: true })).toHaveAttribute("aria-current", "true");
      expect((await listSnoozeNotices(fixture, "inbox")).map(noticeFact)).toEqual(before);
      await expect(inboxRow(first.page, neighbor.issue)).toHaveCount(1);
      await first.capture("en-save-failure-retained-input");
      const saved = await uiSnoozeResponse(first, fixture, `/api/inbox/${id}/snooze`, () => snoozePicker(first.page).getByRole("button", { name: "Snooze", exact: true }).click());
      const original = InboxSnoozeReceiptSchema.parse(await saved.json());
      const oldUndo = currentUndo(first); await oldUndo.hover();
      await expect(inboxRow(second.page, group.issue)).toHaveCount(1);
      const later = await customTime(second, group.issue, true, 3);
      const edited = await uiSnoozeResponse(second, fixture, `/api/inbox/${id}/snooze`, () => snoozePicker(second.page).getByRole("button", { name: "Save time", exact: true }).click());
      const latest = InboxSnoozeReceiptSchema.parse(await edited.json());
      expect(latest.revision).toBeGreaterThan(original.revision);
      await uiSnoozeResponse(first, fixture, `/api/inbox/${id}/snooze/undo`, () => oldUndo.click(), 409);
      await expect(first.page.getByText("This reminder changed after your action. Its latest state was kept.", { exact: true })).toBeVisible();
      expect(new Date((await listSnoozeNotices(fixture, "snoozed")).find((item) => item.id === id)!.snoozed_until!).toISOString()).toBe(later.instant);
      await snoozedEntry(first).click();
      await openRowSnooze(first, group.issue, true);
      await snoozePicker(first.page).getByRole("button", { name: "Choose date and time...", exact: true }).click();
      await expect(snoozePicker(first.page).getByLabel("Reminder date and time", { exact: true })).toHaveValue(later.value);
      evidence.record("failure_and_stale_undo_confirmed", { issue_id: group.issue.id, notification_id: id, first_revision: original.revision,
        kept_revision: latest.revision, kept_until: later.instant, stale_undo_status: 409, original_input_retained: true });
    } finally { await closeSnoozeJourney(fixture, sessions); }
  });

  test("Web ordinary and group mentions stay snoozed; personal mentions and assignment wake across devices while another workspace is active", async ({ browser }, info) => {
    const evidence = new SnoozeEvidence(info, allocation);
    const fixture = await createSnoozeFixture(allocation, actors, evidence);
    const group = await seedSnoozeIssue(fixture, "personal delivery");
    const sessions: SnoozeSession[] = [];
    try {
      const source = await openSnoozeSession(browser, fixture, "web"); sessions.push(source);
      const other = await openSnoozeSession(browser, fixture, "web"); sessions.push(other);
      await switchWorkspace(other, fixture.workspace, fixture.other);
      const id = group.notices[0]!.id;
      const snoozed = await chooseSnoozePreset(source, fixture, group.issue, id);
      const ordinary = await postSnoozeComment(fixture, group.issue, "Ordinary update during the persisted snooze");
      await expect.poll(async () => (await listSnoozeNotices(fixture, "snoozed")).filter((item) => item.issue_id === group.issue.id).length).toBe(2);
      const broadcast = await postSnoozeComment(fixture, group.issue, "[@all](mention://all/all) Group update during the persisted snooze");
      await expect.poll(async () => (await listSnoozeNotices(fixture, "snoozed")).some((item) => noticeFact(item).comment_id === broadcast.id)).toBe(true);
      const sleeping = (await listSnoozeNotices(fixture, "snoozed")).filter((item) => item.issue_id === group.issue.id);
      expect(sleeping.every((item) => item.snoozed_until === snoozed.snoozed_until)).toBe(true);
      await source.traffic.settle(); await other.traffic.settle();
      expect(source.traffic.events.filter((event) => event.type === "inbox:new" && event.issue_id === group.issue.id)).toEqual([]);
      await expect(snoozedEntry(source)).toHaveAccessibleName("Snoozed 1");
      expect((await listSnoozeNotices(fixture, "inbox", fixture.other)).filter((item) => item.issue_id === group.issue.id)).toEqual([]);
      const marker = other.traffic.responses.length;
      const eventMarker = other.traffic.events.length;
      const direct = await postSnoozeComment(fixture, group.issue, `[@${actors.owner.name}](mention://member/${actors.owner.id}) Personal review needed now`);
      await expect(inboxRow(source.page, group.issue)).toHaveCount(1);
      await expect(inboxRow(source.page, group.issue)).toContainText("Snoozed reminder");
      await expect.poll(() => other.traffic.events.slice(eventMarker).some((event) => event.workspace_id === fixture.workspace.id &&
        ((event.type === "inbox:updated" && event.group_id === group.issue.id) || (event.type === "inbox:new" && event.issue_id === group.issue.id)))).toBe(true);
      await expect.poll(() => other.traffic.responses.slice(marker).some((response) => response.path === "/api/inbox/unread-summary" &&
        Array.isArray(response.projection) && response.projection.some((item: { workspace_id?: string; count?: number }) =>
          item.workspace_id === fixture.workspace.id && typeof item.count === "number" && item.count > 0))).toBe(true);
      await expect(other.page).toHaveURL(new RegExp(`/${fixture.other.slug}/inbox`));
      await switchWorkspace(other, fixture.other, fixture.workspace);
      await expect(inboxRow(other.page, group.issue)).toHaveCount(1);
      const awake = (await listSnoozeNotices(fixture, "inbox")).filter((item) => item.issue_id === group.issue.id);
      expect(awake.every((item) => !item.read && !item.snoozed_until)).toBe(true);
      expect(awake.some((item) => noticeFact(item).comment_id === direct.id && noticeFact(item).mention_scope === "member")).toBe(true);
      expect(awake.some((item) => item.id === id)).toBe(true);
      const latest = awake.sort((a, b) => Date.parse(b.created_at) - Date.parse(a.created_at))[0]!;
      await chooseSnoozePreset(source, fixture, group.issue, latest.id);
      await expect(inboxRow(other.page, group.issue)).toHaveCount(0);
      selectApiWorkspace(actors.author, fixture.workspace);
      const updated = await actors.author.client.updateIssue(group.issue.id, { description: `A new description [@${actors.owner.name}](mention://member/${actors.owner.id}) requests an immediate decision.` });
      evidence.record("public_description_mention_source", { issue_id: group.issue.id, revision: updated.revision, actor_id: actors.author.id });
      await expect(inboxRow(source.page, group.issue)).toHaveCount(1);
      await expect(inboxRow(other.page, group.issue)).toHaveCount(1);
      const afterDescription = (await listSnoozeNotices(fixture, "inbox")).filter((item) => item.issue_id === group.issue.id).sort((a, b) => Date.parse(b.created_at) - Date.parse(a.created_at));
      await chooseSnoozePreset(source, fixture, group.issue, afterDescription[0]!.id);
      await actors.author.client.updateIssue(group.issue.id, { assignee_type: "member", assignee_id: actors.owner.id });
      await expect(inboxRow(source.page, group.issue)).toHaveCount(1); await expect(inboxRow(other.page, group.issue)).toHaveCount(1);
      const assigned = (await listSnoozeNotices(fixture, "inbox")).filter((item) => item.issue_id === group.issue.id);
      expect(assigned.some((item) => item.type === "issue_assigned")).toBe(true); expect(assigned.every((item) => !item.read && !item.snoozed_until)).toBe(true);
      evidence.record("source_workspace_and_cross_device_delivery_confirmed", { issue_id: group.issue.id, other_workspace_id: fixture.other.id,
        ordinary_comment_id: ordinary.id, group_comment_id: broadcast.id, direct_comment_id: direct.id,
        original_notification_id: id, final_notices: assigned.map(noticeFact), ordinary_inbox_new_events: 0,
        literal_source_replay: "Covered by canonical Go listener tests" });
    } finally { await closeSnoozeJourney(fixture, sessions); }
  });

  test("Web server expiry returns original unread notifications near the top and never revives an archived plan", async ({ browser }, info) => {
    const evidence = new SnoozeEvidence(info, allocation);
    const fixture = await createSnoozeFixture(allocation, actors, evidence);
    const due = await seedSnoozeIssue(fixture, "expiry", 2), archived = await seedSnoozeIssue(fixture, "archived plan"), newer = await seedSnoozeIssue(fixture, "newer source");
    const sessions: SnoozeSession[] = [];
    try {
      const session = await openSnoozeSession(browser, fixture, "web"); sessions.push(session);
      const dueRow = inboxRow(session.page, due.issue), id = await dueRow.getByRole("button", { name: due.issue.title, exact: true }).getAttribute("data-inbox-selection");
      await uiSnoozeResponse(session, fixture, `/api/inbox/${id}/read`, () => dueRow.getByRole("button", { name: due.issue.title, exact: true }).click());
      const receipt = await chooseSnoozePreset(session, fixture, due.issue, id!);
      await chooseSnoozePreset(session, fixture, archived.issue, archived.notices[0]!.id);
      await snoozedEntry(session).click();
      await inboxRow(session.page, archived.issue).click({ button: "right" });
      await uiSnoozeResponse(session, fixture, `/api/inbox/${archived.notices[0]!.id}/archive`, () => session.page.getByRole("menuitem", { name: "Archive", exact: true }).click());
      expect((await snoozeStateFact(fixture, archived.issue)).snoozed_until).toBeNull();
      await snoozedEntry(session).click();
      await makeSnoozeDue(fixture, due.issue);
      await expect.poll(() => session.traffic.events.some((event) => event.type === "inbox:updated" && event.group_id === due.issue.id &&
        typeof event.revision === "number" && event.revision > receipt.revision), { timeout: 20_000, message: "The persisted plan must be consumed by the production 15-second sweep" }).toBe(true);
      await expect(inboxRow(session.page, due.issue)).toHaveCount(1);
      await expect(inboxRow(session.page, due.issue)).toContainText("Snoozed reminder");
      const title = inboxRow(session.page, due.issue).getByRole("button", { name: due.issue.title, exact: true });
      await expect(title).toHaveAttribute("data-inbox-selection", id!); await expect(title).not.toHaveAttribute("aria-current", "true");
      const listed = await listSnoozeNotices(fixture, "inbox"), awake = listed.filter((item) => item.issue_id === due.issue.id);
      expect(awake.map((item) => item.id).sort()).toEqual(due.notices.map((item) => item.id).sort());
      expect(awake.every((item) => !item.read && item.reminder_reason === "due")).toBe(true);
      expect(listed.findIndex((item) => item.issue_id === due.issue.id)).toBeLessThan(listed.findIndex((item) => item.issue_id === newer.issue.id));
      const state = await snoozeStateFact(fixture, due.issue); expect(state.snoozed_until).toBeNull(); expect(state.reminder_reason).toBe("due");
      expect((await listSnoozeNotices(fixture, "archived")).filter((item) => item.issue_id === archived.issue.id).map((item) => item.id)).toEqual(archived.notices.map((item) => item.id));
      expect(listed.some((item) => item.issue_id === archived.issue.id)).toBe(false);
      await session.traffic.settle();
      await session.page.reload({ waitUntil: "domcontentloaded" });
      await expect(inboxRow(session.page, due.issue)).toContainText("Snoozed reminder");
      evidence.record("server_expiry_and_archive_confirmed", { issue_id: due.issue.id, notification_ids: awake.map((item) => item.id),
        consumed_state: state, archived_issue_id: archived.issue.id, original_comment_ids: due.sources.map((source) => source.id) });
      await session.capture("en-server-expiry-original-unread");
    } finally { await closeSnoozeJourney(fixture, sessions); }
  });

  test("Web Chinese touch controls preserve keyboard focus, custom time and narrow layouts in both themes", async ({ browser }, info) => {
    const evidence = new SnoozeEvidence(info, allocation);
    const fixture = await createSnoozeFixture(allocation, actors, evidence);
    const group = await seedSnoozeIssue(fixture, "narrow Chinese long notification title with retained source context");
    await actors.owner.client.requestJSON("/api/me", { method: "PATCH", body: { language: "zh-Hans" } });
    const sessions: SnoozeSession[] = [];
    try {
      const session = await openSnoozeSession(browser, fixture, "web", { locale: "zh-Hans", touch: true, width: 390, height: 844 }); sessions.push(session);
      const row = inboxRow(session.page, group.issue), menu = row.getByRole("button", { name: "通知操作", exact: true });
      await touchTarget(menu, evidence);
      await menu.tap(); await touchTarget(session.page.getByRole("menuitem", { name: "延后...", exact: true }), evidence);
      await session.capture("zh-narrow-light-touch-menu");
      await session.page.getByRole("menuitem", { name: "延后...", exact: true }).tap();
      const picker = snoozePicker(session.page), first = picker.getByRole("button", { name: /^1 小时后/ });
      await expect(first).toBeFocused();
      await touchTarget(first, evidence); await session.capture("zh-narrow-light-presets");
      await first.press("ArrowDown"); await expect(picker.getByRole("button", { name: /^3 小时后/ })).toBeFocused();
      expect(await row.getByRole("button", { name: group.issue.title, exact: true }).getAttribute("aria-current")).toBeNull();
      await picker.getByRole("button", { name: /^3 小时后/ }).press("Escape"); await expect(picker).toHaveCount(0);
      await expect(row.getByRole("button", { name: group.issue.title, exact: true })).toBeFocused();
      await menu.tap(); await session.page.getByRole("menuitem", { name: "延后...", exact: true }).tap();
      await snoozePicker(session.page).getByRole("button", { name: "选择日期和时间...", exact: true }).tap();
      const input = snoozePicker(session.page).getByLabel("提醒日期和时间", { exact: true });
      await expect(input).toBeFocused(); await touchTarget(input, evidence);
      const value = await session.page.evaluate(() => `${new Date().getFullYear() + 1}-01-04T09:15`);
      await input.fill(value);
      await expect(snoozePicker(session.page)).toContainText("Asia/Shanghai");
      await expect(snoozePicker(session.page)).toContainText(String(new Date().getFullYear() + 1));
      expect(await session.page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
      await session.capture("zh-narrow-light-custom-cross-year");
      await snoozePicker(session.page).getByRole("button", { name: "取消", exact: true }).tap();
      await setSnoozeTheme(session, evidence, "dark", "zh-Hans");
      await menu.tap(); await session.page.getByRole("menuitem", { name: "延后...", exact: true }).tap();
      await snoozePicker(session.page).getByRole("button", { name: "选择日期和时间...", exact: true }).tap();
      await input.fill(value);
      await session.capture("zh-narrow-dark-custom-cross-year");
      await uiSnoozeResponse(session, fixture, `/api/inbox/${group.notices[0]!.id}/snooze`, () => snoozePicker(session.page).getByRole("button", { name: "延后", exact: true }).tap());
      const entry = mainList(session).getByRole("button", { name: "已延后 1", exact: true }); await touchTarget(entry, evidence); await entry.tap();
      await expect(inboxRow(session.page, group.issue)).toHaveCount(1);
      expect(await session.page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
      await session.capture("zh-narrow-dark-snoozed");
      await session.traffic.settle();
      await session.page.reload({ waitUntil: "domcontentloaded" }); await expect(inboxRow(session.page, group.issue)).toHaveCount(1);
      evidence.record("touch_and_keyboard_confirmed", { viewport_width: 390, minimum_target: 44, locale: "zh-Hans",
        issue_id: group.issue.id, saved_local_time: value, timezone: "Asia/Shanghai", selected_behind_picker: false, escape_focus_returned: true });
    } finally { await closeSnoozeJourney(fixture, sessions); }
  });

  test("Native Electron uses the shared snooze journey and keeps main, Snoozed and Archived tabs independent", async ({ browser }, info) => {
    const evidence = new SnoozeEvidence(info, allocation);
    const fixture = await createSnoozeFixture(allocation, actors, evidence);
    const group = await seedSnoozeIssue(fixture, "native shared journey"), archived = await seedSnoozeIssue(fixture, "native archived tab");
    const sessions: SnoozeSession[] = [];
    try {
      const native = await openSnoozeSession(browser, fixture, "desktop"); sessions.push(native);
      const web = await openSnoozeSession(browser, fixture, "web"); sessions.push(web);
      const receipt = await chooseSnoozePreset(native, fixture, group.issue, group.notices[0]!.id);
      await expect(inboxRow(web.page, group.issue)).toHaveCount(0); await expect(snoozedEntry(web)).toHaveAccessibleName("Snoozed 1");
      await inboxRow(native.page, archived.issue).hover();
      await uiSnoozeResponse(native, fixture, `/api/inbox/${archived.notices[0]!.id}/archive`, () => inboxRow(native.page, archived.issue).getByRole("button", { name: "Archive", exact: true }).click());
      await snoozedEntry(native).click(); await expect(inboxRow(native.page, group.issue)).toHaveCount(1);
      await native.capture("en-wide-light-snoozed");
      await openRowSnooze(native, group.issue, true); await native.capture("en-wide-light-presets");
      await snoozePicker(native.page).getByRole("button", { name: "Choose date and time...", exact: true }).click();
      const nativeTime = snoozePicker(native.page).getByLabel("Reminder date and time", { exact: true });
      await expect(nativeTime).toBeFocused();
      const nativeCustom = await native.page.evaluate(() => {
        const date = new Date(); date.setDate(date.getDate() + 2); date.setHours(10, 30, 0, 0);
        const two = (value: number) => String(value).padStart(2, "0");
        return { value: `${date.getFullYear()}-${two(date.getMonth() + 1)}-${two(date.getDate())}T10:30`, instant: date.toISOString() };
      });
      await nativeTime.fill(nativeCustom.value);
      await native.capture("en-wide-light-custom");
      const edited = await uiSnoozeResponse(native, fixture, `/api/inbox/${group.notices[0]!.id}/snooze`, () =>
        snoozePicker(native.page).getByRole("button", { name: "Save time", exact: true }).click());
      expect(new Date(InboxSnoozeReceiptSchema.parse(await edited.json()).snoozed_until!).toISOString()).toBe(nativeCustom.instant);
      await uiSnoozeResponse(native, fixture, `/api/inbox/${group.notices[0]!.id}/snooze/undo`, () => currentUndo(native).click());
      await expect.poll(async () => (await listSnoozeNotices(fixture, "snoozed")).find((item) => item.id === group.notices[0]!.id)?.snoozed_until).toBe(receipt.snoozed_until);
      const viewOf = (url: string) => new URL(url, allocation.rendererUrl).searchParams.get("view") ?? "inbox";
      const sleepingTabs = await observeNativeTabs(native, fixture);
      const sleepingTab = sleepingTabs.tabs.find((tab) => viewOf(tab.url) === "snoozed");
      expect(sleepingTab).toBeDefined(); expect(sleepingTab!.id).toBe(sleepingTabs.active);
      const tabFrames = native.page.locator("[data-tab-frame]");
      await expect(native.page.locator(`[data-tab-frame][data-tab-id="${sleepingTab!.id}"]`).getByRole("button", { name: "Snoozed", exact: true })).toBeVisible();
      await native.page.getByRole("link", { name: /^Inbox(?:\s+\d+)?$/ }).click({ modifiers: [process.platform === "darwin" ? "Meta" : "Control"] });
      await expect.poll(async () => (await observeNativeTabs(native, fixture)).tabs.length).toBe(2);
      const secondTabs = await observeNativeTabs(native, fixture);
      const firstMain = secondTabs.tabs.find((tab) => viewOf(tab.url) === "inbox"); expect(firstMain).toBeDefined();
      await native.page.locator(`[data-tab-frame][data-tab-id="${firstMain!.id}"]`).getByRole("button", { name: "Inbox", exact: true }).click();
      await mainList(native).getByRole("button", { name: /^Archived\s*1$/ }).click();
      await expect(inboxRow(native.page, archived.issue)).toHaveCount(1);
      const archiveTabs = await observeNativeTabs(native, fixture);
      expect(archiveTabs.tabs.find((tab) => tab.id === firstMain!.id)).toMatchObject({ url: `/${fixture.workspace.slug}/inbox?view=archived` });
      await native.page.getByRole("link", { name: /^Inbox(?:\s+\d+)?$/ }).click({ modifiers: [process.platform === "darwin" ? "Meta" : "Control"] });
      await expect.poll(async () => (await observeNativeTabs(native, fixture)).tabs.length).toBe(3);
      const allTabs = await observeNativeTabs(native, fixture);
      expect(allTabs.tabs.map((tab) => viewOf(tab.url)).sort()).toEqual(["archived", "inbox", "snoozed"]);
      const tabIds = await tabFrames.evaluateAll((frames) => frames.map((frame) => frame.getAttribute("data-tab-id")));
      expect(new Set(tabIds).size).toBe(3);
      expect([...tabIds].sort()).toEqual(allTabs.tabs.map((tab) => tab.id).sort());
      const mainTab = allTabs.tabs.find((tab) => viewOf(tab.url) === "inbox")!;
      await native.page.locator(`[data-tab-frame][data-tab-id="${mainTab.id}"]`).getByRole("button", { name: "Inbox", exact: true }).click();
      await expect(snoozedEntry(native)).toHaveAccessibleName("Snoozed 1");
      await native.page.getByRole("link", { name: /^Inbox(?:\s+\d+)?$/ }).click({ modifiers: [process.platform === "darwin" ? "Meta" : "Control"] });
      expect((await observeNativeTabs(native, fixture)).tabs).toEqual(allTabs.tabs);
      await native.page.locator(`[data-tab-frame][data-tab-id="${sleepingTab!.id}"]`).getByRole("button", { name: "Snoozed", exact: true }).click();
      await expect(inboxRow(native.page, group.issue)).toHaveCount(1);
      await native.traffic.settle();
      await native.page.reload({ waitUntil: "domcontentloaded" }); await expect(inboxRow(native.page, group.issue)).toHaveCount(1);
      const reloadedTabs = await observeNativeTabs(native, fixture);
      expect(reloadedTabs.tabs).toEqual(allTabs.tabs);
      await expect(native.page.locator(`[data-tab-frame][data-tab-id="${sleepingTab!.id}"]`).getByRole("button", { name: "Snoozed", exact: true })).toBeVisible();
      await setSnoozeTheme(native, evidence, "dark");
      await native.capture("en-wide-dark-snoozed-tabs");
      await remindNow(native, fixture, group.issue, group.notices[0]!.id);
      await expect(inboxRow(web.page, group.issue)).toHaveCount(1);
      expect((await listSnoozeNotices(fixture, "inbox")).filter((item) => item.issue_id === group.issue.id).every((item) => !item.read)).toBe(true);
      await snoozedEntry(native).click();
      await chooseSnoozePreset(native, fixture, group.issue, group.notices[0]!.id);
      await snoozedEntry(native).click();
      const window = await native.native!.app.browserWindow(native.page);
      await window.evaluate((target) => { target.setContentSize(900, 1000); target.webContents.setZoomFactor(2); }); await window.dispose();
      await expect.poll(() => native.page.evaluate(() => innerWidth)).toBe(450);
      await actors.owner.client.requestJSON("/api/me", { method: "PATCH", body: { language: "zh-Hans" } });
      await native.context.addInitScript(() => { localStorage.setItem("multica-locale", "zh-Hans"); document.cookie = "multica-locale=zh-Hans; path=/; SameSite=Lax"; });
      await native.traffic.settle();
      await native.page.reload({ waitUntil: "domcontentloaded" });
      await expect(native.page.getByRole("heading", { name: "收件箱", exact: true })).toBeVisible();
      await inboxRow(native.page, group.issue).hover();
      const pointer = await native.page.evaluate(() => ({ hover: matchMedia("(hover:hover)").matches, coarse: matchMedia("(pointer:coarse)").matches }));
      evidence.record("native_pointer_capability", pointer); expect(pointer.hover).toBe(true); expect(pointer.coarse).toBe(false);
      await inboxRow(native.page, group.issue).click({ button: "right" }); await native.capture("zh-narrow-dark-native-menu");
      await native.page.getByRole("menuitem", { name: "更改时间...", exact: true }).click();
      await snoozePicker(native.page).getByRole("button", { name: "选择日期和时间...", exact: true }).click();
      await expect(snoozePicker(native.page).getByLabel("提醒日期和时间", { exact: true })).toBeFocused();
      await expect(snoozePicker(native.page).getByLabel("提醒日期和时间", { exact: true })).toBeInViewport();
      const nativeInputTheme = await snoozePicker(native.page).getByLabel("提醒日期和时间", { exact: true }).evaluate((element) => {
        const style = getComputedStyle(element);
        return { dark: document.documentElement.classList.contains("dark"), classes: document.documentElement.className,
          colorScheme: style.colorScheme, color: style.color, background: style.backgroundColor,
          disabled: (element as HTMLInputElement).disabled, value: (element as HTMLInputElement).value };
      });
      evidence.record("native_dark_custom_input_theme", nativeInputTheme);
      expect(nativeInputTheme.dark).toBe(true); expect(nativeInputTheme.colorScheme).toContain("dark");
      expect(nativeInputTheme.disabled).toBe(false); expect(nativeInputTheme.value.length).toBeGreaterThan(0);
      expect(await native.page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
      await native.capture("zh-narrow-dark-native-custom");
      await snoozePicker(native.page).getByRole("button", { name: "取消", exact: true }).click();
      await setSnoozeTheme(native, evidence, "light", "zh-Hans");
      await inboxRow(native.page, group.issue).click({ button: "right" });
      await native.page.getByRole("menuitem", { name: "更改时间...", exact: true }).click();
      await snoozePicker(native.page).getByRole("button", { name: "选择日期和时间...", exact: true }).click();
      await expect(snoozePicker(native.page).getByLabel("提醒日期和时间", { exact: true })).toBeInViewport();
      await native.capture("zh-narrow-light-native-custom");
      await snoozePicker(native.page).getByRole("button", { name: "取消", exact: true }).click();
      await native.capture("zh-narrow-light-native-snoozed");
      evidence.record("native_shared_journey_and_tab_identities_confirmed", { issue_id: group.issue.id, original_notification_id: group.notices[0]!.id,
        first_revision: receipt.revision, tab_ids: tabIds, independent_views: ["inbox", "snoozed", "archived"], real_native_compositor: true,
        renderer_origin: allocation.rendererUrl, provider_execution: false, web_peer_updated_without_reload: true });
    } finally { await closeSnoozeJourney(fixture, sessions); }
  });
});
