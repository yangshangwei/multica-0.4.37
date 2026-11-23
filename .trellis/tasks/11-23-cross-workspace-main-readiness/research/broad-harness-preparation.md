# Broad harness preparation

Task-only extensions are implemented in private `core/core-run.py`; no repository product or fixture file changed in this preparation slice.

- `TASK_DATABASE_NAME` reflects the new per-attempt database, and native endpoints use both collaboration/changelog fixture conventions.
- The start/end source manifest hashes all tracked and nonignored untracked apps/packages/server/scripts/e2e/workflow inputs. A drift receipt makes the attempt non-successful. Renderer/preload, binary, harness and selected-spec identities are retained.
- `--provider assist|recommendation` launches the repository's deterministic fixture on a task-owned port before API startup and configures only that API; no inherited provider credentials are admitted.
- `--admin` requires password mode, creates a synthetic registered account, runs the explicit server bootstrap command for its UUID, and keeps credentials private (0600). Fresh per-attempt DB avoids role/re-auth interference.
- `--cli` creates a verified copy of the built task CLI inside the private checkout's `server/bin` to satisfy existing fixture containment, then removes only that matching copy.
- `--catalog` allocates a private MCP template directory. `--changelog` copies the embedded feed under the fixture-required private-checkout acceptance directory and verifies restoration.
- The command no longer forces tracing over specs that intentionally disable it for authentication-sensitive flows.
- Each actual environment performs `--list --reporter=json` separately before execution; its collection output cannot overwrite actual execution JSON.

Verified by `python3 -m py_compile`, argument help, and a read-only source snapshot of 9,334 inputs (1.5 seconds). No services or builds were started. Dynamic provider/admin admission remains to be tested after the parent's source/build-ready signal.

Suggested broad default after build:

```sh
python3 core-run.py run --mode legacy --native --provider assist --cli --catalog --changelog --label final-default
```

Recommendation-only cases run separately with `--provider recommendation`; the two provider contracts cannot share one configured API LLM endpoint. Conditional gates and special allocations not configured remain explicit skips.

Harness SHA-256 at preparation: `842321fcac6bb5eb5fcceeb403f24d9a2e9122f56622d68d941a1deebd7f88be`.
