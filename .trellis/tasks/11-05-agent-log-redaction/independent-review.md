# Independent agent output redaction review

Reviewer: native `trellis-check` (`p1_independent_check`). Worktree: `/Volumes/artisan/code/2026/mc-agent-rnd-p1-20261105`. Reviewed the five implementation files and dedicated output-redaction spec after implementation completed; source hashes in `research/independent-reviewed-sources.json` are the final reviewed bytes.

## Findings fixed

**Existing collaboration-capability boundary coverage regressed.** The implementation moved the old `mcb_`/`mct_` regex into the new scanner, whose `tokenByte` includes hyphen. A capability in `value-<capability>` or `<capability>-suffix` was masked by the original word-boundary regex but retained by the new generic scanner. Reproduced using only synthetic capabilities and the actual package; saved `independent-capability-boundary.go.txt` and `.log` record `old_masks=true current_masks=false` twice.

Restored the existing collaboration regex in `patterns` and removed those prefixes from the new token scanner and its now-unreachable placeholder switch. Added `TestRedactCollaborationCapabilitiesPreservesWordBoundaries`: both capability prefixes next to hyphens, adjacent capabilities, idempotence, and embedded-identifier controls. New PAT/daemon/task/plugin boundary policy stays unchanged. This is a local compatibility repair, not a broader token-pattern redesign.

## Findings not fixed

No blocking correctness or spec issue remains in this bounded patch after the above fix.

Known limits remain explicitly out of scope: cross-event/chunk streaming, arbitrary encoded or encrypted secrets, dynamic per-task exact-secret registries, historic log cleanup and guessing the externally issued `mcn_` token format. These must not be represented as fixed. Recognized malformed/oversized sensitive JSON conservatively discards trailing text; this loses diagnostics by design.

The local benchmark observes current cost only and has no baseline or production latency claim. No throughput improvement claim was accepted. Full deployed provider behavior remains unverified.

## Boundary and adequacy review

- Sensitive-key matching is shared across nested maps/string maps and JSON members. It recognizes explicit credential names/suffixes including `secret_access_key`; ordinary token usage/count/public fields are preserved. Non-string sensitive values are masked and the caller input is retained.
- JSON member value spans are replaced without whole-document reserialization, preserving large numeric literals, order, duplicate members and surrounding text. Key escapes are decoded. Recognized values have a 1MiB decode budget and depth32 bound; malformed/over-budget values fail closed.
- PAT/daemon/task formats were checked against auth/jwt.go; plugin token length/charset against plugin_token.go; installation proof prefix against installation_metadata.go. Adjacent-token handling does not consume separators. Original collaboration-capability coverage is now preserved independently.
- The daemon invokes redaction on the full tool result before the existing UTF-8/8192-byte preview. The fake executeAndDrain→recorder test covers the actual call site, not just helper composition. Server ingest retains its second redaction boundary.
- Codex production behavior is unchanged; the one assertion update correctly expects semantic-field masking instead of the previous vendor-specific placeholder. Its secret-removal and provider-result-sanitization checks remain intact.

## Verification

Independently executed two synthetic stdlib probes against the actual package, with Git-external temporary source/state and no credentials or accounts:

- 1,000 deterministic valid JSON cases, using varied escaped values and nine sensitive field names: all removed the synthetic secret, remained valid JSON and were idempotent. Code/output retained as `independent-json-property.go.txt` / `.log`.
- Six boundary cases (quoted diagnostic prefix, escaped JSON keys, adjacent first-party tokens and embedded JSON fragments) passed; three normal examples were checked. Code/output retained as `independent-boundary.go.txt` / `.log`.

After the capability fix, independently ran:

| Command | Result |
| --- | --- |
| `go -C server test -json -p1 ./pkg/redact -count=1` | 77 named pass, 0 fail, 0 skip |
| `go -C server test -race -json -p1 ./pkg/redact -count=1` | 77 named pass, 0 fail, 0 skip |
| `go -C server vet -p1 ./pkg/redact` | exit0 |
| `gofmt -l` on all five Go files and scoped `git diff --check` | no findings |

Final command vectors, duration and inventories are retained in `research/independent-redact-summary.json` and `independent-redact*-final.log`. The prior implementation's 74-case normal/race runs remain preserved rather than overwritten. Its unchanged fake daemon37/provider16 named pass inventories were independently parsed from raw logs, with no failure/skip events; no unrelated expensive rerun was performed after restoring only the legacy capability boundary.

No live DB, service, model, provider CLI, credentials, release or network access was used by this reviewer. Main session owns commits, task status and final delivery.
