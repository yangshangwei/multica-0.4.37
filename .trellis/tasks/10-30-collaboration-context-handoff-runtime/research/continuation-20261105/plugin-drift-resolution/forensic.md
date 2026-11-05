# E0a plugin-tree drift: bounded forensic result

The exact prelaunch `617d9c5d…050d8c` tree could not be reconstructed from the
retained material. The original E0a runtime pass and outer-helper failure remain
unchanged. This investigation read local data only; it did not edit operator
configuration, execute a Provider, build, test or access a database.

The old inventory contains61,947 regular-file hashes but omits link identities.
Adding the195 current link targets under the launcher's exact path/kind/content
algorithm reproduces its original `b9942cf3…13ac0` hash exactly. The186 apparent
new paths are links omitted by that regular-file inventory, not evidence of186
new plugin contents. Five regular files have different metadata and one old
regular path is absent. Those observations do not identify when their contents
changed or who wrote them.

The64 bounded combinations of old/current hashes for those five files and the
removed-file state reproduce the postrun `91deb2c6…d87660` tree exactly. None
reproduces the registered prelaunch617 tree. Thus we can reconcile the older
inventory to the observed postrun state, but cannot recover the missing
intermediate prelaunch leaf inventory. Full paths, hashes and candidate maps
are retained privately under `runtime-real-20261105/plugin-forensic/`; the public
[record](forensic.json) retains counts and evidence hashes.

The three files whose mtimes overlap the trial cannot simply be exempted:

- The marketplace registry includes `source`, `installLocation` and
  `lastUpdated`. The installed Provider binary's retained diagnostics refer to
  in-place plugin loading and background reconciliation through this registry;
  it contains more than timestamps.
- The directory cache includes catalog descriptions, release/source data,
  exposed surfaces, entry names and claims. Its exact prelaunch values and
  consumer behavior were not recovered.
- The built-in agents-md store has an opaque `usage-rows-queued` string. The
  installed binary contains that identifier, but this does not prove the field
  is behavior-neutral accounting.

Bounded searches found no old copies of these three files in the plugin tree or
local backup names, and no debug file with a modification time inside the trial
window. The plugin root is not a Git checkout; immutable marketplace sources do
not reconstruct mutable registry/cache/store bytes. These are search limits,
not claims that every possible historical backup was exhausted.

The minimal prospective capture is a private per-file inventory using the same
regular-file/link hashing rule, at actual admission and after the invocation.
Bind both inventories to the aggregate operator hash, retain exact added/removed/
changed path identities and changed operator fields, and independently review
their meaning. Pre-execution equality with the reviewed registration remains
mandatory. Postrun drift must remain an explicit observation and must not hide
the actual trial result, lifecycle failure or budget consumption. No filename
category should be ignored, and no future capture retroactively proves E0a's
missing prelaunch inventory.
