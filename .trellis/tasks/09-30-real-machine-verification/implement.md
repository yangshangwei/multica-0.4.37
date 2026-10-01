# Local verification continuation — 2026-09-30

The user requested continued work on the parent close-behavior task, then
confirmed that no target machines are available and local verification should
be completed first. Native Windows/Linux acceptance remains pending.

1. Run the existing Desktop suite, typecheck, lint and locale parity checks.
2. Reproduce and fix the persisted `minimize` settings label when the current
   session has no tray. Keep the translated label map complete; filter only
   selectable options. Cover the regression with the real Select primitive.
3. Build Windows x64 and Linux x64 local candidates through the existing
   packaging wrapper, with publication disabled. Rebuild after the fix.
4. Verify packaged main/preload/renderer, tray resource, executable architecture,
   version provenance, update metadata, checksums and isolated HTTP downloads.
5. Correct the manual checklist to match the current close-behavior spec.
   Save actual local evidence and explicitly pending native scenarios.

No production update feed or GitHub release is part of this continuation.
The candidate Desktop version is 0.5.3; without a release tag, the bundled CLI
retains its actual git-derived development version. Record both accurately.
