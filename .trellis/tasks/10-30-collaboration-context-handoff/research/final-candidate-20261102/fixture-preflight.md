# Final candidate fixture preflight

Candidate `7f8ca87252b53f16041485ac867e501962377802`. Result: **no blocking missing prerequisite identified by static inspection**. Root's complete `make check` was still running; this is not an execution or acceptance pass.

The check loads the source environment before deriving the allocated Web and native renderer origins (`scripts/check.sh:131`, `:132`, `:93`, `:94`), then starts its own API and production Web. The private pnpm adapter injects the allocated collaboration/changelog API and matching candidate renderer before Playwright imports the specs. Both renderer selectors were already configured before preparation. The exact source CLI, Main/preload/renderer outputs and task catalog directories exist; CLI/build hashes match the candidate build metadata. MCP API/test directories are identical. No checkout dotenv file can replace the task values. Knowledge context is false and direct LLM configuration is blank.

The 147-spec inventory scan and focused caller review found no additional password helper that would run unguarded in this legacy lane. Offline/runtime handoff, retained/source-surface and managed control callers have their password guard before setup. Host, cross-client and installed selectors are absent and gated. Creation migration744 defaults human creation on; knowledge human-UI cases explicitly set and restore only their dedicated gate values. Mounted skill templates append to the embedded catalog, so an empty task directory does not remove built-in templates (`server/internal/service/skill_template_dir.go:49`).

| Boundary | Current static expectation |
| --- | --- |
| Optional compiled CLI | `MULTICA_E2E_CLI_BINARY` is unset. `upstream-backports-cli.spec.ts:160` and `cli-forbidden-reason.spec.ts:80` skip; the source-surface `COLLABORATION_SOURCE_CLI` variable does not activate them. A configured binary must resolve inside this checkout's `server/bin`. |
| Password/managed controls, runtime and installation | Remain separate required gates; an unfiltered legacy command does not execute those configured journeys. New runtime password guards are present before fixture setup. |
| Live changelog publication / auxiliary providers | Reader/native API wiring is present, but publication is guarded by the absent `CHANGELOG_E2E_FEED`; optional assistant/recommendation fake providers are also not selected. |
| Knowledge-on injection | `collaboration-knowledge.spec.ts:20` explicitly retains disabled execution. Human review/correction UI evidence cannot close injection with the knowledge switch on. |

No interruption, extra test, provider lookup or product/configuration change is recommended from this inspection. Record the actual final skip inventory separately and run the necessary gated lanes after the complete check. Presence/hash facts, precise references and source hashes are in `fixture-preflight.json`; private environment values were not copied.

Only these two report files were written. No tests, browser, database, service, installed Agent CLI, Provider, external operation or git mutation ran from this child.
