// @vitest-environment node
import { expect, it } from "vitest";
import { managementAuthVersion, matchesInstallationMetadata } from "./managed-installation";

const userId = "11111111-1111-4111-8111-111111111111";
const token = (payload: string) => `header.${Buffer.from(payload).toString("base64url")}.signature`;

it("reads the exact decimal JWT version without treating it as authentication", () => {
  expect(managementAuthVersion(token(`{"sub":"${userId}","auth_version":1}`), userId)).toBe("1");
  expect(managementAuthVersion(token(`{"sub":"${userId}","auth_version":9007199254740993}`), userId)).toBe("9007199254740993");
  expect(managementAuthVersion(token(`{"sub":"another","auth_version":1}`), userId)).toBeNull();
  expect(managementAuthVersion("mul_personal-token", userId)).toBeNull();
});

it("rejects metadata delivered for a stale user, server or version", () => {
  const scope = { serverUrl: "https://example.test", userId, authVersion: "2" };
  const metadata = { ...scope, proof: "mip_test-only-proof" };
  expect(matchesInstallationMetadata(metadata, scope)).toBe(true);
  for (const update of [{ serverUrl: "https://other.test" }, { userId: "another" }, { authVersion: "1" }, { proof: "mdt_not-metadata" }]) {
    expect(matchesInstallationMetadata({ ...metadata, ...update }, scope)).toBe(false);
  }
});
