const fs = require("node:fs");
const vm = require("node:vm");
const crypto = require("node:crypto");
const ts = require("/Volumes/artisan/code/2026/mc-collaboration-closure-v12-20261103/node_modules/typescript");
const sourcePath = "/Volumes/artisan/code/2026/mc-collaboration-closure-v12-20261103/e2e/fixtures/collaboration-native.ts";
const source = fs.readFileSync(sourcePath, "utf8");
const ast = ts.createSourceFile(sourcePath, source, ts.ScriptTarget.Latest, true);
const found = new Map();
function visit(node) {
  if (ts.isVariableDeclaration(node) && ["sameLayout", "samePart"].includes(node.name.getText(ast))) found.set(node.name.getText(ast), node.initializer.getText(ast));
  ts.forEachChild(node, visit);
}
visit(ast);
if (found.size !== 2) throw new Error("Actual private predicates were not found");
const extracted = [...found].map(([name, initializer]) => `const ${name} = ${initializer};`).join("\n") + "\n({sameLayout,samePart})";
const compiled = ts.transpileModule(extracted, {compilerOptions: {target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS}}).outputText;
const predicates = vm.runInNewContext(compiled, {}, {timeout: 1000});
const observations = JSON.parse(fs.readFileSync("/Volumes/artisan/code/2026/multica-0.4.37/.trellis/tasks/11-01-collaboration-knowledge-audit-fixes/research/progress-recheck-20261103-01a1a1c6-97cddb08/v15-native-capture/geometry-observations.json", "utf8"));
const actual = observations.geometries.find(x => x.data.stage === "adoption" && x.data.size === "200-percent").data.observations[0];
const clone = () => JSON.parse(JSON.stringify(actual));
const cases = [];
function check(name, mutate, layout, part) {
  const changed = clone(); mutate(changed);
  const observed = {layout: predicates.sameLayout(actual, changed), part: predicates.samePart(actual, changed)};
  if (observed.layout !== layout || observed.part !== part) throw new Error(name + ": " + JSON.stringify(observed));
  cases.push({name, expected: {layout, part}, observed});
}
check("same actual frame", () => {}, true, true);
check("planned next-tile vertical scroll remains globally legal", x => {x.focus.y += 100; x.focus.top += 100; x.focus.bottom += 100; x.scroll.top += 100;}, true, false);
check("below-target content growth does not freeze whole scrollHeight", x => {x.scroll.scrollHeight += 500; x.scroll.maxScroll += 500;}, true, true);
check("changed target height refuses frozen layout", x => {x.focus.height += 2; x.focus.bottom += 2;}, false, false);
check("changed target horizontal position refuses frozen layout", x => {x.focus.x += 2; x.focus.left += 2; x.focus.right += 2;}, false, false);
check("changed root vertical position refuses frozen layout", x => {x.root.y += 2; x.root.top += 2; x.root.bottom += 2;}, false, false);
check("changed viewport integer height refuses frozen layout", x => {x.viewport.height += 1;}, false, false);
check("changed root clientHeight refuses frozen layout", x => {x.scroll.clientHeight += 1;}, false, false);
check("post-capture actual scroll movement refuses part", x => {x.scroll.top += 2;}, true, false);
check("existing one-pixel geometry precision remains allowed", x => {x.focus.height += 1; x.focus.bottom += 1;}, true, true);
const result = {sourcePath, sourceSha256: crypto.createHash("sha256").update(source).digest("hex"), cases, count: cases.length, scope: "Actual private predicates extracted from current source, checked against declared snapshot drift/allowed scrolling contract; no UI, DB, service, Provider, or duplicate E2E execution"};
fs.writeFileSync(__dirname + "/predicate-controls-result.json", JSON.stringify(result, null, 2) + "\n");
process.stdout.write(JSON.stringify({count: cases.length, all_passed: true, sourceSha256: result.sourceSha256}) + "\n");
