import { randomUUID } from "node:crypto";
import { expect, test } from "@playwright/test";
import type { CollaborationIssue, CollaborationMemory, CollaborationResourceIdentityView, CollaborationResourceOptions, CollaborationResources } from "@multica/core/collaboration";
import { authorizeHostIssue, hostControl, hostRuntimeConfigured, openHostConsent, seedHostRuntime, sendHostComment, waitHostSettlement,
  type HostIssue, type HostVisit } from "./fixtures/collaboration-host-runtime";
import { memoryAuthority, saveMemoryInUI } from "./fixtures/collaboration-memory";

const versioned = (path: string) => `${path}${path.includes("?") ? "&" : "?"}execution_view_version=6`;
function resourceAuthority(issue: CollaborationIssue) {
  const { expected_collaboration_revision: _revision, ...authority } = memoryAuthority(issue);
  return { ...authority, execution_view_version: 6 };
}
type MemoryVisit = HostVisit & {
  memory_input: { version: string; content: string };
  memory_shown: { issue_id: string; run_id: string; version: string; content: string };
  memory_written: { code: string; issue_id: string; run_id: string; version: string };
  instruction_files_clean: boolean;
};

test.use({ actionTimeout: 15_000, screenshot: "only-on-failure", trace: "off" });
test("two native runtime profiles hand off shared context through the actual run-local CLI", async ({ page, browser }, info) => {
  test.setTimeout(300_000);
  test.skip(!hostRuntimeConfigured(), "Requires explicit task-owned host daemon, memory-provider and Multica CLI executables");
  const first = await seedHostRuntime();
  let second: Awaited<ReturnType<typeof seedHostRuntime>> | undefined;
  const successorContext = await browser.newContext();
  const evidence: Record<string, unknown> = {
    scope: "Two separately enrolled native runtime processes, user accounts, private workdirs and empty provider-account profiles on this host; test-owned ordinary Claude protocol only",
    exclusions: ["physical second machine", "commercial provider credentials or quota", "other operating systems"],
  };
  try {
    second = await seedHostRuntime();
    const successor = second;
    const root = `/api/collaboration/issues/${first.main.id}`;
    const read = () => first.webActor.client.requestJSON<CollaborationIssue>(versioned(root));
    const memory = () => first.webActor.client.requestJSON<CollaborationMemory>(`${root}/memory`);
    await first.webActor.authenticateWeb(page);
    await page.addInitScript(() => { localStorage.setItem("multica-locale", "en"); localStorage.setItem("multica:chat:isOpen", "false"); });
    await page.goto(`/account?view=participating&issue=${first.main.id}#memory`);
    await openHostConsent(page, first.main);
    await authorizeHostIssue(page, first, first.main);
    // Consent retains the initial target hold. Release it explicitly, then
    // pause the empty issue so a genuine eligible command can wait in queue.
    await hostControl(page, first.main, "resume", "issue");
    await hostControl(page, first.main, "stop", "issue");
    // Queue before writing v1. The prompt must observe v1 at launch rather
    // than freezing the empty document at command admission.
    const initial = await sendHostComment(page, first.main, "Memory runtime A handoff begins after the queued document update", first.webActor.id);
    expect(first.daemon.visits).toHaveLength(0);
    const seed = "Queued handoff decision: retain the source findings.";
    await saveMemoryInUI(page, seed, "0");
    await hostControl(page, first.main, "resume", "issue");
    const visitA = await first.daemon.waitForVisit(1) as MemoryVisit;
    expect(visitA.resource_owner_id).toBe(first.webActor.id);
    expect(visitA.memory_input).toEqual({ version: 1, content: seed });
    expect(visitA.memory_shown).toMatchObject({ issue_id: first.main.id, run_id: visitA.identity.run_id, version: "1", content: seed });
    expect(visitA.memory_written).toMatchObject({ code: "memory_updated", issue_id: first.main.id, run_id: visitA.identity.run_id, version: "2" });
    expect(visitA.instruction_files_clean).toBe(true);
    expect(visitA.inputs.map(input => input.command_id)).toEqual([initial.commandId]);
    first.daemon.release(visitA);
    await waitHostSettlement(first, first.main, visitA);
    const afterA = await memory();
    expect(afterA).toMatchObject({ version: "2", author: { agent: { agent_id: first.main.agentId, run_id: visitA.identity.run_id }, resource_owner: { user_id: first.webActor.id } } });

    // The current handler transfers the real issue to the second runtime's
    // owner. Its resource remains private until publicly offered, adopted and
    // explicitly consented for this issue.
    const current = await first.desktopActor.client.requestJSON<CollaborationIssue>(versioned(root));
    await first.desktopActor.client.requestJSON(`${root}/handoffs`, { method: "POST", headers: { "Idempotency-Key": randomUUID() },
      body: { ...memoryAuthority(current), purpose: "transfer", target_user_id: successor.webActor.id, note: "Continue on the second runtime and its private provider profile." } });
    const successorRead = () => successor.webActor.client.requestJSON<CollaborationIssue>(versioned(root));
    expect((await successorRead()).handler.user_id).toBe(successor.webActor.id);
    const agent = await successor.webActor.client.requestJSON<{ id: string; name: string }>("/api/agents", { method: "POST",
      body: { name: "Memory successor agent", runtime_id: successor.runtime.runtimeId, permission_mode: "private" } });
    const options = await successor.webActor.client.requestJSON<CollaborationResourceOptions>(versioned(`${root}/resources/offers/options?execution_workspace_id=${successor.workspace.id}`));
    const option = options.items.find(item => item.kind === "agent" && item.resource_id === agent.id);
    expect(option?.available).toBe(true);
    if (!option) throw new Error("Second runtime agent is not offerable");
    const offered = await successor.webActor.client.requestJSON<{ offer_id: string; offer_version: string }>(`${root}/resources/offers`, {
      method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: { ...resourceAuthority(await successorRead()),
        kind: "agent", resource_id: agent.id, execution_workspace_id: successor.workspace.id,
        roster: option.resources.map(({ role, agent_id, runtime_id, execution_workspace_id, owner_user_id }) => ({ role, agent_id, runtime_id, execution_workspace_id, owner_user_id })) },
    });
    const resources = await successor.webActor.client.requestJSON<CollaborationResources>(versioned(`${root}/resources`));
    const adopted = await successor.webActor.client.requestJSON<{ target_id: string }>(`${root}/resources/offers/${offered.offer_id}/adopt`, {
      method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: { ...resourceAuthority(await successorRead()),
        expected_offer_version: offered.offer_version, expected_target_set_version: resources.target_set_version },
    });
    const withTarget = await successorRead();
    const target = withTarget.execution?.targets.find(item => item.id === adopted.target_id);
    if (!target?.channel_id) throw new Error("Second resource adoption did not create its own execution channel");
    const identity = await successor.webActor.client.requestJSON<CollaborationResourceIdentityView>(versioned(`${root}/resources/identity?target_id=${target.id}&agent_id=${agent.id}`));
    expect(identity.identity?.installation_id).toBe(successor.runtime.installationId);
    await successor.webActor.client.requestJSON(`${root}/targets/${target.id}/consents`, { method: "POST", headers: { "Idempotency-Key": randomUUID() },
      body: { ...resourceAuthority(await successorRead()), agent_id: agent.id, expected_target_epoch: target.target_epoch,
        expected_config_version: target.config_version, identity: identity.identity } });
    const secondIssue: HostIssue = { ...first.main, targetId: target.id, channelId: target.channel_id, agentId: agent.id, name: agent.name };
    const successorPage = await successorContext.newPage();
    await successor.webActor.authenticateWeb(successorPage);
    await successorPage.addInitScript(() => { localStorage.setItem("multica-locale", "en"); localStorage.setItem("multica:chat:isOpen", "false"); });
    await successorPage.goto(new URL(`/account?view=participating&issue=${first.main.id}#memory`, page.url()).href);
    await hostControl(successorPage, secondIssue, "resume", "issue");
    const followup = await sendHostComment(successorPage, secondIssue, "Memory runtime B preserves A's findings before its own decision", successor.webActor.id);
    const visitB = await successor.daemon.waitForVisit(1) as MemoryVisit;
    expect(successor.runtime.runtimeId).not.toBe(first.runtime.runtimeId);
    expect(visitB.identity.installation_id).not.toBe(visitA.identity.installation_id);
    expect(visitB.resource_owner_id).toBe(successor.webActor.id);
    expect(visitB.memory_input).toEqual({ version: 2, content: afterA.content });
    expect(visitB.memory_shown).toMatchObject({ version: "2", content: afterA.content });
    expect(visitB.memory_written).toMatchObject({ code: "memory_updated", run_id: visitB.identity.run_id, version: "3" });
    expect(visitB.instruction_files_clean).toBe(true);
    expect(visitB.workdir_hash).not.toBe(visitA.workdir_hash);
    expect(visitB.session_hash).not.toBe(visitA.session_hash);
    expect(visitB.inputs.map(input => input.command_id)).toEqual([followup.commandId]);
    successor.daemon.release(visitB);
    await expect.poll(async () => (await successorRead()).execution?.runs.find(run => run.id === visitB.identity.run_id), { timeout: 30_000 })
      .toMatchObject({ progress_state: "settled", proof_state: "host_exit_observed", business_outcome: "success" });
    const final = await memory();
    expect(final).toMatchObject({ version: "3", author: { agent: { agent_id: agent.id, run_id: visitB.identity.run_id }, resource_owner: { user_id: successor.webActor.id } } });
    expect(final.content).toContain(seed);
    expect(final.content).toContain(first.main.agentId);
    expect(final.content).toContain(agent.id);
    const recorded = (await successorRead()).execution!.runs;
    expect(recorded.find(run => run.id === visitA.identity.run_id)?.memory).toEqual({ support: "supported", base_version: "1", written_version: "2" });
    expect(recorded.find(run => run.id === visitB.identity.run_id)?.memory).toEqual({ support: "supported", base_version: "2", written_version: "3" });
    await expect(page.locator("#memory").getByText(`Decision by agent ${agent.id}: preserve prior findings when handing off.`, { exact: false })).toBeVisible();
    await page.locator("#memory").getByRole("button", { name: "Version history", exact: true }).click();
    await page.locator("#memory").getByRole("button", { name: "View v2", exact: true }).click();
    const versionA = page.getByRole("dialog", { name: "Shared context v2", exact: true });
    await expect(versionA).toContainText("Shared host fixture agent (agent)");
    await versionA.getByRole("button", { name: "Close", exact: true }).click();
    await page.locator("#memory").getByRole("button", { name: "View v3", exact: true }).click();
    await expect(page.getByRole("dialog", { name: "Shared context v3", exact: true })).toContainText("Memory successor agent (agent)");
    evidence.issue_id = first.main.id;
    evidence.runtime_ids = [first.runtime.runtimeId, successor.runtime.runtimeId];
    evidence.actor_ids = [first.webActor.id, successor.webActor.id];
    evidence.runs = recorded.map(run => ({ id: run.id, agent_id: run.agent_id, memory: run.memory }));
    evidence.final_version = final.version;
    await page.screenshot({ path: info.outputPath("memory-two-runtime-attribution.png"), animations: "disabled" });
  } finally {
    evidence.first_daemon = await first.daemon.close();
    if (second) evidence.second_daemon = await second.daemon.close();
    await successorContext.close();
    const encoded = JSON.stringify(evidence, null, 2);
    expect(encoded).not.toMatch(/\b(?:mcb_|mct_|mdt_|mat_)[A-Za-z0-9_.-]+/);
    await info.attach("memory-runtime-handoff-evidence", { contentType: "application/json", body: encoded });
  }
});
