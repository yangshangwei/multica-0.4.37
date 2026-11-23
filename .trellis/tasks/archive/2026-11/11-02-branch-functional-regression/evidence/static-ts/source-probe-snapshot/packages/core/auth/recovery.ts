import { ApiError } from "../api/client";

export interface ConnectionFailure {
  kind: "timeout" | "network" | "server" | "unknown";
  occurredAt: number;
  status?: number;
}

export interface AuthRecoveryState {
  isRetrying: boolean;
  nextRetryAt: number | null;
  failure: ConnectionFailure | null;
}

/** Recovery details are an allowlist; never carry raw bodies or error text. */
export function classifyConnectionFailure(error: unknown, occurredAt = Date.now()): ConnectionFailure {
  if ((error instanceof Error || error instanceof DOMException) && error.name === "TimeoutError") return { kind: "timeout", occurredAt };
  if (error instanceof TypeError) return { kind: "network", occurredAt };
  if (error instanceof ApiError) {
    return { kind: error.status >= 500 ? "server" : "unknown", occurredAt, status: error.status };
  }
  return { kind: "unknown", occurredAt };
}
