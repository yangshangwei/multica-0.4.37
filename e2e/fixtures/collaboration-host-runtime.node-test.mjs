import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import vm from "node:vm";
import test from "node:test";

const source = readFileSync(new URL("./collaboration-host-runtime.ts", import.meta.url), "utf8").replace(/\r\n/g, "\n");

function closeFixture(platform, forceFallback = false) {
  const start = source.indexOf("  const close = async () => {");
  const end = source.indexOf("\n  try {\n    await expect.poll", start);
  assert(start > 0 && end > start, "Read the actual native Host close function");
  const closeSource = source.slice(start, end).replaceAll("new Promise<void>", "new Promise");
  const signals = [], actions = [], messages = [], ledger = [{ phase: "proof_acknowledged", proof_state: "host_exit_observed" }];
  const timeline = [];
  const mark = event => { assert.equal(actions.length, 0, "Close marks its timeline before EOF"); timeline.push(event); };
  let exited;
  const exit = new Promise(resolve => { exited = resolve; });
  const child = { exitCode: null,
    kill(signal) {
      signals.push(signal);
      if (platform === "win32" && ["SIGCONT", "SIGSTOP"].includes(signal)) throw new TypeError(`Unknown signal: ${signal}`);
      if (signal === "SIGKILL") { child.exitCode = 9; exited(9); }
      return true;
    },
    stdin: { end() {
      actions.push("stdin_eof");
      if (!forceFallback) {
        messages.push({ claims: 3, invalid_capacity: false, available_slots: 1, active_tasks: 0 });
        child.exitCode = 0; exited(0);
      }
    } },
  };
  let fallback;
  const timer = { callback: null, cleared: false };
  const factory = vm.compileFunction(`let closed = false; ${closeSource}; return close;`,
    ["process", "child", "exit", "sockets", "server", "directory", "runtime", "messages", "errors", "stderr", "hostLedgerEvidence", "rmSync", "join", "setTimeout", "clearTimeout", "mark", "timeline"]);
  const close = factory({ platform }, child, exit, new Set([{ destroy() { actions.push("socket_closed"); } }]),
    { close(done) { actions.push("callback_closed"); done(); } }, "/fixture", { deploymentId: "deployment", installationId: "installation" }, messages, [], "",
    () => ledger, () => actions.push("private_directory_removed"), (...parts) => parts.join("/"),
    (callback, milliseconds) => { assert.equal(milliseconds, 20_000); fallback = callback; return timer; },
    value => { assert.equal(value, timer); timer.cleared = true; }, mark, timeline);
  return { close, signals, actions, messages, ledger, timer, timeline, fireFallback: () => fallback() };
}

test("Windows Host close delivers EOF and retains the graceful final summary without POSIX signals", async () => {
  const f = closeFixture("win32");
  const result = await f.close();
  assert.deepEqual(f.signals, []);
  assert.deepEqual(f.actions, ["stdin_eof", "socket_closed", "callback_closed", "private_directory_removed"]);
  assert.equal(result.exit_code, 0);
  assert.deepEqual(result.messages, [{ claims: 3, invalid_capacity: false, available_slots: 1, active_tasks: 0 }]);
  assert.equal(result.ledger, f.ledger);
  assert.deepEqual(result.timeline, ["stdin_end_requested"]);
  assert.equal(result.timeline, f.timeline);
  assert.deepEqual(result.errors, []);
  assert.equal(f.timer.cleared, true);
  await assert.rejects(f.close(), /cleanup was invoked twice/);
});

test("POSIX Host close still resumes a suspended daemon before EOF", async () => {
  const f = closeFixture("darwin");
  const result = await f.close();
  assert.deepEqual(f.signals, ["SIGCONT"]);
  assert.equal(result.exit_code, 0);
  assert.equal(result.messages.length, 1);
  assert.equal(f.timer.cleared, true);
});

test("Windows fallback remains bounded and an abrupt exit cannot become a graceful pass", async () => {
  const f = closeFixture("win32", true);
  const pending = f.close();
  // A baseline portability failure must remain observed even if the next
  // assertion catches that EOF was never reached.
  void pending.catch(() => undefined);
  assert.deepEqual(f.actions, ["stdin_eof"]);
  f.fireFallback();
  const result = await pending;
  assert.deepEqual(f.signals, ["SIGKILL"]);
  assert.equal(result.exit_code, 9);
  assert.deepEqual(result.messages, []);
  assert.equal(f.timer.cleared, true);
});

