# Frozen group dependencies, M08 and M11

The authorized continuation retains the existing task, execution identity model,
FIFO, proof barriers and disabled public squad entry. This lane owns service and
runtime group foundations; the main session owns generation, route/daemon wiring,
native capacity evidence and the final rollout decision.

## Smallest behavior gaps and sequence

1. Promotion currently stores only the effective run's grant, while admission
   rereads other input targets' current authorization. Persist each input's exact
   target/configuration and full grant set. Preserve author generations on the
   immutable command. Validate that frozen set before each claim, permit and
   business call. Add real-service PostgreSQL regressions for replacement grants,
   resource drift, separate authors and retained proof after revocation.
2. Claim currently recomputes its prompt. Persist the exact run input snapshot
   once and return it on replay. This is the prerequisite for M11's consumed
   boundary, not evidence that unread joins already work.
3. Add comment/member delegation identity and independently admitted member and
   continuation runs. At most one unclaimed leader continuation may be queued.
   A member reply or completion coalesces into it. The leader ends its turn;
   every run needs its own resource reservation, permit, nonce and proof.
4. Permit M11 joins only into a live, unsealed squad group without an earlier
   eligible waiting input. Append per-command dependencies, consume at claim,
   detach unread input on cancellation/revocation/stop, and stop the group for
   invalid already-consumed dependencies. Normal settlement uses the last
   leader result and atomically proves that no live/queued work remains.

## File and data boundaries

- `collaboration_runtime*.go`, `collaboration_groups.go`,
  `collaboration_batches.go`: promote/admission, exact prompt, dispatch and
  settlement, reusing the existing lock and proof helpers.
- Dedicated dependency and business-comment services and SQL: no ordinary task
  fallback, no new CLI command or dependency. New migrations start at 717.
- `collaboration_group_dependency` records per-command target identity and grant
  snapshots. This provenance permits removing only an unread join's dependency;
  historical identities must remain recoverable.
- Atomic Switch is separately owned and must continue rejecting live or
  proof-unconfirmed historical consumers. The lane never edits authority.go.
- The public squad capability stays closed until the main session has a real
  capacity-one leader/member/continuation execution through the router and Linux
  supervisor. Synthetic group rows do not constitute that evidence.

## Verification

Use the dedicated `collaboration_squad` PostgreSQL database and Go 1.26.6 through
the repository CLI guard. Add focused failing behavioral regressions, then run
runtime/control/author invalidation regressions and vet. Main owns full checks,
index recovery registration, sqlc consistency and native process verification.

No task status, specs, commits or unrelated files are owned by this lane. New
forward migrations explicitly reject rollback to the obsolete execution model.
