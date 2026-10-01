import { ApiError } from "@multica/core/api";
import type authMessages from "../locales/en/auth.json";

interface PasswordErrorInput {
  username: string;
  name: string;
  password: string;
  registering: boolean;
}

interface PasswordErrorDetails {
  key: keyof typeof authMessages.password;
  length?: number;
}

// Older installed servers report field validation under a single generic code.
const legacyValidationCodes = new Map([
  ["username must contain 3–32 ASCII characters", "invalid_username"],
  ["username must contain only letters, numbers or underscores", "invalid_username"],
  ["username must start with a letter and contain only letters, numbers or underscores", "username_rules_outdated"],
  ["name must contain 1–80 characters", "invalid_name"],
  ["password must contain 12–128 characters and at most 512 bytes", "invalid_password"],
  ["Password must contain 12–128 characters", "invalid_password"],
  ["password must contain 6–128 characters and at most 512 bytes", "invalid_password"],
  ["Password must contain 6–128 characters", "invalid_password"],
]);

function usernameError(username: string): PasswordErrorDetails {
  const value = username.trim();
  const length = Array.from(value).length;
  if (!length) return { key: "username_required" };
  if (!/^[A-Za-z0-9_]+$/.test(value)) return { key: "username_invalid_characters" };
  if (length < 3) return { key: "username_too_short", length };
  if (length > 32) return { key: "username_too_long", length };
  return { key: "invalid_username" };
}

export function passwordErrorDetails(error: unknown, input: PasswordErrorInput): PasswordErrorDetails {
  if (!(error instanceof ApiError)) {
    return { key: input.registering ? "registration_uncertain" : "connection_failed" };
  }
  if (error.status === 429) return { key: "rate_limited" };
  const body = error.body;
  let code = body && typeof body === "object" && "code" in body ? body.code : undefined;
  if (code === "invalid_request") {
    const message = body && typeof body === "object" && "error" in body && typeof body.error === "string" ? body.error : error.message;
    code = legacyValidationCodes.get(message) ?? code;
    if (message === "Invalid username or password format") {
      const passwordLength = Array.from(input.password).length;
      if (!/^[A-Za-z0-9_]{3,32}$/.test(input.username.trim())) code = "invalid_username";
      else if (passwordLength < 6 || passwordLength > 128) code = "invalid_password";
    }
  }
  switch (code) {
    case "username_rules_outdated": {
      const detail = usernameError(input.username);
      return detail.key === "invalid_username" && !/^[A-Za-z]/.test(input.username.trim()) ? { key: "username_rules_outdated" } : detail;
    }
    case "invalid_username":
      return usernameError(input.username);
    case "invalid_name": {
      const length = Array.from(input.name.trim()).length;
      if (!length) return { key: "name_required" };
      return length > 80 ? { key: "name_too_long", length } : { key: "invalid_name" };
    }
    case "invalid_password": {
      const length = Array.from(input.password).length;
      if (length < 6) return { key: "password_too_short", length };
      return length > 128 ? { key: "password_too_long", length } : { key: "invalid_password" };
    }
    case "username_taken":
    case "invalid_credentials":
    case "signup_disabled":
    case "invalid_request":
      return { key: code };
    case "auth_unavailable":
      return { key: "auth_unavailable" };
    default:
      return { key: "request_failed" };
  }
}
