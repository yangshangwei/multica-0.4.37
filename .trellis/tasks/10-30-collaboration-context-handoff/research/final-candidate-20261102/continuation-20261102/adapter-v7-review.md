# Independent final adapter v7 review

Verdict: **changes-required**. `blocking_findings_open: 1`. Reviewer: `/root/adapter_v5_review` (`trellis-check`). This verdict applies only to Go `eafed32e38ea69de0f73272df5c9cc817ec0741726b54fc19d27466742f57076` and pnpm `7a416eb60f1bdbe6bdf7421d2f61346d74d0e2575ae8a5236e450627db63ff48`. It grants no launcher or integrated-v6 clearance.

## Findings (fixed)

No reviewer adapter edits. The original AV5-1 canonical-package/-C/default-selector and AV5-2 nonblank explicit-context checks are corrected on their guarded paths. Supplied binding30/inherited14/pnpm5 and reused independent20 pass. The three specified short-flag/test-alias scanner methods pass within binding30. AV7-1 still bypasses those guards through a false compile exemption.

## Findings (not fixed)

### AV7-1 — high, blocking, open

File: `.omx/private/collaboration-resume-20261101/adapter-v7-knowledge-advance-20261102/check-bin/go`, line 41; related scanner/admission lines 46, 52–65, 85, 92 and 98.

The scanner omits common nonboolean build flags and does not follow Go’s double-dash/`--args` normalization. Its early compile forwarding can therefore execute ordinary tests without required context, canonical package binding or test pool-variable removal.

| Example argv after `go` | Real Go semantics | Exact v7 fake observation |
| --- | --- | --- |
| `test -installsuffix -c example/first` | Suffix value is `-c`; ordinary tests | Missing-context fallback exec |
| `test -gccgoflags -c example/second` | Compiler flags value is `-c`; ordinary second-package tests | Other package forwarded on owner clone |
| `test --run -c example/first` | Regex value is `-c`; ordinary tests | Missing-context fallback exec |
| `test -c=true --c=false example/first` | Last real compile flag is false | True exemption retained; fallback exec |
| `test example/first --args -c` | `-c` belongs to binary arguments | Tail interpreted as compile exemption |
| `test --c=true example/first` | Genuine compile-only alias | Empty context refused incorrectly |

The two new independent suites run six methods each: **10 build-value + 9 alias failing assertions**, across **nine failing methods**, **zero errors**. All execution was intercepted; no real Go test or DB write was attempted.

Cached Go1.27.0 confirms the grammar without tests: network-denied `go list -installsuffix -c .`, `go list -gccgoflags -c .` and double-dash installsuffix all return `example/first` with exit 0. Cached `cmd/go/internal/work/build.go` registers these value flags for `test`; `cmdflag/flag.go` normalizes two dashes and consumes nonboolean values; `test/testflag.go` accepts `--args` and applies real `c` flags in order. All source/raw hashes and actual commands are in the JSON.

Correction requested: classify all supported common build and forwarded test flag types from the actual Go contract, normalize supported spellings while retaining original argv, and honor `-args`/`--args`/bare `--` plus final real Boolean `c` semantics. Unknown or ambiguous options must fail closed. Independently recheck a new version; this reviewer does not edit adapters or launcher/status state.

## Verification

- Lint: **not run**; no project lint pass claimed. In-memory Python syntax/AST checks: **9 pass**.
- TypeCheck: **not applicable** to this private Python review; no product TS/Go edits or project typecheck claim.
- Tests: **fail for clearance**. Binding **30 pass** (scanner3 is a subset), inherited **14 pass**, pnpm **5 pass**, reused independent **20 pass**. New build suite: **6 methods / 4 failing / 10 assertions**; alias suite: **6 methods / 5 failing / 9 assertions**; both exit 1, zero errors.
- Normal tested paths keep fresh per-package clones, original check flags, named inventory and no-test accounting. Ordinary tests remove inherited API pool limits; non-test calls retain startup limits.
- pnpm stays byte-identical and preserves selection/workers/retries/timeouts/reporters and repository Agent CLI guard wiring/refusal. These are fake subprocess checks, not browser execution.
- Nine actual cached Go metadata commands (version/help/list only) pass under network denial; `-C`, `-C=` and default/relative private module selection agree with the corrected binding seam.

The reused V5 fixture initially reported **19 pass / 1 error** because it assumed exec was the first call; v7 correctly adds a list call. The private copy now selects the exec record while retaining the owner-clone assertion, and passes20. Initial source/raw/JSON remain under `reused-independent-initial/`; no adapter assertion or source was weakened.

## Source and raw evidence

| Artifact | SHA-256 |
| --- | --- |
| v7 go | `eafed32e38ea69de0f73272df5c9cc817ec0741726b54fc19d27466742f57076` |
| v7 pnpm | `7a416eb60f1bdbe6bdf7421d2f61346d74d0e2575ae8a5236e450627db63ff48` |
| binding30 raw | `c0efb3cecb9b6b3eb829bf6f1b45d468f788c35036eb9baf033fdfc8e614e2c9` |
| inherited14 raw | `adb4f75430bec8fe732eaff9d7a58940bbb77a4f16fbd4f46d82331e2407d971` |
| pnpm5 raw | `4c0a8d8dfd4606ae9cfe3d17fcbd385501b3d4522cda522cf9cd43fd74a56b76` |
| reused20 corrected raw | `1ac0d4ce37488c927a960507466cb2f977baa8c56981c7677e39c7e026d84197` |
| new build-value failures raw | `1c2ed5c9fbea16ace4272aded22751eaab80804c99f0ea03df6c273bccbf200e` |
| new alias failures raw | `0fc4cfd8785ea585ea735c93d8c4eec30ed664bc3676cca9f8b92c2331b86970` |
| cached Go parser source | `49f4b387f80e46a674e755c8c767170aacf98851746ee18e7f1cf1f9d1902327` |

All 19 incoming/first-green/final snapshot checks pass, live v7 matches final-source, and the 89-line v6-to-v7 patch hash matches its handoff. Historical red-v5, scanner-red-v5/first-v7 and final green records were read and raw/source digests captured. Some command records have no prior combined raw-log digest; the JSON explicitly distinguishes computed digests from available source/count comparisons.

Private output: `.omx/private/collaboration-resume-20261101/adapter-v7-independent-review-20261102/`. The original v5 reports retain hashes `17a74f72d74c86b748b2fb0ef09990769ee7b3bd726bd66063abc9197e38239b` (MD) and `64c4f0de0167ad42549bb69ed6aa940ed18ab09733a8abfb68b52dd4bea922aa` (JSON).

## Limits and handoff

No real TestMain/Go tests, DB/migrations, services, browsers, account/agent CLI, Provider, downloads, global environment changes, shared resources, product/adapter edits, launcher/status changes, commits or children occurred. Cached Go ran only version/help/list with private module fixtures, HOME/caches and network denied. Redis16422/renderer19845 and shared final4 builds were untouched.

The broader inherited outer-mode splitter/module/overlay compatibility is not certified. This review grants no shared complete-check or integrated-v6 acceptance. The coordinator owns the new correction and its next independent scope. Existing v5/v7 failures must remain retained.
