# Release lane independent code review

Date: 2026-09-13. Reviewer: `review_release`. Scope: the complete release lane,
its focused tests, and Docker/Compose artifact integration. This review read
`CLAUDE.md`, the parent PRD/design/implementation plan, the release child context
and verification report, and the applicable intranet documentation rules.

Final disposition: **APPROVE**. All findings covering history/retry, CR/LF
validation, dotenv encoding, the shipped guide, and Docker mount quoting are
resolved and independently verified. No remaining blocker was found in the
reviewed release lane. Original reproductions and correction evidence are kept
below. The reviewer changed only this file; implementation changes belong to
the implementation/integration lanes.

## 1. Resolved — a saved older publication artifact could invalidate later history

Original locations: `scripts/release-changelog.mjs:172–205` and the
`publish-changelog` job in `.github/workflows/release.yml:422–453`.

Originally, `publishRelease` checked the immutable tag and, for an already
published target, that target's previous assets. It did not compare the supplied artifact with
the other releases currently published in the repository. Workflow-wide
concurrency cannot protect a successful preparation/build artifact reused by a
later **Re-run failed jobs** operation.

Reproduced using the real generator and publisher against a local in-memory
GitHub HTTP-response fixture, with a temporary Git repository:

1. Prepare `v1.0.0`, then fail its first asset upload. Its Release remains a draft.
2. Prepare and successfully publish `v1.1.0`.
3. Retry only publication of `v1.0.0`, using its original successful build artifact.
4. Prepare the next tag, `v1.2.0`.

Observed output:

```text
v1.0.0 initial publish: GitHub POST /uploads/1 returned HTTP 503; history was not reset
delayed v1.0.0 retry was accepted
v1.1.0: draft=false, make_latest=true, versions=[v1.1.0,v0.4.37]
v1.0.0: draft=false, make_latest=true, versions=[v1.0.0,v0.4.37]
next preparation fails: published release histories conflict or lack cumulative entries; repair history explicitly
```

This broke R8's cumulative-history guarantee and the runbook's promise that an
older retry retains later releases and leaves the latest stable marker intact.
It is separate from the documented inability to transactionally replace several
GitHub assets.

Before any mutation, publication must verify that its exact built artifact
contains the current published cumulative history unchanged. A stale artifact
must fail with an instruction to re-prepare and rebuild; changing its JSON in
the publication step would violate exact build/asset reuse. Add a regression
with a failed older draft, an intervening successful release, and a delayed
publication-only retry, asserting no mutation and no latest-marker change.

Recovery originally also failed at line 143: `--first-base` was used only when
`history.initial` was true. After the newer release, the older failed target had
no ancestral published fork entry, so a full re-prepare failed with
`--base is required`.

**Verified correction:** `publishRelease` now validates current cumulative
history before any write and rejects an omitted or altered publication record.
`selectBase` uses the explicitly configured first base only when there is no
published ancestor, checks that it is an ancestor, and leaves the downloaded
history intact. Existing ancestral releases win; incomparable bases still fail.
GitHub latest eligibility compares stable numeric versions, so rebuilding an
older tag later does not promote it above a higher stable version.

The regression at `scripts/release-changelog.test.mjs:182` passed independently.
It asserts zero POST/PATCH/DELETE and unchanged release state on a stale retry;
then it executes actual CLI `prepare` and `publish` subprocesses against the
local HTTP fixture. The seed path is deliberately absent during recovery, a
nonancestor first base is rejected, later history survives, altered historical
content is rejected without mutations, and the restored older release publishes
with `make_latest=false`.

## 2. Resolved — the shell wrapper truncated newline paths before rejection

Location: `scripts/install-changelog.sh:39–45`.

Compose's `config --environment` output is line-based. A multiline
`CHANGELOG_DIRECTORY` is emitted on multiple lines, but `sed -n
's/^CHANGELOG_DIRECTORY=//p'` retains only its first line. The subsequent newline
guard and Node's single-line check therefore receive an already truncated path.
The installer can create/install to a different directory and persist that
different value, instead of rejecting the supplied path.

Reproduced with real Docker Compose parsing, without starting any container or
changing a deployment:

```text
requested: /tmp/fixture/directory\nsecond-line
Compose exit: 0
value after the installer's sed: /tmp/fixture/directory
newline reaches Node validator: false
```

Validate the complete value before line extraction, or retrieve the relevant
configuration through a structured format. The new Node-only newline tests do
not cover the shell entry point; add a wrapper-level rejection test that checks
the previous feed/configuration and filesystem destination remain unchanged.

