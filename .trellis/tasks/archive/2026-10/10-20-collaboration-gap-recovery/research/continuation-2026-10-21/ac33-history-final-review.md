# AC33 history and cross-client execution — independent final review

Reviewed 2026-10-21 against HEAD `7c77bf157c3e78786abef97034c6c60108211e7a` and the preserved shared working tree. Verdict: the current AC33 journey and bounded nullable-reason repair have passing execution evidence. No new product defect was found. The coordinator may promote AC33 through the acceptance audit; this reviewer changed only this report, not source, matrix, specs, environment, gates, or task status.

The final run is one combined production Web/actual Electron test, not two separately counted tests. It uses two distinct human accounts, two private capacity-one Linux daemons and an explicitly built synthetic provider. Its result is **1 passed / 0 failed / 0 skipped / 0 flaky**. Actual provider/version/account/quota support and packaged full Main startup are outside this result.

## Evidence identity and provenance

The original `/tmp/multica-gap-recovery-20261021-h6phxwo9/cross-client-run4.json` is SHA-256 `2569f54320469122d23ac80c85d1219d7ff26b0fafee7630aae666649adc1f31`. Its JSON reports start `2026-10-21T08:13:42.821Z`, total 29.039 seconds, expected 1, unexpected/skipped/flaky 0; the single test result is `passed`, retry 0, 27.441 seconds. The retained copy and screenshots listed in [ac33-run4-result.json](ac33-run4-result.json) all exist and match their recorded SHA-256 values.

The task-safe [ac33-cross-client-run4-identities.json](ac33-cross-client-run4-identities.json), SHA-256 `370bd5232900fa78c0a830c38845049c76223954464ef91747709735c9b780e6`, is exactly equal to the decoded `cross-client-execution-identities` attachment in that raw report. This was compared structurally, rather than trusting the safe file's summary.

The actual command recorded by the coordinator was:

```sh
python3 /tmp/multica-gap-recovery-20261021-h6phxwo9/admin-test.py env \
  COLLABORATION_CROSS_CLIENT_DAEMON=/tmp/multica-gap-recovery-20261021-h6phxwo9/continuation-linux/daemon.test \
  COLLABORATION_CROSS_CLIENT_PROVIDER=/tmp/multica-gap-recovery-20261021-h6phxwo9/continuation-linux/cross-client-provider \
  COLLABORATION_CROSS_CLIENT_HOST_IPV4=192.168.65.254 \
  pnpm exec playwright test e2e/collaboration-cross-client-execution.spec.ts \
  --workers=1 --retries=0 --project=chromium --reporter=json \
  --output=/tmp/multica-gap-recovery-20261021-h6phxwo9/cross-client-run4
```

This review did not relaunch the browser, Electron, daemons, or API.

[ac33-rerun-preflight.json](ac33-rerun-preflight.json) and the refreshed API running record prove API `http://localhost:18996`, listener PID `35969`, source ID `ce1302b0147cb2da420e5dc24069143c5f2a4a41933a56d9e4b8ce6711f28e90`, configuration ID `805fc993af4daf895e4de11a22cd372244d7845eed807b7eaa56a50f30b39cd3`. Production Web is `http://localhost:13916`, `next start`, listener PID `76944`, build ID `tP4r55FoDsuyXfi1EylGw`; the retained snapshot BUILD_ID matches. Renderer `http://localhost:6090` was HTTP 200 with index SHA-256 `337c08a228217fb574b77bb5fb6942bcbc40f2f1ce42587ceec5264204f3b35c`.

The refreshed API snapshot's history source matches the repaired worktree source exactly. The API's embedded historical commit `8ecb98883` does not by itself identify uncommitted code; [ac33-api-source-refresh.json](ac33-api-source-refresh.json), source hashes and listener provenance provide that evidence. Its only changed production file relative to the preceding snapshot was the history projection; the other listed changes were tests. Password/admin/managed fixture flags apply to the isolated task deployment. Production rollout and real provider accounts were not changed.

