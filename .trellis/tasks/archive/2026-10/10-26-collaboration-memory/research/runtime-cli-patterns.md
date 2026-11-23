# Research: runtime/CLI patterns for collaboration memory

- **Query**: host launch inputs, runtime API gating, capability negotiation, CLI structure, fixed instruction text, squad members, Linux path
- **Scope**: internal
- **Date**: 2026-10-26 (tree at 1868ff776; host files committed in 8bfc84ad1, clean at read time)

## 1. Host path: claim → preparation → permit → launch

- Server claim `service/collaboration_runtime_capabilities.go:125` → `preparationForClaim` (`collaboration_runtime_preparation.go:145-220`) → `claimPromptSnapshot` (`collaboration_runtime_capabilities.go:279-315`). **Freeze point**: first claim only (`PromptSnapshot == nil`). Inputs = group's in-flight commands (`:284-293`); `freezePreparedPrompt` (`collaboration_runtime_preparation.go:325-367`) builds the manifest (locked agent invocation, attachments, git, limits, host capability + digest) and writes both via `FreezeCollaborationPreparedRunPrompt` (`pkg/db/queries/collaboration_preparation.sql:6-11`, requires both NULL and `authorized_attempt_seq=0`). Later claims re-read and recheck (`readRuntimePreparation` `:369-401`).
- DB: `prompt_snapshot` is a JSON array (`migrations/717…up.sql:75-76`), immutable once set (`:100`); `preparation_snapshot` ≤98,304 bytes, immutable (`737…up.sql:7-9, 33-37`).
- Wire: `CollaborationRunClaimResponse` (`pkg/protocol/collaboration.go:60-71`) = identity, inputs, agent/owner/workspace IDs, termination token, `Preparation`, `HostContext` (`collaboration_host.go:31-38`, filled at `collaboration_runtime_capabilities.go:264-269`).
- Daemon: `Runner.Run` sends v3 to `runHost` (`collaborationrun/runner.go:51-53`). `host_runner.go:31-218`: Claim `:45`, claim stored in ledger (`host_ledger.go:35`), session lock `:58`, permit `:92` (`authorize` `:226-261`), prepare `:97`, brief `:128-129`, env `:133`, prompt `:160`, execute `:169`; the fresh-session retry rewrites the brief and prompt (`:170-185`). Each launch goes through `BeforeLaunch` and gets a new permit (`:263-338`).
- `hostBrief` (`host_runner.go:462-471`): four fixed lines plus `Preparation.Invocation.Instructions` (plus the continuity notice). Written by `execenv.InjectCollaborationBrief` (`execenv/collaboration.go:23-36`) into CLAUDE.md/AGENTS.md/… (`runtime_config.go:197-222`) as a marker block (`:247`). The work dir `<Directory>/<sessionKey>/work` is reused across runs that share a session key (`host_preparation.go:46-47`).
- `hostPrompt` (`host_runner.go:473-493`): a fixed sentence that already says "Read the issue's shared context before acting" (`:488`), plus JSON `{issue_id, agent_id, agent_owner_user_id, resource_owner_user_id, parent_run_id, inputs, attachments, git}`. `SystemPrompt=""` (`:159`).
- Limits and validation: claim body ≤2 MiB (`client.go:258-264`), strict decode with unknown fields rejected (`ledger.go:478-493`, `collaboration_preparation.go:178-192`). `validateClaimResponse` allows 1–1024 inputs (`client.go:98-136`). Manifest shape (`collaboration_preparation.go:347-445`): command_ids ≤1024, instructions ≤64 KiB UTF-8 with no NUL (`:18, 319-340`), attachment limits (`:263-268`), SHA-256 manifest digest (`:397-416`). Daemon prepare rechecks digest, provider, host, git and limits (`host_preparation.go:41-44`). Ledger file ≤4 MiB (`ledger.go:71`). Human run records project `prompt_snapshot` inputs (`service/collaboration_records.go:195-224`).

## 2. Runtime business API (`cmd/server/router.go:1399-1417`)

