// @vitest-environment node
import { expect, it } from "vitest";
import { ApiError } from "../api/client";
import { classifyConnectionFailure } from "./index";

it.each([
  { error: new DOMException("fixture-secret", "TimeoutError"), expected: { kind: "timeout" } },
  { error: new TypeError("fixture-secret"), expected: { kind: "network" } },
  { error: new ApiError("fixture-secret", 503, "Unavailable", { token: "fixture-secret" }), expected: { kind: "server", status: 503 } },
  { error: new ApiError("fixture-secret", 403, "Forbidden"), expected: { kind: "unknown", status: 403 } },
  { error: new Error("fixture-secret"), expected: { kind: "unknown" } },
  { error: { message: "fixture-secret", status: 503 }, expected: { kind: "unknown" } },
])("exposes only known failure facts: $expected.kind", ({ error, expected }) => {
  expect(classifyConnectionFailure(error, 123)).toEqual({ ...expected, occurredAt: 123 });
});
