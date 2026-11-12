# Codex 0.166.0 bounded harness review

Date: 2026-11-05 Asia/Shanghai. Reviewer: codex_integration. Read-only diff from code baseline `429b1b6b4`; no builds/tests/database/CLI/model execution. Product adapter files are unchanged. Historical 0.165 final admission remains unchanged and does not admit this new test binary/version.

## Initial verdict

One fail-closed metadata issue requires correction before final static clearance. The bounded exact-version extension is otherwise consistent with the prior contract.

## Reviewed scope

Four tagged harness files: `rk1k_codex_registration_test.go`, `rk1k_codex_provider_test.go`, `rk1k_fixture_test.go`, `rk1k_codex_controls_test.go`. The initial diff has75 insertions/5 deletions. Registration admits only exact0.165.0 and0.166.0, not ranges, suffixes or automatic newer versions. Cap15, source/config/executable/test-binary hash checks, expiry, ledger and scenario schedule remain unchanged.

The fixture writes a private metadata binding before any Codex wrapper version probe. The wrapper reads the registration path and digest from that binding and passes it through the same full authorization/hash/version/expiry checks before reporting the registered version. It never runs a real executable to answer the wrapper probe. This is *registered version metadata*, not independent evidence of the native binary's version; root must retain an actual version/hash binding for the selected0.166.0 native binary.

## Required correction V1

`rk1kCodexReportedVersion` returns fake0.165.0 whenever `codex-version-binding.json` is absent, including a real-gated process. If the0.166 binding is removed or never materialized, daemon discovery can report0.165 while later registration admission still starts the registered0.166 executable. This breaks truthful invocation/version provenance despite retaining execution-hash safety.

When `MULTICA_RUN_REAL_AGENT_SMOKE=1`, absence of the metadata binding must refuse; fake-only fixtures with no real gate may keep the historical0.165 default. Malformed/unreadable bindings already refuse. Add a no-binding/real-gate control, retaining the fake-no-binding positive and valid bound0.166 positive. C and root were notified; no source edits were made by reviewer.

## Installed schema delta reviewed

`version0166-protocol.json` and `version0166-schema-delta.json` list11 nested differences. ThreadStartParams and ThreadResumeParams are byte-identical. Response top-level properties and the required native thread identity/ephemeral paths are unchanged. Nested additions are: `policyCapacityUnavailable` error enum, optional started/completed millisecond item timestamps, optional image-generation `generationId`, and optional AdditionalContextEntry.name. Image item required fields are unchanged. No observed change requires modifying the knowledge adapter or weakening RPC/thread/terminal checks. Schema compatibility is static evidence, not a successful live0.166 run.

## Validation needed after shared window release

- Compile the changed tagged harness and run the extended version registration/metadata controls, including V1, without real Agent execution.
- Rebind the complete current source/embedded/dependency input map, new test binary and CLI. Preserve prior0.165 results as historical; do not rewrite their hashes/version.
- Complete the required fresh final normal/race fake proof on current source; verify expanded named inventory, fixed15measured+3seed/0paid and artifact hashes.
- Prepare a new false registration using the actual installed0.166 native binary/version/hash and retained user model/account configuration. Recheck exclusive execution-window release, unchanged15-start authorization, empty ledger and new proof output before root enables the one-shot permit.

No extra user approval is required for the already authorized bounded continuation. Real start count remains0.

## Final static recheck

**PASS — V1 corrected.** `rk1kCodexReportedVersion` now refuses a missing binding while the real-agent gate is1; fake-only absence retains historical0.165. The new `missing_real_version_binding` control creates/deletes the binding under the real gate and requires refusal; `fake_version_unchanged` explicitly empties the gate. Existing invalid/expired/unauthorized/config/source/binary/executable/registration controls also exercise reported-version metadata, including exact0.166 positive and unreviewed/patch/suffix/blank negative cases. No remaining static blocker was found in this bounded four-file change.

Dynamic validation and new0.166 native/source/testbinary/CLI registration binding remain pending; the previous0.165 final admission is historical. No tests/builds/DB/CLI/model were run in this review. User authorization remains15 real starts, consumed0.

Reviewed final source hashes:
```json
{
  "server/cmd/server/rk1k_codex_registration_test.go": "3a15e4876c33232fe20eec5d0e0f0dc54434bb1272ccbca41ce9015e00179b00",
  "server/cmd/server/rk1k_codex_provider_test.go": "191c328a581652ea4385f9e5541e26ce701308bc175e053b2ed7fe497ed20978",
  "server/cmd/server/rk1k_fixture_test.go": "8f2f26773ccfb3d159a7cce22a0b334e0f6aa3a92e6c47712907ae60f5ea2b1b",
  "server/cmd/server/rk1k_codex_controls_test.go": "baf1b77d8dd9e9f3d3e76bf3c6c37ad965b6f616f5783b55e83620523762abf2"
}
```
