# Test design

Use independent lanes for TypeScript/mobile units and native-agent evaluations.
The leader owns Go/database integration, isolated API/web setup, full E2E and
the final evidence review. Do not share test databases with the visible desktop
workspace. Database-dependent suites run sequentially when they share fixtures.

Real-agent tests use disposable fixture workspaces containing the seven current
SKILL.md files, safe local data and no production application credentials. Test
natural selection separately from explicit invocation where the native runtime
supports it. Capture tool reads and artifacts to establish that a skill was
actually loaded and applied. A small scenario suite demonstrates these cases,
not a statistically general comparison of model quality or all providers.

Logs and machine-readable results live under
`.omx/reports/skill-descriptions-full-validation-20260912/`; the task records the
final summary. Reuse installed tools and repository test harnesses.
