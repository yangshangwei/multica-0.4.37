import assert from "node:assert/strict";
import { Buffer } from "node:buffer";
import { spawnSync } from "node:child_process";
import { chmodSync, mkdtempSync, mkdirSync, readFileSync, realpathSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import process from "node:process";
import { fileURLToPath, pathToFileURL } from "node:url";
import test from "node:test";
import ts from "typescript";

const fixturePath = join(dirname(fileURLToPath(import.meta.url)), "collaboration-stage-execution.ts");
const source = readFileSync(fixturePath, "utf8");
const tree = ts.createSourceFile(fixturePath, source, ts.ScriptTarget.Latest, true);
const names = new Set(["stageArtifactEnvironment", "verifyStageArtifactTar"]);
const declarations = tree.statements.filter((node) =>
  ts.isFunctionDeclaration(node) && node.name && names.has(node.name.text) ||
  ts.isVariableStatement(node) && node.declarationList.declarations.some((item) => ts.isIdentifier(item.name) && ["stageBytesHash", "canonicalUUID"].includes(item.name.text)));
assert.equal(declarations.length, 4);
const directory = mkdtempSync(join(tmpdir(), "stage-artifact-module-")); chmodSync(directory, 0o700);
const modulePath = join(directory, "artifact.mjs");
const moduleSource = `import {createHash} from "node:crypto";import {lstatSync,realpathSync} from "node:fs";import {isAbsolute,normalize} from "node:path";\n${declarations.map((node) => node.getText(tree)).join("\n")}`;
writeFileSync(modulePath, ts.transpileModule(moduleSource, { compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext } }).outputText);
const { verifyStageArtifactTar, stageArtifactEnvironment } = await import(pathToFileURL(modulePath).href);
process.on("exit", () => rmSync(directory, { recursive: true, force: true }));

test("actual tar verifier accepts independent USTAR output and rejects wrong source, headers, extra entries and padding", () => {
  const bytes = Buffer.from('{"version":1,"callbacks":["a","a"],"expected_effects":1}');
  // Python's standard tar writer is independent of the Go product and TS validator.
  const generated = spawnSync("python3", ["-c", `import io,sys,tarfile\ntarfile.RECORDSIZE=512\nb=sys.stdin.buffer.read();o=io.BytesIO()\nwith tarfile.open(fileobj=o,mode='w',format=tarfile.USTAR_FORMAT) as t:\n i=tarfile.TarInfo('stage-regression.json');i.size=len(b);i.mode=0o644;i.mtime=0;t.addfile(i,io.BytesIO(b))\nsys.stdout.buffer.write(o.getvalue())`],
    { input: bytes, timeout: 20_000, maxBuffer: 32 * 1024 });
  assert.equal(generated.status, 0);
  const archive = generated.stdout;
  const proof = verifyStageArtifactTar(archive, bytes);
  assert.match(proof.artifact_digest, /^[0-9a-f]{64}$/);
  assert.equal(proof.size_bytes, archive.length);
  assert.throws(() => verifyStageArtifactTar(archive, Buffer.from("wrong source")));
  for (const [end, replacement] of [[108, 120], [116, 120], [124, 120], [136, 120], [148, 120], [156, 120], [116, 48]]) {
    const invalidOctal = Buffer.from(archive);
    invalidOctal[end - 1] = replacement;
    if (end !== 156) {
      invalidOctal.fill(32, 148, 156);
      const checksum = invalidOctal.subarray(0, 512).reduce((sum, byte) => sum + byte, 0);
      invalidOctal.write(`${checksum.toString(8).padStart(6, "0")}\0 `, 148, "latin1");
    }
    assert.throws(() => verifyStageArtifactTar(invalidOctal, bytes), `Invalid numeric field ending at ${end} must not be accepted through a valid checksum`);
  }
  for (const index of [0, 100, 156, 257, 345, 512, archive.length - 1]) {
    const bad = Buffer.from(archive); bad[index] ^= 1;
    assert.throws(() => verifyStageArtifactTar(bad, bytes));
  }
  assert.throws(() => verifyStageArtifactTar(Buffer.concat([archive, archive]), bytes));
  assert.throws(() => verifyStageArtifactTar(archive.subarray(0, archive.length - 512), bytes));
});

test("artifact generation allocation requires exact privately owned immutable-root modes and unique slots", () => {
  const root = realpathSync(mkdtempSync(join(tmpdir(), "stage-artifact-allocation-")));
  chmodSync(root, 0o700);
  const staging = join(root, "staging"), output = join(root, "output");
  mkdirSync(staging, { mode: 0o500 }); mkdirSync(output, { mode: 0o700 });
  const keys = ["MULTICA_ARTIFACT_STAGING_ROOT", "MULTICA_ARTIFACT_OUTPUT_ROOT", "COLLABORATION_STAGE_ARTIFACT_SLOTS"];
  const prior = keys.map((key) => process.env[key]);
  const slots = Array.from({ length: 8 }, (_, i) => `11111111-1111-4111-8111-${String(i + 1).padStart(12, "0")}`);
  try {
    process.env.MULTICA_ARTIFACT_STAGING_ROOT = staging; process.env.MULTICA_ARTIFACT_OUTPUT_ROOT = output;
    process.env.COLLABORATION_STAGE_ARTIFACT_SLOTS = JSON.stringify(slots);
    assert.deepEqual(stageArtifactEnvironment(), { staging, output, slots });
    chmodSync(staging, 0o700); assert.throws(() => stageArtifactEnvironment()); chmodSync(staging, 0o500);
    process.env.COLLABORATION_STAGE_ARTIFACT_SLOTS = JSON.stringify([...slots.slice(0, 7), slots[0]]);
    assert.throws(() => stageArtifactEnvironment());
    process.env.COLLABORATION_STAGE_ARTIFACT_SLOTS = JSON.stringify(slots.slice(0, 7));
    assert.throws(() => stageArtifactEnvironment());
  } finally {
    for (let i = 0; i < keys.length; i++) { if (prior[i] === undefined) delete process.env[keys[i]]; else process.env[keys[i]] = prior[i]; }
    chmodSync(staging, 0o700); rmSync(root, { recursive: true, force: true });
  }
});
