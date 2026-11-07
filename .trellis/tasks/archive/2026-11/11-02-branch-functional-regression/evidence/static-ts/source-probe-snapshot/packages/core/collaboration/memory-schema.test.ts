// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";
import { setSchemaLogger } from "../api/schema";
import { noopLogger } from "../logger";
import {
  COLLABORATION_MEMORY_HARD_MAX_BYTES,
  collaborationMemoryContentValid,
  compareCollaborationMemoryVersions,
  nextCollaborationMemoryVersion,
  parseCollaborationMemory,
  parseCollaborationMemoryReceipt,
  parseCollaborationMemoryVersions,
} from "./memory-schema";

const id = (tail: number) => `00000000-0000-4000-8000-${String(tail).padStart(12, "0")}`;
const member = { user_id: id(1), name: "Writer", avatar_url: "", external: true };
const author = { kind: "member", member, agent: null, resource_owner: null };
const current = {
  issue_id: id(2), version: "1", content: "Decision", bytes: 8, max_bytes: 131072,
  author, created_at: "2026-10-26T00:00:00Z", restored_from_version: null, can_edit: true, can_purge: false,
};

afterEach(() => setSchemaLogger(noopLogger));

describe("shared-context response boundaries", () => {
  it("accepts the empty projection and a signed current version", () => {
    expect(parseCollaborationMemory({ issue_id: id(2), version: "0", content: "", bytes: 0, max_bytes: 131072,
      author: null, created_at: null, restored_from_version: null, can_edit: true, can_purge: true })).toMatchObject({ version: "0" });
    expect(parseCollaborationMemory(current)).toMatchObject({ version: "1", content: "Decision", author: { member } });
    expect(nextCollaborationMemoryVersion("9007199254740993")).toBe("9007199254740994");
    expect(compareCollaborationMemoryVersions("10", "2")).toBe(1);
  });

  it.each([
    { ...current, version: "01" },
    { ...current, bytes: -1 },
    { ...current, author: null },
    { ...current, content: "x", bytes: 1, max_bytes: 524289 },
    { ...current, created_at: null },
  ])("rejects malformed current projections: %j", (value) => {
    expect(parseCollaborationMemory(value)).toBeNull();
  });

  it("rejects forged history order, cursor and purge metadata", () => {
    const item = { version: "2", bytes: 8, author, created_at: "2026-10-26T00:00:00Z", restored_from_version: null,
      purged: false, purged_at: null, purged_by: null, purged_by_admin: false };
    const page = { issue_id: id(2), current_version: "2", max_bytes: 131072, can_purge: true, items: [item], next_cursor: null };
    expect(parseCollaborationMemoryVersions(page)).not.toBeNull();
    expect(parseCollaborationMemoryVersions({ ...page, items: [item, item] })).toBeNull();
    expect(parseCollaborationMemoryVersions({ ...page, next_cursor: "1" })).toBeNull();
    expect(parseCollaborationMemoryVersions({ ...page, items: [{ ...item, purged: true }] })).toBeNull();
  });

  it("accepts only sealed receipts and never logs malformed private content", () => {
    const receipt = { request_id: id(3), action: "memory_update", status: 200, code: "memory_updated", protocol_version: 1,
      issue_id: id(2), version: "2", current_version: "2", bytes: 8, max_bytes: 131072, event_seq: "4" };
    expect(parseCollaborationMemoryReceipt(receipt)).toMatchObject({ code: "memory_updated", version: "2" });
    expect(parseCollaborationMemoryReceipt({ ...receipt, status: 409, code: "memory_updated", version: null, event_seq: undefined })).toBeNull();
    const warn = vi.fn();
    setSchemaLogger({ ...noopLogger, warn });
    expect(parseCollaborationMemory({ ...current, content: "PRIVATE_MEMORY_RESPONSE" })).not.toBeNull();
    expect(parseCollaborationMemoryReceipt({ ...receipt, code: "PRIVATE_RECEIPT" })).toBeNull();
    expect(JSON.stringify(warn.mock.calls)).not.toContain("PRIVATE_RECEIPT");
  });
});

describe("shared-context content limits", () => {
  it("counts UTF-8 bytes and enforces the hard ceiling", () => {
    expect(collaborationMemoryContentValid("界".repeat(Math.floor(COLLABORATION_MEMORY_HARD_MAX_BYTES / 3)))).toBe(true);
    expect(collaborationMemoryContentValid("界".repeat(Math.floor(COLLABORATION_MEMORY_HARD_MAX_BYTES / 3) + 1))).toBe(false);
    expect(collaborationMemoryContentValid("has\0nul")).toBe(false);
  });
});
