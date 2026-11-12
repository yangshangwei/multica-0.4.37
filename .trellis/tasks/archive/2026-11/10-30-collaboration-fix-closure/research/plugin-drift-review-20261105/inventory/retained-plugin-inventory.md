# Research: Retained plugin inventories and endpoint hash gap

- Query: Does a retained per-file inventory reconstruct the exact prelaunch or postrun plugin tree, and can the changed files be proved finite?
- Scope: internal task/private evidence metadata; one bounded three-file hash/schema read; no full live plugin tree scan, user configuration contents or conversations emitted.
- Date: 2026-11-05

## Findings

**No matching per-file endpoint inventory was found in the inspected task/runtime-private evidence. The retained61947-row inventory cannot independently reproduce even its own stored tree hash under the launcher's exact algorithm. Therefore it cannot prove that only the three mtime candidates changed.** This is a limitation of retained evidence, not proof that recovery from an external backup is impossible.

| Evidence | Exact SHA-256 |
|---|---|
| Registered prelaunch plugin tree | `617d9c5d6b208b7b1b44aa4ac5bff262a738b926b70886f16dda143873050d8c` |
| Observed postrun plugin tree | `91deb2c67550be5d1d1b366dde25543a78fec36ec719e7d9adcd29f3e3d87660` |
| Older inventory's stored tree hash | `b9942cf349f7109090dd3c01e0b1724fed795733faae080bb0a42d0637013ac0` |
| Reconstruction treating all61947 rows as regular files | `6064f16dd5fd249e3ff6fb3d5d19407315c88e1e7bdc0da8a40f148ae54d2a52` |
| Older inventory file itself | `71fa208010b90575659838757c3683ba2da1f53ba357ca97b41ca47d92b3bd61` |

The older inventory timestamp is `2026-11-04T17:23:33.076044+00:00`; the later E0a trial began on November5 around03:42UTC. The earlier recheck reports zero content changes/removals between17:23 and17:27 only. It cannot fill the intervening interval or establish the prelaunch fingerprint. Existing raw launcher/public behavior pass and outer helper configuration failure must both remain unchanged.

### Exact algorithm and missing fields

`research/real-model-rk1/run_rk1.py:76` walks with default `followlinks=False`, emits normal files and both file/directory symlinks, then sorts complete record strings and SHA-256 hashes their UTF-8/surrogateescape encoding:

- Regular file: `relative_path + NUL + "file" + NUL + sha256(file_bytes) + LF`.
- Symlink: `relative_path + NUL + "link" + NUL + os.readlink(path) + LF`.
- OSError reading a file: `relative_path + NUL + "unreadable" + NUL + LF`.

Ordinary directories have no record; symlink directories have a record and are not followed. Mtime, bytes, mode and the root's own timestamp are not hashed.

The retained rows contain **only path, bytes, mtime_ns and sha256**. Every row has a64-character lowercase hexadecimal digest and paths are unique. There is no recorded kind, link target or unreadable marker. Reconstructing the natural all-regular-file representation yields6064…2a52, which differs from storedb994…3ac0 and both endpoint hashes. This shows the available fields do not validate the recorded tree hash under this algorithm. It does **not** identify whether missing symlink metadata, enumeration timing or a different inventory-generation serialization caused the discrepancy; that cause was not retained in the inspected artifacts.

No additional per-file plugin snapshot was found by filename search across the runtime task's continuation evidence and `/Volumes/artisan/code/2026/mc-runtime-acceptance-20261104/.omx/private/`. Aggregate operator facts, intended-environment facts and registration contain hashes only. Captures under runtime-real-20261105 are two Multica binaries, not plugin snapshots. A parallel `plugin-forensic/` directory appeared later; root owns any newly produced evidence there.

### What the three metadata candidates prove

The stored mtime-only observation names:

- `known_marketplaces.json`
- `plugin-directory-cache-v2.json`
- `store/cc-plugin-agents-md_builtin-b049b47b3618.json`

A bounded current hash read confirms all three differ from their entries in the **older unmatched** inventory; their current sizes/mtimes agree with the recorded postrun size/time values. Exact old/current file hashes are in `inventory-findings.json`. This is evidence of change over the longer older-inventory→current interval. It is not an exact617d→91de file delta, proof that only these files changed, proof of unchanged deleted entries, or writer attribution. Matching mtime/size cannot substitute for a captured historical content digest.

The current metadata shapes also do not repair the gap: known_marketplaces holds marketplace records; plugin-directory-cache-v2 has schemaVersion/fetchedAt/listings/entryNames/entryNameClaims; the store entry has a usage-rows-queued string. No complete historical file/type/link-target inventory is present in these inspected structures. They may suggest cache/bookkeeping activity, but cannot establish harmlessness without the exact before bytes and complete endpoint binding.

### Minimal missing proof

To prove an exact finite changed-file set, recover a prelaunch inventory or snapshot whose complete path/type/content-or-target entries recompute **617d…0d8c**, plus a postrun counterpart recomputing **91de…7660**. To judge changes harmless, retain and compare the differing contents as well. A SHA-256 root alone cannot reconstruct those entries. A new current tree scan cannot retroactively supply the missing prelaunch snapshot. No exception to the original equality rule follows from the available metadata.

## Files found

- `inventory-findings.json` — exact algorithm, available hashes, old-row reconstruction, finite current hash observations, artifact hashes and explicit limits.
- Runtime private `operator-inventory-initial.json` —61947 older per-file digest/stat rows, lacking kinds and link targets.
- Runtime private `post-run-binding-observation.json` — actual pre/post aggregate plugin hash pair; other reported operator keys unchanged.
- Runtime private `plugin-post-run-mtime.json` — three path/stat candidates, explicitly mtime-only.
- Runtime `real-entry/e0a-independent-review.md` and JSON — one actual E0a behavior/lifecycle pass, outer helper failure and held successor execution.
- Runtime `real-model-rk1/run_rk1.py:76` — canonical tree-hash implementation.

## Related specs / external references

The runtime registration's strict operator-configuration equality is the evidence contract. `.trellis/spec/server/collaboration-context-experiments.md` requires retaining failures and separating environment/provenance evidence from behavior. No external reference was needed.

## Caveats / Not Found

No tests, Provider/version/Go commands, DB/services or configuration changes were made. Raw plugin/config/conversation contents were not copied into output. Search is bounded to the named evidence roots; it does not claim all backups on the machine were searched. The current configuration gap remains unresolved by this retained inventory; any later disposition belongs to root/runtime owner.
