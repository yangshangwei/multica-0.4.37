# Release lane implementation and verification

Date: 2026-09-13. Owner: `implement_release`. Parent contract:
[`09-12-desktop-changelog/design.md`](../09-12-desktop-changelog/design.md).
This is lane evidence for independent review; it does not close the parent task
or claim desktop/server end-to-end acceptance.

## Delivered files

- `scripts/changelog-lib.mjs`: strict v1 validation, 2 MiB bounded UTF-8 reads,
  immutable identity checks, shared formatting and flushed atomic file staging.
- `scripts/generate-changelog.mjs`: argv-based immutable Git generation, public
  trailers, ancestor selection, cumulative history and same-record Markdown.
- `scripts/publish-changelog.mjs`: standalone validated exact-byte publication.
- `scripts/release-changelog.mjs`: fail-closed GitHub history discovery,
  immutable build metadata and draft-first publication to the requested fork.
- `scripts/install-changelog.{sh,mjs}`: target-side publication through the
  already-loaded frontend Node image and durable narrow dotenv updates.
- `scripts/{offline-bundle,offline-installer,offline-upgrade,build-offline-upgrade}.sh`:
  exact artifact staging/embedding, complete archive contents and deployment
  handoff before service restart.
- `.github/workflows/release.yml`, `.github/RELEASING.md`: serialized generation
  before builds, fork-safe publication gates and operational documentation.
- `scripts/{generate-changelog,publish-changelog,release-changelog,install-changelog,offline-changelog,changelog-workflow}.test.mjs`
  and `scripts/changelog-test-helpers.mjs`: focused contract and execution tests.

No dependencies were added. Root-owned seed, deployment files, package aliases,
and unrelated existing changes were preserved. The root integration lane added
the Dockerfile/Compose `CHANGELOG_ARTIFACT_PATH` build argument consumed here.

## Requirement audit

| Requirement | Evidence |
| --- | --- |
| Deterministic notes from committed immutable range | Temporary Git fixtures cover merged non-merge work, Conventional/legacy subjects, unrelated high tags, annotated targets, dirty preview exclusion and release cleanliness. Preview JSON and Markdown are compared against saved first-run bytes. |
| Publication truth and public overrides | Published status requires immutable ref/tag, explicit timestamp and valid version/status pairing. `Release-note` and `Changelog` overrides include skip and malformed/conflicting cases. Markdown escapes hostile strings without shell execution. |
| Preserve full history and reject conflicts | Required history, foreign/mixed-fork rejection, duplicate/invalid/oversized feeds, incomparable bases, first → prerelease → stable → older retry, same-version revision/status/base conflicts, covered/later previews. |
| Exact built artifact is published after required gates | Tests parse the actual release job graph. `changelog` runs once before Docker consumers; `publish-changelog` depends on verification and both image manifests, never the skipped upstream jobs. Local HTTP fixtures verify exact uploaded bytes, draft-before-assets, explicit repository/tag, tag drift, malformed/auth/network/missing-asset failures and idempotence. |
| Atomic local publication | Tests verify a changed inode, exact bytes, mode `0644`, temp cleanup, invalid input and rename failure retention. Installer tests add incompatible paths, nonregular/symlink targets, write permission failures, multiline dotenv preservation and rollback after injected env-finalization failure. |
| Offline distribution and installation without host Node | Actual generic, upgrade and combined desktop archive commands run with fixture image builds. Archive contents and staged Docker-input bytes equal the delivered feed. Target fixtures hide host Node and run the real bundled publisher through a Docker driver. An additional opt-in test used the actual loaded frontend image with real directory mounts, `--pull never`, `--network none`, and host UID/GID. |
| Durable runtime source configuration | Real Compose parsing after installation, with shell overrides removed, confirms persisted `CHANGELOG_FILE` and `CHANGELOG_DIRECTORY`; tested directory includes apostrophe, dollar sign and backslash. Upgrade asks Compose to use the deployment directory, preserving the stack/project identity and relative-path meaning. |

## Executed checks

