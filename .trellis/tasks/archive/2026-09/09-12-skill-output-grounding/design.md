# Design

Update the three skill bodies in place. Grounding instructions should be short
and operational: assemble source-backed facts, keep unknowns explicit, and verify
the final artifact against source evidence. Do not accumulate broad prohibitions.

Put CSV-specific knowledge in a linked reference under requirement clarification,
with official sources and application-dependent validation limits. Do not ship a
new sanitizer or claim a universal spreadsheet-safe encoding.

Reuse the existing isolated Claude evaluation harness. Retain a minimal reusable
version under scripts/skill-eval, with no implicit account access in normal tests.
The local MCP adapter returns literal search matches, path/line/hash evidence and
coverage metadata, without model answers or grading criteria. Search and command
boundaries must remain independently testable with Python's standard library.

The user deferred further real calls. Keep the current Claude adapter explicit;
do not switch providers or present it as Codex-compatible. Preserve portable
cases, skill snapshots and raw evidence for later native Codex integration.
Local verification covers macOS aliases and distinct files on a case-sensitive
Linux filesystem; neither environment's result proves model output quality.

The leader owns skill bodies, registry versions, template checks, workspace
integration and final evidence. A bounded helper owns the evaluation harness and
its tests; another verifies CSV reference facts. No shared-file edits.

Changing templates does not overwrite existing workspace copies. Apply only the
three intended updates after comparing their stored content with the baseline;
preserve metadata/customizations and add the required supporting reference.
