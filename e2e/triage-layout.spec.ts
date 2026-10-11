import { randomUUID } from "node:crypto";
import { expect, test, type Page, type TestInfo } from "@playwright/test";
import { TestApiClient } from "./fixtures";

async function capture(page: Page, info: TestInfo, name: string) {
  await page.evaluate(() => document.fonts.ready);
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBe(true);
  const path = info.outputPath(`${name}.png`);
  const image = await page.screenshot({ path, fullPage: false, scale: "css", animations: "disabled" });
  const viewport = page.viewportSize()!;
  expect({ width: image.readUInt32BE(16), height: image.readUInt32BE(20) }).toEqual(viewport);
  await info.attach(name, { path, contentType: "image/png" });
}

for (const locale of ["en", "zh-Hans"] as const) {
  test(`triage review layout keeps queue controls and decisions usable (${locale})`, async ({ page }, info) => {
    test.setTimeout(120_000);
    const api = new TestApiClient();
    const slug = `triage-layout-${randomUUID().slice(0, 8)}`;
    const zh = locale === "zh-Hans";
    const title = zh ? "客户反馈：附件上传后无法预览" : "Customer report: uploaded attachments cannot be previewed";
    const secondTitle = zh ? "补充接口限流说明" : "Document API rate limits";
    const copy = {
      queue: zh ? "分拣队列" : "Triage queue",
      search: zh ? "搜索标题或编号" : "Search title or identifier",
      reviewLink: zh ? "前往分拣台审核" : "Open in triage",
      back: zh ? "返回队列" : "Back to queue",
      accept: zh ? "接受" : "Accept",
      acceptOptions: zh ? "接受选项" : "Acceptance options",
      execute: zh ? "接受并执行" : "Accept and execute",
      reject: zh ? "拒绝" : "Reject",
      filters: zh ? "筛选" : "Filters",
    };
    let workspace: { id: string; slug: string } | undefined;
    const errors: string[] = [];
    page.on("pageerror", error => errors.push(error.message));
    try {
      await api.login(`${slug}@example.invalid`, zh ? "审核测试" : "Review tester");
      workspace = await api.ensureWorkspace(zh ? "产品反馈" : "Product feedback", slug);
      await api.markUserOnboarded();
      await api.requestJSON("/api/me", { method: "PATCH", body: { language: locale } });
      const settings = await api.requestJSON<{ revision: number }>("/api/triage/settings");
      await api.requestJSON("/api/triage/settings", {
        method: "PUT",
        body: { enabled: true, acceptance_status: "todo", require_priority: false, responsibility_mode: "none", responsibility_member_id: null, expected_revision: settings.revision },
      });
      const item = await api.requestJSON<{ issue: { id: string; identifier: string } }>("/api/triage/items", {
        method: "POST",
        body: {
          request_id: randomUUID(), title, priority: "urgent",
          description: zh
            ? "客户在任务中上传 PDF 附件后，预览窗口一直显示加载中。请确认影响范围，并补充复现步骤后安排处理。"
            : "After uploading a PDF attachment, the preview remains in a loading state. Confirm the affected users and reproduction steps before scheduling the fix.",
        },
      });
      await api.requestJSON("/api/triage/items", { method: "POST", body: { request_id: randomUUID(), title: secondTitle } });
      const comment = zh ? "已在桌面端复现，原始文件可以正常下载。" : "Reproduced on desktop. The original file still downloads correctly.";
      await api.requestJSON(`/api/issues/${item.issue.id}/comments`, { method: "POST", body: { content: comment } });
      await page.addInitScript(({ token, language }) => {
        localStorage.setItem("multica_token", token!);
        localStorage.setItem("multica-locale", language);
        localStorage.setItem("multica:chat:isOpen", "false");
        localStorage.setItem("theme", "light");
      }, { token: api.getToken(), language: locale });
      await page.setViewportSize({ width: 2048, height: 1172 });
      await page.goto(`/${workspace.slug}/triage?issue=${item.issue.id}`);
      await expect(page.getByRole("button", { name: title, exact: true })).toBeVisible();
      await expect(page.getByText(comment, { exact: true })).toBeVisible();
      await expect(page.getByRole("link", { name: copy.reviewLink, exact: true })).toHaveCount(0);
      const queue = page.getByRole("region", { name: copy.queue, exact: true });
      await expect(queue.getByRole("textbox", { name: copy.search, exact: true })).toBeVisible();
      expect((await queue.boundingBox())!.width).toBeLessThanOrEqual(480);
      await capture(page, info, `triage-${locale}-wide`);

      await queue.getByRole("button", { name: copy.filters, exact: true }).click();
      expect(await queue.evaluate(element => element.scrollWidth <= element.clientWidth + 1)).toBe(true);
      await queue.getByRole("button", { name: copy.filters, exact: true }).click();
      await page.setViewportSize({ width: 1280, height: 900 });
      await capture(page, info, `triage-${locale}-desktop`);
      await page.getByRole("button", { name: copy.acceptOptions, exact: true }).click();
      await page.getByRole("menuitem", { name: copy.execute, exact: true }).click();
      await expect(page.getByRole("dialog", { name: copy.execute, exact: true })).toBeVisible();
      expect(await api.countIssueDispatches(item.issue.id)).toBe(0);
      await page.keyboard.press("Escape");
      await expect(page.getByRole("dialog")).toHaveCount(0);

      await page.setViewportSize({ width: 390, height: 844 });
      await expect(page.getByRole("textbox", { name: copy.search, exact: true })).toHaveCount(0);
      await expect(page.getByRole("button", { name: copy.accept, exact: true })).toBeVisible();
      await expect(page.getByRole("button", { name: copy.reject, exact: true })).toBeVisible();
      await capture(page, info, `triage-${locale}-compact-detail`);
      await page.getByRole("button", { name: copy.back, exact: true }).click();
      await expect(queue.getByRole("textbox", { name: copy.search, exact: true })).toBeVisible();
      await capture(page, info, `triage-${locale}-compact-queue`);
      await queue.getByRole("button", { name: `${item.issue.identifier} ${title}`, exact: true }).click();
      await page.getByRole("button", { name: copy.accept, exact: true }).click();
      await page.getByRole("dialog").getByRole("button", { name: copy.accept, exact: true }).click();
      await expect(page.getByRole("dialog")).toHaveCount(0);
      await expect(page.getByRole("button", { name: secondTitle, exact: true })).toBeVisible();
      expect(await api.countIssueDispatches(item.issue.id)).toBe(0);
      expect(errors).toEqual([]);
    } finally {
      if (workspace) await api.deleteFeatureWorkspace(workspace.id);
    }
  });
}
