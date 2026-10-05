import { randomUUID } from "node:crypto";
import type { Page } from "@playwright/test";
import type { TestApiClient } from "../fixtures";
import { p1DB, p1EnableTriage, p1Project, type P1Issue, type P1Overview, type P1Project } from "./project-p1";

export type P1RiskSignal = "blocked" | "overdue" | "unassigned" | "in_review";
export const p1RiskLabels: Record<P1RiskSignal, string> = {
  blocked: "Blocked", overdue: "Overdue", unassigned: "Unassigned", in_review: "Awaiting review",
};
export interface P1RiskPage {
  items: P1Issue[];
  next_cursor: string | null;
  refreshed: boolean;
  snapshot_version: string;
  total: number;
  overview: P1Overview;
}

export function p1Metric(page: Page, label: string) {
  return page.locator("dl > div").filter({ has: page.locator("dt", { hasText: new RegExp(`^${label}$`) }) }).locator("dd");
}

export function p1DescriptionEditor(page: Page) {
  return page.getByRole("button", { name: "Description", exact: true }).locator("..").locator('[contenteditable="true"]');
}

export async function p1PendingCandidate(api: TestApiClient, projectId: string, title = "Candidate not counted until accepted") {
  await p1EnableTriage(api);
  return (await api.requestJSON<{ issue: P1Issue }>("/api/triage/items", {
    method: "POST", body: { title, request_id: randomUUID(), candidate_project_id: projectId },
  })).issue;
}

export async function p1FormalGolden(api: TestApiClient, project: P1Project) {
  const issues: P1Issue[] = [];
  for (let i = 0; i < 10; i++) {
    issues.push(await api.createIssue(`Formal item ${i + 1}`, {
      project_id: project.id, status: i < 6 ? "done" : i < 8 ? "cancelled" : "todo",
      assignee_type: null, assignee_id: null, due_date: "2026-10-15",
    }));
  }
  return issues;
}

export async function p1PagedRisks(api: TestApiClient, workspaceId: string) {
  const project = await p1Project(api);
  const issues = await api.seedTableIssues(Array.from({ length: 156 }, (_, i) => ({
    title: `Overdue delivery ${String(i + 1).padStart(3, "0")}`, status: "todo" as const,
  })));
  const ordered = [...issues].sort((a, b) => a.id.localeCompare(b.id));
  const reentering = ordered[0]!;
  await p1DB("UPDATE issue SET project_id=$1,due_date='2020-01-01' WHERE workspace_id=$2 AND id=ANY($3::uuid[])", [project.id, workspaceId, issues.map((issue) => issue.id)]);
  await p1DB("UPDATE issue SET status='done' WHERE workspace_id=$1 AND id=$2", [workspaceId, reentering.id]);
  await p1DB("UPDATE issue SET parent_issue_id=$1 WHERE workspace_id=$2 AND id=$3", [ordered[10]!.id, workspaceId, ordered[20]!.id]);
  return { project, ordered, reentering };
}

export function p1RiskSection(page: Page, signal: P1RiskSignal) {
  return page.locator("section").filter({ has: page.getByRole("heading", { name: new RegExp(`^${p1RiskLabels[signal]} ·`) }) });
}

export function p1RiskResponse(page: Page, projectId: string, cursor: boolean, status = 200) {
  return page.waitForResponse((response) => {
    const url = new URL(response.url());
    return url.pathname === `/api/projects/${projectId}/health/issues`
      && url.searchParams.has("cursor") === cursor && response.status() === status;
  });
}

export async function p1OpenRisk(page: Page, workspaceSlug: string, projectId: string, signal: P1RiskSignal, count: number) {
  await page.goto(`/${workspaceSlug}/projects/${projectId}?section=overview`);
  const response = p1RiskResponse(page, projectId, false);
  await page.getByRole("button", { name: `${p1RiskLabels[signal]} ${count}`, exact: true }).focus();
  await page.keyboard.press("Enter");
  const first: P1RiskPage = await (await response).json();
  return { first, risk: p1RiskSection(page, signal) };
}

export async function p1NextRisk(page: Page, projectId: string) {
  const response = p1RiskResponse(page, projectId, true);
  await page.getByRole("button", { name: "Next page", exact: true }).click();
  const next: P1RiskPage = await (await response).json();
  return next;
}

/** Mutate only after the browser submits its captured cursor, avoiding an
 * unrelated websocket refresh while retaining a real database/API handoff. */
export async function p1ReenterOnContinuation(page: Page, workspaceId: string, projectId: string, issueId: string) {
  let changed = false;
  await page.route(`**/api/projects/${projectId}/health/issues?*`, async (route) => {
    if (!changed && new URL(route.request().url()).searchParams.has("cursor")) {
      changed = true;
      await p1DB("UPDATE issue SET status='todo',revision=revision+1 WHERE workspace_id=$1 AND id=$2", [workspaceId, issueId]);
    }
    await route.continue();
  });
}
