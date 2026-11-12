# v6 private verification adapter independent review

Verdict: **changes-required**, **1 blocking finding** in launcher `836f2d066b74edbb14e39fda49ac314e40f11b9618cbd8d07eb44c020747ad1f`. No combined launch clearance was issued.

| Component | Exact identity | Verdict |
| --- | --- | --- |
| Go adapter | `eabbfdfe047ec686a6e25d413be9dc14dc1ce068ac72317530d6ec049f84aa29` | pass |
| pnpm adapter | `7a416eb60f1bdbe6bdf7421d2f61346d74d0e2575ae8a5236e450627db63ff48` | pass, unchanged from v5 |
| Launcher | `836f2d066b74edbb14e39fda49ac314e40f11b9618cbd8d07eb44c020747ad1f` | changes-required |

## Findings (not fixed)

**AV6-1 — medium, blocking:** `adapter-v6/run-make-check.py:45` quotes literal values for Bash, then passes the same file to `make check` at lines136–139. The actual candidate Makefile first runs `include $(ENV_FILE)` (line8) and globally exports values (line29). Make evaluates its own expressions inside single-quoted assignment text; Shell quoting does not protect this parser.

The independent probe used the exact v6 canonical writer and a reviewer-owned Makefile containing only the same include/export statements and one harmless recipe. In a sandbox with all network denied, `/usr/bin/make` executed an accepted literal `$(shell printf … > <owned marker>)` and created the marker. No business Makefile or private env was executed. A second owned probe shows a multiline literal (supported by the current parser/Bash tests) produces an exit2 `missing separator` error in Make. The non-zero output was read and retained; no retry or business-failure classification was made.

The current eleven launcher contracts use a fake Make program that directly sources the file with Bash, so they do not protect the real Make parser boundary. Reject Make-unsafe/unsupported input before producing its file, or give Make a separate safe input surface. Add a pure actual-Make include/export regression and an explicit multiline contract. Only the main session may delegate this repair; this reviewer modified no implementation.

## Prior findings

- **AV5-1 closed:** v6 removes both API pool env keys after non-test/compile-only forwarding, before fresh and recursive executable tests. API paths retain10/1; test-owned later overrides and HOME still work.
- **AV5-2 partially fixed:** source parsing does not evaluate shell, and canonical input remains literal in Bash. The full downstream contract remains blocked by AV6-1.
- **AV5-N1 closed:** actual adapter and launcher file hashes must equal configured and independently reviewed hashes before Make. Altered source/adapter/launcher, dirty candidate, missing clearance/nonprivate HOME and prior record overwrite are rejected by contracts.
- **AV5-N3 addressed:** the launcher selects a private owned0700 HOME under candidate evidence, preserves explicit Go/cache/browser paths, and avoids global task-root overrides.

## Verification

- Independent network-denied Go adapter contracts: **17 pass, exit0**.
- Independent network-denied launcher contracts: **11 pass, exit0**; this remains limited to their fake Make boundary.
- Original actual pure-file dbstats evidence was independently audited: v5 **7 run,3 pass,4 fail,exit1**; v6 **7 run,7 pass,0 fail/skip,exit0**. Both raw JSONL hashes, seven actual names, command file lists excluding DB TestMain and exact existing source hashes match. The reviewer did not rerun these actual Go tests.
- All five versioned source files AST-parse, and their runtime copies remain byte-identical. pnpm remains byte-identical to v5.
- Actual Make literal evaluation and multiline refusal are preserved separately in `make-loader-probe.json` and `make-multiline-probe.json`.
- Repo lint/TypeScript typecheck were not run in this private Python review slice. No Python lint package is installed. The single authorized fullcheck owner remains responsible for whole-package verification.

The exact frozen Go/pnpm component pass can be shared with that owner; it does not authorize a different launcher or accept a product candidate. The final5 example registration is not a permit. No DB operation, service, GUI, provider CLI or duplicate wholecheck ran, and original exit2 remains unchanged.

Evidence: `review.json`, `source-and-pool-verification.json`, `go-contracts.json/.log`, `launcher-contracts.json/.log`, `make-loader-probe.json` and `make-multiline-probe.json`.
