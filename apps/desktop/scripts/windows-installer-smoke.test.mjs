// @vitest-environment node
import { existsSync, readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

const repoRoot = [process.cwd(), resolve(process.cwd(), "../..")].find((root) =>
  existsSync(resolve(root, ".github/workflows/desktop-smoke.yml")),
);
const workflow = readFileSync(resolve(repoRoot, ".github/workflows/desktop-smoke.yml"), "utf8");
const defaultWorkflow = workflow.split("\n  windows-acceptance:")[0];
const steps = [...defaultWorkflow.matchAll(/^ {6}- name: (.+)$/gm)].map((match, index, matches) => ({
  name: match[1],
  body: defaultWorkflow.slice(match.index, matches[index + 1]?.index ?? defaultWorkflow.length),
}));

// The PowerShell script itself is the native Windows acceptance test. These
// checks protect its workflow gate and diagnostic delivery on every platform.
describe("Windows installer smoke workflow", () => {
  it("gates desktop artifact upload on the bounded Windows-only native check", () => {
    const packageIndex = steps.findIndex(({ body }) => body.includes("node scripts/package.mjs"));
    const smokeIndex = steps.findIndex(({ body }) => body.includes("scripts/smoke-windows-installer.ps1"));
    const uploadIndex = steps.findIndex(({ body }) => body.includes("path: apps/desktop/dist"));
    expect(smokeIndex).toBeGreaterThan(packageIndex);
    expect(uploadIndex).toBeGreaterThan(smokeIndex);
    const smoke = steps[smokeIndex].body;
    expect(smoke).toMatch(/^\s+if: matrix\.target == 'win'$/m);
    expect(smoke).toMatch(/^\s+shell: pwsh$/m);
    const timeout = Number(smoke.match(/^\s+timeout-minutes: (\d+)$/m)?.[1]);
    expect(timeout).toBeGreaterThan(0);
    expect(timeout).toBeLessThanOrEqual(15);
    expect(smoke).toContain("deriveVersion");
    expect(smoke).toContain("-ExpectedVersion");
    expect(smoke).not.toContain("continue-on-error: true");
    expect(steps[uploadIndex].body).not.toMatch(/if:.*(?:always\(\)|failure\(\))/);
    expect(existsSync(resolve(repoRoot, "apps/desktop/scripts/smoke-windows-installer.ps1"))).toBe(true);
  });

  it.each(["x64", "ia32"])("uploads the %s report even when the native check fails", (arch) => {
    const diagnostics = steps.find(({ body }) => body.includes(`name: windows-${arch}-installer-verification`));
    expect(diagnostics).toBeDefined();
    expect(diagnostics.body).toContain("uses: actions/upload-artifact@v4");
    expect(diagnostics.body).toMatch(/if:.*always\(\).*matrix\.target == 'win'/);
    const report = `multica-windows-${arch}-installer-smoke.json`;
    expect(diagnostics.body).toContain(`path: \${{ runner.temp }}/${report}`);
    const smoke = steps.find(({ body }) => body.includes("scripts/smoke-windows-installer.ps1"));
    expect(smoke.body).toContain(`-ExpectedArch ${arch}`);
    expect(smoke.body).toContain(`-ReportPath "$env:RUNNER_TEMP/${report}"`);
  });

  it("builds Windows architectures in one invocation so a later cleanup cannot erase a checked installer", () => {
    const packaging = steps.filter(({ body }) => body.includes("node scripts/package.mjs"));
    expect(packaging).toHaveLength(1);
    expect(packaging[0].body).toContain("${{ matrix.architectures }}");
    expect(workflow).toMatch(/target: win\n\s+architectures: --x64 --ia32 --arm64/);
    expect(workflow).toMatch(/target: linux\n\s+architectures: --x64 --arm64\s*\n/);
    expect(packaging[0].body).toContain("--publish never");
  });

  it("validates the installed application and bundled CLI against the requested architecture", () => {
    const script = readFileSync(resolve(repoRoot, "apps/desktop/scripts/smoke-windows-installer.ps1"), "utf8");
    expect(script).toContain("[ValidateSet('x64', 'ia32')][string]$ExpectedArch = 'x64'");
    expect(script).toContain("0x014c");
    expect(script).toContain("'386'");
    expect(script).toContain("'amd64'");
    expect(script).toContain("-ne $expectedPeMachine");
    expect(script).toContain("$cliVersion['arch'] -cne $expectedGoArch");
    expect(script).toContain("expected_arch = $ExpectedArch");
  });
});


describe("Windows candidate acceptance workflow", () => {
  const job = workflow.split("\n  windows-acceptance:")[1];
  it("is explicitly selected, Windows native, and never publishes a release", () => {
    expect(job).toContain("if: inputs.windows_acceptance == true");
    expect(job).toContain("runs-on: windows-latest");
    expect(job).toContain("--win --x64 --publish never");
    expect(job).not.toContain("--publish always");
    expect(job).not.toContain("continue-on-error: true");
    expect(defaultWorkflow).toContain("if: inputs.windows_acceptance != true");
  });
  it("verifies a pinned previous release and refuses silent native-test skips", () => {
    expect(job).toContain("Previous installer checksum mismatch");
    expect(job).toContain("-Algorithm SHA256");
    expect(job).toContain("verify-go-test-events.mjs");
    expect(job).toContain("TestManagedIdentityWindowsProfileACL");
    expect(job).toContain("TestManagedConnectionDetachFailsPendingRPCAndPreservesOtherTransport");
    expect(job).toContain("-PreviousInstallerPath $previous");
    expect(job).toContain("-RequireSigned:([bool]::Parse($env:REQUIRE_SIGNED))");
  });
  it("uploads failure diagnostics but only accepts candidate artifacts after lifecycle success", () => {
    expect(job).toMatch(/name: Upload native Windows evidence\n\s+if: always\(\)/);
    expect(job).toMatch(/name: Upload accepted Windows candidate\n\s+uses: actions\/upload-artifact@v4/);
    expect(job.indexOf("name: Upload accepted Windows candidate")).toBeGreaterThan(job.indexOf("name: Accept Windows installer lifecycle"));
  });
});