| Route | Token | Service |
|---|---|---|
| POST outcome | mcb_ | `RecordRuntimeOutcome` capabilities.go:411 |
| GET context | mcb_ | `RuntimeContext` collaboration_squad.go:146 |
| GET preparation, GET attachments/{id}/content, POST preparation/observations | mcb_ | preparation.go:403, 415, 452 |
| POST comments / squad-activity | mcb_ | squad.go:205 / 370 |
| GET stop-intent, POST proof, POST launches/{id}/termination | mct_ | capabilities.go:565, 589; host_runtime.go:214 |
| list, capabilities, claim, permit | mdt_ + DaemonAuth | router :1409-1417 |

- Handlers: bearer must be prefix + 64 hex (`handler/collaboration_runtime.go:198-215`). Body must be JSON, ≤96 KiB (outcome 2 MiB), unknown fields rejected (`:47-74`), plus per-type forbidden fields (`:76-121`).
- `lockRuntimeBusiness` (`collaboration_squad.go:84-106`): token must not be revoked and its `attempt_seq` must equal `authorized_attempt_seq`, else 403 `business_capability_required`. Then `currentAdmission` and `requireReservation` (`:108-121`).
- `currentAdmission` (`collaboration_runtime.go:336-425`) returns these errors:
  - settled run or forced disposition: `execution_stopped`
  - gate off: `collaboration_execution_unavailable`
  - issue terminal or not admitted: `issue_closed` (`:347-349`)
  - target held: `target_held`
  - queue, expansion seal, settled group or batch state: `execution_stopped` (`:353-356`)
  - epoch, grant, binding or deployment mismatch: `stale_authority` (`:367-372`)
  - input authors no longer valid: `stale_authority` (`:298-334`)
  - revoked grants: `grant_required` via `collaborationRuntimeTargetSupported` (`:34-82`)
  - installation not accepting: `installation_stopped`
- Stop revokes every mcb_ of the run (`collaboration_control.go:701-706`); proven completion also revokes (`collaboration_runtime_capabilities.go:710`). `lockRuntimeBusiness` does not compare fence or claim_generation; outcome (`:447-459`) and host permit (`collaboration_host_runtime.go:109, 127, 134`) do. The CLI rejects a `/context` whose identity differs from its env identity (`internal/cli/collaboration.go:201`).
- Comment add: body `{comment_id: <request-id>, content, parent_id?}`, and the comment ID is the request UUID (`collaboration_squad.go:237`). Idempotency uses event `"comment:"+id`: the same input replays the saved receipt, a changed one gets 409 `comment_conflict` (`:219-230`). Content must be non-empty and ≤64 KiB (`collaboration_mentions.go:118-121`). Squad activity is idempotent by `event_id` (`collaboration_squad.go:386-393`).

## 3. Capability and version negotiation

- The collaboration path has no CLI-version gate. `CheckMinCLIVersion*`/`Min*CLIVersion` (`pkg/agent/version.go:24-118`) serve quick-create only and read runtime metadata `cli_version` (`handler/issue.go:2841-2875, 2896`). The daemon collaboration client sends no version headers (`collaborationrun/client.go:245-249`).
- Negotiation is protocol_version 2/3 + containment (`collaboration_host.go:136-139`) + preparation_version 1/2 + the registered capability.
- Declaration: `HostConfig.capabilityShape`/`Capabilities()` (`host_config.go:85-127`). The daemon registers each poll (`daemon/collaboration.go:276-289`) via `RegisterRuntimeCapabilities` (`collaboration_runtime_preparation_capabilities.go:83-143`), stored in `agent_runtime.collaboration_capabilities` (≤16 KiB, `737…up.sql:4-6`). A change advances the config epoch (trigger `:14-30`).
- The schema is closed: exact versions and fields (`collaboration_preparation.go:297-317`), stored copy decoded strictly (`preparation_capabilities.go:24-45`).
- Per-run checks:
  - claim digest (`collaboration_runtime_preparation.go:194-199`)
  - `Host` frozen into the manifest (`:343`), compared on every read (`:396-398`)
  - daemon prepare (`host_preparation.go:41-44`)
  - `BeforeLaunch` digest (`host_runner.go:267-277`)
