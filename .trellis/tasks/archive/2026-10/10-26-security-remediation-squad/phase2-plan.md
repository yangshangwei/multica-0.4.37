# Phase two: built-in security remediation squad

## Authorization and evidence boundary

After receiving the simulation results and the explicit statement that real
model behavior remained unverified, the user requested “推进第二阶段”. This
authorizes phase-two implementation in this existing task. The earlier live-model
prerequisite is replaced for this delivery by the verified simulation, product
contracts and the checks below. `real_model_gate_passed` remains false: no
historical result is relabelled, and the user's no-real-runtime constraint stays
in force. No new Trellis task, production deployment or external publication is
part of this step.

## Smallest behavior gap

The approved security flow currently requires manual squad setup. Add one
discoverable built-in `security-remediation` template with a dedicated unlisted
`security-remediation-lead`, reusing the seven existing working roles. Preserve
the existing template keys, versions, runtime bindings, permission defaults and
workspace-owned content.

## Change boundary

- Server registries and two instruction files: v1 squad/leader, Coordinator
  ceiling, concurrency two, no Operator, no new role skill or execution model.
  Put the new squad before the Operator-bearing release/incident entries.
- Service/handler tests: roster identity, declared authority, private creation,
  reuse/permission boundaries, languages and prompt safety contracts. Update
  genuinely affected catalog counts; retain assertions for existing templates.
- Shared views: localized catalog summary and an existing shield icon if the
  avatar mapping requires it. Use the existing components and unknown-key
  description fallback; no layout, routing or state-management redesign.
- English/Chinese squad and role/security documentation, relevant built-in skill
  references, and generated Chinese docs bundle. No claim of a sandbox or
  validated autonomous model behavior.
- Relevant existing catalog/project-squad E2E tests, with fixture runtimes only.

Do not change databases, endpoint shapes, listed working-role instructions,
autonomy definitions, dependencies, production permissions or runtime providers.

## Routing contract

Dependency: applicability/security, conditional migration, one minimal repair,
independent QA and review. Code: security, diagnosis when unknown, separate
implementation, QA and review. Other risks: evidence-first security review and
`hold/unknown` with an owner when evidence is missing. Production exploitability,
credential/customer-data exposure and high-risk operations stop local execution
and go to human/incident/release handling. The leader only coordinates, uses the
actual roster's exact mention link, dispatches one next step, records `action`
and exits; actual member delivery precedes the next step and final `no_action`.
Final delivery is `in_review`, not an agent-declared `done`. Keep complete triage,
actual source/command/test evidence, rollback limits and residual risks.

Existing member stop rules remain authoritative. In particular, the listed
Diagnostician stops for security defects; the new leader must retain that stop
and hand it to a human rather than override the role or silently continue into
implementation. The template does not promise automatic closure of every case.

## Execution and validation

1. Implement backend registration, instruction contracts and focused tests.
2. Add shared catalog copy/icon, associated tests, docs and regenerated bundle.
3. Run focused Go tests with the ambient-agent guard and an explicitly isolated,
   migrated test database; reject skips. Run related views/docs tests, lint,
   typecheck and generated-doc checks.
4. Run the repository check pipeline (`scripts/check.sh`, the `make check`
   implementation): complete static checks, TS tests, Go race tests and vet,
   followed by the relevant production-Web squad/catalog/adoption and autonomy
   E2E files passed to its supported file selector. Do not run unrelated browser
   suites or real-provider smoke flows. No `agentintegration` tag, real-agent
   smoke, or real daemon provider startup.
5. Independent Trellis check, spec/acceptance update and a scoped Lore commit.
   Record remaining real-model verification honestly; do not deploy or publish.
