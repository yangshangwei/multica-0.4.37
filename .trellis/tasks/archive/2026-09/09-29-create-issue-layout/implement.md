Two independent lanes: assist component+copy/tests and dialog shell+panel sizing/tests. Root owns integration and full-modal Chromium QA. No API changes, no dependencies, preserve prior work.

Verification completed:
- 108 relevant tests passed, ESLint and views typecheck passed.
- After the final flex sizing fix, all 89 shell/panel tests and scoped lint passed again.
- Full CreateIssueDialog with real panels/editors and mocked API: both modes at 1440x900, 1280x720, 390x844, 375x667. No horizontal overflow or page errors; auto fill, fold preserving answers, merge and undo pass.
- Long Markdown + file card at 375x400: merge reachable, file card retained, footer stable in both modes. Fixed editor overlap uncovered by this check.
- Design detector and git diff --check passed. Screenshots under .omx/qa/create-issue-layout/.
- No deployment or commits. Real device keyboard and native Electron window chrome were not tested.
