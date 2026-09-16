# V0.4.46 verification and release design

Use the existing release and offline-upgrade pipeline. Do not introduce new dependencies or change business behavior without a reproduced defect. Test the current main checkout plus all changes since v0.4.45; preserve the two pre-existing untracked GitLab planning documents.

## Isolation and evidence

- Use a dedicated local verification database and distinct API / production Web ports. Existing interactive desktop sessions must remain usable.
- Run actual production Next.js build and server for full Playwright business tests; test the real Go API and PostgreSQL database, not mocked business responses.
- Complement the full suite with recent-change regressions and a desktop smoke test where existing tests require a desktop renderer.
- Keep logs, screenshots, test reports, exact commands, artifact manifests, hashes, and release API results in `.omx/reports/release-v0.4.46/`.

## Packaging and publication

- Preserve current source-of-truth version and changelog conventions; use v0.4.46 as explicitly requested.
- Use Linux amd64 offline image bundles and the documented upgrade/installer wrapper, including the matching deployment changelog.
- Confirm the Windows architecture from artifact machine headers and native install smoke. The optional clarification was unanswered; the literal x86 request is fulfilled with genuine ia32, alongside the previous x64 delivery. Windows-only ia32 support uses a 386 CLI and a separate update channel.
- Run source gates before commit/push. Run the new Windows packaging/install lane on the pushed main candidate before creating the immutable release tag. Rebuild the final tagged artifacts, and verify the upgrade and published asset hashes before completion.
- Use normal, non-force pushes. Do not rewrite or replace an existing release tag.

## Failure handling

Reproduce failures and repair the smallest responsible source/configuration area. Add a regression test for behavior defects. Re-run affected gates when code changes. Retain existing release artifacts and use isolated test installations to exercise upgrade / rollback assumptions.
