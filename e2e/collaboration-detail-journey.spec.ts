import { expect, test } from "@playwright/test";
import { writeFileSync } from "node:fs";
import type { CollaborationIssue, CollaborationSubmission } from "@multica/core/collaboration";
import { parseAccountPath } from "@multica/core/paths";
import { nativeScreenshot } from "./fixtures/collaboration-native";
import {
  assertJourneyScopeRestoration, captureJourney, changeJourneySourceComment, chooseJourneyRecipient,
  journeyCopy, nativeJourneyEvidence, openJourneyAction, openJourneySession, publishJourneyComment,
  readJourneyComment, readJourneyHistory, readJourneyIssue, refreshJourneyIssue, refreshJourneyIssueInApp, seedDetailJourney,
  submissionRecord, submitJourneyAction, submitJourneyWithLostResponse, switchJourneyLocale, visibleJourneySubmission,
  type DetailJourneySession, type OrdinaryJourneyIssue,
} from "./fixtures/collaboration-detail-journey";

test.use({ actionTimeout: 15_000, trace: "off", screenshot: "off" });

// Server delivery/policy suites own role, status, legacy and admission matrices.
// Core delivery/action/comment suites own parsing and captured-intent matrices.
// This spec owns the complete public Web and native client entry-point journeys.
function noAIWork(issue: CollaborationIssue) {
  expect(issue.ai_policy).toBe("manual");
  expect(issue.capabilities.submit_command).toBe(false);
  expect(issue.execution).not.toBeNull();
  expect(issue.execution!.targets).toEqual([]);
  expect(issue.execution!.channels).toEqual([]);
  expect(issue.execution!.commands).toEqual([]);
  expect(issue.execution!.groups).toEqual([]);
  expect(issue.execution!.runs).toEqual([]);
  expect(issue.execution!.execution_gate.enabled).toBe(false);
}

function immutableResult(submission: CollaborationSubmission) {
  const { decision: _decision, ...result } = submission;
  return result;
}

