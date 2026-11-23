# Scripted security-squad simulation

The user explicitly requested simulation without calling a real runtime. This
supersedes further attempts to call Codex/OpenCode or the model provider during
this continuation. The real Codex container and model gateway are stopped, and
their temporary development/model credentials have been removed.

## What this run proves

Use a deterministic provider double with the existing Multica daemon and local
development API. The double implements only the app-server protocol required by
the adapter and executes predefined, clearly labelled simulation steps. It does
not launch Codex/OpenCode, contact an LLM, or read personal model credentials.
Real application code still owns queueing, task-token identity, precise mentions,
member return, leader wakeup, activity and issue status.

The simulation covers three synthetic security cases plus an escalation drill:

- Dependency: triage, security applicability, migration review, scripted replay
  of the existing two-file dependency patch, independent QA and code review.
- Authorization: security review, diagnostic reproduction of the failure,
  separate scripted implementation, independent QA and code/security review.
- Supply chain: missing evidence remains `hold/unknown`, with an explicit owner
  and release conditions, through a real leader/member/leader queue roundtrip.
- Production escalation: record a human/incident handoff and prohibited actions;
  execute no production operation or operator task.

The model's planning, judgment, instruction following, evidence quality and
resilience are **not** exercised. Script-authored reports and patches must never
be presented as model-generated results or proof of autonomous security repair.

## Implementation boundary

Only add/update task-local simulation scripts, container configuration, tests and
evidence. Reuse the unchanged fixture and its already-recorded patch. Do not edit
product source, template registries, shared historical roles or fixture baseline.
Use a simulation-labelled provider/daemon/project and API-only network gateway;
there must be no real agent binary, model route or model credential in that
execution environment. The original real-runtime failures remain historical.

The simulator must use the task-scoped Multica CLI configuration supplied by the
daemon for comments and activity. It must not use an operator's human credential
to impersonate a member. Every comment/result includes a simulation marker.
Dispatch a single next role and exit; let the real queue release its concurrency
slot, execute that role and wake the leader from the member's real API comment.
Bound rounds and fail visibly if the expected product transition does not occur.

## Acceptance and verification

1. No real agent executable is installed/resolved/executed in the simulation
   container, and model/external network destinations are denied.
2. Capture task IDs, actor IDs, trigger comment IDs, delegation attribution,
   member comments, leader activity and final status from the real API.
3. Verify the baseline has six tests with two intended failures, dependency QA
   has three passes after its patch, and final independent QA has six passes.
   Retain raw commands, exit codes, hashes and the exact patch replayed.
4. Require diagnostic, implementation and QA actor identities to be distinct.
   Exercise malformed mention and terminal/no-action behavior against actual
   product handling, not a simulator-only assertion.
5. The supply-chain and escalation cases keep their declared unresolved risk;
   none of the cases is marked `done` by the simulated squad.
6. An independent Trellis check reviews simulation boundaries and evidence, and
   runs the relevant local tests. Finish with no active tasks or temporary
   credentials in the disposable environment.

This deliverable can establish workflow feasibility. It does not satisfy the
original phase-one real-model gate or automatically authorize phase two.
