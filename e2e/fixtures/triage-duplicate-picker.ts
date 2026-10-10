import { randomUUID } from "node:crypto";
import { expect, type Page, type TestInfo } from "@playwright/test";
import type { TestApiClient } from "../fixtures";
import { p1EnableTriage } from "./project-p1";

export type DuplicatePickerLocale = "en" | "zh-Hans";
type Session = { api: TestApiClient; workspace: { id: string; slug: string } };
type Issue = { id: string; title: string; identifier: string; admission_status: string; revision: number };
type Item = { issue: Issue; duplicate_issue_id: string | null };

async function pending(api: TestApiClient, title: string) {
  return api.requestJSON<Item>("/api/triage/items", {
    method: "POST", body: { request_id: randomUUID(), title },
  });
}

async function decide(api: TestApiClient, item: Item, action: string, fields: Record<string, unknown> = {}) {
  return api.requestJSON<{ item: Item }>(`/api/triage/items/${item.issue.id}/actions`, {
    method: "POST",
    body: { request_id: randomUUID(), expected_revision: item.issue.revision, action, ...fields },
  });
}

async function seedCandidates(api: TestApiClient) {
  // More than the picker's 30-row page, without dozens of setup HTTP requests.
  const rows = await api.seedTableIssues(Array.from({ length: 31 }, (_, index) => ({
    title: index === 0 ? "Resolved UI/UX upload failure" : index === 1 ? "Cancelled migration" : `Formal delivery task ${index}`,
    status: index === 0 ? "done" : index === 1 ? "cancelled" : "todo",
  })));
  const done = await api.requestJSON<Issue>(`/api/issues/${rows[0].id}`);
  const cancelled = await api.requestJSON<Issue>(`/api/issues/${rows[1].id}`);
  const child = await api.requestJSON<Issue>("/api/issues", {
    method: "POST", body: { title: "Formal delivery subtask", parent_issue_id: rows[2].id },
  });
  await p1EnableTriage(api);
  const admitted = await pending(api, "Accepted intake target");
  const accepted = (await decide(api, admitted, "accept")).item.issue;
  const rejected = await pending(api, "Rejected intake must be excluded");
  await decide(api, rejected, "reject", { reason: "Outside the agreed scope" });
  const duplicate = await pending(api, "Duplicate intake must be excluded");
  await decide(api, duplicate, "duplicate", { duplicate_issue_id: done.id, reason: "Previously reviewed" });
  const otherPending = await pending(api, "Unreviewed intake must be excluded");
  const source = await pending(api, "Customer reports the same upload failure");
  return {
    done, cancelled, child, accepted, source,
    formalTitles: [...rows.map(row => row.title), child.title, accepted.title],
    excluded: [source, rejected, duplicate, otherPending],
  };
}

async function capturePicker(page: Page, info: TestInfo, name: string, width: number) {
  const viewport = { width, height: 900 };
  await page.setViewportSize(viewport);
  await page.evaluate(() => document.fonts.ready);
  const dialog = page.getByRole("dialog");
  await expect(dialog).toBeVisible();
  expect(await page.evaluate(() => ({ width: innerWidth, height: innerHeight }))).toEqual(viewport);
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBe(true);
  expect(await dialog.evaluate(element => element.scrollWidth <= element.clientWidth + 1)).toBe(true);
  const path = info.outputPath(`${name}.png`);
  const png = await page.screenshot({ path, fullPage: false, scale: "css", animations: "disabled" });
  expect({ width: png.readUInt32BE(16), height: png.readUInt32BE(20) }).toEqual(viewport);
  expect(await page.evaluate(() => ({ width: innerWidth, height: innerHeight }))).toEqual(viewport);
  await info.attach(name, { path, contentType: "image/png" });
}

