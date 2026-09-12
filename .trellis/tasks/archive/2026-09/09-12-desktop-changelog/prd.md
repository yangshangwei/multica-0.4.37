# Desktop changelog and automatic release notes

## Goal

Let Multica desktop users open "变更说明" from the left-hand Help menu, read an understandable history of their deployment, and discover newly published changes without reinstalling or restarting the client. Generate future release notes from committed changes as part of the release process.

The user requested requirements, technical design, task decomposition, Trellis tracking, implementation, and verified acceptance. This PRD describes the required behavior; [design.md](./design.md) and [implement.md](./implement.md) define the solution and execution.

## Reference and current product facts

- The official reference is <https://multica.ai/changelog>, inspected on 2026-09-12. Its useful reading structure is a chronological timeline, month navigation, version/date headings, descriptive titles, and categorized updates.
- This checkout is a privately deployed fork with intranet users. Its clients must remain usable when the public Multica website and GitHub are unreachable.
- The fork has no observed published GitHub Releases. Its reachable local baseline is v0.4.37; the official v0.4.37 entry is dated 2026-08-31. Later imported upstream tags are not evidence of releases of this fork.
- Already committed fork changes and current uncommitted work are different evidence. Neither may be presented as a completed fork release without publication.

## Requirements

### R1. An in-app Help destination

- Help contains an item named "变更说明" in Simplified Chinese and a localized equivalent in the other supported languages.
- Selecting it in a workspace opens an ordinary desktop tab with a recognizable title and icon. Back/forward navigation and reopening the tab work like the existing documentation reader.
- The shared Help menu has a working web destination as well. Opening Help before a workspace is selected does not crash or show a broken destination.
- The existing desktop update notification's release-notes action opens the in-app reader for that version. Its restart action continues to work; receiving an update before the workspace shell mounts does not lose the notification.

### R2. Readable release history

- The page shows release version, date when known, descriptive title, publication status, source, and grouped plain-text changes.
- Readers can navigate release/month entries, scroll a long history, and open a specific release. An unknown requested version leaves the history readable and explains that the requested entry is unavailable.
- The layout follows the official reference's reading structure while fitting the existing app shell, typography, light/dark themes, and desktop widths. Long titles and change descriptions wrap without covering controls.
- Headings, navigation, refresh, and status messages work with keyboard and assistive technology. The page has complete English, Simplified Chinese, Japanese, and Korean UI translations.

### R3. Honest source and publication labels

- Initial content includes an attributed summary of official Multica v0.4.37 and meaningful committed fork changes, visibly marked "未发布" until a release exists.
- Upstream reference entries, fork releases, prereleases, and unpublished fork work remain distinguishable even when version labels match.
- Official v0.4.38–v0.4.43 must not be advertised as installed or published by this fork merely because they appear on the official website or as imported tags.
- Uncommitted changes, task bookkeeping, and test-only commits must not become published product updates. Historical committed user-facing improvements remain eligible even when their commit messages predate the chosen release-note convention.

### R4. Distinct version meanings

- When available, the page separately identifies the installed desktop version and the running server version.
- The newest published stable fork release is identified from release records. An upstream entry, prerelease, or preview cannot be mistaken for that stable release.
- Missing version information or an absence of published fork releases is expressed honestly; the UI does not infer publication from a package version or git tag.
- Reading release notes works when automatic binary updates are disabled and does not imply that a matching desktop installer exists.

### R5. Fresh content for new and existing clients

- A fresh client with this feature reads the latest content available from its configured deployment on first opening the reader.
- An already-open, visible reader discovers a successfully published feed update within 60 seconds plus request completion, without a client reload, reinstall, or server restart.
- Opening/reopening the reader, returning focus, reconnecting, activating its desktop tab, and manually refreshing also request current content.
- Hidden windows/inactive tabs do not require continuous polling. Returning to the reader restores freshness. No always-on Help notification badge is required.
- Public internet access and binary-updater preferences do not determine release-note freshness.

