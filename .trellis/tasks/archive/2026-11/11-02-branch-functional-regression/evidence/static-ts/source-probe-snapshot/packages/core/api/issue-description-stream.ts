import { z } from "zod";
import type { OptimizeIssueDescriptionResponse } from "../types";
import { parseWithFallback } from "./schema";
import { OptimizeIssueDescriptionResponseSchema } from "./schemas";

const EventSchema = z.discriminatedUnion("event", [
  z.object({ event: z.literal("text_delta"), data: z.object({ text: z.string() }) }),
  z.object({ event: z.literal("done"), data: OptimizeIssueDescriptionResponseSchema }),
  z.object({ event: z.literal("error"), data: z.object({ code: z.string(), error: z.string() }) }),
]);

export class IssueDescriptionStreamError extends Error {
  constructor(readonly code = "ai_invalid_output") {
    // Neither model output nor server-supplied messages belong in diagnostics.
    super("AI optimization stream failed");
    this.name = "IssueDescriptionStreamError";
  }
}

/** Decode SSE independently of network chunk boundaries. Only done is adoptable. */
export async function readIssueDescriptionStream(
  response: Response,
  options: { onText?: (text: string) => void; signal?: AbortSignal } = {},
): Promise<OptimizeIssueDescriptionResponse> {
  if (!response.body) throw new IssueDescriptionStreamError();
  const reader = response.body.getReader();
  const decoder = new TextDecoder("utf-8", { fatal: true });
  const cancel = () => { void reader.cancel().catch(() => {}); };
  const checkAbort = () => {
    if (options.signal?.aborted) throw new DOMException("Aborted", "AbortError");
  };
  options.signal?.addEventListener("abort", cancel, { once: true });
  let buffer = "";
  let event = "";
  let dataLines: string[] = [];
  let eventSize = 0;
  let totalBytes = 0;
  let text = "";

  const dispatch = (): OptimizeIssueDescriptionResponse | undefined => {
    checkAbort();
    if (!dataLines.length || !["text_delta", "done", "error"].includes(event)) return;
    let data: unknown;
    try { data = JSON.parse(dataLines.join("\n")); }
    catch { throw new IssueDescriptionStreamError(); }
    const parsed = parseWithFallback<z.infer<typeof EventSchema> | null>(
      { event, data }, EventSchema, null,
      { endpoint: "POST /api/issues/optimize-description stream", redact: true },
    );
    if (!parsed) throw new IssueDescriptionStreamError();
    switch (parsed.event) {
      case "text_delta":
        text += parsed.data.text;
        if (text.length > 120000) throw new IssueDescriptionStreamError();
        checkAbort();
        options.onText?.(text);
        return;
      case "error":
        throw new IssueDescriptionStreamError(parsed.data.code);
      case "done":
        return parsed.data;
    }
  };

  try {
    for (;;) {
      checkAbort();
      const chunk = await reader.read();
      checkAbort();
      if (chunk.done) throw new IssueDescriptionStreamError();
      totalBytes += chunk.value.byteLength;
      if (totalBytes > 2 * 1024 * 1024) throw new IssueDescriptionStreamError();
      buffer += decoder.decode(chunk.value, { stream: true });
      let newline: number;
      while ((newline = buffer.indexOf("\n")) >= 0) {
        const line = buffer.slice(0, newline).replace(/\r$/, "");
        buffer = buffer.slice(newline + 1);
        eventSize += line.length;
        if (eventSize > 256 * 1024) throw new IssueDescriptionStreamError();
        if (!line) {
          const result = dispatch();
          if (result) return result;
          event = "";
          dataLines = [];
          eventSize = 0;
        } else if (line.startsWith("event:")) {
          event = line.slice(6).replace(/^ /, "");
        } else if (line.startsWith("data:")) {
          dataLines.push(line.slice(5).replace(/^ /, ""));
        }
      }
      if (buffer.length + eventSize > 256 * 1024) throw new IssueDescriptionStreamError();
    }
  } catch (error) {
    checkAbort();
    if (error instanceof IssueDescriptionStreamError) throw error;
    throw new IssueDescriptionStreamError("ai_generation_failed");
  } finally {
    options.signal?.removeEventListener("abort", cancel);
    cancel();
    reader.releaseLock();
  }
}