// Validator and failure matrices belong to core/views unit tests. This flow owns
// real API pagination, focus/keyboard integration and the durable review decision.
export async function duplicatePickerFlow(
  page: Page,
  session: Session,
  info: TestInfo,
  locale: DuplicatePickerLocale,
  native = false,
) {
  const { api, workspace } = session;
  const fixture = await seedCandidates(api);
  const zh = locale === "zh-Hans";
  const copy = {
    triage: zh ? "分拣台" : "Triage",
    queue: zh ? "分拣队列" : "Triage queue",
    duplicate: zh ? "标记重复" : "Mark duplicate",
    target: zh ? "重复目标任务" : "Duplicate target task",
    candidates: zh ? "符合条件的任务" : "Eligible tasks",
    more: zh ? "加载更多任务" : "Load more tasks",
    reason: zh ? "原因" : "Reason",
    done: zh ? "已完成" : "Done",
    cancelled: zh ? "已取消" : "Cancelled",
  };
  const pageErrors: string[] = [];
  page.on("pageerror", error => pageErrors.push(error.message));
  if (native) {
    await api.requestJSON("/api/me", { method: "PATCH", body: { language: locale } });
    await page.evaluate(value => localStorage.setItem("multica-locale", value), locale);
    await page.reload();
    await page.getByRole("link", { name: new RegExp(copy.triage) })
      .and(page.locator('[data-sidebar="menu-button"]')).click();
    await page.getByRole("region", { name: copy.queue, exact: true })
      .getByRole("button", { name: new RegExp(fixture.source.issue.title) }).click();
  } else {
    await page.goto(`/${workspace.slug}/triage?issue=${fixture.source.issue.id}`);
  }
  await page.getByRole("button", { name: copy.duplicate, exact: true }).click();
  const dialog = page.getByRole("dialog", { name: copy.duplicate, exact: true });
  const input = dialog.getByRole("combobox", { name: copy.target, exact: true });
  const list = dialog.getByRole("listbox", { name: copy.candidates, exact: true });
  const options = list.getByRole("option");
  const reason = zh ? "同一上传故障，保留客户的补充说明" : "Same upload failure; retain the customer's additional context";
  await dialog.getByRole("textbox", { name: copy.reason, exact: true }).fill(reason);
  await input.click();
  await expect(input).toHaveValue("");
  await expect(list).toBeVisible();
  await expect(options).toHaveCount(30);
  await capturePicker(page, info, `duplicate-picker-${locale}-wide`, 1280);

  if (zh || native) {
    // Reach the second page through the actual scroll surface.
    await list.evaluate(element => { element.scrollTop = element.scrollHeight; });
  } else {
    const firstPage = await options.allTextContents();
    const more = dialog.getByRole("button", { name: copy.more, exact: true });
    const isNextPage = (url: URL) => url.pathname === "/api/issues" && Number(url.searchParams.get("offset")) > 0;
    let releaseFailure!: () => void;
    const failureGate = new Promise<void>(resolve => { releaseFailure = resolve; });
    await page.route(isNextPage, async route => {
      await failureGate;
      await route.fulfill({ status: 503, contentType: "application/json", body: JSON.stringify({ error: "Temporary candidate page failure" }) });
    }, { times: 1 });
    const failedResponse = page.waitForResponse(response => isNextPage(new URL(response.url())) && response.status() === 503);
    try {
      await input.press("Tab");
      await expect(more).toBeFocused();
      await page.keyboard.press("Escape");
      await expect(list).toHaveCount(0);
      await expect(dialog).toBeVisible();
      await expect(input).toBeFocused();
      await input.press("ArrowDown");
      await expect(list).toBeVisible();
      await input.press("Tab");
      await expect(more).toBeFocused();
      await page.keyboard.press("Enter");
      await expect(list).toHaveAttribute("aria-busy", "true");
      await expect(dialog.getByRole("button", { name: "Loading…", exact: true })).toBeVisible();
      await expect(options).toHaveCount(30);
    } finally {
      releaseFailure();
    }
    await failedResponse;
    await expect(dialog.getByRole("alert")).toHaveText("Could not load more tasks. Try again.");
    expect(await options.allTextContents()).toEqual(firstPage);
    const retry = dialog.getByRole("button", { name: "Retry", exact: true });
    await expect(retry).toBeVisible();
    await input.focus();
    await input.press("Tab");
    await expect(retry).toBeFocused();
    await page.keyboard.press("Enter");
  }
  await expect(options).toHaveCount(fixture.formalTitles.length);
  await expect(dialog.getByRole("button", { name: copy.more, exact: true })).toHaveCount(0);
  const optionText = await options.allTextContents();
  for (const title of fixture.formalTitles) expect(optionText.some(text => text.includes(title)), title).toBe(true);
  for (const item of fixture.excluded) {
    await expect(options.filter({ hasText: item.issue.title })).toHaveCount(0);
  }
  const doneOption = options.filter({ hasText: fixture.done.title });
  await expect(doneOption).toContainText(fixture.done.identifier);
  await expect(doneOption).toContainText(copy.done);
  await expect(options.filter({ hasText: fixture.cancelled.title })).toContainText(copy.cancelled);

  await input.press("Escape");
  await expect(list).toHaveCount(0);
  await expect(dialog).toBeVisible();
  await expect(input).toBeFocused();
  await input.click();
  await expect(list).toBeVisible();
  await capturePicker(page, info, `duplicate-picker-${locale}-narrow`, native ? 680 : 390);

  await input.fill(fixture.done.title);
  await expect(options).toHaveCount(1);
  await expect(options.first()).toContainText(fixture.done.title);
  await input.press("ArrowDown");
  await input.press("Enter");
  await expect(list).toHaveCount(0);
  await expect(input).toHaveValue(fixture.done.identifier);
  await expect(dialog).toBeVisible();
  expect((await api.requestJSON<Item>(`/api/triage/items/${fixture.source.issue.id}`)).issue.admission_status).toBe("pending");

  await input.fill(fixture.cancelled.identifier);
  await expect(options).toHaveCount(1);
  await expect(options.first()).toContainText(fixture.cancelled.title);
  const link = new URL(`/${workspace.slug}/issues/${fixture.done.id}`, page.url()).href;
  await input.fill(link);
  await expect(options).toHaveCount(1);
  await expect(options.first()).toContainText(fixture.done.title);
  await input.fill("");
  await expect(options).toHaveCount(fixture.formalTitles.length);
  await input.fill(fixture.done.title);
  await expect(options).toHaveCount(1);
  await options.first().click();
  await expect(input).toHaveValue(fixture.done.identifier);
  await expect(list).toHaveCount(0);
  await expect(dialog.getByRole("textbox", { name: copy.reason, exact: true })).toHaveValue(reason);
  await dialog.getByRole("button", { name: copy.duplicate, exact: true }).click();
  await expect(dialog).toHaveCount(0);
  const result = await api.requestJSON<Item>(`/api/triage/items/${fixture.source.issue.id}`);
  expect(result).toMatchObject({ issue: { admission_status: "duplicate" }, duplicate_issue_id: fixture.done.id });
  const history = await api.requestJSON<{ events: { action: string; reason: string | null }[] }>(`/api/triage/items/${fixture.source.issue.id}/history`);
  expect(history.events).toEqual([expect.objectContaining({ action: "duplicate", reason })]);
  expect(pageErrors).toEqual([]);
}
