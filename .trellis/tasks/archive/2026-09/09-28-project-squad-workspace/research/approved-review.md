# Approved review and implementation constraints

Source: `.impeccable/critique/2026-09-28T11-52-28Z__iews-projects-components-project-squad-section-tsx.md` (2026-09-28).

The user explicitly authorized creating a Trellis task and designing/implementing that review. The first delivery consists of compact summary, focused management, explicit default, unified issue creation and the first-issue state. Project attention/approval metrics need real task/run data and are a later contract.

Business source: `.trellis/spec/server/project-execution-squad.md`. Candidate squads are ordered. The first is the future-issue default, subject to higher-priority explicit/view/group defaults. Each issue has one selected assignee. Setup/readiness is not execution progress. Existing issue ownership is unchanged when project candidates change. The actual leader/member runtime and complete roster determine readiness; requested runtime_id alone does not.

Existing extension points: IssueSurface renderEmpty and controller.openCreateIssue; existing issue picker and useUpdateIssue mutation. Table/Gantt intentionally do not derive controller.isEmpty from unloaded rows. Use a true project count rather than a fake clientFilter to provide their initial project empty experience.

The checkout contains unrelated skill-library and dev-environment work from another session. Task creation used this session's CODEX_THREAD_ID and preserved the other task pointer. Never stage or revert unrelated edits.
