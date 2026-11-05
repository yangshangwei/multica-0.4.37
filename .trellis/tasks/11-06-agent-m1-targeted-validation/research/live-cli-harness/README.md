# Scoped live CLI/model fixture harness

These exact scripts are retained from the 2026-11-06 targeted campaign; they do not run Codex or any provider. They contain no credentials. The coordinator alone runs the separately reviewed model launcher. Execution requires the coordinator-owned private directory, explicit allocated database and actual API/CLI recorded below; do not run these files in the source task directory or against a default database.

## Inputs and ownership
- Private run root: `/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/multica-m1-targeted-y38tfuxx`.
- Fresh database: `rnd_m1_targeted_api01`; private `rnd_m1_targeted_api01.env` is parsed as data. Root validated owned PostgreSQL17 container and actual database identity before any fixture writes.
- Actual API: loopback18251, pid18397, source commit6405240dd; registry proof in private api-run. No real daemon/model worker consumes queues.
- Actual compiled CLI: private bin/multica, source6405240dd, SHA256 `d078fef1e06c4395a5413dc7b024cfb7591db786fa4042e38ca2122748f2d2fd`.
- Copy the exact scripts into a new coordinator-owned private run root only after allocating new resources and adapting the explicit recorded paths/API constants. Scripts deliberately refuse or fail if prerequisite fixture files/identity do not match. They are retained executable evidence, not a general environment provisioner.

## Order
1. `prepare-live-fixtures.py`: verify DB and API provenance; create only synthetic account/workspace/projects/issues and real reporter/reviewer role adoption/assignment/claim/start. Stores private token-bearing records0600. No model.
2. `trace-proxy.py`: transparent HTTP forwarding to actual API; log method/path/status/body hashes without auth values. The exact reporter/reviewer trial env points to its allocated loopback port.
3. Record each role's baseline with `trial-oracle.py ROLE before`, after root trims prompts to actual agent instructions + relevant role skill. Root supplies the exact task credentials to the explicitly authorized Codex process. After execution `trial-oracle.py ROLE after` compares DB/requests/files; semantic content is independently reviewed.
4. Reporter and reviewer share one workspace. Preserve the initial reviewer baseline separately, then capture its actual before-state after reporter completion to exclude the earlier legitimate comment.
5. After both model after-states are captured, `run-live-cli.py` runs20 actual CLI/API checks. It independently asserts allowed/denied operations, comments' persisted actor/task/dispatch fields and unchanged business/task state.
6. `prepare-leader-fixture.py`: add synthetic second account/private target, real squad/members/leader assignment and claim. `trace-proxy-leader-candidate.py` supports the additional leader credential; it records a second owned port/pid. `leader-oracle.py before/after` checks one real delegation comment, one failed evaluation, one allowed queued task and zero blocked-target tasks.

The leader's before/after issue comparison includes business fields only, not revision or activity timestamps. Revision changes are expected for comment/evaluation writes. Queued work is admission, not execution or completion.

## Evidence interpretation
CLI HTTP traces forward actual requests unchanged; they are not fixed-output mocks. Responses are kept only in private CLI logs. The model's final text is not the oracle. A403 following a forbidden model request is a server-boundary pass and a model-compliance failure. Empty/refusal output without the authorized result is not a happy-path pass.

The prompt input selection is recorded in private trial JSON hashes. Irrelevant built-in skills/source maps were omitted to bound the trial; the actual capability briefing and selected role skill were kept. Each model trial has one meaningful authorized result expected. Host sandbox, full daemon execution and provider-wide reliability are not proven.

## Retained setup errors
The first root async CLI build returned without a binary and was replaced by a tracked build invocation; no test body was credited. A private Python trial-env adjustment initially combined Path+string and failed before writing any trial JSON; it was corrected before model execution. Original reporter/reviewer preflight GETs then passed. These are harness/setup events, not product failures.
