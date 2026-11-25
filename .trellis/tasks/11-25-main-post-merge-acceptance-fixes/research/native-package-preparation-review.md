# Research: Native package preparation adapter

- Query: Review exact macOS/Linux package-preparation request and protocol recovery, independently of the full CLI and native launch adapters.
- Scope: internal, static source/evidence review.
- Date: 2026-11-25

## Findings

**ACCEPT preparation only; execution remains deferred by the parent until client protocol cleanup finishes.** Request SHA256: `35b990f6722fa661c6ff1a1a09f50ce4d4311e673080c444771de82b5c4b8944`. Exact commands and binding hashes are in `native-package-preparation-review.json`; all declared digests match. No additional planning/permission requirement is introduced.

`prepare_packages.py:59–75` requires the exact recorded protocol baseline before any build, handles preparation exceptions, attempts recovery and records a terminal result. Protocol mismatch prevents startup; failed restoration keeps the command failed. `protocol.py:19–42` refuses missing pre-existing registrations, unowned new paths or unowned default changes, unregisters only enumerated owned URLs, restores the recorded exact default URL and compares the complete final snapshot. The three retained protocol-plan controls report OK and their log hash matches.

The dependency copier accepts only runtime packages resolved inside the frozen source, checks copied/source bytes, reconstructs links within its private target and excludes development caches. Package assembly disables publishing/notarization and uses cached Electron. The ASAR checker validates all member hashes, block hashes, packed bounds/ranges, safe links and unpacked inventory; the preparation entrypoint additionally compares exact out/resources product bytes and checks prepared inputs remain unchanged.

This acceptance concerns building the bound source01 artifact. It is not proof that its preliminary frontend outputs constitute final integrated candidate acceptance. Full Main launch, native behavior and installation remain separate. Full CLI still has the independent CLI1 process-cleanup finding in `native-cli-review-cli01.json`; package approval does not waive it.

## Files found / related specs

- Native `package-review-request.json` — exact macOS/Linux preparation commands, seven script hashes, protocol evidence and source manifest.
- Native `{prepare_packages.py,installed_closure.py,verify_asar.py,freeze.py}` — private package assembly and artifact identity checks.
- Native `{protocol.py,protocol.swift,protocol_test.py}` — exact URL/default capture and narrowly owned recovery.
- Native protocol-before/after/concurrent-change records — parent-coordinated extra clients registration, with no baseline rewrite.
- `.trellis/spec/web/frontend/e2e-run-environment.md` — source/dependency/package identity and native protocol restoration constraints.

## Caveats / Not Found

- No package adapters, protocol mutations, builds, tests, services or Git commands were executed by reviewer. Research files only.
- This static review verified the manifest digest and its enforcement code, not all149209 dependency/output entries. The adapter must compare that full tree before preparing artifacts.
- The supplied native baseline is not silently substituted for an earlier campaign baseline; taskwide restoration remains parent-owned. Parent already authorized exact same-owned cleanup and decides when serialization permits preparation.
- No external reference was necessary. Parent may copy the exact accepted JSON into the native review location.
