// Deterministic OpenAI-compatible fixture for issue-assist-flow.spec.ts.
// See README.md for isolated backend configuration and test invocation.
import http from "node:http";

const records = [];
const wait = (milliseconds) => new Promise((resolve) => setTimeout(resolve, milliseconds));
const firstQuestion = "Who writes and reads the weekly report?";
const secondQuestion = "Should reports be written manually or generated from task progress?";

const server = http.createServer(async (request, response) => {
  if (request.url === "/health") {
    response.end("ok");
    return;
  }
  if (request.url === "/requests") {
    response.setHeader("content-type", "application/json");
    response.end(JSON.stringify(records));
    return;
  }

  let raw = "";
  for await (const chunk of request) raw += chunk;
  let body;
  try {
    body = JSON.parse(raw);
  } catch {
    response.writeHead(400).end();
    return;
  }

  const message = body.messages?.findLast((entry) => entry.role === "user");
  let input;
  try {
    input = JSON.parse(message?.content ?? "{}");
  } catch {
    input = { text: message?.content ?? "" };
  }
  // Tiptap escapes underscores when serializing the scenario markers.
  const source = (input.text ?? "").replaceAll("\\_", "_");
  records.push({ mode: input.mode, text: source, stream: body.stream });
  const questions = source.includes("E2E_ASSIST_ZERO")
    ? []
    : source.includes("E2E_ASSIST_ONE") ? [firstQuestion] : [firstQuestion, secondQuestion];
  const output = source.includes("E2E_ASSIST_FAIL")
    ? "{invalid-json"
    : JSON.stringify({ text: "Build a team weekly report feature.", questions });
  if (source.includes("E2E_ASSIST_DELAY")) await wait(3000);

  if (body.stream) {
    response.writeHead(200, {
      "content-type": "text/event-stream",
      "cache-control": "no-cache",
    });
    for (let offset = 0; offset < output.length; offset += 30) {
      if (response.destroyed) return;
      const chunk = { choices: [{ delta: { content: output.slice(offset, offset + 30) }, finish_reason: null }] };
      response.write(`data: ${JSON.stringify(chunk)}\n\n`);
      await wait(15);
    }
    const terminal = { choices: [{ delta: {}, finish_reason: "stop" }] };
    response.end(`data: ${JSON.stringify(terminal)}\n\ndata: [DONE]\n\n`);
  } else {
    response.setHeader("content-type", "application/json");
    response.end(JSON.stringify({ choices: [{ message: { content: output }, finish_reason: "stop" }] }));
  }
});

const port = Number(process.env.E2E_ASSIST_PROVIDER_PORT || 14592);
server.listen(port, "127.0.0.1", () => console.log(`Controlled E2E provider: ${port}`));
