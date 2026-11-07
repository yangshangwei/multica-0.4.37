const fs = require('node:fs');
const path = require('node:path');
const { createRequire } = require('node:module');
const root = '/Volumes/artisan/code/2026/mc-collaboration-closure-v12-20261103';
const ts = createRequire(path.join(root, 'package.json'))('typescript');
const files = [
  'e2e/collaboration-boundaries-web.spec.ts',
  'e2e/collaboration-workbench-web.spec.ts',
  'e2e/collaboration-memory-web.spec.ts',
  'e2e/fixtures/collaboration-mention-journeys.ts',
  'e2e/collaboration-preview-web.spec.ts',
  'e2e/collaboration-preparation-records-web.spec.ts',
  'e2e/settings-preferences.spec.ts',
];
const program = ts.createProgram(files.map(file => path.join(root, file)), {
  target: ts.ScriptTarget.ES2022,
  lib: ['lib.dom.d.ts', 'lib.dom.iterable.d.ts', 'lib.esnext.d.ts'],
  module: ts.ModuleKind.ESNext,
  moduleResolution: ts.ModuleResolutionKind.Bundler,
  noEmit: true,
  strict: true,
  skipLibCheck: true,
  esModuleInterop: true,
  allowJs: true,
  resolveJsonModule: true,
  jsx: ts.JsxEmit.ReactJSX,
});
const diagnostics = ts.getPreEmitDiagnostics(program).map(d => ({
  file: d.file && path.relative(root, d.file.fileName),
  code: d.code,
  message: ts.flattenDiagnosticMessageText(d.messageText, '\n'),
}));
const result = { typescript_version: ts.version, root_files: files, method: 'strict TypeScript noEmit over final seven-file E2E overlay and imports', diagnostics, exit_code: diagnostics.length ? 1 : 0 };
fs.writeFileSync(path.join(__dirname, 'preservation-final-types.json'), JSON.stringify(result, null, 2) + '\n', { flag: 'wx' });
process.stdout.write(JSON.stringify(result) + '\n');
process.exitCode = result.exit_code;
