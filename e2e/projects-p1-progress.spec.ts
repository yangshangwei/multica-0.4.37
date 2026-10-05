import { expect } from "@playwright/test";
import { p1Capture, p1Member, p1Overview, p1Raw, type P1Project } from "./fixtures/project-p1";
import {
  test, addProgressEvidence, addProgressMention, correctProgressInUI, fillAcceptance,
  openProgress, passProgressDebounceWindow, previewProgress, progressAgentTaskCount,
  progressComposer, progressDraft, progressEditor, progressNotifications, progressOutboxCount,
  progressRevisions, progressUpdates, publishProgress, seedProgress, seedProgressAgent,
  seedProgressExecution, withProgressMemberPage, type ProgressPreview, type ProgressWrite,
} from "./fixtures/project-p1-progress";

// Each P-number owns its own workspace through the progress fixture. API writes
// below prepare prerequisites; the named publication/recovery/read action is UI.

test("P1-P01 mention preview identifies its recipient without publishing or executing", async ({ page, progress }) => {
  const { api, workspace, project } = progress;
  const reviewer = await p1Member(workspace);
  const { agent } = await seedProgressAgent(api);
  await openProgress(page, workspace, project);
  await progressEditor(page).fill("Review the customer delivery");
  await addProgressMention(page, "P1 reviewer");
  await addProgressMention(page, agent.name, true);
  const response = await previewProgress(page, project.id);
  expect(response.status()).toBe(200);
  const preview: ProgressPreview = await response.json();
  expect(preview.recipients.map((member) => member.id)).toEqual([reviewer.user.id]);
  expect(preview.draft.body).toContain(`mention://member/${reviewer.user.id}`);
  expect(preview.draft.body).toContain(`mention://agent/${agent.id}`);
  await expect(progressComposer(page).getByText("P1 reviewer", { exact: true })).toBeVisible();
  await expect(progressComposer(page).getByText("Agent mentions are references and do not start execution.", { exact: true })).toBeVisible();
  expect(await progressUpdates(api, project.id)).toEqual([]);
  expect(await progressOutboxCount(project.id, reviewer.user.id)).toBe(0);
  expect(await progressNotifications(reviewer.api, project.id)).toEqual([]);
  expect(await progressAgentTaskCount(agent.id)).toBe(0);
});

test("P1-P02 a fast trailing-space mention preview stays publishable past the editor debounce", async ({ page, progress }, info) => {
  const { api, workspace, project } = progress;
  const reviewer = await p1Member(workspace);
  const { agent } = await seedProgressAgent(api);
  await openProgress(page, workspace, project);
  await progressEditor(page).fill("Delivery verified");
  await addProgressMention(page, "P1 reviewer");
  await addProgressMention(page, agent.name, true);
  let requestedAt = 0;
  page.on("request", (request) => { if (new URL(request.url()).pathname.endsWith(`/projects/${project.id}/updates/preview`)) requestedAt = Date.now(); });
  const editedAt = Date.now();
  await progressEditor(page).pressSequentially(" ");
  const response = await previewProgress(page, project.id);
  expect(response.status()).toBe(200);
  expect(requestedAt - editedAt).toBeLessThan(300);
  const preview: ProgressPreview = await response.json();
  expect(preview.draft.body.endsWith(" ")).toBe(false);
  expect(preview.recipients.map((member) => member.id)).toEqual([reviewer.user.id]);
  await passProgressDebounceWindow(page);
  await expect(progressComposer(page).getByRole("button", { name: "Publish", exact: true })).toBeEnabled();
  await expect(progressEditor(page)).toHaveCount(0);
  await expect(progressComposer(page).getByText("P1 reviewer", { exact: true })).toBeVisible();
  expect(await progressUpdates(api, project.id)).toEqual([]);
  await p1Capture(page, info, "P02-normalized-mention-preview");
});

