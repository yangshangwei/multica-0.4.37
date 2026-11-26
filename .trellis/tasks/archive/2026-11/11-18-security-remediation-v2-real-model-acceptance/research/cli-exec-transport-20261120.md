# Official Codex exec transport

The user explicitly changed this trial to the official direct CLI surface. The new task-only `codex_exec_container.py` runs the unmodified image's `codex exec --json --skip-git-repo-check --output-last-message <task-home-file> -`. It does not launch app-server, select a clientInfo, inject User-Agent, rewrite model messages, or translate SDK protocol. The generated AGENTS.md and raw prompt stdin are preserved and hashed; native stdout JSONL is forwarded unchanged. Product app-server acceptance remains a separate unproved claim.

The only accepted external argv is `exec --json --skip-git-repo-check -`, or `exec resume --json --skip-git-repo-check <exact UUID> -`. The resume UUID must equal the actual manifest's resume_session_id and have an owned retained rollout. `--last`, arbitrary configuration flags and sandbox-bypass arguments are refused. Official installed0.164 help was inspected offline; both initial and resume entries expose --json and --output-last-message.

Readonly role containers retain the earlier exact task CLI spool, original mat_, per-run request/response mounts, immutable Codex configuration/managed requirements, explicit internal model network, no user HOME/account mount, capability drop and nonroot execution. This first direct CLI wrapper refuses writable roles. Shared helper imports preserve the earlier no-follow session retention and registration checks without editing frozen app-server source. A real configuration must name transport_mode=official_cli_exec and cannot select a fake endpoint launcher.

The wrapper records a durable start before forwarding prompt stdin, applies the registered technical watchdog and a64MiB stdout limit, retains the native event stream and last-message file, and requires exactly one valid native thread UUID and one turn.completed without top-level failure. The final native agent message must equal the official last-message file. Missing or inconsistent terminal evidence is incomplete even after process exit0; this is runtime evidence, not platform/semantic acceptance. Usage is copied from the native terminal without inventing values.

## No-provider verification

Four pure controls pass: exact argv and explicit resume shape; actual terminal/usage extraction; missing/failed/duplicate terminal refusal; invalid JSON/item/thread refusal. Python AST and whitespace checks pass.

`probe_exec_transport.py` launches the actual wrapper and official Codex exec inside Docker network=none. A hash-bound fake Responses server runs in that same container and sends scripted native tool calls, then a final message. It is never a real-model or supplier request. The fake endpoint's launcher is rejected in real mode.

Final r4 passed:5 synthetic Responses requests, original-task CLI API exactly once, untouched prompt/AGENTS, one completed native turn and matching final-message file. Observed native User-Agent was `codex_exec/0.164.0 (Debian 12.0.0; aarch64) unknown (codex_exec; 0.164.0)`; this value came from the official program, not the test. Control-plane localhost peer returned200 before shell socket access was denied. Native shell could not modify code, AGENTS, task config or managed requirements. A scripted `sandbox_permissions=require_escalated` command was rejected by policyNever. No supplier account or remote model was accessed.

Raw evidence outside repository: `/Volumes/artisan/code/2026/secrem-v2-cli-evidence-20261120/exec-probe-r4/`. Summary+hashes: cli-exec-native-r4-20261120.json. r1 setup failed because the fake endpoint was not a private file; r2's socket constructor was denied before the negative probe's try block, so its oracle stayedfalse; r3 passed the base route; r4 added file/escalation assertions. Prior failures remain unchanged.

Remaining limits: actual direct CLI provider permission and behavior require the separately reviewed real run; resume execution has only argument/store controls so far; readonly peer/mount constraints share earlier helper proof and the final real environment must remain bound. C1/D1 writable execution is not admitted by this wrapper.


## Successor: no-repository scratch and declared CLI discovery

The first actual direct P1 consumed the earlier readonly-cwd wrapper and reached a real model, but the platform instructions require writing a UTF-8 comment file inside the work directory. That wrapper also blocked declared CLI --help and the workspace squad inventory. Preserve that model result as a harness/environment failure; this successor changes no template text or expected business conclusion.

An actual claim may now set no_repository=true only with repo_count=0 and project_resource_count=0. The wrapper refuses an undeclared source file, a nonempty Git directory or aliases. For this narrow mode the task-owned cwd is writable scratch while AGENTS, agent context, daemon task metadata, reserved credential/config/Git directories and any hash-bound immutable fixture files are separate readonly mounts. Skills in the task Codex home are readonly. `.multica/comments` can hold result files. A claim with an actual repository remains readonly in this wrapper; no source-editing capability is granted by scratch mode.

The broker now accepts --help only after an exact declared command/group prefix and with no positional/extra flags/stdin, so it cannot smuggle a write or global account override. `squad list --output json` reaches the real CLI with the original task token and fixed workspace; restricted reviewers still cannot inventory squads. Eight broker controls and six direct-exec controls pass.

Actual native scratch r1 passed with9 synthetic Responses requests inside network=none: help issued no API request; original task/workspace issued GETissue, GETsquads, and one POSTcomment carrying exactly the UTF-8 bytes written through `.multica/comments/handoff.md` and --content-file. Six immutable input/config file writes were refused; live control-plane peer was reachable before native shell networking was refused; escalation remained rejected by Never. Full prompt and AGENTS unchanged. No supplier/model call occurred in this probe. Summary `cli-exec-scratch-r1-20261120.json` and private raw `exec-scratch-r1/` retain hashes.

Final successor wrapper SHA256 f4fc42205e08f80f6f86908a1509c55fc51c3a64c1e55bdde83cb6557099bfd9; broker SHA256 eaadfae12b9ce9aba4cb01c556bbaec7c6c5099ca0644211bbe185674def18e5. Actual direct-run predecessor bytes are retained in the private source-history/transport-before-scratch directory. The initial readonly proof and this scratch proof are separate evidence; no historical verdict is rewritten.
