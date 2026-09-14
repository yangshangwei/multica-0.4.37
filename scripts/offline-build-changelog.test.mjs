import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { cpSync, existsSync, mkdirSync, readFileSync, readdirSync, writeFileSync } from "node:fs";
import { join, resolve } from "node:path";
import test from "node:test";
import { gunzipSync } from "node:zlib";
import { scratch, scripts, seed } from "./changelog-test-helpers.mjs";

const composeAvailable = spawnSync("docker", ["compose", "version"], { encoding: "utf8" }).status === 0;

test("self-host builds stamp both images with the selected version and default to dev", { skip: !composeAvailable }, async (t) => {
  for (const version of [undefined, "v0.4.45"]) {
    await t.test(version ?? "default dev", () => {
      const env = { ...process.env, JWT_SECRET: "fixture-secret" };
      if (version === undefined) delete env.VERSION;
      else env.VERSION = version;
      const result = spawnSync("docker", [
        "compose", "--env-file", "/dev/null",
        "-f", resolve(scripts, "../docker-compose.selfhost.yml"),
        "-f", resolve(scripts, "../docker-compose.selfhost.build.yml"),
        "config", "--format", "json",
      ], { env, encoding: "utf8" });
      assert.equal(result.status, 0, result.stderr);
      const { backend, frontend } = JSON.parse(result.stdout).services;
      const expected = version ?? "dev";
      assert.deepEqual({
        backendImage: backend.image,
        backendVersion: backend.build.args.VERSION,
        frontendImage: frontend.image,
        frontendVersion: frontend.build.args.NEXT_PUBLIC_APP_VERSION,
      }, {
        backendImage: `multica-backend:${expected}`,
        backendVersion: expected,
        frontendImage: `multica-web:${expected}`,
        frontendVersion: expected,
      });
    });
  }
});

function fixture(t) {
  const cwd = scratch(t);
  for (const directory of ["scripts", "bin", "docs", "server/internal/changelog/content"]) {
    mkdirSync(join(cwd, directory), { recursive: true });
  }
  for (const name of ["offline-bundle.sh", "build-offline-upgrade.sh", "offline-upgrade.sh", "changelog-lib.mjs", "publish-changelog.mjs", "install-changelog.sh", "install-changelog.mjs"]) {
    cpSync(join(scripts, name), join(cwd, "scripts", name));
  }
  for (const name of ["docker-compose.selfhost.yml", "docker-compose.selfhost.build.yml", ".env.example"]) {
    cpSync(resolve(scripts, "..", name), join(cwd, name));
  }
  writeFileSync(join(cwd, "server/internal/changelog/content/changelog.json"), JSON.stringify(seed(), null, 2) + "\n");
  writeFileSync(join(cwd, "docs/offline-upgrade.zh-CN.md"), "Fixture upgrade guide\n");
  writeFileSync(join(cwd, ".env"), "VERSION=v0.0.1\nJWT_SECRET=private-build-host-secret\n");
  // Stub only Docker's external build/pull/save boundary. Return deliberately
  // unordered image names, as Compose does, and record no runtime secrets.
  writeFileSync(join(cwd, "bin/docker"), `#!${process.execPath}
import { appendFileSync, readFileSync, writeFileSync } from 'node:fs';
const args = process.argv.slice(2);
appendFileSync(process.env.DOCKER_LOG, JSON.stringify({ args, version: process.env.VERSION, platform: process.env.DOCKER_DEFAULT_PLATFORM }) + '\\n');
if (args[0] === 'compose' && args.includes('version')) process.stdout.write('2.39.4\\n');
else if (args[0] === 'compose' && args.includes('port')) process.stdout.write('127.0.0.1:' + args.at(-1) + '\\n');
else if (args[0] === 'compose' && args.includes('up')) { /* No running containers in script tests. */ }
else if (args[0] === 'compose' && args.includes('config') && args.includes('--images')) {
  if (process.env.FAIL_CONFIG === '1') process.exit(23);
  process.stdout.write(['pgvector/pgvector:pg17', 'multica-web:' + process.env.VERSION, 'multica-backend:' + process.env.VERSION].join('\\n') + '\\n');
} else if (args[0] === 'compose' && args.includes('build')) {
  if (!process.env.CHANGELOG_ARTIFACT_PATH?.startsWith('.changelog-build/')) process.exit(24);
  writeFileSync(process.env.DOCKER_LOG + '.embedded', readFileSync(process.env.CHANGELOG_ARTIFACT_PATH));
} else if (args[0] === 'save') process.stdout.write(JSON.stringify(args.slice(3)));
else if (args[0] !== 'pull') process.exit(25);
`, { mode: 0o755 });
  const env = {
    ...process.env,
    PATH: `${join(cwd, "bin")}:${process.env.PATH}`,
    VERSION: "v0.4.45",
    COMMIT: "fixture",
    DOCKER_LOG: join(cwd, "docker.log"),
    JWT_SECRET: "private-build-host-secret",
  };
  return { cwd, env };
}