test("P1-P03 ordinary publication preserves its server statistics snapshot after issue changes", async ({ page, progress }) => {
  const { api, workspace, project, owner } = progress;
  const issue = await api.createIssue("Completed delivery fixture", { project_id: project.id, status: "done", assignee_type: "member", assignee_id: owner.id });
  const composer = await openProgress(page, workspace, project);
  await progressEditor(page).fill("Customer delivery is ready for review");
  await composer.getByRole("checkbox", { name: "Attach current system statistics", exact: true }).check();
  const previewResponse = await previewProgress(page, project.id);
  expect(previewResponse.status()).toBe(200);
  const preview: ProgressPreview = await previewResponse.json();
  expect(preview.statistics_snapshot?.counts).toMatchObject({ total: 1, completed: 1, cancelled: 0, open: 0 });
  expect((await publishProgress(page, project.id)).status()).toBe(201);
  await expect(composer).toHaveCount(0);
  const updates = await progressUpdates(api, project.id);
  expect(updates).toHaveLength(1);
  expect(updates[0]!.author.id).toBe(owner.id);
  expect(updates[0]!.current.body).toBe("Customer delivery is ready for review");
  expect(updates[0]!.current.statistics_snapshot?.counts.completed).toBe(1);
  await api.requestJSON(`/api/issues/${issue.id}`, { method: "PUT", body: { status: "todo" } });
  await page.reload();
  await expect(page.getByText("Statistics at publication", { exact: false })).toBeVisible();
  expect((await progressUpdates(api, project.id))[0]!.current.statistics_snapshot?.counts.completed).toBe(1);
  expect((await p1Overview(api, project.id)).statistics.counts.completed).toBe(0);
});

test("P1-P04 cancelling immediately retains the last characters when reopening and reloading", async ({ page, progress }) => {
  const { api, workspace, project } = progress;
  await openProgress(page, workspace, project);
  let writeRequests = 0;
  page.on("request", (request) => { if (new URL(request.url()).pathname.startsWith(`/api/projects/${project.id}/updates`) && request.method() !== "GET") writeRequests++; });
  const text = "Unpublished customer explanation — final characters";
  await progressEditor(page).fill(text);
  await progressComposer(page).getByRole("button", { name: "Cancel", exact: true }).click();
  await expect(progressComposer(page)).toHaveCount(0);
  await page.getByRole("button", { name: "Write progress", exact: true }).click();
  await expect(progressEditor(page)).toContainText(text);
  await page.reload();
  await page.getByRole("button", { name: "Write progress", exact: true }).click();
  await expect(progressEditor(page)).toContainText(text);
  expect(writeRequests).toBe(0);
  expect(await progressUpdates(api, project.id)).toEqual([]);
});

test("P1-P05 leaving the overview retains pending progress text without a publication", async ({ page, progress }) => {
  const { api, workspace, project } = progress;
  await openProgress(page, workspace, project);
  const text = "Navigation must retain this final draft sentence";
  await progressEditor(page).fill(text);
  await page.getByRole("button", { name: "Issues", exact: true }).click();
  await expect(progressComposer(page)).toHaveCount(0);
  await page.getByRole("button", { name: "Overview", exact: true }).click();
  await page.getByRole("button", { name: "Write progress", exact: true }).click();
  await expect(progressEditor(page)).toContainText(text);
  expect(await progressUpdates(api, project.id)).toEqual([]);
});