### R6. Useful degraded states

- Initial loading, an empty valid history, an unavailable/older server, and a failed first request are distinct readable states with an appropriate retry/refresh action.
- A later refresh failure preserves previously loaded releases and visibly states that refreshing failed or content may be stale.
- If a deployment feed is missing or invalid, existing valid content remains available and the UI does not claim that it is current.
- Unknown additive fields or enum values from a newer server do not crash the reader. An unknown publication status cannot create a false "latest release" badge.
- Release text is rendered as text; embedded HTML or script-like commit subjects never execute.

### R7. Commit-driven generation

- A release operation generates categorized Markdown notes and a structured client history from the same immutable committed range.
- The base is an explicit valid ancestor or the previous reachable published fork release. An unrelated higher version tag cannot change the selected range.
- The first fork release supports an explicit baseline. Merged feature work is included; bookkeeping can be omitted and release-note wording/category can be overridden through documented commit trailers.
- Invalid history, conflicting version identity, an invalid range, or malformed overrides stop generation with an actionable error rather than produce misleading release notes.

### R8. Durable cumulative history

- A new publication retains every earlier published entry and its provenance, including stable releases, prereleases, and attributed upstream history.
- Publishing a fork release replaces the relevant unpublished preview without promoting unrelated previews or erasing history.
- Repeating the same release operation is idempotent. A version already associated with a different immutable commit is rejected.
- Both connected CI and offline release operations can carry the previously published history into the next release. A failed history download cannot silently reset history to the initial seed.

### R9. Publication reaches the reader

- After the required release checks and artifact builds succeed, the fork's release process publishes the exact generated notes/history to the fork's release, using fork-owned authorization.
- Failed prerequisites prevent release-note publication. The fork flow does not require access to upstream's Homebrew repository.
- The same history can be included in server builds and delivered through the intranet release/deployment procedure to a running server without rebuilding it. The delivery operation validates the complete file and replaces it atomically.
- A failed intranet publication leaves the previous valid feed intact. First installation and a running client see the same successfully published history.

### R10. Reviewable completion

- Trellis contains this PRD, a concrete technical design, execution tasks, research references, and acceptance evidence.
- Acceptance covers the real Help entry, version navigation, generation, history carry-forward, publication failure guards, and a feed update appearing in an already-mounted reader.
- Required lint, type checks, tests, and static checks are executed; results and any environmental limits are recorded accurately.

## Acceptance checklist

- [x] A1 — Help opens the desktop reader; tab title/icon, shared web route, no-workspace behavior, and update-notification navigation satisfy R1.
- [x] A2 — The rendered timeline, month/release navigation, localization, keyboard use, narrow layout, and long text satisfy R2.
- [x] A3 — Initial official/fork content is traceable to its sources, with unpublished fork labels and no invented release or installed-version claims (R3–R4).
- [x] A4 — With public internet unavailable, a fresh session sees the current deployment history; a running visible reader observes an atomically replaced feed within the freshness bound (R5, R9).
- [x] A5 — First-load failure, 404, malformed data, empty history, retained cached content, unknown enums, and configured-source failure produce the specified degraded behavior (R6).
- [x] A6 — Temporary-repository tests prove ancestry-aware generation, committed-only content, trailer handling, history retention, stable/prerelease distinction, and deterministic reruns (R7–R8).
- [x] A7 — Release/publication fixtures prove exact artifact reuse, fork ownership, success prerequisites, and failure without history reset or file corruption (R9).
- [x] A8 — Trellis records completed tasks and actual verification evidence for all acceptance items (R10).

## Scope boundaries

This task delivers the working feature and release/publication mechanism. Creating a real production tag, remotely publishing a new production release, changing the binary-updater transport, adding a CMS, mobile UI, or fetching public release data at client runtime is outside this task. Clients built before this feature need one normal client upgrade to acquire the new Help page; later content updates require no such upgrade.
