# Independent review: autonomous security squad

## Scope
Reviewed the approved task artifacts and the runtime return, SQL, attribution, instruction/role/skill, template-version, bilingual documentation, catalog and scripted-flow changes. Unrelated Git-material, navigation and daemon changes were excluded. No product code was edited by this reviewer; findings were sent to the owning backend implementer.

## Findings addressed during review

1. **P2: refused returns could starve the bounded outbox.** A result signal persisted before reassignment/revocation stayed at the oldest position forever because the target resolver returned no-op without settling it. The fix withdraws only that platform return by making it dispatch-ineligible, excludes withdrawn entries from the scan, and includes an oldest-refused/newer-valid regression with maxPerTick=1. Restoring permissions must not revive the withdrawn obligation.
2. **P2: the new guard overreached into ordinary claim finalization.** FinalizeTaskClaim's existing SQL fence permits deleted planned comments while the response builder records survivors; unconditional GetComment in the new guard made unrelated finalization fail. Missing ordinary IDs are now ignored by this narrow guard. StartTask/checkIssueExecution already has a stricter missing-input rule, which must remain unchanged. Dedicated delegated_result signals still fail closed when their trigger row or pointer is absent.
3. **Failure wording conflicted with required review.** The shared failure prompt allowed skipping work. Security-specific failure content now requires bounded redispatch or explicit blocker/handoff and forbids skipping required review or treating failure as acceptance.
4. **Withdrawn callback replay.** Replaying completion after authority was restored attempted to dispatch the deliberately inert signal. The dispatcher now treats already-withdrawn signals as settled no-ops.

## Final fanout check
The final source publishes suppress_execution=true for both security-return and exhaustion comment-created events. This closes the fanout gap: SubscribePluginEvents forwards comments to plugins unless that event flag is true. Stored dispatch eligibility may remain true solely for internal outbox admission. Subscriber and notification listeners already skip author_type=system, and completion reconciliation routes system progress through the controlled return method. Independently configured task lifecycle subscriptions retain their existing authority.

## Behavior and evidence assessment
- Immutable template keys, real source task/comment lineage, same originator/issue, current assigned squad/leader/roster, runtime ownership/membership and invocation grants scope the return. Old creation version metadata is not rewritten; saved v2 human-gate instructions still require explicit body updates.
- Squad membership race concern was ruled out: Add/Remove/UpdateSquadMember already lock the owning squad row, matching the return guard's squad fence. Principal/invoke locks use the existing transaction pattern.
- The full flow fixture uses actual CreateIssue, claim, task-token comment, completion and status handlers. It asserts agent creator, all-agent roster, code/dependency review batches, retained leader/squad context, inert reviewer comments, zero human comments and final in_review. This is valid plumbing evidence.
- Scripted result text and coordinator decisions do not prove actual vulnerability repair, model compliance, runtime isolation, or correctness of an LLM's evidence judgment. These limitations are explicit in docs and tests.
- No broad capability grant or original-review-comment dispatch relaxation was found.

## Independent verification
- go vet -p 2 on service, handler, attribution and generated DB packages: PASS.
- git diff --check: PASS.
- DB execution: not run by this reviewer; parent/backend own exclusive test permits and final test evidence. Requested real finalization revoke/coalesced tests and concurrency/race coverage in addition to the start guard and sequential multi-reviewer flow.

Final source review has no remaining blocking findings. The owner still needs to complete the recorded integration/race test runs before overall task completion. New tests cover genuine FinalizeTaskClaim refusal for dedicated and coalesced revoked returns (zero tokens), NULL/deleted dedicated signals, withdrawn replay after permission restoration, missing ordinary inputs at finalization, plugin suppression and simultaneous two-reviewer coalescence. Final independent go vet and git diff --check passed again after those fixes.
