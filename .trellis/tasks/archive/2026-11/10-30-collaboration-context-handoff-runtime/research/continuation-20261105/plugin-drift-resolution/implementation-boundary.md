# Prospective runtime evidence boundary

E0a's original frozen launcher records admission-time operator facts and scores
the actual runtime evidence. It does not impose post-run plugin-tree equality.
The later E0a helper added that stronger integrity check. Its failure remains
unchanged; E0a's registered result and budget accounting also remain unchanged.

The missing observation is a per-file before/after inventory. A coarse tree
hash cannot identify which paths changed. Add this evidence to a successor
task-private helper, preserving the original launcher and seventeen test files.

## Allowed changes

- A successor to the reviewed E0a helper accepts one registered scenario at a
  time, obtains its frozen trial/count information from the runtime addendum,
  and calls the original launcher at its next unattempted trial.
- Full current operator equality remains mandatory before build/check/run.
  Capture private per-file plugin manifests at admission and afterward using
  the same path/kind/content hashing rule, and retain both aggregate hashes.
- Post-run evidence collection must continue when operator fingerprints differ:
  record exact changed fields and per-file changes separately from immutable
  source/framework/tool/CLI binding. Do not infer harmlessness from a filename.
- Reconcile every actual requested public result, physical launch and captured
  per-run CLI; include fake seed CLI copies without counting them as paid calls.
- Any actual launcher/runtime/canary/incomplete failure still stops the series.
  A drift observation requires root review before any next scenario; no automatic
  follow-up or silent rebaseline. Preserve all original evidence files.
- Credential-free controls must prove admission still rejects drift, post-run
  drift does not hide failed outcomes, inventory changes are exact, and scenario
  accounting respects H1's three calls and H6's paired launches.

## Explicit exclusions

No product change, no original launcher or framework change, no operator config
edit, no ignored plugin/cache category, no model/prompt/scoring/budget alteration,
no E0a replacement, no paid call or database work during preparation.

Root must publish the criteria disposition and independently review the successor
helper before re-enabling registration. Future runs still require a new explicit
shared window and exact committed admission facts.
