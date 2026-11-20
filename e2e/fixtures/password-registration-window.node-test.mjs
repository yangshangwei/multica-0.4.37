import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import { setImmediate } from "node:timers/promises";
import { URL } from "node:url";
import { compileFunction } from "node:vm";
import ts from "typescript";

const source = readFileSync(new URL("../fixtures.ts", import.meta.url), "utf8");
const { outputText } = ts.transpileModule(source, {
  compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS },
});
const loadFixture = compileFunction(outputText, ["exports", "require", "process", "fetch", "setTimeout", "console"]);
const base = "https://fixture.invalid";
const account = { username: "fixture-user", password: "synthetic-only", name: "Fixture User" };
const register = (client) => client.registerPassword(account.username, account.password, account.name);
const limited = { code: "rate_limited", error: "Too many attempts. Try again later" };
const user = { id: "fixture-id", email: "", onboarded_at: null };

function denial({ status = 429, retryAfter = "2", body = limited } = {}) {
  return globalThis.Response.json(body, { status, headers: retryAfter === null ? {} : { "Retry-After": retryAfter } });
}

function fixture(context, flag, replies, timer = () => assert.fail("unexpected wait")) {
  const env = { NEXT_PUBLIC_API_URL: base };
  if (flag !== undefined) env.E2E_WAIT_FOR_REGISTRATION_WINDOW = flag;
  const fetch = context.mock.fn(async () => {
    assert.ok(replies.length, "unexpected request");
    const reply = replies.shift();
    if (reply instanceof Error) throw reply;
    return reply;
  });
  const exports = {};
  const log = context.mock.fn();
  context.after(() => assert.equal(log.mock.callCount(), 0, "registration must not log"));
  loadFixture(exports, (name) => {
    assert.ok(["./env", "pg"].includes(name));
    return {};
  }, { env }, fetch, timer, { log, info: log, warn: log, error: log, debug: log });
  return { client: new exports.TestApiClient(), fetch };
}

async function assertDenied(context, flag, response) {
  const { client, fetch } = fixture(context, flag, [response]);
  await assert.rejects(register(client), { message: `password registration failed: ${response.status}` });
  assert.equal(fetch.mock.callCount(), 1);
  assert.equal(client.getToken(), null);
}

for (const seconds of [1, 300]) {
  test(`registration waits the full ${seconds}s before repeating the identical POST`, async (context) => {
    context.mock.timers.enable({ apis: ["setTimeout"] });
    const timer = context.mock.fn(globalThis.setTimeout);
    const { client, fetch } = fixture(context, "1", [
      denial({ retryAfter: String(seconds) }),
      globalThis.Response.json({ token: "synthetic-session", user }, { status: 201 }),
    ], timer);
    const result = Promise.allSettled([register(client)]);
    const request = [`${base}/auth/register`, {
      method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(account),
    }];
    await setImmediate();
    assert.equal(timer.mock.callCount(), 1);
    context.mock.timers.tick(seconds * 1000 - 1);
    await setImmediate();
    assert.deepEqual(fetch.mock.calls.map((call) => call.arguments), [request]);
    assert.equal(client.getToken(), null);
    context.mock.timers.tick(1);
    await setImmediate();
    assert.deepEqual(fetch.mock.calls.map((call) => call.arguments), [request, request]);
    assert.deepEqual(await result, [{ status: "fulfilled", value: user }]);
    assert.equal(client.getToken(), "synthetic-session");
  });
}

test("a second limiter denial is final even with another valid Retry-After", async (context) => {
  context.mock.timers.enable({ apis: ["setTimeout"] });
  const timer = context.mock.fn(globalThis.setTimeout);
  const { client, fetch } = fixture(context, "1", [denial(), denial()], timer);
  const rejected = assert.rejects(register(client), { message: "password registration failed: 429" });
  await setImmediate();
  context.mock.timers.tick(2000);
  await setImmediate();
  assert.equal(timer.mock.callCount(), 1);
  assert.equal(fetch.mock.callCount(), 2);
  await rejected;
  assert.equal(client.getToken(), null);
});

test("only the exact opt-in consumes the denial and waits", async (context) => {
  for (const flag of [undefined, "0", "true", " 1"]) {
    const response = denial();
    await assertDenied(context, flag, response);
    assert.equal(response.bodyUsed, false);
  }
});

test("missing, malformed and out-of-window Retry-After values fail without waiting", async (context) => {
  for (const retryAfter of [null, "", "0", "-1", "+1", "1.5", "1e2", "2x", "1, 2", "301", "Wed, 21 Oct 2015 07:28:00 GMT"]) {
    await assertDenied(context, "1", denial({ retryAfter }));
  }
});

test("only an explicit registration limiter denial can admit the second POST", async (context) => {
  for (const body of [null, limited.code, { error: limited.code }, { error: { code: limited.code } },
    { ...limited, code: "other_limit" }, { code: limited.code }, { code: limited.code, error: null }]) {
    await assertDenied(context, "1", denial({ body }));
  }
  await assertDenied(context, "1", new globalThis.Response(`not JSON: ${account.password}`, {
    status: 429, headers: { "Retry-After": "2" },
  }));
  await assertDenied(context, "1", Object.defineProperty(denial(), "redirected", { value: true }));
  for (const status of [401, 403, 409, 503]) await assertDenied(context, "1", denial({ status }));
});

test("transport failures and unusable success replies never replay a registration", async (context) => {
  const dropped = new TypeError("connection lost");
  const { client, fetch } = fixture(context, "1", [dropped]);
  await assert.rejects(register(client), (error) => error === dropped);
  assert.equal(fetch.mock.callCount(), 1);
  const ambiguous = fixture(context, "1", [globalThis.Response.json({ user }, { status: 201 })]);
  await assert.rejects(register(ambiguous.client), { message: "Password registration returned an unusable test session" });
  assert.equal(ambiguous.fetch.mock.callCount(), 1);
  assert.equal(ambiguous.client.getToken(), null);
});

test("the registration opt-in never retries login or other fixture writes", async (context) => {
  const { client, fetch } = fixture(context, "1", [denial(), denial()]);
  await assert.rejects(client.loginPassword(account.username, account.password), { message: "password login failed: 429" });
  await assert.rejects(client.requestJSON("/api/fixture", { method: "POST" }), { message: "POST /api/fixture failed: 429" });
  assert.deepEqual(fetch.mock.calls.map((call) => call.arguments[0]), [`${base}/auth/login`, `${base}/api/fixture`]);
});
