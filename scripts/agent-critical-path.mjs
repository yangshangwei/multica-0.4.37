import { spawn, spawnSync } from "node:child_process";
import { createHash, randomBytes } from "node:crypto";
import { closeSync, existsSync, lstatSync, mkdirSync, openSync, readFileSync, readdirSync, realpathSync, writeFileSync } from "node:fs";
import { dirname, isAbsolute, join, relative, resolve, sep } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

const checkout = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const allowedConfigKeys = ["version", "owner", "databaseName", "databaseUrl", "browserPath"];

export function validateSmokeConfig(value) {
  if (!value || Array.isArray(value) || typeof value !== "object" ||
      Object.keys(value).some((key) => !allowedConfigKeys.includes(key)) ||
      value.version !== 1 || value.owner !== "agent-critical-path" ||
      !/^[a-z][a-z0-9_]{0,62}$/.test(value.databaseName ?? "") ||
      ["postgres", "template0", "template1", "multica"].includes(value.databaseName) ||
      typeof value.browserPath !== "string" || !isAbsolute(value.browserPath)) {
    throw new Error("Invalid smoke allocation; require version, owner, explicit database and browser path");
  }
  try {
    const url = new URL(value.databaseUrl);
    if (!["postgres:", "postgresql:"].includes(url.protocol) ||
        !["localhost", "127.0.0.1", "[::1]"].includes(url.hostname) ||
        !url.port || Number(url.port) < 1 || !url.username || url.hash ||
        decodeURIComponent(url.pathname) !== `/${value.databaseName}` ||
        [...url.searchParams.keys()].some((key) => !["sslmode", "connect_timeout"].includes(key)) ||
        [...url.searchParams.keys()].some((key) => url.searchParams.getAll(key).length !== 1) ||
        url.searchParams.get("sslmode") !== "disable" ||
        /[\x00-\x1f\x7f]/.test(decodeURIComponent(value.databaseUrl))) throw new Error();
  } catch {
    // Never echo a rejected URL or the parser exception: both may contain secrets.
    throw new Error("Smoke requires the exact named database on an explicit loopback PostgreSQL port");
  }
  return value;
}

export function rejectAmbientDotenv(root) {
  for (const dir of [root, join(root, "apps/web")]) {
    for (const name of readdirSync(dir)) {
      if ((name === ".env" || name.startsWith(".env.")) && !name.endsWith(".example")) {
        throw new Error("Smoke refuses ambient dotenv files; use an isolated checkout and a private JSON allocation");
      }
    }
  }
}

export function smokeEnvironment(config, runDir, path, caches) {
  // No ambient accounts, integration secrets, Go flags, database options or
  // dotenv paths survive. The private home does not redirect installed caches.
  return {
    PATH: path,
    HOME: join(runDir, "home"),
    TMPDIR: join(runDir, "tmp"), TMP: join(runDir, "tmp"), TEMP: join(runDir, "tmp"),
    LANG: "en_US.UTF-8", TZ: "UTC", CI: "true", NO_COLOR: "1",
    GOCACHE: caches.GOCACHE, GOMODCACHE: caches.GOMODCACHE, GOPATH: caches.GOPATH,
    GOMAXPROCS: "2", GOFLAGS: "-p=1", GOTOOLCHAIN: "local",
    COREPACK_ENABLE_NETWORK: "0", NEXT_TELEMETRY_DISABLED: "1", ANALYTICS_DISABLED: "true",
    PLAYWRIGHT_BROWSERS_PATH: config.browserPath,
    PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD: "1",
    MULTICA_SMOKE_RUN_DIR: runDir,
    MULTICA_SMOKE_DATABASE_NAME: config.databaseName,
    DATABASE_URL: config.databaseUrl, PGCONNECT_TIMEOUT: "5",
    MULTICA_DEV_HOME: join(runDir, "registry"),
    MULTICA_DEV_TMPDIR: join(runDir, "tmp"),
    LOCAL_UPLOAD_DIR: join(runDir, "uploads"),
    MULTICA_AUTH_MODE: "legacy", MULTICA_DEV_VERIFICATION_CODE: "888888",
    MULTICA_DEVICE_AUTH_ENABLED: "false", MULTICA_MANAGED_INSTALLATIONS_ENABLED: "false",
    MULTICA_KNOWLEDGE_CONTEXT_ENABLED: "false", MULTICA_PLATFORM_ADMIN_ENABLED: "false",
    MULTICA_PLUGINS_V1: "false", MULTICA_CLOUD_URL: "",
    APP_ENV: "development", JWT_SECRET: randomBytes(32).toString("hex"),
    RATE_LIMIT_AUTH: "1000", RATE_LIMIT_AUTH_VERIFY: "1000",
    DATABASE_MAX_CONNS: "8", DATABASE_MIN_CONNS: "1",
  };
}

