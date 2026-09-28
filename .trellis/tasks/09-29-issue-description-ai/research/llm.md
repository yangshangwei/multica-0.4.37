# Backend research

- `server/pkg/llm/client.go` is the required entry point for server-owned AI calls. Existing `GenerateJSON` fixes JSON-object output and uses the configured deployment default model. It has bounded transport retries and an HTTP client seam; no new SDK or agent runtime is needed.
- `Handler.LLM` is constructed from `MULTICA_LLM_API_KEY`, `MULTICA_LLM_BASE_URL`, `MULTICA_LLM_DEFAULT_MODEL`, `MULTICA_LLM_MAX_RETRIES`. With neither key nor URL it is disabled and must make zero upstream requests.
- Existing consumers: `internal/handler/chat_title.go` and `internal/service/chat_quick_actions_generate.go`. The latter demonstrates deadlines, concurrency admission and structured response validation.
- API routes use `/api/issues` with workspace selection by headers and `RequireWorkspaceMember`. Register `/optimize-description` here; the earlier design's workspace URL is conceptual, not the actual API convention.
- `pkg/llm/outbound_contract_test.go` inventories consumers and forbids SDK imports elsewhere. Add the new consumer to that inventory, package documentation, `.env.example`, and both current environment-variable docs locales.
- Goldmark is already a direct dependency. It can verify preservation of code and link/image destinations before returning suggestions.
- Tests should use a local fake completion HTTP server, existing `testutil.Call`, actual membership fixtures, and never use account credentials or installed agent CLIs.