| Source or executable | SHA-256 at passing run/review |
| --- | --- |
| `e2e/collaboration-cross-client-execution.spec.ts` | `3485662826b2afc9ab82dadbbe3e3d71dfad6bcb9c3bee9c4419c63a6d3b5416` |
| `e2e/fixtures/collaboration-cross-client-execution.ts` | `eab460c2681c95696ea90034ed63b34fc523169bf2d10d151c079c98cdda6c40` |
| `e2e/fixtures/collaboration-cross-client-provider.go` | `356dfab491c55e0647e1523e30334c8043c597c2426e045ca22dd25545f17c88` |
| `server/internal/service/collaboration_command_history.go` | `a23ad56dae751bc9cf7f0ef226292f3bea7ff3661cc7ebc6310bde49ef79e526` |
| `server/internal/service/collaboration_command_history_test.go` | `869196731ce555206557c1464a4495a1aad6bab3aed22994ba56fcedb16df976` |
| explicit Linux `daemon.test` | `57f9aa8c2306992964aa714eef324fb5435e8218f143bb28702378adbe67d358` |
| explicit Linux provider | `80c3225030ff105764e3b65309d9b3e599c223a8914def1cb37ab6f3217a9f94` |

These are historical pass identities if a subsequent coordinator-owned extraction moves the helper or test. Preserve them rather than relabeling the old run with later file hashes.

## Actual behavior and cross-layer review

Issue `01a15d85-fe06-73b5-bbb6-6a455f97dba5` has Web creator `5db0d0db-67d3-4c51-b0ff-5c63c52799a3` and Electron handler `9bad3d86-fc04-44d1-ac7b-2a96f3cc59dd`. All accounts, installations, bindings, offers, adoption and owner consent originate in public API fixtures. The fixture does not seed claim/proof or alter database rows to make this execution pass.

| Execution | Target / channel | Run | Group | Namespace and observed timing (ms) |
| --- | --- | --- | --- | --- |
| A1 from Web | `5f6e710e-6b2f-4b8d-a3c8-a1863d5082d3` / `da4b95c3-db62-4b13-b2db-5a0558a81139` | `941e2474-7411-4eeb-a4e3-578a4470aa93` | `ded757b2-b24b-4a08-ad99-1ee40875f19d` | `pid:[4026535233]`; start 7160.478, release 14241.887 |
| B1 from Electron | `550f3d2b-913d-40f0-9298-dd7ab5939bc4` / `cb2d0065-3200-4975-8b47-a1ff11cab0cf` | `454423a9-c0af-4f3d-ad02-5a2160cf9ff5` | `74d449cb-1640-4205-8e06-c5e036cd88e2` | `pid:[4026535238]`; start 9955.538, close 10820.528 |
| One A follow-up from both authors | A target / channel above | `6f8887ac-ac7d-4090-b8d2-a327e193ece5` | `9a23e2e7-13f8-41c4-b642-d8eca1c9dabd` | `pid:[4026535233]`; start 15598.021, close 16123.709 |

B completes with natural proof while A1 remains live, `in_flight`, unknown proof and no business outcome. Concurrent A/B namespaces differ; reuse of A's namespace inode after A1 exits is permitted by Linux and does not imply run reuse. The first A batch is `22098347-0dfb-4983-a352-b6a69c541545`; the one pending follow-up batch is `2aa1bdeb-fa2e-4b77-860e-020497c59dfe`.

The three later A inputs retain sequences 2, 3 and 4 in that pending batch. Electron cancels only command `bef04517-cb73-4c37-a587-5cb075bf33bc` (sequence 3, originally authored by Web). Session/channels/targets/groups/runs and every other command compare equal before/after cancellation. Its exact cancellation event is `17f1bb23-628e-4919-8071-d850fa51fc81`, sequence 26, manual actor the Electron handler, reason `cancelled_by_participant`; it stays group-less history.

The follow-up provider receives exactly command `b1853ed7-62af-4c87-8c85-bac66a3de304` from Electron and `d04622b0-c02b-444c-bc83-891af18bc2d5` from Web, in sequence order 2/4. It receives no cancelled input. Original provider bodies, authors, target/channel/batch and installation match the submitted receipts. Group input memberships, consuming/reader run IDs, recorded resource owner/runtime/workspace, run/target/group and result identities also match. The reviewer independently replayed these structural invariants from the retained JSON; all passed.

Final state contains five commands (four succeeded, one cancelled), three succeeded groups/runs, natural `exited_proven` plus business `success` on all runs, no active channel group and no target holds. Daemon summaries contain exactly A claims 2/B claims 1, capacity one, `invalid_capacity=false`, one available slot/zero active tasks, exit 0 and empty stderr.

