// @vitest-environment node
import { describe, expect, it } from "vitest";
import { KnowledgeProjectionPolicy } from "./knowledge-lease";
import { knowledgeLeaseFixture as projection } from "./knowledge-test-fixtures";

function clock() { let now = 1000; const policy = new KnowledgeProjectionPolicy(() => now); return { policy, advance: (ms: number) => { now += ms; } }; }
describe("finite knowledge authorization projections", () => {
  it("charges response latency to the 30-second monotonic request-start ceiling", () => {
    const f = clock(), read = f.policy.begin("source"); f.advance(12_000);
    expect(f.policy.accept(read, { ...projection, valid_until: "2026-11-01T01:00:00Z" }).deadline).toBe(31_000);
    f.advance(18_000);
    expect(() => f.policy.accept(read, projection)).toThrow(/expired/);
  });
  it("uses a shorter server interval or expiry regardless of the client's wall clock", () => {
    const f = clock(); const read = f.policy.begin("target");
    expect(f.policy.accept(read, { ...projection, next_expiry_at: "2026-11-01T00:00:09Z" }).deadline).toBe(10_000);
    const next = f.policy.begin("other");
    expect(f.policy.accept(next, { ...projection, projection_checked_at: "2050-01-01T00:00:00Z", valid_until: "2050-01-01T00:00:04Z" }).deadline).toBe(5000);
  });
  it("does not renew older pages or replayed first-page authorization", () => {
    const f = clock(); const first = f.policy.accept(f.policy.begin("target"), projection);
    f.advance(20_000);
    expect(f.policy.accept(f.policy.begin("target", first), projection)).toBe(first);
    expect(f.policy.accept(f.policy.begin("target"), projection).deadline).toBe(31_000);
    f.advance(10_000);
    expect(() => f.policy.accept(f.policy.begin("target"), projection)).toThrow(/expired/);
  });
  it("rejects an inconsistent page boundary, late generation and fabricated parent", () => {
    const f = clock(); const first = f.policy.accept(f.policy.begin("source"), projection);
    expect(() => f.policy.begin("source", { ...first })).toThrow(/expired/);
    expect(() => f.policy.accept(f.policy.begin("source", first), { ...projection, valid_until: "2026-11-01T00:00:29Z" })).toThrow(/expired/);
    const read = f.policy.begin("source"); f.policy.invalidate();
    expect(() => f.policy.accept(read, projection)).toThrow(/expired/);
  });
  it("only accepts the latest root read and a current server check", () => {
    const f = clock(); const old = f.policy.begin("source"); const fresh = f.policy.begin("source");
    expect(() => f.policy.accept(old, projection)).toThrow(/expired/);
    f.policy.accept(fresh, { ...projection, projection_checked_at: "2026-11-01T00:00:01Z" });
    expect(() => f.policy.accept(f.policy.begin("source"), projection)).toThrow(/expired/);
  });
  it("rejects already expired, invalid and backwards-clock responses", () => {
    const f = clock();
    for (const fields of [{ next_expiry_at: "2026-10-31T23:00:00Z" }, { valid_until: "invalid" }, { projection_checked_at: "invalid" }]) {
      expect(() => f.policy.accept(f.policy.begin("source"), { ...projection, ...fields })).toThrow(/expired/);
    }
    const read = f.policy.begin("source"); f.advance(-100);
    expect(() => f.policy.accept(read, projection)).toThrow(/expired/);
  });
});