**Verified correction:** the wrapper now uses `config --format json` and sends
the complete structured source path to Node in the loaded image over stdin.
The preflight container has no mounts or network. It rejects complete CR/LF
values before the first host `mkdir`; only validated strings enter the final
two-line shell handoff. It decodes one layer of Compose's `$$` serialization.
The independently executed real shell regressions passed for embedded LF,
trailing LF, and CR, with no destination/prefix creation, bind mount, feed change,
or dotenv change. The real positive test also preserved `$`, `$${literal}`,
apostrophes, and a backslash through installation and subsequent Compose parsing.

## 3. Resolved — the guide shipped in upgrade archives was outdated

`scripts/build-offline-upgrade.sh:72–73` replaces the bundle README with
`docs/offline-upgrade.zh-CN.md` and ships that file again as `操作文档.md`.
The guide's lines 33–42 omit changelog installation and still say
“脚本不会覆盖 `/opt/multica/.env`”. The upgraded script now persists
`CHANGELOG_FILE` and `CHANGELOG_DIRECTORY` before recreating services.

Update the actual shipped guide, including the new installation step, its
failure/recovery behavior, and preservation of all other dotenv settings.
Changes only to `.github/RELEASING.md` or the generic bundle README do not reach
this archive's operator instructions.

**Verified correction:** the actual source guide now describes the feed and
loaded-image publisher, adds feed installation before service recreation,
documents exactly the two updated dotenv keys and preserved remaining settings,
and includes feed-only refresh instructions. The packaging script still copies
that corrected source into both shipped guide files.

## 4. Resolved — valid dotenv paths could produce invalid Docker mount CSV

Original location: `scripts/install-changelog.sh:80–84`.

The original preflight accepted a directory containing a literal double quote,
and the dotenv writer correctly encoded it. However, Docker parses each
`--mount` value as CSV. Directly inserting the path into
`type=bind,src=$directory,dst=...` produced an invalid CSV field for a path such
as `/tmp/a"quote`.

An independent Docker argument-parser reproduction returned exit 125:

```text
invalid argument "type=bind,src=/tmp/changelog-review-a\"quote,dst=/probe" for "--mount" flag:
parse error on line 1, column 38: bare " in non-quoted-field
```

This failure is safe for the existing feed and dotenv because Docker rejects
the request before running the installer. It nevertheless prevents the offline
wrapper from using a legal custom path that the dotenv tests support, and can
leave a newly created empty directory. CSV-quote source fields, including the
package and deployment mounts, and test a real positive shell path containing
double quotes. Alternatively, explicitly reject/document this character before
creating the directory instead of allowing it through preflight.

**Verified correction:** `mount_source` now CSV-quotes the entire `src=` field
and doubles embedded double quotes. All four bind sources use it: the package
tools directory, package feed directory, deployment feed directory, and
deployment directory. Shell argv remains individually quoted and no `eval` is
used. Commas are now supported rather than rejected by the path preflight.

Independent focused verification passed against the already-loaded image:

```bash
MULTICA_RUN_DOCKER_CHANGELOG_SMOKE=1 node --test \
  --test-name-pattern='real shell entry preserves quotes and commas' \
  scripts/offline-changelog.test.mjs
```

Result: **3 passed, 0 failed, 0 skipped** (two path cases plus the parent test).
The quote-only case verifies the originally failing feed directory; the second
case places quotes and commas simultaneously in package, deployment, and feed
paths. Both verify exact installed bytes, preserved unrelated dotenv settings,
and the durable directory value after removing shell overrides and reparsing
with real Compose. This closes the last finding.

## Resolved during review — dotenv escaping

The original `scripts/install-changelog.mjs:44` escaped apostrophes but not the
interaction between backslashes and single-quote delimiters. Installation
reported success while persisting invalid dotenv for both `/srv/path\` and
`/srv/a\'s/path`. Real Compose then failed with “unterminated quoted value” or
“unexpected character … in variable name”.

The implementation lane changed the encoder to double quotes with escaped
backslashes/quotes and literal dollar handling. Independent verification passed:

```bash
MULTICA_RUN_DOCKER_CHANGELOG_SMOKE=1 node --test \
  --test-name-pattern='real Compose' scripts/install-changelog.test.mjs
```