Source assertions independently open command history and run records in both actual clients after reload (`spec.ts:137`, fixture `inspectCrossClientRecords`). Each shows original body/author/typed target and terminal status; run records show exact input command IDs, synthetic result and confirmed termination. Refresh leaves the execution projection unchanged and does not start another provider. Captured client writes are exactly five sends, three Web/two Electron, five distinct original keys, one deliberate Resume and one cancellation; no page errors or ambient workspace account headers are accepted.

The natural proof path is real: the daemon harness runs production `runBatchPoller` with `MaxConcurrentTasks=1` and the actual initialized collaboration poller; Runner Claim/Permit/Burn/Start/observe/Reap/Seal/Proof/ACK calls remain intact. The stdlib provider observes `/proc/self/ns/pid` and the frozen input, blocks on its socket, and writes only a business result. It neither fabricates a termination proof nor calls a mocked runtime endpoint. Native Electron uses an actual Electron binary/BrowserWindow and built renderer through the existing account acceptance harness; native-service stubs remain a full-Main/packaging boundary.

## Nullable-reason regression and retained failures

The narrow helper maps invalid or empty `pgtype.Text` to nil and preserves every nonempty value unchanged, including whitespace. It is used directly by `collaborationCommandHistoryEntry`; generic `util.TextToPtr`, stored reasons and strict Core `reason: z.string().min(1).nullable()` are unchanged. There is no authorization, queue, proof or public schema expansion in this repair.

[history-empty-reason.md](history-empty-reason.md) and raw result companions confirm actual schema-737 `service_s5` DB execution, not a no-DATABASE_URL skip. `history-projection-database-20261021.json` records PostgreSQL 17.11, migration 737 and the real `agent_runtime.collaboration_capabilities` column.

```sh
# From server/, executed by the preceding verification lane:
GOTOOLCHAIN=go1.26.6 python3 /tmp/multica-gap-recovery-20261021-h6phxwo9/with-db.py service_s5 \
  bash ../scripts/go-test-with-agent-cli-guard.sh go test -json -race ./internal/service \
  -run '^TestCollaborationCommandHistoryProjectsEmptyTerminalReasonAsNull$' \
  -p 1 -parallel 1 -count=1 -timeout=30m

GOTOOLCHAIN=go1.26.6 python3 /tmp/multica-gap-recovery-20261021-h6phxwo9/with-db.py service_s5 \
  bash ../scripts/go-test-with-agent-cli-guard.sh go test -json -race ./internal/service \
  -run '^(TestCollaborationCommandHistory|TestCollaborationCommandCancellation|TestCollaborationFIFOCancellationAndPromotionSerialize)' \
  -p 1 -parallel 1 -count=1 -timeout=30m
```

Focused result: one top-level pass, zero fail/skip, package 2.985 s, raw SHA-256 `2895d3429b8aa80887649868106eee6b6a8f3150f42e33926c0b13a40596096b`. Adjacent result: 13 top-level/39 including subtests passed, zero fail/skip, package 16.367 s, raw SHA-256 `18792b5bad5a478a66294a369f45a26fa06426cbeb45173699541e3fa854983b`. Source hashes stayed unchanged. This reviewer parsed actual Go events and verified these counts. The current regression drives managed submit/Resume/Claim/Permit/outcome/proof, then asserts success history with nil reason and wire `null`; its separate manual cancellation fixture asserts exact reason. Its local `t.Setenv("MULTICA_AUTH_MODE", "legacy")` correctly restores the mode required by that second legacy JWT fixture, without weakening service authorization.

The previously missing AC33 identity `TestCollaborationQueueCancelUsesCurrentRolesAndPreservesOrder` also has an actual passing event in `go-service-partition-2.jsonl:2253` at `2026-10-21T13:34:51.695496+08:00`, elapsed 1.65 s, in the guarded schema-737 `service_s6_collab` partition. The authoritative audit can remove that outdated missing-evidence entry when incorporating this report.

Keep all earlier failures as failures. `history-empty-reason-red.jsonl` SHA-256 `771631428cc70751ea00049d0ebe36ee17882286924cbaff5c00f07fdb23daab` genuinely failed on empty reason; misleadingly named `history-empty-reason-green.jsonl` SHA-256 `d1da0a340189431b233da14bd95a82ee220308fa2976ae247e06c0259e4511a1` failed on later fixture auth. The first DB race attempt also failed on fixture auth. Cross-client runs 1/2/3 remain failed enrollment-fixture / namespace-inode-assumption / product-projection attempts. Old green filenames and the earlier skipped DB command are not passing evidence.

