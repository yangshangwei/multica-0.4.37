# Execution Record

1. Created `/Volumes/artisan/code/2026/multica-upstream-b1` from base `7845de31e6bc7d021bba6005bccdab44fc9958e6`; verified upstream SHAs and preserved the original checkout.
2. Reproduced and backported the three approved changes in independent commits, with focused regression coverage and current English/Chinese guidance.
3. Ran static lint/typecheck and UI export checks successfully. Focused cache tests passed 113/113; guarded CLI tests passed.
4. Ran all TypeScript package tests. Core 2,057, docs 62, views 5,446 (single worker), web 261 and desktop 718 passed. The aggregate parallel run had one order-sensitive failure in the unrelated issue-limit dialog Escape test; the focused test passed on base and feature.
5. Ran `scripts/test-go.sh --race` and guarded `go vet -p 2 ./...` against a task-owned migrated database; both passed.
6. Built and started production Web/API from `67d97e612`. The B1 compiled CLI Playwright suite passed 3/3 against both development and production API/Web.
7. `make check` stopped at the parallel views test failure and did not reach its remaining phases. Those Go, build, and B1 browser checks were run separately. The repository-wide Playwright suite was not run.
8. Recorded upstream SHAs, verification, the parallel-run failure and the deliberate exclusions in `docs/upstream-backports.md`.
9. Rechecked `main` at the original base and fast-forwarded it to `9ed9c8e018d332dd2493c95bba07ea1fbaf1e3bf`, preserving all three feature commits. Confirmed the original `.impeccable/` files were unchanged. No remote push, release tag or deployment was performed.
