# 内置进展报告员与日报模板，验证并发布 v0.4.45

## Goal

Add one built-in Progress Reporter, adjacent daily/weekly reporting automation templates, full E2E coverage, commit and push, v0.4.45 release with verified intranet upgrade and Windows desktop artifacts.

## Authorization and scope

The user accepted the preceding recommendation and explicitly requested implementation, complete end-to-end testing, commit/push and v0.4.45 publication. Their screenshots identify the existing built-in agent catalog and role picker. This authorizes the normal implementation and release workflow without another approval gate. Existing unrelated GitLab planning files in the main checkout are out of scope.

## Requirements

1. Add exactly one listed built-in role, `progress-reporter`, displayed as 进展报告员 / Progress Reporter. It must appear in the existing catalog and template creation flow, create an ordinary editable agent, and support both daily and weekly reports.
2. Add `daily-progress-report` as a separate built-in automation template adjacent to `weekly-progress-report`. Daily reporting uses the last 24 hours, every day at 18:00 in the selected timezone, and creates a dated report issue. Preserve the weekly cadence (Monday 17:00, last seven days).
3. Reports read real task state, state-change history and comments; paginate; distinguish missing data from zero; show traceable counts, completions, ongoing work and blockers. Exclude reporting issues from business counts. Scope and reporting cutoffs must be explicit.
4. Report on the issue created by the automation. Keep business issues read-only; permit only the current report issue to move to `in_review` after successful comment delivery so the run reaches completed. Do not claim a model task finishing alone closes an issue. Reconcile the existing weekly prompt's conflicting blanket status prohibition through a versioned default-template update, without rewriting stored copies.
5. Use the existing localized catalogs and shared web/desktop UI. No new dependency, database schema, squad type or runtime protocol.
6. Provide focused real-API E2E coverage of role discovery/creation and daily/weekly adoption, ordering, cadence, timezone, prompt persistence, dispatch, report delivery and run closeout, plus full existing E2E and relevant unit/integration/static checks. Test workspaces/runtimes must be isolated.
7. Submit and push verified commits, publish `v0.4.45` to `yangshangwei/multica-0.4.37`, and deliver a verified Linux amd64 intranet upgrade archive and a Windows desktop installer with checksums and upgrade instructions. The historical Windows artifact is x64/x86-64; an asynchronous clarification about ia32 is pending.

## Acceptance evidence

- [x] Nine listed roles; Progress Reporter creates with the correct instructions, role skill and autonomy.
- [x] Ten automation templates; daily and weekly are consecutive in the API and rendered catalog.
- [x] Daily and weekly select the same reporter independently and retain distinct prompts/cadences.
- [x] Successful report delivery moves only its report issue to review and completes the automation run; source issue state stays unchanged.
- [x] Chinese UI screenshots match the supplied catalog style, without clipped content.
- [x] Focused and full E2E, TypeScript checks/tests/lint, Go tests/vet, relevant packaging checks pass with saved logs.
- [x] Independent spec and code review findings resolved.
- [x] Main, remote and v0.4.45 identify the verified source; release workflow finishes successfully.
- [x] Offline archive starts against an isolated database, serves version 0.4.45 and both new catalogs, includes upgrade script/changelog/checksum.
- [x] Windows artifact architecture, embedded CLI, renderer version, archive integrity and update metadata are verified.
- [x] Deliverables are downloaded/present locally and linked with concise verification limits.
