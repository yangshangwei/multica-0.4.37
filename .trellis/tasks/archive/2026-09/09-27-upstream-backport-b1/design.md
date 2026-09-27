# Design

Use a sibling worktree at ../multica-upstream-b1 on sync/upstream-b1. The approved 1fc789556 base advanced only by the user's previously observed selfhost-config.test.sh fix; preserve that committed fix by starting at 7845de31e6bc7d021bba6005bccdab44fc9958e6.

The original checkout stays untouched until the authorized final local merge. No remote push or release. CLI labels and comment editing are client adapters over existing APIs; no server mutation contracts change. Cache rate reuses shared view utilities. Runtime-generated guidance and built-in skill docs must describe the new CLI accurately using the current split skills; do not restore the absent multica-platform directory.

One executor owns CLI files/docs and completes labels before comments; another owns the six cache rate files. The leader owns metadata, generated Chinese docs, ledger, fresh verification and commits. Tasks may prepare independent diffs in parallel, but only explicit path lists are staged. The leader asks the CLI executor to pause between feature commits.

The two-language boundary and source template content are unchanged. Existing backend permission and conditional revision checks remain the authority. Upstream helper/test intent is retained while adapting stale test/document context only where required.