test("P1-P06 retrying a committed lost response reuses one intent, record and member notification", async ({ page, progress }, info) => {
  test.setTimeout(90_000);
  const { api, workspace, project } = progress;
  const reviewer = await p1Member(workspace);
  const execution = await seedProgressExecution(api);
  await openProgress(page, workspace, project);
  await progressEditor(page).fill("Confirm this delivery exactly once");
  await addProgressMention(page, "P1 reviewer");
  await addProgressEvidence(page, "execution", execution.id);
  await progressComposer(page).getByRole("checkbox", { name: "Attach current system statistics", exact: true }).check();
  expect((await previewProgress(page, project.id)).status()).toBe(200);
  const payloads: unknown[] = [];
  let committed: ProgressWrite | undefined;
  await page.route(`**/api/projects/${project.id}/updates`, async (route) => {
    if (route.request().method() !== "POST") return route.continue();
    payloads.push(route.request().postDataJSON());
    const response = await route.fetch();
    if (payloads.length === 1) {
      expect(response.status()).toBe(201); committed = await response.json();
      return route.abort("failed");
    }
    expect(response.status()).toBe(200);
    return route.fulfill({ response });
  });
  const composer = progressComposer(page);
  await composer.getByRole("button", { name: "Publish", exact: true }).click();
  await expect(composer.getByRole("alert")).toContainText("Publication has not been confirmed");
  expect(await progressUpdates(api, project.id)).toHaveLength(1);
  const retryResponse = page.waitForResponse((response) => new URL(response.url()).pathname === `/api/projects/${project.id}/updates` && response.request().method() === "POST");
  await composer.getByRole("button", { name: "Publish", exact: true }).click();
  const replay: ProgressWrite = await (await retryResponse).json();
  await expect(composer).toHaveCount(0);
  expect(payloads).toHaveLength(2); expect(payloads[1]).toEqual(payloads[0]);
  expect(replay).toMatchObject({ replayed: true, update_id: committed!.update_id, result_revision: 1 });
  expect(await progressUpdates(api, project.id)).toHaveLength(1);
  await expect.poll(async () => (await progressNotifications(reviewer.api, project.id)).length, { timeout: 15_000 }).toBe(1);
  expect(await progressOutboxCount(project.id, reviewer.user.id)).toBe(1);
  expect(await progressAgentTaskCount(execution.agent.id)).toBe(1);
  await info.attach("P06-identical-write-intents", { body: JSON.stringify(payloads, null, 2), contentType: "application/json" });
});

test("P1-P07 correction requires a reason and retains immutable original text and authorship", async ({ page, progress }) => {
  const { api, workspace, project, owner } = progress;
  const original = await seedProgress(api, project, progressDraft("Original delivery agreement"));
  const before = (await progressUpdates(api, project.id))[0]!;
  await page.goto(`/${workspace.slug}/projects/${project.id}?section=overview`);
  await correctProgressInUI(page, "Delivery agreement with corrected date");
  const invalid = await previewProgress(page, project.id);
  expect(invalid.status()).toBe(422);
  await expect(progressComposer(page).getByRole("alert")).toBeVisible();
  await expect(progressEditor(page)).toContainText("Delivery agreement with corrected date");
  expect((await progressRevisions(api, project.id, original.update_id)).map((revision) => revision.revision)).toEqual([1]);
  const reason = `Customer approved the correction on ${new Date().toISOString().slice(0, 10)}`;
  await progressComposer(page).getByRole("textbox", { name: "Reason for correction", exact: true }).fill(reason);
  expect((await previewProgress(page, project.id)).status()).toBe(200);
  expect((await publishProgress(page, project.id, original.update_id)).status()).toBe(200);
  await expect(progressComposer(page)).toHaveCount(0);
  await page.getByRole("button", { name: "Revision history", exact: true }).click();
  await expect(page.getByText("Original delivery agreement", { exact: true })).toBeVisible();
  await expect(page.getByText(reason, { exact: false }).first()).toBeVisible();
  const revisions = await progressRevisions(api, project.id, original.update_id);
  expect(revisions.map((revision) => revision.revision)).toEqual([2, 1]);
  expect(revisions[0]!.correction_reason).toBe(reason); expect(revisions[1]!.body).toBe("Original delivery agreement");
  expect(revisions[1]!.editor.id).toBe(owner.id);
  const after = (await progressUpdates(api, project.id))[0]!;
  expect(after.published_at).toBe(before.published_at); expect(after.author).toEqual(before.author);
});

test("P1-P08 stale correction compares the latest revision before an explicit retry", async ({ page, progress }) => {
  const { api, workspace, project } = progress;
  const colleague = await p1Member(workspace);
  const original = await seedProgress(api, project, progressDraft("Original status note"));
  await page.goto(`/${workspace.slug}/projects/${project.id}?section=overview`);
  await correctProgressInUI(page, "My reviewed correction", "Resolve the concurrent wording");
  await seedProgress(colleague.api, project, progressDraft("Colleague's intervening correction", { operation: "correct", update_id: original.update_id, expected_revision: 1, correction_reason: "Clarified by the reviewer" }));
  const conflict = await previewProgress(page, project.id);
  expect(conflict.status()).toBe(409);
  expect((await conflict.json()).code).toBe("project_update_revision_conflict");
  await expect(progressEditor(page)).toContainText("My reviewed correction");
  await expect(progressComposer(page).getByText("Colleague's intervening correction", { exact: true })).toBeVisible();
  await progressComposer(page).getByRole("button", { name: "Save reviewed draft", exact: true }).click();
  expect((await previewProgress(page, project.id)).status()).toBe(200);
  expect((await publishProgress(page, project.id, original.update_id)).status()).toBe(200);
  await expect(progressComposer(page)).toHaveCount(0);
  const revisions = await progressRevisions(api, project.id, original.update_id);
  expect(revisions.map((revision) => revision.revision)).toEqual([3, 2, 1]);
  expect(revisions[0]!.body).toBe("My reviewed correction"); expect(revisions[1]!.editor.id).toBe(colleague.user.id);
});

