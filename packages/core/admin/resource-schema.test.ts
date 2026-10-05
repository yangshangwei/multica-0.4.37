// @vitest-environment node
import { describe, expect, it } from "vitest";
import { parseAdminResourceList, parseAdminResourcePreview, parseAdminResourceResult } from "./resource-schema";

export const resourceFixture = { kind: "skill", key: "team-style", name: "Team style", description: "Style guidance", source: "managed", state: "published", version: "revision-2", content_digest: "content-1", file_count: 1, byte_count: 80, updated_at: null, updated_by: null };
export const limitsFixture = { max_upload_bytes: 16777216, max_primary_bytes: 1048576, max_file_bytes: 1048576, max_supporting_bytes: 8388608, max_total_bytes: 9437184, max_files: 257, max_archive_entries: 512, max_mcp_bytes: 65536, max_managed_entries: 256, max_active_bytes: 67108864, max_index_bytes: 16777216, max_mcp_entries: 256, max_mcp_catalog_bytes: 4194304 };

describe("resource response boundaries", () => {
  it("keeps mutation revisions distinct from content digests", () => {
    expect(parseAdminResourceList({ enabled: true, can_publish: true, items: [resourceFixture], limits: limitsFixture })?.items[0]).toMatchObject({ version: "revision-2", contentDigest: "content-1" });
  });
  it("fails closed on malformed capability or target data", () => {
    for (const raw of [null, {}, { enabled: true, can_publish: "true", items: [], limits: limitsFixture }, { enabled: true, can_publish: true, items: [{ ...resourceFixture, key: "../escape" }], limits: limitsFixture }]) expect(parseAdminResourceList(raw)).toBeNull();
    expect(parseAdminResourceResult({ resource: resourceFixture, replayed: false })).toBeNull();
    expect(parseAdminResourcePreview({ resource: resourceFixture, files: [], preview: "secret", expected_version: null })).toBeNull();
  });
  it("retains unfamiliar states as read-only unknown values", () => {
    expect(parseAdminResourceList({ enabled: false, can_publish: false, items: [{ ...resourceFixture, state: "future", source: "future" }], limits: limitsFixture })?.items[0]).toMatchObject({ state: "unknown", source: "unknown" });
  });
});
