# Research: Client plan and private infrastructure review

- Query: Independently review the client journey plan and fresh allocation/record scripts without executing them; identify focused actionable issues.
- Scope: internal, static inspection and read-only metadata inspection.
- Date: 2026-11-25
- Review snapshot: 2026-11-25T09:59:47Z
- Task/worktree: `11-25-main-post-merge-acceptance-fixes` in `/Users/artisan/.codex/worktrees/main-acceptance-fixes/multica-0.4.37`.

## Findings

**Client plan: ACCEPT.** It covers D06–D09 and D11, preserves full assertions and original deadlines, separates real macOS/Linux Main from renderer evidence, retains all ten old-CLI operations, and diagnoses overhead before optimization. Its fixture-only write boundary and escalation for a demonstrated product defect are appropriate. The inherited parent gates now explicitly cover frozen acceptance source, resource allocation, macOS protocol restoration and exact regression reconciliation. No additional planning document or design review cycle is needed before the parent releases this lane for bounded implementation.

**Infrastructure: suitable for preliminary diagnostics with the fixes below; not a final acceptance launcher.** The coordinator explicitly labels current-worktree builds preliminary. Their use of moving source is therefore not a new blocker. Final integrated acceptance still consumes a frozen candidate under the already accepted parent plan.

### I1 — Recheck the allocated PostgreSQL owner before CREATE DATABASE

- Location: `.omx/private/acceptance-fixes/allocate.py:70–74`; `sql()` at `:32–35`.
- Problem: the database branch reads a saved port and immediately executes `CREATE DATABASE`. Its subsequent `current_database()` check proves the name only, after the first write. It never verifies that the recorded container is still running with the recorded owner label, image, volume and loopback port. A stopped/restarted/replaced allocation can leave a stale endpoint; successful credentials and a matching newly created name are not independent ownership proof.
- Small fix: before the first SQL write, inspect the exact recorded container ID and compare its running state, owner label, image, mounted volume and current loopback port with `allocation.json`. Refuse a mismatch rather than silently rebinding. A deliberate port change needs a new allocation receipt and new derived environment. This can remain a short check in the existing allocator; no new orchestration framework is needed.
- Boundary: this is a missing pre-write guard, **not evidence that any present database is foreign**. Read-only inspection found all six saved environment URLs match the saved port and expected database names. No live Docker/SQL inspection was performed.

### I2 — Generated URLs are incompatible with the controlled-output admission guard

- Location: `allocate.py:78` always emits `?sslmode=disable`; `e2e/fixtures/collaboration-runtime-output-allocation.mjs:50–56` rejects any query string before admission. `collaboration-runtime-output.ts:350` actually calls that guard.
- Effect: passing one of these environment files unchanged to a controlled-output journey will fail before its business assertions. The six inspected files all contain the `sslmode` query key. This does **not** mean ordinary Go/database usage or every client journey is broken; it is a concrete integration incompatibility for the guarded output path.
- Small fix: create a separately recorded, query-free URL/environment for consumers of this guard, selecting TLS behavior through the appropriate supported connection configuration for that consumer. Pin the exact new URL digest in its allocation. Preserve prior environment records and keep the guard's rejection of query/fragment overrides. Do not weaken it to accommodate a generic allocator URL.

### I3 — Recorder drops the terminal receipt on launch failure or interruption

- Location: `.omx/private/acceptance-fixes/record.py:42–54`.
- Problem: `result.json` is written only after `subprocess.run()` returns normally and the second source snapshot succeeds. A missing executable/cwd, KeyboardInterrupt, or a failed final snapshot leaves logs/source-before without a terminal disposition. This repeats the evidence gap that made previous apparently passing substeps insufficient for acceptance. Ordinary nonzero child exits are handled correctly.
- Small fix: write an initial running receipt, then finalize through an exception-safe path with `completed`, `launch_failed` or `interrupted`, preserving observed child exit/signal when available and leaving it unknown otherwise. Keep an original command failure distinct from an audit failure. Record only safe error classification, and preserve available log hashes. Ensure owned child handling on interruption is explicit. An uncatchable termination can leave the initial running receipt for later truthful reconciliation; never synthesize exit0.
- Scope: fix before relying on this wrapper for long diagnostic/test runs. Existing completed build receipts remain usable as preliminary records; they do not need to be discarded.

### Accepted constraints and practical handoff

- The allocator uses a fresh random owner, loopback-only PostgreSQL publishing, labeled container/volume, exclusive JSON publication, private credentials, per-label HOME/database and a fresh signing key. Its base environment omits API pool/publisher settings. Those choices align with the existing isolation rules.
- The state root is mode0700, allocation/env records mode0600 and inspected paths are not symlinks. Per-lane parent directories are mode0755 but remain below the mode0700 state root; that observation is not itself a secrecy defect.
- `record.py` preserves normal exit codes, exclusive per-run directories, before/after selected source hashes and stdout/stderr hashes. The recorded `web-build-01` and `desktop-build-01` JSON receipts report exit0 and no selected source-byte changes; this review did not rerun or independently validate those builds.
- Selected source hashes do not constitute a freeze: `snapshot():15–23` follows file contents and omits modes, physical dependency resolution, ignored build/tool files and root inputs such as Makefile/tsconfig/Turbo configuration. Keep this wrapper diagnostic-only as instructed. The final frozen acceptance path must bind the actual inputs and outputs; no need to turn this small diagnostic utility into that system now.
- The recorder does not install the agent-CLI guard or enforce database-package permits. For Go test commands, call the existing guarded entry point with the parent's exact package/database allocation. Do not infer those protections merely from running through `record.py`.
- CLI lifecycle cleanup needs the planned correction in the owned fixture: `e2e/fixtures/collaboration-installed-clients.ts:63–65` and `:132–139` currently swallow removal errors. Preserve primary failure and report cleanup failure separately, as the client design already promises. Keep one positive command and the three three-command refusal stages in `e2e/collaboration-installed-clients.spec.ts:72–98`, `:104`, `:112`, `:129`; all ten remain inside the original deadline at `:32`.
- Parent macOS LaunchServices recovery at `design.md:26` applies before package preparation as well as launch. The client child need not duplicate that complete contract.

