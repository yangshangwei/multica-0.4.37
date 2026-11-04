import { createHash, randomUUID } from "node:crypto";
import { expect, type ElectronApplication, type Locator, type Page, type TestInfo } from "@playwright/test";
import type { CollaborationIssue, CollaborationKnowledgeDetail, CollaborationKnowledgeProposal, CollaborationKnowledgeReceipt,
  CollaborationKnowledgeTargetList } from "@multica/core/collaboration";
import { TestApiClient } from "../fixtures";
import { authenticateCollaborationEditor } from "./collaboration-editing";
import { memoryAuthority, memoryEnvironment } from "./collaboration-memory";
import { nativeScreenshot, selectDesktopWorkspace } from "./collaboration-native";
import { revealCollaborationDisclosure } from "./collaboration-disclosures";

export const digest = (text: string) => createHash("sha256").update(text).digest("hex");
export const sourcePanel = (page: Page) => page.getByRole("region", { name: "Knowledge candidates", exact: true });
export const targetPanel = (page: Page) => page.getByRole("region", { name: "Managed contributions", exact: true });
export const reviewPanel = (page: Page) => page.getByRole("region", { name: "Review candidate", exact: true });
const signals = new WeakMap<Page, { frames: unknown[]; changes: { issue_id: string; event_seq: string }[]; created: Set<string> }>();

export function observeKnowledgeHints(page: Page) {
  if (!signals.has(page)) {
    const value = { frames: [] as unknown[], changes: [] as { issue_id: string; event_seq: string }[], created: new Set<string>() };
    signals.set(page, value);
    page.on("websocket", socket => socket.on("framereceived", ({ payload }) => {
      try {
        const frame = JSON.parse(payload.toString());
        if (frame.type === "collaboration:knowledge_changed") value.frames.push(frame.payload);
        if (frame.type === "collaboration:changed") {
          const change = frame.payload;
          if (typeof change?.issue_id === "string" && typeof change.event_seq === "string" && /^[1-9]\d*$/.test(change.event_seq)) {
            value.changes.push({ issue_id: change.issue_id, event_seq: change.event_seq });
          }
        }
      } catch { /* Heartbeat. */ }
    }));
  }
  const { frames } = signals.get(page)!;
  return {
    frames,
    async expectAdvance(targetId: string, previous: string) {
      await expect.poll(() => frames.some(frame => {
        if (!frame || typeof frame !== "object") return false;
        const hint = frame as Record<string, unknown>;
        return hint.target_id === targetId && typeof hint.knowledge_revision === "string" && /^\d+$/.test(hint.knowledge_revision)
          && BigInt(hint.knowledge_revision) > BigInt(previous);
      }), { timeout: 10_000 }).toBe(true);
      for (const frame of frames) {
        expect(frame).toEqual({ target_kind: expect.stringMatching(/^(workspace|project)$/), target_id: expect.any(String), knowledge_revision: expect.stringMatching(/^\d+$/) });
      }
    },
  };
}

/** Every human/resource row comes from its public API. Gate setup is owned by
 * the spec; no runtime, engine task, Provider, or installed executable is used. */
export async function seedKnowledge() {
  memoryEnvironment();
  const suffix = randomUUID().slice(0, 10);
  const owner = new TestApiClient();
  await owner.login(`knowledge-owner-${suffix}@example.invalid`, "Knowledge source owner");
  const workspace = await owner.ensureWorkspace("Knowledge source workspace", `knowledge-${suffix}`);
  await owner.markUserOnboarded();
  const ownerId = (await owner.requestJSON<{ id: string }>("/api/me")).id;
  const handler = new TestApiClient();
  await handler.login(`knowledge-handler-${suffix}@example.invalid`, "External knowledge handler");
  await handler.markUserOnboarded();
  const handlerId = (await handler.requestJSON<{ id: string }>("/api/me")).id;
  expect(await handler.getWorkspaces()).toEqual([]);
  const base = "Human-maintained base context stays separate.";
  await owner.requestJSON(`/api/workspaces/${workspace.id}`, { method: "PATCH", body: { context: base } });
  const project = await owner.requestJSON<{ id: string; title: string }>("/api/projects", { method: "POST", body: {
    title: `Knowledge project ${suffix}`, description: base,
  } });
  const issue = await owner.requestJSON<CollaborationIssue>("/api/collaboration/issues", { method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: {
    source_workspace_id: workspace.id, handler_user_id: handlerId, project_id: project.id,
    title: `Knowledge review ${suffix}`, description: "Synthetic source for independently approved, traceable knowledge.",
  } });
  const root = `/api/collaboration/issues/${issue.id}`;
  const comment = "Observed result: repeated delivery must preserve the original request identity.";
  const authority = memoryAuthority(issue);
  const posted = await owner.requestJSON<{ comment_id: string; comment_revision: string }>(`${root}/comments`, {
    method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: { content: comment, target_versions: [],
      expected_lifecycle_epoch: authority.expected_lifecycle_epoch, expected_participant_epoch: authority.expected_participant_epoch,
      expected_account_epoch: authority.expected_account_epoch },
  });
  const memory = "Shared decision: a repeated request must not publish a second contribution.";
  await owner.requestJSON(`${root}/memory`, { method: "PUT", headers: { "Idempotency-Key": randomUUID() }, body: { expected_version: "0", content: memory } });
  return { owner, ownerId, handler, handlerId, workspace, project, issue, root, base, comment, commentId: posted.comment_id, memory,
    path: `/account?view=participating&issue=${issue.id}` };
}
export type KnowledgeFixture = Awaited<ReturnType<typeof seedKnowledge>>;

