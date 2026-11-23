// @vitest-environment node
import { describe, expect, it } from "vitest";
import { createI18n } from "@multica/core/i18n/react";
import { RESOURCES } from "@multica/views/locales";
import type { DaemonDiagnostic, DaemonDiagnosticCode } from "../../../shared/daemon-types";
import { daemonDiagnosticText } from "./daemon-diagnostics";

const codes: DaemonDiagnosticCode[] = [
  "challenge_expired", "challenge_time_window", "challenge_scope_mismatch", "challenge_invalid",
  "cli_unavailable", "cli_download_failed", "cli_checksum_failed", "cli_extract_failed", "cli_install_failed", "cli_validation_failed",
  "request_failed", "operation_failed",
];

describe.each(["en", "zh-Hans"] as const)("daemon diagnostic translations: %s", locale => {
  const t = createI18n(locale, RESOURCES).getFixedT(locale, "settings");
  it.each(codes)("provides a localized summary and ordered initial steps for %s", code => {
    const text = daemonDiagnosticText({ code, stage: "registration", occurredAt: 1 }, t);
    expect(text.summary).toBeTruthy();
    expect(text.summary).not.toContain("desktop.daemon");
    expect(text.steps.length).toBeGreaterThanOrEqual(2);
    expect(text.steps.every(step => step.length > 15 && !step.includes("desktop.daemon"))).toBe(true);
    if (locale === "zh-Hans") expect(text.summary).toMatch(/[\u4e00-\u9fff]/);
  });

  it("maps scope fields to safe names and ignores arbitrary values", () => {
    const diagnostic = JSON.parse('{"code":"challenge_scope_mismatch","stage":"binding","occurredAt":1,"fields":["account","private-token","binding_epoch"],"message":"private-token"}') as DaemonDiagnostic;
    const text = daemonDiagnosticText(diagnostic, t);
    expect(text.fields).toBe(locale === "en" ? "account / binding version" : "账号 / 绑定版本");
    expect(JSON.stringify(text)).not.toContain("private-token");
  });

  it("uses a safe fallback for missing or unknown categories and stages", () => {
    const unexpected = JSON.parse('{"code":"private-token","stage":"private-stage","occurredAt":1}') as DaemonDiagnostic;
    expect(daemonDiagnosticText(unexpected, t)).toEqual(daemonDiagnosticText(undefined, t));
    expect(daemonDiagnosticText(undefined, t, "cli_unavailable").code).toBe("cli_unavailable");
  });

  it.each([
    '{"code":{"toString":null},"stage":{},"fields":"private-value"}',
    '{"code":["challenge_expired"],"stage":null,"fields":42}',
    "null",
  ])("uses a safe fallback for malformed diagnostic data: %s", raw => {
    expect(daemonDiagnosticText(JSON.parse(raw), t)).toEqual(daemonDiagnosticText(undefined, t));
  });
});
