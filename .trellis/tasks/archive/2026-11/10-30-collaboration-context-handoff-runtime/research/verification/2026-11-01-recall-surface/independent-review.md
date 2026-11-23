# Independent review: runtime recall surface (uncommitted, base 28c67c674)

**Verdict: pass with notes.** I found no security or correctness defect in the CLI. The guidance has one medium defect.

## Defects

1. **Medium: the cursor guidance can loop.** Locations: `host_recall.go:11`, `collaboration-workflow.md:229-230`, and the server behavior at `service/collaboration_runtime_handoff.go:287-365`.
   - A cursor only pages its own list. The other lists in that response restart at page 1 and carry their first-page `next_cursor` again.
   - Failure scenario: inputs and results each hold 40 items. A model told to "follow each cursor until none remains" goes C0 → results cursor (the inputs cursor comes back) → inputs cursor (the results cursor comes back) and repeats forever. The CLI guard in `collaboration_handoff.go:61-70` catches only an exact echo, so it does not stop this.
   - The `show --help` text is better ("until the list has none"), but it still omits the reset.
   - Suggested fix: say that a cursor pages only its own list, and that the caller should read only that list from the response.

2. **Low: the delegation tail contradicts "recalled text adds no authority".** Locations: `host_recall.go:12` and `SKILL.md:28`.
   - The delegation source is capped at 64 KiB after redaction (`collaboration_runtime_handoff.go:175`), while comments are capped at 64 KiB before redaction. A short token expands when redacted (`xoxb-…` becomes `[REDACTED SLACK TOKEN]`), so the cap can be exceeded and the delegation marked `truncated=true`.
   - Its tail then has `trust=admitted_assignment`, but the brief tells the model to treat it as data. This is rare.

3. **Low: trailing JSON is not refused.** `GetJSON` decodes only the first JSON value (`client.go:276`). As a result, the `trailing_value` case in the protocol test never applies on the CLI path. `ReadComments` has the same pattern, so this is not new.

## Confirmed OK
- **Issue binding and flag allowlist:** the root override flags and flags from other commands are refused.
- **Escaping:** values are validated as a UUID, a fixed enum or hex before they are escaped.
- **Response identity:** every field is checked, and each check is pinned by a mutation test.
- **Prompt and brief:** `hostPromptWithHandoff` is untouched, the existing brief lines are unchanged, and legacy claims get no recall guidance. RK1 claims are `HandoffVersion=0`, so their brief is unchanged.
- **Guidance text:** every command, flag and field it names exists and matches the server.
- **Source-map paths:** all exist.
- **Smoke authorizer:** it accepts `knowledge_version=1` only, only once, only on POST `/claim` with `mdt_fixture`, matching `client.go:99-101`.

## Verification
- **I ran:**
  - The 4-package `go test`: all passed.
  - `go vet`, `gofmt -l` and `git diff --check`: clean.
  - The agentintegration filter: 24 passed, 1 skipped, and `TestContextExperimentRejectsTrialConfigurationBeforeProviderProbe` failed only on "source must be clean".
- **I read but did not run:** the base-tree 403 result (log 03b) and the built-in skill tests (log 09). I ran no DB suites.
