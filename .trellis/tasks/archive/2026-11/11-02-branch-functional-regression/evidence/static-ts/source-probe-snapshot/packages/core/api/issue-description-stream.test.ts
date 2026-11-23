// @vitest-environment node
import { describe, expect, it, vi } from "vitest";
import { readIssueDescriptionStream } from "./issue-description-stream";

const encode = new TextEncoder();
const frame = (event: string, data: unknown) => `event: ${event}\ndata: ${JSON.stringify(data)}\n\n`;
const result = { text: "你好 🌍", questions: [] };
function streamResponse(chunks: Uint8Array[], cancel = vi.fn()) {
  return new Response(new ReadableStream({
    start(controller) { for (const chunk of chunks) controller.enqueue(chunk); controller.close(); },
    cancel,
  }), { headers: { "Content-Type": "text/event-stream" } });
}

describe("issue description event stream", () => {
  it("delivers provisional text before the final event arrives", async () => {
    let source!: ReadableStreamDefaultController<Uint8Array>;
    const response = new Response(new ReadableStream({ start(controller) { source = controller; } }));
    const onText = vi.fn();
    const done = vi.fn();
    const promise = readIssueDescriptionStream(response, { onText }).then(done);
    source.enqueue(encode.encode(frame("text_delta", { text: "你好 " })));
    await vi.waitFor(() => expect(onText).toHaveBeenCalledWith("你好 "));
    expect(done).not.toHaveBeenCalled();
    source.enqueue(encode.encode(frame("text_delta", { text: "🌍" }) + frame("done", result)));
    await promise;
    expect(onText).toHaveBeenLastCalledWith(result.text);
    expect(done).toHaveBeenCalledWith(result);
  });

  it("handles split UTF-8, split CRLF delimiters, multiline data, comments and unknown events", async () => {
    const payload = ': heartbeat\r\n\r\nevent: future\r\ndata: {}\r\n\r\n'
      + frame("text_delta", { text: "你好 🌍" }).replaceAll('\n','\r\n')
      + 'event: done\r\ndata: {"text":"你好 🌍",\r\ndata: "questions":[]}\r\n\r\n';
    const bytes = encode.encode(payload);
    const onText = vi.fn();
    await expect(readIssueDescriptionStream(streamResponse(Array.from(bytes, (b) => new Uint8Array([b]))), { onText }))
      .resolves.toEqual(result);
    expect(onText).toHaveBeenCalledWith(result.text);
  });

  it.each([
    frame("text_delta", { text: 123 }),
    frame("done", { text: " " }),
    'event: done\ndata: {bad-json}\n\n',
    frame("text_delta", { text: "unfinished" }),
    '',
  ])("rejects malformed or unterminated streams %#", async (payload) => {
    await expect(readIssueDescriptionStream(streamResponse([encode.encode(payload)])))
      .rejects.toMatchObject({ code: "ai_invalid_output" });
  });

  it("rejects an error event after partial text and never adopts it", async () => {
    const onText = vi.fn();
    await expect(readIssueDescriptionStream(streamResponse([encode.encode(
      frame("text_delta", { text: "partial" }) + frame("error", { code: "ai_timeout", error: "safe message" }),
    )]), { onText })).rejects.toMatchObject({ code: "ai_timeout" });
    expect(onText).toHaveBeenCalledWith("partial");
  });

  it("cancels the reader after done instead of waiting for the upstream to close", async () => {
    const cancel = vi.fn();
    const response = new Response(new ReadableStream({
      start(controller) { controller.enqueue(encode.encode(frame("done", result))); }, cancel,
    }));
    await expect(readIssueDescriptionStream(response)).resolves.toEqual(result);
    expect(cancel).toHaveBeenCalled();
  });

  it("aborts a pending read and closes the body", async () => {
    const cancel = vi.fn();
    const response = new Response(new ReadableStream({ cancel }));
    const controller = new AbortController();
    const promise = readIssueDescriptionStream(response, { signal: controller.signal });
    controller.abort();
    await expect(promise).rejects.toMatchObject({ name: "AbortError" });
    expect(cancel).toHaveBeenCalled();
  });

  it("does not accept done from the same network chunk after cancellation", async () => {
    const controller = new AbortController();
    await expect(readIssueDescriptionStream(streamResponse([encode.encode(
      frame("text_delta", { text: "partial" }) + frame("done", result),
    )]), { signal: controller.signal, onText: () => controller.abort() }))
      .rejects.toMatchObject({ name: "AbortError" });
  });

  it("bounds incomplete event buffers", async () => {
    await expect(readIssueDescriptionStream(streamResponse([encode.encode('data: ' + 'a'.repeat(256 * 1024))])))
      .rejects.toMatchObject({ code: "ai_invalid_output" });
  });
});
