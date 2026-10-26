import { expect, type Frame, type Page, type Request } from "@playwright/test";
import type { CollaborationIssue, CollaborationOperation, CollaborationStatusOptions } from "@multica/core/collaboration";

export type DirectedAction = "request_information" | "return_information" | "submit_acceptance";

/** Copied from collaboration-installed-main.spec.ts:21-49 (that spec only runs
 * in the packaged-image lane). Reload, then read the exact issue the page fetched. */
export async function refreshDirectedIssue(page: Page, issueId: string) {
  let navigated = false;
  const requests = new WeakSet();
  const navigation = (frame: Frame) => { if (frame === page.mainFrame()) navigated = true; };
  const observe = (value: Request) => {
    if (navigated && value.method() === "GET" && new URL(value.url()).pathname === `/api/collaboration/issues/${issueId}`) requests.add(value);
  };
  page.on("framenavigated", navigation);
  page.on("request", observe);
  try {
    const pending = page.waitForResponse((response) => requests.has(response.request()) && response.status() === 200);
    await page.reload();
    const response = await pending;
    expect(response.status()).toBe(200);
    const current = await page.request.get(response.url());
    expect(current.status()).toBe(200);
    const issue: CollaborationIssue = await current.json();
    expect(issue.id).toBe(issueId);
    const heading = page.getByRole("heading", { name: issue.title, exact: true });
    await expect(heading).toBeVisible();
    await expect(heading.locator("..").locator("..").locator("dl > div")
      .filter({ has: page.getByText("Handler", { exact: true }) }).locator("dd")).toContainText(issue.handler.name);
    await page.evaluate(() => new Promise<void>((resolve) => requestAnimationFrame(() => requestAnimationFrame(() => resolve()))));
    return issue;
  } finally {
    page.off("framenavigated", navigation);
    page.off("request", observe);
  }
}

/** Copied from collaboration-installed-main.spec.ts:98-137: the Web form for a
 * directed handoff, asserting the single offered status and the exact observations sent. */
export async function directedFormThroughUI(page: Page, issueId: string, action: DirectedAction, note: string, recipient?: { id: string; name: string }) {
  const baseline = await refreshDirectedIssue(page, issueId);
  expect(baseline.capabilities[action]).toBe(true);
  const label = { request_information: "Request information", return_information: "Return information", submit_acceptance: "Submit for acceptance" }[action];
  const optionsPending = page.waitForResponse((response) => new URL(response.url()).pathname === `/api/collaboration/issues/${issueId}/status-options`
    && new URL(response.url()).searchParams.get("action") === action && response.request().method() === "GET");
  await page.getByRole("button", { name: label, exact: true }).click();
  const form = page.locator("form").filter({ has: page.getByRole("heading", { name: label, exact: true }) });
  const optionsResponse = await optionsPending;
  expect(optionsResponse.status()).toBe(200);
  const options: CollaborationStatusOptions = await optionsResponse.json();
  expect(options.action).toBe(action);
  expect(options.options).toHaveLength(1);
  if (action === "request_information") {
    if (!recipient) throw new Error("Request information needs the exact participant recipient");
    await form.getByLabel("Exact recipient", { exact: true }).fill(recipient.id);
    await form.getByRole("button", { name: "Find account", exact: true }).click();
    await expect(form.getByText(`Selected account: ${recipient.name}`, { exact: true })).toBeVisible();
  }
  const select = form.getByLabel("Status", { exact: true });
  await expect(select.locator("option")).toHaveCount(1);
  await select.selectOption(options.default_status);
  await form.getByLabel("Note", { exact: true }).fill(note);
  const endpoint = `/api/collaboration/issues/${issueId}/${action === "request_information" ? "handoffs" : action === "return_information" ? "information-returns" : "acceptance-submissions"}`;
  const pending = page.waitForResponse((response) => new URL(response.url()).pathname === endpoint && response.request().method() === "POST");
  await form.getByRole("button", { name: "Confirm", exact: true }).click();
  const response = await pending;
  expect(response.status()).toBe(200);
  const receipt: CollaborationOperation = await response.json();
  const input: Record<string, unknown> = response.request().postDataJSON();
  const requestId = response.request().headers()["idempotency-key"]!;
  expect(requestId).toMatch(/^[0-9a-f-]{36}$/i);
  expect(input).toMatchObject({ expected_collaboration_revision: baseline.collaboration_revision, expected_lifecycle_epoch: baseline.lifecycle_epoch,
    expected_participant_epoch: baseline.write_context!.participant_epoch, expected_account_epoch: baseline.write_context!.account_epoch,
    expected_status: options.default_status, status_options_version: options.status_options_version, note });
  if (action === "return_information") expect(input.information_request_id).toBe(baseline.current_information_request!.id);
  await expect(form).toHaveCount(0);
  return { receipt, input, requestId, baseline, selectedStatus: options.default_status };
}
