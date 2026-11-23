import { writeFileSync } from "node:fs";
import { expect, test, type Page } from "@playwright/test";
import type { CollaborationIssue, CollaborationResourceIdentityView, CollaborationRunRecords } from "@multica/core/collaboration";
import { hostAccountPath, hostAuthority, hostControl, hostMode, hostPanel, hostRuntimeConfigured, issueRoot, versioned } from "./fixtures/collaboration-host-runtime";
import { sendSquadComment, type SquadPick } from "./fixtures/collaboration-host-squad";
import { listRuntimeRuns, postCollaboration, seedHostSwitch } from "./fixtures/collaboration-host-switch";

test.use({ actionTimeout: 15_000, screenshot: "only-on-failure", trace: "off" });

// AC39–AC42 after a real run: the Switch is refused while the queue runs, while
// the old runtime has not confirmed termination, and before the new resource's
// owner consents; it then holds; only explicit Resume runs on the new runtime,
// which receives the explicitly delivered file but none of the old local state.
test("switching a squad to another native runtime waits for confirmed Stop and owner consent, then resumes there without moving local files", async ({ page, browser }, testInfo) => {
  test.setTimeout(300_000);
  test.skip(!hostRuntimeConfigured(), "Requires explicit task-owned host daemon/provider/CLI executables; configured runs fail if any environment input is missing");
  const fixture = await seedHostSwitch();
  const handlerContext = await browser.newContext();
  const pageErrors: string[] = [];
  let closed = false;
  const root = issueRoot(fixture.issue.id);
  const read = async () => (await fixture.owner.client.requestJSON<CollaborationIssue>(versioned(root)));
  const target = async () => (await read()).execution!.targets.find((item) => item.id === fixture.issue.targetId)!;
  const evidence: Record<string, unknown> = {
    exercised: ["configuration change holds the live target and stops its work", "Switch refused before the old runtime confirms termination", "consent only by the resource owner",
      "Switch refused before the new owner consents", "switch_hold without execution", "explicit Resume on the replacement runtime",
      "stopped input not replayed", "unread joined input returned to waiting by the stop (M11)", "original typed target kept", "explicit attachment delivered", "old workdir and local file not moved"],
    exclusions: ["commercial Provider accounts", "Git preparation (Go: collaborationrun host_git_test.go)", "Switch refused on an unheld queue (Go: CommitTargetSwitch target_pause_required)", "a second human resource owner", "Electron client"],
    identities: { issue_id: fixture.issue.id, target_id: fixture.issue.targetId, squad_id: fixture.squad.id, old_leader: fixture.oldLeader.id,
      new_leader: fixture.newLeader.id, first_runtime: fixture.runtimes.first.runtimeId, second_runtime: fixture.runtimes.second.runtimeId },
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
    const S: SquadPick = { kind: "squad", resourceId: fixture.squad.id, name: fixture.issue.name };
    await hostControl(page, fixture.issue, "resume", "issue");
    // A real first run on runtime 1. It receives the explicit attachment and
    // leaves an uncommitted local file in its own workdir.
    const first = await sendSquadComment(page, fixture, "Owner asks squad S to start on runtime one", [S], async () => undefined);
    const c1 = first.byTarget(fixture.issue.targetId);
    const v1 = await fixture.daemons.first.waitForVisit(1);
    expect(v1.identity).toMatchObject({ issue_id: fixture.issue.id, target_id: fixture.issue.targetId, channel_id: fixture.issue.channelId });
    expect(v1).toMatchObject({ agent_id: fixture.oldLeader.id, cli_context: { step_kind: "leader", squad_id: fixture.squad.id } });
    expect(v1.inputs.map((input) => input.command_id)).toEqual([c1.command_id]);
    expect(v1.attachment_hashes).toEqual({ [fixture.issue.attachment!.id]: fixture.issue.attachment!.sha256 });
    const localDraft = "notes/uncommitted-draft.md";
    const [wrote] = await fixture.daemons.first.act(v1, [{ kind: "write_file", path: localDraft, content: "Runtime one local work in progress.\n" }]);
    expect(wrote!.sha256).toMatch(/^[0-9a-f]{64}$/);

    // S's group is still running and unsealed, so a second @S joins it as an
    // unread input for the next leader continuation.
    const second = await sendSquadComment(handlerPage, fixture, "Handler continues the work after the first turn", [S], async () => undefined);
    const c2 = second.byTarget(fixture.issue.targetId);
    expect(c2).toMatchObject({ state: "waiting", code: "command_joined_squad", group_id: v1.identity.group_id, channel_id: fixture.issue.channelId });

    // Runtime 1 is suspended, so it cannot report termination. Then the owner
    // moves S's leader to L2: the server holds the live target and stops its
    // work, but the old run's end stays unconfirmed while runtime 1 is silent.
    fixture.daemons.first.freeze();
    await fixture.owner.client.requestJSON(`/api/squads/${fixture.squad.id}`, { method: "PUT", body: { leader_id: fixture.newLeader.id } });
    await expect.poll(async () => (await target()).hold_reason, { timeout: 30_000 }).toBe("config_invalid");
    expect((await read()).execution!.runs.find((run) => run.id === v1.identity.run_id)?.settled_at).toBeNull();
    const managed = hostPanel(page);
    const manage = managed.getByRole("button", { name: "Manage AI resources", exact: true });
    if ((await manage.getAttribute("aria-expanded")) !== "true") await manage.click();
    const toggle = page.getByRole("button", { name: "Switch proposals", exact: true });
    await toggle.click();
    const proposals = page.locator(`[id="${await toggle.getAttribute("aria-controls")}"]`);
    await proposals.getByRole("button", { name: "Propose configuration switch", exact: true }).click();
    await proposals.getByLabel("Resource workspace", { exact: true }).selectOption(fixture.workspace.id);
    await proposals.getByLabel("AI target", { exact: true }).selectOption(`squad:${fixture.squad.id}`);
    const proposed = page.waitForResponse((response) => new URL(response.url()).pathname === `${root}/targets/${fixture.issue.targetId}/switch-proposals`
      && response.request().method() === "POST");
    await proposals.getByRole("button", { name: "Create Switch proposal", exact: true }).click();
    const proposal: { code: string; proposal_id: string } = await (await proposed).json();
    expect(proposal.code).toBe("switch_proposed");
    await expect(page.getByText("Switch proposal created. Collect each owner's consent before applying it.")).toBeVisible();
    const proposalRoot = `${root}/targets/${fixture.issue.targetId}/switch-proposals/${proposal.proposal_id}`;
    const commit = async () => {
      const current = await target();
      return postCollaboration(fixture.apiUrl, fixture.owner, `${proposalRoot}/commit`, { ...hostAuthority(await read()), control: {
        target_id: current.id, expected_target_epoch: current.target_epoch, expected_config_version: current.config_version,
        expected_hold_version: current.hold_version, expected_control_version: current.control.control_version,
        observed_group_id: current.active_group_id, observed_group_generation: current.group_generation,
      } });
    };

    // AC39: the old execution has not confirmed its end, so the Switch cannot complete.
    try {
      expect(await commit()).toMatchObject({ status: 409, body: { code: "stop_unconfirmed" } });
    } finally { fixture.daemons.first.thaw(); }
    // Only the replacement resource's owner can consent for it.
    const identity = await fixture.owner.client.requestJSON<CollaborationResourceIdentityView>(
      versioned(`${root}/resources/identity?target_id=${fixture.issue.targetId}&agent_id=${fixture.newLeader.id}&proposal_id=${proposal.proposal_id}`));
    expect(identity.identity?.installation_id).toBe(fixture.runtimes.second.installationId);
    const handlerIssue = await fixture.handler.client.requestJSON<CollaborationIssue>(versioned(root));
    expect(await postCollaboration(fixture.apiUrl, fixture.handler, `${proposalRoot}/consents`, { ...hostAuthority(handlerIssue),
      agent_id: fixture.newLeader.id, identity: identity.identity })).toMatchObject({ status: 403, body: { code: "forbidden" } });

    // Runtime 1 now observes the stop and reports its end.
    await expect.poll(async () => (await read()).execution!.runs.find((run) => run.id === v1.identity.run_id), { timeout: 30_000 })
      .toMatchObject({ progress_state: "settled", proof_state: "host_exit_observed", forced_disposition: "stopped" });
    expect(v1.released_ms).toBeNull();
    // M11: the stop returns the unread joined input to waiting; it is neither run nor dropped.
    expect((await read()).execution!.commands.find((item) => item.id === c2.command_id)).toMatchObject({ state: "waiting", group_id: null, channel_id: fixture.issue.channelId });
    // Termination is confirmed, but the new owner has not consented yet.
    expect(await commit()).toMatchObject({ status: 409, body: { code: "switch_consent_required" } });
    // The owner consents for the replacement leader; the live target is unchanged.
    const beforeConsent = await target();
    expect(beforeConsent).toMatchObject({ hold_reason: "config_invalid", channel_id: fixture.issue.channelId, active_group_id: null });
    // The leader change keeps L1 in S as a member, so the proposal needs consent
    // for both resources before it can be committed.
    for (const agent of [fixture.newLeader, fixture.oldLeader]) {
      await proposals.getByRole("button", { name: `Authorize ${agent.name}`, exact: true }).click();
      await expect(proposals.getByRole("heading", { name: "Allow this issue to use this agent", exact: true })).toBeVisible();
      const consented = page.waitForResponse((response) => new URL(response.url()).pathname === `${proposalRoot}/consents` && response.request().method() === "POST");
      await proposals.getByRole("button", { name: "Authorize for this issue", exact: true }).click();
      const response = await consented;
      expect(response.status()).toBe(201);
      expect(response.request().postDataJSON()).toMatchObject({ agent_id: agent.id });
    }
    await expect(page.getByText("Proposal consent recorded. The live configuration has not changed.")).toBeVisible();
    expect(await target()).toEqual(beforeConsent);
    expect((await read()).execution!.commands.find((item) => item.id === c2.command_id)).toMatchObject({ state: "waiting", channel_id: fixture.issue.channelId });

    await proposals.getByRole("button", { name: "Commit Switch", exact: true }).click();
    await proposals.getByRole("button", { name: "Apply configuration", exact: true }).click();
    await expect(page.getByText("Configuration switched. Explicit resume is still required.")).toBeVisible();
    const switched = await target();
    expect(switched).toMatchObject({ id: fixture.issue.targetId, kind: "squad", resource_id: fixture.squad.id, hold_reason: "switch_hold" });
    expect(BigInt(switched.config_version)).toBeGreaterThan(BigInt(beforeConsent.config_version));
    expect(switched.channel_id).not.toBe(fixture.issue.channelId);
    let execution = (await read()).execution!;
    // AC40: history and the original typed target stay; the stopped input stays stopped.
    expect(execution.commands.find((item) => item.id === c1.command_id)).toMatchObject({ state: "stopped", target_id: fixture.issue.targetId, channel_id: fixture.issue.channelId });
    expect(execution.commands.find((item) => item.id === c2.command_id)).toMatchObject({ state: "waiting", group_id: null, target_id: fixture.issue.targetId,
      channel_id: switched.channel_id });
    // The join had queued a leader continuation in the old group; the stop settles
    // it as provably never launched instead of leaving it to run after the Switch.
    const oldRuns = execution.runs.filter((run) => run.group_id === v1.identity.group_id);
    expect(execution.runs).toHaveLength(2);
    expect(oldRuns.map(({ id, step_kind, proof_state, forced_disposition, launch_count }) => ({ id, step_kind, proof_state, forced_disposition, launch_count }))
      .sort((a, b) => a.step_kind.localeCompare(b.step_kind))).toEqual([
      { id: expect.any(String), step_kind: "continuation", proof_state: "never_launched_proven", forced_disposition: "stopped", launch_count: "0" },
      { id: v1.identity.run_id, step_kind: "leader", proof_state: "host_exit_observed", forced_disposition: "stopped", launch_count: "1" },
    ]);
    // AC41: the Switch itself starts nothing, even when the new runtime polls.
    expect(await listRuntimeRuns(fixture, "second")).toEqual([]);
    expect(fixture.daemons.second.visits).toHaveLength(0);
    // Only explicit Resume continues, on runtime 2, with a fresh workdir and session.
    const hostIssue = { ...fixture.issue, channelId: switched.channel_id!, agentId: fixture.newLeader.id };
    await hostControl(page, hostIssue, "resume", "target");
    const v2 = await fixture.daemons.second.waitForVisit(1);
    expect(v2.identity).toMatchObject({ issue_id: fixture.issue.id, target_id: fixture.issue.targetId, channel_id: switched.channel_id,
      installation_id: fixture.runtimes.second.installationId });
    expect(v2).toMatchObject({ agent_id: fixture.newLeader.id, resource_owner_id: fixture.owner.id, resumed: false,
      cli_context: { step_kind: "leader", squad_id: fixture.squad.id } });
    expect(v2.inputs).toEqual([{ command_id: c2.command_id, author_user_id: fixture.handler.id, body: second.content, attachment_ids: [] }]);
    // AC41/AC42: the explicitly delivered file is prepared; runtime 1's local file is not.
    expect(v2.attachment_hashes).toEqual({ [fixture.issue.attachment!.id]: fixture.issue.attachment!.sha256 });
    expect(Object.values(v2.workdir_files)).toContain(fixture.issue.attachment!.sha256);
    expect(Object.keys(v2.workdir_files)).not.toContain(localDraft);
    expect(Object.values(v2.workdir_files)).not.toContain(wrote!.sha256);
    expect(v2.workdir_hash).not.toBe(v1.workdir_hash);
    expect(v2.session_hash).not.toBe(v1.session_hash);
    expect(fixture.daemons.first.visits).toHaveLength(1);
    fixture.daemons.second.release(v2);
    await expect.poll(async () => (await read()).execution!.runs.find((run) => run.id === v2.identity.run_id), { timeout: 30_000 })
      .toMatchObject({ progress_state: "settled", proof_state: "host_exit_observed", execution_mode: hostMode, business_outcome: "success" });

    execution = (await read()).execution!;
    expect(execution.runs).toHaveLength(3);
    expect(execution.runs.find((run) => run.id === v1.identity.run_id)).toMatchObject({ agent_id: fixture.oldLeader.id, forced_disposition: "stopped" });
    expect(execution.runs.find((run) => run.id === v2.identity.run_id)).toMatchObject({ agent_id: fixture.newLeader.id, forced_disposition: null,
      step_kind: "leader", group_id: v2.identity.group_id });
    expect(v2.identity.group_id).not.toBe(v1.identity.group_id);
    expect(execution.commands.find((item) => item.id === c1.command_id)?.state).toBe("stopped");
    expect(execution.commands.find((item) => item.id === c2.command_id)?.state).toBe("succeeded");
    for (const [run, agentId, runtimeId] of [[v1.identity.run_id, fixture.oldLeader.id, fixture.runtimes.first.runtimeId],
      [v2.identity.run_id, fixture.newLeader.id, fixture.runtimes.second.runtimeId]] as const) {
      const record = await fixture.owner.client.requestJSON<CollaborationRunRecords>(versioned(`${root}/runs/${run}/records?limit=100`));
      expect(record.resource).toMatchObject({ agent_id: agentId, runtime_id: runtimeId, resource_owner_user_id: fixture.owner.id });
    }
    for (const current of [page, handlerPage] as Page[]) {
      await current.reload();
      await expect(current.getByRole("heading", { name: fixture.issue.title, exact: true })).toBeVisible();
      await expect(hostPanel(current).locator(`[id="run-${v1.identity.run_id}"]`)).toContainText("Managed execution stopped");
      await expect(hostPanel(current).locator(`[id="run-${v2.identity.run_id}"]`)).toContainText("Managed execution ended");
    }
    await page.screenshot({ path: testInfo.outputPath("host-switch-owner.png"), fullPage: true });
    expect(fixture.daemons.first.errors).toEqual([]);
    expect(fixture.daemons.second.errors).toEqual([]);
    const [firstSummary, secondSummary] = await fixture.close();
    closed = true;
    evidence.daemons = [firstSummary, secondSummary].map((summary) => ({ exit_code: summary!.exit_code, ledger: summary!.ledger }));
    for (const [summary, visit] of [[firstSummary!, v1], [secondSummary!, v2]] as const) {
      expect(summary.exit_code).toBe(0);
      expect(summary.stderr).toBe("");
      expect(summary.messages.find((item) => "claims" in item)).toMatchObject({ claims: 1, invalid_capacity: false, active_tasks: 0 });
      expect(summary.ledger).toEqual([expect.objectContaining({ run_id: visit.identity.run_id, sealed: true, proof_state: "host_exit_observed" })]);
    }
    expect(pageErrors).toEqual([]);
  } finally {
    evidence.provider_observations = { first: fixture.daemons.first.evidence(), second: fixture.daemons.second.evidence() };
    if (!closed) await fixture.close();
    evidence.page_errors = pageErrors;
    const encoded = JSON.stringify(evidence, null, 2);
    expect(encoded).not.toMatch(/\b(?:mcb_|mct_|mdt_|mat_)[A-Za-z0-9_.-]+/);
    const evidencePath = testInfo.outputPath("host-switch-evidence.json");
    writeFileSync(evidencePath, encoded, { mode: 0o600 });
    await testInfo.attach("host-switch-evidence", { contentType: "application/json", path: evidencePath });
    await handlerContext.close();
  }
});
