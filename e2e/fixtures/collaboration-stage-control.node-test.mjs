import assert from "node:assert/strict";
import { Buffer } from "node:buffer";
import { once } from "node:events";
import { chmodSync, mkdtempSync, readFileSync, realpathSync, rmSync, writeFileSync } from "node:fs";
import { createServer } from "node:net";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import process from "node:process";
import { fileURLToPath, pathToFileURL } from "node:url";
import test from "node:test";
import ts from "typescript";

const sourcePath = join(dirname(fileURLToPath(import.meta.url)), "collaboration-stage-control.ts");
const moduleRoot = mkdtempSync(join(tmpdir(), "stage-control-module-"));
chmodSync(moduleRoot, 0o700);
const modulePath = join(moduleRoot, "control.mjs");
const output = ts.transpileModule(readFileSync(sourcePath, "utf8"), {
  compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext },
});
writeFileSync(modulePath, output.outputText, { mode: 0o600 });
const { StageExecutionControl, encodeStageControlRequest, parseStageControlReply } = await import(pathToFileURL(modulePath).href);
process.on("exit", () => rmSync(moduleRoot, { recursive: true, force: true }));

const id = "11111111-1111-4111-8111-111111111111";
const issue = "22222222-2222-4222-8222-222222222222";
const approval = "33333333-3333-4333-8333-333333333333";
const candidate = "44444444-4444-4444-8444-444444444444";
const intent = "55555555-5555-4555-8555-555555555555";
const digest = "a".repeat(64);
const request = { request_id: id, action: "execution.prepare", issue_id: issue, approval_decision_id: approval,
  candidate: { candidate_id: candidate, version: "1", content_digest: digest }, target_digest: digest };
const receipt = { version: 1, issue_id: issue, request_id: id, intent_id: intent, approval_decision_id: approval,
  kind: "deployment", state: "unknown", fact_id: null };
const reply = { request_id: id, ok: true, result: receipt, calls: { count: 1, last: { intent_id: intent,
  kind: "deployment", target_digest: digest, artifact_digest: digest, external_request_key: id, conditional_version: "1" } } };

// Retained R22 reply data is parsed offline. Socket tests below use a fresh
// synthetic request identity and never contact or replay the real harness.
const observedRollbackReply = {
  request_id: "61b4733a-f70e-424c-8f4b-c337ea9c3f52", ok: true,
  result: { version: 1, issue_id: "01a1e574-a1ac-74de-89bd-9a58186b56ad",
    request_id: "61b4733a-f70e-424c-8f4b-c337ea9c3f52", intent_id: "01a1e575-5feb-7382-a862-81b0f0793478",
    approval_decision_id: "01a1e575-5cf4-71a5-9607-64b9d8f0b9ab", kind: "rollback", state: "succeeded",
    fact_id: "01a1e575-600d-7ccd-800c-cd8ec86aa8ef", observation_id: "01a1e575-600d-7ccd-800c-cd8ec86aa8ef" },
  calls: { count: 2, last: { intent_id: "01a1e575-5feb-7382-a862-81b0f0793478", kind: "rollback",
    target_digest: "69f136894a3bb3a99dbbba3f55a2644c13f8eff1a73b580fea107e66f229b5f3",
    artifact_digest: "ee8e03a8180cc18648b74891cf7ebab384debe3e542b2f9b89d54928cdf0ea4f",
    external_request_key: "01a1e575-5feb-7382-a862-81b0f0793478",
    conditional_version: "1/after/01a1e575-3092-7cb9-bfa2-8a28d87745ff" } },
};
const opaqueVersion = observedRollbackReply.calls.last.conditional_version;

test("private control accepts a frozen original request and bounded native operation IDs", () => {
  assert.equal(encodeStageControlRequest(request), `${JSON.stringify(request)}\n`);
  const native = { request_id: id, action: "source.behavior", behavior: "succeeded", external_operation_id: "deploy/provider:作业-1" };
  assert.equal(JSON.parse(encodeStageControlRequest(native)).external_operation_id, native.external_operation_id);
  assert.deepEqual(parseStageControlReply(JSON.stringify(reply), id), reply);
});