export async function openKnowledge(page: Page, actor: TestApiClient, path: string) {
  observeKnowledgeHints(page);
  await authenticateCollaborationEditor(page, actor.getToken()!);
  await page.goto(path);
  await revealCollaborationDisclosure(page, "Knowledge candidates");
  await expect(sourcePanel(page)).toBeVisible();
}

export async function selectKnowledgeOption(page: Page, form: Locator, name: string, option: string | RegExp) {
  await form.getByRole("combobox", { name, exact: true }).click();
  await expect(page.getByRole("listbox")).toBeVisible();
  await page.getByRole("option", { name: option, exact: typeof option === "string" }).click();
  await expect(page.getByRole("listbox")).toHaveCount(0);
}

export async function fillKnowledge(page: Page, summary: string, content: string, kind: "Comment" | "Shared context" = "Comment", destination?: RegExp) {
  const form = page.getByRole("form", { name: "Propose knowledge", exact: true });
  await expect(form.getByLabel("Summary", { exact: true })).toBeFocused();
  await form.getByLabel("Summary", { exact: true }).fill(summary);
  await form.getByLabel("Conclusion (Markdown)", { exact: true }).fill(content);
  if (destination) await selectKnowledgeOption(page, form, "Destination", destination);
  await selectKnowledgeOption(page, form, "Sources", kind);
  await form.getByRole("checkbox", { name: new RegExp(`^${kind} ·`) }).first().check();
  return form;
}

export async function submitKnowledge(page: Page) {
  const [received] = await Promise.all([
    page.waitForResponse((r) => /\/knowledge-candidates$/.test(new URL(r.url()).pathname) && r.request().method() === "POST"),
    page.getByRole("form", { name: "Propose knowledge", exact: true }).getByRole("button", { name: "Submit candidate", exact: true }).click(),
  ]);
  expect(received.status()).toBe(201);
  const receipt: CollaborationKnowledgeReceipt = await received.json();
  const sent: CollaborationKnowledgeProposal = received.request().postDataJSON();
  expect(receipt).toMatchObject({ code: "knowledge_proposed", state: "pending", candidate_id: sent.candidate_id, request_id: sent.request_id });
  signals.get(page)?.created.add(receipt.candidate_id);
  await expect(page.getByRole("form", { name: "Propose knowledge", exact: true })).toHaveCount(0);
  return { receipt, sent };
}

export async function chooseCandidate(page: Page, summary: string, panel = sourcePanel(page)) {
  await panel.getByRole("button", { name: new RegExp(`^${summary.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")}`) }).click();
  await expect(reviewPanel(page).getByText(summary, { exact: true })).toBeVisible();
}

