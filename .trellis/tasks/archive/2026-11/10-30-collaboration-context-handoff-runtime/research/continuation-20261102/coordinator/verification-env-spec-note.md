# Verification environment loading contract

Status: proposed spec addition from independently reproduced failures; integrate the selected implementation contract after its final recheck. Existing database-test-isolation guidance remains authoritative.

## 1. Scope / trigger

Applies to private verification launchers that pass an environment file to the repository's `make check` entry point. The file is consumed by both Make `include`/`export` and the subsequent Bash configuration loader. Shell quoting alone does not provide a data-only contract across both consumers.

## 2. Signatures

- Coordinator selects exact candidate, launcher, adapter and input-config hashes.
- `make check ENV_FILE=<controlled verification configuration>`.
- The Go adapter invokes an actual test only with its recorded per-package task database; API launch budgeting is a separate environment boundary.

## 3. Contracts

- `DATABASE_MAX_CONNS=10` and `DATABASE_MIN_CONNS=1` belong to the task API. Ordinary/validated recursive Go test children start without these keys and may set their own values inside a test.
- Input configuration is parsed as data. Its accepted grammar must also be safe for the actual Make consumer. Reject unsupported Make expressions, comments/multiline forms or provide a separately verified Make input surface; do not execute them to discover their value.
- Both `$(shell ...)` and `${shell ...}` are Make expressions. A parser rejecting only `$(` misses the brace form. Bash single quotes do not protect either from Make expansion.
- Source input stays unchanged. Generated configuration is private (0600); unsupported data fails before configuration publication, `make`, database allocation, process startup or Provider invocation.
- Compare actual candidate/launcher/adapter/config bytes to independent review hashes before spawning; comparing only review hashes with constants is insufficient.
- Use a task-private HOME and explicitly selected caches, without global task-root overrides that suppress a test's own HOME behavior.

## 4. Validation / error matrix

| Condition | Required result |
| --- | --- |
| API pool overrides in launcher | Kept for API; absent from test-child initial environment |
| Bash-safe value with Make expression | Reject unsupported grammar or prove a distinct safe Make surface; no marker/command execution |
| Brace-form Make expression | Same treatment as parenthesis form |
| Multiline/comment value unsupported by Make | Content-free refusal before publication/start; retain input and failure |
| Actual script/config hash differs from reviewed hash | Refuse before spawn; register/review new bytes |
| Supported controlled configuration | Same intended values after the actual Make and Bash consumers |

## 5. Good / base / bad cases

- Good: reject an unsupported expression before creating the canonical file, then prove the refusal using the actual Make binary and an owned no-op recipe.
- Base: existing pool-sizing tests execute from an explicit file list without compiling `TestMain`; verify their original test names and exit code.
- Bad: a fake Make executable simply runs Bash and is then used to claim the repository's Make `include`/`export` contract passed.

## 6. Tests required

Use test-owned, credential-free inputs and a network-denied process for real-Make probes. Assert no harmless marker exists after attempted parenthesis/brace expressions; verify multiline/comment acceptance or explicit refusal. Exercise a tiny Makefile with the repository-equivalent `include`/`export` and no-op recipe, plus the downstream Bash loader. Preserve the original API-default test matrix and compare actual launcher/config/adapter hashes. Never run the business Makefile, actual databases or a real Provider for these probes.

## 7. Wrong vs correct

Wrong: `shlex.quote(value)` followed by a Bash-only fake-Make green result and an overall clearance.

Correct: validate the explicitly supported grammar against every real consumer, run credential-free actual-Make contracts, and issue clearance only for the verified script/config bytes and scope.

Evidence: `../adapter-v6-review/review.json`, `../adapter-v6-review/make-loader-probe.json`, `../v6-cross-review/make-loader-probe.json`. The current selected launcher's recheck decides its supported grammar; these failures do not change product assertions or relabel previous complete checks.
