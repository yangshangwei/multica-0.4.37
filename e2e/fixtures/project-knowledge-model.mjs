import assert from "node:assert/strict";
import { appendFileSync } from "node:fs";
import { createServer } from "node:http";

// A local deterministic gateway, never an account/model proxy. Requests are
// retained only in the task's private external directory.
const port = Number(process.env.KNOWLEDGE_MODEL_PORT);
const log = process.env.KNOWLEDGE_MODEL_LOG;
assert(Number.isInteger(port) && port > 1024 && log?.startsWith("/"));
const calls = [];
const held = new Set();
const waiters = new Map();
const generated = new Set();
const json = (response, value) => { response.writeHead(200, { "Content-Type": "application/json" }); response.end(JSON.stringify(value)); };
createServer(async (request, response) => {
  try {
    const chunks = [];
    for await (const chunk of request) chunks.push(chunk);
    const raw = Buffer.concat(chunks).toString("utf8");
    if (request.url === "/control/state") return json(response, { calls, held: [...held] });
    if (request.url === "/control/hold" || request.url === "/control/release") {
      const { marker } = JSON.parse(raw);
      assert(typeof marker === "string" && marker.startsWith("P6-"));
      if (request.url.endsWith("hold")) held.add(marker);
      else { held.delete(marker); for (const release of waiters.get(marker) ?? []) release(); waiters.delete(marker); }
      return json(response, { ok: true });
    }
    assert(request.url === "/v1/chat/completions" && request.method === "POST");
    const wire = JSON.parse(raw);
    const input = JSON.parse(wire.messages.findLast((message) => message.role === "user").content);
    const marker = input.sources[0].result.match(/P6-[a-z0-9-]+/)?.[0];
    assert(marker, "Only task-created source text may enter this gateway");
    const stage = input.mode ? "generating" : "checking";
    const call = { number: calls.length + 1, marker, stage, input, model: wire.model, received_at: new Date().toISOString() };
    calls.push(call);
    appendFileSync(log, `${JSON.stringify({ event: "request", ...call })}\n`, { mode: 0o600 });
    if (held.has(marker)) await new Promise((resolve) => waiters.set(marker, [...(waiters.get(marker) ?? []), resolve]));
    let result;
    if (stage === "generating") {
      const text = input.sources[0].result;
      if (input.mode === "review") {
        result = { candidates: input.predecessors.map((previous) => ({ ref: previous.ref, summary: previous.summary,
          statements: [{ text: previous.content, source_ref: input.sources[0].ref, field: "result", quote: previous.content }] })) };
      } else if (generated.has(marker)) result = { candidates: [] };
      else {
        generated.add(marker);
        result = { candidates: [{ ref: "k1", summary: `Accepted retry rule ${marker}`,
          statements: [{ text, source_ref: input.sources[0].ref, field: "result", quote: text }] }] };
      }
    } else result = { checks: input.candidates.map((candidate) => {
      const predecessor = input.comparisons.find((entry) => entry.predecessor_for === candidate.ref);
      return { ref: candidate.ref, support: "supported", reuse: "reusable", relation: predecessor ? "duplicate" : "novel",
        related_refs: predecessor ? [predecessor.ref] : [], reason: "Controlled exact-source fixture check." };
    }) };
    const completion = { id: `p6-${call.number}`, object: "chat.completion", model: "p6-local-controlled-v1",
      choices: [{ index: 0, message: { role: "assistant", content: JSON.stringify(result) }, finish_reason: "stop" }],
      usage: { prompt_tokens: 120, completion_tokens: 40, total_tokens: 160 } };
    call.response = result;
    call.completed_at = new Date().toISOString();
    appendFileSync(log, `${JSON.stringify({ event: "response", number: call.number, completion })}\n`, { mode: 0o600 });
    json(response, completion);
  } catch (error) {
    appendFileSync(log, `${JSON.stringify({ event: "fixture_error", message: error.message })}\n`, { mode: 0o600 });
    response.writeHead(500); response.end("Controlled gateway rejected the request");
  }
}).listen(port, "127.0.0.1");
