// @vitest-environment node

import { readdirSync, readFileSync } from "node:fs";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ApiClient } from "./client";
import { setSchemaLogger } from "./schema";
import { noopLogger } from "../logger";

// The answers of the budget API that the server's handlers and the CLI are
// tested against. They are read where they are: a copy kept beside this file
// could drift from what the server answers.
const fixtures = new URL("../../../server/internal/handler/testdata/issue_budget_http_v1/", import.meta.url);
const showFixtures = readdirSync(fixtures).filter((name) => /^show-.*\.json$/.test(name)).sort();
const fixture = (name: string): string => readFileSync(new URL(name, fixtures), "utf8");

const ROOT = "7d0c2b9e-5a41-4f7c-8e63-1b2a9c4d5e6f";
const ENDPOINT = "GET /api/issue-budgets/:id";

type Json = null | boolean | number | string | Json[] | { [key: string]: Json };

const open = fixture("show-open.json");

/**
 * The text of `show-open.json` after one change: the value at a path such as
 * `unknown.by_reason.0.count` is replaced, or its key is removed when no
 * value is given.
 */
function openWith(path: string, ...value: [Json?]): string {
  const answer: Json = JSON.parse(open);
  const keys = path.split(".");
  let node: Json = answer;
  for (const key of keys.slice(0, -1)) node = childOf(node, key);
  const last = keys[keys.length - 1] ?? "";
  if (Array.isArray(node)) {
    if (value.length === 0) node.splice(Number(last), 1);
    else node[Number(last)] = value[0] ?? null;
  } else if (node !== null && typeof node === "object") {
    if (value.length === 0) {
      if (!(last in node)) throw new Error(`show-open.json has no ${path}`);
      delete node[last];
    } else {
      node[last] = value[0] ?? null;
    }
  } else {
    throw new Error(`show-open.json has nothing at ${path}`);
  }
  return JSON.stringify(answer);
}

function childOf(node: Json, key: string): Json {
  const child = Array.isArray(node) ? node[Number(key)]
    : node !== null && typeof node === "object" ? node[key] : undefined;
  if (child === undefined) throw new Error(`show-open.json has no ${key}`);
  return child;
}

const fetchMock = vi.fn();
const warn = vi.fn();
const requestLog = vi.fn();
const client = new ApiClient("https://budgets.test", {
  logger: { debug: requestLog, info: requestLog, warn: requestLog, error: requestLog },
});

beforeEach(() => {
  fetchMock.mockReset();
  warn.mockReset();
  requestLog.mockReset();
  vi.stubGlobal("fetch", fetchMock);
  setSchemaLogger({ ...noopLogger, warn });
});

afterEach(() => {
  vi.unstubAllGlobals();
  setSchemaLogger(noopLogger);
});

function reply(body: string, status = 200): void {
  fetchMock.mockResolvedValue(new Response(status === 204 ? null : body, {
    status, headers: { "Content-Type": "application/json" },
  }));
}

/**
 * Every run of eight or more characters of a name, an amount, an ID or a
 * time in a body. The root's own ID is left out: the caller asked with it,
 * and the request log names the path.
 */
function partsOf(body: string): string[] {
  return (body.match(/[\w.-]{8,}/g) ?? []).filter((part) => part !== ROOT);
}

/**
 * Neither the schema warning nor the request log may hold a part of the
 * answer. Eight characters in a row are enough: the text of a syntax error
 * quotes only a few characters around the place it stopped at.
 */
function expectNothingOfTheAnswerLogged(body: string): void {
  // The text of an error is not among its own keys, so it is written out.
  const logged = JSON.stringify([...warn.mock.calls, ...requestLog.mock.calls],
    (_key, value: unknown) => (value instanceof Error ? `${value.name}: ${value.message}` : value));
  for (const part of partsOf(body)) {
    for (let at = 0; at + 8 <= part.length; at += 1) expect(logged).not.toContain(part.slice(at, at + 8));
  }
}

