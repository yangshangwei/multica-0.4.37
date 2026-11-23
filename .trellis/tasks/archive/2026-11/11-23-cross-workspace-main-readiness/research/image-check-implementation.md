# Image budget check implementation

## Scope and source

- Private checkout: `.omx/private/qa-work/main-readiness-20261123/src`.
- Candidate HEAD: `cb192014b2f27e9bb64112db52073cbb31ce8f04`.
- Comparison base: `dd87e0b33c1d6a64d488f7ee9ecd0a65984a68ff`.
- Owned changes: `scripts/check-image-budget.mjs`, new `scripts/check-image-budget.test.mjs`, and image-budget test wiring in `.github/workflows/ci.yml`.
- No root product files, image bytes, historical evidence, dependency declarations or Git index/commits were changed by this lane.

## Behavior and implementation

The checker previously buffered the complete `git diff --name-status` output with `execFileSync`'s default approximately 1 MiB maximum. It also split Git's quoted path output by lines and tabs. The regression creates a real temporary Git repository whose staged path list exceeds that limit and places an oversized PNG after all long non-image paths.

The checker now reads the Git child stdout asynchronously with `-z`, retains only the incomplete field between chunks, and yields status/path pairs. UTF-8 decoding is supplied by the Node stream; tabs, newlines and non-ASCII filenames remain inside the path. Spawn failures, nonzero Git exits and incomplete records refuse a successful check. Early reader termination closes the owned Git child.

The budget remains 300 KiB. New or growing oversized images still fail; shrinking/deleting existing oversized images and the existing explicit reason-bearing PR exemption retain their behavior. All changed paths are still examined; no directory is exempted. CI selects the image job when the script or new test changes, selects Node 22, and runs the regression suite before checking the actual PR.

## Verification

Command: `node --test scripts/check-image-budget.test.mjs`.

- Original implementation: **8 pass, 3 fail**. The diagnostic red run retains `ENOBUFS` from the large-list case and the two quoted-path failures.
- Final implementation: **11 pass, 0 fail, 0 skip**, approximately 1.84 seconds on local Node 22.23.2 / macOS arm64.
- Tests cover exact threshold/non-images, new and growing oversized files, shrink/delete, explicit and empty exemptions, Unicode/space/tab/newline paths, invalid base, missing Git, and the real large diff with the last bitmap still checked. The control-character filename case explicitly skips on Windows, where those names are unsupported; CI runs this job on Ubuntu.

Additional checks passed:

```sh
node --check scripts/check-image-budget.mjs
node --check scripts/check-image-budget.test.mjs
pnpm exec eslint --no-ignore -c packages/eslint-config/base.js \
  --global process --global console --global Buffer --global URL \
  scripts/check-image-budget.mjs scripts/check-image-budget.test.mjs
pnpm exec tsc --allowJs --checkJs --noEmit --skipLibCheck \
  --module nodenext --target es2022 --types node \
  scripts/check-image-budget.mjs scripts/check-image-budget.test.mjs
git diff --check -- scripts/check-image-budget.mjs .github/workflows/ci.yml
```

The first scoped ESLint invocation used the generic repository base without Node globals and reported only undefined Node globals, including pre-existing script uses. Re-running with the actual Node environment globals passed without changing source or suppressing a rule. Both pnpm commands print the existing root pnpm-field configuration warning. A parsed YAML assertion verified the Node version, test command and script/test path filters.

## Actual candidate results and limits

Command: `node scripts/check-image-budget.mjs --base dd87e0b33c1d6a64d488f7ee9ecd0a65984a68ff`.

1. Empty `PR_BODY`: **exit 1, 121 oversized bitmaps reported, no ENOBUFS**. The checker defect is resolved; the image-policy gate remains failed until material disposition is addressed.
2. Test-only `Oversized image exemption: ...` reason: **exit 0, all 121 bitmaps still reported**. This verifies the existing mechanism only; it is not an approved exemption or evidence that a real PR passed.

No remote Actions job, application build, full repository test suite or image conversion ran in this lane. The coordinator owns final candidate refresh, independent review and integration.

## Evidence

Raw outputs and receipts are retained in the ignored task-owned directory `.omx/private/qa-work/main-readiness-20261123/image-check/`:

- `red-tests.*` and `red-tests-diagnostic.*`
- `green-tests.*`
- `lint.*` and `checkjs.*`
- `candidate-budget.*` and `candidate-exemption-mechanism.*`
- `final-sources-checks.json`, with final source hashes and syntax/whitespace results

Final source SHA-256 values:

| File | SHA-256 |
| --- | --- |
| `scripts/check-image-budget.mjs` | `5928e58770905257e47a8e9d714e0ec3e864612c2ded9444863908e392fec18f` |
| `scripts/check-image-budget.test.mjs` | `382e908b72447c5aa607f29e83bbc369fb2db2ac7d7637fdffc78fcf12a55123` |
| `.github/workflows/ci.yml` | `f7a043109f5a8fce3e583ea9ff0e49de8960ee0190f324712406a2a2b6446b90` |
