import assert from "node:assert/strict";
import { Buffer } from "node:buffer";
import { createHash } from "node:crypto";
import { once } from "node:events";
import { chmodSync, lstatSync, mkdtempSync, readFileSync, realpathSync, rmSync, writeFileSync } from "node:fs";
import { createServer } from "node:net";
import { tmpdir } from "node:os";
import { dirname, isAbsolute, join, normalize } from "node:path";
import process from "node:process";
import test from "node:test";
import { fileURLToPath, pathToFileURL } from "node:url";
import ts from "typescript";

const fixtureDirectory = dirname(fileURLToPath(import.meta.url));
const sourcePath = join(fixtureDirectory, "collaboration-stage-execution.ts");
const controlPath = join(fixtureDirectory, "collaboration-stage-control.ts");
const hash = (bytes) => createHash("sha256").update(bytes).digest("hex");

async function loadActualConfiguration(t) {
  const moduleDirectory = mkdtempSync(join(tmpdir(), "stage-configuration-module-"));
  chmodSync(moduleDirectory, 0o700);
  t.after(() => rmSync(moduleDirectory, { recursive: true, force: true }));
  const source = readFileSync(sourcePath, "utf8");
  const tree = ts.createSourceFile(sourcePath, source, ts.ScriptTarget.Latest, true);
  const selected = tree.statements.filter((node) =>
    ts.isFunctionDeclaration(node) && node.name && ["configureStageExecution", "stageString"].includes(node.name.text) ||
    ts.isVariableStatement(node) && node.declarationList.declarations.some((item) =>
      ts.isIdentifier(item.name) && item.name.text === "stageBytesHash"));
  assert.equal(selected.length, 3, "configuration must execute the three actual fixture declarations");
  const imports = `import { createHash, randomUUID } from "node:crypto";\nimport { expect } from ${JSON.stringify(import.meta.resolve("@playwright/test"))};\n`;
  const helperPath = join(moduleDirectory, "configuration.mjs");
  const actualControlPath = join(moduleDirectory, "control.mjs");
  const options = { compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext } };
  writeFileSync(helperPath, ts.transpileModule(imports + selected.map((node) => node.getText(tree)).join("\n"), options).outputText, { mode: 0o600 });
  writeFileSync(actualControlPath, ts.transpileModule(readFileSync(controlPath, "utf8"), options).outputText, { mode: 0o600 });
  const helper = await import(pathToFileURL(helperPath).href);
  const control = await import(pathToFileURL(actualControlPath).href);
  return { ...helper, ...control, helperPath, actualControlPath };
}

