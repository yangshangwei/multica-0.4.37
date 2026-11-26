# Research: Repaired native timing01 adapter acceptance

- Query: Review the concrete refreshed request and approve its safe subset independently of unfinished full CLI/packaging work.
- Scope: internal; static adapter/evidence review only.
- Date: 2026-11-25

## Findings

**ACCEPT timing01 only**, for request SHA256 `7abe85ffe2a1fe6f47a5f66fc8b0e2d109448490da7ec196eddae7a2217d3ab0`. The exact command, files, tools, relay/build and source-manifest bindings are recorded in `native-adapter-review-timing01.json`; all declared hashes matched at completion. There are no remaining blockers for this bounded diagnostic invocation.

The request itself excludes full CLI API testing and packaged Main preparation/run. Those subsets are **not reviewed or cleared** by this acceptance; their absence does not block timing01. A successful synthetic409 transaction proves timing/transport behavior only, not the original ten-operation business journey.

### Previous blockers closed

- N1: the4079-byte concrete request now exists with exact command and digests.
- N2: `cleanup.py:16–38` attempts each resource separately; errors remain explicit. `timing_probe.py:45–54` retains primary failure and writes terminal state through finally. `cleanup.py:39–49` boundedly reaps the owned guard child; forced fallback preserves failure.
- N3: `cli_guard.py:104–106` persists exact create intents before mutation. `cleanup.py:10–13,21–36` reconciles intent names, requires owner/image/known-ID agreement, removes exact owned IDs/names and verifies absence. A failed cleanup cannot be counted as pass.

The Docker endpoint is pinned to the local Unix socket and a newly created empty task config replaces the personal Docker configuration. The probe passes only synthetic credentials; no product API or personal account is contacted. Its exact old-image command allowlist retains network-none/read-only/linux-amd64 and, for the executing relay, cap-drop ALL/no-new-privileges/bounded tmpfs/readonly relay volume. No privileged/unconfined flags are accepted. The staging container is never started.

## Files found / verification

- Native `adapter-review-request.json`, `timing_probe.py`, `cli_guard.py`, `cleanup.py` — concrete request and repaired executable paths.
- Native `guard_test.py`, `cleanup_test.py` and their retained logs — four allowlist controls and two cleanup controls report OK; hashes verified, tests not rerun.
- Bound relay source/build receipt/executable — source hashes agree with frozen source, compiler hash matches retained supported Go identity, executable is Linux/amd64 ELF mode0755.
- Native source manifest and five declared check logs — hashes verified; no whole149209-entry package/dependency audit is claimed for this relay-only request.
- `.trellis/spec/web/frontend/e2e-run-environment.md` and parent review — preserve source identity, exact ownership and separate packaged-client acceptance.

## Caveats / Not Found

- No adapter, Docker operation, service, test or product edit was executed by this reviewer. Only research review files were written.
- Exact native state/home/tmp directories inspected are nonsymlinks mode0700; timing01 output was absent at review completion.
- Parent may copy the exact accepted JSON to `native/adapter-review.json`. Acceptance applies to these bytes and this command; later changes must not inherit it silently.
- No external references were needed. Earlier rejected reviews remain intact as historical evidence.
