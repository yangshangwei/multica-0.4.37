# Agent output client implementation — 2026-11-19

Disposition: **SHARED CLIENT IMPLEMENTATION VERIFIED.** This report covers the
allocated Core/Views parsing and rendering slice. Server output writing, stored
outcome freezing, current production Web/Electron execution and full S3 acceptance
remain Main's separate work.

## Ownership and source

- Agent: `/root/agent_output_client_implement`, dispatched Trellis implementer.
- Task: `.trellis/tasks/11-13-pay-241-s3-recovery`.
- Product source:
  `/Volumes/artisan/.multica-acceptance/pay241-s3-output-20261119-tm89sjgy/source/client-work-r01`.
- Initial write ownership was the attachment schema, run-record schema, shared
  run-record component and their colocated tests. Main subsequently authorized
  exactly one null guard in private `packages/core/api/client.ts`, after its
  actual type/runtime failure was identified. No attachment renderer-file change
  was necessary.
- ROOT product source, other private clients, server/DB/browser/process state,
  task state, specs, plans, manifests and Git were not changed. This report and
  its companion JSON are the only ROOT writes by this agent.

The complete saved native hook, task PRD/design/implement, accepted output
contract, CLAUDE.md and relevant Core/Views/record/content guidance were loaded
before product work. The Trellis pre-development, test-first, systematic
debugging and verification workflows were applied directly.

## Product behavior

`attachment-schema.ts` preserves the existing name/type/size/version/date and
source/issue constraints. Human rows retain their draft and user UUIDs and accept
absent or explicitly nullable new provenance. An Agent row requires null human
draft/uploader, nonzero Agent/run UUIDs, a canonical positive signed-int64 attempt
string and a delivered issue/time. Mixed, half-present or invalid attribution
refuses the entire response through the existing redacted `parseWithFallback`.
Human draft collections continue to exclude Agent outputs.

`run-records-schema.ts` preserves optional comment/result `attachment_ids` using
the existing canonical, unique, maximum-32 attachment-reference schema. Legacy
absence remains absent; explicit empty sets remain explicit. The original input
variant retains its required reference list and existing cardinality. No request
wire or client reconstruction of server-frozen results was introduced.

`RecordedEntry` uses `CollaborationCommentAttachments` for input, comment and
result references. Input labels/ID display remain input-specific. Metadata,
preview and download still use the existing protected issue queries and helpers.
There is one shared rendering path and no new local state or platform wiring.

The additional API guard checks `result.draft_id !== null` before the existing
human-draft recovery identity comparison. A wrong-kind recovery response is
unavailable/unconfirmed; returning null does not confirm rejection, retire an
intent or create a new request.

## Actual RED and retained failures

All test runs used the private recorder and closed client configuration. Product
files remained at their allocated baseline until the first schema/component RED
captures completed.

| Record | Files | Named cases | PASS | FAIL | Actual failure |
| --- | ---: | ---: | ---: | ---: | --- |
| `client-output-core-red-r01` | 2 | 99 | 79 | 20 | Agent row rejected; new provenance/refs stripped; malformed refs ignored |
| `client-output-views-red-r01` | 1 | 3 | 2 | 1 | Real comment record lacked `Preview agent-output.txt` |
| `client-output-core-red-r02` | 2 | 99 | 79 | 20 | Same product RED after making the case-equivalent UUID fixture contain a hex letter |
| `client-output-views-red-r02` | 1 | 3 | 2 | 1 | Same missing-control RED after using the real `Close preview` label |
| `client-output-core-draft-recovery-red-r01` | 2 | 100 | 99 | 1 | Actual `ApiClient` recovery threw on null draft identity |

The last RED was recorded before the authorized API guard. Its failing name is
`keeps human draft upload recovery limited to human uploads`. The corresponding
final 100-name Core GREEN set is identical. The final Views set is identical to
its corrected three-name RED set. The earlier 99-name Core RED/GREEN sets also
match; the API regression is the one later addition.

Both initial package type checks (`client-output-*-typecheck-r01`) failed solely
at `api/client.ts:1140`, where nullable `draft_id` reached a string-only identity
helper. These failure records remain intact. `client-output-views-lint-r02`
contains an incorrect invocation with an extra source-directory argument; Node
reported `MODULE_NOT_FOUND` before ESLint ran. Its corrected fresh invocation is
`client-output-views-lint-r03`. No failed record was overwritten or relabelled.

## Final checks

