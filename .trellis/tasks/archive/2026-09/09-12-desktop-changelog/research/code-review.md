# Backend and reader implementation review

Date: 2026-09-13. Reviewer: independent Trellis check sub-agent. Initial review covered the working-tree implementation on top of `01353260b`; closeout inspected the corrected working tree on top of `d96d40127`. Release-tool implementation and unrelated changes were excluded. No application source was edited by this reviewer.

**Final disposition: APPROVE for the bounded backend, reader, and deployment-configuration scope.** Both P2 navigation findings and the P3 documentation finding below are closed. No unresolved defect remains from this review. Runtime acceptance and the full repository pipeline remain the leader's verification responsibility.

## Closeout verification

- `packages/views/changelog/changelog-page.tsx:45–68` and `:79–92` reuse the existing per-tab view-state channel to record a consumed hash/version request. A remount restores the saved reading position without replaying that request; a new request still lands. The permanent tests exercise both retained hash and updater-version URLs across unmount/remount.
- Explicit navigator click and Enter activation call the jump function even when the selected URL is unchanged (`:183–188`). Its geometry adjustment updates only the reader's `scrollTop` and focuses with `preventScroll`, removing the native `scrollIntoView` ancestor risk. Modifier clicks retain the existing new-tab behavior. Permanent tests assert the repeated jump, focus, unchanged shell scroll position, and no replay after refresh.
- `packages/core/changelog/selection.ts:3–41` now selects the greatest valid numeric `vX.Y.Z` among published entries from the single fork, independent of publication time. Component comparison uses `bigint`, so multi-digit and large numeric versions sort correctly. Unknown status, prerelease/development/build suffixes, malformed versions, and other sources cannot become the latest stable release; timeline ordering remains chronological.
- `packages/core/diagnostics/diagnostic-context.ts:86` registers the changelog workspace route in the existing diagnostic bucket table. The canonical route-builder parity test passes.
- `SELF_HOSTING.md:374–375` now distinguishes public Google sign-in from deployment-served Help documentation and changelog pages. The contradictory offline limitation is removed.

Executed during closeout, all with exit 0:

| Working directory | Command | Result |
| --- | --- | --- |
| `packages/core` | `pnpm exec vitest run changelog` | 3 files, 41 tests passed |
| `packages/views` | `pnpm exec vitest run changelog/changelog-page.test.tsx` | 1 file, 16 tests passed |
| `packages/core` | `pnpm exec vitest run diagnostics/diagnostic-context.test.ts` | 1 file, 15 tests passed |

These 72 focused tests and the source inspection close the findings below. The initial Go race test remains the backend execution evidence; it was not redundantly rerun for these client/documentation corrections.

## Closed findings from the initial review

Locations and reproduction descriptions in this section refer to the initial implementation, before the corrections summarized above.

### CLOSED — P2 — Reopening a release deep link overrides the saved reading position

Location: `packages/views/changelog/changelog-page.tsx:62–67`.

Open a release using the left navigation or an updater version link, scroll farther down its history, switch desktop tabs, and return. `TabContent` remounts the active host on every tab switch (`apps/desktop/src/renderer/src/components/tab-content.tsx:18–21`, `:59–61`). `useRestoredScrollRef("changelog")` restores the saved offset during ref attachment, but the selected-release effect subsequently always calls `scrollIntoView` and focuses the original release heading. The retained hash/version is treated as a fresh navigation even though the user is resuming a tab, so the reading position is lost.

The existing restoration test uses no hash/version and cannot expose this interaction. A temporary test injected into the existing component suite supplied a retained release hash and a saved scroll entry. It failed because `scrollIntoView` was called once after restoration when no new selection occurred. This proves the unwanted anchor replay; physical scrolling in Electron was left to the leader's live acceptance.

Recommendation: remember that a particular release deep link has already landed through the existing per-tab view-state restoration channel, or otherwise distinguish restored mounts from fresh selection requests. Preserve initial deep-link navigation and later changes to the selected release. Add a regression that switches away and remounts with a selected hash plus a saved offset.

### CLOSED — P2 — The currently selected release link stops acting as a jump target

Locations: `packages/views/changelog/changelog-page.tsx:62–67` and `:158–165`.

After selecting a release, scroll away and click the same release in the navigator. The link only pushes the same URL, while the scrolling effect depends only on `selectedId` and `active`. Neither changes. On desktop, the navigation adapter explicitly returns when `active.url === path` (`apps/desktop/src/renderer/src/platform/navigation.tsx:197`), and `AppLink` intercepts the browser's default anchor behavior, so nothing scrolls or focuses the requested release.

A second temporary component test started with the selected hash, cleared the initial scroll spy, manually changed the main container's scroll position, and clicked that same release link. The navigation push was observed, but the expected scroll call count was zero rather than one.

