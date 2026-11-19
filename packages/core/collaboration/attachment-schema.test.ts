// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiClient } from "../api";
import { setSchemaLogger } from "../api/schema";
import { noopLogger } from "../logger";
import { parseCollaborationAttachment, parseCollaborationAttachments, parseCollaborationDraftAttachments,
  parseCollaborationAttachmentCapability, parseCollaborationAttachmentDeliveryReceipt } from "./attachment-schema";

const actor = "00000000-0000-4000-8000-000000000001";
const issue = "00000000-0000-4000-8000-000000000002";
const other = "00000000-0000-4000-8000-000000000003";
const file = "00000000-0000-4000-8000-000000000004";
const draft = "00000000-0000-4000-8000-000000000005";
const timestamp = "2026-10-18T00:00:00Z";
const metadata = { id: file, draft_id: draft, issue_id: null, intended_issue_id: issue, source_workspace_id: other, uploader_user_id: actor,
  name: "private.txt", content_type: "text/plain", size_bytes: 10, version: "9007199254740993", created_at: timestamp, delivered_at: null };
const agent = "00000000-0000-4000-8000-0000000000a6";
const run = "00000000-0000-4000-8000-0000000000b7";
const agentMetadata = { ...metadata, id: "00000000-0000-4000-8000-000000000006", draft_id: null,
  issue_id: issue, uploader_user_id: null, uploader_agent_id: agent, run_id: run, attempt_seq: "9007199254740993", delivered_at: timestamp };
afterEach(() => { setSchemaLogger(noopLogger); vi.unstubAllGlobals(); });

describe("human and Agent attachment provenance", () => {
  it("preserves legacy human field absence and explicit nullable output provenance", () => {
    expect(parseCollaborationAttachment(metadata)).toEqual(metadata);
    const human = { ...metadata, issue_id: issue, delivered_at: timestamp,
      uploader_agent_id: null, run_id: null, attempt_seq: null };
    expect(parseCollaborationAttachment(human)).toEqual(human);
  });

  it("retains a delivered Agent file's original Agent, run and exact attempt without inventing a human draft", () => {
    expect(parseCollaborationAttachment(agentMetadata)).toEqual(agentMetadata);
    expect(parseCollaborationAttachment({ ...agentMetadata, uploader_agent_id: agent.toUpperCase(), run_id: run.toUpperCase() })).toEqual(agentMetadata);
    expect(parseCollaborationAttachment({ ...agentMetadata, intended_issue_id: null })).toEqual({ ...agentMetadata, intended_issue_id: null });
  });

  it("keeps human and Agent issue deliveries together while refusing an Agent file in a human draft", () => {
    const human = { ...metadata, issue_id: issue, delivered_at: timestamp };
    const collection = { issue_id: issue, attachments: [human, agentMetadata] };
    expect(parseCollaborationAttachments(collection)).toEqual(collection);
    expect(parseCollaborationDraftAttachments({ draft_id: draft, attachments: [agentMetadata] })).toBeNull();
  });

  it.each([
    { name: "a missing Agent", patch: { uploader_agent_id: undefined } },
    { name: "a null Agent", patch: { uploader_agent_id: null } },
    { name: "a malformed Agent", patch: { uploader_agent_id: "not-an-agent" } },
    { name: "a zero Agent", patch: { uploader_agent_id: "00000000-0000-0000-0000-000000000000" } },
    { name: "a missing run", patch: { run_id: undefined } },
    { name: "a null run", patch: { run_id: null } },
    { name: "a malformed run", patch: { run_id: "not-a-run" } },
    { name: "a zero run", patch: { run_id: "00000000-0000-0000-0000-000000000000" } },
    { name: "a human uploader", patch: { uploader_user_id: actor } },
    { name: "a human draft", patch: { draft_id: draft } },
    { name: "no delivered issue", patch: { issue_id: null, delivered_at: null } },
    { name: "no delivery time", patch: { delivered_at: null } },
  ])("rejects an Agent output with $name", ({ patch }) => {
    expect(parseCollaborationAttachment({ ...agentMetadata, ...patch })).toBeNull();
  });

  it.each([
    { name: "missing", value: undefined }, { name: "null", value: null }, { name: "numeric", value: 1 },
    { name: "zero", value: "0" }, { name: "leading-zero", value: "01" }, { name: "fractional", value: "1.0" },
    { name: "exponential", value: "1e3" }, { name: "out-of-range", value: "9223372036854775808" },
  ])("rejects a $name Agent attempt sequence", ({ value }) => {
    expect(parseCollaborationAttachment({ ...agentMetadata, attempt_seq: value })).toBeNull();
  });

  it("rejects mixed provenance and half-present human identities", () => {
    for (const patch of [
      { draft_id: null }, { uploader_user_id: null }, { draft_id: null, uploader_user_id: null },
      { uploader_agent_id: agent }, { run_id: run }, { attempt_seq: "1" },
      { uploader_agent_id: agent, run_id: run, attempt_seq: "1" },
    ]) {
      expect(parseCollaborationAttachment({ ...metadata, issue_id: issue, delivered_at: timestamp, ...patch })).toBeNull();
    }
  });
});

