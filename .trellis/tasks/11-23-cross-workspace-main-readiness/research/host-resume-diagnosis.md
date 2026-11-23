# Host continuation failure: incompatible knowledge-mode admission

2026-11-23. Read-only diagnosis on private source `73a28ed90` plus approved readiness changes. No product, fixture, harness or test edits and no new execution. Existing host/conditional lanes were left running.

## Conclusion

The original host-runtime case's second-launch `resumed === true` expectation conflicts with the **knowledge-enabled collaboration host policy used for this allocation**. The observed fresh session is explained by the actual negotiated knowledge path, including its empty-approved-set rule. This is a task-harness configuration mismatch, not evidence that production resume logic should be relaxed.

Run the unchanged ordinary-resumption original in a fresh host allocation with `MULTICA_KNOWLEDGE_CONTEXT_ENABLED=false`; retain knowledge enabled and `QA_KNOWLEDGE_CONTEXT=on` for S1's knowledge scenario. Do not change `expect(second.resumed).toBe(true)` to accept either result, remove no-session-persistence, or reuse the existing knowledge-bearing allocation/session ledger. The replay is still required before closing the failed case.

## Actual evidence

The failure is `e2e/collaboration-host-runtime.spec.ts:78`, after the initial Web run settles and the next Electron input is launched. Its retained attachment is:

`.omx/private/qa-work/main-readiness-20261123/core/final-host/results/collaboration-host-runtime-ae4a5-ly-resume-after-scoped-Stop-chromium/attachments/host-runtime-evidence-eda1c06f3c7dec614654938637cca152f87141ed.json`.

Both provider observations have:

- `resumed=false`;
- `knowledge_observation.section_present=true`;
- `knowledge_observation.contributions=[]`;
- `knowledge_observation.no_session_persistence=true`.

Their session hashes, workdir hashes and retained host ledger session keys differ. This matches intentional per-run isolation even without a knowledge contribution. The fixture exports only hashes and sanitized identity facts; no credential, full argv or native session pointer was printed or copied into this report. Its daemon errors/stderr and both browser-error arrays are empty at this failure.

The exact journey, fixture, provider-source and daemon-test-source hashes match current source and `final-host/source-start.json`. The attachment's daemon, Provider and Multica CLI binary hashes match `final-host/run.json`. Thus the diagnosis is anchored to the executed binaries/source rather than a different local implementation.

## Causal code path

1. Task-only `core/core-run.py:146–150` enabled `MULTICA_KNOWLEDGE_CONTEXT_ENABLED=true` and `QA_KNOWLEDGE_CONTEXT=on` for every `host` lane, including this ordinary-resumption test. S1 explicitly requires that flag (`collaboration-business-scenarios.spec.ts:406`); this shared allocation unintentionally applied its different lifecycle contract to the other original.
2. `server/internal/daemon/collaborationrun/host_runner.go:51` advertises supported knowledge protocol on the claim. `service/collaboration_knowledge_runtime.go:80–95` admits local supported hosts when the server flag is enabled and binds the knowledge namespace to **`claim.Identity.RunID`**. Admission does not require a nonempty contribution set.
3. `collaborationrun/host_session.go:40–44` appends the negotiated knowledge protocol and that per-run namespace to the session key. Successive runs therefore choose distinct actual host directories/session identities even on the same issue/agent/channel.
4. `host_runner.go:133–140` refuses a reusable pointer in a knowledge namespace and uses an empty resume ID. At `:195–203`, Claude launches include `--no-session-persistence`; at `:267`, any returned provider session ID is cleared before saving session state.
5. The fixture Provider at `e2e/fixtures/collaboration-host-provider.go:610–645` derives `resumed` from the real `--resume` argument and records the real no-persistence argument. Both observed false/true values are therefore consistent with the selected launch policy.

This is specifically **RunID-bound knowledge admission**, not a guessed input-revision or transient-cache explanation.

## Existing contract and counterfactual

`.trellis/spec/server/collaboration-knowledge-execution.md:247–256` states that every admitted collaboration run, **including an empty approved set**, uses a unique actual directory/session key and neither reads nor saves reusable native pointers. Claude print invocations must include `--no-session-persistence` before fingerprinting.

Existing canonical tests `TestHostKnowledgeActualInputPreservesHandoffAndFreshNamespace` and `TestHostKnowledgeEmptyAdmissionStillUsesFreshNamespace` enforce this boundary. The former rejects resume/continue argv and verifies the fresh key in actual cwd; the latter prevents empty admission from downgrading to a reusable namespace. They were inspected, not rerun for this diagnosis.

Without knowledge admission, `hostSessionKey` retains the same ordinary agent/channel/configuration key across groups, `host_runner.go:183` uses the retained session ID, and successful result pointers can be saved. That is the contract the original `second.resumed`, equal session/workdir, different temp-directory assertions test. An explicit knowledge-off allocation is appropriate isolation of those semantics; it does not weaken knowledge protection or alter a product default.

## Follow-up evidence needed

- Preserve the original `final-host` failure and its knowledge-enabled receipts.
- Record knowledge mode explicitly in the new run's nonsecret configuration/admission receipt.
- Replay the entire unchanged host-runtime case on a fresh knowledge-off DB/process/ledger; confirm its original session continuity and later scoped Stop/Resume assertions, not just the failing line.
- Keep S1's actual knowledge input/reuse acceptance under its separate knowledge-enabled scope and report both modes honestly. No real Provider-account claim follows from either deterministic fixture run.
