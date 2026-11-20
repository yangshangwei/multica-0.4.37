import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { mkdirSync, mkdtempSync, realpathSync, renameSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { test as nodeTest } from "node:test";
import { dirname, join } from "node:path";
import pg from "pg";
import { assertRuntimeOutputAllocation as guard } from "./collaboration-runtime-output-allocation.mjs";

const test = (name, callback) => nodeTest(name, { skip: typeof process.getuid !== "function" ? "POSIX ownership guard is unsupported on this platform" : false }, callback);
const sha = (value) => createHash("sha256").update(value).digest("hex");
const activeTask = ".trellis/tasks/11-20-full-e2e-regression-20261120";
const archivedTask = ".trellis/tasks/archive/2026-11/11-20-full-e2e-regression-20261120";

function fixture(legacy = false, task = activeTask) {
  const checkout = "/owned/checkout";
  const state = legacy ? "/Users/artisan/.multica/acceptance/pay241-s3-output-20261119-tm89sjgy" : "/owned/output-current";
  const storage = `${state}/uploads`, allocationFile = `${state}/allocation.json`;
  const taskFilename = join(checkout, task, "task.json");
  const database = legacy ? "multica_pay241_s3o_20261119_tm89sjgy_web" : "multica_qa_output_current";
  const env = { DATABASE_URL: `postgres://fixture:synthetic@127.0.0.1:24567/${database}`, TASK_DATABASE_NAME: database,
    LOCAL_UPLOAD_DIR: storage, COLLABORATION_OUTPUT_STORAGE_ROOT: storage,
    COLLABORATION_E2E_API_URL: "http://localhost:18581", NEXT_PUBLIC_API_URL: "http://localhost:18581",
    PLAYWRIGHT_BASE_URL: "http://localhost:13501", COLLABORATION_ELECTRON_RENDERER_URL: "http://127.0.0.1:5675",
    COLLABORATION_ELECTRON_SOURCE_ROOT: checkout, MULTICA_DEPLOYMENT_ID: "11111111-1111-4111-8111-111111111111" };
  const allocation = { task, source_root: checkout, state_root: state, storage_root: storage, database,
    database_url_sha256: sha(env.DATABASE_URL), api_origin: env.COLLABORATION_E2E_API_URL,
    web_origin: env.PLAYWRIGHT_BASE_URL, renderer_origin: env.COLLABORATION_ELECTRON_RENDERER_URL,
    deployment_id: env.MULTICA_DEPLOYMENT_ID };
  const files = new Map(), metadata = new Map();
  const addDirectory = (path, mode = 0o700) => {
    for (let current = path; !metadata.has(current); current = dirname(current)) {
      metadata.set(current, { mode, uid: process.getuid(), isDirectory: () => true, isFile: () => false, isSymbolicLink: () => false });
      if (current === "/") break;
    }
  };
  addDirectory(checkout); addDirectory(join(checkout, task)); addDirectory(storage);
  metadata.set(taskFilename, { mode: 0o644, uid: process.getuid(), isDirectory: () => false, isFile: () => true, isSymbolicLink: () => false });
  files.set(taskFilename, Buffer.from(JSON.stringify({ id: "full-e2e-regression-20261120", name: "full-e2e-regression-20261120" })));
  metadata.set(allocationFile, { mode: 0o600, uid: process.getuid(), isDirectory: () => false, isFile: () => true, isSymbolicLink: () => false });
  const io = {
    realpathSync: (path) => { if (!metadata.has(path)) throw new Error("Fixture path missing"); return path; },
    lstatSync: (path) => { const value = metadata.get(path); if (!value) throw new Error("Fixture path missing"); return value; },
    readFileSync: (path) => { const value = files.get(path); if (!value) throw new Error("Fixture file missing"); return value; },
  };
  const pin = () => { const bytes = Buffer.from(JSON.stringify(allocation)); files.set(allocationFile, bytes); env.COLLABORATION_OUTPUT_ALLOCATION_SHA256 = sha(bytes); };
  if (!legacy) Object.assign(env, { COLLABORATION_OUTPUT_TASK: task, COLLABORATION_OUTPUT_STATE_ROOT: state,
    COLLABORATION_OUTPUT_ALLOCATION_FILE: allocationFile });
  if (!legacy) pin();
  return { env, checkout, task, taskFilename, state, storage, allocationFile, allocation, files, metadata, io, pin,
    run: () => guard(env, checkout, io) };
}

test("a fresh exact current-task allocation is admitted before output setup", () => {
  assert.equal(fixture().run().task, activeTask);
});
test("an explicitly pinned archived task is admitted without its old active directory", () => {
  const f = fixture(false, archivedTask);
  assert.equal(f.metadata.has(join(f.checkout, activeTask)), false);
  assert.equal(f.run().task, archivedTask);
});
test("a real task archive requires a newly pinned allocation at its explicit new location", () => {
  const root = mkdtempSync(join(realpathSync(tmpdir()), "multica-output-archive-"));
  try {
    const checkout = join(root, "checkout"), state = join(root, "state"), storage = join(state, "uploads");
    const allocationFile = join(state, "allocation.json");
    mkdirSync(join(checkout, activeTask), { recursive: true });
    mkdirSync(state, { mode: 0o700 }); mkdirSync(storage, { mode: 0o700 });
    writeFileSync(join(checkout, activeTask, "task.json"), JSON.stringify({
      id: "full-e2e-regression-20261120", name: "full-e2e-regression-20261120", status: "in_progress",
    }));
    const f = fixture();
    Object.assign(f.env, { COLLABORATION_ELECTRON_SOURCE_ROOT: checkout,
      COLLABORATION_OUTPUT_STATE_ROOT: state, COLLABORATION_OUTPUT_ALLOCATION_FILE: allocationFile,
      LOCAL_UPLOAD_DIR: storage, COLLABORATION_OUTPUT_STORAGE_ROOT: storage });
    Object.assign(f.allocation, { source_root: checkout, state_root: state, storage_root: storage });
    const pin = () => {
      const bytes = Buffer.from(JSON.stringify(f.allocation));
      writeFileSync(allocationFile, bytes, { mode: 0o600 });
      f.env.COLLABORATION_OUTPUT_ALLOCATION_SHA256 = sha(bytes);
    };
    pin();
    assert.equal(guard(f.env, checkout).task, activeTask);
    mkdirSync(dirname(join(checkout, archivedTask)), { recursive: true });
    renameSync(join(checkout, activeTask), join(checkout, archivedTask));
    assert.throws(() => guard(f.env, checkout), { code: "ENOENT" });
    f.env.COLLABORATION_OUTPUT_TASK = archivedTask;
    assert.throws(() => guard(f.env, checkout), /differs from the allocation/);
    f.allocation.task = archivedTask; pin();
    assert.equal(guard(f.env, checkout).task, archivedTask);
  } finally {
    rmSync(root, { recursive: true, force: true });
  }
});
test("the historical exact allocation still passes without reading real historical paths", () => { fixture(true).run(); });
test("the historical fallback refuses a different database", () => {
  const f = fixture(true); f.env.DATABASE_URL += "_foreign"; f.env.TASK_DATABASE_NAME += "_foreign";
  f.env.DATABASE_URL = f.env.DATABASE_URL.replace("tm89sjgy", "foreign"); assert.throws(f.run);
});
test("the historical fallback refuses storage outside its original root", () => {
  const f = fixture(true); f.io.realpathSync = () => "/owned/foreign/uploads"; assert.throws(f.run);
});
test("partial explicit allocation cannot fall back to the historical allocation", () => {
  for (const key of ["TASK", "STATE_ROOT", "ALLOCATION_FILE", "ALLOCATION_SHA256"]) {
    const f = fixture(true); f.env[`COLLABORATION_OUTPUT_${key}`] = ""; assert.throws(f.run);
  }
});
test("allocation bytes must match their pin", () => {
  const f = fixture(); f.files.set(f.allocationFile, Buffer.from("{}")); assert.throws(f.run);
});
test("pinned malformed JSON is rejected", () => {
  const f = fixture(); f.files.set(f.allocationFile, Buffer.from("{")); f.env.COLLABORATION_OUTPUT_ALLOCATION_SHA256 = sha("{"); assert.throws(f.run);
});
test("a different task is refused even when both environment and record agree", () => {
  const f = fixture(); f.env.COLLABORATION_OUTPUT_TASK = f.allocation.task = ".trellis/tasks/old-task"; f.pin(); assert.throws(f.run);
});
test("the exact selected task directory must exist", () => {
  for (const task of [activeTask, archivedTask]) {
    const f = fixture(false, task); f.metadata.delete(join(f.checkout, f.task)); assert.throws(f.run);
  }
});
test("an old active allocation is not silently redirected to the archived task", () => {
  const f = fixture(false, archivedTask);
  f.env.COLLABORATION_OUTPUT_TASK = f.allocation.task = activeTask; f.pin();
  assert.throws(f.run, /Fixture path missing/);
});
test("only canonical active or month-scoped archive paths are admitted", () => {
  for (const task of [
    `${archivedTask}/`, `${archivedTask}/nested`, `/${archivedTask}`,
    `${activeTask}\n`, `${archivedTask}\r\n`, `${archivedTask}\u2028`,
    archivedTask.replace("2026-11/", "2026-00/"), archivedTask.replace("2026-11/", "2026-13/"),
    archivedTask.replace("2026-11/", "2026-1/"), archivedTask.replace("2026-11/", "2026-11/../2026-11/"),
    archivedTask.replace("2026-11/", ""), activeTask.replace("tasks/", "tasks/../tasks/"),
  ]) {
    assert.throws(fixture(false, task).run, /the task is not/, task);
  }
});
test("task metadata must exist for active and archived allocations", () => {
  for (const task of [activeTask, archivedTask]) {
    const f = fixture(false, task); f.metadata.delete(f.taskFilename); f.files.delete(f.taskFilename);
    assert.throws(f.run, /Fixture path missing/);
  }
});
test("task metadata must identify this exact acceptance task", () => {
  for (const task of [activeTask, archivedTask]) {
    for (const value of [null, [], "foreign", {},
      { id: "foreign", name: "full-e2e-regression-20261120" },
      { id: "full-e2e-regression-20261120", name: "foreign" }]) {
      const f = fixture(false, task); f.files.set(f.taskFilename, Buffer.from(JSON.stringify(value)));
      assert.throws(f.run, /task metadata/, JSON.stringify(value));
    }
    const f = fixture(false, task); f.files.set(f.taskFilename, Buffer.from("{"));
    assert.throws(f.run, SyntaxError);
  }
});
test("task metadata must be a regular file without symlink segments", () => {
  for (const task of [activeTask, archivedTask]) {
    for (const mutation of [{ isFile: () => false }, { isSymbolicLink: () => true }]) {
      const f = fixture(false, task); Object.assign(f.metadata.get(f.taskFilename), mutation);
      assert.throws(f.run, /task metadata|symlink/);
    }
    const f = fixture(false, task); f.metadata.get(join(f.checkout, task)).isSymbolicLink = () => true;
    assert.throws(f.run, /symlink/);
  }
});
test("the allocation pins the explicitly selected task location", () => {
  for (const [selected, recorded] of [[activeTask, archivedTask], [archivedTask, activeTask]]) {
    const f = fixture(false, selected); f.allocation.task = recorded; f.pin();
    assert.throws(f.run, /differs from the allocation/);
  }
});
test("another checkout cannot borrow the allocation", () => {
  const f = fixture(); f.allocation.source_root = "/owned/another-checkout"; f.pin(); assert.throws(f.run);
});
test("changed database connection bytes cannot borrow a same-name database", () => {
  const f = fixture(); f.env.DATABASE_URL = f.env.DATABASE_URL.replace("24567", "24568"); assert.throws(f.run);
});
test("remote, default and invalid-port databases are rejected even when pinned", () => {
  for (const url of ["postgres://fixture:synthetic@remote.invalid:24567/multica_qa_output_current",
    "postgres://fixture:synthetic@127.0.0.1:24567/postgres", "postgres://fixture:synthetic@127.0.0.1:0/multica_qa_output_current"]) {
    const f = fixture(); f.env.DATABASE_URL = url; f.env.TASK_DATABASE_NAME = f.allocation.database = new URL(url).pathname.slice(1);
    f.allocation.database_url_sha256 = sha(url); f.pin(); assert.throws(f.run);
  }
});
test("database query overrides and fragments are refused even when pinned", () => {
  for (const suffix of ["?host=example.invalid", "?port=5432", "?dbname=postgres", "?sslmode=disable", "#fragment"]) {
    for (const legacy of [false, true]) {
      const f = fixture(legacy); f.env.DATABASE_URL += suffix;
      if (!legacy) { f.allocation.database_url_sha256 = sha(f.env.DATABASE_URL); f.pin(); }
      assert.throws(f.run, /database identity/);
    }
  }
});
test("an admitted URL has the exact effective pg host, port and database without connecting", () => {
  const f = fixture(); f.run();
  const { connectionParameters } = new pg.Client(f.env.DATABASE_URL);
  assert.equal(connectionParameters.host, "127.0.0.1");
  assert.equal(connectionParameters.port, 24567);
  assert.equal(connectionParameters.database, f.allocation.database);
  // Demonstrate why URL.hostname alone does not describe the actual PG destination.
  const override = new pg.Client(`${f.env.DATABASE_URL}?host=example.invalid&port=5432`);
  assert.equal(override.connectionParameters.host, "example.invalid");
  assert.equal(override.connectionParameters.port, 5432);
});
test("storage must match both the allocation and API upload directory exactly", () => {
  for (const key of ["LOCAL_UPLOAD_DIR", "COLLABORATION_OUTPUT_STORAGE_ROOT"]) {
    const f = fixture(); f.env[key] = "/owned/different-uploads"; assert.throws(f.run);
  }
});
test("storage cannot escape the private state through sibling prefixes", () => {
  const f = fixture(); f.allocation.storage_root = f.env.LOCAL_UPLOAD_DIR = f.env.COLLABORATION_OUTPUT_STORAGE_ROOT = `${f.state}-other/uploads`;
  f.pin(); assert.throws(f.run);
});
test("state and storage require owner-only permissions and ownership", () => {
  for (const key of ["state", "storage"]) for (const mutation of [{ mode: 0o755 }, { uid: process.getuid() + 1 }]) {
    const f = fixture(); Object.assign(f.metadata.get(f[key]), mutation); assert.throws(f.run);
  }
});
test("symlink path segments and noncanonical aliases are rejected", () => {
  for (const target of ["/owned", "/owned/output-current"]) {
    const f = fixture(); f.metadata.get(target).isSymbolicLink = () => true; assert.throws(f.run);
  }
  const f = fixture(); f.io.realpathSync = (path) => path === f.state ? "/owned/redirected" : path; assert.throws(f.run);
});
test("the allocation file is private, regular and inside its recorded state", () => {
  for (const mutation of [{ mode: 0o644 }, { isFile: () => false }, { isSymbolicLink: () => true }]) {
    const f = fixture(); Object.assign(f.metadata.get(f.allocationFile), mutation); assert.throws(f.run);
  }
});
test("API, Web, renderer and deployment must match the allocation", () => {
  for (const key of ["COLLABORATION_E2E_API_URL", "NEXT_PUBLIC_API_URL", "PLAYWRIGHT_BASE_URL", "COLLABORATION_ELECTRON_RENDERER_URL", "MULTICA_DEPLOYMENT_ID"]) {
    const f = fixture(); f.env[key] += "changed"; assert.throws(f.run);
  }
});
