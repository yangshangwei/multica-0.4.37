import type { useT } from "@multica/views/i18n";
import type { ChallengeMismatchField, DaemonDiagnostic, DaemonDiagnosticCode } from "../../../shared/daemon-types";

export type DaemonSettingsT = ReturnType<typeof useT<"settings">>["t"];

const DIAGNOSTIC_STEPS = {
  challenge_expired: ["clock", "fresh_challenge", "server_clock"],
  challenge_time_window: ["clock", "fresh_challenge", "server_clock"],
  challenge_scope_mismatch: ["scope", "fresh_challenge", "support"],
  challenge_invalid: ["scope", "fresh_challenge", "support"],
  cli_unavailable: ["executable", "retry_setup", "support"],
  cli_download_failed: ["network", "retry_setup", "support"],
  cli_checksum_failed: ["checksum", "retry_setup", "support"],
  cli_extract_failed: ["extract", "retry_setup", "support"],
  cli_install_failed: ["permissions", "retry_setup", "support"],
  cli_validation_failed: ["executable", "retry_setup", "support"],
  request_failed: ["retry", "support"],
  operation_failed: ["retry", "support"],
} as const satisfies Record<DaemonDiagnosticCode, readonly string[]>;
const FIELDS: readonly ChallengeMismatchField[] = ["request", "deployment", "organization", "account", "auth_version", "installation", "workspace", "daemon", "binding_epoch", "key"];
const STAGES: readonly DaemonDiagnostic["stage"][] = ["cli_setup", "registration", "binding", "credentials", "restart"];

/** Only finite categories are translated. Unknown IPC data never becomes copy. */
export function daemonDiagnosticText(diagnostic: DaemonDiagnostic | undefined, t: DaemonSettingsT, fallback: DaemonDiagnosticCode = "operation_failed") {
  const code = typeof diagnostic?.code === "string" && Object.hasOwn(DIAGNOSTIC_STEPS, diagnostic.code) ? diagnostic.code : fallback;
  const stage = diagnostic && STAGES.includes(diagnostic.stage) ? diagnostic.stage : fallback === "cli_unavailable" ? "cli_setup" : "restart";
  const fields = Array.isArray(diagnostic?.fields) ? FIELDS.filter(field => diagnostic.fields!.includes(field)) : [];
  return {
    code,
    summary: t($ => $.desktop.daemon.diagnostic.summary[code]),
    stage: t($ => $.desktop.daemon.diagnostic.stages[stage]),
    fields: fields.map(field => t($ => $.desktop.daemon.diagnostic.fields_labels[field])).join(" / "),
    steps: DIAGNOSTIC_STEPS[code].map(key => t($ => $.desktop.daemon.diagnostic.steps[key])),
  };
}