test("private control rejects unknown actions/fields, duplicate response keys and contradictory replies", () => {
  for (const value of [{ ...request, action: "shutdown" }, { ...request, verified: true },
    { ...request, candidate: { ...request.candidate, version: "01" } },
    { request_id: id, action: "source.behavior", behavior: "succeeded", external_operation_id: "界".repeat(86) }]) {
    assert.throws(() => encodeStageControlRequest(value));
  }
  assert.throws(() => parseStageControlReply(JSON.stringify(reply), issue));
  assert.throws(() => parseStageControlReply(JSON.stringify({ ...reply, code: "conflict" }), id));
  assert.throws(() => parseStageControlReply(JSON.stringify({ ...reply, calls: { count: -1, last: null } }), id));
  assert.throws(() => parseStageControlReply(`{"request_id":"${id}","request_id":"${id}","ok":false,"code":"x","calls":{"count":0,"last":null}}`, id));
  assert.throws(() => parseStageControlReply(JSON.stringify({ ...reply, result: { ...receipt, secret: "x".repeat(65536) } }), id));
});

test("a succeeded rollback reply preserves its opaque conditional token and bounded UTF-8 state", () => {
  const parsed = parseStageControlReply(JSON.stringify(observedRollbackReply), observedRollbackReply.request_id);
  assert.deepEqual(parsed, observedRollbackReply);
  assert.equal(parsed.result.state, "succeeded");
  assert.equal(parsed.calls.count, 2);
  assert.equal(parsed.calls.last.conditional_version, opaqueVersion);
  for (const value of ["0", "01", "provider/etag:v2", "x".repeat(256), "界".repeat(85) + "a"]) {
    const bounded = { ...reply, calls: { count: 1, last: { ...reply.calls.last, conditional_version: value } } };
    assert.equal(parseStageControlReply(JSON.stringify(bounded), id).calls.last.conditional_version, value);
  }
});

test("malformed conditional tokens cannot expose a confirmed stage-control reply", () => {
  for (const value of ["", null, undefined, 1, false, {}, [], "a\0b", "\ud800", "\udfff", "x".repeat(257), "界".repeat(86)]) {
    const malformed = { ...reply, calls: { count: 1, last: { ...reply.calls.last, conditional_version: value } } };
    assert.throws(() => parseStageControlReply(JSON.stringify(malformed), id));
  }
});

test("opaque reply tokens do not loosen request candidate, native or initial configuration versions", () => {
  const observe = { request_id: id, action: "execution.observe", intent_id: intent,
    native_id: "synthetic-observation", native_version: "1", result: "unknown" };
  const configure = { request_id: id, action: "source.configure", workspace_id: issue,
    deployment_integration_id: approval, metric_integration_id: candidate, environment: "synthetic",
    remote_instance: "fixture", remote_tenant: "fixture", remote_target: "fixture", conditional_version: "1",
    current_artifact_digest: null, query_id: "query", query_digest: digest, rule_id: "rule", rule_digest: digest,
    metric_freshness_seconds: 120 };
  assert.equal(JSON.parse(encodeStageControlRequest(observe)).native_version, "1");
  assert.equal(JSON.parse(encodeStageControlRequest(configure)).conditional_version, "1");
  assert.throws(() => encodeStageControlRequest({ ...request, candidate: { ...request.candidate, version: opaqueVersion } }));
  assert.throws(() => encodeStageControlRequest({ ...observe, native_version: opaqueVersion }));
  assert.throws(() => encodeStageControlRequest({ ...configure, conditional_version: opaqueVersion }));
});