export async function approveKnowledge(page: Page, actor: TestApiClient, candidateId: string) {
  const reviewed = (await actor.requestJSON<CollaborationKnowledgeDetail>(`/api/collaboration/knowledge-candidates/${candidateId}`)).candidate;
  if (reviewed.availability !== "available") throw new Error("Approval requires readable material");
  // Wait for this page's own proposal invalidation before human confirmation.
  // A network-idle delay alone can end before the outbox delivers that event.
  const observed = signals.get(page);
  if (observed?.created.has(candidateId)) {
    const source = await actor.requestJSON<CollaborationIssue>(`/api/collaboration/issues/${reviewed.source_issue_id}`);
    const sequence = source.realtime?.event_seq;
    if (!sequence) throw new Error("Knowledge acceptance needs the source snapshot cursor");
    await expect.poll(() => observed.changes.some(change => change.issue_id === reviewed.source_issue_id && BigInt(change.event_seq) >= BigInt(sequence))).toBe(true);
  }
  await page.waitForLoadState("networkidle");
  const panel = reviewPanel(page);
  await expect(panel.getByRole("button", { name: "Approve exact version", exact: true })).toBeDisabled();
  const confirmation = panel.getByRole("checkbox", { name: "I reviewed this exact version and its sources.", exact: true });
  await confirmation.focus();
  await page.keyboard.press("Space");
  await expect(confirmation).toBeChecked();
  const [approved] = await Promise.all([
    page.waitForResponse((r) => new URL(r.url()).pathname === `/api/collaboration/knowledge-candidates/${candidateId}/approve`),
    panel.getByRole("button", { name: "Approve exact version", exact: true }).click(),
  ]);
  expect(approved.status()).toBe(200);
  expect(approved.request().postDataJSON()).toMatchObject({ expected_version: reviewed.version,
    expected_state_revision: reviewed.state_revision, expected_content_sha256: reviewed.content_sha256 });
  await expect(panel.getByText("Approved", { exact: true })).toBeVisible();
  return approved.request().postDataJSON();
}

export async function apiProposal(f: KnowledgeFixture, options: Partial<CollaborationKnowledgeProposal> = {}, actor = f.handler) {
  const body: CollaborationKnowledgeProposal = { protocol_version: 1, candidate_id: randomUUID(), request_id: randomUUID(),
    summary: "Traceable shared decision", content: "Approved material follows current source authority.",
    target: { kind: "workspace", workspace_id: f.workspace.id }, sources: [{ kind: "memory", issue_id: f.issue.id, version: "1", sha256: digest(f.memory) }],
    evidence: [], applicability: { surfaces: ["issue", "collaboration"] }, expires_at: null, supersedes_candidate_id: null,
    derived_from_candidate_id: null, ...options };
  const receipt = await actor.requestJSON<CollaborationKnowledgeReceipt>(`${f.root}/knowledge-candidates`, { method: "POST", body });
  expect(receipt.code).toBe("knowledge_proposed");
  return { body, receipt };
}

export async function apiApprove(actor: TestApiClient, candidateId: string) {
  const c = (await actor.requestJSON<CollaborationKnowledgeDetail>(`/api/collaboration/knowledge-candidates/${candidateId}`)).candidate;
  if (c.availability !== "available") throw new Error("Fixture approval requires lawful full source");
  return actor.requestJSON<CollaborationKnowledgeReceipt>(`/api/collaboration/knowledge-candidates/${candidateId}/approve`, { method: "POST", body: {
    protocol_version: 1, request_id: randomUUID(), expected_version: c.version, expected_state_revision: c.state_revision, expected_content_sha256: c.content_sha256,
  } });
}

export async function targetItems(actor: TestApiClient, id: string, kind: "workspaces" | "projects" = "workspaces") {
  return actor.requestJSON<CollaborationKnowledgeTargetList>(`/api/${kind}/${id}/knowledge`);
}

async function exposeProjectProperties(page: Page, panel: Locator) {
  const toggle = page.getByRole("button", { name: "Toggle right sidebar", exact: true });
  if (!await toggle.isVisible()) return;
  // A collapsed resizable panel keeps its children mounted; visibility alone
  // does not establish that a human can interact inside its clipped width.
  const usable = await panel.isVisible() && await panel.evaluate(element => {
    const container = element.closest('[data-panel="true"]');
    return !container || container.getBoundingClientRect().width >= 200;
  });
  if (!usable) await toggle.click();
}

export async function openTarget(page: Page, f: KnowledgeFixture, kind: "project" | "workspace", native?: ElectronApplication) {
  if (!native) {
    // Enter each target with the wide shell before its deliberate capture cycle.
    // This avoids carrying an open mobile properties portal into that cycle.
    await page.setViewportSize({ width: 1380, height: 960 });
    await page.goto(`/${f.workspace.slug}/${kind === "project" ? `projects/${f.project.id}` : "settings?tab=workspace"}`);
  }
  else {
    const window = await native.browserWindow(page);
    await window.evaluate(w => w.setContentSize(1380, 960));
    await window.dispose();
    await expect.poll(() => page.evaluate(() => innerWidth)).toBe(1380);
    await selectDesktopWorkspace(page, f.workspace);
    await page.getByRole("link", { name: kind === "project" ? "Projects" : "Settings", exact: true }).and(page.locator('[data-sidebar="menu-button"]')).click();
    if (kind === "project") await page.getByText(f.project.title, { exact: true }).first().click();
    else await page.getByRole("tab", { name: "General", exact: true }).click();
  }
  if (kind === "project") {
    await expect(page.getByRole("button", { name: "Toggle right sidebar", exact: true })).toBeVisible();
    await exposeProjectProperties(page, targetPanel(page));
  }
  await expect(targetPanel(page)).toBeVisible();
}