Recommendation: let an explicit navigator activation scroll/focus its target even when selection identity is unchanged, while keeping polling and restored mounts from replaying navigation. Preserve modified-click/new-tab behavior.

### CLOSED — P3 — Offline guidance still says Help changelog links require the public website

Location: `SELF_HOSTING.md:374`.

The offline limitations list says that docs/changelog links in Help point at `multica.ai`. The new shared Help destination and the new deployment-history section contradict this statement. Remove the obsolete docs/changelog limitation while retaining the Google sign-in limitation.

## Initial review evidence and coverage

- Executed `go test -race ./internal/changelog -count=1` from `server/`: **PASS**, package runtime 1.832 seconds. The tests cover bounded whole-file validation, embedded/file fallback, retained last-valid content, atomic replacement, instance isolation, response immutability, and concurrent reads.
- Executed a **temporary Vite transform** over the existing `packages/views/changelog/changelog-page.test.tsx`, adding only the two reproducer tests below in memory. No repository test/source files were edited. Vitest reported **2 failed, 10 skipped**, exit 1; failures were the expected behavior assertions described above.
- Inspected the handler and real-router auth/CORS tests. The route is inside `middleware.Auth`, has no workspace requirement or client-selected source, and uses the existing ETag matcher. Those handler/router tests were not rerun in this bounded pass.
- Inspected core parsing, Query observer tests, selection tests, Help tests, updater bridge tests, all four changelog locale resources and registrations, tab title/icon/path registrations, and the shared web wrapper. The query captures its API instance, keys data by deployment, overrides global stale/focus settings, retains successful data on thrown parse/network errors, and has finite inherited GC. Actual desktop panes unmount when inactive; reactivation therefore also exercises the explicit mount refresh policy.
- Inspected Go validation and snapshot locking. Fork identities are repository-qualified and mixed forks are rejected; the complete read/validate/snapshot operation is serialized. Complete serialized response metadata participates in the ETag, including stale and warning transitions. The concurrency test exercises concurrent replacement but does not deliberately force a delayed-old-read ordering; the enclosing lock is the implementation evidence for that ordering invariant.
- Inspected update-notification placement. Event state remains at App scope outside the conditional shell; only the navigation callback is bridged under the navigation/workspace providers. With no workspace the notes action is disabled with an explanation and restart remains available.
- Inspected Compose, Helm, root Dockerfile, and the associated environment/deployment instructions. The new runtime mounts cover directories rather than individual files or `subPath`; empty file configuration retains embedded mode, and the optional Helm claim is guarded independently of uploads. Actual container/PVC mounting was not executed here.
- The initial seed labels committed fork work as unreleased and keeps official v0.4.37 separately attributed. No later upstream version or local package version is promoted to a published fork release by the reader.

Historical temporary regression bodies appended to the existing component test module before the fixes. The permanent geometry and memento tests described above supersede these probes; their original failures do not describe the current implementation:

```tsx
it("review repro: restored position survives a retained selection", async () => {
  navigation.hash = `#${encodeURIComponent(fixture.releases[1]!.id)}`;
  render(
    <ScrollRestorationProvider adapter={{ get: () => ({ top: 120, height: 500 }) }}>
      {page()}
    </ScrollRestorationProvider>,
  );
  await screen.findByRole("heading", { name: "Official baseline" });
  expect(Element.prototype.scrollIntoView).not.toHaveBeenCalled();
});

it("review repro: current release link jumps again after manual scrolling", async () => {
  navigation.hash = `#${encodeURIComponent(fixture.releases[1]!.id)}`;
  render(page());
  await screen.findByRole("heading", { name: "Official baseline" });
  vi.mocked(Element.prototype.scrollIntoView).mockClear();
  const nav = screen.getByRole("navigation", { name: en.navigation });
  screen.getByRole("main").scrollTop = 75;
  await act(async () =>
    fireEvent.click(within(nav).getByRole("link", { name: /v0.4.37/ })),
  );
  expect(navigation.push).toHaveBeenCalledWith(`/acme/changelog${navigation.hash}`);
  expect(Element.prototype.scrollIntoView).toHaveBeenCalledTimes(1);
});
```

## Remaining acceptance boundaries

The leader owns real browser/Electron acceptance and broad verification; release/publication tooling remains outside this report. The leader reported that publication-to-open-reader acceptance passed, but this independent closeout did not rerun that workflow or certify the full pipeline. The earlier native `scrollIntoView` concern is resolved in the inspected source and permanent container-scroll tests. Actual browser/Electron layout behavior and container/PVC mounting remain covered by the leader's separate runtime evidence, rather than these unit tests.
