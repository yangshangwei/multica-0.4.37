// @vitest-environment node
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { createHash, createPublicKey, verify } from "node:crypto";
import { chmod, mkdtemp, readFile, rm, stat, symlink, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { loadManagedInstallation, type ChallengeScope } from "./managed-installation";

const deploymentId = "11111111-1111-4111-8111-111111111111";
const installationId = "22222222-2222-4222-8222-222222222222";
const userId = "33333333-3333-4333-8333-333333333333";
const organizationId = "44444444-4444-4444-8444-444444444444";
let home: string;
beforeEach(async () => { home = await mkdtemp(join(tmpdir(), "managed-installation-")); });
afterEach(async () => { await rm(home, { recursive: true, force: true }); });

function challenge(fingerprint: string, publicKey: string) {
  const body = Buffer.from(JSON.stringify({ public_key: publicKey, desktop_version: "test", os: "linux" }));
  const payload = {
    protocol_version: "1", purpose: "enroll", challenge_id: "55555555-5555-4555-8555-555555555555",
    deployment_id: deploymentId, organization_id: organizationId, user_id: userId,
    auth_version: "1", installation_id: null, workspace_id: null, daemon_id: null,
    public_key_fingerprint: fingerprint, expected_binding_epoch: null,
    nonce: Buffer.alloc(32, 1).toString("base64url"), expires_at: "2000000060",
    method: "POST", path: "/api/installations/enroll", body_sha256: createHash("sha256").update(body).digest("hex"),
  };
  const scope: ChallengeScope = {
    purpose: "enroll", deploymentId, organizationId, userId, authVersion: "1",
    installationId: null, workspaceId: null, daemonId: null, expectedBindingEpoch: null,
    method: "POST", path: "/api/installations/enroll", bodyPayload: body.toString("base64url"),
  };
  return { response: { challenge_id: payload.challenge_id, nonce: payload.nonce, deployment_id: deploymentId, expires_at: payload.expires_at, signature_payload: Buffer.from(JSON.stringify(payload)).toString("base64url"), body_payload: body.toString("base64url") }, scope, payload };
}

describe("managed installation storage", () => {
  it("persists one key across profiles and concurrent creation; separates homes and deployments", async () => {
    const identities = await Promise.all(Array.from({ length: 8 }, () => loadManagedInstallation(home, deploymentId)));
    expect(new Set(identities.map((identity) => identity.publicInfo.publicKey)).size).toBe(1);
    expect((await stat(join(home, ".multica/management", deploymentId, "installation.json"))).mode & 0o777).toBe(0o600);
    await identities[0]!.saveEnrollment(installationId, "1");
    expect((await loadManagedInstallation(home, deploymentId)).publicInfo.installationId).toBe(installationId);
    const other = await loadManagedInstallation(home, "66666666-6666-4666-8666-666666666666");
    expect(other.publicInfo.publicKey).not.toBe(identities[0]!.publicInfo.publicKey);
    expect(JSON.stringify(identities[0])).not.toContain("private_key_seed");
  });

  it("loads and signs the shared Node/Go protocol fixture", async () => {
    await loadManagedInstallation(home, deploymentId);
    const fixtureRoot = resolve(__dirname, "../../../../server/internal/daemon/testdata");
    const path = join(home, ".multica/management", deploymentId, "installation.json");
    await writeFile(path, await readFile(join(fixtureRoot, "managed-identity-v1.json")), { mode: 0o600 });
    const fixture = JSON.parse(await readFile(join(fixtureRoot, "managed-challenge-v1.json"), "utf8"));
    const identity = await loadManagedInstallation(home, deploymentId);
    const { scope } = challenge(identity.publicInfo.fingerprint, identity.publicInfo.publicKey);
    expect(identity.signChallenge(fixture.challenge, scope, 2_000_000_000).signature).toBe(fixture.signature);
    const vector = JSON.parse(await readFile(join(fixtureRoot, "managed-daemon-id-v1.json"), "utf8"));
    expect(identity.publicInfo.daemonNamespaceId).toBe(vector.namespace);
    expect(identity.managedDaemonId(vector.user_id)).toBe(vector.daemon_id);
    expect(identity.managedDaemonId(organizationId)).not.toBe(vector.daemon_id);
  });

  it("does not replace corrupt, insecure or symlinked private identity data", async () => {
    await loadManagedInstallation(home, deploymentId);
    const path = join(home, ".multica/management", deploymentId, "installation.json");
    await writeFile(path, "corrupt");
    await expect(loadManagedInstallation(home, deploymentId)).rejects.toThrow();
    expect(await readFile(path, "utf8")).toBe("corrupt");
    await rm(path);
    await symlink(join(home, "outside.json"), path);
    await expect(loadManagedInstallation(home, deploymentId)).rejects.toThrow();
    await rm(path);
    await loadManagedInstallation(home, deploymentId);
    await chmod(path, 0o644);
    await expect(loadManagedInstallation(home, deploymentId)).rejects.toThrow();
  });

  it("does not overwrite a different confirmed server installation", async () => {
    const first = await loadManagedInstallation(home, deploymentId);
    const second = await loadManagedInstallation(home, deploymentId);
    await first.saveEnrollment(installationId, "1");
    await expect(second.saveEnrollment("66666666-6666-4666-8666-666666666666", "1")).rejects.toThrow();
  });
});

describe("challenge proof", () => {
  it("signs the original validated bytes without exposing its private seed", async () => {
    const identity = await loadManagedInstallation(home, deploymentId);
    const { response, scope } = challenge(identity.publicInfo.fingerprint, identity.publicInfo.publicKey);
    const proof = identity.signChallenge(response, scope, 2_000_000_000);
    const publicKey = createPublicKey({ key: Buffer.concat([Buffer.from("302a300506032b6570032100", "hex"), Buffer.from(identity.publicInfo.publicKey, "base64url")]), format: "der", type: "spki" });
    expect(verify(null, Buffer.from(response.signature_payload, "base64url"), publicKey, Buffer.from(proof.signature, "base64url"))).toBe(true);
    expect(proof).toMatchObject({ challenge_id: response.challenge_id, signature_payload: response.signature_payload, body_payload: response.body_payload });
  });

  it("rejects wrong purpose, principal, epoch, body hash, duplicate keys, and expiry", async () => {
    const identity = await loadManagedInstallation(home, deploymentId);
    const { response, scope, payload } = challenge(identity.publicInfo.fingerprint, identity.publicInfo.publicKey);
    for (const change of [{ purpose: "bind" }, { user_id: organizationId }, { auth_version: "2" }, { body_sha256: "a".repeat(64) }, { expires_at: "1999999999" }]) {
      const tampered = { ...response, signature_payload: Buffer.from(JSON.stringify({ ...payload, ...change })).toString("base64url") };
      expect(() => identity.signChallenge(tampered, scope, 2_000_000_000)).toThrow();
    }
    const duplicate = JSON.stringify(payload).replace('"purpose":"enroll"', '"purpose":"bind","purpose":"enroll"');
    expect(() => identity.signChallenge({ ...response, signature_payload: Buffer.from(duplicate).toString("base64url") }, scope, 2_000_000_000)).toThrow();
  });
});