test("P1-P09 missing acceptance basis preserves input and can be corrected before publishing", async ({ page, progress }) => {
  const { api, workspace, project } = progress;
  const composer = await openProgress(page, workspace, project, true);
  await fillAcceptance(composer, "Customer goal accepted", "The customer delivery goal");
  expect((await previewProgress(page, project.id)).status()).toBe(422);
  await expect(composer.getByRole("alert")).toBeVisible();
  await expect(progressEditor(page)).toContainText("Customer goal accepted");
  await expect(composer.getByRole("textbox", { name: "Acceptance scope", exact: true })).toHaveValue("The customer delivery goal");
  expect(await progressUpdates(api, project.id)).toEqual([]);
  await composer.getByRole("textbox", { name: "Verifiable explanation", exact: true }).fill(`The signed checklist was reviewed on ${new Date().toISOString().slice(0, 10)}.`);
  expect((await previewProgress(page, project.id)).status()).toBe(200);
  expect((await publishProgress(page, project.id)).status()).toBe(201);
  await expect(composer).toHaveCount(0);
  expect((await p1Overview(api, project.id)).current_description_acceptance).toMatchObject({ conclusion: "passed", description_revision: project.description_revision });
  expect((await api.requestJSON<P1Project>(`/api/projects/${project.id}`)).status).toBe(project.status);
});

test("P1-P10 changed goals require explicit description adoption before acceptance publication", async ({ page, progress }) => {
  const { api, workspace, project } = progress;
  const composer = await openProgress(page, workspace, project, true);
  await fillAcceptance(composer, "My acceptance explanation remains intact", "Customer delivery", "Verified against the review checklist.");
  const changed = await api.requestJSON<P1Project>(`/api/projects/${project.id}`, { method: "PUT", body: { description: "Goal changed before acceptance preview", expected_description_revision: project.description_revision } });
  const conflict = await previewProgress(page, project.id);
  expect(conflict.status()).toBe(409); expect((await conflict.json()).code).toBe("project_description_conflict");
  await expect(progressEditor(page)).toContainText("My acceptance explanation remains intact");
  await expect(composer.getByText("Goal changed before acceptance preview", { exact: true })).toBeVisible();
  expect(await progressUpdates(api, project.id)).toEqual([]);
  await composer.getByRole("button", { name: "Review acceptance against this description", exact: true }).click();
  const preview = await previewProgress(page, project.id); expect(preview.status()).toBe(200);
  expect((await preview.json()).draft.expected_description_revision).toBe(changed.description_revision);
  expect((await publishProgress(page, project.id)).status()).toBe(201);
  await expect(composer).toHaveCount(0);
  expect((await p1Overview(api, project.id)).current_description_acceptance?.description_revision).toBe(changed.description_revision);
});

