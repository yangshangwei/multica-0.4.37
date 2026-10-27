# Scripted security-squad simulation

This harness checks workflow feasibility without a real agent or model call.
It uses the existing Multica daemon, development API and task-scoped credentials;
the provider is a finite Python script. Every generated report identifies itself
as `SCRIPTED SIMULATION — NO MODEL`.

The queue, comment authorization, exact mentions, delegation attribution, member
return, leader wakeup, activity and issue status are real product paths. Role
choices, security assessments and the existing synthetic patch are predefined
test inputs. Successful simulation does not prove model judgment, instruction
following, independent AI review, or production security.

## Files and local checks

- `provider.py`: minimal app-server protocol and bounded scripted role stages.
- `codex-scripted-simulation`: drops from the daemon UID to `node` and grants
  only the generated task/lab/session paths. It launches Python, never Codex.
- `Dockerfile`: contains Multica, the script and ordinary Python/Git tools;
  it does not install an agent CLI, model authentication, or personal skills.
- `nginx-api-only.conf`: permits only the fixed local development API and
  WebSocket path; there is no model upstream. Docker DNS is restricted to IPv4
  for this host because its advertised IPv6 host route was unreachable.
- `test_provider.py`: configuration, provenance, protocol, CLI file-boundary
  and ordered fixture-replay checks, plus negative mutations of the adjacent
  `../evidence/scripted-simulation-20261027` snapshots. Keep those offline
  fixtures available; the checks make no real account or API requests.
- `validate_evidence.py`: checks captured product task/comment/activity rows
  against the operation log. It requires real task attribution and member-return
  links, product source-task IDs, matching audit actors and zero recorded model
  calls; a script's final success message alone is insufficient.

Run local checks from the repository root:

```bash
python3 -B .trellis/tasks/10-26-security-remediation-squad/pilot/simulation/test_provider.py
sh -n .trellis/tasks/10-26-security-remediation-squad/pilot/simulation/codex-scripted-simulation
```

Independent existing handler tests and their disposable-database/no-skip
requirements are documented in
`../../research/simulation-contract-checks-20261027.md`. They must not use the
running application's database or resolve ambient agent CLIs.

## Execution configuration

The main operator prepares `/work/simulation/config.json`, root-owned and readable
by the provider. Its public fields are:

```json
{
  "simulation_id": "security-squad-sim-20261027",
  "workspace_id": "<workspace UUID>",
  "squad_id": "<simulation squad UUID>",
  "api_url": "http://<internal gateway IPv4>:8080",
  "lab_path": "/work/lab",
  "patch_path": "/work/simulation/lab-diff.patch",
  "roles": {
    "maintenance-lead": "<coordinator UUID>",
    "product-analyst": "<triage UUID>",
    "security-reviewer": "<security UUID>",
    "diagnostician": "<diagnostic UUID>",
    "implementer": "<implementation UUID>",
    "qa-engineer": "<QA UUID>",
    "migration-reviewer": "<migration UUID>",
    "code-reviewer": "<review UUID>"
  },
  "cases": {
    "<case issue UUID>": {"key": "other-supply-chain"}
  }
}
```

The four supported case keys are `other-supply-chain`, `dependency-advisory`,
`code-authorization`, and `production-escalation`. Run dependency before
authorization on a fresh copy of `../fixtures/`. Put the existing
`../evidence/lab-diff-20261027.patch` at the configured patch path. The baseline
must have six tests, four passes and two intentional assertion failures.

The roles must have distinct IDs. The script reads `MULTICA_AGENT_ID`,
`MULTICA_TASK_ID`, `MULTICA_WORKSPACE_ID`, `MULTICA_SERVER_URL` and the active
`mat_` token supplied by the daemon. It verifies the current running task against
the API before doing work. HTTP reads use that same task token; writes go through
the existing Multica CLI and its task-scoped configuration. A human token is
rejected. Comment files are created inside the current task workdir, respecting
the CLI's file-boundary check.

Start only the script-backed daemon with
`MULTICA_CODEX_PATH=/usr/local/bin/codex-scripted-simulation`, one concurrent task,
a bounded task timeout, and automatic update/reload disabled. The provider name
`codex` selects the existing adapter; the fake version string is protocol
compatibility data, not evidence that Codex is installed or executed.

The provider container belongs only to a Docker `--internal` network, with
external DNS disabled. The gateway also belongs to an egress network and exposes
only `host.docker.internal:18572`; publish no ports and mount no host home/socket.
Its configuration contains no credentials. The daemon's temporary development
profile remains under root's `0700` directory, inaccessible to UID 1000.

The simulator never invents members when routing fails. It publishes one exact
member mention and `action`, then ends the turn. The member posts its own marked
comment; the real product wakes the next leader. After the final result the
leader posts a summary, moves the issue to `in_review`, then records `no_action`.
The supply-chain case also submits an intentionally malformed mention and checks
that the product does not create a member task for it.

## Evidence and recovery

Operations are recorded in `/work/simulation/audit/events.jsonl` without tokens.
Collect the final task runs, comments and timeline from the API for each case,
plus the baseline, fixture command output, final diff, source hashes and network
boundary checks. The standalone validator expects the snapshot filenames used in
the recorded simulation evidence directory and checks every stage/return link.

A transport timeout is not proof that a write failed. Stop and read the API
before any new trigger; do not blindly repeat a comment. A failed attempt remains
in the evidence with its reason and fresh recovery trigger. The outer CLI process
budget leaves the CLI its own transport/context grace so its diagnostic is not
prematurely truncated. The script never increases or disables the API timeout.

At completion, verify no queued/running task remains, export the synthetic lab
evidence, remove temporary credentials and stop/remove the disposable execution
containers. Keep simulated results separate from the earlier real-runtime
failures and the still-unverified real-model acceptance gate.