| Final record | Files | Named PASS | FAIL / pending | Exit |
| --- | ---: | ---: | ---: | ---: |
| `client-output-core-green-r02` | 2 | 100 | 0 / 0 | 0 |
| `client-output-views-green-r02` | 1 | 3 | 0 / 0 | 0 |
| `client-output-core-controls-r01` | 5 | 95 | 0 / 0 | 0 |
| `client-output-views-controls-r01` | 2 | 24 | 0 / 0 | 0 |
| `client-output-core-typecheck-r02` | Whole Core | — | — | 0 |
| `client-output-views-typecheck-r02` | Whole Views | — | — | 0 |
| `client-output-core-lint-r02` | 5 touched files | — | — | 0 |
| `client-output-views-lint-r03` | 2 touched files | — | — | 0 |

The four final Vitest profiles executed **222 named cases**: Core 195 and Views
27, with no failure or pending case. Per-file counts are:

- Core schema: attachment 32, run records 68.
- Core existing controls: run-record API transport 5, delivery API 6, delivery 23,
  preparation schema 34, protected run-record paging 27.
- Views: new run-record wiring 3, existing attachment panel 12 and execution panel 12.

The DOM suite exercises the real shared file controls, preview bytes and a
synthetic `desktopAPI.downloadURL` bridge. It retains input/legacy behavior and
proves that legacy records with no references do not request file metadata. Pure
schema/identity/boundary matrices remain in the colocated Node suites.

Commands ran through:

```text
python3 <allocation>/helpers/record-run.py <unique-label> \
  <allocation>/configs/client-tests-r01.json <private-package-cwd> \
  <allocation>/source/client-work-r01 /opt/homebrew/opt/node@22/bin/node \
  <existing-private-node_modules-tool> <arguments>
```

Vitest arguments were `run <explicit test files> --reporter=json
--no-file-parallelism`. Type checks used `tsc.js --noEmit --incremental false`.
ESLint selected only the seven touched files across their owning packages and
used `--max-warnings 0`. The companion JSON records every exact argv/cwd,
config/tool/recorder hash, raw output hash and named inventory. No dependency was
installed and no ambient environment was inherited.

All final check stdout/stderr and source-before/after hashes match their recorded
results. Final type/lint output is empty. Each final check has zero
source/config/tool drift and its full source-after inventory equals the current
private source.

## Final source delta and integration guard

The initial manifest contains 4,133 source records; final source has 4,134.
Exactly seven paths differ: four product files and three colocated test files.
All existing source modes are preserved. The API client retains its allocated
`0755` mode; the other touched files are `0644`. The new DOM test is `0644`.

| Path | Final SHA-256 |
| --- | --- |
| `packages/core/api/client.ts` | `284a12fa28dfebdfee58098d3570327c19a59cd99f3b5b65b470ac8f94199695` |
| `packages/core/collaboration/attachment-schema.ts` | `15358b441161ee340e1a8ca4c8b3ccec2b5ea4c01b55668c335ac460ed105ed9` |
| `packages/core/collaboration/attachment-schema.test.ts` | `9e1ff6d1571d9a47cc6042128a7b97d2e33ba2721753a5fffe148504daaf96c7` |
| `packages/core/collaboration/run-records-schema.ts` | `1d01c9f04a6bdb8ca18ef83360e6f5d184879e10b81e03c358784aa85b10a82e` |
| `packages/core/collaboration/run-records-schema.test.ts` | `81c99a488e9f7e3630f1d4879fb5e34fd020a357a430bc8a2e2856c5a3b52896` |
| `packages/views/collaboration/run-records.tsx` | `7e64fc4423e92500a01b5001bf68c15280d22858c559faef7fb07fc55f807cfc` |
| `packages/views/collaboration/run-records.test.tsx` | `b6603e212563cb7f046a34191380d5c39cad2775aa8f2d07485f4e167039a095` |

Reversing the following single line reconstructs the initial full API file SHA
`f3e16943502b902e90dee526135c0dc2ff692826f286c825f4409fa11455487b`:

```ts
return result && result.draft_id !== null && sameResourceIdentity(result.draft_id, draftId) ? result : null;
```

The line including its original indentation and LF has SHA
`f42ff81a4daeecc27799397bd88158450b7d99a20c82435fae8e460c6d6d0bc2`.
Main must integrate/stage only this API guard alongside the other owned files;
the private API file contains existing WIP outside this change.

## Remaining limits

These are pure TypeScript/jsdom checks. They do not prove a real Provider output,
encrypted storage commit, server-owned frozen result, database permission fence,
production Web rebuild, actual Electron download IPC, runtime withdrawal or
older installed-client output display. Main's server/current-client acceptance
and independent Trellis review remain required. This report closes no parent
test case or S3 requirement.
