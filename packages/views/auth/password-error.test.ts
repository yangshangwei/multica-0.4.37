// @vitest-environment node
import { describe, expect, it } from "vitest";
import { ApiError } from "@multica/core/api";
import { passwordErrorDetails } from "./password-error";

const input = { username: "11052", name: "Test User", password: "a long password", registering: true };
const failure = (code: string, message = "Validation failed") => new ApiError(message, 400, "Bad Request", { code, error: message });

describe("password error details", () => {
  it.each([
    ["invalid_username", { username: "" }, { key: "username_required" }],
    ["invalid_username", { username: "12" }, { key: "username_too_short", length: 2 }],
    ["invalid_username", { username: "1".repeat(33) }, { key: "username_too_long", length: 33 }],
    ["invalid_username", { username: "user-name" }, { key: "username_invalid_characters" }],
    ["invalid_name", { name: " " }, { key: "name_required" }],
    ["invalid_name", { name: "😀".repeat(81) }, { key: "name_too_long", length: 81 }],
    ["invalid_password", { password: "short" }, { key: "password_too_short", length: 5 }],
    ["invalid_password", { password: "abc123" }, { key: "invalid_password" }],
    ["invalid_password", { password: "😀".repeat(129) }, { key: "password_too_long", length: 129 }],
  ])("explains %s for %j", (code, values, expected) => {
    expect(passwordErrorDetails(failure(code), { ...input, ...values })).toEqual(expected);
  });

  it.each([
    ["username must contain 3–32 ASCII characters", { username: "12" }, { key: "username_too_short", length: 2 }],
    ["username must start with a letter and contain only letters, numbers or underscores", {}, { key: "username_rules_outdated" }],
    ["username must start with a letter and contain only letters, numbers or underscores", { username: "a-b" }, { key: "username_invalid_characters" }],
    ["name must contain 1–80 characters", { name: " " }, { key: "name_required" }],
    ["password must contain 12–128 characters and at most 512 bytes", { password: "short" }, { key: "password_too_short", length: 5 }],
    ["password must contain 6–128 characters and at most 512 bytes", { password: "short" }, { key: "password_too_short", length: 5 }],
    ["Invalid username or password format", { password: "short" }, { key: "password_too_short", length: 5 }],
    ["Invalid username or password format", { password: "abc123" }, { key: "invalid_request" }],
    ["Invalid username or password format", {}, { key: "invalid_request" }],
  ])("recognizes older backend validation: %s", (message, values, expected) => {
    expect(passwordErrorDetails(failure("invalid_request", message), { ...input, ...values })).toEqual(expected);
  });

  it("prefers a modern code over a conflicting message", () => {
    expect(passwordErrorDetails(failure("username_taken", "name must contain 1–80 characters"), input)).toEqual({ key: "username_taken" });
  });

  it.each([null, "Bad request", { code: 12 }, { code: "future_code", error: "Do not display this" }])("keeps unknown API bodies localized: %j", (body) => {
    expect(passwordErrorDetails(new ApiError("Do not display this", 500, "Server Error", body), input)).toEqual({ key: "request_failed" });
  });

  it("preserves rate limiting and uncertain registration", () => {
    expect(passwordErrorDetails(new ApiError("Too many attempts", 429, "Too Many Requests"), input)).toEqual({ key: "rate_limited" });
    expect(passwordErrorDetails(new TypeError("Failed to fetch"), input)).toEqual({ key: "registration_uncertain" });
  });
});
