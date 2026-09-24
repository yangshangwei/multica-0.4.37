import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { chmodSync, cpSync, existsSync, mkdirSync, mkdtempSync, readFileSync, readdirSync, rmSync, statSync, symlinkSync, writeFileSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import test from "node:test";
import { scratch, scripts, seed } from "./changelog-test-helpers.mjs";

const composeAvailable = spawnSync("docker", ["compose", "version"], { encoding: "utf8" }).status === 0;

function offlineFixture(t, directory) {
  const cwd = directory ?? scratch(t);
  mkdirSync(join(cwd, "scripts"));
  mkdirSync(join(cwd, "server/internal/changelog/content"), { recursive: true });
  mkdirSync(join(cwd, "docs"));
  mkdirSync(join(cwd, "bin"));
  mkdirSync(join(cwd, "apps/desktop/scripts"), { recursive: true });
  cpSync(resolve(scripts, "../apps/desktop/scripts/update-artifacts.mjs"), join(cwd, "apps/desktop/scripts/update-artifacts.mjs"));
  symlinkSync(resolve(scripts, "../apps/desktop/node_modules"), join(cwd, "apps/desktop/node_modules"), "dir");
  for (const name of ["offline-bundle.sh", "offline-installer.sh", "build-offline-upgrade.sh", "offline-upgrade.sh", "changelog-lib.mjs", "publish-changelog.mjs", "install-changelog.sh", "install-changelog.mjs"]) {
    const source = join(scripts, name);
    if (existsSync(source)) cpSync(source, join(cwd, "scripts", name));
  }
  for (const name of ["docker-compose.selfhost.yml", "docker-compose.selfhost.build.yml", ".env.example"]) cpSync(resolve(scripts, "..", name), join(cwd, name));
  writeFileSync(join(cwd, "server/internal/changelog/content/changelog.json"), JSON.stringify(seed(), null, 2) + "\n");
  writeFileSync(join(cwd, "docs/offline-upgrade.zh-CN.md"), "Offline fixture upgrade guide\n");
  const fake = join(cwd, "bin/docker");
  writeFileSync(fake, `#!${process.execPath}
import { appendFileSync, readFileSync, writeFileSync } from 'node:fs';
import { spawnSync } from 'node:child_process';
import { resolve } from 'node:path';
const args = process.argv.slice(2);
appendFileSync(process.env.DOCKER_LOG, JSON.stringify(args) + '\\n');
if (args[0] === 'save') process.stdout.write('fixture image archive');
else if (args[0] === 'run') {
  if (!args.includes('--pull') || args[args.indexOf('--pull') + 1] !== 'never') process.exit(40);
  if (!args.includes('--entrypoint') || args[args.indexOf('--entrypoint') + 1] !== 'node') process.exit(41);
  if (!args.includes('--user')) process.exit(42);
  if (process.env.FAIL_CONTAINER === '1') process.exit(43);
  const mounts = args.flatMap((arg, index) => {
    if (arg !== '--mount') return [];
    const spec = args[index + 1];
    const src = spec.match(/(?:^|,)"src=((?:[^"]|"")*)"(?:,|$)/)?.[1].replace(/""/g, '"');
    const dst = spec.match(/(?:^|,)dst=([^,]+)/)?.[1];
    if (!src || !dst) process.exit(45);
    return [{ src, dst }];
  });
  const nodeArgs = args.slice(args.includes('-e') ? args.indexOf('-e') : args.findIndex((arg) => arg.endsWith('/install-changelog.mjs')));
  const mapped = nodeArgs.map((arg) => {
    const mount = mounts.find((item) => arg.startsWith(item.dst + '/'));
    return mount ? mount.src + arg.slice(mount.dst.length) : arg;
  });
  const child = spawnSync(${JSON.stringify(process.execPath)}, mapped, { stdio: 'inherit', env: process.env });
  process.exit(child.status ?? 1);
} else if (args[0] === 'compose' && args.includes('build')) {
  if (!process.env.CHANGELOG_ARTIFACT_PATH?.startsWith('.changelog-build/')) process.exit(44);
  writeFileSync(process.env.DOCKER_LOG + '.embedded', readFileSync(process.env.CHANGELOG_ARTIFACT_PATH));
} else if (args[0] === 'compose' && args.includes('config') && args.includes('--images')) {
  const version = process.env.VERSION || 'dev';
  process.stdout.write(['multica-backend:' + version, 'multica-web:' + version, 'pgvector/pgvector:pg17'].join('\\n') + '\\n');
} else if (args[0] === 'compose' && args.includes('config')) {
  const env = readFileSync(args[args.indexOf('--env-file') + 1], 'utf8');
  const values = {};
  for (const key of ['CHANGELOG_FILE', 'CHANGELOG_DIRECTORY']) {
    const line = env.split('\\n').findLast((line) => line.startsWith(key + '='));
    let value = line?.slice(key.length + 1) ?? '';
    if (value.startsWith('"')) value = JSON.parse(value).replace(/\\$\\$/g, '$');
    else if (value.startsWith("'")) value = value.slice(1, -1);
    values[key] = value;
  }
  const source = resolve(args[args.indexOf('--project-directory') + 1], values.CHANGELOG_DIRECTORY || './changelog').replace(/\\$/g, () => '$$');
  process.stdout.write(JSON.stringify({ services: { backend: { environment: { CHANGELOG_FILE: values.CHANGELOG_FILE }, volumes: [{ type: 'bind', source, target: '/app/data/changelog' }] } } }));
} else if (args[0] === 'compose' && args.includes('port')) process.stdout.write('127.0.0.1:8080\\n');
else if (args[0] === 'compose' && args.includes('exec')) process.stdout.write('fixture database dump\\n');
else if (args[0] === 'compose' && args.includes('up') && process.env.FAIL_UP === '1') process.exit(46);
`);
  chmodSync(fake, 0o755);
  writeFileSync(join(cwd, "bin/curl"), "#!/bin/sh\nexit 0\n", { mode: 0o755 });
  const env = { ...process.env, PATH: `${join(cwd, "bin")}:${process.env.PATH}`, VERSION: "v1.0.0-fixture", COMMIT: "fixture", DOCKER_LOG: join(cwd, "docker.log"), JWT_SECRET: "fixture-secret" };
  return { cwd, env };
}

function bash(cwd, env, ...args) { return spawnSync("/bin/bash", args, { cwd, env, encoding: "utf8" }); }

function upgradeFixture(t) {
  const fixture = offlineFixture(t);
  fixture.env.VERSION = "v0.4.45";
  const platform = process.arch === "arm64" ? "linux/arm64" : "linux/amd64";
  const built = bash(fixture.cwd, fixture.env, "scripts/build-offline-upgrade.sh", "--output", "upgrade", "--platform", platform);
  assert.equal(built.status, 0, built.stderr);
  const packageDir = join(fixture.cwd, "upgrade", `multica-server-upgrade-v0.4.45-${platform.replace("/", "-")}`);
  const deployment = join(fixture.cwd, "deployment");
  mkdirSync(deployment);
  const preservedEnv = "# Existing deployment\nJWT_SECRET=preserve-secret\nPOSTGRES_PASSWORD=preserve-password\nOTHER_KEY=preserve-value\nPRIVATE_NOTE='first line\nMULTICA_IMAGE_TAG=inside-note\nlast line'\n";
  const originalEnv = preservedEnv + "MULTICA_BACKEND_IMAGE=old-backend\nexport MULTICA_WEB_IMAGE=old-web\nMULTICA_IMAGE_TAG=dev\nCHANGELOG_DIRECTORY=custom-feed\n";
  writeFileSync(join(deployment, ".env"), originalEnv, { mode: 0o600 });
  cpSync(join(packageDir, "docker-compose.selfhost.yml"), join(deployment, "docker-compose.selfhost.yml"));
  const backup = join(deployment, "backups/fixture");
  const withoutNode = { ...fixture.env, PATH: `${join(fixture.cwd, "bin")}:/usr/bin:/bin:/usr/sbin:/sbin` };
  return { ...fixture, packageDir, deployment, backup, withoutNode, preservedEnv, originalEnv };
}

test("offline bundle carries exact feed and every publisher helper, then installs without host Node", (t) => {
  const fixture = offlineFixture(t);
  const output = join(fixture.cwd, "bundle");
  const built = bash(fixture.cwd, fixture.env, "scripts/offline-bundle.sh", "--output", output, "--platform", "linux/amd64");
  assert.equal(built.status, 0, built.stderr);
  for (const path of ["changelog/changelog.json", "scripts/changelog-lib.mjs", "scripts/publish-changelog.mjs", "scripts/install-changelog.mjs", "install-changelog.sh"]) assert.ok(existsSync(join(output, path)), `bundle missing ${path}`);
  assert.equal(readFileSync(fixture.env.DOCKER_LOG + ".embedded", "utf8"), readFileSync(join(output, "changelog/changelog.json"), "utf8"));
  assert.deepEqual(readdirSync(join(fixture.cwd, ".changelog-build")), [], "own Docker context staging is cleaned");
  const deployment = join(fixture.cwd, "deployment");
  mkdirSync(deployment);
  writeFileSync(join(deployment, ".env"), "JWT_SECRET=keep-this\nOTHER=keep-too\n");
  const withoutNode = { ...fixture.env, PATH: `${join(fixture.cwd, "bin")}:/usr/bin:/bin:/usr/sbin:/sbin` };
  assert.notEqual(spawnSync("node", ["--version"], { env: withoutNode }).status, 0, "host Node must be unavailable for this fixture");
  const installed = bash(output, withoutNode, "install-changelog.sh", "--deployment-dir", deployment, "--web-image", "multica-web:dev");
  assert.equal(installed.status, 0, installed.stderr);
  assert.equal(readFileSync(join(deployment, "changelog/changelog.json"), "utf8"), readFileSync(join(output, "changelog/changelog.json"), "utf8"));
  assert.match(readFileSync(join(deployment, ".env"), "utf8"), /CHANGELOG_FILE='\/app\/data\/changelog\/changelog.json'/);
  const runs = readFileSync(fixture.env.DOCKER_LOG, "utf8").trim().split("\n").map(JSON.parse).filter((args) => args[0] === "run");
  assert.equal(runs.length, 2);
  assert.ok(runs[0].includes(`${process.getuid()}:${process.getgid()}`));
  assert.ok(runs[0].includes("-e"));
  assert.equal(runs[0].includes("--mount"), false);
  assert.ok(runs[1].some((arg) => arg.includes(`"src=${deployment}/changelog",dst=/changelog-output`)));
});

test("upgrade archive includes publication tools and upgrade performs the handoff before service restart", (t) => {
  const fixture = offlineFixture(t);
  const platform = process.arch === "arm64" ? "linux/arm64" : "linux/amd64";
  const built = bash(fixture.cwd, fixture.env, "scripts/build-offline-upgrade.sh", "--output", "upgrade", "--platform", platform);
  assert.equal(built.status, 0, built.stderr);
  const archive = readdirSync(join(fixture.cwd, "upgrade")).find((name) => name.endsWith(".tar.gz"));
  const listing = spawnSync("tar", ["-tzf", join(fixture.cwd, "upgrade", archive)], { encoding: "utf8" });
  for (const file of ["changelog/changelog.json", "scripts/changelog-lib.mjs", "scripts/install-changelog.mjs", "install-changelog.sh"]) assert.ok(listing.stdout.includes(file), `archive omitted ${file}`);
  const packageDir = join(fixture.cwd, "upgrade", archive.slice(0, -7));
  const deployment = join(fixture.cwd, "deployment");
  mkdirSync(deployment);
  const originalEnv = "JWT_SECRET=preserve-secret\nOTHER_KEY=preserve-value\nCHANGELOG_DIRECTORY=custom-feed\n";
  writeFileSync(join(deployment, ".env"), originalEnv);
  const withoutNode = { ...fixture.env, PATH: `${join(fixture.cwd, "bin")}:/usr/bin:/bin:/usr/sbin:/sbin` };
  const upgraded = bash(packageDir, withoutNode, "offline-upgrade.sh", "--deployment-dir", deployment, "--yes");
  assert.equal(upgraded.status, 0, upgraded.stderr);
  assert.ok(existsSync(join(deployment, "custom-feed/changelog.json")));
  assert.match(readFileSync(join(deployment, ".env"), "utf8"), /OTHER_KEY=preserve-value/);
  const calls = readFileSync(fixture.env.DOCKER_LOG, "utf8").trim().split("\n").map(JSON.parse);
  const installed = calls.findIndex((args) => args[0] === "run");
  const restarted = calls.findIndex((args) => args[0] === "compose" && args.includes("up"));
  assert.ok(installed > 0 && restarted > installed);
  assert.ok(calls[restarted].includes("--project-directory"));
});

test("upgraded deployments retain exact image selection in later Compose invocations", { skip: !composeAvailable }, async (t) => {
  for (const overrides of [false, true]) {
    await t.test(overrides ? "explicit registry and tag" : "package images", (t) => {
      const f = upgradeFixture(t);
      const backend = overrides ? "registry.intra.example.com:5000/multica-backend" : "multica-backend";
      const web = overrides ? "registry.intra.example.com:5000/multica-web" : "multica-web";
      const tag = overrides ? "v0.4.45-custom" : "v0.4.45";
      const args = overrides ? ["--backend-image", backend, "--web-image", web, "--image-tag", tag] : [];
      const result = bash(f.packageDir, f.withoutNode, "offline-upgrade.sh", "--deployment-dir", f.deployment, "--backup-dir", f.backup, "--yes", ...args);
      assert.equal(result.status, 0, result.stderr);
      // This is real Compose parsing after the upgrade process has exited:
      // no image overrides and no Docker daemon/container are involved.
      const env = { ...process.env };
      for (const name of ["MULTICA_BACKEND_IMAGE", "MULTICA_WEB_IMAGE", "MULTICA_IMAGE_TAG", "JWT_SECRET"]) delete env[name];
      const parsed = spawnSync("docker", ["compose", "--project-directory", f.deployment, "--env-file", join(f.deployment, ".env"), "-f", join(f.deployment, "docker-compose.selfhost.yml"), "config", "--format", "json"], { env, encoding: "utf8" });
      assert.equal(parsed.status, 0, parsed.stderr);
      const { services } = JSON.parse(parsed.stdout);
      assert.equal(services.backend.image, `${backend}:${tag}`);
      assert.equal(services.frontend.image, `${web}:${tag}`);
      assert.equal(services.backend.environment.JWT_SECRET, "preserve-secret");
      assert.ok(readFileSync(join(f.deployment, ".env"), "utf8").startsWith(f.preservedEnv));
      assert.equal(statSync(join(f.deployment, ".env")).mode & 0o777, 0o600);
      assert.equal(readFileSync(join(f.backup, ".env"), "utf8"), f.originalEnv);
      assert.equal((result.stdout + result.stderr).includes("preserve-secret"), false);
    });
  }
});

test("upgrade failures retain backups and never claim container or database rollback", async (t) => {
  for (const failure of ["FAIL_CONTAINER", "FAIL_UP", "FAIL_HEALTH"]) {
    await t.test(failure, (t) => {
      const f = upgradeFixture(t);
      writeFileSync(join(f.cwd, "bin/curl"), "#!/bin/sh\n[ \"${FAIL_HEALTH:-0}\" != 1 ]\n", { mode: 0o755 });
      writeFileSync(join(f.cwd, "bin/sleep"), "#!/bin/sh\nexit 0\n", { mode: 0o755 });
      const result = bash(f.packageDir, { ...f.withoutNode, [failure]: "1" }, "offline-upgrade.sh", "--deployment-dir", f.deployment, "--backup-dir", f.backup, "--yes");
      assert.notEqual(result.status, 0);
      assert.equal(readFileSync(join(f.backup, ".env"), "utf8"), f.originalEnv);
      const current = readFileSync(join(f.deployment, ".env"), "utf8");
      const calls = readFileSync(f.env.DOCKER_LOG, "utf8").trim().split("\n").map(JSON.parse);
      if (failure === "FAIL_CONTAINER") {
        assert.equal(current, f.originalEnv);
        assert.equal(calls.some((args) => args[0] === "compose" && args.includes("up")), false);
      } else {
        assert.match(current, /^MULTICA_IMAGE_TAG=["']?v0\.4\.45["']?$/m);
        assert.match(result.stderr, /upgrade is incomplete/);
        assert.match(result.stderr, /not rolled back/);
        assert.ok(current.startsWith(f.preservedEnv));
      }
      assert.equal(result.stdout.includes("✓ Upgrade completed"), false);
      assert.equal((result.stdout + result.stderr).includes("preserve-secret"), false);
    });
  }
});

test("missing image execution or malformed bundled feed leaves existing feed and config unchanged", (t) => {
  const fixture = offlineFixture(t);
  const output = join(fixture.cwd, "bundle");
  assert.equal(bash(fixture.cwd, fixture.env, "scripts/offline-bundle.sh", "--output", output).status, 0);
  const deployment = join(fixture.cwd, "deployment");
  mkdirSync(join(deployment, "changelog"), { recursive: true });
  const envBytes = "JWT_SECRET=original\n";
  writeFileSync(join(deployment, ".env"), envBytes);
  writeFileSync(join(deployment, "changelog/changelog.json"), "previous bytes");
  for (const failImage of [true, false]) {
    if (!failImage) writeFileSync(join(output, "changelog/changelog.json"), "{}");
    const failed = bash(output, { ...fixture.env, FAIL_CONTAINER: failImage ? "1" : "0" }, "install-changelog.sh", "--deployment-dir", deployment, "--web-image", "multica-web:dev");
    assert.notEqual(failed.status, 0);
    assert.equal(readFileSync(join(deployment, ".env"), "utf8"), envBytes);
    assert.equal(readFileSync(join(deployment, "changelog/changelog.json"), "utf8"), "previous bytes");
  }
});

test("combined archive rejects an unapproved prerelease before starting builds", (t) => {
  const fixture = offlineFixture(t);
  writeFileSync(join(fixture.cwd, "bin/pnpm"), "#!/bin/sh\nexit 92\n", { mode: 0o755 });
  const built = bash(fixture.cwd, fixture.env, "scripts/offline-installer.sh", "--output", "combined", "--desktop-target", "win-ia32");
  assert.notEqual(built.status, 0);
  assert.match(built.stderr, /stable VERSION or add --allow-prerelease/);
  assert.equal(existsSync(fixture.env.DOCKER_LOG), false, "no Docker build may start for a rejected desktop version");
  assert.equal(existsSync(join(fixture.cwd, "combined")), false);
});

test("combined desktop/server archive forwards an explicit cumulative artifact and includes complete desktop update files", (t) => {
  const fixture = offlineFixture(t);
  const feed = seed();
  feed.releases[0].title = "Explicit offline artifact fixture";
  const input = join(fixture.cwd, "selected-changelog.json");
  writeFileSync(input, JSON.stringify(feed, null, 2) + "\n");
  writeFileSync(join(fixture.cwd, "bin/pnpm"), `#!${process.execPath}
import { mkdirSync, writeFileSync } from 'node:fs';
import { createHash } from 'node:crypto';
if (!process.argv.includes('never')) process.exit(30);
if (!process.argv.includes('--ia32')) process.exit(31);
const directory = 'apps/desktop/dist/win-ia32';
mkdirSync(directory, { recursive: true });
const name = 'multica-desktop-1.0.0-fixture-windows-ia32.exe';
const bytes = 'fixture installer';
const hash = createHash('sha512').update(bytes).digest('base64');
writeFileSync(directory + '/' + name, bytes);
writeFileSync(directory + '/' + name + '.blockmap', 'fixture blockmap');
writeFileSync(directory + '/latest-ia32.yml', JSON.stringify({ version: '1.0.0-fixture', files: [{ url: name, sha512: hash, size: Buffer.byteLength(bytes) }], path: name, sha512: hash }));
writeFileSync(directory + '/builder-debug.yml', 'debug');
`, { mode: 0o755 });
  const built = bash(fixture.cwd, fixture.env, "scripts/offline-installer.sh", "--output", "combined", "--desktop-target", "win-ia32", "--allow-prerelease", "--changelog", input);
  assert.equal(built.status, 0, built.stderr);
  const archive = readdirSync(join(fixture.cwd, "combined")).find((name) => name.endsWith(".tar.gz"));
  const listing = spawnSync("tar", ["-tzf", join(fixture.cwd, "combined", archive)], { encoding: "utf8" });
  for (const file of ["server/changelog/changelog.json", "server/install-changelog.sh", "server/scripts/install-changelog.mjs", "desktop/multica-desktop-1.0.0-fixture-windows-ia32.exe", "desktop/multica-desktop-1.0.0-fixture-windows-ia32.exe.blockmap", "desktop/latest-ia32.yml"]) assert.ok(listing.stdout.includes(file), `combined archive omitted ${file}`);
  assert.equal(listing.stdout.includes("builder-debug.yml"), false);
  const packageDir = join(fixture.cwd, "combined", archive.slice(0, -7));
  assert.equal(readFileSync(join(packageDir, "server/changelog/changelog.json"), "utf8"), readFileSync(input, "utf8"));
  assert.equal(readFileSync(fixture.env.DOCKER_LOG + ".embedded", "utf8"), readFileSync(input, "utf8"));
  assert.match(readFileSync(join(packageDir, "README.md"), "utf8"), /bash install-changelog.sh --deployment-dir "\$PWD"/);
});

test("actual loaded frontend image installs with UID permissions and durable Compose dotenv escaping", { skip: process.env.MULTICA_RUN_DOCKER_CHANGELOG_SMOKE !== "1" }, (t) => {
  // Docker Desktop may not share macOS's per-user temporary directory. Keep
  // this explicitly enabled integration fixture in the shared checkout.
  const shared = resolve(scripts, "../.changelog-build");
  mkdirSync(shared, { recursive: true });
  const directory = mkdtempSync(join(shared, "smoke-"));
  t.after(() => rmSync(directory, { recursive: true, force: true }));
  const fixture = offlineFixture(t, directory);
  const output = join(fixture.cwd, "bundle");
  const built = bash(fixture.cwd, fixture.env, "scripts/offline-bundle.sh", "--output", output);
  assert.equal(built.status, 0, built.stderr);
  const deployment = join(fixture.cwd, "real-deployment");
  mkdirSync(deployment);
  writeFileSync(join(deployment, ".env"), "JWT_SECRET=fixture-secret\nOTHER_KEY=keep-value\n");
  const docker = spawnSync("which", ["docker"], { encoding: "utf8" }).stdout.trim();
  assert.ok(docker, "the opt-in smoke test needs an installed Docker CLI");
  const feedDirectory = join(deployment, "a's $notes $${literal}\\literal");
  const environment = { ...process.env, PATH: `${dirname(docker)}:/usr/bin:/bin:/usr/sbin:/sbin`, CHANGELOG_DIRECTORY: feedDirectory, CHANGELOG_FILE: "", JWT_SECRET: "fixture-secret" };
  assert.notEqual(spawnSync("node", ["--version"], { env: environment }).status, 0);
  const installed = bash(output, environment, "install-changelog.sh", "--deployment-dir", deployment, "--web-image", process.env.MULTICA_CHANGELOG_SMOKE_IMAGE ?? "multica-web:dev");
  assert.equal(installed.status, 0, installed.stderr);
  assert.equal(readFileSync(join(feedDirectory, "changelog.json"), "utf8"), readFileSync(join(output, "changelog/changelog.json"), "utf8"));
  delete environment.CHANGELOG_DIRECTORY;
  delete environment.CHANGELOG_FILE;
  const resolved = spawnSync(docker, ["compose", "--project-directory", deployment, "--env-file", join(deployment, ".env"), "-f", join(output, "docker-compose.selfhost.yml"), "config", "--environment"], { env: environment, encoding: "utf8" });
  assert.equal(resolved.status, 0, resolved.stderr);
  assert.ok(resolved.stdout.split("\n").includes(`CHANGELOG_DIRECTORY=${feedDirectory}`));
  assert.ok(resolved.stdout.split("\n").includes("CHANGELOG_FILE=/app/data/changelog/changelog.json"));
});

test("real shell entry rejects CR/LF directory values before mkdir or any bind mount", { skip: process.env.MULTICA_RUN_DOCKER_CHANGELOG_SMOKE !== "1" }, async (t) => {
  const shared = resolve(scripts, "../.changelog-build");
  mkdirSync(shared, { recursive: true });
  const directory = mkdtempSync(join(shared, "shell-boundary-"));
  t.after(() => rmSync(directory, { recursive: true, force: true }));
  const fixture = offlineFixture(t, directory);
  const output = join(fixture.cwd, "bundle");
  assert.equal(bash(fixture.cwd, fixture.env, "scripts/offline-bundle.sh", "--output", output).status, 0);
  const docker = spawnSync("which", ["docker"], { encoding: "utf8" }).stdout.trim();
  const wrapper = join(fixture.cwd, "real-docker-bin");
  mkdirSync(wrapper);
  writeFileSync(join(wrapper, "docker"), '#!/bin/sh\nprintf "%s\\n" "$@" >> "$MULTICA_DOCKER_TRACE"\nexec "$MULTICA_REAL_DOCKER" "$@"\n', { mode: 0o755 });
  for (const [index, suffix] of ["\nsecond-line", "\n", "\rsecond-line"].entries()) {
    await t.test(JSON.stringify(suffix), () => {
      const deployment = join(fixture.cwd, `deployment-${index}`);
      mkdirSync(join(deployment, "changelog"), { recursive: true });
      const envFile = join(deployment, ".env");
      const feed = join(deployment, "changelog/changelog.json");
      const originalEnv = "JWT_SECRET=fixture-secret\nOTHER_KEY=keep-value\n";
      writeFileSync(envFile, originalEnv);
      writeFileSync(feed, "previous feed bytes");
      const prefix = join(deployment, "must-not-be-created");
      const invalid = prefix + suffix;
      const trace = join(deployment, "docker-args.txt");
      const environment = { ...process.env, PATH: `${wrapper}:${dirname(docker)}:/usr/bin:/bin:/usr/sbin:/sbin`, CHANGELOG_DIRECTORY: invalid, CHANGELOG_FILE: "", JWT_SECRET: "fixture-secret", MULTICA_REAL_DOCKER: docker, MULTICA_DOCKER_TRACE: trace };
      assert.notEqual(spawnSync("node", ["--version"], { env: environment }).status, 0);
      const result = bash(output, environment, "install-changelog.sh", "--deployment-dir", deployment, "--web-image", process.env.MULTICA_CHANGELOG_SMOKE_IMAGE ?? "multica-web:dev");
      assert.notEqual(result.status, 0, "invalid directory must not be truncated into an accepted path");
      assert.equal(existsSync(prefix), false);
      assert.equal(existsSync(invalid), false);
      assert.equal(readFileSync(envFile, "utf8"), originalEnv);
      assert.equal(readFileSync(feed, "utf8"), "previous feed bytes");
      assert.equal(readFileSync(trace, "utf8").split("\n").includes("--mount"), false);
    });
  }
});

test("real shell entry preserves quotes and commas in every Docker bind source", { skip: process.env.MULTICA_RUN_DOCKER_CHANGELOG_SMOKE !== "1" }, async (t) => {
  const shared = resolve(scripts, "../.changelog-build");
  mkdirSync(shared, { recursive: true });
  const directory = mkdtempSync(join(shared, "mount-csv-"));
  t.after(() => rmSync(directory, { recursive: true, force: true }));
  const fixture = offlineFixture(t, directory);
  const output = join(fixture.cwd, "bundle");
  assert.equal(bash(fixture.cwd, fixture.env, "scripts/offline-bundle.sh", "--output", output).status, 0);
  const docker = spawnSync("which", ["docker"], { encoding: "utf8" }).stdout.trim();
  for (const [index, suffix] of ["quote-only", '"quote,comma'].entries()) {
    await t.test(suffix, () => {
      const packageDir = join(fixture.cwd, `package-${suffix}`);
      cpSync(output, packageDir, { recursive: true });
      const deployment = join(fixture.cwd, `deployment-${suffix}`);
      mkdirSync(deployment);
      const envFile = join(deployment, ".env");
      writeFileSync(envFile, "JWT_SECRET=fixture-secret\nOTHER_KEY=preserved\n");
      const feedDirectory = join(deployment, index === 0 ? 'feed"quote' : 'feed"quote,comma');
      const environment = { ...process.env, PATH: `${dirname(docker)}:/usr/bin:/bin:/usr/sbin:/sbin`, CHANGELOG_DIRECTORY: feedDirectory, CHANGELOG_FILE: "", JWT_SECRET: "fixture-secret" };
      assert.notEqual(spawnSync("node", ["--version"], { env: environment }).status, 0);
      const installed = bash(packageDir, environment, "install-changelog.sh", "--deployment-dir", deployment, "--web-image", process.env.MULTICA_CHANGELOG_SMOKE_IMAGE ?? "multica-web:dev");
      assert.equal(installed.status, 0, installed.stderr);
      assert.equal(readFileSync(join(feedDirectory, "changelog.json"), "utf8"), readFileSync(join(output, "changelog/changelog.json"), "utf8"));
      assert.ok(readFileSync(envFile, "utf8").includes("OTHER_KEY=preserved\n"));
      delete environment.CHANGELOG_DIRECTORY;
      delete environment.CHANGELOG_FILE;
      const resolved = spawnSync(docker, ["compose", "--project-directory", deployment, "--env-file", envFile, "-f", join(packageDir, "docker-compose.selfhost.yml"), "config", "--environment"], { env: environment, encoding: "utf8" });
      assert.equal(resolved.status, 0, resolved.stderr);
      assert.ok(resolved.stdout.split("\n").includes(`CHANGELOG_DIRECTORY=${feedDirectory}`));
    });
  }
});