- `execution_view_version` 5/6 is human-client read/write negotiation only (`handler/collaboration_execution_version.go:13-47`).

## 4. CLI

- There is no `multica collaboration` command (root list `cmd/multica/main.go:77-98`). Root `PersistentPreRunE: guardCollaborationCommand` (`main.go:33`).
- Allowlist `collaborationCommandAllowed` (`cmd_collaboration.go:14-21`); the operation is the command path (`:28`). Arg 0 must equal the admitted issue UUID (`:36-38`).
- Flag allowlist via `Visit` (`:39-54`): output and debug everywhere; comment add also allows content, content-stdin, parent and request-id; squad activity also allows reason and request-id. A canonical non-nil UUID request-id is required (`:55-61`).
- These are re-routed ordinary commands: `runCollaborationCLI` is called from `cmd_issue.go:874, 1876, 2046` and `cmd_squad.go:465`; `--request-id` flags at `cmd_issue.go:619` and `cmd_squad.go:556`. Ordinary clients refuse inside a run (`cmd_agent.go:255-256, 429-430`).
- `runCollaborationCLI` (`cmd_collaboration.go:65-150`) GETs `/context` before every op, writes included (`:109`). Bodies go through `resolveTextFlag` (`cmd_issue.go:45-102`):
  - `--content` is backslash-unescaped
  - `--content-stdin` reads all of stdin and trims one trailing newline
  - `--content-file` exists but is not allowlisted
  - `guardLocalPathLinks` is applied (`cmd_collaboration.go:87`)
- The unknown-result retry message is at `internal/cli/collaboration.go:207-224`.
- Identity: five env vars (`internal/cli/collaboration.go:19-25`); context ≤4096 bytes, mcb_ token, v2 bare vs v3 wrapper (`:46-84, 134-155, 237-249`). `/context` is decoded non-strictly (`internal/cli/client.go:276`).

## 5. Fixed text and docs naming the CLI

- Host fixed text: `hostContinuityNotice` (`host_runner.go:20`), `hostBrief` (`:462-471`), `hostPrompt` prefix (`:488`).
- The Linux path has no fixed brief. Instructions go in `--append-system-prompt` and the `ProviderInput` JSON is the user frame (`preparation_linux.go:340-355`, `pkg/agent/claude_codec.go:20-35`).
- Docs listing the commands:
  - `builtin_skills/multica-working-on-issues/SKILL.md:19-29`
  - `references/collaboration-workflow.md:133-181` (commands `:146-154`, unsupported list `:163-165`)
  - `references/working-on-issues-source-map.md:38-39`
  - specs `collaboration-runtime-cli.md:15-36`, `collaboration-host-execution.md`
  - tests `cmd/multica/cmd_collaboration_test.go`, `internal/cli/collaboration_test.go`

## 6. Squad members

- Members and continuations are separate `collaboration_run` rows (`createDerivedRun`, `collaboration_squad.go:316-343`), each with its own claim, termination token and per-attempt mcb_ (`collaboration_host_runtime.go:187-199`).
- The host session key adds group and step key for members (`host_session.go:17-44`).
- Business endpoints do not check step kind. Squad activity requires a squad target (`collaboration_squad.go:379-381`). A member comment queues the leader continuation (`:271-276`). `/context` fills `dispatch_comment` for the member (`:187-200`).

## 7. Old Linux namespace path

- Inputs: `runner.go:73-180` builds `ProviderInput{Identity, Inputs, Preparation}` (`:102`; struct `controller.go:36-43`). It is serialized into the helper request, ≤2 MiB (`controller.go:169-183`), decoded strictly (`controller_linux.go:62-74`), then sent as `/agent` stdin (`:121-131`) or as the Claude frame (`preparation_linux.go:348`).
- Mode split: the server uses `run.Containment` and the host gate (`collaboration_runtime_capabilities.go:158-180`); v2 claims carry no HostContext or protocol_version (`client.go:110-111`); the controller is Linux-only (`controller_unsupported.go:1`).

## Caveats

- Line numbers can drift if a peer edits these files.
- Not read: host_ledger registration internals, the full record projection, and the effective `/context` response size bound.