async function fixture(t, receive) {
  // AF_UNIX paths are limited to 103 bytes on Darwin, independently of TMPDIR.
  const directory = mkdtempSync(join(realpathSync("/tmp"), "pay241-socket-"));
  chmodSync(directory, 0o700);
  const path = join(directory, "control.sock");
  const sockets = new Set();
  const server = createServer((socket) => {
    sockets.add(socket); socket.on("close", () => sockets.delete(socket));
    let buffered = "";
    socket.on("data", (bytes) => {
      buffered += bytes.toString("utf8");
      let end;
      while ((end = buffered.indexOf("\n")) !== -1) {
        const line = buffered.slice(0, end); buffered = buffered.slice(end + 1);
        receive(JSON.parse(line), socket);
      }
    });
  });
  const control = new StageExecutionControl(path);
  t.after(async () => {
    control.close();
    for (const socket of sockets) socket.destroy();
    await new Promise((resolve) => server.close(resolve));
    rmSync(directory, { recursive: true, force: true });
  });
  server.listen(path); await once(server, "listening"); chmodSync(path, 0o600);
  return { control, path };
}

test("one correlated socket reply confirms a succeeded rollback with an opaque conditional token", async (t) => {
  const received = [];
  const terminal = { ...reply, result: { ...receipt, kind: "rollback", state: "succeeded", fact_id: approval },
    calls: { count: 2, last: { ...reply.calls.last, kind: "rollback", conditional_version: opaqueVersion } } };
  const { control } = await fixture(t, (value, socket) => {
    received.push(value);
    socket.write(`${JSON.stringify(terminal)}\n`);
  });
  const result = await control.send(request);
  assert.deepEqual(result, terminal);
  assert.equal(result.calls.last.conditional_version, opaqueVersion);
  assert.deepEqual(received, [request]);
});

test("a malformed conditional token from a socket remains unconfirmed without replay", async (t) => {
  for (const value of [null, 1, "a\0b", "界".repeat(86)]) {
    let count = 0;
    const malformed = { ...reply, calls: { count: 1, last: { ...reply.calls.last, conditional_version: value } } };
    const { control } = await fixture(t, (_request, socket) => { count++; socket.write(`${JSON.stringify(malformed)}\n`); });
    await assert.rejects(control.send(request), /unconfirmed/i);
    assert.equal(count, 1);
  }
});

test("receipt recovery may reuse an already completed original request ID without a second prepare", async (t) => {
  const requests = [];
  const { control } = await fixture(t, (value, socket) => {
    requests.push(value);
    socket.write(`${JSON.stringify(reply)}\n`);
  });
  assert.deepEqual(await control.send(request), reply);
  assert.deepEqual(await control.send({ ...request, action: "execution.receipt" }), reply);
  assert.deepEqual(requests.map((r) => r.action), ["execution.prepare", "execution.receipt"]);
  assert.equal(requests[0].request_id, requests[1].request_id);
});

test("disconnected prepare stays uncertain and is never resent", async (t) => {
  let count = 0;
  const { control } = await fixture(t, (_value, socket) => { count++; socket.destroy(); });
  await assert.rejects(control.send(request), /unconfirmed/i);
  assert.equal(count, 1);
});

test("a mismatched response cannot confirm another original request", async (t) => {
  const { control } = await fixture(t, (_value, socket) => socket.write(`${JSON.stringify({ ...reply, request_id: issue })}\n`));
  await assert.rejects(control.send(request), /unconfirmed/i);
});

test("shared-write socket permissions are refused before sending", async (t) => {
  let count = 0;
  const { control, path } = await fixture(t, () => { count++; });
  chmodSync(path, 0o660);
  await assert.rejects(control.send(request));
  assert.equal(count, 0);
});

test("overlong Unix socket paths fail before an operating-system truncation", async () => {
  const control = new StageExecutionControl(`/private/tmp/${"x".repeat(104)}`);
  await assert.rejects(control.send(request), /103 path bytes/);
});

