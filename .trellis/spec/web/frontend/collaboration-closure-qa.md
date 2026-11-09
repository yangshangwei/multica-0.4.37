# Multi-account collaboration closure QA

## 1. Scope / trigger

Use when claiming a complete cross-workspace human/Agent delivery flow. Separate
human-only, squad and context tests do not establish one shared issue's closure.
The canonical synthetic scenario is `e2e/collaboration-multi-account-closure.spec.ts`.

## 2. Signatures

- `COLLABORATION_HOST_PROVIDER` selects the explicit task-owned
  `collaboration-multi-account-provider.go` binary for the closure spec.
- Host squad/switch specs use `collaboration-host-provider.go`; memory-runtime
  uses `collaboration-memory-provider.go`. Their callback protocols differ.
- Public collaboration comments expose `author_type`; `author` is a human
  summary and is null for Agent comments. There is no public `author_id` field.
- Exact Agent attribution comes from the same run's records and resource,
  plus `memory.author.agent` and `memory.author.resource_owner`.

## 3. Contracts

Use three independent accounts, source/resource workspaces and enrolled runtime
identities. Resource suppliers offer, a creator/handler adopts, and each owner
consents. An external supplier cannot inherit the manager's adoption authority.

The deterministic Provider must actually launch through the production poller
and write through the run-local CLI. Verify the same issue ID, command author,
resource owner, Agent, installation and exact prior context on every run. Check
the second delivery's exact acceptance after the first delivery's return.

Capture failures from custom browser contexts before closing them. Persist
evidence even when body execution or cleanup fails. A passed evidence result
requires successful body assertions, restored gate, closed owned daemons and
each actual run's sealed `proof_acknowledged` / `host_exit_observed` ledger.

## 4. Validation / error matrix

| Observation | Required classification |
|---|---|
| A comment lacks `author_id` | Check the public DTO; do not weaken identity checks or invent the field |
| Comment ID/body matches an exact run record with correct resource attribution | Supported Agent authorship evidence |
| Run queued or permit issued without a Provider observation | Not an executed Agent |
| New test fails before a field-shape correction | Retain the original failed attempt separately from rerun |
| Cleanup or evidence persistence fails | Fail the test; do not emit a passed evidence result |
| Full-page screenshot includes still-loading history | Use for visible completion only; history requires its separate assertions |

## 5. Good / base / bad cases

Good: three distinct owners produce 1/2/2 actual runs, context v1 to v6,
two submissions and one exact initiator acceptance; ledger checks agree.
Base: existing single-owner squad tests retain their original Provider.
Bad: count an unlaunched continuation as a process, or call synthetic business
text proof that a real model repaired payment code.

## 6. Tests required

Keep private ordinary issue denial and unchanged-state checks, repeated original
AI POST key/body with the same receipt and no extra process, separate owner
consent, explicit Resume, ordinary-comment non-dispatch, context conflict and
human-only completion. Stop during a process and squad delegation have separate
host-switch/squad scenarios; report their ownership and coverage separately.

## 7. Wrong versus correct

Wrong: concatenate unrelated passing suites into a complete multi-owner story,
or label all attempts green after fixing a test assertion.

Correct: retain the same-issue closure evidence, exact final case inventory and
first failed attempt. State that synthetic processes establish orchestration,
not commercial model reasoning, multiple physical machines or OS containment.
