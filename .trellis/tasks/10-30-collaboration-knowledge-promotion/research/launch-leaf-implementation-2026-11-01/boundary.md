# C1 launch leaf boundary

Date: 2026-11-01. Owner: knowledge_launch_leaf. No commits, Provider calls or database access.

The existing launch observer records native processes but filters ordinary task credentials and does not bind raw Claude session observations to the command that produced them. C1 adds an explicit ordinary environment policy, immutable per-command intent/admission identity, synchronous native-session observation, and bounded effective-invocation validation. A changed or unrecordable native identity seals the lifecycle and preserves actual process/output/usage facts.

Owned files: `server/pkg/agent/agent.go` (options/result contract), `launch_lifecycle.go` (existing start/stop/wait gate), `claude.go` (actual raw system/result events), new adjacent knowledge helpers/tests, and this evidence directory. Existing native process management and collaboration environment filtering are reused. The existing collaboration constructor keeps its behavior; ordinary callbacks cannot add or replace any environment entry.

Only the admitted built-in local Claude stream-json adapter is eligible. Custom profile executables, other families and remote modes remain unsupported for new knowledge. The new argument guard is reusable by integration, but a generic blacklist cannot prove arbitrary custom/remote executable semantics. Unsupported ordinary execution and proven legacy history remain the integration owner's decision; nil knowledge options keep the existing leaf behavior.

Configuration identity and scope generation remain distinct from a claim generation. A separate invocation fingerprint binds app-managed path/version/cwd/options; it does not claim hermetic host configuration or vendor-version rollout proof. Actual stream IDs, never requested IDs or final-result fallbacks, establish an observed native session. No new vendor `--session-id` semantics are assumed.

Excluded: preparation/authorization HTTP or SQL, protocol DTOs, pointer persistence, daemon retries, source policy, UI, knowledge prompt injection, G2 enablement and all other agents' files. Leaf fake-executable regressions do not substitute for ordinary-entry service/daemon integration tests.

Verification: fail-first fake executable tests, existing collaboration lifecycle and Claude regressions, race checks, gofmt, vet and build. No tests may resolve an installed agent CLI. Capture source hashes and actual command outcomes in this directory.