test("one pending original ID cannot be sent twice concurrently", async (t) => {
  let arrived, target, count = 0;
  const seen = new Promise((resolve) => { arrived = resolve; });
  const { control } = await fixture(t, (_value, socket) => { count++; target = socket; arrived(); });
  const first = control.send(request);
  await seen;
  await assert.rejects(control.send(request), /already pending/);
  target.write(`${JSON.stringify(reply)}\n`);
  await first;
  assert.equal(count, 1);
});

test("oversized and invalid UTF-8 replies remain unconfirmed without replay", async (t) => {
  for (const output of [Buffer.alloc(65538, 65), Buffer.from([255, 10])]) {
    let count = 0;
    const { control } = await fixture(t, (_value, socket) => { count++; socket.write(output); });
    await assert.rejects(control.send(request), /unconfirmed/i);
    assert.equal(count, 1);
  }
});

async function loadCompletionFault() {
  const path = join(dirname(sourcePath), "collaboration-stage-execution.ts");
  const raw = readFileSync(path, "utf8"), tree = ts.createSourceFile(path, raw, ts.ScriptTarget.Latest, true);
  const selected = tree.statements.filter((node) =>
    ts.isFunctionDeclaration(node) && node.name && ["stageObject", "loseStageCompletionBeforeForward"].includes(node.name.text) ||
    ts.isVariableStatement(node) && node.declarationList.declarations.some((item) => ts.isIdentifier(item.name) && ["stageBytesHash", "canonicalUUID"].includes(item.name.text)));
  assert.equal(selected.length, 4, "completion fault driver must use the actual bounded interception helper");
  const pathOut = join(moduleRoot, `completion-${Math.random().toString(36).slice(2)}.mjs`);
  writeFileSync(pathOut, ts.transpileModule(`import {createHash} from "node:crypto";\n${selected.map((node) => node.getText(tree)).join("\n")}`, {
    compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext },
  }).outputText);
  return (await import(pathToFileURL(pathOut).href)).loseStageCompletionBeforeForward;
}

test("Web completion fault retains the exact original body and aborts before any API forwarding", async () => {
  const lose = await loadCompletionFault();
  const body = JSON.stringify({ action: "complete", submission_id: candidate, completion: { authorization_id: approval, operation_id: id, expected_issue_revision: 3 } });
  let handler, forwards = 0, aborts = 0, removed = 0, continued = 0;
  const page = { route: async (_match, callback) => { handler = callback; }, unroute: async (_match, callback) => { assert.equal(callback, handler); removed++; } };
  const route = { request: () => ({ method: () => "POST", headers: () => ({ "idempotency-key": id }), postData: () => body }),
    fetch: async () => { forwards++; throw new Error("The fault must not contact the API"); }, abort: async () => { aborts++; }, continue: async () => { continued++; } };
  const captured = await lose(page, { id: issue }, "/api/collaboration/issues/x/lifecycle", () => handler(route));
  assert.equal(captured.request_id, id); assert.deepEqual(captured.request_body, JSON.parse(body));
  assert.equal(captured.transport, "aborted_before_forward");
  assert.deepEqual({ forwards, aborts, removed, continued }, { forwards: 0, aborts: 1, removed: 1, continued: 0 });
});

test("completion fault refuses a different operation key without forwarding or repairing the request", async () => {
  const lose = await loadCompletionFault();
  let handler, forwards = 0, aborts = 0, removed = 0;
  const page = { route: async (_match, callback) => { handler = callback; }, unroute: async () => { removed++; } };
  const route = { request: () => ({ method: () => "POST", headers: () => ({ "idempotency-key": id }),
    postData: () => JSON.stringify({ action: "complete", completion: { authorization_id: approval, operation_id: candidate, expected_issue_revision: 3 } }) }),
    fetch: async () => { forwards++; }, abort: async () => { aborts++; }, continue: async () => undefined };
  await assert.rejects(lose(page, { id: issue }, "/api/collaboration/issues/x/lifecycle", () => handler(route)), /original operation/i);
  assert.deepEqual({ forwards, aborts, removed }, { forwards: 0, aborts: 1, removed: 1 });
});
