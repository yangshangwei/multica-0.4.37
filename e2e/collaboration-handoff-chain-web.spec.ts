import { test, expect, type BrowserContext } from "@playwright/test";
import type { CollaborationIssue } from "@multica/core/collaboration";
import { assertLists, checkVisibleLists, handoffNotifications, seedOfflineHandoff, uiWorkflow, type HandoffActor, type HandoffSession } from "./fixtures/collaboration-offline-handoff";
import { directedFormThroughUI } from "./fixtures/collaboration-handoff-chain";
import { commentThroughUI, gapWorkflowObservations, lifecycleThroughUI } from "./fixtures/collaboration-control-gaps";

test.use({ actionTimeout: 15_000, screenshot: "only-on-failure", trace: "retain-on-failure" });

/** Gap 4 (C02): the directed chain moves the account workbench's current todo
 * with responsibility, and only the creator's explicit action completes. */
test("Web directed handoff chain moves current todos with responsibility and only the creator completes", async ({ browser }, info) => {
  test.setTimeout(240_000);
  test.skip(process.env.E2E_PASSWORD_AUTH !== "1", "Requires task-owned password production services");
  const fixture = await seedOfflineHandoff();
  const origin = info.project.use.baseURL!;
  expect(["localhost", "127.0.0.1"]).toContain(new URL(origin).hostname);
  const P = fixture.creator;
  const H = fixture.recipient;
  const root = `/api/collaboration/issues/${fixture.issueId}`;
  const read = (actor: HandoffActor = P) => actor.client.requestJSON<CollaborationIssue>(root);
  const contexts: BrowserContext[] = [];
  const errors: string[] = [];
  const open = async (actor: HandoffActor): Promise<HandoffSession> => {
    const context = await browser.newContext();
    contexts.push(context);
    const page = await context.newPage();
    page.on("pageerror", (error) => errors.push(error.message));
    await actor.authenticateWeb(page);
    await page.goto(new URL(fixture.accountPath, origin).href);
    await expect(page.getByRole("heading", { name: fixture.title, exact: true })).toBeVisible();
    return { page, navigate: async (next) => { await page.goto(new URL(next, origin).href); }, close: () => context.close() };
  };
  try {
    const original = await read();
    const sameBusiness = (after: CollaborationIssue) => {
      expect({ id: after.id, title: after.title, description: after.description, creator: after.creator, source_workspace: after.source_workspace })
        .toEqual({ id: original.id, title: original.title, description: original.description, creator: original.creator, source_workspace: original.source_workspace });
      // No directed step stops, resumes or starts AI work (AC26/AC30).
      expect(after.session).toEqual(original.session);
      expect(after.execution).toEqual(original.execution);
    };
    const pSession = await open(P);

    // 1. P assigns H: H's current todo appears, P's leaves (AC03, AC77).
    const transfer = await uiWorkflow(pSession, fixture, P, "transfer", H, "Please implement and ask me if anything is unclear.");
    const transferred = await read();
    expect(transferred.handler.user_id).toBe(H.id);
    sameBusiness(transferred);
    await assertLists(P, fixture, false, true);
    await assertLists(H, fixture, true, true);

    // 2. H asks P for information through the Web form (AC25).
    const hSession = await open(H);
    const requested = await directedFormThroughUI(hSession.page, fixture.issueId, "request_information", "Which payment provider should the retry use?", { id: P.id, name: P.name });
    expect(requested.receipt.code).toBe("information_requested");
    const afterRequest = await read();
    expect(afterRequest.handler.user_id).toBe(P.id);
    expect(afterRequest.current_information_request).toMatchObject({ requester: { user_id: H.id }, recipient: { user_id: P.id },
      note: "Which payment provider should the retry use?" });
    expect(afterRequest.status).toBe(requested.selectedStatus);
    expect(afterRequest.status_category).toBe("blocked");
    sameBusiness(afterRequest);
    await assertLists(P, fixture, true, true);
    await assertLists(H, fixture, false, true);
    await checkVisibleLists(pSession, fixture, P, true);
    await checkVisibleLists(hSession, fixture, H, false);

    // 3. Plain comments from the recipient and the requester do not return it (AC26).
    const recipientComment = await commentThroughUI(pSession.page, fixture.issueId, "Use the provider already configured for the source workspace.");
    expect(recipientComment.receipt.targets).toEqual([]);
    expect(recipientComment.preview.recipients.map((item) => item.recipient.user_id)).toEqual([H.id]);
    const requesterComment = await commentThroughUI(hSession.page, fixture.issueId, "Thanks, waiting for your explicit return.");
    expect(requesterComment.receipt.targets).toEqual([]);
    const afterComments = await read();
    expect(afterComments.handler.user_id).toBe(P.id);
    expect(afterComments.current_information_request?.id).toBe(afterRequest.current_information_request!.id);
    expect(afterComments.status).toBe(afterRequest.status);
    sameBusiness(afterComments);
    await assertLists(P, fixture, true, true);
    await assertLists(H, fixture, false, true);

    // 4. P explicitly returns: responsibility goes back to the requester H,
    // the saved status is restored and nothing completes (AC25, AC26, AC28).
    const returned = await directedFormThroughUI(pSession.page, fixture.issueId, "return_information", "Answered: use the configured provider.");
    expect(returned.receipt.code).toBe("information_returned");
    const afterReturn = await read();
    expect(afterReturn.handler.user_id).toBe(H.id);
    expect(afterReturn.current_information_request).toBeNull();
    expect(afterReturn.status).toBe(transferred.status);
    expect(afterReturn.status).toBe(returned.selectedStatus);
    expect(afterReturn.status_category).not.toBe("done");
    sameBusiness(afterReturn);
    await assertLists(P, fixture, false, true);
    await assertLists(H, fixture, true, true);

    // 5. H submits for acceptance: responsibility and the todo go to the
    // creator; the issue is in review, not complete (AC29, AC30, AC28).
    const accepted = await directedFormThroughUI(hSession.page, fixture.issueId, "submit_acceptance", "Implemented; please accept.");
    expect(accepted.receipt.code).toBe("acceptance_submitted");
    const afterAcceptance = await read();
    expect(afterAcceptance.handler.user_id).toBe(P.id);
    expect(afterAcceptance.status).toBe(accepted.selectedStatus);
    expect(afterAcceptance.status_category).toBe("in_review");
    expect(afterAcceptance.current_information_request).toBeNull();
    sameBusiness(afterAcceptance);
    await assertLists(P, fixture, true, true);
    await assertLists(H, fixture, false, true);
    await checkVisibleLists(pSession, fixture, P, true);
    await checkVisibleLists(hSession, fixture, H, false);

    // H, now neither creator nor handler, keeps discussion/AI control but cannot complete (AC27, AC54).
    const hView = await read(H);
    expect(hView.capabilities).toMatchObject({ complete: false, cancel: false, transfer: false, submit_acceptance: false, comment: true, control_ai: true, leave: true });
    await expect(hSession.page.getByRole("button", { name: "Leave issue", exact: true })).toBeVisible();
    for (const label of ["Complete issue", "Cancel issue", "Submit for acceptance", "Transfer"]) {
      await expect(hSession.page.getByRole("button", { name: label, exact: true })).toHaveCount(0);
    }
    await expect(pSession.page.getByRole("button", { name: "Complete issue", exact: true })).toBeVisible();
    const hOptions = await fetch(`${fixture.apiUrl}${root}/status-options?action=complete`, { headers: { Authorization: `Bearer ${H.client.getToken()!}` } });
    expect(hOptions.status).toBe(403);
    const pOptions = await P.client.requestJSON<{ status_options_version: string }>(`${root}/status-options?action=complete`);
    const forgedKey = crypto.randomUUID();
    const forged = await fetch(`${fixture.apiUrl}${root}/lifecycle`, { method: "POST", headers: { Authorization: `Bearer ${H.client.getToken()!}`,
      "Content-Type": "application/json", "Idempotency-Key": forgedKey },
      body: JSON.stringify({ ...gapWorkflowObservations(hView), action: "complete", status: "done", status_options_version: pOptions.status_options_version }) });
    expect(forged.status).toBe(403);
    expect(await forged.json()).toMatchObject({ code: "forbidden", status: 403 });
    // The refusal was recorded for H's real session, so it is a role decision, not an authentication failure.
    expect(await H.client.requestJSON(`${root}/operations/${forgedKey}`)).toMatchObject({ code: "forbidden" });
    expect(await read()).toEqual(afterAcceptance);

    // 6. The creator completes: no current todo remains, both keep the history (AC27, AC28, AC02).
    await pSession.page.reload();
    await expect(pSession.page.getByRole("heading", { name: fixture.title, exact: true })).toBeVisible();
    const completed = await lifecycleThroughUI(pSession.page, fixture.issueId, "complete");
    const done = await read();
    expect(done.status_category).toBe("done");
    expect(done.session.queue_state).toBe("closed");
    expect(done.handler.user_id).toBe(P.id);
    await assertLists(P, fixture, false, true);
    await assertLists(H, fixture, false, true);
    await checkVisibleLists(pSession, fixture, P, false);
    expect(await P.client.countIssueDispatches(fixture.issueId)).toBe(0);

    // 7. Each directed step notified only the other party once; completion adds no notice (AC83).
    const pNotices = await handoffNotifications(P, fixture.issueId);
    const hNotices = await handoffNotifications(H, fixture.issueId);
    expect(pNotices.map((item) => item.kind).sort()).toEqual(["acceptance_submitted", "comment_created", "information_requested"]);
    expect(hNotices.map((item) => item.kind).sort()).toEqual(["comment_created", "information_returned", "transferred"]);
    expect([...pNotices, ...hNotices].every((item) => item.can_open === true)).toBe(true);
    const activity = await P.client.requestJSON<{ items: { kind: string; actor: { user_id: string } | null; handoff: { purpose: string; from: { user_id: string } | null; to: { user_id: string } | null; note: string } | null }[] }>(`${root}/activity?limit=100`);
    const count = (kind: string) => activity.items.filter((item) => item.kind === kind).length;
    for (const kind of ["transferred", "information_requested", "information_returned", "acceptance_submitted", "completed"]) expect(count(kind), kind).toBe(1);
    expect(count("comment_created")).toBe(2);
    const handoff = (kind: string) => activity.items.find((item) => item.kind === kind)!.handoff;
    expect(handoff("information_requested")).toMatchObject({ purpose: "information", from: { user_id: H.id }, to: { user_id: P.id }, note: "Which payment provider should the retry use?" });
    expect(handoff("information_returned")).toMatchObject({ purpose: "information_return", from: { user_id: P.id }, to: { user_id: H.id } });
    expect(handoff("acceptance_submitted")).toMatchObject({ purpose: "acceptance", from: { user_id: H.id }, to: { user_id: P.id } });
    expect(errors).toEqual([]);
    await info.attach("handoff-chain-evidence", { contentType: "application/json", body: JSON.stringify({ issue_id: fixture.issueId, creator: P.id, handler: H.id,
      transfer: transfer.receipt, requested: requested.receipt, returned: returned.receipt, accepted: accepted.receipt, completed: completed.receipt,
      forged_complete_by_former_handler: { status: 403, request_id: forgedKey }, notifications: { creator: pNotices, handler: hNotices },
      activity_kinds: activity.items.map((item) => item.kind), final_issue: done }, null, 2) });
  } finally {
    for (const context of contexts) await context.close().catch(() => undefined);
  }
});