## Fresh reviewer verification

The following read-only commands were rerun from this repository against the exact source hashes above, using the existing static configs (strict inherited TypeScript, `noEmit`, `incremental=false`). No build, provider, browser/native launch or environment mutation was performed.

```sh
pnpm exec tsc --project /tmp/multica-gap-recovery-20261021-h6phxwo9/cross-client-static/tsconfig.json
node packages/views/node_modules/eslint/bin/eslint.js \
  --config /tmp/multica-gap-recovery-20261021-h6phxwo9/cross-client-static/eslint.config.mjs \
  e2e/fixtures/collaboration-cross-client-execution.ts \
  e2e/collaboration-cross-client-execution.spec.ts
pnpm exec playwright test e2e/collaboration-cross-client-execution.spec.ts \
  e2e/collaboration-control-journey-web.spec.ts \
  e2e/collaboration-control-journey-desktop.spec.ts --list --reporter=list
gofmt -d server/internal/service/collaboration_command_history.go \
  server/internal/service/collaboration_command_history_test.go \
  e2e/fixtures/collaboration-cross-client-provider.go
# From server/:
GOTOOLCHAIN=go1.26.6 go vet -p 1 ./internal/service
```

TypeScript exit 0 (3.61 s), ESLint exit 0/no diagnostics (1.29 s), discovery exit 0/five tests in three files (1.35 s), gofmt exit 0/empty diff, service vet exit 0/empty diagnostics (2.23 s). pnpm printed only its existing ignored package-field warning. Discovery is recorded as discovery, not a new execution. Browser/runtime/DB suites were not redundantly repeated after the retained pass and stable source hashes.

## Safe Lore commit recommendation

Do not stage the entire untracked history feature as a nullable-reason fix. `collaboration_command_history.go` includes the new DTO, pagination/ACL reader, generated history queries and exact cancellation-event decoding. Its test file includes provenance/lifecycle/rollback and cancellation helpers. Generated `models.go`, queue/group/lifecycle changes and the history/cancellation query/migrations belong to a broader shared S3b dependency slice; simply staging both full files would not form a self-contained change.

The lightest bounded option is the coordinator's proposed local extraction:

- Move only the existing helper, preserving name/body, to `collaboration_command_history_reason.go` (package service, only `pgtype` import). Keep the untracked projection calling it and preserve the complete current CommandHistory regression as integration evidence.
- Add `collaboration_command_history_reason_test.go` that depends on tracked helpers only. Cover invalid SQL-null text, valid empty text, exact nonempty manual/failure text and whitespace preservation; assert exact JSON `{"reason":null}` through a local struct containing `Reason *string` tagged with the JSON key `reason`, rather than the untracked history DTO.
- Its DB regression should use tracked `newManagedCollaborationFixture`, `submitManaged`, `resumeManaged`, `claimManaged`, Permit/outcome/proof and then `f.fx.QueryRow(t, "SELECT state,reason FROM collaboration_command WHERE id=$1 AND session_id=$2", commandID, sessionID).Scan(&state, &reason)` with `string`/`pgtype.Text`. Assert a real `succeeded`, valid-empty stored reason and exact nullable wire through the helper. No direct state mutation or synthetic success-row insert is necessary. `FinishCollaborationBatchInputs` already persisted its string reason at HEAD; the fixture and managed lifecycle are tracked at HEAD.
- Prove those two files compile/run against a repository snapshot containing exactly HEAD plus the pair, outside the shared checkout, with the task DB and CLI guard. This catches accidental references to untracked S3b/S4 helpers or regenerated fields. Alternatively use a complete Go overlay that hides every untracked source and restores each tracked dependency to HEAD; a partial overlay over history alone is insufficient.

This is a mechanical extraction, not approval for changing public DTOs, generic utilities, schema, cancellation semantics, provider support or cleanup of unrelated tracked hunks. Stage only the verified pair and explicitly owned evidence; inspect the index before a Lore commit. The full current-tree history race and Web/Electron run establish integration behavior of the shared tree, not that this isolated helper commit alone delivers the new history API. Record that distinction in `Tested`/`Not-tested`; rerun the focused helper and current-tree history test after moving files, retaining old run hashes. Root owns the extraction, final staged verification and commit.

## Unfixed findings and limits

No new product issue requiring a fix was found. At review time the matrix and older cross-client/history handoff reports still describe AC33 as open; their historical failures remain valid, but a current final addendum/catalog/matrix update is coordinator-owned. The current server wire spec and Core pointer already describe nullable reasons and separate natural kernel proof from synthetic service proof; subsequent extraction should keep those references accurate.

