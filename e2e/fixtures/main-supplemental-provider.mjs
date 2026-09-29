// Task-owned OpenAI-compatible provider for the committed main UI contract.
// No outbound connections or model calls. Bind the existing provider port only
// after its owner stops the earlier fixture, so the API need not be restarted.
import { createServer } from "node:http";
import { setTimeout } from "node:timers/promises";

const stats = { fixture: "main-supplemental-qa", calls: 0, recommendations: 0, descriptions: 0 };
const question = "Who writes and reads the weekly report?";
const refined = "Build a team weekly report feature.";

const server = createServer(async (request, response) => {
  if (request.method === "GET" && request.url === "/health") {
    response.writeHead(200, { "Content-Type": "application/json" });
    response.end(JSON.stringify(stats));
    return;
  }
  if (request.method !== "POST" || !new URL(request.url, "http://localhost").pathname.endsWith("/chat/completions")) {
    response.writeHead(404).end();
    return;
  }
  try {
    const chunks = [];
    let size = 0;
    for await (const chunk of request) {
      size += chunk.length;
      if (size > 256 * 1024) { response.writeHead(413).end(); return; }
      chunks.push(chunk);
    }
    const body = JSON.parse(Buffer.concat(chunks).toString());
    const input = JSON.parse(body.messages?.findLast((message) => message.role === "user")?.content ?? "{}");
    if (typeof input.text !== "string") throw new Error("Unsupported consumer");
    const creator = Array.isArray(input.candidates);
    if (!creator && !["manual", "agent"].includes(input.mode)) throw new Error("Unsupported consumer");
    stats.calls++;
    stats[creator ? "recommendations" : "descriptions"]++;
    if (input.text.includes("CREATORSLOW") || input.text.includes("E2EASSISTDELAY")) await setTimeout(3000);
    if (response.destroyed) return;
    if (input.text.includes("CREATORFAIL") || input.text.includes("E2EASSISTFAIL")) {
      response.writeHead(503, { "Content-Type": "application/json" });
      response.end(JSON.stringify({ error: { message: "Controlled local fixture failure" } }));
      return;
    }
    const content = JSON.stringify(creator ? {
      recommendations: input.candidates.slice(0, 3).map((candidate) => ({
        ref: candidate.ref,
        evidence: Array.from(candidate.description_excerpt).slice(0, 120).join(""),
      })),
    } : { text: refined, questions: input.text.includes("E2EASSISTZERO") ? [] : [question] });
    if (body.stream === true) {
      response.writeHead(200, { "Content-Type": "text/event-stream", "Cache-Control": "no-cache" });
      for (let offset = 0; offset < content.length; offset += 24) {
        if (response.destroyed) return;
        response.write(`data: ${JSON.stringify({ choices: [{ delta: { content: content.slice(offset, offset + 24) }, finish_reason: null }] })}\n\n`);
        await setTimeout(20);
      }
      response.end(`data: ${JSON.stringify({ choices: [{ delta: {}, finish_reason: "stop" }] })}\n\ndata: [DONE]\n\n`);
    } else {
      response.writeHead(200, { "Content-Type": "application/json" });
      response.end(JSON.stringify({ choices: [{ message: { content }, finish_reason: "stop" }] }));
    }
  } catch {
    if (!response.headersSent) response.writeHead(400, { "Content-Type": "application/json" });
    response.end(JSON.stringify({ error: { message: "Unsupported local fixture payload" } }));
  }
});
server.listen(Number(process.env.E2E_ASSIST_PROVIDER_PORT) || 0, "127.0.0.1", () => {
  process.stdout.write(JSON.stringify({ ...stats, pid: process.pid, url: `http://127.0.0.1:${server.address().port}` }) + "\n");
});