describe("the request for an issue budget", () => {
  it("asks for the root by its ID in the workspace it is given", async () => {
    reply(open);
    await client.getIssueBudget(ROOT, { workspaceId: "workspace-1" });
    expect(fetchMock).toHaveBeenCalledOnce();
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit & { headers: Record<string, string> }];
    expect(url).toBe(`https://budgets.test/api/issue-budgets/${ROOT}`);
    expect(init.method ?? "GET").toBe("GET");
    expect(init.body).toBeUndefined();
    expect(init.headers["X-Workspace-ID"]).toBe("workspace-1");
  });

  it("escapes the ID in the path", async () => {
    reply(open);
    await client.getIssueBudget("a/b?c#d");
    expect(fetchMock.mock.calls[0]?.[0]).toBe("https://budgets.test/api/issue-budgets/a%2Fb%3Fc%23d");
  });

  it("is not sent once the caller has given up", async () => {
    const gone = new AbortController();
    gone.abort();
    await expect(client.getIssueBudget(ROOT, { signal: gone.signal })).rejects.toMatchObject({ name: "AbortError" });
    expect(fetchMock).not.toHaveBeenCalled();
  });

  // A refusal is an error the caller can tell apart by its status and code. It
  // is not "no answer".
  it.each<[string, number, string]>([
    ["error-budget-forbidden.json", 403, "budget_forbidden"],
    ["error-not-found.json", 404, "not_found"],
  ])("fails with the status and the code of %s", async (name, status, code) => {
    reply(fixture(name), status);
    await expect(client.getIssueBudget(ROOT)).rejects.toMatchObject({ status, body: { error: code, code } });
    expect(warn).not.toHaveBeenCalled();
  });

  it.each([400, 401, 403, 404, 409, 500, 502])("keeps a %i error body out of the client logger", async (status) => {
    const body = { error: "private budget reason, amount 73500000000", code: "internal_error" };
    reply(JSON.stringify(body), status);

    await expect(client.getIssueBudget(ROOT)).rejects.toMatchObject({ status, body });

    expect(warn).not.toHaveBeenCalled();
    expect(requestLog).toHaveBeenCalledWith(`← ${status} /api/issue-budgets/${ROOT}`, {
      rid: expect.any(String), duration: expect.any(String), error: "Budget request failed",
    });
    expectNothingOfTheAnswerLogged(JSON.stringify(body));
  });
});

