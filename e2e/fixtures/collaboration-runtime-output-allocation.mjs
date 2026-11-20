import { createHash } from "node:crypto";
import { lstatSync, readFileSync, realpathSync } from "node:fs";
import { dirname, isAbsolute, join, relative, resolve } from "node:path";

const taskIdentity = "full-e2e-regression-20261120";
const taskPath = /^\.trellis\/tasks\/(?:archive\/\d{4}-(?:0[1-9]|1[0-2])\/)?11-20-full-e2e-regression-20261120$/;
const historicalState = "/Users/artisan/.multica/acceptance/pay241-s3-output-20261119-tm89sjgy";
const sha256 = (bytes) => createHash("sha256").update(bytes).digest("hex");
const explicitKeys = ["COLLABORATION_OUTPUT_TASK", "COLLABORATION_OUTPUT_STATE_ROOT",
  "COLLABORATION_OUTPUT_ALLOCATION_FILE", "COLLABORATION_OUTPUT_ALLOCATION_SHA256"];

function requireCondition(condition, message) {
  if (!condition) throw new Error(`Output allocation: ${message}`);
}

function descendant(parent, path) {
  const part = relative(parent, path);
  return part !== "" && part !== ".." && !part.startsWith(`..${process.platform === "win32" ? "\\" : "/"}`) && !isAbsolute(part);
}

function canonical(path, io) {
  requireCondition(typeof path === "string" && isAbsolute(path) && resolve(path) === path,
    "an absolute canonical path is required");
  requireCondition(io.realpathSync(path) === path, "a path resolves outside its recorded identity");
  for (let part = path; ; part = dirname(part)) {
    requireCondition(!io.lstatSync(part).isSymbolicLink(), "symlink path segments are not permitted");
    if (dirname(part) === part) break;
  }
  return path;
}

function privatePath(path, io, directory) {
  canonical(path, io);
  const stat = io.lstatSync(path);
  requireCondition(directory ? stat.isDirectory() : stat.isFile(), "private state has the wrong file type");
  requireCondition(typeof process.getuid === "function" && stat.uid === process.getuid()
    && (stat.mode & 0o777) === (directory ? 0o700 : 0o600), "private state ownership or permissions differ");
}

function origin(value) {
  const url = new URL(value);
  requireCondition(["http:", "https:"].includes(url.protocol) && ["localhost", "127.0.0.1", "[::1]"].includes(url.hostname)
    && !url.username && !url.password && !url.search && !url.hash && url.pathname === "/" && url.port !== "0",
  "service origins must be credential-free loopback endpoints");
  return url.origin;
}

/** Read-only admission runs before gate writes, registration, storage reads or daemon launch. */
export function assertRuntimeOutputAllocation(env, checkout, io = { lstatSync, readFileSync, realpathSync }) {
  const databaseUrl = new URL(env.DATABASE_URL);
  const database = decodeURIComponent(databaseUrl.pathname.slice(1));
  requireCondition(["postgres:", "postgresql:"].includes(databaseUrl.protocol)
    && ["localhost", "127.0.0.1", "[::1]"].includes(databaseUrl.hostname) && databaseUrl.port !== "0"
    && !databaseUrl.search && !databaseUrl.hash
    && database === env.TASK_DATABASE_NAME && /^[a-zA-Z0-9_]+$/.test(database)
    && !["multica", "postgres", "template0", "template1"].includes(database), "database identity is not an isolated loopback database");

  if (!explicitKeys.some((key) => env[key] !== undefined)) {
    requireCondition(/^multica_pay241_s3o_20261119_tm89sjgy_[a-z0-9_]+$/.test(database), "historical database identity differs");
    const storage = env.COLLABORATION_OUTPUT_STORAGE_ROOT;
    requireCondition(typeof storage === "string" && isAbsolute(storage)
      && io.realpathSync(storage).startsWith(`${historicalState}/`)
      && io.realpathSync(storage) === io.realpathSync(env.LOCAL_UPLOAD_DIR), "historical storage identity differs");
    return { mode: "historical", database, storage_root: io.realpathSync(storage) };
  }

  requireCondition(explicitKeys.every((key) => typeof env[key] === "string" && env[key].length > 0), "explicit allocation is incomplete");
  const task = env.COLLABORATION_OUTPUT_TASK;
  requireCondition(taskPath.test(task), "the task is not the current acceptance task");
  canonical(checkout, io);
  const taskDirectory = canonical(join(checkout, task), io);
  requireCondition(io.lstatSync(taskDirectory).isDirectory(), "the current task directory is missing");
  const taskFilename = canonical(join(taskDirectory, "task.json"), io);
  requireCondition(io.lstatSync(taskFilename).isFile(), "task metadata must be a regular file");
  const taskMetadata = JSON.parse(io.readFileSync(taskFilename).toString("utf8"));
  requireCondition(taskMetadata !== null && typeof taskMetadata === "object" && !Array.isArray(taskMetadata)
    && taskMetadata.id === taskIdentity && taskMetadata.name === taskIdentity, "task metadata identifies a different task");
  const state = env.COLLABORATION_OUTPUT_STATE_ROOT;
  privatePath(state, io, true);
  requireCondition(state !== checkout && state !== dirname(state) && !descendant(checkout, state), "state must be an external private allocation");
  const filename = env.COLLABORATION_OUTPUT_ALLOCATION_FILE;
  requireCondition(descendant(state, filename), "allocation record must belong to the private state");
  privatePath(filename, io, false);
  const bytes = io.readFileSync(filename);
  requireCondition(/^[a-f0-9]{64}$/.test(env.COLLABORATION_OUTPUT_ALLOCATION_SHA256)
    && sha256(bytes) === env.COLLABORATION_OUTPUT_ALLOCATION_SHA256, "allocation bytes differ from their pin");
  const allocation = JSON.parse(bytes.toString("utf8"));
  requireCondition(allocation !== null && typeof allocation === "object" && !Array.isArray(allocation), "allocation record is malformed");
  requireCondition(allocation.task === task && allocation.source_root === checkout
    && allocation.state_root === state && allocation.database === database
    && allocation.database_url_sha256 === sha256(env.DATABASE_URL), "task, checkout, state or database differs from the allocation");
  const storage = env.COLLABORATION_OUTPUT_STORAGE_ROOT;
  requireCondition(storage === allocation.storage_root && storage === env.LOCAL_UPLOAD_DIR && descendant(state, storage),
    "upload and observation storage must equal the allocated private directory");
  privatePath(storage, io, true);
  requireCondition(env.COLLABORATION_ELECTRON_SOURCE_ROOT === checkout
    && origin(env.COLLABORATION_E2E_API_URL) === allocation.api_origin
    && origin(env.NEXT_PUBLIC_API_URL) === allocation.api_origin
    && origin(env.PLAYWRIGHT_BASE_URL) === allocation.web_origin
    && origin(env.COLLABORATION_ELECTRON_RENDERER_URL) === allocation.renderer_origin
    && env.MULTICA_DEPLOYMENT_ID === allocation.deployment_id, "service configuration differs from the allocation");
  return { mode: "explicit", task, database, state_root: state, storage_root: storage,
    allocation_sha256: sha256(bytes) };
}