function inspectKnowledgePanel(element: HTMLElement, offset: number | null) {
  const ancestors: HTMLElement[] = [];
  let node = element.parentElement;
  while (node) { ancestors.push(node); node = node.parentElement; }
  const root = ancestors.find(parent => /auto|scroll/.test(getComputedStyle(parent).overflowY) && parent.scrollHeight > parent.clientHeight);
  if (!root) throw new Error("Knowledge capture requires its actual scroll root");
  const clip = { left: 0, top: 0, right: innerWidth, bottom: innerHeight };
  for (const parent of ancestors) {
    const style = getComputedStyle(parent), bounds = parent.getBoundingClientRect();
    if (/auto|scroll|hidden|clip/.test(style.overflowX)) { clip.left = Math.max(clip.left, bounds.left); clip.right = Math.min(clip.right, bounds.right); }
    if (/auto|scroll|hidden|clip/.test(style.overflowY)) { clip.top = Math.max(clip.top, bounds.top); clip.bottom = Math.min(clip.bottom, bounds.bottom); }
  }
  if (offset !== null) root.scrollTop += element.getBoundingClientRect().top - clip.top - 16 + offset;
  const rect = element.getBoundingClientRect();
  const sheet = element.closest<HTMLElement>('[data-slot="sheet-content"]');
  const sheetStyle = sheet ? getComputedStyle(sheet) : null;
  const chain = [element, ...ancestors];
  const opacity = chain.reduce((value, parent) => value * Number(getComputedStyle(parent).opacity), 1);
  const animations = new Set(chain.flatMap(parent => parent.getAnimations()));
  const runningAnimations = [...animations].filter(animation => animation.playState === "running"
    && animation.effect?.getComputedTiming().iterations !== Infinity).length;
  const visibleStart = Math.max(0, clip.top - rect.top);
  const visibleEnd = Math.max(0, Math.min(rect.height, clip.bottom - rect.top));
  const visibleTop = Math.max(rect.top, clip.top), visibleBottom = Math.min(rect.bottom, clip.bottom);
  const x = (Math.max(rect.left, clip.left) + Math.min(rect.right, clip.right)) / 2;
  const unobscured = visibleBottom > visibleTop && [visibleTop + 4, (visibleTop + visibleBottom) / 2, visibleBottom - 4].every(y => {
    const hit = document.elementFromPoint(x, y);
    return hit !== null && element.contains(hit);
  });
  const controls = [...element.querySelectorAll<HTMLElement>('button,input,textarea,summary,a,[role="checkbox"]')]
    .filter(control => {
      const closed = control.closest("details:not([open])");
      if (closed && !closed.querySelector(":scope > summary")?.contains(control)) return false;
      const box = control.getBoundingClientRect();
      return box.width > 0 && box.height > 0;
    });
  const last = controls.at(-1);
  return { viewport: { width: innerWidth, height: innerHeight }, devicePixelRatio, rect: rect.toJSON(), clip,
    root: root.getBoundingClientRect().toJSON(), scrollTop: root.scrollTop, scrollHeight: root.scrollHeight,
    visibleStart, visibleEnd, opacity, runningAnimations, unobscured,
    sheet: sheetStyle ? { opacity: sheetStyle.opacity, transform: sheetStyle.transform, translate: sheetStyle.translate,
      background: sheetStyle.backgroundColor, starting: sheet!.hasAttribute("data-starting-style"), ending: sheet!.hasAttribute("data-ending-style") } : null,
    lastControl: last ? { text: last.textContent?.trim(), rect: last.getBoundingClientRect().toJSON() } : null,
    overflow: document.documentElement.scrollWidth > innerWidth + 1 };
}

async function settleKnowledgeCapture(page: Page, panel: Locator) {
  await expect.poll(async () => {
    const before = await panel.evaluate(inspectKnowledgePanel, null);
    await page.evaluate(() => new Promise<void>(resolve => requestAnimationFrame(() => requestAnimationFrame(() => resolve()))));
    const after = await panel.evaluate(inspectKnowledgePanel, null);
    return { opaque: after.opacity >= 0.999, animations: after.runningAnimations,
      stable: ["x", "y", "width", "height"].every(key => Math.abs(before.rect[key] - after.rect[key]) < 0.25),
      settledSheet: !after.sheet || after.sheet.opacity === "1" && !after.sheet.starting && !after.sheet.ending
        && ["none", "0px", "0px 0px"].includes(after.sheet.translate),
      usableWidth: after.rect.width >= 200 && after.clip.right - after.clip.left >= 200 };
  }, { timeout: 10_000 }).toEqual({ opaque: true, animations: 0, stable: true, settledSheet: true, usableWidth: true });
}

