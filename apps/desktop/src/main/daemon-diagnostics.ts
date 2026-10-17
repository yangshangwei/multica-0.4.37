import type { ChallengeMismatchField, DaemonDiagnostic, DaemonDiagnosticCode } from "../shared/daemon-types";

/** Internal source error. Only the explicit projection below crosses IPC. */
export class DaemonDiagnosticError extends Error {
  constructor(
    readonly code: DaemonDiagnosticCode,
    readonly stage: DaemonDiagnostic["stage"],
    readonly fields?: ChallengeMismatchField[],
  ) {
    super(code);
    this.name = "DaemonDiagnosticError";
  }
}

export function daemonDiagnostic(error: unknown, stage: DaemonDiagnostic["stage"]): DaemonDiagnostic {
  const httpStatus = error && typeof error === "object" && "status" in error &&
    typeof error.status === "number" && Number.isInteger(error.status) && error.status >= 400 && error.status <= 599
    ? error.status : undefined;
  if (error instanceof DaemonDiagnosticError) {
    return { code: error.code, stage: error.stage, occurredAt: Date.now(), ...(error.fields ? { fields: [...error.fields] } : {}) };
  }
  return {
    code: httpStatus ? "request_failed" : "operation_failed", stage, occurredAt: Date.now(),
    ...(httpStatus ? { httpStatus } : {}),
  };
}
