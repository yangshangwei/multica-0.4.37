# Codex harness independent pre-real review

Date: 2026-11-05 Asia/Shanghai. Reviewer: codex_integration (author of B product integration, not the C harness reviewed here).
Scope: read-only review of current `rk1k` Codex/provider/registration/fixture/scenario files, tagged daemon driver, `compile-harness.py`, `run-fake.py`, and registration template. No code edits, tests, database queries, CLI/model calls or credential reads were performed by this review. Current normal fake execution belongs to C and must finish unchanged.

## Verdict

**Changes required before real execution.** The framework correctly routes through the production Codex backend and real fixture APIs and preserves the fixed 15 measured + 3 fake-seed schedule. Four evidence/admission gaps below need correction and bounded negative controls before authorizing real calls. A passing current fake run is not sufficient to resolve these source findings.

## Blocking findings

### H1 — Correlate the answer and terminal to the exact native turn

File: `server/cmd/server/rk1k_codex_provider_test.go`, `rk1kObserveCodex`.

The parser accepts `item/completed` agent messages using only matching `threadId`; the decoded `turnId` is never compared with the actual turn. A response for another turn in the same thread can supply the expected answer and be combined with the current turn's terminal to pass scoring. It also does not require exactly one successful matching thread response, accepts repeated/overwritten request IDs, ignores RPC errors and asynchronous error records, and ignores `turn.error` when status is `completed`. Missing successful turn-start response is only indirectly caught by an empty Turn value; request/response cardinality and response identity are not enforced.

Require exactly one successful thread/start or exact thread/resume request/response pair and one turn/start request/response pair, validate the requested/resumed/returned/native thread consistently, correlate every scored agentMessage with that exact thread+turn, and require one authoritative terminal with status completed and no error. Reject duplicate conflicting identities/responses/terminals and error-bearing completed claims. Do not silently reclassify unrelated events as proof.

Add negative controls for same-thread/wrong-turn answer, missing thread response, RPC error response, error-bearing terminal, duplicate response/request IDs and conflicting terminals. Existing wrong-thread and missing-terminal controls do not cover this.

### H2 — Require actual run-scoped CLI reads for measured Host samples

File: `server/cmd/server/rk1k_scenarios_test.go`, `hostSample`.

The real Host assignment asks the model to read collaboration memory and handoff using the scoped Multica CLI. The current criteria check role, actual injected body, authorization hash, process proof, recording and namespace, but never assert either CLI request occurred. `recording_complete` can succeed with just an agent message. A model that simply quotes injected knowledge without executing either requested command currently passes.

Require successful `mcb`/CLI requests for the exact admitted run's memory and handoff endpoints, using the trace's credential-purpose/platform labels, not command text alone or supervisor's own handoff snapshot. Record these as behavior criteria, keeping absence in an otherwise complete model run distinguishable from missing trace evidence. Add a fake negative control that emits the correct answer while omitting these real requests; it must fail model/behavior acceptance.

### H3 — Bind complete build/runtime artifacts, including non-Go embedded inputs

Files: `research/codex-real-20261104/compile-harness.py`, `run-fake.py`, `server/cmd/server/rk1k_codex_registration_test.go` and the future main-owned real execution package.

Both Python manifests hash only `server/**/*.go`. This omits `go.mod`, `go.sum`, embedded SQL/migrations/JSON/built-in skill resources and other build inputs. A changed embedded resource can produce a different binary with the same recorded source manifest; the selected CLI binary is separately hashed but there is no compile provenance for it in these scripts. Registration checks any nonempty `source_files` map rather than proving its required coverage.

Before a real run, bind the actual compile input set (Go sources, dependency manifests and all package embed files; build tags/toolchain/command), the CLI's own source/build provenance, registration, and exact executable chain. Local `codex` may be a Node launcher/symlink: hashing only its JS launcher does not bind the native Codex binary/package it dispatches. Main can supply a direct native executable with verified version or record the complete chain. Preserve the reported configured model and provider; do not treat the wrapper's hardcoded `--version` response as independent real CLI version evidence.