This pass does not validate paid real providers, general provider support, Windows/macOS execution containment, packaged full Main lifecycle, S3/Redis fault scenarios, or all other 96 criteria. The standalone DB test uses a synthetic proof; natural Linux proof is supplied by the combined client journey. Production creation/execution rollout stays closed; task-only fixture gates do not authorize rollout. No task/parent archive or completion is recommended until the full 97-row ledger has current evidence.

Changed file in this review: `research/continuation-2026-10-21/ac33-history-final-review.md` only. No production simplification was performed; the recommended helper extraction remains a coordinator follow-up.

Finalization note: while this report was being saved, the coordinator's independent implementation lane began the approved helper extraction. The projection's hash changed to `6ddc3e26ef0d06a449140e6638731cb9693660b38b1d78ccd83d3c0c860ff2be`; comparison against the retained passing API snapshot confirms its only change is removal of the exact helper body into `collaboration_command_history_reason.go` (SHA-256 `b015d70086ff0646cd3ca16ad4012e2d48b2dea15d9138a78754c46c8e0f5a1b`). The original DB regression remains SHA-256 `869196731ce555206557c1464a4495a1aad6bab3aed22994ba56fcedb16df976`. This reviewer did not change those files or reattribute the prior pass to their new hashes. Extraction-specific verification and the two-file HEAD isolation proof remain owned by that lane; report links and whitespace were checked after this observation.

## Final standalone commit check

The extracted pair is now reviewed and **approved for the bounded Lore commit; no defect or blocker remains**. Current helper/test hashes are unchanged at `b015d70086ff0646cd3ca16ad4012e2d48b2dea15d9138a78754c46c8e0f5a1b` and `3152071d3350f18099e924cd978067763b98dfbaa3e07c17ceb70523f17935ec`. The helper remains exactly the narrow null/empty mapping; its regression covers six nullable/exact-value cases and a real managed success, reads the persisted `state,reason`, and asserts exact JSON null without importing the untracked history DTO/query. Fresh pair gofmt produces no diff.

The reviewer independently compared all 4056 archived HEAD server/guard files against the outside snapshot in `history-reason-isolated-source.json`: zero content mismatches, missing files or extra source files, with exactly the two reason files added. The retained scoped archive SHA-256 `b866cda0224116cfcbe7c99b4ea57b51e67d4f847ef9f70b2ce33f417e1540c3` matches a fresh read-only `git archive` of HEAD `7c77bf157c3e78786abef97034c6c60108211e7a` for `server`, `scripts/agent-cli-command-names.txt` and `scripts/go-test-with-agent-cli-guard.sh`. No shared dirty S3b/S4 source was copied. An initial comparison against a whole-repository archive was overbroad; the preserved snapshot contains precisely the build-required server and two guard scripts, so the correctly scoped comparison passes.

Actual `history-reason-isolated-head-race.jsonl` SHA-256 `132fe8c59a6611a9d63a15039ade1d38e194ea8b6e83d1b55781a7299c180ceb` reports **2 top-level/8 total passes, 0 fail/skip**, exit 0, package 3.106 s, empty stderr, against schema-737 `service_s5`. The recorded command is the guarded `go test -json -race ./internal/service -run '^TestCollaborationCommandHistoryReason' -p 1 -parallel 1 -count=1 -timeout=30m` from the isolated HEAD snapshot. This proves the pair's compile and DB regression independent of the untracked feature.

The post-extraction current-tree integration run separately reports **15 top-level/47 total passes, 0 fail/skip**, package 16.798 s, raw SHA-256 `552081bafbfde04feb1968fa2239415320de6b299613bac20b130ad1bd76eac1`; all source hashes match. The intentionally restored generic-mapping overlay remains red for valid empty text and managed-success wire, without editing shared source. Counts and outcomes were checked from actual JSONL events, not filenames.

The standalone helper has no production caller in HEAD until the broader history feature is committed. Therefore this commit delivers the independently tested projection primitive/regression and its executable contract; the retained full history/Web/Electron result is shared-tree integration evidence. The spec and focused report accurately state the nullable wire contract and must not be read as proof that the two-file commit alone adds the endpoint. Root's described scope excludes untracked S3b feature code and generated SQL; no index writes or additional product suites were performed by this reviewer.