describe("the answer for an issue budget", () => {
  it("finds the five answers of the fixture directory", () => {
    expect(showFixtures).toEqual([
      "show-known.json", "show-no-limit.json", "show-open.json", "show-tombstoned-off.json", "show-unavailable.json",
    ]);
  });

  // Amounts and revisions are 64-bit values. They arrive as decimal strings
  // and are returned as the same strings; nothing is added and nothing is lost.
  it.each(showFixtures)("returns %s with the values it came with", async (name) => {
    const body = fixture(name);
    reply(body);
    expect(await client.getIssueBudget(ROOT)).toStrictEqual(JSON.parse(body));
    expect(warn).not.toHaveBeenCalled();
  });

  // A later server may add a field. It does not cost the answer, and it is
  // left out of it: what this version returns is what its type names.
  it.each<[string, string]>([
    ["beside the amounts", openWith("spent_ticks", "1")],
    ["among the amounts", openWith("amounts.refunded_ticks", "2")],
    ["in the unknown part", openWith("unknown.since", "2026-11-19")],
    ["in a reason", openWith("unknown.by_reason.0.detail", {})],
    ["in the coverage", openWith("coverage.note", null)],
  ])("leaves out a field of a later server %s", async (_name, body) => {
    expect(body).not.toBe(open);
    reply(body);
    expect(await client.getIssueBudget(ROOT)).toStrictEqual(JSON.parse(open));
    expect(warn).not.toHaveBeenCalled();
  });

  // A later server may add a state, a mode, a reason or a name, and a later
  // slice sets what the first one leaves null. Each is returned as it came.
  it.each<[string, string]>([
    ["a state of a later server", openWith("admission_state", "frozen")],
    ["a mode of a later server", openWith("mode", "enforce")],
    ["a ledger state of a later server", openWith("ledger_state", "rebuilding")],
    ["a reason of a later server", openWith("unknown.by_reason.0.reason", "quota_drift")],
    ["a name of a later server that is not counted", openWith("coverage.not_applicable.3", "webhook_run")],
    ["a settled revision", openWith("settled_as_of_revision", "41")],
    ["an amount that no JSON number can hold", openWith("limit_ticks", "9223372036854775807")],
    ["an amount of zero", openWith("limit_ticks", "0")],
  ])("returns %s as it came", async (_name, body) => {
    expect(body).not.toBe(open);
    reply(body);
    expect(await client.getIssueBudget(ROOT)).toStrictEqual(JSON.parse(body));
    expect(warn).not.toHaveBeenCalled();
  });

  // A malformed answer is no answer: never an object of zeros and never a
  // state the server did not give. What is logged names the endpoint and
  // holds no part of the answer.
  it.each<[string, string]>([
    ["an amount as a number", openWith("limit_ticks", 250000000000)],
    ["an amount as a number that no JSON reader keeps", open.replace('"limit_ticks":"250000000000"', '"limit_ticks":9223372036854775807')],
    ["an amount of the ledger as a number", openWith("amounts.actual_known_ticks", 73500000000)],
    ["the remaining amount as a number", openWith("remaining_ticks", 0)],
    ["an amount with a leading zero", openWith("limit_ticks", "0250000000000")],
    ["an amount of two zeros", openWith("amounts.overrun_ticks", "00")],
    ["an amount with a minus sign", openWith("limit_ticks", "-250000000000")],
    ["an amount with a plus sign", openWith("attempt_reserve_ticks", "+5000000000")],
    ["an amount in dollars", openWith("limit_ticks", "25.00")],
    ["an amount with an exponent", openWith("limit_ticks", "25e10")],
    ["an amount that is empty", openWith("limit_ticks", "")],
    ["an amount followed by a line feed", openWith("limit_ticks", "250000000000\n")],
    ["an amount after a space", openWith("limit_ticks", " 250000000000")],
    ["an amount on the second line of a text", openWith("limit_ticks", "limit\n250000000000")],
    ["a missing limit_ticks", openWith("limit_ticks")],
    ["a missing attempt_reserve_ticks", openWith("attempt_reserve_ticks")],
    ["a missing remaining_ticks", openWith("remaining_ticks")],
    ["a missing settled_as_of_revision", openWith("settled_as_of_revision")],
    ["a missing amounts key", openWith("amounts")],
    ["a missing included_attempts", openWith("coverage.included_attempts")],
    ["an amount of the ledger that is missing", openWith("amounts.overrun_ticks")],
    ["an amount of the ledger of null", openWith("amounts.estimated_ticks", null)],
    ["amounts that are a list", openWith("amounts", [])],
    ["a revision as a number", openWith("policy_revision", 3)],
    ["an epoch as a number", openWith("policy_epoch", 1)],
    ["a ledger revision of null", openWith("ledger_revision", null)],
    ["a settled revision as a number", openWith("settled_as_of_revision", 7)],
    ["a missing ledger_state", openWith("ledger_state")],
    ["a ledger state of null", openWith("ledger_state", null)],
    ["a state that is a number", openWith("admission_state", 1)],
    ["a missing admission_state", openWith("admission_state")],
    ["a mode of null", openWith("mode", null)],
    ["a missing total_known", openWith("total_known")],
    ["a total_known of null", openWith("total_known", null)],
    ["a total_known that is a word", openWith("total_known", "false")],
    ["a count as a string", openWith("unknown.by_reason.0.count", "1")],
    ["the count from before activation as a string", openWith("unknown.pre_activation", "1")],
    ["a count with a fraction", openWith("unknown.pre_activation", 1.5)],
    ["a count below zero", openWith("unknown.by_reason.0.count", -1)],
    ["a reason without its count", openWith("unknown.by_reason.1.count")],
    ["a reason that is a number", openWith("unknown.by_reason.0.reason", 7)],
    ["reasons of null", openWith("unknown.by_reason", null)],
    ["a missing pre_activation", openWith("unknown.pre_activation")],
    ["a missing unknown part", openWith("unknown")],
    ["included attempts as a string", openWith("coverage.included_attempts", "12")],
    ["included attempts below zero", openWith("coverage.included_attempts", -1)],
    ["a name that is null", openWith("coverage.not_applicable.1", null)],
    ["a missing list of what is not counted", openWith("coverage.not_applicable")],
    ["a missing coverage", openWith("coverage")],
    ["an activation time of null", openWith("activated_at", null)],
    ["a missing budget_root_id", openWith("budget_root_id")],
    ["a root issue that is a number", openWith("root_issue_id", 7)],
    ["an answer of null", "null"],
    ["an answer that is a list", `[${open}]`],
    ["an answer that is a text", JSON.stringify(open)],
    ["the answer of another route", fixture("attempts-page.json")],
    ["the body of a refusal under a 200", fixture("error-not-found.json")],
    ["an answer cut in the middle", open.slice(0, open.length / 2)],
    ["an answer with a stray character after an amount", open.replace('"limit_ticks":"250000000000"', '"limit_ticks":250000000000x')],
    // At a word or a sign where a value belongs, a JSON reader's error quotes
    // what stands around it: here the name of the field and the amount.
    ["an amount that is not a number", open.replace('"limit_ticks":"250000000000"', '"limit_ticks":NaN')],
    ["an amount as a number with a plus sign", open.replace('"limit_ticks":"250000000000"', '"limit_ticks":+250000000000')],
    ["a state without its quotes", open.replace('"ledger_state":"available"', '"ledger_state":available')],
    ["a page instead of an answer", "<html><body>budget 250000000000 of 0b7f4c1e-93a2-4d58-b6e1-52c8a7f0d314</body></html>"],
    ["an empty answer", ""],
  ])("gives no answer for %s", async (_name, body) => {
    expect(body).not.toBe(open);
    reply(body);
    expect(await client.getIssueBudget(ROOT)).toBeNull();
    expect(warn.mock.calls).toStrictEqual([
      [`API response failed schema validation: ${ENDPOINT}`, { endpoint: ENDPOINT }],
    ]);
    expectNothingOfTheAnswerLogged(body);
  });

  it("gives no answer for a 204", async () => {
    reply("", 204);
    expect(await client.getIssueBudget(ROOT)).toBeNull();
    expect(warn).toHaveBeenCalledOnce();
  });

  // The check above must be able to fail: the same search finds an amount
  // when a log does hold one, also when the text of an error quotes no more
  // than ten characters on each side of one place of the answer.
  it("would notice an amount in a log", () => {
    warn("API response failed schema validation", { received: JSON.parse(open) });
    expect(() => expectNothingOfTheAnswerLogged(open)).toThrow();
  });

  it("would notice an error in a log that quotes a few characters of the answer", () => {
    const at = open.indexOf("250000000000");
    const quoted = open.slice(at - 10, at + 11);
    requestLog("request failed", new SyntaxError(`Unexpected token, ...${JSON.stringify(quoted)}... is not valid JSON`));
    expect(() => expectNothingOfTheAnswerLogged(open)).toThrow();
  });

  it("finds nothing of the answer in the log of a request that was answered well", async () => {
    reply(open);
    await client.getIssueBudget(ROOT);
    expect(requestLog).toHaveBeenCalled();
    expectNothingOfTheAnswerLogged(open);
  });
});