## Files found

- `.trellis/tasks/11-25-acceptance-client-journeys/{prd,design,implement}.md` — accepted client scope, hypotheses, ownership and complete replay plan.
- `.trellis/tasks/11-25-main-post-merge-acceptance-fixes/{prd,design,implement}.md` — corrected D10/D11 identities, diagnostic/acceptance distinction and inherited resource gates.
- `.omx/private/acceptance-fixes/allocate.py` — campaign PostgreSQL allocation, database/environment creation and migrations.
- `.omx/private/acceptance-fixes/record.py` — command runner with selected source and log fingerprints.
- `.omx/private/acceptance-fixes/allocation.json`, six lane `env.json` files — inspected structure, file modes and nonsecret consistency predicates only; secrets omitted from output/report.
- `.omx/private/acceptance-fixes/runs/{web-build-01,desktop-build-01}/result.json` — inspected diagnostic result metadata only.
- `e2e/fixtures/collaboration-runtime-output-allocation.mjs`, `collaboration-runtime-output.ts` — actual query-free database admission contract and caller.
- `e2e/collaboration-installed-clients.spec.ts`, `e2e/fixtures/collaboration-installed-clients.ts` — ten-operation old-CLI scenario, original budget and current cleanup behavior.
- `scripts/test-go.sh:49–54` — existing guarded Go commands; stock wrapper shares a URL, so parent per-package allocation remains necessary for joint acceptance.

## Related specs

- `CLAUDE.md` — package/test ownership, agent-CLI protection and no claimed verification without execution.
- `.trellis/spec/server/database-test-isolation.md:20–40` — recorded ownership before migration/test side effects; fresh per-package databases.
- `.trellis/spec/server/verification-environment.md:16–41`, `:79–124` — process-specific environment, run admission and final identity audit.
- `.trellis/spec/web/frontend/e2e-run-environment.md:937–973` — query-free effective database destination, immutable allocation digest, live service/artifact identity.
- Original acceptance `report.md`, `defects.md`, `test-plan.md` under the read-only `/Volumes/artisan/code/2026/multica-0.4.37/.trellis/tasks/11-24-main-post-merge-e2e-acceptance/` — already reviewed failure/accounting and cleanup requirements; not executed or modified.

## Reviewed byte identities

| File | SHA-256 |
| --- | --- |
| `allocate.py` | `b0052567a8c4e38e2ae293ae5570ac6d5ae815269663106483211c8559619229` |
| `record.py` | `08f2d011a017e9f225c076dcd12dcf062d3a04e3fd3238a4276d880c607e1539` |
| Client `prd.md` | `4a06427a5a7ab5354385f9ff6ee8299d88a362ace9295e0e51d677de02e96d00` |
| Client `design.md` | `1465023035d4b7e44e1292e49777b2f684f8fa337408b637637d988d4d72fc36` |
| Client `implement.md` | `8d1898fe347d1766434f9f7d542e5f3da7b7e0dbdac033c2e5185d73185c40e9` |

## Caveats / Not Found

- Static review only: no allocation/record script execution, Docker/SQL calls, services, tests, product edits, Git commands or recursive agents. Only this research file was written.
- No fresh client runtime admission adapters were supplied as completed review inputs. Their runtime proof is still required when the lane produces them; this does not block the accepted fixture implementation plan.
- Final frozen acceptance machinery, live listener ownership and cleanup are not certified by this review. The parent's explicit preliminary-build classification is accepted.
- External references: none required or fetched; conclusions follow repository code and existing local contracts. This is not a review of the security lane's tool downloads, advisory choices or runner.

## Coordinator update before handoff

The coordinator moved newly generated execution HOME paths outside Git to `/private/tmp/acceptance-fixes-4b9d3494a2/<label>/home`, preserved existing environment records, and explicitly retains the preliminary classification of earlier Git-nested HOME/build/database runs. A targeted reread confirms the new external HOME construction in `allocate.py:75` and `record.py:38`. This is accepted; old records need no rewrite. Lanes must derive fresh external HOME/TMP environments for frozen claimable runs, as instructed.

The targeted reread also confirms that the three reported mechanisms remain: generated URLs still include the query string, database creation has no container inspection, and recorder finalization still follows the normal subprocess-return path. Updated script fingerprints are `allocate.py = f415a36e53790f8e0c8ed92f093f9ca19a08eeac319246729ea33fe326fb9ab0` and `record.py = 737474daeffe7181c4e93e5f4aee27764d94dcfb6b388a2cdf49dd597f6d163c`. These supplement, rather than replace, the original review-input identities above.
