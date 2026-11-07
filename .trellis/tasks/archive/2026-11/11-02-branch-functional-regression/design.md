# Verification design

## Boundaries

This is a test campaign, not a product implementation. The coordinator owns scope, evidence, environment allocation and final conclusions. Check agents execute explicitly bounded permits and write only task evidence. Research/check agents must not start independent APIs or change shared singleton gates concurrently.

The candidate is `7af9ab9905c98c77bb091223ebe687c639475bb4`. Product and harness revisions, build artifacts and configuration are separate identities. Historical reports remain historical even if a product tree is equal. Any candidate drift creates a new recorded run.

## Evidence flow

`requirements + branch diff -> risk/feature matrix -> collected test identities -> environment permit -> actual run -> raw result -> defect/coverage reconciliation -> independent review -> report and handoff`.

`test-plan.md` is the user-facing scope. `coverage.csv` initially holds every collected E2E identity as NOT_RUN; execution updates require matching raw reporter evidence. `evidence/<run>/` contains command/time/exit/provenance, stdout/stderr, framework JSON and artifacts. `progress.md` points to authoritative results and the next runnable step.

## Environment contract

Use a new task-owned environment rather than the mismatched listener currently registered for this checkout. The private source environment is copied, never sourced as shell code or printed. Verify database ownership and live API/Web identity before fixture writes. Fresh databases isolate Go DB-backed packages; browser lanes use separate databases. Cap owned pools and serialize API startups against aggregate PostgreSQL capacity. Never terminate another session or raise shared limits.

Web is built and served in production mode. Native runs use actual Electron, matching renderer/preload and task-owned userData. Native origin configuration must exist before API configuration identity is recorded. A source checkout lock protects its build directory. Use a detached same-commit worktree if isolation is needed; preserve the original untracked acceptance folder and all existing worktrees.

The stock `make check` is the canonical repository pipeline, but it shares one Go database across packages and can skip conditional E2E lanes. The stronger acceptance contract requires a reviewed per-package isolation runner and supplemental conditional lanes. A full stock command pass alone is insufficient for this campaign. Do not reuse the old private launcher or create its clearance file until the adapter-v5/RKI6 result reviews are actually complete.

## Test layers

- Static/unit checks establish code and parser/state invariants; they do not establish user journeys.
- API/DB tests establish authorization, transactions, history, idempotency, races and persistence.
- Production Chromium tests establish actual Web workflows and failure recovery.
- Actual Electron establishes native focus, IPC, navigation, clipboard, popouts and restart behavior.
- Task-only fake Provider processes establish physical input and lifecycle plumbing; they do not establish paid vendor compatibility or model comprehension.
- Mobile, Windows/Linux native installers, upgrades and real vendor accounts require their own explicit evidence. Missing prerequisites remain BLOCKED/NOT_RUN.

## Recovery and cleanup

Retain first failures and databases. A corrected prerequisite gets a newly numbered run. No timeout increase, assertion removal or automatic retry is permitted to obtain green. Product fixes are separate work. Snapshot and restore the task's gate booleans while preserving monotonic epochs. Stop only task-owned processes; retain evidence, data and artifact pins for replay. Never destroy the default or another task's database.

## Decisions and tradeoffs

Prioritize complete core and recent-change coverage over rerunning only recently edited tests. Execute resource-independent checks concurrently only when evidence ownership is clear; serialize database/API/native stages. Avoid new test frameworks and dependencies. Existing review gaps and lack of external authorization remain explicit rather than hidden by a new task ID.
