// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiClient } from "../api/client";
import { setSchemaLogger } from "../api/schema";
import { setCurrentWorkspace } from "../platform/workspace-storage";
import { canReviewMaterialRevision, newestMaterialRevision, parseMaterialDetail, parseMaterialList, parseMaterialSettings, type MaterialDetail } from "./schema";

const sha = "a".repeat(40);
const digest = "b".repeat(64);
function detail(): MaterialDetail {
  return {
    material: { id: "material", workspace_id: "ws", project_id: "project", title: "Refunds", path: "requirements/REQ-1/prd.md", version: 3, status: "in_review", blocker: null, created_at: "2026-11-26T00:00:00Z", updated_at: "2026-11-26T00:01:00Z" },
    revisions: [{ id: "revision", kind: "requirement", content: "Private requirement", plan_content: null, content_sha256: digest, sources: [{ resource_id: "resource", commit: sha, path: "src/refund.go" }], acceptance_criteria: ["AC1: Refund succeeds"], blocking_questions: [], source_baseline_id: null, publication_status: "in_review", review_url: "https://github.com/org/materials/pull/1", head_sha: sha, created_at: "2026-11-26T00:01:00Z" }],
    decisions: [], comments: [], baseline: null, plan: null,
    capabilities: { can_edit: true, can_review: true, can_start_plan: false, can_retry: false },
  };
}
const settings = { version: 2, enabled: true, provider: "github", resource_id: "resource", connection_id: "installation-row", repository: "org/materials", base_branch: "main", root_path: "", reviewer_ids: ["user"], plan_agent_id: null, auto_start_plan: false };
const envelope = () => ({ workspace_id: "ws", project_id: "project", can_configure: true, settings });
function respond(value: unknown) {
  const fetcher = vi.fn().mockImplementation(async () => new Response(JSON.stringify(value), { status: 200, headers: { "Content-Type": "application/json" } }));
  vi.stubGlobal("fetch", fetcher);
  return fetcher;
}
afterEach(() => { vi.unstubAllGlobals(); setCurrentWorkspace(null, null); setSchemaLogger({ debug() {}, info() {}, warn() {}, error() {} }); });

