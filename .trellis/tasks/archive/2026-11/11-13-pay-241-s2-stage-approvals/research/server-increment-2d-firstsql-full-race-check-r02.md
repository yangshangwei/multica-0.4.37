# First-SQL full race diagnostic — r02/r03/r04 evidence handoff

The diagnostic remains **setup blocked**. The original **NOT_ACCEPTABLE_R03**, V01/V02, P2=false and incomplete S2/A01–A12 remain unchanged. None of these attempts supports NOT_REPRODUCED or closes the original SQL failure.

The installed Go1.27 source establishes a current environment mismatch: `testing.common.TempDir` reads **GOTMPDIR** directly. r04 kept HOME/TMPDIR trusted but placed GOTMPDIR on the external group-writable volume. The artifact fixture uses `t.TempDir`, so this profile still selects rejected ancestry. This is a static current-profile finding; the earliest actual runtime rejection and generated path metadata were not captured by these runs.

## Scope and immutable inputs

Reviewer: `/root/s2_final_code_check_r05`; active task: `/Volumes/artisan/code/2026/multica-0.4.37/.trellis/tasks/11-13-pay-241-s2-stage-approvals`. One exact37-top `./internal/service` race selection, explicit `/opt/homebrew/bin/go`, original `run.mjs`, count1/JSON/race, unchanged default10m and assertions. No extra normal, pure, Go static, migration, account or production action was performed in this slice. R04 issued no Git command; the earlier r02/r03 environment audits retain their recorded read-only Git-root probes. The later SDK/harness continuation executed no Go/test/SQL.

Diagnostic manifest `c1f3ac5be782ab0292a88292cd2872cca49b06a3a4d87a4c8041a3141e77c250` contains4,966 regular files over the4,965-file base `81cacce71bc781e8c6ca2a6f27fa379ba9befe3671b7187fb81baf71b818720a`. Only two existing test files and one added passive driver-tracer file differ. Whole source, full current parent and configuration inventories matched before/after. No product or frozen-source edits were made by this reviewer.

Original reports remain byte-identical:

- `server-increment-2d-final-code-check-r05.md`: `db8191b67bbc4838a2b777fa55628845d470f53de4a952a7c2a0ac7bd1d587ab`.
- `server-increment-2d-final-code-check-r05.json`: `e847823c715d4b58ae73a0634671da2ff8d4c3bcab56f3ba8f3579523e24eaf7`.

## Three separate preserved attempts

| Run | Config SHA256 | Actual named result | Package elapsed | Launcher/compiler preparation | Classification |
| --- | --- | --- | ---: | ---: | --- |
| r02 | `ea5256f7d2f428c4d4921a300acf9430f3ef23e459f1b675245d742779b745d0` | 0 tests; all37 NOT_RUN | 0 / FailedBuild | 3.646s | Link ENOSPC |
| r03 | `443d468f1f42c04073f14bd349dc6e977bc501b5bbb91eab5f46344c9b218c84` | 40PASS /88FAIL;37 tops,95 leaves | 100.505s | 417.657s | Artifact fixture setup blocked |
| r04 | `0369b70daabdbfab362df18584102830fc3c660257de048f29ee4321943ff5d3` | 40PASS /88FAIL;37 tops,95 leaves | 37.187s | 6.308s | Artifact fixture setup blocked |

Preparation measures helper-start to the Go package-start event; it includes launcher/identity and compilation preparation, not only compiler CPU time. r02 emitted a FailedBuild package terminal and never ran test bodies. Its linker reported `mapping output file failed: no space left on device`. The initial DB was proven unused and was explicitly reused only for r03. r03 and r04 actually ran fixtures; preserve both databases and use a new fresh DB for a later complete run.

r03/r04 each have128 unique named terminal records:13PASS/24FAIL tops and33PASS/62FAIL leaves. All62 failing leaves report `artifact adapter unavailable`;26 additional failure records are parent aggregation. There are zero skips, missing tops, extra tops, unfinished names, duplicate terminals or malformed JSON. These are not88 independent product defects.

Both actor/result target branches failed at `collaboration_stage_completion_tx_test.go:296`, during fixture construction. They did not reach the corruption seam, completion return or hits witness. Each run has **zero** first-SQL driver/return diagnostic records. `hits` is unobserved here, not a measured0. These runs neither reproduce SQLSTATE25P02 nor establish NOT_REPRODUCED.

Raw hashes:

- r02: `fe3f9151954fbb129ae5cc4ac13270ee7fcd5e9b9e3e966bcca929ad489ab295`.
- r03: `8b87e9ba0f940b5ec223c927649632a0132746087272f53f17865e62c36f041d`.
- r04: `eb3874f96c46ced7a0e9b76ee9dd1bebf38dabfd3117c4804ee15351bf771039`.

## Verified static environment and call paths

