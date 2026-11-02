# Commit boundary

The work commit records one coherent repair: make the optional source parent discoverable before the user knows a task identifier, and support the identifier shown by the source catalog.

It includes the 13 verified product/test files listed in `task.json`, the source-picker/spec index/source-editing contracts, and this task's own planning and verification artifacts. The payload is the exact isolated candidate snapshot, not a whole-file copy of overlapping, later worktree edits.

Concurrent source-lock changes in creation-form source/tests and all other unrecognized worktree changes are excluded. The source-lock authoring boundary was confirmed with this task's UI implementer. The live working files remain intact.

After the work commit, archive only this task and record its journal with automatic commits disabled; any explicit bookkeeping commit must also use the repository's Lore trailers. No remote publication is part of this task.