- `MULTICA_RUN_DOCKER_CHANGELOG_SMOKE=1 node --test scripts/*changelog*.test.mjs`
  — **43 passed, 0 failed, 0 skipped** after independent-review corrections. The real-container test used already
  loaded `multica-web:dev` on Docker Desktop; no image pull or production
  service mutation occurred. Test containers and task-owned temporary stages
  were removed.
- `node_modules/.bin/eslint --no-config-lookup --config /tmp/multica-changelog-eslint.config.mjs scripts/*changelog*.mjs`
  — exit 0, no diagnostics. The temporary config imports the existing
  `packages/eslint-config/base.js`, declares Node globals for `.mjs`, and enables
  `no-unused-vars`; no lint dependency or repository configuration was added.
- `bash -n scripts/install-changelog.sh scripts/offline-bundle.sh scripts/offline-installer.sh scripts/offline-upgrade.sh scripts/build-offline-upgrade.sh`
  — exit 0.
- `node --check` over the 12 `scripts/*changelog*.mjs` files — all passed.
- Ruby standard-library `Psych.safe_load` parsed `.github/workflows/release.yml`
  successfully, with all 10 expected jobs. The Node graph tests separately
  check dependency and guard semantics; YAML parsing alone is not that proof.
- `git diff --check -- scripts .github/workflows/release.yml .github/RELEASING.md`
  — exit 0.

Initial generator/publisher tests failed before their implementation existed;
all three initial workflow tests also failed against the original workflow.
Later regression tests caught malformed-release bootstrap and missing explicit
publication identity checks before those fixes were added. The offline tests
also exposed two existing touched-path issues: unquoted installer README
heredoc command expansion and a GNU-only sed expression in the upgrade script.

## Frozen checkout dry run

Executed successfully:

```bash
node scripts/generate-changelog.mjs \
  --ref 719cb82d02c1ff80b80e094ba39ffd0f040950a5 \
  --base f1c059edee4ec39d464915e9c9a83cf6c0c37925 \
  --history server/internal/changelog/content/changelog.json \
  --repository yangshangwei/multica-0.4.37 \
  --version Unreleased --status unreleased \
  --output /tmp/multica-frozen-changelog-preview.json \
  --markdown /tmp/multica-frozen-changelog-preview.md
```

Output contains the preserved official reference plus an explicitly unpublished
fork preview (`published_at: null`) with 23 feature, 2 improvement, 18 fix and
20 legacy/other commit entries. Representative evidence includes in-app docs,
role/squad templates, deployment-owned logout, session protection and offline
installation. The root's curated product seed was not overwritten.

## Decisions and limits

- GoReleaser builds only `server/cmd/multica`, not the backend/changelog package.
  Its original `needs: verify`, upstream owner guard, and dirty-tree validation
  remain intact. The parent approved excluding that non-consuming job from seed
  replacement; copying a generated tracked file there would break validation.
- A Node CLI `--env-file` option is reserved by the runtime and can be consumed
  before the script. The internal installer uses `--deployment-env` instead.
- Docker Desktop did not share this machine's per-user temporary folder. The
  explicitly enabled real-container smoke uses its own ignored
  `.changelog-build/smoke-*` directory under the shared checkout and removes it.
- No real tag, push, GitHub Release, Homebrew update or production deployment was
  attempted. GitHub API failure cases are proven with a local HTTP fixture; the
  actual Actions graph is inspected/tested but has not run on a production tag.
- New releases remain drafts until all assets upload. Replacing an existing
  published release's cumulative assets is not a multi-file GitHub transaction;
  an interrupted retry must be repaired from the verified workflow artifact if
  its required assets become incomplete. History retrieval intentionally fails
  closed in that state; the runbook documents restoration rather than reset.
- If both local configuration finalization and filesystem rollback fail, the
  installer reports explicit recovery instructions. The normal rollback path is
  covered by an injected rename failure; no claim of crash-atomic updates across
  two distinct files is made.
- Final server/API/UI acceptance, broad repository checks and task status changes
  remain with the root integration lane and independent reviewer.

No commit, tag, task archival or unrelated-file rollback was performed.

## Independent-review corrections

### Dotenv literal path encoding

The reviewer found that the original single-quoted directory encoding produced
invalid Compose dotenv for a trailing backslash or a backslash immediately
before an apostrophe. The original real-container smoke had those characters
separated and did not cover their adjacency.

