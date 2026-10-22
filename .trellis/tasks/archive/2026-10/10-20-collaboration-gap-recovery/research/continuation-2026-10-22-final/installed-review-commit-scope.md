# Independent oldCLI commit and spec-append scope validation

Active task: `.trellis/tasks/10-20-collaboration-gap-recovery`; reviewer
`/root/evidence_check` (`trellis-check`). Read-only except these two assigned
`installed-review-*` files. No product tests/client/DB/container/environment action,
source/spec/ledger mutation, commit or archive by reviewer.

## Decision

**Accept commit a1d17d2ee5c5b723e65d9ddbbb5778e8a03e4065 scope.** The current
oldCLI three-file slice and exact new AC32 appendix match reviewed receipts.
The prior original AC32 recommendation stands; root owns final ledger timing.
This is shared-working-tree verification, not a standaloneHEAD typecheck claim.

## Commit isolation

The commit's only non-evidence files are the three oldCLI test/fixture/stdlib
relay files plus `.trellis/spec/server/collaboration-runtime-cli.md`. Its87
evidence files all remain within this task's `installed-*` lane. There is no
product implementation, global config, dependency, ledger or task-status change.

- Spec committed SHA `fba1a84b9e177018dc301b99f7555254cbf25683dac016c04e1731b3499a935a`.
- New19-line appendix SHA
  `675a6240ff767541252880f0782a6a2ebf2da4ca68cdee9fce4034b855695c5c`.
- Everything before the appended section remains exactly the parent commit's
  prefix. The current dirty spec diff contains only the two preexisting S4
  header/prepared closure corrections. The append-only index patch did not
  swallow those old hunks.
- Spec/final oldCLI test and TS/Go fixture hashes remain respectively
  `f556266ea6cf579b05ab3f6d2e21fe8c2f38552a0a6ee0a3e77a194511965f17`,
  `51487a607e05e52d9cf33a5357a889fc422f7713dc63e10b6720657a627d45c9`,
  `79e19af24fed37dfe3a44e022e9ef1f82ff323ef071186ad6ad14d7b2401571d`.
  All committed bytes equal current reviewed sources.

## Appendix vs actual contract

The paragraph accurately scopes immutable priorOCI byte/version provenance,
network-none/read-only private stdio expected legacy method/path, ordinary200 and9
state-preserving paused/unconfirmed/removed409 requests. It preserves independent
source membership and exact original own receipt, with no invented redacted flag.
No-file strict legacy Claim wire and unsupported-file noClaim come from actual
current20/47; real Linux managed daemon/proof/reconnect comes from separate current
2/2, not the oldCLI process. Commercial provider/rollout and publisher/oldDesktop
trust remain separate. No statement requires an absent olddaemon new command.

## Privacy and dependencies

The three sources execute only explicit immutable /app/multica issue commands.
Credentials stay private stdin and child environment, not arguments/evidence.
The host checks exact expected method/path/version/platform/OS/auth and forwards
body bytes/actual upstream response. Task-owned volume/tmpfs/config and cleanup
leave no userHOME/CODEX_HOME/config mutation. The87 committed evidence files have
zero concrete credential/token/JWT pattern matches in this read-only scan.

A known **type-only shared feature prerequisite** remains: the test imports
CollaborationCommandHistory from the current working-tree Core index/history
feature, absent in the standaloneHEAD Core export and schema. The import is
runtime-erased; actual test uses the committed control fixture, whose bytes equal
HEAD. Existing TestApiClient methods exist inHEAD; its dirty count row check is a
mechanical current helper correction. Current shared strict tsc/lint passed;
no independent standaloneHEAD typecheck is claimed. Root preserves and owns the
wider shared feature prerequisite, so this is recorded, not silently replaced
with another untracked import or an artificial compatibility type.

## Retained failures match facts

The six initial failures are fixture authoring/transport setup: collection
parameter, readonly cp, omitted zero exit frame, guessed historyURL404, another
actor's operation404, and invented redaction fields. Actual outputs support
these descriptions; none is counted as a product failure or unioned into a pass.
The final oldCLI1/1, current20/47 and actual managed reconnect2/2 are distinct.

## Findings (fixed)

None by reviewer. Root's appendix-only staging already preserved the old hunks.

## Findings (not fixed)

No new blocking issue. Known type-only working-tree prerequisite and existing
olddaemon/installedMain/OS/provider boundaries remain explicit. Root's second
current2/2 and ledger timing remain its own authorized completion sequence.

## Verification

Current strict tsc/lint/relay vet receipts unchanged and pass; read-only spec
`git diff --check` clean. Committed three files/current hashes, exact append/prefix,
91-file scope and87 evidence privacy scan verified. No product test rerun.