/** Capture settled, unobscured viewport frames through the current panel end.
 * Responsive project properties remount, so restore the reviewed candidate. */
export async function captureKnowledge(page: Page, panel: Locator, info: TestInfo, stage: string, native?: ElectronApplication, reviewSummary?: string) {
  const frames: unknown[] = [];
  for (const width of [1380, 640]) {
    if (native) {
      const window = await native.browserWindow(page);
      await window.evaluate((w, size) => w.setContentSize(size, 960), width);
      await window.dispose();
    } else await page.setViewportSize({ width, height: 960 });
    await expect.poll(() => page.evaluate(() => innerWidth)).toBe(width);
    if (stage === "project-managed") {
      const sidebar = page.locator('[data-right-sidebar-panel="true"]');
      const sheet = page.locator('[data-slot="sheet-content"]');
      // The viewport changes before React switches between panel and sheet.
      await expect(sidebar).toHaveCount(width < 768 ? 0 : 1);
      if (width < 768) {
        // ProjectDetail's resize effect closes the mobile sheet after commit.
        // Let that paint/effect boundary finish before opening the real control.
        await page.evaluate(() => new Promise<void>(resolve => requestAnimationFrame(() => requestAnimationFrame(() => resolve()))));
        const openSheet = page.locator('[data-slot="sheet-content"][data-open]');
        if (await openSheet.count() === 0) await page.getByRole("button", { name: "Toggle right sidebar", exact: true }).click();
        await expect(openSheet).toBeVisible();
      } else {
        await expect(sheet).toHaveCount(0);
        await exposeProjectProperties(page, panel);
      }
    }
    await expect(panel).toBeVisible();
    await page.evaluate(() => document.fonts.ready);
    await page.waitForLoadState("networkidle");
    if (reviewSummary) {
      if (!await reviewPanel(page).getByText(reviewSummary, { exact: true }).isVisible()) await chooseCandidate(page, reviewSummary, panel);
      await expect(reviewPanel(page).getByText(reviewSummary, { exact: true })).toBeVisible();
    }
    await settleKnowledgeCapture(page, panel);
    let covered = 0;
    let complete = false;
    for (let part = 0; part < 20; part++) {
      await panel.evaluate(inspectKnowledgePanel, Math.max(0, covered - 64));
      await settleKnowledgeCapture(page, panel);
      const measured = await panel.evaluate(inspectKnowledgePanel, null);
      expect(measured.overflow).toBe(false);
      expect(measured.unobscured).toBe(true);
      expect(measured.visibleStart).toBeLessThanOrEqual(covered + 1);
      expect(measured.visibleEnd).toBeGreaterThan(covered);
      expect(measured.rect.left).toBeGreaterThanOrEqual(measured.clip.left - 1);
      expect(measured.rect.right).toBeLessThanOrEqual(measured.clip.right + 1);
      covered = Math.max(covered, measured.visibleEnd);
      const name = `${native ? "desktop" : "web"}-${stage}-${width}-part-${part + 1}.png`;
      const path = info.outputPath(name);
      if (native) await nativeScreenshot(native, page, path);
      else await page.screenshot({ path, fullPage: false, animations: "disabled", scale: "css" });
      const after = await panel.evaluate(inspectKnowledgePanel, null);
      expect(after.opacity).toBeGreaterThanOrEqual(0.999);
      expect(after.unobscured).toBe(true);
      for (const key of ["x", "y", "width", "height"]) expect(Math.abs(measured.rect[key] - after.rect[key])).toBeLessThan(0.25);
      await info.attach(name, { path, contentType: "image/png" });
      frames.push({ name, ...measured, after });
      if (after.rect.bottom <= after.clip.bottom - 4) {
        expect(covered).toBeGreaterThanOrEqual(after.rect.height - 1);
        if (after.lastControl) {
          expect(after.lastControl.rect.top).toBeGreaterThanOrEqual(after.clip.top);
          expect(after.lastControl.rect.bottom).toBeLessThanOrEqual(after.clip.bottom - 4);
        }
        complete = true;
        break;
      }
    }
    expect(complete, `${stage} ${width}px must include the entire panel and last control`).toBe(true);
  }
  await info.attach(`${stage}-capture-geometry`, { contentType: "application/json", body: JSON.stringify(frames, null, 2) });
}