it("keeps intended issue and immutable versions exact without accepting mismatched delivery", () => {
  expect(parseCollaborationAttachment(metadata)).toEqual(metadata);
  const delivered = { ...metadata, issue_id: issue, delivered_at: timestamp };
  expect(parseCollaborationAttachment(delivered)).toEqual(delivered);
  for (const patch of [{ intended_issue_id: undefined }, { version: 1 }, { version: "0" }, { version: "1e3" }, { size_bytes: -1 },
    { size_bytes: 104857601 }, { issue_id: issue }, { delivered_at: timestamp }, { issue_id: other, delivered_at: timestamp }]) {
    expect(parseCollaborationAttachment({ ...metadata, ...patch })).toBeNull();
  }
  expect(parseCollaborationAttachment({ ...delivered, intended_issue_id: null })).not.toBeNull();
});

it("requires complete, same-object, unique attachment collections", () => {
  const delivered = { ...metadata, issue_id: issue, delivered_at: timestamp };
  expect(parseCollaborationAttachments({ issue_id: issue, attachments: [delivered] })?.attachments).toHaveLength(1);
  expect(parseCollaborationAttachments({ issue_id: other, attachments: [delivered] })).toBeNull();
  expect(parseCollaborationAttachments({ issue_id: issue, attachments: [delivered, delivered] })).toBeNull();
  expect(parseCollaborationAttachments({ issue_id: issue, attachments: null })).toBeNull();
  expect(parseCollaborationDraftAttachments({ draft_id: draft, attachments: [metadata] })).not.toBeNull();
  expect(parseCollaborationDraftAttachments({ draft_id: other, attachments: [metadata] })).toBeNull();
});

it("does not retain private URLs in metadata or log malformed file/capability details", () => {
  const warn = vi.fn(); setSchemaLogger({ ...noopLogger, warn });
  expect(parseCollaborationAttachment({ ...metadata, url: "PRIVATE_FILE_SENTINEL", storage_key: "PRIVATE_FILE_SENTINEL" })).toEqual(metadata);
  expect(parseCollaborationAttachment({ ...metadata, version: "PRIVATE_FILE_SENTINEL" })).toBeNull();
  expect(parseCollaborationAttachmentCapability({ issue_id: issue, attachment_id: file, version: 1, url: "PRIVATE_CAP_SENTINEL", expires_at: timestamp }, "https://server.example", issue, file, metadata.version)).toBeNull();
  expect(warn).toHaveBeenCalledTimes(2);
  expect(JSON.stringify(warn.mock.calls)).not.toMatch(/PRIVATE_(FILE|CAP)_SENTINEL/);
});

it("rejects expired, extended and wrong-version native capabilities", () => {
  const now = Date.parse(timestamp);
  const capability = { issue_id: issue, attachment_id: file, version: metadata.version,
    url: `/api/collaboration/issues/${issue}/attachments/${file}/capability?cap=opaque`, expires_at: new Date(now + 30_000).toISOString() };
  expect(parseCollaborationAttachmentCapability(capability, "https://server.example", issue, file, metadata.version, now)).not.toBeNull();
  for (const patch of [{ version: "2" }, { expires_at: timestamp }, { expires_at: new Date(now + 120_000).toISOString() },
    { url: `//outside.example${capability.url}` }, { url: `${capability.url}&filename=other` }]) {
    expect(parseCollaborationAttachmentCapability({ ...capability, ...patch }, "https://server.example", issue, file, metadata.version, now)).toBeNull();
  }
});

it("does not accept an incomplete or duplicate-file delivery receipt", () => {
  const receipt = { status: 200, code: "attachments_delivered", protocol_version: 2, issue_id: issue, attachment_ids: [file], event_id: actor, event_seq: "9007199254740993" };
  expect(parseCollaborationAttachmentDeliveryReceipt(receipt)).toEqual(receipt);
  for (const patch of [{ attachment_ids: [] }, { attachment_ids: [file, file] }, { event_id: null }, { event_seq: 1 }, { status: 202 }]) {
    expect(parseCollaborationAttachmentDeliveryReceipt({ ...receipt, ...patch })).toBeNull();
  }
});

it("refuses a same-source upload response intended for another issue", async () => {
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify({ ...metadata, intended_issue_id: other }))));
  const api = new ApiClient("https://server.example");
  await expect(api.uploadCollaborationAttachment({ issue_id: issue, draft_id: draft }, actor, new File(["0123456789"], "private.txt"))).resolves.toBeNull();
});

it("keeps human draft upload recovery limited to human uploads", async () => {
  vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(new Response(JSON.stringify(agentMetadata)))
    .mockResolvedValueOnce(new Response(JSON.stringify(metadata))));
  const api = new ApiClient("https://server.example");
  await expect(api.getCollaborationAttachmentUpload(draft, actor)).resolves.toBeNull();
  await expect(api.getCollaborationAttachmentUpload(draft, actor)).resolves.toEqual(metadata);
});

it("rejects a multipart response that finishes parsing after credentials change", async () => {
  let complete!: (value: unknown) => void;
  const response = new Response("{}");
  response.json = () => new Promise((resolve) => { complete = resolve; });
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(response));
  const api = new ApiClient("https://server.example");
  api.setToken("first-test-credential");
  const pending = api.uploadCollaborationAttachment({ issue_id: issue, draft_id: draft }, actor, new File(["0123456789"], "private.txt"));
  await vi.waitFor(() => expect(complete).toBeTypeOf("function"));
  api.setToken("second-test-credential");
  complete(metadata);
  await expect(pending).rejects.toMatchObject({ name: "AbortError" });
});
