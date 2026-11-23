# Execution checklist

- [x] Freeze candidate, source inventory, tool versions and fresh checkout; install locked dependencies without adding dependencies.
- [x] Curate current Playwright inventory and E01-E17 mapping; classify auth/gates/native/Provider requirements.
- [x] Run bounded frontend lint/typecheck/unit/UI exports; run Mobile typecheck/lint/unit separately.
- [x] Allocate private DB template and per-package Go databases; run guarded named package tests/race and vet with environment isolation.
- [x] Start isolated production Web/API; verify actual DB, PID, source and rendered API/WS destinations before fixtures.
- [x] Execute baseline Playwright suite, preserve raw results and classify conditional skips.
- [ ] Prepare password/device/Host/knowledge/native lanes using current fixtures; execute reachable mandatory groups.
- [x] Execute existing iteration/triage performance gates on a fresh dedicated DB without race.
- [ ] Investigate genuine failures; preserve originals; repair narrow fixtures or confirmed product defects and run associated regressions.
- [ ] Run independent evidence review, final identity audit, owned-resource cleanup and report.

Validation entry points: pnpm exec playwright test --list; bounded turbo lint/typecheck/test --force; pnpm check:ui-exports; pnpm -C apps/mobile typecheck/lint/test; guarded Go test -json -count=1 -race per exact package; go vet; production Web Playwright --workers=1 --retries=0. `make check` alone reuses one Go DB across packages and defaults to legacy, so current database isolation spec requires coordinated per-package Go execution and separate conditional browser lanes. Do not claim one complete make check pass from separate commands.

If a mandatory native/Provider requirement is unavailable, record BLOCKED with inspected prerequisite and recovery action, continue independent reachable work, and retain incomplete acceptance.