Result: **6 passed, 0 failed, 0 skipped** (five path cases plus the parent test).
It covers trailing backslash, backslash before either quote, repeated
backslashes, dollar literals, and installing the same configuration twice.
This closes the original encoding finding; finding 2 concerns the earlier shell
input boundary instead.

## Reviewed guarantees and documented limits

- Generator refs resolve to immutable commits once; explicit/derived bases are
  checked by ancestry, including merged non-merge work and incomparable bases.
  Mixed/foreign fork history is rejected. Published records are compared for
  immutable content; an older already-published retry preserves later bytes.
- Commit messages are obtained through argv-based Git calls. Preview ignores
  dirty worktree text; publication requires a clean tracked tree, explicit
  timestamp, and supported version/status. Public trailers and Markdown text
  escaping are tested. Unsupported producer source/status/schema were also
  directly checked and rejected during this review.
- GitHub history retrieval includes prereleases and rejects authentication,
  retrieval, malformed metadata, missing assets, digest mismatch, and conflicting
  cumulative histories. It bootstraps only with zero published releases.
- The actual job dependency graph makes the fork publication depend on verify,
  generation, and both successful image manifests, without depending on skipped
  upstream Homebrew/GoReleaser/desktop jobs. Builds download the same generated
  JSON before compilation. Dockerfile's `CHANGELOG_ARTIFACT_PATH` and the Compose
  build argument preserve exact offline input bytes without rewriting the seed.
- Offline packaging **does not generate current-commit notes automatically**.
  `offline-bundle.sh:32,150` validates/copies the seed unless `--changelog` or
  `CHANGELOG_ARTIFACT` is supplied; both wrapper commands forward that option.
  This is explicitly documented in `.github/RELEASING.md:134–139` and is treated
  as a limit of the supplied-artifact packaging design, not as evidence that
  automatic offline generation was tested or delivered.
- The target installer uses the loaded frontend image, `--pull never`,
  `--network none`, host UID/GID, and containing-directory mounts. It stages both
  files before renaming and restores the old feed on configuration-finalization
  failure. The release lane's original loaded-image evidence was inspected;
  after the preflight changes, this review reran the focused real-image/shell
  tests. No production images were built.
- Replacing several existing published GitHub assets is not transactional. An
  interrupted replacement may require restoration from the retained workflow
  artifact before discovery/retry succeeds. The implementation report states
  that limit. Likewise, filesystem rollback can fail if the filesystem stops
  accepting writes; the installer reports that case instead of claiming success.

## Verification run by this reviewer

- `node --test scripts/release-changelog.test.mjs scripts/install-changelog.test.mjs scripts/changelog-workflow.test.mjs`
  — **12 passed, 0 failed, 1 skipped** at the observed implementation state. The
  skipped real-Compose test was then explicitly enabled and passed as above.
- Temporary-repository delayed-publication reproduction — confirmed finding 1.
- Real Compose path serialization and line-extraction reproductions — confirmed
  the original escaping defect and finding 2.
- Direct producer validation probes — unsupported source, status, and schema
  were rejected with actionable errors.
- `git diff --check -- scripts .github/workflows/release.yml .github/RELEASING.md Dockerfile docker-compose.selfhost.build.yml docker-compose.selfhost.yml`
  — exit 0.

Focused follow-up verification:

- `node --test scripts/release-changelog.test.mjs scripts/generate-changelog.test.mjs scripts/changelog-workflow.test.mjs`
  — **16 passed, 0 failed, 0 skipped**.
- `MULTICA_RUN_DOCKER_CHANGELOG_SMOKE=1 node --test --test-name-pattern='actual loaded frontend|real shell entry' scripts/offline-changelog.test.mjs`
  — **5 passed, 0 failed, 0 skipped**, using the already loaded frontend image.
- `bash -n scripts/install-changelog.sh scripts/offline-bundle.sh scripts/offline-installer.sh scripts/offline-upgrade.sh scripts/build-offline-upgrade.sh`
  — exit 0.
- Diff whitespace check including `docs/offline-upgrade.zh-CN.md` — exit 0.
- Final Docker CSV path regression, using the exact focused command above —
  **3 passed, 0 failed, 0 skipped**. Only this new real shell regression was rerun
  after the final mount-encoding change; no broad suite was repeated.

No real release/tag/push/deployment, authenticated model call, source edit, or
large suite rerun was performed. Server/API/UI acceptance remains owned by the
root integration lane. All review findings are closed. The documented limits
above remain: offline packaging requires an explicit generated artifact for
new notes, and GitHub asset replacement is not a multi-file transaction.
