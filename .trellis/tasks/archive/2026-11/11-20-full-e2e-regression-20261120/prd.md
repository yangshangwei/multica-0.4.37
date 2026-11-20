# Multica fresh end-to-end regression

## Goal
Revalidate the current working tree from a user's point of view, prioritize core business journeys and button interactions, then cover all locally runnable features. Reproduce and repair defects discovered in this run.

## Background
The user reports a recent button regression and explicitly rejects historical passes as current evidence. On 2026-11-20 the user authorized immediate execution after the proposed core-first/full-regression plan and task creation request. Existing uncommitted work must be retained. Playwright targets production Web and built Desktop assets with a real isolated Go API and PostgreSQL.

## Requirements
- R1: Fresh core login/workspace/issue creation, editing, status, assignment, comments, completion and navigation tests, including canonical URL interaction and quick-create controls.
- R2: Fresh broader project/iteration/triage/agents/squads/chat/skills/MCP/settings/collaboration and native-renderer tests.
- R3: Configure locally available password/admin/AI/provider/daemon opt-ins and record remaining prerequisite gaps accurately.
- R4: For failures retain original output, distinguish fixture/environment defects from product bugs, reproduce and fix the owning behavior, add meaningful regression coverage and replay impacted flows.
- R5: Run static, unit, backend and script checks fresh; retain exact candidate/runtime/command provenance and a per-case result inventory.
- R6: Preserve user work and normal environments. Only synthetic actors and owned disposable environments may be mutated. No real messages, production deployments or uncontrolled installed-agent execution.

## Acceptance criteria
- Core tests run before the broad browser suite; all executed cases have current-run evidence.
- Each discovered failure is resolved and replayed, or explicitly documented with concrete evidence of the external prerequisite and impact.
- Pass/fail/skip/not-run counts are separately reported; fixture-only AI and Desktop renderer evidence are described precisely.
- Findings include reproduction, fix, changed files, validation and remaining gaps. Historical results are never counted as current passes.

## Out of scope
Unrequested redesigns, broad refactors, external publication, mutation of real user data, real-provider spending without an applicable opt-in, and claims of packaged/mobile/native acceptance based solely on browser tests.
