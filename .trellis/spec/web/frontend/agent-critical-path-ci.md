# Agent critical-path PR smoke

## 1. Scope / trigger
A separate pull-request workflow targets main and codex/cross-workspace-collaboration for server/Web/shared-package/e2e/launcher/config changes. It runs four existing ordinary agent journeys with SimulatedDaemon, not a provider process or paid model. This named check does not itself configure remote branch protection.

## 2. Signatures
`node scripts/agent-critical-path.mjs --config /absolute/private/config.json --output /absolute/new/run-dir`

Configuration version1, owner `agent-critical-path`, explicit `databaseName`, loopback `databaseUrl` and absolute installed Playwright `browserPath`. The file is user-owned0600; output is a new private directory outside the checkout. `scripts/agent-critical-path.sh` is the internal clean-environment runner; `check-agent-critical-path.mjs <playwright.json>` verifies actual inventory.

## 3. Contracts
Caller/CI allocates an isolated database. Validate actual current_database before migration or fixtures; never fall back to local multica, ensure_database, cmd_up or shared services. Reject ambient root/Web dotenv files. Generate the narrow child environment and private runtime/registry state; retain only explicitly selected tool caches. API auth is synthetic legacy; Web is a production build/start. Never run a real daemon or discover an installed provider.

Reuse dev-env's port allocation, process identity and lower start/stop primitives. Cleanup is registered before startup and owns only this run's API/Web; partial startup and failed cleanup fail the run. Keep the database/evidence for diagnosis. Compare source/config identity before and after; a changed source invalidates acceptance. Record actual rendered API/WS URLs, not just health.

Playwright selects exactly G1-01 assignment/result, G1-02 failure/retry/cancel, G6 comment/mention/note, and G8 running cancellation/late completion from their original two spec files. Chromium only, worker1, retry0, repeatEach1. Verify exact file/title/project identity and one passed result per case. Process exit0 alone is insufficient.

The evidence export is an explicit sanitized allowlist. Never upload the generated env, private registry manifest, database URL or signing key. Keep original local failures; each retry has a new run/output directory and database if fixture isolation requires it.

## 4. Validation / error matrix
| Condition | Required result |
| --- | --- |
| Missing/foreign/unavailable DB or absent executable/browser | Refuse before fixture writes |
| Ambient dotenv or invalid private configuration | Refuse; do not print secret input |
| Busy unrelated listener | Refuse; never reuse or kill it |
| Partial API/Web startup | Stop only own components; retain failure |
| Missing/skipped/failed/retried/duplicated/extra test or malformed report | Inventory failure |
| Four real first-attempt passes, unchanged source/config, successful cleanup | Smoke acceptance |
| Fake launcher/empty-body reporter probe passes | Contract preparation only, not product acceptance |

## 5. Good / base / bad cases
Good: fresh allocation, owned production services, four actual browser cases and unchanged identities. Base: local/hosted smoke share the same launcher and assertions. Bad: read a skipped report's exit0 as passed behavior, use next dev or reuse another checkout's API.

## 6. Tests required
Node suites exercise inventory failures, allocation/environment/redaction, missing DB, partial startup, process ownership, identity drift, retained evidence and workflow trigger/artifact contract. Validate selection and reporter format against the locked Playwright installation, then execute the actual four cases on production Web. Hosted execution stays untested until it actually occurs.

## 7. Wrong vs correct
Wrong: upload the entire runtime directory or call the broad dev-env up/destroy entry with an already allocated database.
Correct: verify exact allocation, reuse lower-level lifecycle helpers, export only sanitized evidence and retain raw private diagnostics locally.
