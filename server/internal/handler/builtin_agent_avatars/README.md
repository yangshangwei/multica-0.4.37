# Built-in agent artwork

`afu-seal-v1.svg` is the editable outline source for Xiao Afu's cinnabar seal.
The glyph is outlined: clients do not need a Chinese font to display it.
`afu-seal-v1.png` is a 512 × 512 transparent rendering of that SVG, embedded by
`builtin_agent_avatar.go` and supported by native React Native `Image` as well
as web/desktop browsers.

The public route is immutable and cached for one year. When changing the
artwork, use a new versioned filename and update the handler route, the default
URL in `mika_agent.go`, and `MIKA_AVATAR_PATH` in `packages/core/onboarding/mika.ts`.
Migrate only the previous built-in default; never overwrite custom avatars.