test("P1-P11 previous acceptance stays on its original goals and a new failed result can stand alone", async ({ page, progress }) => {
  const { api, workspace, project } = progress;
  const first = await seedProgress(api, project, progressDraft("Phase one accepted", { kind: "acceptance", expected_description_revision: project.description_revision, acceptance: { conclusion: "passed", scope: "Phase one", explanation: "Precondition: signed customer evidence" } }));
  const changed = await api.requestJSON<P1Project>(`/api/projects/${project.id}`, { method: "PUT", body: { description: "Phase two requires independent acceptance", expected_description_revision: project.description_revision } });
  await page.goto(`/${workspace.slug}/projects/${project.id}?section=overview`);
  await expect(page.getByText("For an older description; review again", { exact: true }).first()).toBeVisible();
  expect((await p1Overview(api, project.id)).current_description_acceptance).toBeNull();
  await page.getByRole("button", { name: "Record acceptance", exact: true }).click();
  const composer = progressComposer(page);
  await fillAcceptance(composer, "Phase two has not passed", "Phase two");
  await composer.getByRole("combobox", { name: "Acceptance", exact: true }).selectOption("failed");
  expect((await previewProgress(page, project.id)).status()).toBe(200);
  expect((await publishProgress(page, project.id)).status()).toBe(201);
  await expect(composer).toHaveCount(0);
  expect((await p1Overview(api, project.id)).current_description_acceptance).toMatchObject({ conclusion: "failed", description_revision: changed.description_revision });
  expect((await progressRevisions(api, project.id, first.update_id))[0]!.acceptance).toMatchObject({ conclusion: "passed", description_revision: project.description_revision, description_snapshot: project.description });
});

test("P1-P12 execution evidence opens its real authorized revision-scoped transcript", async ({ page, progress }, info) => {
  const { api, workspace, project } = progress;
  const execution = await seedProgressExecution(api);
  await openProgress(page, workspace, project);
  await progressEditor(page).fill("Review the completed synthetic execution");
  await addProgressEvidence(page, "execution", execution.id);
  const preview = await previewProgress(page, project.id); expect(preview.status()).toBe(200);
  expect((await preview.json()).evidence_versions).toEqual([expect.objectContaining({ kind: "execution", id: execution.id })]);
  expect((await publishProgress(page, project.id)).status()).toBe(201);
  await expect(progressComposer(page)).toHaveCount(0);
  const update = (await progressUpdates(api, project.id))[0]!;
  const read = page.waitForResponse((response) => response.url().endsWith(`/updates/${update.id}/revisions/1/executions/${execution.id}`));
  await page.getByRole("button", { name: `Execution ${execution.id}`, exact: true }).click();
  const response = await read; expect(response.status()).toBe(200);
  expect(await response.json()).toMatchObject({ workspace_id: workspace.id, project_id: project.id, update_id: update.id, revision: 1, task: { id: execution.id, status: "completed" } });
  await expect(page.getByRole("dialog")).toBeVisible();
  await p1Capture(page, info, "P12-authorized-execution-transcript");
  await page.keyboard.press("Escape"); await expect(page.getByRole("dialog")).toHaveCount(0);
  expect(await progressAgentTaskCount(execution.agent.id)).toBe(1);
});

test("P1-P13 a real notification opens an older update without exposing its private execution", async ({ page, browser, progress }, info) => {
  test.setTimeout(120_000);
  const { api, workspace, project } = progress;
  const reviewer = await p1Member(workspace);
  const execution = await seedProgressExecution(api);
  const targetBody = `Old notification target [@P1 reviewer](mention://member/${reviewer.user.id})`;
  const target = await seedProgress(api, project, progressDraft(targetBody, { evidence: [{ kind: "execution", id: execution.id, url: null }] }));
  for (let index = 0; index < 20; index++) await seedProgress(api, project, progressDraft(`Later unrelated update ${index + 1}`));
  expect((await progressUpdates(api, project.id)).some((update) => update.id === target.update_id)).toBe(false);
  await expect.poll(async () => (await progressNotifications(reviewer.api, project.id)).length, { timeout: 15_000 }).toBe(1);
  const notification = (await progressNotifications(reviewer.api, project.id))[0]!;
  expect(notification.details).toMatchObject({ project_id: project.id, update_id: target.update_id, revision: "1" });
  await withProgressMemberPage(browser, reviewer.api, info, async (memberPage) => {
    await memberPage.goto(`/${workspace.slug}/inbox?issue=${notification.id}`);
    await memberPage.getByRole("button", { name: "Open project update", exact: true }).click();
    await expect(memberPage).toHaveURL(new RegExp(`update=${target.update_id}`));
    expect(new URL(memberPage.url()).pathname).toBe(`/${workspace.slug}/projects/${project.id}`);
    await expect(memberPage.getByText(targetBody, { exact: true }).first()).toBeVisible();
    await expect(memberPage.getByRole("button", { name: `Execution ${execution.id}`, exact: true })).toHaveCount(0);
    const denied = await p1Raw(reviewer.api, workspace.id, `/api/projects/${project.id}/updates/${target.update_id}/revisions/1/executions/${execution.id}`, "GET");
    expect(denied.status).toBe(403); expect((await denied.json()).code).toBe("project_evidence_forbidden");
    await expect(memberPage.getByRole("button", { name: "Write progress", exact: true })).toBeVisible();
    await p1Capture(memberPage, info, "P13-private-notification-deep-link");
  });
  // Keep the owning context on a meaningful page for teardown diagnostics.
  await page.goto(`/${workspace.slug}/projects/${project.id}?section=overview`);
});