test("Windows suspend and resume helpers refuse before sending unsupported signals", () => {
  const signals = [];
  const child = { exitCode: null, kill(signal) { signals.push(signal); return true; } };
  for (const name of ["freeze", "thaw"]) {
    const body = source.match(new RegExp(`      ${name}\\(\\) \\{([\\s\\S]*?)\\n      \\},`))?.[1];
    assert(body, `Read the actual ${name} helper`);
    const invoke = vm.compileFunction(body, ["process", "closed", "child"]);
    assert.throws(() => invoke({ platform: "win32" }, false, child), /unsupported on Windows/);
  }
  assert.deepEqual(signals, []);
});

function controlFixture({ squad = false, singleTarget = false, duplicateRow = false } = {}) {
  const targets = [
    { id: "11111111-1111-4111-8111-111111111111", resource_id: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", kind: "agent" },
    { id: "22222222-2222-4222-8222-222222222222", resource_id: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", kind: squad ? "squad" : "agent" },
  ];
  if (singleTarget) targets.splice(0, 1);
  const snapshot = { id: "99999999-9999-4999-8999-999999999999", execution: { protocol_version: 6, targets } };
  const selected = targets.at(-1);
  const issue = { id: snapshot.id, targetId: selected.id,
    agentId: squad ? "cccccccc-cccc-4ccc-8ccc-cccccccccccc" : selected.resource_id, name: "Shared synthetic QA name" };
  const states = new Map(targets.map(target => [target.id, "paused"]));
  const reads = [], writes = [], schemaReads = [], waiters = [];
  const emit = (action, target) => {
    const scope = target ? "target" : "issue";
    for (const id of target ? [target.id] : states.keys()) states.set(id, action === "resume" ? "active" : "paused");
    const body = { action, scope, execution_view_version: 6, ...(target ? { targets: [{ target_id: target.id }] } : {}) };
    writes.push(body);
    const response = { url: () => `http://127.0.0.1:64922/api/collaboration/issues/${issue.id}/control`, status: () => 200,
      request: () => ({ method: () => "POST", postDataJSON: () => body }),
      json: async () => ({ action, scope, target_id: target?.id ?? null, queue_state: action === "resume" ? "active" : "paused" }) };
    for (const waiter of waiters.splice(0)) { assert(waiter.match(response)); waiter.resolve(response); }
  };
  const button = (action, target) => ({ role: "button", name: `${action === "stop" ? "Stop" : "Resume"} ${target ? "target" : "all"}`,
    children: [], click: () => emit(action, target) });
  const resourceRow = target => ({ role: "listitem", children: [{ text: target.resource_id, children: [] }, button("stop", target), button("resume", target)] });
  const rows = targets.map(resourceRow);
  if (duplicateRow) rows.push(resourceRow(selected));
  // A retained execution record repeats the resource ID without target controls.
  rows.push({ role: "listitem", children: [{ text: selected.resource_id, children: [] }] });
  const panel = { role: "region", name: "AI execution", children: [...rows, button("stop"), button("resume")] };
  const descendants = node => node.children.flatMap(child => [child, ...descendants(child)]);
  const roleMatch = (role, options = {}) => node => node.role === role && (!options.name || node.name === options.name);
  const locator = (elements, matches) => ({ elements, matches,
    getByRole(role, options) { const predicate = roleMatch(role, options); return locator(elements.flatMap(descendants).filter(predicate), predicate); },
    filter({ has }) { return locator(elements.filter(node => descendants(node).some(has.matches)), matches); },
    async click() { assert.equal(elements.length, 1, "strict mode violation: target control is not unique"); elements[0].click(); },
  });
  const page = {
    getByRole(role, options) { const predicate = roleMatch(role, options); return locator([panel, ...descendants(panel)].filter(predicate), predicate); },
    getByText(text, options) { assert.equal(options.exact, true); const predicate = node => node.text === text;
      return locator(descendants(panel).filter(predicate), predicate); },
    evaluate: (callback, argument) => callback(argument),
    waitForResponse: match => new Promise(resolve => waiters.push({ match, resolve })),
  };
  const partial = (actual, expected) => {
    if (Array.isArray(expected)) assert.equal(actual.length, expected.length);
    if (expected && typeof expected === "object") for (const [key, value] of Object.entries(expected)) partial(actual[key], value);
    else assert.equal(actual, expected);
  };
  const expect = value => ({ toBe: expected => assert.equal(value, expected), toMatchObject: expected => partial(value, expected),
    toHaveCount: async expected => assert.equal(value.elements.length, expected, "Target resource must have exactly one control row") });
  const context = vm.createContext({ URL,
    localStorage: { getItem(key) { assert.equal(key, "multica_token"); return squad ? "synthetic-native-token" : null; } },
    fetch: async (url, options) => { reads.push({ url, options }); return { ok: true, status: 200, json: async () => structuredClone(snapshot) }; },
  });
  const start = source.indexOf("export async function hostControl(");
  const end = source.indexOf("\nexport async function sendHostComment", start);
  assert(start > 0 && end > start, "Read the actual shared Host control helper");
  const body = source.slice(start, end)
    .replace('export async function hostControl(page: Page, issue: HostIssue, action: "stop" | "resume", scope: "target" | "issue")',
      "async function hostControl(page, issue, action, scope)")
    .replace("const receipt: CollaborationControlReceipt", "const receipt");
  const factory = vm.compileFunction(`${body}; return hostControl;`, ["expect", "hostPanel", "issueRoot", "versioned", "CollaborationIssueSchema", "process"], { parsingContext: context });
  const control = factory(expect, page => page.getByRole("region", { name: "AI execution", exact: true }),
    id => `/api/collaboration/issues/${id}`, path => `${path}?execution_view_version=6`,
    { parse(value) { schemaReads.push(value); return value; } }, { env: { COLLABORATION_E2E_API_URL: "http://127.0.0.1:64922" } });
  return { control: (action, scope, candidate = issue) => control(page, candidate, action, scope), issue, selected, targets, states, reads, writes, schemaReads };
}

test("Host target Resume controls the selected public target among repeated buttons and retained run rows", async () => {
  const f = controlFixture();
  const before = f.states.get(f.targets[0].id);
  const receipt = await f.control("resume", "target");
  assert.equal(receipt.target_id, f.selected.id);
  assert.equal(f.states.get(f.selected.id), "active");
  assert.equal(f.states.get(f.targets[0].id), before);
  assert.equal(f.reads.length, 1);
  assert.equal(f.schemaReads.length, 1);
  assert.equal(f.reads[0].url, `http://127.0.0.1:64922/api/collaboration/issues/${f.issue.id}?execution_view_version=6`);
  assert.equal(f.reads[0].options.credentials, "include");
  assert.deepEqual(f.writes[0].targets, [{ target_id: f.selected.id }]);
});

test("Host target Stop resolves the squad resource independently of its current leader agent", async () => {
  const f = controlFixture({ squad: true });
  for (const id of f.states.keys()) f.states.set(id, "active");
  assert.notEqual(f.issue.agentId, f.selected.resource_id);
  const receipt = await f.control("stop", "target");
  assert.equal(receipt.target_id, f.selected.id);
  assert.equal(f.states.get(f.selected.id), "paused");
  assert.equal(f.states.get(f.targets[0].id), "active");
  assert.equal(f.reads.length, 1);
  assert.equal(f.reads[0].options.headers.Authorization, "Bearer synthetic-native-token");
});

test("Host issue Resume retains the all-target control without a target lookup", async () => {
  const f = controlFixture({ squad: true });
  const receipt = await f.control("resume", "issue");
  assert.equal(receipt.target_id, null);
  assert.equal(receipt.scope, "issue");
  assert.deepEqual([...f.states.values()], ["active", "active"]);
  assert.equal(f.reads.length, 0);
  assert.equal(f.schemaReads.length, 0);
  assert.deepEqual(f.writes, [{ action: "resume", scope: "issue", execution_view_version: 6 }]);
});

test("Host control refuses a missing public target before any control submission", async () => {
  const f = controlFixture({ singleTarget: true });
  await assert.rejects(f.control("resume", "target", { ...f.issue, targetId: "33333333-3333-4333-8333-333333333333" }));
  assert.equal(f.writes.length, 0);
  assert.equal(f.states.get(f.selected.id), "paused");
});

test("Host control refuses duplicate resource controls before any control submission", async () => {
  const f = controlFixture({ duplicateRow: true });
  await assert.rejects(f.control("resume", "target"), /strict mode violation|exactly one control row/);
  assert.equal(f.writes.length, 0);
  assert.deepEqual([...f.states.values()], ["paused", "paused"]);
});
