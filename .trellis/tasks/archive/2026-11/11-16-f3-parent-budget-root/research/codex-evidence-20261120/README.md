# Durable F3 evidence

`manifest.json` maps every retained artifact to its original location, uncompressed byte length and SHA256. `.gz` files decompress to the original raw bytes; no failure result was rewritten. Private database configuration and credentials are excluded.

Start with `takeover-integrated-run-binding.json`:62 S1 rows,142 cited names,571 expected top-level names and91 canonical M1/F2 names all bind to normal/race raw logs for combined source `8126073f6` (tree `695a4a84fc50a9223f56cda07fbfb2cf89936452`). The original baseline remains in `b54-*` and `baseline-b54-*` artifacts. `takeover-integrated-source-manifest.json.gz` records7916 source hashes; `takeover-shared-fast-forward-proof.json` verifies the same bytes on the shared branch and zero changes to13200 unrelated dirty files.

Fresh final execution receipts: `takeover-integrated-db-summary.json`, `takeover-nodb-summary.json`, `takeover-integrated-frontend-summary.json`, `takeover-integrated-static-analysis.json`. Their raw logs, runner scripts and per-package summaries are adjacent. The database allocation record contains names and cluster ID only. Actual credentials remain in private scratch, not in this directory.

Earlier `final-r*`, `cx*`, `caller-*`, and Views r1/r2/r3 failures remain historical evidence. Views r4 passed608/7441 on the old feature source; the combined source subsequently passed619/7610. Core similarly passed366/5172 then374/5355. The full final Go run is selected F3/M1/F2 acceptance plus metrics, not a claim that every repository backend test ran. Test counts in summaries include named subtests.

`historical-*check6kd1_test.go.txt` are unfinished independent review source, retained for provenance and never claimed as executed final acceptance. Production telemetry has no trusted actual-cost producer; pure/injected actual and settlement tests do not establish real Provider billing, full coverage or production release of holds.
