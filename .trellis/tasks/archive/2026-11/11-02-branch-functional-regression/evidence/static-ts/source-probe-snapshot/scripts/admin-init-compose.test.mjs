import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { readFileSync } from "node:fs";
import test from "node:test";
import { credentialsTarget, initializationPlan, initializationReceipt } from "./admin-init-compose.mjs";

const directory = '/opt/customer\'s $notes $${literal} "quoted",drive:1\\files';
const identity = "1001:1002";
const image = "registry.intranet:5000/multica-backend:v1.0.0";
const env = { MULTICA_AUTH_MODE: "password", MULTICA_PLATFORM_ADMIN_ENABLED: "true", DATABASE_URL: "private-database-secret", ALLOW_SIGNUP: "false" };
const config = () => ({ services: { backend: { image, environment: { ...env }, networks: { internal: null }, volumes: [{ type: "bind", source: "/srv/data", target: "/app/data" }] } } });

test("initialization override freezes the resolved image and adds only UID and literal deployment bind", () => {
  const source = config();
  const before = structuredClone(source);
  const plan = initializationPlan(source, directory, identity, image);
  assert.equal(plan.status, "initialize");
  assert.deepEqual(plan.overlay, { services: { backend: {
    image, user: identity,
    volumes: [{ type: "bind", source: directory.replace(/\$/g, () => "$$"), target: "/deployment-admin-init", bind: { create_host_path: false } }],
  } } });
  assert.deepEqual(source, before);
  assert.equal(JSON.stringify(plan).includes("private-database-secret"), false);
});

test("disabled settings skip before mount or image checks with backend-exact semantics", () => {
  assert.deepEqual(initializationPlan({ services: { backend: {} } }, directory, identity), { status: "skipped_mode_disabled" });
  for (const mode of ["legacy", "", undefined]) {
    const source = config();
    source.services.backend.environment.MULTICA_AUTH_MODE = mode;
    source.services.backend.image = null;
    assert.deepEqual(initializationPlan(source, directory, identity), { status: "skipped_mode_disabled" });
  }
  for (const enabled of ["false", "TRUE", " true ", true, undefined]) {
    const source = config();
    source.services.backend.environment.MULTICA_PLATFORM_ADMIN_ENABLED = enabled;
    assert.deepEqual(initializationPlan(source, directory, identity), { status: "skipped_admin_disabled" });
  }
  const source = config();
  source.services.backend.environment.MULTICA_AUTH_MODE = " password ";
  assert.equal(initializationPlan(source, directory, identity).status, "initialize");
});

test("conflicting reserved mounts, malformed paths and image changes fail closed", () => {
  for (const path of ["relative", "/", "/opt/../tmp", "/opt/./tmp", "/opt/trailing/", "/opt/line\n", "/opt/line\r", "/opt/tab\t", "/opt/null\0"]) {
    assert.throws(() => initializationPlan(config(), path, identity), /invalid_deployment_path/);
  }
  for (const target of ["/deployment-admin-init", "/deployment-admin-init/", "/deployment-admin-init/admin-initial-password.txt", "/deployment-admin-init/../deployment-admin-init", "/"]) {
    const source = config();
    source.services.backend.volumes.push({ type: "bind", source: "/unknown", target });
    assert.throws(() => initializationPlan(source, directory, identity), /reserved_mount_conflict/, target);
  }
  assert.throws(() => initializationPlan(config(), directory, identity, "old:v0"), /backend_image_mismatch/);
  assert.throws(() => initializationPlan(config(), directory, "operator"), /invalid_operator_identity/);
  assert.throws(() => initializationPlan({}, directory, identity), /invalid_backend_configuration/);
});

test("receipt reports a host path only for delivered credentials and rejects secret or malformed output", () => {
  const receipt = { schema_version: 1, status: "initialized", credentials_file: credentialsTarget, user_id: "eeeeeeee-1111-2222-3333-444444444444" };
  assert.deepEqual(initializationReceipt(receipt, directory), { ...receipt, credentials_file: directory + "/admin-initial-password.txt" });
  assert.deepEqual(initializationReceipt({ schema_version: 1, status: "already_initialized" }, directory), { schema_version: 1, status: "already_initialized" });
  assert.equal(initializationReceipt({ ...receipt, status: "already_initialized" }, directory).credentials_file, directory + "/admin-initial-password.txt");
  for (const invalid of [
    { ...receipt, password: "private-initial-password" },
    { ...receipt, schema_version: 2 },
    { ...receipt, credentials_file: "/unexpected/file" },
    { ...receipt, credentials_file: undefined },
    { ...receipt, status: "preserved_existing_admin" },
    { ...receipt, user_id: "private-initial-password" },
    { schema_version: 1, status: "credentials_unavailable" },
  ]) assert.throws(() => initializationReceipt(invalid, directory), /invalid_initialization_receipt/);
});

test("container parser bounds input and never echoes malformed or unexpected secret values", () => {
  const source = readFileSync(new URL("./admin-init-compose.mjs", import.meta.url), "utf8");
  for (const input of ["private-initial-password", JSON.stringify({ schema_version: 1, status: "initialized", password: "private-initial-password" }), "x".repeat(16385)]) {
    const result = spawnSync(process.execPath, ["--input-type=module", "-e", source, "admin-init-compose", "receipt", directory], { input, encoding: "utf8" });
    assert.notEqual(result.status, 0);
    assert.equal(result.stdout, "");
    assert.equal(result.stderr.includes("private-initial-password"), false);
  }
});