test("stage execution registers the original 120-second metric horizon through its real private control", async (t) => {
  const captureDirectory = process.env.PAY241_R19_CAPTURE_DIR;
  const commandPath = process.env.PAY241_R19_COMMAND_PATH;
  assert.ok(captureDirectory && isAbsolute(captureDirectory), "the run must retain its socket evidence in an absolute directory");
  assert.ok(commandPath && isAbsolute(commandPath), "the run must retain its command binding");
  const directoryStat = lstatSync(captureDirectory);
  assert.ok(directoryStat.isDirectory() && directoryStat.mode % 0o1000 === 0o700 && directoryStat.uid === process.getuid());
  const commandBytes = readFileSync(commandPath);
  const command = JSON.parse(commandBytes);
  const { configureStageExecution, StageExecutionControl, encodeStageControlRequest, helperPath, actualControlPath } = await loadActualConfiguration(t);
  assert.equal(command.source_sha256, hash(readFileSync(sourcePath)));
  assert.equal(command.control_sha256, hash(readFileSync(controlPath)));
  assert.equal(command.test_sha256, hash(readFileSync(fileURLToPath(import.meta.url))));

  // Darwin limits AF_UNIX paths to 103 bytes, independently of the isolated TMPDIR.
  const socketDirectory = mkdtempSync(join(realpathSync("/tmp"), "pay241-socket-"));
  chmodSync(socketDirectory, 0o700);
  const socketPath = join(socketDirectory, "control.sock");
  const sockets = new Set();
  const frames = [];
  const acknowledgements = [];
  const targetDigest = "b".repeat(64);
  const callBaseline = 7;
  const server = createServer((socket) => {
    sockets.add(socket);
    socket.on("close", () => sockets.delete(socket));
    let buffered = Buffer.alloc(0);
    socket.on("data", (chunk) => {
      buffered = Buffer.concat([buffered, chunk]);
      let end;
      while ((end = buffered.indexOf(10)) !== -1) {
        const frame = buffered.subarray(0, end + 1);
        buffered = buffered.subarray(end + 1);
        frames.push(Buffer.from(frame));
        const received = JSON.parse(frame.toString("utf8"));
        const ack = { request_id: received.request_id, ok: true, result: { target_digest: targetDigest }, calls: { count: callBaseline, last: null } };
        const encodedAck = `${JSON.stringify(ack)}\n`;
        acknowledgements.push(encodedAck);
        socket.write(encodedAck);
      }
    });
  });
  const control = new StageExecutionControl(socketPath);
  t.after(async () => {
    control.close();
    for (const socket of sockets) socket.destroy();
    await new Promise((resolve) => server.close(resolve));
    rmSync(socketDirectory, { recursive: true, force: true });
  });
  server.listen(socketPath);
  await once(server, "listening");
  chmodSync(socketPath, 0o600);
  assert.equal(normalize(socketPath), socketPath);
  assert.equal(realpathSync(socketDirectory), socketDirectory);
  assert.ok(Buffer.byteLength(socketPath) <= 103);
  assert.equal(lstatSync(socketDirectory).mode % 0o1000, 0o700);
  assert.equal(lstatSync(socketPath).mode % 0o1000, 0o600);
  assert.equal(lstatSync(socketPath).uid, process.getuid());

  const workspaceId = "11111111-1111-4111-8111-111111111111";
  const sources = { deployment_integration_id: "22222222-2222-4222-8222-222222222222", metric_integration_id: "33333333-3333-4333-8333-333333333333" };
  const previousArtifact = "a".repeat(64);
  const suffix = "r19-original-horizon";
  const configured = await configureStageExecution(control, workspaceId, sources, previousArtifact, suffix);
  const rawFrame = Buffer.concat(frames);
  const rawAck = acknowledgements.join("");
  const received = JSON.parse(rawFrame.toString("utf8"));
  const acknowledged = JSON.parse(rawAck);
  const encodedReturnedRequest = encodeStageControlRequest(configured.request);
  const capture = {
    captured_at: new Date().toISOString(),
    source_path: sourcePath,
    source_sha256: hash(readFileSync(sourcePath)),
    control_path: controlPath,
    control_sha256: hash(readFileSync(controlPath)),
    test_path: fileURLToPath(import.meta.url),
    test_sha256: hash(readFileSync(fileURLToPath(import.meta.url))),
    command_path: commandPath,
    command_sha256: hash(commandBytes),
    actual_selected_helper_sha256: hash(readFileSync(helperPath)),
    actual_entire_control_sha256: hash(readFileSync(actualControlPath)),
    socket_path: socketPath,
    socket_parent_mode: "0700",
    socket_mode: "0600",
    socket_owner_uid: lstatSync(socketPath).uid,
    request_count: frames.length,
    raw_frame: rawFrame.toString("utf8"),
    raw_frame_sha256: hash(rawFrame),
    raw_ack: rawAck,
    raw_ack_sha256: hash(rawAck),
    request: received,
    reply: acknowledged,
    helper_return: configured,
    actual_encoder_output: encodedReturnedRequest,
    expected_metric_freshness_seconds: 120,
    captured_metric_freshness_seconds: received.metric_freshness_seconds,
    capture_precedes_final_horizon_assertion: true,
  };
  // Preserve the real frame and its immutable inputs even when the final horizon assertion fails.
  writeFileSync(join(captureDirectory, "source.configure.frame.jsonl"), rawFrame, { mode: 0o600, flag: "wx" });
  writeFileSync(join(captureDirectory, "source.configure.ack.jsonl"), rawAck, { mode: 0o600, flag: "wx" });
  writeFileSync(join(captureDirectory, "source.configure.capture.json"), `${JSON.stringify(capture, null, 2)}\n`, { mode: 0o600, flag: "wx" });

  assert.equal(frames.length, 1, "the actual helper must send exactly one configuration request");
  assert.equal(acknowledgements.length, 1);
  assert.match(received.request_id, /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/);
  assert.notEqual(received.request_id, "00000000-0000-0000-0000-000000000000");
  assert.deepEqual(received, configured.request);
  assert.equal(rawFrame.toString("utf8"), encodedReturnedRequest);
  assert.notEqual(received.deployment_integration_id, received.metric_integration_id);
  assert.deepEqual(received, {
    request_id: configured.request.request_id,
    action: "source.configure",
    workspace_id: workspaceId,
    deployment_integration_id: sources.deployment_integration_id,
    metric_integration_id: sources.metric_integration_id,
    environment: `synthetic-stage-${suffix}`,
    remote_instance: "controlled-local-stage-fixture",
    remote_tenant: suffix,
    remote_target: `target-${suffix}`,
    conditional_version: "1",
    current_artifact_digest: previousArtifact,
    query_id: "synthetic-payment-error-rate",
    query_digest: hash("controlled-query-v1"),
    rule_id: "synthetic-test-rule",
    rule_digest: hash("controlled-rule-v1"),
    metric_freshness_seconds: received.metric_freshness_seconds,
  });
  assert.deepEqual(acknowledged, { request_id: received.request_id, ok: true, result: { target_digest: targetDigest }, calls: { count: callBaseline, last: null } });
  assert.equal(configured.target, targetDigest);
  assert.equal(configured.callBaseline, callBaseline);
  assert.equal(received.metric_freshness_seconds, 120, "registered metric freshness must match the original scenario's 120-second horizon");
});