// Used only to export explicitly selected text evidence. Raw private logs and
// registry/env files remain outside the artifact directory.
export function redactSmokeLog(text, config, env) {
  const url = new URL(config.databaseUrl);
  const secrets = [config.databaseUrl, url.password, decodeURIComponent(url.password), env.JWT_SECRET]
    .filter(Boolean).sort((a, b) => b.length - a.length);
  for (const secret of secrets) text = text.split(secret).join("[REDACTED]");
  return text.replace(/postgres(?:ql)?:\/\/[^\s"'<>]+/g, "[REDACTED_DATABASE_URL]");
}

function writeJSON(file, value) {
  writeFileSync(file, `${JSON.stringify(value, null, 2)}\n`, { mode: 0o600, flag: "wx" });
}

function browserInfo(env) {
  // Resolve the exact locked Playwright registry under the child's environment.
  // Internal registry access is kept here and verified against the installed
  // package; no executable lookup or download fallback occurs at runtime.
  const probe = spawnSync(process.execPath, ["--input-type=commonjs", "-e", `
    const fs = require('fs'), path = require('path'), crypto = require('crypto');
    const packageFile = require.resolve('playwright-core/package.json');
    const {registry} = require(path.join(path.dirname(packageFile), 'lib/server/registry/index.js'));
    const hashTree = (root) => {
      const hash = crypto.createHash('sha256');
      const walk = (dir) => {
        for (const name of fs.readdirSync(dir).sort()) {
          const file = path.join(dir, name), s = fs.lstatSync(file);
          if ((!s.isSymbolicLink() && s.mode & 0o022) || ![0, process.getuid()].includes(s.uid)) throw new Error('Unsafe browser installation ownership');
          hash.update(JSON.stringify([path.relative(root,file),s.mode]));
          if (s.isSymbolicLink()) {
            if (!fs.realpathSync(file).startsWith(fs.realpathSync(root)+path.sep)) throw new Error('Browser link escapes installation');
            hash.update(fs.readlinkSync(file));
          }
          else if (s.isDirectory()) walk(file);
          else if (s.isFile()) hash.update(fs.readFileSync(file));
          else throw new Error('Invalid browser file');
        }
      };
      walk(root); return hash.digest('hex');
    };
    const installations = ['chromium','chromium-headless-shell'].map(name => {
      const e = registry.findExecutable(name), executable = e.executablePath();
      fs.accessSync(executable,fs.constants.X_OK);
      fs.accessSync(path.join(e.directory,'INSTALLATION_COMPLETE'));
      const actual = fs.realpathSync(e.directory);
      if (!fs.realpathSync(executable).startsWith(actual+path.sep)) throw new Error('Browser executable escapes installation');
      return {name,directory:actual,executable,sha256:hashTree(e.directory)};
    });
    console.log(JSON.stringify({version:require(packageFile).version,platform:process.platform,arch:process.arch,installations}));
  `], { cwd: checkout, env, encoding: "utf8", maxBuffer: 2 * 1024 * 1024 });
  if (probe.status !== 0) throw new Error("Explicit Playwright browser installation is unavailable or unsafe");
  return JSON.parse(probe.stdout);
}

async function launch(configPath, outputPath) {
  const info = lstatSync(configPath);
  if (!info.isFile() || (info.mode & 0o077) !== 0 || (process.getuid && info.uid !== process.getuid())) {
    throw new Error("Smoke allocation must be a private regular file owned by this user (0600)");
  }
  const config = validateSmokeConfig(JSON.parse(readFileSync(configPath, "utf8")));
  rejectAmbientDotenv(checkout);
  const parent = realpathSync(dirname(outputPath));
  const output = join(parent, outputPath.split(sep).at(-1));
  const rel = relative(checkout, output);
  if (rel === "" || (!rel.startsWith(`..${sep}`) && !isAbsolute(rel))) {
    throw new Error("Smoke evidence must be outside the source checkout");
  }
  mkdirSync(output, { mode: 0o700 }); // exclusive: never erase an earlier run.
  for (const name of ["home", "tmp", "uploads", "evidence"]) mkdirSync(join(output, name), { mode: 0o700 });
  const cacheProbe = spawnSync("go", ["env", "-json", "GOCACHE", "GOMODCACHE", "GOPATH"], { encoding: "utf8" });
  if (cacheProbe.status !== 0) throw new Error("Cannot resolve the installed Go caches");
  const caches = JSON.parse(cacheProbe.stdout);
  if (Object.values(caches).some((value) => !value || !isAbsolute(value))) throw new Error("Go caches must be explicit absolute paths");
  const env = smokeEnvironment(config, output, process.env.PATH, caches);
  const evidence = join(output, "evidence");
  const browser = browserInfo(env);
  writeJSON(join(evidence, "browser.json"), browser);
  writeJSON(join(output, "allocation.json"), {
    owner: config.owner, database: config.databaseName, host: new URL(config.databaseUrl).hostname,
    port: Number(new URL(config.databaseUrl).port), checkout, browserVersion: browser.version,
  });
  const logFD = openSync(join(output, "launcher.log"), "wx", 0o600);
  let code;
  try {
    const child = spawn("bash", [join(checkout, "scripts/agent-critical-path.sh")], {
      cwd: checkout, env, stdio: ["ignore", logFD, logFD],
    });
    const signal = (name) => child.kill(name);
    const onInt = () => signal("SIGINT"), onTerm = () => signal("SIGTERM");
    process.once("SIGINT", onInt); process.once("SIGTERM", onTerm);
    code = await new Promise((res, rej) => { child.once("error", rej); child.once("exit", (status) => res(status ?? 1)); });
    process.removeListener("SIGINT", onInt); process.removeListener("SIGTERM", onTerm);
  } finally { closeSync(logFD); }
  const names = ["launcher.log", "allocation.json", "playwright.json", "inventory.json", "result.json", "identity.json", "rendered.json",
    "registry/envs/smoke/api.running.json", "registry/envs/smoke/web.running.json",
    "registry/envs/smoke/logs/migrate.log", "registry/envs/smoke/logs/api.log", "registry/envs/smoke/logs/web.log", "registry/envs/smoke/logs/web-build.log"];
  for (const name of names) {
    const source = join(output, name);
    if (!existsSync(source)) continue;
    writeFileSync(join(evidence, name.replaceAll("/", "-")), redactSmokeLog(readFileSync(source, "utf8"), config, env), { mode: 0o600, flag: "wx" });
  }
  // Traces/screenshots contain only isolated synthetic fixtures. Keep their
  // paths separate from private state; workflow uploads just this directory.
  const results = join(output, "test-results");
  if (existsSync(results)) {
    const { cpSync } = await import("node:fs");
    cpSync(results, join(evidence, "test-results"), { recursive: true, errorOnExist: true, force: false });
  }
  writeJSON(join(evidence, "launcher-result.json"), { exit: code, browser_sha256: createHash("sha256").update(JSON.stringify(browser)).digest("hex") });
  console.log(`Critical-path smoke ${code === 0 ? "passed" : "failed"}; evidence: ${evidence}`);
  return code;
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  try {
    const [configFlag, config, outputFlag, output] = process.argv.slice(2);
    if (process.argv.length !== 6 || configFlag !== "--config" || outputFlag !== "--output" || !isAbsolute(config ?? "") || !isAbsolute(output ?? "")) {
      throw new Error("Usage: node scripts/agent-critical-path.mjs --config /private/allocation.json --output /new/run-directory");
    }
    process.exitCode = await launch(config, output);
  } catch (error) {
    console.error(error instanceof SyntaxError ? "Invalid smoke allocation JSON" : error.message);
    process.exitCode = 1;
  }
}
