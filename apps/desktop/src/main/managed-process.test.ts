// @vitest-environment node
import { execFile } from "node:child_process";
import { inspect } from "node:util";
import { afterEach, describe, expect, it, vi } from "vitest";
import { managementProcessIdentity, parseWindowsIdentity } from "./managed-process";

vi.mock("node:child_process", () => ({ execFile: vi.fn() }));

const HOST = "E9F42A64-BF8E-4739-A7CD-3AFE6EBD3AD1";
const BOOT = "2026-10-28T01:02:03.4567890Z";
const MARKED = `MULTICA_IDENTITY:{"host":"${HOST}","boot":"${BOOT}"}`;
// Pinned byte for byte in server/internal/daemon/managed_process_test.go too.
const COMMAND = "$o=Get-CimInstance Win32_OperatingSystem;$h=(Get-ItemProperty 'HKLM:\\SOFTWARE\\Microsoft\\Cryptography').MachineGuid;'MULTICA_IDENTITY:'+(@{host=$h;boot=$o.LastBootUpTime.ToUniversalTime().ToString('o')}|ConvertTo-Json -Compress)";
// Written to stdout by a DLP hook DLL injected into powershell.exe on an affected intranet host.
const HOOK_LINE = "[0x7FFF1DA6F990] ANOMALY: meaningless REX prefix used";
// server/internal/daemon/managed_process_test.go pins the same digests: lock owner
// recovery compares host and boot IDs written by both processes.
const EXPECTED = {
  hostId: "d2229f218bb4a985010fa17309d8f6d745c4c62f2cfcfe45a7dc97402f8a20dd",
  bootId: "539f8d036b71470fea1fd61b4ef852fe043661af4ae70451c7953fc35b083a1e",
};

describe("parseWindowsIdentity", () => {
  it.each([
    ["clean output", `${MARKED}\r\n`],
    ["a hook line after the identity", `${MARKED}\r\n${HOOK_LINE}\r\n`],
    ["a hook line before the identity", `${HOOK_LINE}\r\n${MARKED}\r\n`],
    ["LF line endings", `${HOOK_LINE}\n${MARKED}\n${HOOK_LINE}\n`],
  ])("reads the marked line from %s", (_name, stdout) => {
    expect(parseWindowsIdentity(stdout)).toEqual(EXPECTED);
  });

  it.each([
    ["unmarked JSON", `{"host":"${HOST}","boot":"${BOOT}"}\r\n`],
    ["two marked lines", `${MARKED}\r\n${MARKED}\r\n`],
    ["malformed JSON", `MULTICA_IDENTITY:{"host":"${HOST}"\r\n`],
    ["a missing boot time", `MULTICA_IDENTITY:{"host":"${HOST}","boot":""}\r\n`],
    ["no output", ""],
  ])("rejects %s without echoing stdout", (_name, stdout) => {
    let message = "";
    try { parseWindowsIdentity(stdout); } catch (error) { message = (error as Error).message; }
    expect(message).toBe("OS boot identity unavailable");
  });
});

describe("managementProcessIdentity on Windows", () => {
  const platform = Object.getOwnPropertyDescriptor(process, "platform")!;
  afterEach(() => { Object.defineProperty(process, "platform", platform); vi.mocked(execFile).mockReset(); });

  it("retries after a failed PowerShell start and caches the first success", async () => {
    Object.defineProperty(process, "platform", { value: "win32", configurable: true });
    // A real execFile timeout keeps whatever PowerShell already printed on the error.
    const timedOut = Object.assign(new Error("Command failed"), { killed: true, signal: "SIGTERM", stdout: `${MARKED}\r\n`, stderr: "" });
    // promisify(execFile) resolves to the callback's single value; the real execFile
    // supplies { stdout, stderr } through util.promisify.custom instead.
    vi.mocked(execFile)
      .mockImplementationOnce(((...args: unknown[]) => { (args.at(-1) as (error: Error) => void)(timedOut); }) as unknown as typeof execFile)
      .mockImplementation(((...args: unknown[]) => { (args.at(-1) as (error: null, value: object) => void)(null, { stdout: `${MARKED}\r\n${HOOK_LINE}\r\n`, stderr: "" }); }) as unknown as typeof execFile);

    const failure = await managementProcessIdentity().then(() => undefined, (error: unknown) => error);
    expect(failure).toEqual(new Error("OS boot identity unavailable"));
    // daemon-manager logs this error; the raw MachineGuid must not reach the log.
    expect(inspect(failure)).not.toContain(HOST);
    await expect(managementProcessIdentity()).resolves.toEqual(EXPECTED);
    await expect(managementProcessIdentity()).resolves.toEqual(EXPECTED);
    expect(execFile).toHaveBeenCalledTimes(2);
    expect(vi.mocked(execFile).mock.calls[1]?.[1]).toEqual(["-NoProfile", "-NonInteractive", "-Command", COMMAND]);
  });
});