function bash({ cwd, env }, ...args) {
  return spawnSync("/bin/bash", args, { cwd, env, encoding: "utf8" });
}

function calls({ env }) {
  return readFileSync(env.DOCKER_LOG, "utf8").trim().split("\n").map(JSON.parse);
}

test("make selfhost-build defaults to dev and respects environment, command-line and env-file versions", async (t) => {
  const cases = [
    { name: "unspecified", expected: "dev" },
    { name: "environment", environment: "v0.4.45", expected: "v0.4.45" },
    { name: "command line", environment: "v0.4.43", envFile: "v0.4.44", argument: "v0.4.45", expected: "v0.4.45" },
    { name: "env file overrides environment", environment: "v0.4.44", envFile: "v0.4.45", expected: "v0.4.45" },
  ];
  for (const scenario of cases) {
    await t.test(scenario.name, (t) => {
      const f = fixture(t);
      cpSync(resolve(scripts, "../Makefile"), join(f.cwd, "Makefile"));
      cpSync(join(scripts, "selfhost-wait.sh"), join(f.cwd, "scripts/selfhost-wait.sh"));
      writeFileSync(join(f.cwd, "bin/curl"), "#!/bin/sh\nexit 0\n", { mode: 0o755 });
      writeFileSync(join(f.cwd, ".env"), "JWT_SECRET=fixture-secret\n" + (scenario.envFile ? `VERSION=${scenario.envFile}\n` : ""));
      if (scenario.environment) f.env.VERSION = scenario.environment;
      else delete f.env.VERSION;
      const result = spawnSync("make", ["--no-print-directory", "selfhost-build", ...(scenario.argument ? [`VERSION=${scenario.argument}`] : [])], {
        cwd: f.cwd, env: f.env, encoding: "utf8",
      });
      assert.equal(result.status, 0, result.stderr);
      assert.equal(calls(f).find(({ args }) => args.includes("up")).version, scenario.expected);
      assert.ok(result.stdout.includes(`Local tags: multica-backend:${scenario.expected} and multica-web:${scenario.expected}.`), result.stdout);
    });
  }
});

test("generic Makefile targets retain their inferred version", (t) => {
  const f = fixture(t);
  delete f.env.VERSION;
  cpSync(resolve(scripts, "../Makefile"), join(f.cwd, "Makefile"));
  writeFileSync(join(f.cwd, ".env"), "JWT_SECRET=fixture-secret\n");
  writeFileSync(join(f.cwd, "bin/git"), "#!/bin/sh\nprintf 'v0.4.44-1-gdeadbeef\\n'\n", { mode: 0o755 });
  const result = spawnSync("make", ["--no-print-directory", "-f", "Makefile", "-f", "-", "version-probe"], {
    cwd: f.cwd, env: f.env, encoding: "utf8", input: "version-probe:\n\t@printf '%s\\n' '$(VERSION)'\n",
  });
  assert.equal(result.status, 0, result.stderr);
  assert.equal(result.stdout.trim(), "v0.4.44-1-gdeadbeef");
});

