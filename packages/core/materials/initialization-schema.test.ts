// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiClient, ApiError } from "../api/client";
import { isProjectAccessLost } from "../projects/access";
import { canPublishInitializedMaterial, canReviewInitialization, decodeMaterialInitializationCommand, initializationReviewKey, parseMaterialInitialization, type MaterialInitialization } from "./initialization-schema";

const sha = "a".repeat(40), digest = "b".repeat(64);
function data(): MaterialInitialization { return {
  workspace_id: "ws", project_id: "project", protocol_version: 1, version: 4, settings_version: 2, binding_generation: 1, status: "in_review", blocker: null,
  readiness: { publish_allowed: false, verified_commit: null, manifest_sha256: null, schema_version: null, bundle_version: null, verified_at: null }, operation: null,
  candidate: { id: "candidate", kind: "initialize", version: 1, binding_generation: 1, bundle_version: "1.0.0", manifest_sha256: digest, base_commit: sha, head_sha: sha, review_url: "https://github.com/acme/material/pull/1", publication_status: "in_review", files: [{ path: "AGENTS.md", action: "add", before_sha256: null, before_content: null, after_sha256: digest, after_content: "# Reviewed rules", explanation: null }] },
  decisions: [], available_bundle_versions: ["1.0.0"], capabilities: { can_check: true, can_review: true, can_retry: false, can_maintain: true },
}; }
afterEach(() => vi.unstubAllGlobals());
describe("repository initialization boundary", () => {
  it("rejects wrong scope, partial or repeated file lists and credentialed links", () => {
    expect(parseMaterialInitialization(data(), "ws", "project")).toEqual(data());
    expect(() => parseMaterialInitialization(data(), "other", "project")).toThrow();
    const value = data(); const file = value.candidate!.files[0]!;
    for (const files of [null, [file, file], [{ ...file, after_content: null }], [{ ...file, path: "../AGENTS.md" }]]) expect(() => parseMaterialInitialization({ ...value, candidate: { ...value.candidate, files } }, "ws", "project")).toThrow();
    expect(() => parseMaterialInitialization({ ...value, candidate: { ...value.candidate, review_url: "https://secret@github.com/acme/material/pull/1" } }, "ws", "project")).toThrow();
  });
  it("displays conflict inspections and old proof history without granting approval", () => {
    const value = data();
    value.status = "blocked"; value.candidate!.manifest_sha256 = ""; value.candidate!.head_sha = null;
    value.candidate!.files[0]!.action = "conflict";
    value.history = [{ publish_allowed: true, verified_commit: sha, manifest_sha256: digest, schema_version: 1, bundle_version: "1.0.0", verified_at: "2026-11-27T00:00:00Z" }];
    expect(parseMaterialInitialization(value, "ws", "project")).toEqual(value);
    expect(canReviewInitialization(value)).toBe(false); expect(canPublishInitializedMaterial(value, 2)).toBe(false);
  });
  it("requires a supported current proof independently from candidate lifecycle", () => {
    const value = data(); expect(canPublishInitializedMaterial(value, 2)).toBe(false);
    value.readiness = { publish_allowed: true, verified_commit: sha, manifest_sha256: digest, schema_version: 1, bundle_version: "1.0.0", verified_at: "2026-11-27T00:00:00Z" };
    expect(canPublishInitializedMaterial(value, 2)).toBe(true);
    expect(canPublishInitializedMaterial(value, 3)).toBe(false);
    expect(canPublishInitializedMaterial({ ...value, protocol_version: 9 }, 2)).toBe(false);
    expect(canPublishInitializedMaterial({ ...value, status: "future" }, 2)).toBe(false);
    expect(canPublishInitializedMaterial({ ...value, readiness: { ...value.readiness, verified_commit: null } }, 2)).toBe(false);
    expect(canPublishInitializedMaterial({ ...value, readiness: { ...value.readiness, publish_allowed: false } }, 2)).toBe(false);
  });
  it("pins maintenance decisions to the full candidate and refuses unknown file actions", () => {
    const value = data(); expect(canReviewInitialization(value)).toBe(true);
    for (const publication_status of ["changes_requested", "withdrawn"]) expect(canReviewInitialization({ ...value, candidate: { ...value.candidate!, publication_status } })).toBe(true);
    const pinned = initializationReviewKey(value);
    expect(canReviewInitialization({ ...value, capabilities: { ...value.capabilities, can_review: false } })).toBe(false);
    for (const patch of [{ head_sha: "c".repeat(40) }, { version: 2 }, { manifest_sha256: "d".repeat(64) }, { binding_generation: 2 }]) expect(initializationReviewKey({ ...value, candidate: { ...value.candidate!, ...patch } })).not.toBe(pinned);
    expect(canReviewInitialization({ ...value, candidate: { ...value.candidate!, files: [{ ...value.candidate!.files[0]!, action: "future" }] } })).toBe(false);
  });
  it("retains exact original command and refuses hidden authority fields", () => {
    const command = { action: "decisions", data: { request_id: "original", expected_version: 4, expected_settings_version: 2, candidate_id: "candidate", head_sha: sha, manifest_sha256: digest, decision: "approve", reason: "Reviewed all changes" } };
    expect(decodeMaterialInitializationCommand(command)).toEqual(command);
    expect(decodeMaterialInitializationCommand({ ...command, data: { ...command.data, actor_id: "admin" } })).toBeNull();
    expect(decodeMaterialInitializationCommand({ action: "maintenance", data: { request_id: "r", expected_version: 4, expected_settings_version: 2, target_bundle_version: "1.0.0" } })).not.toBeNull();
  });
  it("does not turn maintenance permission refusal into project access loss", () => {
    expect(isProjectAccessLost(new ApiError("denied", 403, "Forbidden", { error: "denied", code: "material_maintenance_permission_denied" }))).toBe(false);
    expect(isProjectAccessLost(new ApiError("denied", 403, "Forbidden"))).toBe(true);
  });
  it("captures workspace headers and parses command responses", async () => {
    const fetcher = vi.fn(async (_url: string, _init?: RequestInit) => new Response(JSON.stringify(data()), { headers: { "Content-Type": "application/json" } })); vi.stubGlobal("fetch", fetcher);
    const client = new ApiClient("http://localhost");
    await client.getMaterialInitialization("project", { workspaceId: "ws" });
    const command = { action: "check", data: { request_id: "same", expected_version: 4, expected_settings_version: 2 } } as const;
    await client.sendMaterialInitializationCommand("project", command, { workspaceId: "ws" });
    expect(fetcher.mock.calls.at(-1)?.[0]).toBe("http://localhost/api/projects/project/materials/initialization/check");
  });
});
