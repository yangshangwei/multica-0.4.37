# Criteria review: operator drift and continuation

**Recommendation:** retain E0a as a valid, measured `finished/pass` under the
original RK1 criteria, retain the helper's failed post-run integrity check, and
resolve the continuation hold explicitly. H1 may use the existing budget after
a prospective full operator rebind, reviewed evidence procedure and new window.
Do not claim E0a's operator tree was unchanged, and do not rerun E0a. Reconstruction
of the old plugin tree is separate forensic work, not a newly required condition
for the original E0a behavior criterion.

## What the rules actually require

| Layer | Contract | Consequence |
| --- | --- | --- |
| PRD AC10 (`prd.md:82`) | Record HEAD, command, environment, actual tests and exit; unrun work and environment faults are not passes. | Preserve all observed outcomes and limitations. It does not require every operator cache file to remain immutable. |
| Design (`design.md:76–84`) | Freeze and bind the application's admitted inputs, source revisions and actual prompt to each launch. | This concerns supplied collaboration material. It is not a claim that the operator's entire Claude directory is immutable. |
| Original launcher (`run_rk1.py:299–314`) | Refuse unless product, framework, launcher, Provider bytes and full current `operator_facts()` match registration before the invocation. | Keep this exact admission equality, including plugin/cache directories and missing-value fingerprints. |
| Original invocation record (`run_rk1.py:334–340`) | Save the operator facts used at admission. | E0a has those facts; never replace them with the later tree. |
| Original post-run accounting (`run_rk1.py:364–379`) | Inspect requested public results, original executor exit, incomplete/canary status and public/private launch counts. | No post-run operator recomputation or whole-tree-equality assertion exists here. E0a is complete and the launcher stop state is false. |
| Added E0a helper (`e0a_entry.py:113,249–278`) | Reuse `binding()` after execution, require operator equality again, and require before/after bindings to agree for `ready_for_independent_review`. | This added a stricter post-run integrity claim. The real `exit1/review_ready=false` is correct for that helper and remains unchanged. |

The launcher reviewed here is the frozen, unchanged SHA-256
`3bc7962c996143200b1834e9246ec215e8a0d3e03fc38e832abd2eddb1769c99`.
It checks operator configuration once per scenario invocation before Go runs,
not after the scenario or between every physical launch within that invocation.
Do not describe this as continuous configuration attestation.

`framework.md:53` promises fingerprints of the operator files and content trees.
The later preparation guide says, at lines61–68, that changed configuration must
be recorded/reviewed before the next launch. Neither text claims an original
E0a scoring rule of post-run whole-plugin equality. The helper README also
expressly separates recorder integrity from the model's scenario verdict.

## Resolve the current hold without rewriting the experiment

1. Add a dated disposition linked to the immutable E0a public/invocation,
   helper result,66-check evidence review and forensic findings. State both:
   **original measured smoke behavior/lifecycle passed; added immutable-operator
   integrity claim was not established**. Keep `full_binding_accepted=false` for
   that historical claim and keep all raw exits and hashes unchanged.
2. State why this distinction addresses the original stop rule. Its text is
   “Stop every later trial and scenario after the first incomplete
   runtime/environment/harness result” (`registration.json:91`). The stop was
   appropriately honored for review. The original E0a invocation has no missing
   requested result, no runtime/Provider failure and no canary failure; its
   required prelaunch environment evidence exists. A drifted auxiliary post-run
   assertion alone does not establish an incomplete original sample. This
   disposition lifts the auxiliary hold for continuation; it does not erase the
   failed assertion or assert that the drift was harmless.
3. If forensics instead establishes a behavior-affecting uncontrolled change
   that invalidates the original environment or exposure evidence, do not use
   this distinction to bypass the original stop rule. Preserve the discovered
   failure and resolve that concrete finding before new calls. This review does
   not prejudge the independent forensic result.
4. Preserve the original E0a slot and consumption1/25, leaving24. The next
   registered entry is H1 trial1. Preserve its three trials, fake seed/real
   accounting, original sequence, scoring, timeout and strict incomplete/canary
   stop. Do not spend another call obtaining replacement E0a evidence.
5. Before enabling H1, record/review/commit the actual current full operator
   facts in a new registration revision with its prior revision/hash linked.
   Keep the frozen product,17-file framework, launcher and Provider/model
   choices unchanged. Run the unchanged preflight against that revision in a
   new coordinated window. A new configuration revision must be visible in
   reporting; it cannot be presented as identical to E0a's operator state.

This is not automatic admission merely because `run_rk1.py` would continue. The
current root hold must be dispositioned in writing, and actual source, build,
database, quota and window facts remain required. The E0a helper is single-use
and hard-coded to E0a; retain it as evidence. Any small H1 recorder adaptation
must be reviewed prospectively rather than editing the consumed helper/attempt.

## Minimal prospective evidence change

Use one private per-file before/after manifest for the existing operator-facts
scope, with relative path, kind, link target where applicable and content hash.
The pre-invocation manifest must reproduce the **full** registered tree digest
and be bound to the actual intended environment. Keep relevant before bytes
privately when a later semantic comparison requires them. After the unchanged
invocation exits, capture the corresponding manifest and save the exact delta.
Do not hash repeatedly during model execution or alter the user's configuration.

Separate facts in the new recorder: original invocation/scoring/accounting;
product/framework/tool/actual-CLI integrity; operator-before/after observations;
and the independent delta assessment. A mismatched operator tree should save
the completed independent facts and then hold subsequent invocations for review,
instead of throwing before those facts are recorded. This prospective structure
must not turn the historical E0a helper result green.

Assess the actual changed values and references, not directory names. Enabled
plugin selection/version, resolved cache paths, plugin executable code, hooks,
agents/instructions, configuration selectors and output/routing settings can
affect behavior. “cache” or “marketplace” is not itself a harmless classification.
For changes established to be maintenance-only, record the supporting content
comparison and preserve the full digest change. For unresolved or behavior-
relevant changes, keep later admission held until disposition; never weaken the
original hash function or omit paths to obtain equality. A before/after manifest
does not by itself prove which intermediate bytes the Provider loaded.

If a helper revision is used, validate these reporting branches with local fake
artifacts: unchanged tree; a known maintenance delta; an unresolved/executable
delta; and missing before evidence. No real model probe is needed for that
validation. Do not alter the original17-file framework or its model scoring to
test this recorder behavior.

## Authorization and limits

No read rule requires a new user confirmation for this bounded technical
disposition, evidence capture or next pre-registered H1 preparation. The user
has requested continuation; the original25-call allowance remains. Root owns
the explicit disposition and final readiness decision. A materially different
proposal—dropping the original prelaunch equality, ignoring a proven original
runtime/environment/harness failure, changing model/scenarios or exceeding the
budget—would require a separate decision; this recommendation does none of those.

Only this review document and its JSON were written. No forensic backup analysis,
tests, database/model calls, user-configuration edits or original evidence changes
were performed.
