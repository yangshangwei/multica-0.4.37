# GitHub target inspection — 2026-11-27

User supplied `https://github.com/yangshangwei/test--multica.git` as the response to test-environment planning. Record this as the selected GitHub test repository; the URL alone does not establish the Multica deployment, GitHub App installation or real-model execution policy.

## Observed facts

Read-only GitHub REST calls through the existing `gh` login:

| Field | Observed value |
| --- | --- |
| Repository | yangshangwei/test--multica |
| Repository ID / node ID | 1473759841 / R_kgDOV9fKYQ |
| Visibility | public |
| Default branch | main |
| Inspected main HEAD | 1d4590405fc7bfc06d4ed1ef741f25eeecf322af |
| Root tree | README.md, regular file mode100644; tree not truncated |
| Archived / disabled | false / false |
| Branch protected | false |
| Active branch rules | empty array |
| Current gh actor access | admin, maintain, pull, push and triage reported true |
| Merge methods | merge commit, squash and rebase enabled |

A reported repository size of0 did not mean the repository was uninitialized: its branch/tree query returned the commit and README above. Use the real branch/tree facts.

## Historical server-provider integration observation

`server/internal/handler/material_provider.go:22` resolves the selected installation through the workspace. For GitHub it signs an App JWT and requests a repository-limited installation token with contents/write, pull_requests/write, checks/read, statuses/read, metadata/read and administration/read. It does not use the developer's `gh` token as the product integration identity.

The checked-out root `.env`, parsed as dotenv data (no shell evaluation), has empty GITHUB_APP_ID, GITHUB_APP_PRIVATE_KEY, GITHUB_APP_SLUG and GITHUB_WEBHOOK_SECRET. No values were printed or copied. This only describes that file and the current process; it does not prove that no App exists elsewhere.

`GET /user/installations` with the existing CLI login returned403 requiring a GitHub App user token. Therefore installation presence and permission grants remain unknown; repository admin access is not proof of a usable Multica App installation. Do not replace this failed read with an installation/token write during planning.

## Planning effect

- Use this repository for the first GitHub stage; GitLab remains in the overall scope as a later stage with no selected target yet. Do not mark GitLab accepted based on GitHub.
- Use synthetic, nonprivate material for this public repository; do not copy product source, local secrets or another task's private evidence into it. A separate explicit source fixture supplies engineering references where required, since this target currently contains only README.md.
- `main` is the observed candidate target, to be rechecked before execution. A dedicated material root such as `multica-e2e/<run-id>/requirements/<material-id>/` keeps test writes identifiable without overwriting README.md; freeze the final root before publishing.
- Existing native-policy state cannot prove a blocking-gate scenario. Plan a separate test branch with a controlled required check after explicit test-rule authorization; do not change the ordinary default branch's rules merely to obtain coverage.
- Superseded assumption: App setup was previously proposed as the next step. The user clarified that runtime tools should perform Git operations. App configuration is no longer a prerequisite; see runtime-git-direction.md. The server implementation observation above remains factual, but describes the path that needs design correction.

No branch, commit, PR, comment, review, merge, rule, installation or token was created by this inspection. No model or daemon was launched.
