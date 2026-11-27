import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const readJSON = (relative) => JSON.parse(readFileSync(new URL(relative, import.meta.url), 'utf8'));
const material = readJSON('./assets/.material/schemas/material.schema.json');
const requirement = readJSON('./assets/.material/schemas/requirement.schema.json');
const schemas = {
  material_ref: material.$defs.material_ref.properties.path,
  pinned_ref: material.$defs.pinned_ref.properties.path,
  source: requirement.properties.sources.items.properties.path,
};
const cases = readJSON('./testdata/path-cases.json');
for (const [name, schema] of Object.entries(schemas)) {
  test(`${name} path pattern accepts the same scoped paths as the Go kernel`, () => {
    const expression = new RegExp(schema.pattern, 'u');
    for (const { path, valid } of cases) {
      const allowed = path.length >= schema.minLength && path.length <= schema.maxLength && expression.test(path);
      assert.equal(allowed, valid, JSON.stringify(path));
    }
  });
}
