# S2 joint E2E authoring — increment 3

Owner: `/root/s2_e2e_resume`. Recorded: 2026-11-14.

**The bounded authoring increment is frozen for independent review. Actual scenario executions remain 0.** The coordinator accepted freezing the API-positive completion version now; connecting the final M1 shared completion form after client3d freezes is a required follow-up. This report does not accept A11/A12 or the whole S2 task.

## Scope and source

Five new files are frozen in the JSON companion and under:

`/Users/artisan/.multica/acceptance/pay241-20261114-resume/s2-e2e3/source-v2`

- `e2e/collaboration-stage-execution.spec.ts`: three separately bounded480-second scenarios.
- `e2e/fixtures/collaboration-stage-execution.ts`: explicit source setup, actual artifact API/tar verification and stage/M1 helpers.
- `e2e/fixtures/collaboration-stage-control.ts`: bounded, correlated private JSONL socket client.
- `e2e/fixtures/collaboration-stage-control.node-test.mjs`: private transport regressions.
- `e2e/fixtures/collaboration-stage-artifact.node-test.mjs`: independent USTAR and allocation checks.

The original five increment2 files, including the existing A01–A05 scenario and shared Host runtime/provider, remain byte-identical to their checked inputs. No server, Core/Views, platform, migration, dependency, specification or Git-index file was edited. The earlier five-file source freeze is retained; v2 changes only structured failure recording so native paths are not copied into scenario JSON. Original runner failures remain available in their raw logs.

## Authored scenarios

1. **Release and rollback:** public registration/invitation/membership and explicit artifact-reader grants; actual local Git objects and two physical tar builds; C1→prepared base Host→CLI T1/T2/T3→T4; human QA and Desktop release approval; QA release denial; lost human reply with original-key GET recovery; one unknown execution intent; no fact/advance/revocation shortcut; authenticated original-operation success; explicit A9; anomalous metric→rollback approval→equal-native-version contradictory observation→zero-call refusal; strictly newer metric and new human approval→one exact rollback to the observed immediately previous artifact. The original deployment, report and decisions remain retained.
2. **Late fact:** a real15-second human approval expires after one admitted unknown execution. An authenticated successful result is appended later. The original approval remains expired, A9 refuses, receipt lookup calls no adapter, and a new approval cannot endorse the old deployment fact. No new execution is requested merely to recover progress.
3. **Final writers:** missing-stage submission and self-completion under M1 off/observe/enforce, including an actual valid M1 authorization that remains unconsumed after stage refusal; project move/clear/delete refusal with M1 off so another gate cannot hide the stage assertion; an unmanaged empty-project delete control; actual valid deployment→verification→Web submission→exact M1 enforce authorization/completion/replay; and a separate unfinished root for disabled-creator R38 refusal. R38 uses M1 off independently and restores the synthetic creator afterward; its account-generation changes do not precede consumption of the main root's approvals.

The final positive completion in scenario3 currently uses the real authorization and lifecycle APIs, followed by Web/Desktop reads. **It is not evidence that the shared Web/Desktop M1 authorization form has been exercised.** The full actual writer inventory, transaction races and provider/default-constructor boundaries remain separate server evidence.

## Fixture boundaries

- Direct setup rows are ordinary producer runtime/agent/task prerequisites, synthetic PR/CI observations, distinct deployment/metric integration configuration identities, and the previously approved exact-new-account email bridge. Stage roles, membership grants, artifact requests/approvals/executions, reports, human stage decisions, execution intents/results and metrics use their actual public or frozen internal production paths. No approved artifact, stage approval, report, execution fact or metric row is seeded.
- Artifact inputs come from the exact fixture Git object, not its dirty working tree. The existing artifact-package API creates the request, a current human approves it, and the real local packager writes the tar. Validation checks its single USTAR entry, exact source bytes, header/checksum/padding/EOF and actual output digest. Synthetic controlled deployment/metrics remain distinct from real S4 systems.
- R/Q/L obtain their existing artifact-manager read authority through actual member PATCHes before signing. An ordinary source member remains a negative control: R1 can expose the child while R4/T4 withhold its private artifact/report evidence.
- The E2E connects only to the coordinator's existing private socket. It cannot send `shutdown`, start/stop the harness, register a public verifier or select production simulation. One explicit send gets one correlated response; disconnect/malformed/oversized response stays unconfirmed, and no prepare is automatically resent. A completed original ID may be reused for explicit receipt lookup.
- The scenarios require one Playwright worker and zero automatic retries. Every failed original run remains retained. Existing Host/CLI/heartbeat/poller timeouts, connection pools and cleanup semantics are unchanged.

## Required startup inputs

Use the already-frozen `server-increment-2c-control-transport.md`: harness stdout prefixes are stripped by the coordinator's bridge; the socket carries bare JSONL.

