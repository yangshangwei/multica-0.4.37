# Planning evidence — 2026-11-27

Read-only repository review. No remote account, test database, daemon, model or provider operation was invoked during planning.

## Reusable implementation and evidence

- `../archive/2026-11/11-26-git-material-collaboration/research/final-verification.md`: accepted local delivery, exact package counts, remaining full-Go/live-provider/model/installer gaps. Source manifest is retained alongside it.
- `e2e/fixtures/material-run.md` and `.mjs`: identity-checked local fixture runner. It hardwires createMaterialGitLab and owns a loopback fake provider; do not point it at a real repository and call the result live acceptance.
- `e2e/material-workflow.spec.ts:59`: seedProgressAgent constructs the unclaimed runtime fixture. `:174` opens the human plan editor. Dispatch counts prove enqueue, not native execution. Reuse UI assertions after replacing fixture preparation with a separately reviewed live environment entry.
- `server/internal/service/material_worker.go:365`: materialPlanPrompt includes full PRD, fixed commit/path/digest, sources and CLI submission instructions. Live checks must prove receipt and consumption, not just inspect this string.
- `server/internal/handler/material_capability.go:16`: plan-task token ceiling resolves current persisted task/plan; only material read, project resources, own issue and revision submission are allowed. Verify the installed CLI can use this actual narrow API without swapping in human credentials.
- `server/internal/service/material_plan.go`: current authority and exact baseline/task binding at queue consumption; terminal manual restart stays explicit. Do not seed task states directly to manufacture a successful run.
- `server/internal/materialgit/README.md`: supported HTTPS endpoints, GitHub protection/rulesets, unknown-policy blockers, ambiguous GitLab branch ownership and target-atomicity limitations. Pin actual GitLab version, native rule set and available API permission during execution preflight.
- `server/internal/service/builtin_skills/multica-projects-and-resources/references/git-materials.md`: candidate CLI workflow and request recovery. Agent must author revision JSON with original request identity and submit the actual output.

## Verification environment constraints

- `CLAUDE.md:232`: real agent smoke tests use agentintegration plus MULTICA_RUN_REAL_AGENT_SMOKE=1 after authorization. That smoke proves adapter operation only; full live acceptance must use Multica daemon task claim/start rather than direct shell invocation as a substitute.
- `scripts/test-go.sh`: default wrapper refuses absent DATABASE_URL and guards ambient agent CLIs. It serializes packages sharing one URL. The stronger database-isolation spec requires an owned fresh DB per DB-backed package; prepare/verify a package-isolation launcher or use explicit package runs, without weakening tests.
- `.trellis/spec/server/verification-environment.md`: separate API pool config from test config; bind source/build/config before and after execution. Distinct test HOME/temp roots stay outside all Git worktrees.
- `.trellis/spec/server/database-test-isolation.md`: DB identity, retained failed runs, dedicated Redis where needed and current source/fixture clocks. Existing private material-test databases are historical evidence, not a fresh whole-repository acceptance environment.
- `11-12-collaboration-real-runtime-acceptance/prd.md` owns unrelated runtime/knowledge experiments with their own authorization and budgets. This task does not reuse them.

## Remaining information

The repository cannot determine the user's preferred isolated vs shared staging boundary or authorize real quota/remote writes. Choose that boundary first. Instance/CLI versions and exact credential capabilities are runtime facts to inspect after a permitted environment is identified; no speculative public-web research establishes them.

## Direction correction

User clarified runtime-based Git operation on2026-11-27. See runtime-git-direction.md: existing App/provider behavior is implementation evidence, not a requirement to install an App. Live acceptance must wait for the necessary runtime operation/receipt integration design, not merely a credential substitution.
