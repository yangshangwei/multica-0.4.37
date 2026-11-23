# Large-PR path-filter review

2026-11-23, read-only primary-source check. **Confirmed latent coverage risk; not a reproduced blocker for this candidate.** No workflow/source changes, remote PR creation, authenticated API request, or test interruption occurred.

## Primary-source facts

- GitHub's [List pull request files REST documentation](https://docs.github.com/en/rest/pulls/pulls?apiVersion=2022-11-28#list-pull-requests-files) states: “Responses include a maximum of 3000 files.” Pagination does not remove the endpoint's total cap.
- `dorny/paths-filter@v3` resolved through credential-free `git ls-remote` to annotated tag `15192bc058cc28a13dbf6cde61f19e18988b7af6`, peeled commit `0e4a8c6effa4802afeda77dc8d303f8176d7dfad`. The fetched [action.yml](https://github.com/dorny/paths-filter/blob/0e4a8c6effa4802afeda77dc8d303f8176d7dfad/action.yml#L5-L8) defaults `token` to `${{ github.token }}`.
- Its [main.ts:87–109](https://github.com/dorny/paths-filter/blob/0e4a8c6effa4802afeda77dc8d303f8176d7dfad/src/main.ts#L87-L109) selects REST for PR events when that token is nonempty. [Lines 177–225](https://github.com/dorny/paths-filter/blob/0e4a8c6effa4802afeda77dc8d303f8176d7dfad/src/main.ts#L177-L225) paginate `pulls.listFiles` at 100/page, then return the received files; there is no comparison with the PR's declared changed-file count or truncation refusal/fallback. [Lines 233–247](https://github.com/dorny/paths-filter/blob/0e4a8c6effa4802afeda77dc8d303f8176d7dfad/src/main.ts#L233-L247) emit false when no received path matches a filter.
- The action's [README token documentation](https://github.com/dorny/paths-filter/blob/0e4a8c6effa4802afeda77dc8d303f8176d7dfad/README.md#L148-L155) documents an empty-token Git fallback. That is an available future mitigation, not implemented here. The repository workflow uses the default token.

Fetched `main.ts` SHA-256: `5074df3dacad7f582fc96e469d4dbf7f8bfd53179f4ca36621c747ea0bf19aff`; `action.yml`: `cc582d1420764ea7e8e13bf299d9084f187b0e3eb7d2841b35f2d96e46b646ed`. Anonymous REST tag lookup returned rate-limit 403; public raw source/docs and Git advertisement succeeded. No credentials were used to bypass that limit.

## Repository consequence and current-candidate exception

`.github/workflows/ci.yml:39` uses this action. Its frontend/backend/sqlc results control individual steps; required aggregate jobs accept successful jobs whose work was filtered out. Thus an omitted relevant path can generally produce a misleading green. `images` flows directly from the filter (`:32`) to the image job condition (`:866`), so an omitted bitmap could also skip the budget check. A full-history checkout **inside** that already-filtered job cannot repair its admission decision.

For fixed `73a28ed90fdcfd9eda1ac5d2970df5af2c486f94` against `dd87e0b33c1d6a64d488f7ee9ecd0a65984a68ff`, local Git reports 23,867 changed paths. Its filename-ordered first 3,000 contain:

- `.github/workflows/ci.yml` at position 3, explicitly matching **frontend, backend and sqlc**;
- the first bitmap at position 8 (`.impeccable/audit/2026-10-14-automation/desktop.png`), with 184 bitmaps in the first 3,000, matching **images**. The action uses picomatch `dot: true`, so dot directories are included.

Consequently, the proposed “early .trellis files make this branch's product/image lanes false” does **not** follow from the current diff: even a filename-ordered truncation enables all four gates. GitHub does not promise that API order in the cited contract, and no actual PR file response/run was obtained, so this review does not claim remote filters have executed or prove an exact received subset.

Disposition: record the API-cap vulnerability as a **nonblocking, existing workflow risk**; do not broaden the present repair into unrelated CI redesign. For this candidate, verify actual `changes` outputs when a PR exists, especially `images=true`, while retaining the corrected script's full local diff/121-image evidence. A future filter-hardening change should fail closed or use complete Git enumeration for oversized PRs and cover all four outputs.