for (const platform of ["web", "desktop"] as const) {
  test(`${platform === "web" ? "Production Web" : "Electron Desktop"} manual detail: information, delivery, revision, resubmission and exact acceptance`, async ({ browser }, info) => {
    test.setTimeout(480_000);
    test.skip(!process.env.COLLABORATION_DETAIL_PROVENANCE_FILE,
      "Requires the root session's task-owned production runtime permit");
    const fixture = await seedDetailJourney();
    const id = fixture.shared.issue.id;
    const sessions: DetailJourneySession[] = [];
    const evidence: Record<string, unknown> = { platform, issue_id: id, source_workspace_id: fixture.workspace.id,
      creator_id: fixture.creator.id, external_handler_id: fixture.handler.id, parent_id: fixture.parent.id,
      creation_request_id: fixture.shared.requestId, runtime: fixture.provenance };
    const captures: Awaited<ReturnType<typeof captureJourney>>[] = [];
    let failed = false;
    try {
      const creator = await openJourneySession(browser, fixture, platform, fixture.creator);
      sessions.push(creator);
      const handler = await openJourneySession(browser, fixture, platform, fixture.handler);
      sessions.push(handler);
      const copy = journeyCopy("en");
      for (const session of sessions) await expect(session.page.getByRole("heading", { name: fixture.shared.issue.title, exact: true })).toBeVisible();

      await test.step("A01/A17: manual external handler, compact context and ordered content", async () => {
        const initial = await readJourneyIssue(fixture.creator, id);
        expect(initial).toMatchObject({ handler: { user_id: fixture.handler.id, external: true }, creator: { user_id: fixture.creator.id },
          source_workspace: { id: fixture.workspace.id }, delivery: { current: null, latest: null, has_history: false } });
        noAIWork(initial);
        expect(initial.source_structure?.parent_issue?.id).toBe(fixture.parent.id);
        expect((await readJourneyIssue(fixture.handler, id)).capabilities.edit_source_structure).toBe(false);
        await expect(handler.page.getByRole("button", { name: copy.actions.change_status, exact: true })).toBeVisible();
        await expect(handler.page.getByRole("region", { name: copy.ai, exact: true })).toHaveCount(0);
        await handler.page.getByRole("button", { name: copy.context, exact: true }).click();
        const context = handler.page.getByRole("dialog", { name: copy.context, exact: true });
        await expect(context).toContainText(copy.manual);
        await handler.page.keyboard.press("Escape");
        const participants = handler.page.getByRole("button", { name: "Participants", exact: true });
        await participants.focus();
        await participants.press("Enter");
        await expect(participants).toHaveAttribute("aria-expanded", "true");
        const people = handler.page.getByRole("list", { name: "Participants", exact: true });
        await expect(people).toContainText(fixture.creator.name);
        await expect(people).toContainText(fixture.handler.name);
        const order = await handler.page.evaluate(() => {
          const requirements = document.getElementById("description")!;
          const delivery = document.getElementById("delivery")!;
          const discussion = document.getElementById("discussion")!;
          return { requirementsBeforeResult: !!(requirements.compareDocumentPosition(delivery) & Node.DOCUMENT_POSITION_FOLLOWING),
            resultBeforeDiscussion: !!(delivery.compareDocumentPosition(discussion) & Node.DOCUMENT_POSITION_FOLLOWING) };
        });
        expect(order).toEqual({ requirementsBeforeResult: true, resultBeforeDiscussion: true });
        captures.push(await captureJourney(handler, info, "manual-before-wide", 1280, {
          description: "manual task title, responsibility, actions and participants",
          anchors: [handler.page.getByRole("heading", { name: fixture.shared.issue.title, exact: true }).locator("..").locator("..")],
        }));
      });

      const started = await test.step("The current handler starts through the offered workflow", async () => {
        const form = await openJourneyAction(handler, id, "change_status");
        const result = await submitJourneyAction(form);
        expect(result.receipt.code).toBe("status_changed");
        const current = await readJourneyIssue(fixture.handler, id);
        expect(current).toMatchObject({ id, status_category: "in_progress", handler: { user_id: fixture.handler.id } });
        expect(current.session).toEqual(fixture.shared.issue.session);
        noAIWork(current);
        evidence.started = result;
        return current;
      });

      const information = await test.step("A02: request information records the exact requester and recipient", async () => {
        const form = await openJourneyAction(handler, id, "request_information");
        await chooseJourneyRecipient(form, fixture.creator);
        const note = "Confirm the expected retry behavior before I deliver the result.";
        await form.form.getByLabel(copy.note, { exact: true }).fill(note);
        const requested = await submitJourneyAction(form);
        expect(requested.receipt.code).toBe("information_requested");
        expect(requested.input).toMatchObject({ purpose: "information", target_user_id: fixture.creator.id, note, expected_status: form.options.default_status });
        const current = await readJourneyIssue(fixture.creator, id);
        expect(current).toMatchObject({ status_category: "blocked", handler: { user_id: fixture.creator.id },
          current_information_request: { requester: { user_id: fixture.handler.id }, recipient: { user_id: fixture.creator.id }, note } });
        expect(current.current_information_request!.id).toMatch(/^[a-f0-9-]{36}$/i);
        expect(current.session).toEqual(started.session);
        noAIWork(current);
        evidence.information_requested = requested;
        return current.current_information_request!;
      });

      await test.step("A09: one Publish performs a real preflight without returning information or starting AI", async () => {
        const before = await refreshJourneyIssue(creator, id);
        const comment = await publishJourneyComment(creator, id, "Use the existing retry rule. This comment is background material, not an information return.");
        expect(comment.preview).toMatchObject({ issue_id: id, targets: [] });
        expect(comment.receipt).toMatchObject({ issue_id: id, code: "comment_created", targets: [] });
        expect(comment.writes).toEqual([{ path: `/api/collaboration/issues/${id}/comments/trigger-preview`, method: "POST" },
          { path: `/api/collaboration/issues/${id}/comments`, method: "POST" }]);
        expect(comment.input).toMatchObject({ expected_lifecycle_epoch: comment.preview.lifecycle_epoch,
          expected_participant_epoch: comment.preview.write_context!.participant_epoch, expected_account_epoch: comment.preview.write_context!.account_epoch });
        const after = await readJourneyIssue(fixture.creator, id);
        expect(after.handler).toEqual(before.handler);
        expect(after.status).toBe(before.status);
        expect(after.current_information_request?.id).toBe(information.id);
        expect(after.session).toEqual(before.session);
        noAIWork(after);
        evidence.ordinary_comment = comment;
      });

      await test.step("A02: the receiving form returns the exact saved information request", async () => {
        const form = await openJourneyAction(creator, id, "return_information");
        await expect(form.form.getByText(copy.requestedBy.replace("{{name}}", fixture.handler.name), { exact: true })).toBeVisible();
        await expect(form.form).toContainText(information.note);
        await form.form.getByLabel(copy.note, { exact: true }).fill("Confirmed: retry once, then retain the original failure for review.");
        const returned = await submitJourneyAction(form);
        expect(returned.receipt.code).toBe("information_returned");
        expect(returned.input.information_request_id).toBe(information.id);
        expect(returned.input.expected_status).toBe(started.status);
        const current = await readJourneyIssue(fixture.handler, id);
        expect(current).toMatchObject({ current_information_request: null, handler: { user_id: fixture.handler.id }, status: started.status });
        expect(current.delivery?.current).toBeNull();
        noAIWork(current);
        evidence.information_returned = returned;
      });

      const resultText = Array.from({ length: 7 }, (_, index) => `Result v1 paragraph ${index + 1}: the retry keeps the source task identity and reports the observed failure after one further attempt. The delivered snapshot must survive later comment changes. 长结果应在窄窗口中正常换行。`).join("\n\n");
      const basisText = "Basis v1: the documented retry rule and a reproducible text-only verification. Both attempts retained the same source UUID; the second failure remained visible. No file is required for this result.";
      const sourceComment = await test.step("A03/A12: a comment prepares a result draft only after an explicit action", async () => {
        await refreshJourneyIssue(handler, id);
        const comment = await publishJourneyComment(handler, id, resultText);
        expect(comment.preview.targets).toEqual([]);
        expect(comment.receipt.targets).toEqual([]);
        expect((await readJourneyIssue(fixture.handler, id)).delivery?.current).toBeNull();
        expect((await readJourneyIssue(fixture.handler, id)).handler.user_id).toBe(fixture.handler.id);
        const form = await openJourneyAction(handler, id, "submit_acceptance", { commentId: comment.receipt.comment_id });
        const result = form.form.getByLabel(copy.result, { exact: true });
        const basis = form.form.getByLabel(copy.basis, { exact: true });
        await expect(result).toHaveValue(resultText);
        await expect(basis).toHaveValue("");
        await expect(form.form.getByRole("button", { name: form.label, exact: true })).toBeDisabled();
        await basis.focus();
        await expect(basis).toBeFocused();
        await basis.fill(basisText);
        expect((await readJourneyIssue(fixture.handler, id)).delivery?.current).toBeNull();
        const submitted = await submitJourneyWithLostResponse(form);
        expect(submitted.receipt.code).toBe("acceptance_submitted");
        expect(submitted.input).toMatchObject({ note: resultText, basis: basisText, source_comment_id: comment.receipt.comment_id, attachment_ids: [] });
        expect(submitted.receipt.delivery_confirmation).toEqual({ submission_id: submitted.afterReload.delivery!.current!.id, attachment_ids: [] });
        expect(submitted.afterReload).toMatchObject({ status_category: "in_review", handler: { user_id: fixture.creator.id } });
        expect(submitted.postCount).toBe(1);
        noAIWork(submitted.afterReload);
        evidence.first_submission = submitted;
        evidence.source_comment_id = comment.receipt.comment_id;
        return { comment, submission: submitted.afterReload.delivery!.current! };
      });
      const first = sourceComment.submission;
      expect(first).toMatchObject({ issue_id: id, author_user_id: fixture.handler.id, author: { user_id: fixture.handler.id, name: fixture.handler.name },
        result: resultText, basis: basisText, evidence_state: "available", attachment_ids: [], source_comment_id: sourceComment.comment.receipt.comment_id, decision: null });

      await test.step("A07/A12: author discussion remains available and source-comment changes preserve the delivery", async () => {
        await expect(handler.page.getByRole("button", { name: copy.actions.complete, exact: true })).toHaveCount(0);
        const record = await visibleJourneySubmission(handler, first);
        await expect(record).toContainText("Result v1 paragraph 7");
        const changed = await changeJourneySourceComment(handler, id, sourceComment.comment.receipt.comment_id, "The source comment was edited after formal submission.");
        const tombstone = await readJourneyComment(fixture.handler, id, sourceComment.comment.receipt.comment_id);
        expect(tombstone.deleted_at).not.toBeNull();
        expect(tombstone.content).toBe("");
        const preserved = (await readJourneyIssue(fixture.creator, id)).delivery!.current!;
        expect(preserved).toEqual(first);
        expect(await handler.page.locator(`#comment-${tombstone.id}`).count()).toBe(1);
        evidence.source_comment_changed = changed;
      });

      const returnedFirst = await test.step("A04: initiator returns the exact submission to its explicitly selected actual author", async () => {
        const form = await openJourneyAction(creator, id, "return_acceptance");
        await expect(form.form.getByText(copy.selected.replace("{{name}}", fixture.handler.name), { exact: true })).toBeVisible();
        await expect(form.form.locator(`[data-submission-id="${first.id}"]`)).toHaveCount(1);
        await chooseJourneyRecipient(form, fixture.handler);
        const reason = "Add the recovery result and explain how it differs from the first delivered outcome.";
        await form.form.getByLabel(copy.revisionReason, { exact: true }).fill(reason);
        const returned = await submitJourneyAction(form);
        expect(returned.receipt.code).toBe("acceptance_returned");
        expect(returned.input).toMatchObject({ submission_id: first.id, target_user_id: fixture.handler.id, note: reason, expected_status: form.options.default_status });
        const current = await readJourneyIssue(fixture.handler, id);
        expect(current).toMatchObject({ status_category: "in_progress", handler: { user_id: fixture.handler.id }, delivery: { current: null } });
        const prior = current.delivery!.latest!;
        expect(immutableResult(prior)).toEqual(immutableResult(first));
        expect(prior.decision).toMatchObject({ kind: "returned", submission_id: first.id, actor_user_id: fixture.creator.id,
          actor: { name: fixture.creator.name }, reason, target_user_id: fixture.handler.id });
        expect(BigInt(prior.decision!.event_seq)).toBeGreaterThan(BigInt(first.event_seq));
        noAIWork(current);
        evidence.acceptance_returned = returned;
        return prior;
      });

      const second = await test.step("A04: resubmission creates a new exact result and retains the first decision", async () => {
        const form = await openJourneyAction(handler, id, "submit_acceptance");
        const result = "Result v2: recovery was verified against the same task UUID. A successful retry records the recovered outcome; a second failure keeps the original failure visible. The first submitted snapshot and the initiator's revision remain readable. 修改后的结果仍保留第一版交付与退回原因。";
        const basis = "Basis v2: checked the successful-retry and repeated-failure outcomes against the agreed one-retry rule. The verification used text only, with no uploaded file and no AI execution.";
        await form.form.getByLabel(copy.result, { exact: true }).fill(result);
        await form.form.getByLabel(copy.basis, { exact: true }).fill(basis);
        const submitted = await submitJourneyAction(form);
        expect(submitted.receipt.code).toBe("acceptance_submitted");
        expect(submitted.input).toMatchObject({ note: result, basis, attachment_ids: [] });
        expect(submitted.input).not.toHaveProperty("source_comment_id");
        const current = await readJourneyIssue(fixture.creator, id);
        const latest = current.delivery!.current!;
        expect(latest.id).not.toBe(first.id);
        expect(latest).toMatchObject({ issue_id: id, author_user_id: fixture.handler.id, author: { name: fixture.handler.name },
          result, basis, source_comment_id: null, attachment_ids: [], evidence_state: "available", decision: null });
        expect(submitted.receipt.delivery_confirmation).toEqual({ submission_id: latest.id, attachment_ids: [] });
        const history = await readJourneyHistory(fixture.creator, id);
        expect(history.items.map((item) => item.id)).toEqual([latest.id, first.id]);
        expect(history.items[1]).toEqual(returnedFirst);
        noAIWork(current);
        evidence.resubmitted = submitted;
        return latest;
      });

      const completed = await test.step("A05/A15/A16: accept the exact result from the same source issue without completing the ordinary parent", async () => {
        await creator.openSource(id, fixture.shared.issue.title);
        await expect(creator.page.getByRole("heading", { name: fixture.shared.issue.title, exact: true })).toBeVisible();
        await visibleJourneySubmission(creator, second);
        const form = await openJourneyAction(creator, id, "complete");
        await expect(form.form.locator(`[data-submission-id="${second.id}"]`)).toHaveCount(1);
        const accepted = await submitJourneyAction(form);
        expect(accepted.receipt.code).toBe("completed");
        expect(accepted.input).toMatchObject({ action: "complete", submission_id: second.id, status: form.options.default_status });
        expect(accepted.input).not.toHaveProperty("basis");
        expect(accepted.receipt.delivery_confirmation).toEqual({ submission_id: second.id, attachment_ids: [] });
        const done = await readJourneyIssue(fixture.creator, id);
        expect(done).toMatchObject({ id, status_category: "done", handler: { user_id: fixture.creator.id }, delivery: { current: null }, session: { queue_state: "closed" } });
        const result = done.delivery!.latest!;
        expect(immutableResult(result)).toEqual(immutableResult(second));
        expect(result.decision).toMatchObject({ kind: "accepted", submission_id: second.id, actor_user_id: fixture.creator.id, actor: { name: fixture.creator.name }, target_user_id: null });
        await visibleJourneySubmission(creator, result);
        const source = await fixture.creator.client.requestJSON<OrdinaryJourneyIssue>(`/api/issues/${id}`);
        expect(source).toMatchObject({ id, status: done.status });
        const parent = await fixture.creator.client.requestJSON<OrdinaryJourneyIssue>(`/api/issues/${fixture.parent.id}`);
        expect(parent).toMatchObject({ id: fixture.parent.id, status: fixture.parent.status });
        expect(parent.status).toBe("in_progress");
        expect((await readJourneyIssue(fixture.handler, id)).delivery).toEqual(done.delivery);
        noAIWork(done);
        evidence.accepted = accepted;
        evidence.source_entry = { id: source.id, identifier: source.identifier, status: source.status };
        evidence.ordinary_parent = { id: parent.id, status: parent.status };
        return done;
      });

      await test.step("A12/A17: retained results, exact source-comment anchors and restored account scroll", async () => {
        await creator.openAccount(id);
        await expect(creator.page.getByRole("heading", { name: fixture.shared.issue.title, exact: true })).toBeVisible();
        const refreshed = await refreshJourneyIssueInApp(creator, id);
        expect(refreshed).toMatchObject({ id, status: completed.status });
        expect(refreshed.delivery).toEqual(completed.delivery);
        await visibleJourneySubmission(creator, completed.delivery!.latest!);
        await creator.page.locator("#delivery").getByRole("button", { name: copy.history, exact: true }).click();
        const prior = await visibleJourneySubmission(creator, returnedFirst);
        await expect(prior).toContainText("Result v1 paragraph 7");
        const link = prior.getByRole("link", { name: copy.sourceComment, exact: true });
        const href = await link.getAttribute("href");
        if (!href) throw new Error("The retained result has no source comment link");
        const normalized = new URL(href, fixture.provenance.production_web_url);
        expect(normalized.origin).toBe(platform === "desktop" ? fixture.provenance.api_url : fixture.provenance.production_web_url);
        expect(normalized.pathname).toBe("/account");
        expect(normalized.searchParams.get("issue")).toBe(id);
        expect(parseAccountPath(`${normalized.pathname}${normalized.search}${normalized.hash}`)).toMatchObject({
          issueId: id, commentId: sourceComment.comment.receipt.comment_id,
        });
        await link.click();
        const anchor = creator.page.locator(`#comment-${sourceComment.comment.receipt.comment_id}`);
        await expect(anchor).toBeFocused();
        await expect(anchor.getByText(copy.deleted, { exact: true })).toBeVisible();
        // Repeat the same link after moving focus, proving a fresh navigation
        // intent can consume an already visited comment anchor.
        await creator.page.locator("#description").focus();
        const history = creator.page.locator("#delivery").getByRole("button", { name: copy.history, exact: true });
        await expect(history).toBeVisible();
        if (await history.getAttribute("aria-expanded") !== "true") await history.click();
        await submissionRecord(creator, first.id).getByRole("link", { name: copy.sourceComment, exact: true }).click();
        await expect(anchor).toBeFocused();
        evidence.restored_scroll = await assertJourneyScopeRestoration(creator, id, fixture.shared.issue.title);
        expect((await readJourneyHistory(fixture.creator, id)).items[1]).toEqual(returnedFirst);
      });

      await test.step("A17: both locales and long content at wide, intermediate and narrow sizes", async () => {
        for (const locale of ["en", "zh-Hans"] as const) {
          await switchJourneyLocale(creator, fixture.creator, id, locale);
          const copy = journeyCopy(locale);
          const record = submissionRecord(creator, completed.delivery!.latest!.id);
          const accepted = record.getByText(copy.accepted, { exact: true }).locator("..");
          const result = record.getByRole("heading", { name: copy.result, exact: true }).locator("..");
          const basis = record.getByRole("heading", { name: copy.basis, exact: true }).locator("..");
          for (const [size, width] of [["wide", 1280], ["intermediate", 900], ["narrow", 430]] as const) {
            captures.push(await captureJourney(creator, info, `accepted-${locale}-${size}`, width, {
              description: `${size === "wide" ? "complete accepted delivery" : "accepted decision"} ${completed.delivery!.latest!.id}`,
              anchors: size === "wide" ? [creator.page.locator("#delivery > h3"), record] : [accepted],
            }));
            captures.push(await captureJourney(creator, info, `result-${locale}-${size}`, width, {
              description: `result and basis ${completed.delivery!.latest!.id}`, anchors: [result, basis],
            }));
            await visibleJourneySubmission(creator, completed.delivery!.latest!);
          }
          await creator.page.locator("#delivery").getByRole("button", { name: journeyCopy(locale).history, exact: true }).click();
          const prior = await visibleJourneySubmission(creator, returnedFirst);
          captures.push(await captureJourney(creator, info, `long-result-${locale}-narrow-start`, 430, {
            description: `prior result heading and first paragraph ${returnedFirst.id}`,
            anchors: [prior.getByRole("heading", { name: copy.result, exact: true }),
              prior.locator("p").filter({ hasText: "Result v1 paragraph 1:" }).first()],
          }));
          captures.push(await captureJourney(creator, info, `long-result-${locale}-narrow-end`, 430, {
            description: `prior result final paragraph ${returnedFirst.id}`,
            anchors: [prior.locator("p").filter({ hasText: "Result v1 paragraph 7:" }).first()],
          }));
        }
      });

      await test.step("A06/A17: the initiator records a text-only result and completes atomically in Chinese", async () => {
        await creator.openAccount(fixture.self.issue.id);
        await switchJourneyLocale(creator, fixture.creator, fixture.self.issue.id, "zh-Hans");
        const zh = journeyCopy("zh-Hans");
        const form = await openJourneyAction(creator, fixture.self.issue.id, "complete", { self: true });
        const submit = form.form.getByRole("button", { name: zh.selfComplete, exact: true });
        await expect(submit).toBeDisabled();
        await form.form.getByLabel(zh.result, { exact: true }).fill("已记录本人处理的结果，任务身份保持不变。");
        await expect(submit).toBeDisabled();
        const basis = form.form.getByLabel(zh.basis, { exact: true });
        await basis.focus();
        await basis.fill("依据：核对结果与任务要求一致，本次无需上传文件。");
        const self = await submitJourneyAction(form);
        expect(self.receipt.code).toBe("completed");
        expect(self.input).toMatchObject({ action: "complete", note: "已记录本人处理的结果，任务身份保持不变。", basis: "依据：核对结果与任务要求一致，本次无需上传文件。", attachment_ids: [] });
        expect(self.input).not.toHaveProperty("submission_id");
        expect(self.input).not.toHaveProperty("target_user_id");
        const issue = await readJourneyIssue(fixture.creator, fixture.self.issue.id);
        const result = issue.delivery!.latest!;
        expect(issue).toMatchObject({ status_category: "done", handler: { user_id: fixture.creator.id }, delivery: { current: null } });
        expect(result).toMatchObject({ author_user_id: fixture.creator.id, evidence_state: "available", attachment_ids: [],
          decision: { kind: "accepted", actor_user_id: fixture.creator.id, submission_id: result.id } });
        expect(self.receipt.delivery_confirmation).toEqual({ submission_id: result.id, attachment_ids: [] });
        const history = await readJourneyHistory(fixture.creator, issue.id);
        expect(history.items).toEqual([result]);
        await visibleJourneySubmission(creator, result);
        noAIWork(issue);
        evidence.self_completed = { operation: self, submission: result };
        captures.push(await captureJourney(creator, info, "self-completed-zh-Hans-narrow", 430, {
          description: `self result and accepted decision ${result.id}`,
          anchors: [creator.page.locator("#delivery > h3"), submissionRecord(creator, result.id)],
        }));
        const reopenForm = await openJourneyAction(creator, issue.id, "reopen");
        const reopen = await submitJourneyAction(reopenForm);
        expect(reopen.receipt.code).toBe("reopened");
        const reopened = await readJourneyIssue(fixture.creator, issue.id);
        expect(reopened.status_category).not.toBe("done");
        expect(BigInt(reopened.lifecycle_epoch)).toBeGreaterThan(BigInt(issue.lifecycle_epoch));
        expect(reopened.delivery).toEqual({ current: null, latest: result, has_history: true });
        expect((await readJourneyHistory(fixture.creator, issue.id)).items).toEqual([result]);
        await visibleJourneySubmission(creator, result);
        noAIWork(reopened);
        evidence.self_reopened = { operation: reopen, retained_submission_id: result.id, status: reopened.status,
          lifecycle_epoch: reopened.lifecycle_epoch, original_accepted_decision: result.decision };
        captures.push(await captureJourney(creator, info, "self-reopened-zh-Hans-narrow", 430, {
          description: `retained self result and decision after reopening ${result.id}`,
          anchors: [creator.page.locator("#delivery > h3"), submissionRecord(creator, result.id)],
        }));
      });

      expect(await fixture.handler.client.getWorkspaces()).toEqual([]);
      expect(await fixture.creator.client.countIssueDispatches(id)).toBe(0);
      expect(await fixture.creator.client.countIssueDispatches(fixture.self.issue.id)).toBe(0);
      for (const session of sessions) expect(session.errors).toEqual([]);
      evidence.native = await nativeJourneyEvidence(creator);
      evidence.native_handler = await nativeJourneyEvidence(handler);
      evidence.completed_delivery = completed.delivery;
      evidence.captures = captures;
      evidence.coverage = { full_client_journey: ["A01", "A02", "A03", "A04", "A05", "A06", "A09", "A17"],
        focused_client_wiring: { A07: "former handler has no acceptance control", A12: "source-comment edit/delete retains the result snapshot",
          A14: "committed result response loss; reload and exact original-key GET", A15: "external actor remains outside the source workspace",
          A16: "ordinary parent status stays independent" },
        canonical_server_core_matrices: ["A07", "A08", "A10", "A11", "A12", "A13", "A14", "A15", "A16"] };
    } catch (error) {
      failed = true;
      throw error;
    } finally {
      evidence.captures = captures;
      try {
        if (failed) {
          const failures = await Promise.allSettled(sessions.map(async (session, index) => {
            if (session.page.isClosed()) return;
            const snapshot = info.outputPath(`${platform}-failure-session-${index + 1}.aria.txt`);
            writeFileSync(snapshot, await session.page.locator("body").ariaSnapshot());
            await info.attach(`${platform}-failure-session-${index + 1}-ui`, { path: snapshot, contentType: "text/plain" });
            const screenshot = info.outputPath(`${platform}-failure-session-${index + 1}.png`);
            if (session.native) await nativeScreenshot(session.native.app, session.page, screenshot);
            else await session.page.screenshot({ path: screenshot, fullPage: false, scale: "css" });
            await info.attach(`${platform}-failure-session-${index + 1}`, { path: screenshot, contentType: "image/png" });
          }));
          evidence.failure_artifact_errors = failures.flatMap((failure, index) => failure.status === "rejected"
            ? [{ session: index + 1, error: failure.reason instanceof Error ? failure.reason.name : "artifact_error" }] : []);
        }
        const record = info.outputPath("detail-journey-evidence.json");
        writeFileSync(record, JSON.stringify(evidence, null, 2));
        await info.attach("detail-journey-evidence", { path: record, contentType: "application/json" });
      }
      finally {
        const closed = await Promise.allSettled(sessions.map((session) => session.close()));
        for (const outcome of closed) if (outcome.status === "rejected") throw outcome.reason;
      }
    }
  });
}
