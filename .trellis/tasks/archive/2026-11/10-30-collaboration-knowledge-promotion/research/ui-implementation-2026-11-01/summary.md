# B2 shared UI implementation

Ready for main's independent and production-browser review. Implemented source candidate lists and all-version paging, live full-detail review, exact human approval/rejection/withdrawal, blank immutable correction/derived proposals, original-request recovery, three source pickers, and separate managed-contribution sections in shared project/workspace surfaces. All copy is present in en/zh-Hans.

Core resolves selected memory/comment versions and exact run records through existing lawful readers, preserving their original identity when latest advances. SHA-256 uses exact canonical text. Source discovery is fenced by scope/access, cancellation and the existing ten-second request deadline. There is no new source endpoint or arbitrary page-count cutoff.

The implementation reuses the existing controls, navigation, query policies, source readers and operation helpers. It adds no business store, server-body persistence, provider, route, dependency, asset generation or base-context merge. Blank correction forms avoid retaining copies of server body through purge/expiry. A selected source project is pinned by ID and must be explicitly reselected if its relationship changes.

Verification at the exact hashes in `source-manifest.json`:

- **5 Core tests passed** in one Node suite: exact historical memory hash, retained comment revision/tombstone, run-record pages, purge/foreign scope/late authority, and finite deadline without a page-count cutoff.
- **122 shared View tests passed** across six suites, including 11 knowledge interaction cases plus surrounding collaboration/project/workspace composition and locale parity. Zero failures/skips in either run.
- Full Core and Views TypeScript checks passed.
- Scoped Core and Views ESLint passed with no errors. Two unchanged project-detail dependency warnings remain; its existing deletion test also emits NavigationProvider act warnings. Both are recorded in the logs.
- `git diff --check -- packages/core packages/views` passed.

Exact commands, outcomes, current HEAD and test inventories are in `verification.json`, `*-command.json`, `*-vitest.json`, `test-inventory.json` and logs. The manifest includes all changed/new Core and Views files. `integration-notes.md` describes entry points, source contracts, privacy/recovery behavior and the main-session handoff.

Main still owns the single Impeccable detector pass, independent screenshot finish review, and production Web/native Desktop real-HTTP E2E setup. Those checks, backend authorization/fanout integration and G2/model behavior are not claimed by these component tests. No commits were made by this lane.