test("P1-P14 source-level403 preserves the member's draft and permits publication after evidence removal", async ({ browser, progress }, info) => {
  const { api, workspace, project } = progress;
  const reviewer = await p1Member(workspace);
  const execution = await seedProgressExecution(api, reviewer.user.id);
  await withProgressMemberPage(browser, reviewer.api, info, async (memberPage) => {
    const composer = await openProgress(memberPage, workspace, project);
    const body = "Keep my independent explanation when source access changes";
    await progressEditor(memberPage).fill(body);
    await addProgressEvidence(memberPage, "execution", execution.id);
    expect((await previewProgress(memberPage, project.id)).status()).toBe(200);
    await composer.getByRole("button", { name: "Back to editing", exact: true }).click();
    await api.requestJSON(`/api/agents/${execution.agent.id}`, { method: "PUT", body: { permission_mode: "private", invocation_targets: [] } });
    const denied = await previewProgress(memberPage, project.id);
    expect(denied.status()).toBe(403); expect((await denied.json()).code).toBe("project_evidence_forbidden");
    await expect(composer.getByRole("alert")).toContainText("Your text is kept");
    await expect(progressEditor(memberPage)).toContainText(body);
    expect((await reviewer.api.requestJSON<P1Project>(`/api/projects/${project.id}`)).id).toBe(project.id);
    expect(await progressUpdates(reviewer.api, project.id)).toEqual([]);
    await composer.getByRole("button", { name: "Remove", exact: true }).click();
    expect((await previewProgress(memberPage, project.id)).status()).toBe(200);
    expect((await publishProgress(memberPage, project.id)).status()).toBe(201);
    await expect(composer).toHaveCount(0);
    const record = (await progressUpdates(reviewer.api, project.id))[0]!;
    expect(record.current.body).toBe(body); expect(record.current.evidence).toEqual([]);
    expect(record.author.id).toBe(reviewer.user.id);
  });
  expect(await progressAgentTaskCount(execution.agent.id)).toBe(1);
});

test("P1-P15 correcting repeated mentions neither duplicates notifications nor starts an agent", async ({ page, progress }) => {
  const { api, workspace, project } = progress;
  const reviewer = await p1Member(workspace);
  const { agent } = await seedProgressAgent(api);
  const first = await seedProgress(api, project, progressDraft(`Initial recipient [@P1 reviewer](mention://member/${reviewer.user.id})`));
  await expect.poll(async () => (await progressNotifications(reviewer.api, project.id)).length, { timeout: 15_000 }).toBe(1);
  await page.goto(`/${workspace.slug}/projects/${project.id}?section=overview`);
  await correctProgressInUI(page, "Same audience with an agent reference", "Retain the existing recipient without notifying twice");
  await addProgressMention(page, "P1 reviewer");
  await addProgressMention(page, agent.name, true);
  const preview = await previewProgress(page, project.id); expect(preview.status()).toBe(200);
  expect((await preview.json()).recipients.map((member: { id: string }) => member.id)).toEqual([reviewer.user.id]);
  expect((await publishProgress(page, project.id, first.update_id)).status()).toBe(200);
  await expect(progressComposer(page)).toHaveCount(0);
  expect((await progressUpdates(api, project.id))[0]!.current_revision).toBe(2);
  expect(await progressOutboxCount(project.id, reviewer.user.id)).toBe(1);
  expect(await progressNotifications(reviewer.api, project.id)).toHaveLength(1);
  expect(await progressAgentTaskCount(agent.id)).toBe(0);
  expect((await api.requestJSON<P1Project>(`/api/projects/${project.id}`)).status).toBe(project.status);
});
