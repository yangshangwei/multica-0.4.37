# Curated MCP catalog publishing design

User selected the curated official catalog direction on 2026-10-02. Keep the
existing embedded Go roster; add two reviewed public HTTP recipes without
introducing a storage, synchronization or publisher-permissions subsystem.

## Data and lifecycle

Append microsoft-learn and deepwiki (version 1, category documentation) after the
three existing recipes. Config contains only type=http and the official HTTPS
URL. Requirements state Streamable HTTP support, outbound network access and
public-content restrictions. Existing server-side key/version resolution remains
the only trusted creation path. Removing a recipe prevents discovery/new trusted
creation; saved copies and their bindings survive. A config change increments
recipe version and stale creation requests fail. Copy-only edits do not require
a version increment. Release/revert uses existing backend deployment procedures.

## Publishing gate

Strengthen service roster tests with direct bilingual-field checks, positive
version grammar, unique name-safe keys, documentation URL validation, exactly one
supported transport target, typed arguments, config-field allowlists, no secret
fields and no placeholders. Keep checks offline and in normal Go CI. Expose them
with make check-mcp-catalog (service tests only, no database). Test the
roster against actual creation validation and add DB-backed HTTP lifecycle
coverage. No runtime validator layer is needed for compiled, reviewed content.

## Shared presentation

Add a localized documentation category and use existing book/search Lucide
registry icons and semantic colors. Template key keeps icons stable after rename;
no config readback is needed. Existing cards, dialogs, counts, explicit assignment
and unknown metadata handling remain. Use the Impeccable guidance to preserve
incumbent UI and keep new requirements factual, concise and readable.

## Verification

Observe failing new-roster and category regressions before implementation. Run
focused Go service/handler tests, schema/client and MCP component tests, relevant
lint/typecheck and Go vet. Exercise expanded catalog in production Web and native
Electron with the real local API: all recipes save, HTTP transport stays HTTP,
assignment is explicit, rename and reuse work. Record wide/narrow EN/ZH evidence
and a visual-verdict JSON. Upstream anonymous initialize/tools/list evidence is
separate from application tests and never becomes a default test network call.

## Rejected alternatives

- A new JSON catalog loader/DB/admin publishing API: no current need for mutable
  runtime catalogs; duplicates a working trusted source and broadens the task.
- More credential or required-path recipes: existing UI does not collect them.
- External registry synchronization: explicitly a different user choice.
