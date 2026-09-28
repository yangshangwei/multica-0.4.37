# Xiao Afu seal avatar

The user approved the cinnabar circular seal with an ivory 孚 glyph shown in
`.omx/artifacts/afu-seal/logo-concept.png` and requested implementation.

## Acceptance criteria

- Default Xiao Afu avatars match the approved seal and remain clear at 24–40 px.
- Onboarding, runtime setup, agent lists and conversations use the same artwork.
- Existing built-in agents still using the unicorn placeholder receive the seal;
  custom, cleared and ordinary-agent avatars remain untouched.
- Web, desktop and mobile can load the artwork without platform font dependencies.
- Preserve unrelated working-tree edits; add no dependencies.
- Verify focused tests, lint, type checks and actual image rendering.