test("offline bundle saves its versioned build images and records the same identities beside the exact feed", (t) => {
  const f = fixture(t);
  const output = join(f.cwd, "bundle");
  const input = join(f.cwd, "release-changelog.json");
  const feed = seed();
  feed.releases[0].title = "Selected release feed";
  writeFileSync(input, JSON.stringify(feed, null, 2) + "\n");
  const built = bash(f, "scripts/offline-bundle.sh", "--output", output, "--platform", "linux/arm64", "--changelog", input);
  assert.equal(built.status, 0, built.stderr);
  const images = ["multica-backend:v0.4.45", "multica-web:v0.4.45", "pgvector/pgvector:pg17"];
  const manifest = readFileSync(join(output, "MANIFEST.txt"), "utf8");
  assert.match(manifest, /^version:  v0\.4\.45$/m);
  assert.match(manifest, /^platform: linux\/arm64$/m);
  assert.deepEqual(manifest.match(/^images:\n((?:  .+\n)+)/m)?.[1].trim().split("\n").map((line) => line.trim()), images);
  assert.deepEqual(JSON.parse(gunzipSync(readFileSync(join(output, "multica-images.tar.gz")))), images);
  const commands = calls(f);
  const build = commands.find(({ args }) => args[0] === "compose" && args.includes("build"));
  assert.equal(build.version, "v0.4.45");
  assert.equal(build.platform, "linux/arm64");
  assert.deepEqual(commands.find(({ args }) => args[0] === "save").args, ["save", "--platform", "linux/arm64", ...images]);
  assert.deepEqual(commands.find(({ args }) => args[0] === "pull").args, ["pull", "--platform", "linux/arm64", images[2]]);
  assert.equal(readFileSync(f.env.DOCKER_LOG + ".embedded", "utf8"), readFileSync(input, "utf8"));
  assert.equal(readFileSync(join(output, "changelog/changelog.json"), "utf8"), readFileSync(input, "utf8"));
  assert.match(readFileSync(join(output, "README.md"), "utf8"), /MULTICA_IMAGE_TAG=v0\.4\.45/);
  assert.equal(readFileSync(join(output, ".env.example"), "utf8"), readFileSync(join(f.cwd, ".env.example"), "utf8"));
  assert.equal(existsSync(join(output, ".env")), false);
  assert.equal((built.stdout + built.stderr + manifest).includes(f.env.JWT_SECRET), false);
  assert.deepEqual(readdirSync(join(f.cwd, ".changelog-build")), []);
});

test("dry-run resolves the selected version without building, pulling or staging files", (t) => {
  const f = fixture(t);
  const output = join(f.cwd, "bundle");
  const result = bash(f, "scripts/offline-bundle.sh", "--output", output, "--dry-run");
  assert.equal(result.status, 0, result.stderr);
  assert.match(result.stdout, /multica-backend:v0\.4\.45/);
  assert.match(result.stdout, /multica-web:v0\.4\.45/);
  assert.match(result.stdout, /linux\/amd64/);
  assert.equal(existsSync(output), false);
  assert.equal(existsSync(join(f.cwd, ".changelog-build")), false);
  assert.ok(calls(f).every(({ args }) => args[0] === "compose" && args.includes("config") && args.includes("--images")));
});

test("failed image resolution stops before any build or output is staged", (t) => {
  const f = fixture(t);
  f.env.FAIL_CONFIG = "1";
  const output = join(f.cwd, "bundle");
  const result = bash(f, "scripts/offline-bundle.sh", "--output", output);
  assert.notEqual(result.status, 0);
  assert.equal(existsSync(output), false);
  assert.ok(calls(f).every(({ args }) => args.includes("config")));
});

test("upgrade wrapper freezes its inferred version for the child build and archive", (t) => {
  const f = fixture(t);
  delete f.env.VERSION;
  // Simulate the checkout changing between wrapper and child invocations. The
  // child must receive the already selected version instead of deriving again.
  writeFileSync(join(f.cwd, "bin/git"), `#!${process.execPath}
import { existsSync, readFileSync, writeFileSync } from 'node:fs';
const counter = process.env.DOCKER_LOG + '.describe-count';
if (process.argv[2] === 'describe') {
  const count = existsSync(counter) ? Number(readFileSync(counter, 'utf8')) : 0;
  writeFileSync(counter, String(count + 1));
  process.stdout.write(count === 0 ? 'v0.4.45\\n' : 'v0.4.46\\n');
} else if (process.argv[2] === 'rev-parse') process.stdout.write('fixture\\n');
else process.exit(26);
`, { mode: 0o755 });
  const result = bash(f, "scripts/build-offline-upgrade.sh", "--output", "upgrade");
  assert.equal(result.status, 0, result.stderr);
  const name = "multica-server-upgrade-v0.4.45-linux-amd64";
  const packageDir = join(f.cwd, "upgrade", name);
  assert.ok(existsSync(`${packageDir}.tar.gz`));
  assert.ok(existsSync(`${packageDir}.tar.gz.sha256`));
  assert.match(readFileSync(join(packageDir, "MANIFEST.txt"), "utf8"), /^version:  v0\.4\.45$/m);
  assert.equal(calls(f).find(({ args }) => args.includes("build")).version, "v0.4.45");
  assert.equal(readFileSync(f.env.DOCKER_LOG + ".describe-count", "utf8"), "1");
});
