// Pure Compose/receipt boundary shared by the offline helper and its tests.
import { posix } from "node:path";

export const credentialsTarget = "/deployment-admin-init/admin-initial-password.txt";
const mountTarget = posix.dirname(credentialsTarget);
const statuses = new Set(["initialized", "already_initialized", "preserved_existing_admin", "skipped_mode_disabled", "skipped_admin_disabled"]);
const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

function requireValue(condition, code) {
  if (!condition) throw new Error(code);
}

function absolutePath(value) {
  requireValue(typeof value === "string" && value.startsWith("/") && !/[\x00-\x1f\x7f]/.test(value), "invalid_deployment_path");
  requireValue(!value.endsWith("/") && posix.normalize(value) === value, "invalid_deployment_path");
}

export function initializationPlan(config, directory, user, expectedImage = "") {
  absolutePath(directory);
  requireValue(/^\d+:\d+$/.test(user), "invalid_operator_identity");
  const backend = config?.services?.backend;
  requireValue(backend && typeof backend === "object" && !Array.isArray(backend), "invalid_backend_configuration");
  const env = backend.environment ?? {};
  requireValue(typeof env === "object" && !Array.isArray(env), "invalid_backend_configuration");
  if (String(env.MULTICA_AUTH_MODE ?? "").trim() !== "password") return { status: "skipped_mode_disabled" };
  if (env.MULTICA_PLATFORM_ADMIN_ENABLED !== "true") return { status: "skipped_admin_disabled" };

  requireValue(typeof backend.image === "string" && /^[^\s\x00-\x1f\x7f$]+$/.test(backend.image), "invalid_backend_image");
  requireValue(!expectedImage || backend.image === expectedImage, "backend_image_mismatch");
  requireValue(backend.volumes === undefined || Array.isArray(backend.volumes), "invalid_backend_volumes");
  for (const volume of backend.volumes ?? []) {
    requireValue(typeof volume?.target === "string" && volume.target.startsWith("/"), "invalid_backend_volumes");
    const target = posix.normalize(volume.target);
    requireValue(target !== mountTarget && !target.startsWith(mountTarget + "/") && !mountTarget.startsWith(target.replace(/\/$/, "") + "/"), "reserved_mount_conflict");
  }
  return {
    status: "initialize",
    overlay: {
      services: {
        backend: {
          image: backend.image,
          user,
          volumes: [{ type: "bind", source: directory.replace(/\$/g, () => "$$"), target: mountTarget, bind: { create_host_path: false } }],
        },
      },
    },
  };
}

export function initializationReceipt(receipt, directory) {
  absolutePath(directory);
  requireValue(receipt?.schema_version === 1 && statuses.has(receipt.status), "invalid_initialization_receipt");
  const result = { schema_version: 1, status: receipt.status };
  for (const key of ["organization_id", "user_id", "initialization_id"]) {
    if (receipt[key] !== undefined) {
      requireValue(typeof receipt[key] === "string" && uuid.test(receipt[key]), "invalid_initialization_receipt");
      result[key] = receipt[key];
    }
  }
  if (receipt.credentials_file !== undefined) {
    requireValue(["initialized", "already_initialized"].includes(receipt.status) && receipt.credentials_file === credentialsTarget, "invalid_initialization_receipt");
    result.credentials_file = posix.join(directory, "admin-initial-password.txt");
  }
  requireValue(receipt.status !== "initialized" || result.credentials_file !== undefined, "invalid_initialization_receipt");
  // Never relay unknown fields or raw output from the maintenance command.
  requireValue(Object.keys(receipt).every((key) => Object.hasOwn(result, key)), "invalid_initialization_receipt");
  return result;
}

async function main([mode, directory, user, expectedImage]) {
  requireValue(mode === "plan" || mode === "receipt", "invalid_helper_command");
  let input = "";
  process.stdin.setEncoding("utf8");
  for await (const chunk of process.stdin) {
    input += chunk;
    requireValue(Buffer.byteLength(input) <= (mode === "plan" ? 2 * 1024 * 1024 : 16384), "input_size_limit");
  }
  let parsed;
  try { parsed = JSON.parse(input); } catch { throw new Error("invalid_json"); }
  if (mode === "plan") {
    const plan = initializationPlan(parsed, directory, user, expectedImage);
    process.stdout.write(plan.status + "\n" + (plan.overlay ? JSON.stringify(plan.overlay) : ""));
  } else {
    process.stdout.write(JSON.stringify(initializationReceipt(parsed, directory)) + "\n");
  }
}

if (process.argv[1] === "admin-init-compose") {
  main(process.argv.slice(2)).catch((error) => {
    console.error("ERROR: administrator initialization: " + error.message);
    process.exitCode = 1;
  });
}
