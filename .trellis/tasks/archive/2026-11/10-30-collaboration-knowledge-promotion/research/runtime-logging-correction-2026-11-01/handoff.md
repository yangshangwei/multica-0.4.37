# Runtime logging P2 correction handoff

Implementation is ready for main and independent runtime-review recheck. This document does not close the finding or G2.

## Exact changed files

- `server/pkg/agent/agent.go`: per-backend logging-only `Config.PrivateInput`; clarify failed diagnostic evidence.
- `server/pkg/agent/claude.go`: private stderr goes only to the existing bounded tail, not the generic logger; the same logger still records lifecycle metadata. The private tail uses the existing common-secret/control-character sanitizer. A knowledge refusal retains that diagnostic in existing KnowledgeOutput while Error remains the canonical code and failed Output remains empty.
- `server/internal/daemon/daemon.go`: set PrivateInput from the admitted ordinary knowledge descriptor at backend construction.
- `server/internal/daemon/collaborationrun/host_runner.go`: set PrivateInput from the admitted collaboration knowledge descriptor at backend construction.
- `server/pkg/agent/private_stderr_test.go`: test-created executable verifies private/legacy/private runs sharing one logger, bounded diagnostic retention, common-secret sanitation, empty failed Output, and retained start/finish logs.
- `server/internal/daemon/knowledge_context_stderr_test.go`: actual approved stdin echoed to stderr for success, provider failure and knowledge termination refusal; no generic-log body, bounded authorized evidence, unchanged successful output.
- `server/internal/daemon/collaborationrun/host_knowledge_stderr_test.go`: actual approved stdin echoed to stderr for success/provider failure; no generic-log body, retained bounded failed outcome and lifecycle logs.

No protocol, schema, host recording, or shared logger mutation. Main’s strict ACK and C4 implementation changes are preserved.

## Red evidence retained

`red-stderr.jsonl` contains the raw generic logger captures from test fixtures, not real user/model content. All five C3/C4 cases failed the private stderr assertion before the change; C3 knowledge refusal additionally failed diagnostic retention. `red-command.json` and `red-source-sha256.json` identify the run. The same two integration tests pass after the correction.

## Final verification

| Suite | Top-level passes | Pass events including subtests | Skips |
| --- | ---: | ---: | ---: |
| Affected agent regressions | 93 | 195 | 1 |
| Full ordinary daemon suite | 888 | 1676 | 2 |
| Full collaborationrun suite | 113 | 332 | 0 |
| Selected agent race suite | 114 | 229 | 1 |
| Selected ordinary daemon race suite | 56 | 256 | 0 |
| Selected collaborationrun race suite | 71 | 168 | 0 |

All failures: zero. Agent regressions ran in 35.077 seconds; full daemon/host command in 73.558 seconds; race command in 112.151 seconds. Commands and full inventories are in `green-regressions-command.json`, `green-race-command.json`, `green-regressions.jsonl`, `green-race.jsonl`, and `test-inventory.json`. Build, vet and whitespace checks passed (`static-command.json`, `static.log`).

Every executable test ran under the installed-CLI guard. No database packages/TestMain, database connections/writes, real Provider CLI, commits or subagents were used. The observed platform is macOS; the skipped platform tests are listed in the inventory. Source and dependency hashes stayed identical through final checks (`green-source-before.json`, `green-source-after.json`). `artifact-sha256.json` hashes retained evidence.

## Scope of the privacy policy

The raw stderr forwarding sink is disabled only for private admitted backend instances. The existing 2048-byte raw capture limit and common-pattern sanitizer preserve useful bounded diagnostics in the already-authorized failure/evidence path. Sanitization does not make arbitrary source text public-safe; the diagnostic remains private evidence. Legacy nonknowledge raw stderr behavior and process metadata logging are unchanged. This correction adds no execution authority, backend support, model understanding claim or G2 enablement.