1. Preserved Python helper constructs PATH/GOCACHE/GOMODCACHE/GOPATH/LANG and starts Node with cwd=`SOURCE/server`. Original `run.mjs:8–14` copies that environment, removes ambient database/runtime/account fields, merges the exact config and passes env to explicit Go. GOTMPDIR from r04 is preserved.
2. `cmd/go/main.go:408` handles `-C` by changing the Go process cwd. `:304` captures OrigEnv; `toolchain/select.go:76–85` removes only the internal toolchain counter. `cfg/cfg.go:328–333` explicitly distinguishes OrigEnv for user binaries from CmdEnv for tools.
3. `work/action.go:298` selects the compiler WorkDir from GOTMPDIR. The actual test execution path is `test/test.go:1672–1680`: cwd is the package directory, env is OrigEnv plus PATH/PWD, with only optional coverage metadata. This command uses no coverage flag. Neither this branch nor AppendPATH/AppendPWD rewrites TMPDIR or GOTMPDIR. The binary's cwd is the service package, not compiler WorkDir.
4. `testing/testing.go:1573–1575,1613` documents and implements `os.MkdirTemp(os.Getenv("GOTMPDIR"), pattern)`. Only an empty GOTMPDIR falls through `os/tempfile.go:87–89` to `os.TempDir`, which on Unix reads TMPDIR (`os/file_unix.go:390–399`). r03 had empty GOTMPDIR and external TMPDIR; r04 has nonempty external GOTMPDIR and trusted TMPDIR. Both select the external volume for T.TempDir.
5. The actual fixture path is target→`newStageCompletionFixture:50`→`newStageExecutionFixture:182–185` default `stageArtifactReadFixture` callback→`newStageRuntimeFixture:64–67`→`stageArtifactReadFixture:45–47` t.TempDir and stage/output siblings→`artifact.New:50`→`openRoots:303,307`→`openSecureRoot:92`. It does not call `newStageReviewFixture`; it constructs a `stageReviewFixture` and invokes the callback.
6. Frozen service source has no TestMain/init or direct TMPDIR/GOTMPDIR/environment/cwd rewrite matching the recorded scan. `testutil/db_test.go` owns a TestMain for its own test binary; it is not the service suite's TestMain. Relevant managed fixtures set auth/deployment configuration, not these temporary-directory keys.

The current `/Volumes/artisan` metadata is uid501, mode0o775. That matches the existing predicate's rejection of a non-root group/other-writable ancestor. It was not chmodded. Static source excerpts, hashes, exact lines, scan scope and limitations are in `/Volumes/artisan/.multica-acceptance/pay241-20261115-continuation/resume-r05/capacity-r02/firstsql-full-race-r04/sdk-environment-proof.json` (SHA `7543c2faf215d943bb2c4fb07ad1dde08a5961bb8e0261f948557fdebfb60b49`).

## Evidence boundaries and unresolved findings

- **ENV01 remains for ROOT:** bind a new profile controlling the actual Go1.27 T.TempDir base. First obtain an exact single-case runtime env/base/stage/output/earliest-refusal witness on a fresh DB; only then authorize another complete37 run. No fixture, validator, assertions, deadlines or old configs should change to obtain a green result.
- **V01 remains open:** original first race had126PASS/2FAIL in this segment, with result+parent failing, returned25P02 and hits0 before corruption. The first underlying SQL error was not captured. Current profile refusals cannot explain or close that original failure.
- **V02 remains open with the server owner:** original late fact conflict lacks initial Prepare state/callback/terminal/time evidence. This first-SQL slice does not replace that diagnostic.
- Actual runtime env/generated path suffix/UID/modes and earliest New/openRoot refusal were not logged here. Go1.27 TempDir creates a numbered child with0777 before process umask; trusted profile ancestors alone do not prove every generated ancestor's mode. The separately assigned owner metadata source was neither read nor modified by this reviewer.
- QueryTracer has no dedicated Batch/CopyFrom/Connect/Prepare coverage. Hashing, stack capture, synchronization and output add timing overhead. Zero diagnostic lines before the target do not prove absence of every possible driver error.
- This evidence uses Go1.27/macOS arm64. Go1.26/Linux CI, actual Web/Desktop0/4, A01–A12 and fullS2 remain unproved. P2 remainsfalse and no activation is recommended.

## Preservation and verification

Authorized READONLY before/after DB records passed:835 ledger rows, exact864 last/hash, barehex64, all11 stage fact counts0 and otherconnections0. Current r04 Data was about11.07GB before and11.01GB after; external capacity remained about1.076TB. The original fixture explicitly sets creation_enabled=true then cleans it tofalse (`collaboration_test.go:54–55`); captured gate rows are retained, not described as identical. No extra SQL was executed after the read-only SDK dispatch.

Session18051 returnedexit1 and no owned matching process remains. All logs, helpers, full source/config/parent audits, before/after DB snapshots and exit evidence for r04 are in `/Volumes/artisan/.multica-acceptance/pay241-20261115-continuation/resume-r05/capacity-r02/firstsql-full-race-r04`. r02/r03 sources/configs/logs/databases and original reports were preserved. New SDK and outcome proofs were written only there; the two new task reports are the only repository writes.

- Lint: not rerun in this diagnostic scope; the original full review's scoped gofmt/vet PASS is retained.
- TypeCheck: not rerun in this diagnostic scope; the original full build/retained frontend checks remain referenced. r02 test link failed; r03/r04 compiled and executed.
- Tests: **FAIL** — r02 all37 NOT_RUN; r03/r04 each complete40PASS/88FAIL during setup. No passing original37 diagnostic exists.
- Evidence/source/config/parent audits: PASS. All references were rehashed; original report bytes are unchanged.

No product simplification or code fix was made. The private report helper was corrected to read names from the original command's metadata objects; its initial metadata-only error and pre-fix bytes were preserved. ROOT acknowledged the profile mismatch and owns the corresponding spec/state correction. This slice is complete as an evidence handoff, with original acceptance blockers still open.