A real Compose v5.0.2 parser experiment reproduced both failures. It also proved
that simply doubling every backslash inside single quotes changes the literal
path, because Compose preserves those doubled slashes. Double-quoted values
with escaped backslashes/double quotes and doubled literal dollars round-tripped
the original strings exactly. The installer now uses that encoding.

Failing real-parser regression tests were recorded before the fix. The added
matrix checks trailing backslash, backslash before either quote, repeated
backslashes, literal `$`/`${...}`/`$$`, both quote styles, and a second identical
installation. Ordinary tests also pin the critical encoded strings and reject
newlines/carriage returns without touching the previous feed or configuration.

### Deferred publication job retry

The reviewer reproduced a stale-artifact case not prevented by serial active
workflows: v1.0.0 preparation succeeds but upload leaves a draft; v1.1.0 then
publishes; rerunning only v1.0.0's failed publication reuses its old build
artifact. The original publisher accepted it, lost cumulative-history agreement,
and could regress GitHub's latest marker.

The new regression test first failed with `Missing expected rejection`. The
publisher now downloads/validates the current published cumulative history at
the publication boundary and rejects any built feed omitting or conflicting
with published entries before POST/PATCH/DELETE. It directs a full preparation
and rebuild; it never alters the successfully built artifact. The fixture
asserts zero mutations and unchanged release state for both missing and altered
historical entries.

A second red assertion showed a fully re-prepared old stable tag could still
become latest because its preparation timestamp was newer. GitHub latest
eligibility now compares stable numeric version components; an older tag can
publish its complete regenerated artifact without displacing a higher stable
version. Prereleases remain ineligible.

The exact full-preparation rerun also initially failed when the earlier first
tag had no published ancestor yet. The parent approved using the already-
configured `--first-base` only for that missing-ancestor case, through the same
ancestor validation. Existing ancestral bases still win and incomparable ones
still fail. A regression deliberately supplies a missing seed after the newer
publication to prove that current cumulative history is retained, while an
invalid fallback base is rejected.
The successful recovery executes the real `release-changelog.mjs prepare` and
`publish` CLI commands against the fixture HTTP server with the unchanged
workflow argument shape; it does not merely call the exported functions.

### Shell entry validates complete structured paths

The reviewer found the shell's earlier `config --environment | sed` handoff
could truncate newline-containing directories before Node received them. Real
shell-entry tests reproduced embedded LF, trailing LF, and CR failures; the
old code either accepted a truncated directory or created one before rejection.

The shell now obtains `config --format json` and sends it over stdin to Node in
the already-loaded image, with no network and no mounts. Complete directory
strings and the configured container path are validated before any host mkdir
or bind mount. Only these validated strings enter the later line-delimited
shell handoff. The real tests record Docker argv and assert no bind mount,
created directory, feed change or env change on rejection.

A second real parser experiment showed Compose canonical JSON escapes each
literal dollar as `$$`, including already doubled dollars. The validator
decodes precisely that serialization layer after validation. The real positive
shell test includes `$notes`, `$${literal}`, apostrophes and a backslash, and
verifies durable Compose round-tripping afterward.

### Docker mount CSV encoding

Independent recheck identified the next path boundary: Docker interprets each
`--mount` argument as CSV after shell argv parsing. A double quote in an accepted
source path produced `bare " in non-quoted-field`; rejecting commas also
unnecessarily excluded valid host paths.

Real-container red tests reproduced both the quote-only failure and the
quote-plus-comma rejection. Every package, deployment and feed mount now quotes
its complete `src=...` CSV field and doubles internal double quotes, while
retaining ordinary shell argv quoting and no `eval`. Commas are accepted.
The two real regression cases verify exact published bytes and durable Compose
path values; the combined case places quotes and commas in all source paths.
The existing no-host-Node and pre-mkdir CR/LF rejection tests still pass.

Final post-review command output is saved at
`/tmp/multica-changelog-review-fixes.log`: 43 passed, zero failed/skipped.
The focused ESLint command, all five shell syntax checks, and `git diff --check`
also passed after these fixes. No production publication or commit was made.
