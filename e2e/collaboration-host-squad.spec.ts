import { randomUUID } from "node:crypto";
import { writeFileSync } from "node:fs";
import { expect, test, type Page } from "@playwright/test";
import type { CollaborationComments, CollaborationRunRecords } from "@multica/core/collaboration";
import { hostAccountPath, hostRuntimeConfigured, issueRoot, versioned } from "./fixtures/collaboration-host-runtime";
import { readSquadIssue, seedHostSquad, sendSquadComment, squadControl, waitSquadRun, type SquadPick } from "./fixtures/collaboration-host-squad";

test.use({ actionTimeout: 15_000, screenshot: "only-on-failure", trace: "off" });

// AC92/AC94 through the real native poller: one capacity-one runtime runs the
// squad leader, its member and the leader continuation in turn, while a later
// instruction joins the running squad and another squad led by A waits behind it.
test("a capacity-one host squad merges @A with @S, delegates to its member, joins a later instruction and queues another squad", async ({ page, browser }, testInfo) => {
  test.setTimeout(300_000);
  test.skip(!hostRuntimeConfigured(), "Requires explicit task-owned host daemon/provider/CLI executables; configured runs fail if any environment input is missing");
  const fixture = await seedHostSquad();
  const handlerContext = await browser.newContext();
  const pageErrors: string[] = [];
  let daemonClosed = false;
  const evidence: Record<string, unknown> = {
    exercised: ["@A and @S merged into one squad leader run", "leader delegation through the run-local CLI", "member run derived from the leader",
      "capacity one across leader, member and continuation", "human instruction joining the running squad", "leader continuation consuming the joined input",
      "squad activity", "another squad led by A queued in lane A's next batch"],
    exclusions: ["commercial Provider accounts", "Electron client", "Git preparation", "Stop while a joined input is unread (collaboration-host-switch.spec.ts; Go: TestCollaborationSquadStopReturnsUnreadJoinWithoutRestartingIt)"],
    identities: { issue_id: fixture.issue.id, runtime_id: fixture.runtime.runtimeId, leader_id: fixture.leader.id, member_id: fixture.member.id,
      owner_id: fixture.owner.id, handler_id: fixture.handler.id, targets: fixture.targets },
  };
  try {
    const handlerPage = await handlerContext.newPage();
    for (const [actor, current] of [[fixture.owner, page], [fixture.handler, handlerPage]] as const) {
      current.on("pageerror", (error) => pageErrors.push(error.message));
      await actor.authenticateWeb(current);
      await current.addInitScript(() => { localStorage.setItem("multica-locale", "en"); localStorage.setItem("multica:chat:isOpen", "false"); });
      await current.goto(new URL(hostAccountPath(fixture.issue.id), testInfo.project.use.baseURL!).href);
      await expect(current.getByRole("heading", { name: fixture.issue.title, exact: true })).toBeVisible();
    }
    const { agent: A, build: S, review: R } = fixture.targets;
    const pick = {
      A: { kind: "agent", resourceId: A.resourceId, name: A.name },
      S: { kind: "squad", resourceId: S.resourceId, name: S.name },
      R: { kind: "squad", resourceId: R.resourceId, name: R.name },
    } satisfies Record<string, SquadPick>;
    expect(await squadControl(page, fixture, "resume")).toMatchObject({ code: "resumed", queue_state: "active" });
    for (const target of (await readSquadIssue(fixture)).targets) expect(target.hold_reason).toBeNull();
    // AC92: one comment @A + @S (A leads S) previews as one squad execution and
    // runs once, as the squad leader, with both commands as its input.
    const h1 = await sendSquadComment(page, fixture, "Owner asks A and squad S to ship the change together", [pick.A, pick.S], async (grouping, preview) => {
      await expect(grouping.getByText("Expected to merge into one squad execution.", { exact: true })).toBeVisible();
      const merged = preview.targets.filter((item) => item.target_id === A.targetId || item.target_id === S.targetId);
      expect(merged).toHaveLength(2);
      for (const item of merged) expect(item.grouping?.target_ids.slice().sort()).toEqual([A.targetId, S.targetId].sort());
    });
    const h1A = h1.byTarget(A.targetId), h1S = h1.byTarget(S.targetId);
    for (const item of [h1A, h1S]) expect(item).toMatchObject({ state: "waiting", code: "command_queued", group_id: null });
    expect(h1S.channel_id).toBe(h1A.channel_id);
    const lane = h1A.channel_id!;
    const leaderVisit = await fixture.daemon.waitForVisit(1);
    expect(leaderVisit.identity).toMatchObject({ issue_id: fixture.issue.id, target_id: S.targetId, channel_id: lane });
    expect(leaderVisit).toMatchObject({ agent_id: fixture.leader.id, resource_owner_id: fixture.owner.id,
      cli_context: { issue_id: fixture.issue.id, agent_id: fixture.leader.id, step_kind: "leader", squad_id: S.resourceId, dispatch_comment: "" } });
    expect(leaderVisit.cli_context.roster.map(({ agent_id, role }) => [role, agent_id]).sort())
      .toEqual([["leader", fixture.leader.id], ["member", fixture.member.id]].sort());
    expect(leaderVisit.inputs.map(({ command_id, author_user_id }) => [command_id, author_user_id]).sort())
      .toEqual([[h1A.command_id, fixture.owner.id], [h1S.command_id, fixture.owner.id]].sort());
    let execution = await readSquadIssue(fixture);
    expect(execution.runs).toHaveLength(1);
    expect(execution.runs[0]).toMatchObject({ id: leaderVisit.identity.run_id, step_kind: "leader", target_id: S.targetId, agent_id: fixture.leader.id, parent_run_id: null });
    const groupId = execution.runs[0]!.group_id;
    expect(execution.groups.find((group) => group.id === groupId)?.inputs.map((input) => input.command_id).sort())
      .toEqual([h1A.command_id, h1S.command_id].sort());

    // AC94: the leader delegates through its run-local CLI. The member run is
    // derived from the leader and waits, because the only slot is occupied.
    const dispatchText = `[@${fixture.member.name}](mention://agent/${fixture.member.id}) implement the change and report back`;
    const [dispatch] = await fixture.daemon.act(leaderVisit, [{ kind: "comment", request_id: randomUUID(), content: dispatchText }]);
    const dispatched = dispatch!.result as { comment_id: string; run_ids: string[]; continuation_run_id: string | null };
    expect(dispatched.comment_id).toBe(dispatch!.request_id);
    expect(dispatched.run_ids).toHaveLength(1);
    expect(dispatched.continuation_run_id).toBeNull();
    const memberRunId = dispatched.run_ids[0]!;
    execution = await readSquadIssue(fixture);
    expect(execution.runs.find((run) => run.id === memberRunId)).toMatchObject({ step_kind: "member", parent_run_id: leaderVisit.identity.run_id,
      agent_id: fixture.member.id, group_id: groupId, progress_state: "prepared" });
    expect(fixture.daemon.visits).toHaveLength(1);
    fixture.daemon.release(leaderVisit);
    await waitSquadRun(fixture, leaderVisit.identity.run_id);
    execution = await readSquadIssue(fixture);
    expect(execution.groups.find((group) => group.id === groupId)?.settled_at).toBeNull();
    const memberVisit = await fixture.daemon.waitForVisit(2);
    expect(memberVisit.identity).toMatchObject({ run_id: memberRunId, group_id: groupId, target_id: S.targetId, channel_id: lane });
    expect(memberVisit).toMatchObject({ agent_id: fixture.member.id,
      cli_context: { agent_id: fixture.member.id, step_kind: "member", squad_id: S.resourceId, dispatch_comment: dispatchText } });
    expect(memberVisit.inputs.every((input) => input.author_user_id === fixture.owner.id)).toBe(true);

    // While the member runs, the handler's @A joins S's next leader continuation.
    const h2 = await sendSquadComment(handlerPage, fixture, "Handler adds a requirement while the member works", [pick.A], async (grouping, preview) => {
      await expect(grouping.getByText("Expected to join the current squad's next leader continuation.", { exact: true })).toBeVisible();
      expect(preview.targets[0]?.grouping).toMatchObject({ kind: "squad_continuation" });
    });
    const h2A = h2.byTarget(A.targetId);
    expect(h2A).toMatchObject({ state: "waiting", code: "command_joined_squad", waiting_reason: "joined_squad", group_id: groupId, channel_id: lane });
    execution = await readSquadIssue(fixture);
    const joinedGroup = execution.groups.find((group) => group.id === groupId)!;
    expect(joinedGroup.unread_inputs.map((input) => input.command_id)).toEqual([h2A.command_id]);
    const continuationRunId = joinedGroup.queued_continuation_run_id;
    expect(continuationRunId).not.toBeNull();
    expect(execution.runs.find((run) => run.id === continuationRunId)).toMatchObject({ step_kind: "continuation", agent_id: fixture.leader.id,
      parent_run_id: memberRunId, group_id: groupId, progress_state: "prepared" });

    // Another squad led by A does not join S; it waits for lane A's next batch.
    const h3 = await sendSquadComment(handlerPage, fixture, "Handler asks review squad R to check the result afterwards", [pick.R], async (_grouping, preview) => {
      expect(preview.targets[0]?.grouping?.kind).not.toBe("squad_continuation");
    });
    const h3R = h3.byTarget(R.targetId);
    expect(h3R).toMatchObject({ state: "waiting", code: "command_queued", group_id: null, channel_id: lane });
    expect(h3R.batch_id).not.toBe(h1A.batch_id);

    const replyText = "Member B finished the change.";
    const [reply] = await fixture.daemon.act(memberVisit, [{ kind: "comment", request_id: randomUUID(), content: replyText }]);
    expect(reply!.result).toMatchObject({ comment_id: reply!.request_id, run_ids: [], continuation_run_id: continuationRunId });
    fixture.daemon.release(memberVisit);
    await waitSquadRun(fixture, memberRunId);
    // The leader continuation runs after the member and consumes the joined input.
    const continuationVisit = await fixture.daemon.waitForVisit(3);
    expect(continuationVisit.identity).toMatchObject({ run_id: continuationRunId, group_id: groupId, target_id: S.targetId });
    expect(continuationVisit).toMatchObject({ agent_id: fixture.leader.id, cli_context: { step_kind: "continuation", squad_id: S.resourceId } });
    expect(continuationVisit.inputs.map(({ command_id, author_user_id }) => [command_id, author_user_id]).sort())
      .toEqual([[h1A.command_id, fixture.owner.id], [h1S.command_id, fixture.owner.id], [h2A.command_id, fixture.handler.id]].sort());
    expect(continuationVisit.inputs.map((input) => input.command_id)).not.toContain(h3R.command_id);
    expect(continuationVisit.cli_context.comments.filter((comment) => comment.author_type === "agent").map(({ id, author_id }) => [id, author_id]))
      .toEqual([[dispatch!.request_id, fixture.leader.id], [reply!.request_id, fixture.member.id]]);
    const finalText = "Squad S result: change shipped with the handler's added requirement.";
    const [finalComment, finalActivity] = await fixture.daemon.act(continuationVisit, [
      { kind: "comment", request_id: randomUUID(), content: finalText },
      { kind: "squad_activity", request_id: randomUUID(), outcome: "action", reason: "Integrated the member result and the joined requirement" },
    ]);
    expect(finalComment!.result).toMatchObject({ comment_id: finalComment!.request_id, run_ids: [], continuation_run_id: null });
    expect(finalActivity!.result).toMatchObject({ code: "squad_activity_recorded" });
    fixture.daemon.release(continuationVisit);
    await waitSquadRun(fixture, continuationRunId!);
    await expect.poll(async () => (await readSquadIssue(fixture)).groups.find((group) => group.id === groupId), { timeout: 30_000 })
      .toMatchObject({ state: "succeeded", unread_inputs: [], queued_continuation_run_id: null });

    // Only then does review squad R get lane A, in its own group.
    const reviewVisit = await fixture.daemon.waitForVisit(4);
    expect(reviewVisit.identity).toMatchObject({ target_id: R.targetId, channel_id: lane });
    expect(reviewVisit.identity.group_id).not.toBe(groupId);
    expect(reviewVisit).toMatchObject({ agent_id: fixture.leader.id, cli_context: { step_kind: "leader", squad_id: R.resourceId } });
    expect(reviewVisit.cli_context.roster).toEqual([expect.objectContaining({ agent_id: fixture.leader.id, role: "leader" })]);
    expect(reviewVisit.inputs).toEqual([{ command_id: h3R.command_id, author_user_id: fixture.handler.id, body: h3.content, attachment_ids: [] }]);
    await fixture.daemon.act(reviewVisit, [{ kind: "squad_activity", request_id: randomUUID(), outcome: "no_action", reason: "Nothing further to review" }]);
    fixture.daemon.release(reviewVisit);
    await waitSquadRun(fixture, reviewVisit.identity.run_id);
    const visits = [leaderVisit, memberVisit, continuationVisit, reviewVisit];
    execution = await readSquadIssue(fixture);
    expect(execution.runs.map(({ id, step_kind, parent_run_id }) => ({ id, step_kind, parent_run_id }))).toEqual(expect.arrayContaining([
      { id: leaderVisit.identity.run_id, step_kind: "leader", parent_run_id: null },
      { id: memberRunId, step_kind: "member", parent_run_id: leaderVisit.identity.run_id },
      { id: continuationRunId, step_kind: "continuation", parent_run_id: memberRunId },
      { id: reviewVisit.identity.run_id, step_kind: "leader", parent_run_id: null },
    ]));
    expect(execution.runs).toHaveLength(4);
    expect(execution.groups.map(({ id, state }) => ({ id, state })).sort((a, b) => a.id.localeCompare(b.id)))
      .toEqual([{ id: groupId, state: "succeeded" }, { id: reviewVisit.identity.group_id!, state: "succeeded" }].sort((a, b) => a.id.localeCompare(b.id)));
    for (const command of [h1A, h1S, h2A, h3R]) expect(execution.commands.find((item) => item.id === command.command_id)?.state).toBe("succeeded");
    expect(execution.channels.find((channel) => channel.id === lane)).toMatchObject({ queue_state: "active", active_group_id: null });

    // Records keep each run's own resource and business writes.
    const records = async (runId: string) => fixture.owner.client.requestJSON<CollaborationRunRecords>(versioned(`${issueRoot(fixture.issue.id)}/runs/${runId}/records?limit=100`));
    const written = (record: CollaborationRunRecords) => record.items.filter((item) => item.kind === "comment" || item.kind === "squad_activity")
      .map((item) => item.kind === "comment" ? ["comment", item.comment_id] : ["squad_activity", item.outcome]);
    const expectations: [string, string, (string | undefined)[][]][] = [
      [leaderVisit.identity.run_id, fixture.leader.id, [["comment", dispatch!.request_id]]],
      [memberRunId, fixture.member.id, [["comment", reply!.request_id]]],
      [continuationRunId!, fixture.leader.id, [["comment", finalComment!.request_id], ["squad_activity", "action"]]],
      [reviewVisit.identity.run_id, fixture.leader.id, [["squad_activity", "no_action"]]],
    ];
    for (const [runId, agentId, writes] of expectations) {
      const record = await records(runId);
      expect(record.run).toMatchObject({ id: runId, proof_state: "host_exit_observed", launch_count: "1" });
      expect(record.resource).toMatchObject({ agent_id: agentId, runtime_id: fixture.runtime.runtimeId, resource_owner_user_id: fixture.owner.id });
      expect(written(record)).toEqual(writes);
    }
    const comments = await fixture.owner.client.requestJSON<CollaborationComments>(`${issueRoot(fixture.issue.id)}/comments`);
    expect(comments.items.filter((item) => item.author_type === "agent").map((item) => item.id))
      .toEqual([dispatch!.request_id, reply!.request_id, finalComment!.request_id]);
    evidence.records = expectations.map(([runId]) => runId);

    // Both human clients show the agents' comments and every managed run.
    for (const current of [page, handlerPage] as Page[]) {
      await current.reload();
      await expect(current.getByRole("heading", { name: fixture.issue.title, exact: true })).toBeVisible();
      for (const text of [replyText, finalText]) await expect(current.getByText(text, { exact: false }).first()).toBeVisible();
      const panel = current.getByRole("region", { name: "AI execution", exact: true });
      for (const visit of visits) await expect(panel.locator(`[id="run-${visit.identity.run_id}"]`)).toContainText("Managed execution ended");
    }
    await page.screenshot({ path: testInfo.outputPath("host-squad-owner.png"), fullPage: true });

    // One slot: each launch begins only after the previous provider exited.
    for (let index = 1; index < visits.length; index++) {
      expect(visits[index - 1]!.closed_ms).not.toBeNull();
      expect(visits[index]!.started_ms).toBeGreaterThanOrEqual(visits[index - 1]!.closed_ms!);
    }
    expect(fixture.daemon.errors).toEqual([]);
    const summary = await fixture.daemon.close();
    daemonClosed = true;
    evidence.daemon = { exit_code: summary.exit_code, ledger: summary.ledger };
    expect(summary.exit_code).toBe(0);
    expect(summary.stderr).toBe("");
    expect(summary.messages.find((item) => "claims" in item)).toMatchObject({ claims: 4, invalid_capacity: false, available_slots: 1, active_tasks: 0 });
    expect(summary.ledger).toHaveLength(4);
    for (const visit of visits) expect(summary.ledger.find((item) => item.run_id === visit.identity.run_id))
      .toMatchObject({ phase: "proof_acknowledged", sealed: true, proof_state: "host_exit_observed", namespace_present: false });
    expect(pageErrors).toEqual([]);
  } finally {
    evidence.provider_observations = fixture.daemon.evidence();
    if (!daemonClosed) evidence.daemon = await fixture.daemon.close();
    evidence.page_errors = pageErrors;
    const encoded = JSON.stringify(evidence, null, 2);
    expect(encoded).not.toMatch(/\b(?:mcb_|mct_|mdt_|mat_)[A-Za-z0-9_.-]+/);
    const evidencePath = testInfo.outputPath("host-squad-evidence.json");
    writeFileSync(evidencePath, encoded, { mode: 0o600 });
    await testInfo.attach("host-squad-evidence", { contentType: "application/json", path: evidencePath });
    await handlerContext.close();
  }
});