| Input | Exact requirement |
| --- | --- |
| `COLLABORATION_STAGE_CONTROL_SOCKET` | Existing canonical task-owned Unix socket in a0700 directory; socket0600; outside the checkout; at most103UTF-8 path bytes on Darwin. The coordinator owns its lifecycle. |
| `MULTICA_ARTIFACT_STAGING_ROOT` | Same canonical directory passed to the API packager, fixed0500 before API construction; never chmod or replace this root during the run. |
| `MULTICA_ARTIFACT_OUTPUT_ROOT` | Same canonical private0700 output root supplied to the packager. |
| `COLLABORATION_STAGE_ARTIFACT_SLOTS` | JSON array of eight distinct nonzero canonical UUIDs. The coordinator creates all eight generation directories0700 before making the staging root0500. |
| Generation use | Scenarios use slots0–6; slot7 remains spare. Refuse an existing manifest/files directory or changed owner/mode. Write only the selected generation, then files0400 and generation/files directories0500. Preserve them and all outputs afterward. |
| Existing stage environment | Final source-matched API/CLI/production Web/Desktop, explicit Git and report-capable **base** Host provider, actual stage-policy activation after2d, owned database and parent-supplied synthetic administrator. |

Mail and real GitHub refresh credentials must remain disabled in the synthetic API as specified by the handoff. The original gate booleans are captured/restored, epoch remains monotonic, and only scenario-owned daemons, browser contexts, Electron fixture and SQL clients close. The parent separately stops the long-lived stage harness and bridge.

## Verification and retained failures

| Check | Result |
| --- | --- |
| Final pure fixture tests |11 PASS,0 FAIL/SKIP: nine private socket/protocol tests and two artifact checks. The reference tar is generated independently by Python's standard USTAR writer. |
| Final scoped lint | Five files actually reported by ESLint,0 errors/0 warnings, from repository cwd without suppressing ignored-file warnings. |
| TypeScript on declared checked3c reconstruction | PASS:2,526 checked client files plus17 explicitly sourced E2E files,2,543 bound files total. No live3d Core/Views file is imported. |
| Playwright discovery only | Four scenarios in two files: the unchanged original scenario plus three new ones. No browser or scenario body ran. |
| Original file/source audit | Original five files unchanged; new five current/source-v2 hashes match. |

Preserved failures and limitations:

- `control-red-1`: the new transport implementation did not yet exist; this is authoring initialization failure, not a product failure.
- `control-green-1`: a long private TMPDIR exceeded Darwin's Unix-socket path limit. Four fixture chmods failed after the OS truncated the bind pathname. The failed fixture process was terminated only after its exact owned PID was checked; the parent test process then exited. No product process was involved. Later tests use a short canonical private `/tmp` directory and register cleanup before listening. The client rejects overlong paths before connecting.
- The initial source for those two files was reconstructed from this agent's recorded `Add File` tool inputs and labelled as such; it is not presented as a contemporaneous pre-run snapshot.
- `typecheck-1`: three unused symbols; corrected within the new E2E files. `lint-2`: one unnecessary regex escape; corrected. Earlier `lint-1` is not used as a gate because its package cwd and suppressed ignored-file warnings did not prove the intended file scope.
- `typecheck-final-1`: the current shared graph failed in the other lane's in-progress `packages/core/api/client.ts` completion field. The failure remains. The coordinator authorized a checked3c reconstruction plus the preserved increment2 inputs and matching initial-workspace E2E dependencies. Every supplement is declared; no live3d or parallel knowledge-automation source was copied into that baseline. A final candidate graph must be verified again after3d stabilizes.

Raw logs, direct exit sidecars, exact argv, reconstruction manifests and hashes are in the private evidence root and JSON companion. Counts across reruns overlap and are not additional acceptance cases. No database, product API, browser, daemon, real model, real Git fixture or external provider was executed by this authoring lane; the only live transports were private fake Unix-socket unit fixtures.

## Required follow-up

The same E2E owner will reconnect scenario3 to the **frozen** client3d M1 control and rerun static/independent checks. The client's current mechanical draft names region `Completion authorization`, checkbox `Use completion authorization`, source/artifact selectors, `Authorization reason`, `Use latest expiry`, `Authorize exact completion`, and final `Accept and complete` / `Record result and complete`. These labels were supplied during implementation and are not yet a verified interface in this freeze. The authorization's operation ID must be the final existing workflow idempotency key.

After server2d and client3d finish: bind the final full source/build/configuration, run all four authored scenarios and the complete A01–A12 evidence plan, retain every configured failure and its denominator, then perform independent phase review. Parent acceptance and all actual external NOT_RUN items remain open.
