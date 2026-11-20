# S2 A5 source-member fence — independent recheck

- Reviewer: `/root/s2_server_check`, 2026-11-14.
- Verdict: **S2A-F01 resolved**. The remaining server-side source-authority race identified in the 2a follow-up is closed by the reviewed repair. No new finding in this bounded change.
- Scope: only the two files and retained evidence in `server-increment-2a-source-fence-results.json`. Earlier 2a/follow-up reports remain unchanged. This does not accept 2b, the shared client, all S2 writers or A01–A12.

## Findings (fixed in the implementation)

### S2A-F01 — keep A5 source membership valid through scope recording

`collaborationStageSourceMemberKind` now includes `stageKindConfirmScope`. The existing workflow consequently locks the current member row with `FOR SHARE NOWAIT` before its final issue-access read and keeps that lock through the scope transaction. A nonempty selection still checks current `SourceMember`; an absent member row does not itself refuse an empty external scope confirmation. No parallel authorization mechanism was introduced.

The regression uses the real DBTX seam immediately before the generated `CreateCollaborationStageDecision` query. At that point a second database connection invokes the canonical generated `DeleteMember` query and attempts to commit. The original red log contains the decisive witness:

```text
hook=1 deletion=<nil> confirmation=<nil>
```

After the repair, the test requires exactly one hook invocation, successful A5 completion, and a `*pgconn.PgError` with SQLSTATE `55P03` from the second connection. Here `55P03` comes from that connection's task-local 50 ms `lock_timeout`; the A5 acquisition itself uses the existing NOWAIT lock. This proves the member DELETE is blocked by the live fence rather than merely being rejected by an unrelated application condition.

The same test then deletes the member successfully after the A5 commit, submits a fresh request carrying the cached nonempty selection and requires `stage_scope_child_unavailable`. A further fresh request with an empty selection succeeds for the still-designated external product approver. The assertions therefore cover scope-first, revocation-first and the preserved empty-scope control.

This is a database concurrency proof using the actual member-write query. It does not claim execution of the full membership-revocation HTTP handler or its unrelated cleanup effects.

## Findings (not fixed)

None within F01. The eight known unrelated F2 migration cleanup omissions remain visible in the earlier independent global-inventory failure. This repair changes no migration or registry file, so that result was not rerun or relabeled as passing.

## Verification

| Check | Result |
| --- | --- |
| New frozen source | **2/2 hashes match** the dispatch and implementation report. |
| Prior reconstruction | **4,872 files verified** against the original full snapshot, certified 2a follow-up files, fixture supplement and disclosed 826–828 DDL before applying these two files. No live 2b source was copied. |
| Raw evidence | **4/4 logs and direct exit sidecars match**; parsed named inventories match the report. |
| Original red | One A5 failure with both writes committed; one already-passing C1 control. |
| Normal green | A5 passes, plus the existing C1 control; zero failures/skips. |
| Race green | The A5 fence test passes with `-race`; zero failures/skips. |
| Later split-test evidence | A5 also passes in the supplied nine-name QA run. The other names are outside this review and provide no 2b acceptance here. |
| Independent formatting | **PASS**, `gofmt -l` on the two frozen files; empty output. |
| Independent Go type/static check | **PASS**, `go vet ./internal/service` on the supplemented frozen reconstruction, including the standalone fence test; empty output. |

The original C1 control now lives in the 2b test-report test file and was intentionally excluded from the 2a reconstruction. The review uses the standalone A5 test's frozen source and its retained normal/race evidence; it does not infer any C1 or QA implementation result.

Raw independent checks and exact source binding are under `/Users/artisan/.multica/acceptance/pay241-20261114-resume/s2-2a-source-fence-check/`. The adjacent JSON records hashes, commands, inventories and the closure reference. This reviewer ran no database-backed test, migration, API, daemon, model, browser or Desktop process and made no product or Git writes. The implementation's normal/race tests were inspected and verified as retained evidence, not independently rerun against its allocated databases.

This report closes F01 and the remaining server-side C02 write fence. It preserves all earlier failures and their chronology. The shared client's protected-digest behavior, later-stage implementation and complete S2 acceptance remain governed by their own checks.
