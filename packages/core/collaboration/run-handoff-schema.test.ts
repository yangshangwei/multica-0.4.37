// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { ApiClient } from "../api/client";
import { parseCollaborationRunHandoff } from "./run-handoff-schema";

const id = "00000000-0000-4000-8000-000000000001";
const other = "00000000-0000-4000-8000-000000000002";
const page = { total: 0, returned: 0, truncated: false, items: [] };
const handoff = { protocol_version: 1, issue_id: id, run_id: other, mode: "live", event_seq: "9007199254740993",
  authority: { status: "in_progress", handler_id: id }, assignment: { role: "agent", step_key: "agent" }, handoff: null, information_request: null,
  memory: { version: "0", availability: "absent" }, inputs: page, results: page, wakeup_causes: { ...page, completeness: "unknown" }, sources: [], missing: [] };
afterEach(() => vi.unstubAllGlobals());

const materialLimit = 256 * 1024;
const wireLimit = 2 * 1024 * 1024;
const source = { kind: "delegation", id, run_id: other, sha256: "a".repeat(64), content: "", truncated: false, availability: "available", trust: "admitted_assignment" };
const goJSON = (value: unknown) => JSON.stringify(value).replace(/[<>&\u2028\u2029]/g, (char) => `\\u${char.charCodeAt(0).toString(16).padStart(4, "0")}`);

it.each(["<", "&", "\u0001", "界", "😀"])("accepts a legal 64 KiB source containing %s without charging its JSON escape expansion", (character) => {
  const content = character.repeat(Math.floor(65536 / Buffer.byteLength(character)));
  const value = { ...handoff, sources: [{ ...source, content }] };
  expect(parseCollaborationRunHandoff(JSON.parse(goJSON(value)))).toEqual(value);
});

it.each(["<", "&", "\u0001", "界", "😀"])("charges decoded source bytes and all framing/metadata at the material boundary (%s)", (character) => {
  // Match the server's omitted optional fields. HTML/line separators in
  // metadata still pay their complete Go JSON cost, unlike source content.
  const value = { ...handoff, assignment: { role: "agent", step_key: "<&\u2028\u2029" }, sources: [{ ...source }] };
  const remaining = materialLimit - Buffer.byteLength(goJSON(value)) - 1;
  value.sources[0]!.content = character.repeat(Math.floor(remaining / Buffer.byteLength(character))) + "x".repeat(remaining % Buffer.byteLength(character));
  expect(parseCollaborationRunHandoff(JSON.parse(goJSON(value)))).toEqual(value);
  value.sources[0]!.content += "x";
  expect(parseCollaborationRunHandoff(JSON.parse(goJSON(value)))).toBeNull();
});

it("counts additional response metadata before schema normalization", () => {
  expect(parseCollaborationRunHandoff({ ...handoff, future_metadata: "m".repeat(materialLimit) })).toBeNull();
});

it("bounds the actual HTTP JSON bytes including whitespace and the terminating newline", async () => {
  const body = goJSON(handoff);
  const atLimit = body + " ".repeat(wireLimit - Buffer.byteLength(body) - 1) + "\n";
  vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(new Response(atLimit))
    .mockResolvedValueOnce(new Response(atLimit + "\n")));
  const api = new ApiClient("https://handoff.example");
  expect(await api.getCollaborationRunHandoff(id, other)).toEqual(handoff);
  expect(await api.getCollaborationRunHandoff(id, other)).toBeNull();
});

it("preserves absence and exact handoff identity while refusing malformed declared support", () => {
  expect(parseCollaborationRunHandoff(handoff)).toEqual(handoff);
  expect(parseCollaborationRunHandoff(undefined)).toBeNull();
  for (const patch of [{ protocol_version: 2 }, { mode: "snapshot" }, { authority: {} }, { results: {} },
    { results: { ...page, returned: 1 } }, { wakeup_causes: { ...page, completeness: "complete" } }, { missing: undefined }]) {
    expect(parseCollaborationRunHandoff({ ...handoff, ...patch })).toBeNull();
  }
  const source = { kind: "provider_result", id, run_id: other, sha256: "a".repeat(64), content: "Provider output", truncated: false, availability: "available", trust: "data_only" };
  expect(parseCollaborationRunHandoff({ ...handoff, sources: [source] })?.sources[0]?.trust).toBe("data_only");
  expect(parseCollaborationRunHandoff({ ...handoff, sources: [{ ...source, trust: "admitted_assignment" }] })).toBeNull();
  expect(parseCollaborationRunHandoff({ ...handoff, sources: [{ ...source, availability: "purged" }] })).toBeNull();
});

it("keeps a live read's launch base apart from its latest memory and refuses a malformed base", () => {
  const memory = { version: "4", sha256: "a".repeat(64), availability: "available", base_version: "3" };
  expect(parseCollaborationRunHandoff({ ...handoff, memory })?.memory).toEqual(memory);
  expect(parseCollaborationRunHandoff({ ...handoff, memory: { ...memory, base_version: undefined } })?.memory.base_version).toBeUndefined();
  for (const base_version of ["03", "-1", 3, "", null]) {
    expect(parseCollaborationRunHandoff({ ...handoff, memory: { ...memory, base_version } })).toBeNull();
  }
});

it("uses the human handoff endpoint and rejects a response for a different run", async () => {
  const fetch = vi.fn().mockResolvedValueOnce(new Response(JSON.stringify(handoff)))
    .mockResolvedValueOnce(new Response(JSON.stringify({ ...handoff, run_id: id })))
    .mockResolvedValueOnce(new Response("not found", { status: 404 }));
  vi.stubGlobal("fetch", fetch);
  const api = new ApiClient("https://handoff.example");
  expect(await api.getCollaborationRunHandoff(id, other)).toEqual(handoff);
  expect(fetch.mock.calls[0]?.[0]).toContain(`/api/collaboration/issues/${id}/runs/${other}/handoff`);
  expect(await api.getCollaborationRunHandoff(id, other)).toBeNull();
  expect(await api.getCollaborationRunHandoff(id, other)).toBeNull();
  expect(fetch.mock.calls.every(([url]) => !url.includes("/runtime/"))).toBe(true);
});
