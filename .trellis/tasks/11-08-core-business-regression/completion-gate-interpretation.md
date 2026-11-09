# Completion criterion: complete coverage with repaired impacted suites

Independent requirement interpretation; this document does not declare current completion.

The user's objective requires complete core end-to-end coverage, fixes for discovered problems until tests pass, and a report. The current PRD requires exact candidate/configuration identity, preserved failures, no weakened assertions or retry-to-green. The accepted implementation step4 explicitly says “Repeat impacted complete suites.” Neither the user nor these artifacts requires all441identities to pass in one process invocation; many require mutually exclusive auth/provider configurations and legitimately skip in the baseline.

Therefore a complete final441 traversal plus diagnosed narrow repairs and passing complete impacted suites can satisfy the objective. This is not permission to settle for a smaller test set: every required identity and explicit configuration still needs current compatible evidence, every discovered failure must be closed or explicitly belong to the already accepted exclusions, and reporting must distinguish original failed gates from repaired passing runs. A reviewer-created “restart the entire441run after every fixture-only edit” condition is not itself an additional user acceptance requirement.

## Evidence required for the newly failing DELETE wait

1. Finish frozen baseline-final3 and reconcile its complete441inventory/exit/global errors/retries. A progress log or past pass cannot close the new failure.
2. Diagnose the actual DELETE-wait failure using retained trace/API state/visible controls. Prove whether it is a fixture precondition or a product defect. A test-only diff alone is not proof. A later successful rerun without a cause/repair is insufficient and remains retry-to-green.
3. Keep the original failed data and demonstrate the repaired helper still uses actual UI deletion, original request/receipt/version/authority assertions, no force click or increased timeout. Preserve all parent journey persistence/history/identity assertions.
4. Re-run every complete consumer suite with the repaired helper, zero automatic retries. Source search currently finds exactly two consumers of exerciseCollaborationCommandHistory: e2e/collaboration-account-desktop.spec.ts (11cases) and e2e/collaboration-command-history-web.spec.ts (1case). Full12case passing evidence is the correct regression scope if the diagnosis proves the change is local to this helper.
5. If diagnosis instead shows dependence on accumulated state/order/load from other specs, or a changed shared fixture/public API/runtime path, scope must include that interacting context. Passing isolated12cases cannot disprove a known whole-run-only regression. Reproduce the causal condition deterministically or rerun the relevant order/context; do not simply label it flaky.
6. Audit exact final test-file hashes and unchanged production runtime/source/package bytes. A helper-only edit does not require rebuilding unchanged client/CLI artifacts, but evidence must record the changed test variant and current proof binding. Existing root test-only HEAD changes likewise require their full three consumer specs.
7. Aggregate by exact test identity AND required configuration/variant. Latest executed failure overrides any older pass until a later diagnosed repair is verified. OFF/ON knowledge and native/platform variants remain distinct even where titles coincide. Skips never become passes.

## Closing/reporting rule

Once final3 has fully finished, all new failures are diagnosed/repaired with complete impacted-suite passes, all441identities/required variants retain current compatible evidence, full units/static/backend/performance remain applicable, source/runtime cleanup is verified and the report is final, the user goal may be marked complete. No unrequested extra whole-suite invocation is necessary solely because a proven local fixture repair changed test bytes.

The final report must say “441unique cases have passing evidence across explicit configurations and runs,” give the actual failed whole-run counts unchanged, and identify the subsequent complete repaired suites. It must not say “single full gate passed” or rewrite baseline-final3's nonzero exit. This maintains the original scope and evidence standard instead of reducing scope to whichever checks happened to be green.
