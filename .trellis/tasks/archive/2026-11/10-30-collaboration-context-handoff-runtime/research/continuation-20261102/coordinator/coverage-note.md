# Default complete-check coverage and remaining directed gates

Read-only audit of final4 `64a9994ec549e8dff73ab14456dc9a9f4b63f7c7`, product source `9f72fc21…d80ee`. Machine inventory and file hashes: `coverage-note.json`. No tests, database queries/writes, service startup or Provider invocation occurred in this audit.

## Default execution

`scripts/check.sh` runs static checks, every non-mobile TS package, script regressions, isolated normal/race Go packages and vet, a fresh task API, production Web and Playwright. Playwright discovers `e2e/` with one Chromium worker and zero retries; 48 matching collaboration/related spec files exist. This is an inventory, not an assertion that all 48 execute.

The default API forces legacy auth, device/managed/knowledge/platform-admin switches off. The selected pnpm wrapper adds allocated API/renderer URLs and JSON output; it does not enable password or Host executable gates. With standard `E2E_PASSWORD_AUTH` unset/0, password specs skip. The actual run must retain sanitized flags, `/api/config`, API/Web identity and the per-test pass/fail/skip inventory: setting a fixture flag against an incompatible API does not supply coverage.

## Gate mapping

| Scope | Current tests | Default complete-check coverage | Required remaining evidence |
| --- | --- | --- | --- |
| First-round B2 reconsent wording | core/views resource and execution tests; control-journey Web/Electron | Unit coverage executes. Broader password/managed control journeys skip. | Preserve server `can_consent` and Switch-only bounds. Current units plus already verified narrow behavior may close original B2; do not infer all owner/runtime/installed combinations. |
| First-round C3 receiving requester/note/status | `collaboration-handoff-chain-web.spec.ts`, `collaboration-handoff-chain-desktop.spec.ts`, `collaboration-offline-handoff.spec.ts` | Password-gated; default skips. | Same-candidate password production Web and actual Electron: 1 Web directed chain, 1 native information-return, 2 offline platform cases; zero skips/retries. Exact current code/config/renderer equivalent evidence may be reused, otherwise run these 4 cases. |
| Runtime AC9, AC4/AC6/AC7 UI wiring | `collaboration-runtime-handoff-web.spec.ts`, `collaboration-runtime-handoff-desktop.spec.ts` | Password-gated; default skips. | 2 Web cases (ENV-1 startup/live memory distinction and retained source/comment recall/removal) plus 1 actual Electron deep-link/unknown-tail case, bound to the candidate. These are 3 cases, not model-comprehension proof. |
| Runtime AC1–AC8 implementation | Go protocol/service/handler/daemon and core/schema units | Ordinary and race packages execute; tagged suites remain separate. | Actual named pass/skip inventories, plus the independently reviewed same-product RKI6 62 normal/62 race evidence. Neither replaces real-model behavior. |
| Knowledge on/off parent gate | `collaboration-knowledge-deployed.spec.ts` | Skips if `COLLABORATION_KNOWLEDGE_EXPECTED` is unset. Human candidate UI tests separately execute under knowledge-off. | Explicit `false` and `true`, one case per isolated labeled deployment, controlled Host binaries and matching build/config. Production default remains false; no paid Provider is needed. |
| Host/squad/Switch/memory execution | Host specs and `collaboration-memory-runtime.spec.ts` | Skip when no explicit Host binaries are configured; partial configuration must fail setup. | Distinct controlled executable/platform evidence, not an installed commercial Provider account. Broader parent scope cannot be inferred from default skips. |
| Installed/native Windows/offline upgrades/multiple Provider accounts | installed Main/client and release-specific gates | Explicit executable/image/account/environment gates; generic default run cannot close them. | Remain separate parent/release scope. Do not expand first-round B2/C3 or runtime AC9 to these combinations. |
| Real-model experiments | Tagged explicit experiment entries | Default full check invokes no real Provider. | Original frozen series: E3 after trial2 then E4–E6, cap14. Runtime: registered RK1 E0a/H1/H2/H2b/H3a/H3b/H5/H6, cap25. Separate ledgers, exclusive window, first-incomplete stopping rule. |

C3 assertions cover the receiving form's original requester and note, selected status/final handler, offline transfer surviving archive/fresh login until removal, and no AI dispatch. Runtime assertions add provenance/long-comment recall, private-read denial after removal, empty startup memory preserved independently of live latest memory, and honest unknown recording tails. A generic Web transfer note assertion is useful coverage but does not replace the directed C3 native request/return case.

## sqlc evidence reuse

Current inputs and outputs are byte-equivalent to the successful check at `45efa093c1866c6ec988966769bd8f761daf7b50`:

- Evidence: `.trellis/tasks/11-01-collaboration-knowledge-audit-fixes/research/integration-verification/sqlc.json`, committed in `1278d1266c…`.
- Recorded command: `go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate`; exit0, drift `[]`, empty output-log SHA `e3b0c442…b855`.
- Compared `server/sqlc.yaml`, all `server/migrations`, `server/pkg/db/queries`, `server/pkg/db/generated`: no changed paths and no dirty current paths.
- 1,754 tracked related files; 125 files have the sqlc-generated header. Group SHA256: `0cda25902a90218d82cf36199f34e3d1fa2fc1f35af2ae960548af553b2a8422`.
- Configuration is local PostgreSQL schema/queries with Go pgx/v5 output; no remote parser, cloud or plugin download stanza is configured.

Reuse this exact successful generation result; a different Git HEAD alone does not require another generation. The much older first-round 118-file check at `4c833e737` is not the right current proof: later queries/schema/config/output changed. If any relevant input/output changes, revalidate generation in a task-owned scratch directory outside Git, preserve handwritten companions and compare only actual generated headers. This audit did not run generation.

## Migration proof and named inventory

Migration746 up/down SQL and `TestCollaborationOwnerReconsentMigrationSeparatesBoundaryEpoch` are unchanged from the original first-round source. The test covers legacy NULL boundary, nonwidening config edits, widening permission edits, explicit child advance, direct-assignment rejection, restored 704 writer after down, and up/reapply.

The runner change between `4c833e737` and `45efa093c` adds concurrent-index cleanup entries for748–763; it does not change the746 route. Runner/SQL inputs are unchanged from45efa to final4; one later knowledge migration test changed, which does not invalidate the unchanged sqlc input/output comparison. Historical populated745→746/down/up/reapply receipts therefore remain scope-specific746 evidence, not a new current763/offline upgrade claim.

For the complete run, confirm fresh Go/API migrations through763 and actual non-skipped results for the owner-reconsent test and the three prefix/direction lint tests. Migration helpers can skip on an absent/unreachable database, so package exit0 or a global aggregate is insufficient. If those names pass, do not repeat the unchanged746 rehearsal just to obtain another timestamp. If missing/skipped or affected by a later change, preserve the gap and run the minimal owned fresh-database verification.

## Concrete next order

1. Final selected launcher/Go-argument permission review, then the single-owner complete `make check` and actual named inventory.
2. Password production directed C3 four cases and runtime handoff three cases unless exact current equivalent passed evidence is available. Run sequentially after the default environment, with matching private registry, API/build/renderer and controlled Host/CLI paths.
3. Parent-required explicit knowledge false/true deployment gates, plus any independently tracked platform gate; keep those scopes out of first-round claims.
4. Freeze final model registration; schedule original14 and runtime25 in separate serial windows. Model results and absent/invalid attempts remain independently classified.

Commands here describe required evidence; they were not executed by this read-only audit. The latest historical complete run remains exit2 until the unique owner supplies a new completed result.