Add an embedded-file/dependency-manifest mutation control proving admission changes. The existing source.go-only integrity control does not exercise this gap.

### H4 — Durably stop after outer sample/evidence failure

Files: `rk1k_codex_provider_test.go` (`reservation.finish` call), `rk1k_codex_registration_test.go`, `rk1k_scenarios_test.go` (`rk1kSaveSample` / `rk1kRequireSample`).

The wrapper marks ledger `Complete=true` immediately after its process/RPC capture succeeds. Application-level observation, CLI-read/authorization/fence/proof validation and durable sample writing happen later in the test process. If any of those is incomplete or cannot be written, Go t.Fatal stops the current test but the durable ledger still says the latest execution is complete and `Stopped=false`. `rk1kReserveCodexLedger` can then accept a new identity if the same registration is reused. The original result should remain a consumed incomplete attempt, not an apparent successful cursor.

Separate physical completion from sample acceptance or add an outer finalization step. A next reservation must require the preceding sample's durable completion; missing sample publication/fixture errors after a process must durably stop or leave a non-resumable incomplete cursor. The zero-start K2 refusal stage and seed/setup failures also need stop propagation without incrementing paid count. Add sample-write/outer-classification failure controls proving no next paid process can be reserved.

## Confirmed strengths

- Provider identity remains Codex. The wrapper captures native bidirectional JSON-RPC and the actual app-server backend is exercised; no pretend-Claude frame translation.
- Ordinary producers, human approval/version changes, actual claim/prepare/authorize/observe/report calls and Host role/wakeup construction are retained.
- Real plans remove fake expected answers/native IDs/faults. Seed prefixes remain fake and separate. Ordinary scoped project read and quick-create business output are checked against actual HTTP/database effects.
- Ledger reservations precede child spawn; actual-start recording follows successful Start and occurs before forwarding RPC stdin. Failed Start conservatively consumes the reservation but is not labelled an actual launch. O_EXCL ledger lock, cap15, duplicate physical identity and prior incomplete refusal prevent ordinary concurrent/extra launch paths.
- Input/output capture uses private files and checks Sync/Close; registration/model/binary/executable/config/source hashes are validated before each real launch. A captured model differing from the registration refuses completion.
- Config copies are under test-owned private directories with 0600 initial file modes. Tests do not alter the user's global config; main still must verify effective copies and retain secrets only under the private evidence tree.

## Nonblocking limits and root-owned obligations

- Persisted startup/source hashes do not prove service availability; the bounded real baseline is still needed.
- The real entry currently validates DB identity inside the test body, after package TestMain. Root's real launcher must verify the owned DATABASE_URL before launching the test binary, as the fake runner already does.
- The ledger writes existing files using truncate+Sync rather than atomic replacement. A crash can corrupt the ledger, but parsing then refuses and prevents extra calls. Do not repair such a ledger by inventing a successful cursor; preserve it for manual reconciliation.
- The core library model/request counts may exceed physical-start count. The declared 15 limit is real Agent executions, not 15 HTTP requests.
- The fixture can complete with a wrong but well-formed semantic answer marked fail; the current caller stops on any failed sample (stricter than stop-on-incomplete), consuming no additional budget automatically.
- No real pass-through subprocess was exercised by this review. Negative controls need a test-owned native RPC executable behind the same wrapper/ledger to verify actual-start and stream handling without using a live model.

## Dynamic evidence

Main reports original normal01 failed before any launch due to duplicate Codex fixture runtime; C corrected it by reusing the genuine base runtime. A fresh normal2 is pending. This reviewer has not independently opened the raw failure artifact, so this is a coordinator-reported fact, not a verified dynamic result. Source findings H1–H4 remain.

Root selected the direct bundled native Codex executable (local-binding.json), resolving the Node launcher-chain ambiguity for the planned real run. Root also accepted complete embedded/dependency build-source binding as a prerequisite; H3 remains open until the new manifest and admission evidence are reviewed.
