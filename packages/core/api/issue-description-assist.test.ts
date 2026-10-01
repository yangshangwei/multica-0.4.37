// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiClient } from "./client";
import { setSchemaLogger } from "./schema";
import { noopLogger } from "../logger";
import { setCurrentWorkspace } from "../platform/workspace-storage";

function respond(body: unknown, status = 200) {
  const request = vi.fn<typeof fetch>().mockResolvedValue(new Response(JSON.stringify(body), {
    status, headers: { "Content-Type": "application/json" },
  }));
  vi.stubGlobal("fetch", request);
  return request;
}
const input = { text: "Preserve phone login", mode: "manual" as const };
afterEach(() => {
  vi.unstubAllGlobals();
  setSchemaLogger(noopLogger);
  setCurrentWorkspace(null, null);
});

describe("issue description optimization API", () => {
  it("pins workspace and propagates cancellation without serializing the signal", async () => {
    setCurrentWorkspace("other", "ws-other");
    const result = { text: "Keep phone login available.", questions: ["Support account linking?"] };
    const request = respond(result);
    const controller = new AbortController();
    await expect(new ApiClient("https://api.test").optimizeIssueDescription(input, {
      workspaceId: "ws-1", signal: controller.signal,
    })).resolves.toEqual(result);
    expect(request).toHaveBeenCalledWith("https://api.test/api/issues/optimize-description", expect.objectContaining({
      method: "POST", body: JSON.stringify(input), signal: expect.objectContaining({ aborted: false }),
      headers: expect.objectContaining({ "X-Workspace-ID": "ws-1", "X-Workspace-Slug": "" }),
    }));
    controller.abort("request cancelled");
    expect(request.mock.calls[0]?.[1]?.signal?.aborted).toBe(true);
    expect(request.mock.calls[0]?.[1]?.signal?.reason).toBe("request cancelled");
  });

  it.each([null, {}, { text: "  ", questions: [] }, { text: 42 }, { text: "Keep", questions: "bad" }])(
    "rejects malformed suggestions instead of replacing a draft %#", async (body) => {
      respond(body);
      await expect(new ApiClient("https://api.test").optimizeIssueDescription(input, { workspaceId: "ws-1" })).rejects.toThrow();
    },
  );

  it("normalizes absent questions and tolerates future response fields", async () => {
    respond({ text: "Keep phone login.", extra: true });
    await expect(new ApiClient("https://api.test").optimizeIssueDescription(input, { workspaceId: "ws-1" }))
      .resolves.toEqual({ text: "Keep phone login.", questions: [] });
  });

  it("does not log private model output when its shape is invalid", async () => {
    const warn = vi.fn();
    setSchemaLogger({ ...noopLogger, warn });
    respond({ text: "private-description", questions: "private-question" });
    await expect(new ApiClient("https://api.test").optimizeIssueDescription(input, { workspaceId: "ws-1" })).rejects.toThrow();
    expect(warn).toHaveBeenCalled();
    expect(JSON.stringify(warn.mock.calls)).not.toContain("private-");
  });

  it("preserves actionable unavailable errors", async () => {
    respond({ error: "Unavailable", code: "ai_unavailable" }, 503);
    await expect(new ApiClient("https://api.test").optimizeIssueDescription(input, { workspaceId: "ws-1" }))
      .rejects.toMatchObject({ status: 503, body: { code: "ai_unavailable" } });
  });

  it("requests SSE and exposes text chunks before returning the validated result", async () => {
    const result = { text: "Keep phone login.", questions: [] };
    const onText = vi.fn();
    const request = vi.fn().mockResolvedValue(new Response(
      'event: text_delta\ndata: {"text":"Keep "}\n\n'
      + 'event: text_delta\ndata: {"text":"phone login."}\n\n'
      + `event: done\ndata: ${JSON.stringify(result)}\n\n`,
      { headers: { "Content-Type": "text/event-stream; charset=utf-8" } },
    ));
    vi.stubGlobal("fetch", request);
    await expect(new ApiClient("https://api.test").optimizeIssueDescription(input, { workspaceId: "ws-1", onText })).resolves.toEqual(result);
    expect(request).toHaveBeenCalledWith(expect.any(String), expect.objectContaining({
      headers: expect.objectContaining({ Accept: "text/event-stream", "Content-Type": "application/json" }),
      body: JSON.stringify(input),
    }));
    expect(onText.mock.calls).toEqual([["Keep "], ["Keep phone login."]]);
  });

  it("retains a sanitized machine-readable stream failure", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(
      'event: error\ndata: {"code":"ai_timeout","error":"private upstream details"}\n\n',
      { headers: { "Content-Type": "text/event-stream" } },
    )));
    await expect(new ApiClient("https://api.test").optimizeIssueDescription(input, { workspaceId: "ws-1" }))
      .rejects.toMatchObject({ status: 502, body: { code: "ai_timeout" }, message: "AI optimization stream failed" });
  });

  it("rejects aborted requests", async () => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new DOMException("Aborted", "AbortError")));
    const controller = new AbortController();
    controller.abort();
    await expect(new ApiClient("https://api.test").optimizeIssueDescription(input, {
      workspaceId: "ws-1", signal: controller.signal,
    })).rejects.toMatchObject({ name: "AbortError" });
  });
});
