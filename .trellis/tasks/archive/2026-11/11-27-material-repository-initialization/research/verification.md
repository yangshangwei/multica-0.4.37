# Independent initialization kernel verification — 2026-11-27

## Implemented and actually verified

- server/internal/materialrepo renders20 foundation files, validates the supported manifest strictly, inspects complete immutable snapshots, produces add-only deterministic candidates, checks exact publication changes and verifies required/preserved merged bytes.
- Existing README and unmanaged regular test data remain unchanged. Already-compatible content yields no candidate. Unknown managed rules conflict rather than overwrite.
- Root-length, projected tree count/byte capacity and ECMAScript schema-path review findings received regression tests and fixes.
- Final coordinator run: go test ./internal/materialrepo ./internal/material -race -count=1 passed (3.683s /2.026s).
- Final coordinator go vet ./internal/materialrepo passed; Node schema path test3/3 passed; git diff --check passed.
- Task context manifests validate (7 entries each). Independent design review's two P2 issues resolved.

## Real repository read-only preview

Read yangshangwei/test--multica at586e60376890c80a994b93c394d828cf27089f0c through GitHub API. Observed8 entries (directories included) and4 files. Current kernel produced19 additions, preserving README and the3 UAT files. Candidate files and inspection JSON are in the chat visualization directory material-initialization-preview. This is a local preview, not a remote initialization run; no new PR, branch, merge or product ready state was written by this task.

## Unfinished scope

The selected-runtime task remains a dependency. DB/operation lifecycle, maintenance upgrade behavior, independent human-review endpoints/UI, actual runtime publication/merge/readback and normal-publication ready gates are not implemented in this kernel. No full feature completion, deployment or end-to-end claim is made. Task remains in_progress.

The first kernel rejects executable/symlink/submodule entries anywhere and requires exact prior tree plus additions, so mixed code repos/concurrent unrelated target changes may need a future explicit preserved-opaque-entry design. These are declared compatibility limits, not silently skipped checks.

No full repository Go/frontend checks were run because this slice adds an isolated Go package and assets, without frontend/DB/wiring changes. No new dependency; no user database or model account touched.
