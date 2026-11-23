import { createRequire } from 'node:module';
import { existsSync, readFileSync, writeFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { createHash } from 'node:crypto';

const evidenceDir = dirname(fileURLToPath(import.meta.url));
let root = evidenceDir;
while (!existsSync(resolve(root, 'pnpm-workspace.yaml'))) {
  const parent = dirname(root);
  if (parent === root) throw new Error('Repository workspace root not found');
  root = parent;
}
const webRequire = createRequire(resolve(root, 'apps/web/package.json'));
const uiRequire = createRequire(resolve(root, 'packages/ui/package.json'));
const ts = webRequire('typescript');
const postcss = webRequire('postcss');
const tailwind = webRequire('@tailwindcss/postcss');
const { twMerge } = uiRequire('tailwind-merge');
const read = (path) => readFileSync(resolve(root, path), 'utf8');
const sha256 = (path) => createHash('sha256').update(read(path)).digest('hex');
const sources = {
  action: 'packages/views/layout/collection-page.tsx',
  button: 'packages/ui/components/ui/button.tsx',
  tokens: 'packages/ui/styles/tokens.css',
  entry: 'apps/web/app/globals.css',
};

function parse(path) {
  return ts.createSourceFile(path, read(path), ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
}
function find(node, predicate) {
  if (predicate(node)) return node;
  return ts.forEachChild(node, (child) => find(child, predicate));
}
function stringValue(node) {
  if (!node || !ts.isStringLiteralLike(node)) throw new Error('Expected source string literal');
  return node.text;
}
function property(node, name) {
  if (!node || !ts.isObjectLiteralExpression(node)) throw new Error(`Expected source object for ${name}`);
  const entry = node.properties.find((value) => ts.isPropertyAssignment(value) && value.name.getText().replaceAll('"', '') === name);
  if (!entry) throw new Error(`Source property ${name} is missing`);
  return entry.initializer;
}
const buttonSource = parse(sources.button);
const cva = find(buttonSource, (node) => ts.isCallExpression(node) && node.expression.getText(buttonSource) === 'cva');
const variants = property(cva.arguments[1], 'variants');
const buttonBase = stringValue(cva.arguments[0]);
const outline = stringValue(property(property(variants, 'variant'), 'outline'));
const small = stringValue(property(property(variants, 'size'), 'sm'));
const brand = stringValue(property(property(variants, 'variant'), 'brand'));
const collectionSource = parse(sources.action);
const actionFunction = find(collectionSource, (node) => ts.isFunctionDeclaration(node) && node.name?.text === 'CollectionPageHeaderAction');
const actionClassCall = find(actionFunction, (node) => ts.isCallExpression(node) && node.expression.getText(collectionSource) === 'cn');
const actionClasses = stringValue(actionClassCall.arguments[0]);
const mergedClasses = twMerge(buttonBase, outline, small, actionClasses).split(/\s+/);

const entry = resolve(root, sources.entry);
const built = await postcss([tailwind({ base: dirname(entry) })]).process(read(sources.entry), { from: entry });
const flat = [];
let order = 0;
function walk(container, selector = '', media = [], layer = '') {
  container.each((node) => {
    if (node.type === 'decl') flat.push({ selector, property: node.prop, value: node.value, important: node.important === true, media, layer, order: order++ });
    else if (node.type === 'rule') {
      const joined = node.selector.includes('&') ? node.selector.replaceAll('&', selector) : node.selector;
      walk(node, joined, media, layer);
    } else if (node.type === 'atrule') {
      if (node.name === 'media') walk(node, selector, [...media, node.params], layer);
      else if (node.name === 'layer') walk(node, selector, media, node.params);
      else if (node.nodes) walk(node, selector, media, layer);
    }
  });
}
walk(built.root);
const classPattern = /^\.((?:[^.\s:[\\]|\\.)+)$/;
function exactClass(selector) {
  const match = selector.match(classPattern);
  return match ? match[1].replace(/\\(.)/g, '$1') : null;
}
const relevantProperties = new Set(['height', 'width', 'min-height', 'min-width', 'padding-inline', 'box-sizing', 'flex-shrink']);
const classDeclarations = flat.filter((rule) => relevantProperties.has(rule.property) && mergedClasses.includes(exactClass(rule.selector)));
const spacingRule = flat.find((rule) => rule.property === '--spacing' && !rule.media.length);
if (!spacingRule) throw new Error('Compiled spacing token not found');
const rootFontPx = 16;
function toPx(value) {
  if (value === 'auto' || value === 'none') return value;
  if (/^[\d.]+rem$/.test(value)) return Number.parseFloat(value) * rootFontPx;
  if (/^[\d.]+px$/.test(value)) return Number.parseFloat(value);
  const calculated = /^calc\(var\(--spacing\) \* ([\d.]+)\)$/.exec(value);
  if (calculated) return toPx(spacingRule.value) * Number(calculated[1]);
  if (value === '0') return 0;
  throw new Error(`Unmodelled CSS length ${value}`);
}
function matchesMedia(condition, scenario) {
  const width = /^\(width\s*(<|>=)\s*([\d.]+)(rem|px)\)$/.exec(condition);
  if (width) {
    const threshold = Number(width[2]) * (width[3] === 'rem' ? rootFontPx : 1);
    return width[1] === '<' ? scenario.viewportWidthPx < threshold : scenario.viewportWidthPx >= threshold;
  }
  const pointer = /^\(pointer:\s*(coarse|fine)\)$/.exec(condition);
  if (pointer) return scenario.pointer === pointer[1];
  throw new Error(`Unmodelled media condition for target dimensions: ${condition}`);
}
const scenarios = [
  { name: 'narrow fine pointer', viewportWidthPx: 390, pointer: 'fine' },
  { name: 'narrow coarse pointer', viewportWidthPx: 390, pointer: 'coarse' },
  { name: 'desktop fine pointer', viewportWidthPx: 1280, pointer: 'fine' },
  { name: 'desktop coarse pointer', viewportWidthPx: 1280, pointer: 'coarse' },
].map((scenario) => {
  const applied = classDeclarations.filter((rule) => rule.media.every((condition) => matchesMedia(condition, scenario)));
  const winners = {};
  for (const rule of applied) {
    const previous = winners[rule.property];
    if (!previous || rule.important || !previous.important) winners[rule.property] = rule;
  }
  const heightPx = toPx(winners.height.value);
  const width = toPx(winners.width.value);
  const minHeightPx = winners['min-height'] ? toPx(winners['min-height'].value) : 0;
  const minWidthPx = winners['min-width'] ? toPx(winners['min-width'].value) : 0;
  return {
    ...scenario,
    declarations: winners,
    resolved: {
      heightPx: Math.max(heightPx, minHeightPx),
      widthPx: width === 'auto' ? null : Math.max(width, minWidthPx),
      minWidthPx,
      widthBehavior: width === 'auto' ? 'content-sized, subject to min-width; no device/DOM text measurement performed' : 'fixed width constrained by min-width',
    },
  };
});
const sizingReport = {
  kind: 'Static compiled-CSS cascade evidence; not browser, touch, device, or hit-testing measurement',
  sourceHashes: Object.fromEntries(Object.entries(sources).map(([name, path]) => [name, { path, sha256: sha256(path) }])),
  method: 'Extract source classes using TypeScript AST; apply actual tailwind-merge composition; compile current Web entry with PostCSS/Tailwind; resolve matching dimension declarations and media queries',
  assumptions: { rootFontPx, defaultActionProps: { variant: 'outline', size: 'sm', className: 'none' }, noAncestorOverride: true },
  cssOutputSha256: createHash('sha256').update(built.css).digest('hex'),
  spacingToken: spacingRule,
  mergedClasses,
  matchingCompiledDeclarations: classDeclarations,
  scenarios,
};
writeFileSync(resolve(evidenceDir, 'collection-header-compiled-sizing.json'), JSON.stringify(sizingReport, null, 2) + '\n');

const tokenSource = read(sources.tokens);
function tokenBlock(selector) {
  const start = tokenSource.indexOf(`${selector} {`);
  const end = tokenSource.indexOf('\n}', start);
  return new Map([...tokenSource.slice(start, end).matchAll(/(--[\w-]+):\s*([^;]+);/g)].map((m) => [m[1], m[2].trim()]));
}
function tokenValue(values, token) {
  const value = values.get(token);
  if (!value) throw new Error(`Missing ${token}`);
  const alias = /^var\((--[\w-]+)\)$/.exec(value);
  return alias ? tokenValue(values, alias[1]) : value;
}
const clamp = (v) => Math.min(1, Math.max(0, v));
function toRgb(value) {
  const m = /^oklch\(([\d.]+) ([\d.]+) ([\d.]+)\)$/.exec(value);
  if (!m) throw new Error(`Not opaque OKLCH: ${value}`);
  const lightness = Number(m[1]), chroma = Number(m[2]), hue = Number(m[3]) * Math.PI / 180;
  const a = chroma * Math.cos(hue), b = chroma * Math.sin(hue);
  const l = (lightness + .3963377774 * a + .2158037573 * b) ** 3;
  const mid = (lightness - .1055613458 * a - .0638541728 * b) ** 3;
  const s = (lightness - .0894841775 * a - 1.291485548 * b) ** 3;
  const channel = (v) => { v = clamp(v); return Math.round(255 * clamp(v <= .0031308 ? 12.92 * v : 1.055 * v ** (1 / 2.4) - .055)); };
  return [channel(4.0767416621*l - 3.3077115913*mid + .2309699292*s), channel(-1.2684380046*l + 2.6097574011*mid - .3413193965*s), channel(-.0041960863*l - .7034186147*mid + 1.707614701*s)];
}
function luminance(rgb) {
  return rgb.reduce((sum, channel, index) => {
    const encoded = channel / 255;
    const linear = encoded <= .04045 ? encoded / 12.92 : ((encoded + .055) / 1.055) ** 2.4;
    return sum + [.2126, .7152, .0722][index] * linear;
  }, 0);
}
const brandClasses = brand.split(/\s+/);
const contrast = [];
for (const theme of ['light', 'dark']) {
  const values = tokenBlock(theme === 'light' ? ':root' : '.dark');
  const fgToken = '--' + brandClasses.find((c) => c.startsWith('text-')).slice(5);
  const foreground = tokenValue(values, fgToken), foregroundRgb = toRgb(foreground);
  for (const [state, prefix] of [['default', ''], ['hover', 'hover:'], ['active', 'active:'], ['expanded', 'aria-expanded:']]) {
    const utility = brandClasses.find((c) => c.startsWith(prefix + 'bg-'));
    const bgToken = '--' + utility.slice(prefix.length + 3);
    const background = tokenValue(values, bgToken), backgroundRgb = toRgb(background);
    const lum = [luminance(foregroundRgb), luminance(backgroundRgb)].sort((a, b) => a - b);
    const ratio = (lum[1] + .05) / (lum[0] + .05);
    contrast.push({ theme, state, backgroundUtility: utility, fgToken, bgToken, foreground, background, foregroundRgb, backgroundRgb, ratio, normalTextAA: ratio >= 4.5 });
  }
}
const contrastReport = {
  kind: 'WCAG 2.x relative luminance calculation from current opaque source tokens; not screenshot sampling',
  tokensSha256: sha256(sources.tokens), buttonSha256: sha256(sources.button),
  method: 'OKLCH to clamped 8-bit sRGB, then WCAG relative-luminance ratio; actual brand variant utilities extracted from source',
  threshold: 4.5,
  results: contrast,
  minimum: contrast.reduce((a, b) => a.ratio <= b.ratio ? a : b),
};
writeFileSync(resolve(evidenceDir, 'brand-contrast-final.json'), JSON.stringify(contrastReport, null, 2) + '\n');
console.log(JSON.stringify({ sizing: scenarios.map(({ name, resolved }) => ({ name, ...resolved })), contrast: contrast.map(({theme, state, ratio}) => ({theme, state, ratio: Number(ratio.toFixed(3))})), minimumRatio: Number(contrastReport.minimum.ratio.toFixed(3)) }, null, 2));
