# D: managed knowledge across squad roles

Added only `server/internal/service/collaboration_knowledge_squad_runtime_test.go`
and this evidence directory. Production code and default-closed gates are
unchanged. No sqlc generation, commits, subagents or real Provider calls occurred.

`TestCollaborationKnowledgeSquadRootMemberContinuation` exercises the actual
database and service path for a capacity-one squad:

1. A real human source/proposal/approval creates a nonempty workspace
   contribution. A persisted managed runtime is registered as built-in local
   Claude; both squad resources receive explicit owner consent.
2. The leader, member and continuation each claim with `KnowledgeVersion=1`,
   receive an independent real run namespace, prepare the exact approved body,
   bind the handoff/input digest and authorize their actual server-admitted
   sequence/nonce. Original-key preparation and authorization replays remain
   identical, with exactly one stored fact of each kind per run.
3. Both accepted human authors remain attached to all three runs. The member's
   real admitted delegation points to its parent leader and retains the exact
   dispatch text. Its original-input reader lists include both leader/member
   run IDs. Replaying the dispatch preserves the one member identity.
4. The member cannot claim the occupied leader slot and gets no premature
   knowledge descriptor. Two member comments and its proof coalesce into one
   continuation with both comment causes and the proof cause preserved.
5. All three independent run proofs settle, the final leader settles the group,
   and every capacity reservation is released.

The process termination/proof observations in this new service test are
**synthetic inputs to the real service**, explicitly labeled in source and test
output. The invocation/input hashes bind a service test fixture, not a claim that
HostExecutor physically ran here.

The layered evidence is:

| Boundary | Evidence |
| --- | --- |
| Real knowledge claim/prepare/authorize for leader → member → continuation | New service test above, normal and `-race` |
| Actual fake member stdin retains delegated assignment, uses an independent cwd and mandatory nonpersistent argv | Existing `TestHostKnowledgeActualInputPreservesHandoffAndFreshNamespace`, rerun in this increment |
| Mandatory persistence flag participates in the exact actual invocation fingerprint | Existing `TestHostKnowledgePersistenceIsMandatoryInExactInvocationFingerprint`, rerun in this increment |
| Real HTTP/MDT + DB + Runner/HostExecutor with source changes, lost replies and proofs | `../host-composite-acceptance-2026-11-01/handoff.md`, direct-agent scope retained |

Verification used only `multica_knowledge_host_composite_20261101_efb9efe6`, with
its identity checked before each database execution. The normal service test
passed **1 test**; the focused race run passed **1 test**; the fake-member package
run passed **2 tests**. All had zero failures and zero skips. `go vet
./internal/service` and formatting checks passed. Exact commands, names, source
hashes and version information are in `verification.json`, with raw logs and
before/after source inventories alongside it. The first execution passed; there
was no product regression or assertion relaxation.

The owned test remained unchanged during verification. Other active lanes
changed their credential/checkpoint/budget/wakeup tests and project handler/SQL
files; those exact differences are recorded in `verification.json`. Main's final
full Go run owns verification of the final combined tree.

This closes the missing server-side managed-knowledge squad proof by combining
these explicitly different layers. It does not add a real-provider squad
experiment, a physical full-server squad composite, model-understanding evidence
or another OS/account matrix. Main retains the final combined Go run and G2
rollout decision.
