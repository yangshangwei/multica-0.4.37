/* global __dirname */
// Run from the repository root: node apps/mobile/scripts/sync-avatar-icons.cjs
// Mobile cannot import DOM components. Snapshot Lucide nodes and presentation
// metadata for react-native-svg without adding a native dependency.
const fs = require('node:fs');
const path = require('node:path');
const root = path.resolve(__dirname, '../../..');
const lucide = require(path.join(root, 'packages/ui/node_modules/lucide-react'));
const source = fs.readFileSync(path.join(root, 'packages/ui/lib/avatar-icon.ts'), 'utf8');
const componentBlock = source.match(/AVATAR_ICON_COMPONENTS = \{([\s\S]*?)\} satisfies/)[1];
const nodes = Object.fromEntries([...componentBlock.matchAll(/\s*["']?([\w-]+)["']?: (\w+),/g)].map(([, name, component]) => [name, lucide[component].render({}).props.iconNode]));
const legacyBlock = source.match(/LEGACY_EMOJI_ICONS[^=]*= (\{[\s\S]*?\n\});/)[1];
const legacy = JSON.parse(legacyBlock.replace(/,\s*}/g, '}'));
const toneBlock = source.match(/export const AVATAR_ICON_TONE[^=]*= \{([\s\S]*?)\n\};/)[1];
const tones = Object.fromEntries([...toneBlock.matchAll(/["']?([\w-]+)["']?: TONES\.(\w+)/g)].map(([, name, tone]) => [name, tone]));
const tokens = fs.readFileSync(path.join(root, 'packages/ui/styles/tokens.css'), 'utf8');
function rgb(l, c, h) {
  const a = c * Math.cos(h * Math.PI / 180), b = c * Math.sin(h * Math.PI / 180);
  const x = (l + .3963377774 * a + .2158037573 * b) ** 3;
  const y = (l - .1055613458 * a - .0638541728 * b) ** 3;
  const z = (l - .0894841775 * a - 1.291485548 * b) ** 3;
  return '#' + [4.0767416621*x-3.3077115913*y+.2309699292*z, -1.2684380046*x+2.6097574011*y-.3413193965*z, -.0041960863*x-.7034186147*y+1.707614701*z].map(v => Math.round(255 * Math.min(1, Math.max(0, v <= .0031308 ? 12.92 * v : 1.055 * v ** (1/2.4) - .055))).toString(16).padStart(2,'0')).join('');
}
const colors = {};
for (const [, tone, l, c, h] of tokens.matchAll(/--skill-(\w+): oklch\(([\d.]+) ([\d.]+) ([\d.]+)\)/g)) {
  colors[tone] ??= {};
  colors[tone][colors[tone].light ? 'dark' : 'light'] = rgb(+l,+c,+h);
}
const destination = path.join(root, 'apps/mobile/lib/avatar-icons.generated.json');
fs.writeFileSync(destination, JSON.stringify({ nodes, legacy, tones, colors }, null, 2) + '\n');
fs.copyFileSync(path.join(root, 'packages/ui/node_modules/lucide-react/LICENSE'), path.join(root, 'apps/mobile/lib/avatar-icons.LICENSE'));