describe("material API contracts", () => {
  it("preserves explicit false and refuses malformed default-enabled configuration", () => {
    expect(parseMaterialSettings(envelope(), "ws", "project").settings?.auto_start_plan).toBe(false);
    for (const auto_start_plan of [undefined, null, "false", 1]) {
      expect(() => parseMaterialSettings({ ...envelope(), settings: { ...settings, auto_start_plan } }, "ws", "project")).toThrow();
    }
    expect(() => parseMaterialSettings({ ...envelope(), project_id: "foreign" }, "ws", "project")).toThrow();
  });

  it("does not reinterpret malformed lists or authorization as empty or approved", () => {
    expect(() => parseMaterialList({ workspace_id: "ws", project_id: "project", materials: null }, "ws", "project")).toThrow();
    const changed = detail();
    changed.material.workspace_id = "other";
    expect(() => parseMaterialList({ workspace_id: "ws", project_id: "project", materials: [changed.material] }, "ws", "project")).toThrow();
    expect(() => parseMaterialDetail({ ...detail(), capabilities: { can_review: "true" } }, "ws", "project")).toThrow();
    expect(() => parseMaterialDetail(detail(), "ws", "project", "foreign")).toThrow();
  });

  it("keeps private text out of parse diagnostics", () => {
    const warn = vi.fn(); setSchemaLogger({ debug() {}, info() {}, warn, error() {} });
    expect(() => parseMaterialDetail({ private_text: "keep secret" }, "ws", "project")).toThrow();
    expect(JSON.stringify(warn.mock.calls)).not.toContain("keep secret");
    expect(warn.mock.calls[0]?.[1]).toEqual({ endpoint: "project material detail" });
  });

  it("requires consistent baseline content and safe remote links", () => {
    const changed = detail();
    changed.baseline = { id: "baseline", revision_id: "revision", commit: sha, path: changed.material.path, content_sha256: "c".repeat(64), review_url: "https://example.test/review", verified_at: "now" };
    expect(() => parseMaterialDetail(changed, "ws", "project")).toThrow();
    changed.baseline = null; changed.revisions[0]!.review_url = "javascript:alert(1)";
    expect(() => parseMaterialDetail(changed, "ws", "project")).toThrow();
    changed.revisions[0]!.review_url = "https://token@example.test/review";
    expect(() => parseMaterialDetail(changed, "ws", "project")).toThrow();
  });

  it("shows unknown states but never enables their approval", () => {
    const changed = detail(); changed.material.status = "future_state";
    const parsed = parseMaterialDetail(changed, "ws", "project");
    expect(parsed.material.status).toBe("future_state");
    expect(canReviewMaterialRevision(parsed, parsed.revisions[0]!)).toBe(false);
  });

  it("requires the current revision and current requirement baseline for plan approval", () => {
    const current = detail();
    const older = current.revisions[0]!;
    expect(canReviewMaterialRevision(current, older)).toBe(true);
    current.revisions.push({ ...older, id: "newer", created_at: "2026-11-26T00:02:00Z" });
    expect(canReviewMaterialRevision(current, older)).toBe(false);
    const plan = { ...older, id: "plan", kind: "plan", source_baseline_id: "old-baseline", created_at: "2026-11-26T00:03:00Z" };
    current.revisions.push(plan);
    expect(canReviewMaterialRevision(current, plan)).toBe(false);
    current.baseline = { id: "old-baseline", revision_id: older.id, commit: sha, path: current.material.path, content_sha256: digest, review_url: older.review_url!, verified_at: "now" };
    expect(canReviewMaterialRevision(current, plan)).toBe(true);
  });

  it("uses server revision order when RFC3339 timestamp strings disagree or tie", () => {
    const current = detail();
    const older = current.revisions[0]!;
    const newer = { ...older, id: "newer", created_at: "2026-11-26T00:01:00.1Z" };
    const latest = { ...newer, id: "latest" };
    current.revisions.push(newer, latest);
    expect(newestMaterialRevision(current, "requirement")).toBe(latest);
    expect(canReviewMaterialRevision(current, older)).toBe(false);
    expect(canReviewMaterialRevision(current, newer)).toBe(false);
    expect(canReviewMaterialRevision(current, latest)).toBe(true);
  });

  it("captures workspace for reads and sends unchanged original command bodies on retry", async () => {
    const fetcher = respond(detail());
    const client = new ApiClient("https://api.example.test");
    setCurrentWorkspace("other", "other");
    await client.getMaterial("project", "material", { workspaceId: "ws" });
    const command = { action: "decisions" as const, data: { request_id: "original-key", expected_version: 3, revision_id: "revision", head_sha: sha, decision: "approve" as const, reason: "Reviewed" } };
    await client.sendMaterialCommand("project", "material", command, { workspaceId: "ws" });
    await client.sendMaterialCommand("project", "material", command, { workspaceId: "ws" });
    expect(fetcher.mock.calls).toHaveLength(3);
    for (const [, init] of fetcher.mock.calls) expect(new Headers(init.headers).get("X-Workspace-ID")).toBe("ws");
    expect(fetcher.mock.calls[1]?.[0]).toBe("https://api.example.test/api/projects/project/materials/material/decisions");
    expect(fetcher.mock.calls[1]?.[1].body).toBe(fetcher.mock.calls[2]?.[1].body);
    expect(JSON.parse(fetcher.mock.calls[1]?.[1].body)).toEqual(command.data);
  });

  it("sends installation row identity and false setting without optimistic defaults", async () => {
    const fetcher = respond(envelope());
    const { version, ...input } = settings;
    const result = await new ApiClient("https://api.example.test").saveMaterialSettings("project", { ...input, provider: "github", expected_version: version, request_id: "settings-key" }, { workspaceId: "ws" });
    expect(result.settings?.auto_start_plan).toBe(false);
    expect(JSON.parse(fetcher.mock.calls[0]?.[1].body)).toMatchObject({ connection_id: "installation-row", auto_start_plan: false, expected_version: 2 });
  });
});
