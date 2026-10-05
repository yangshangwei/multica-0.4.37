# Administrator password reset and resource publishing

## User authorization and outcome
User requested a new Trellis task and implementation after reviewing requirements. Deliver password reset through existing account details, and administrator file publishing so supported Web/Electron markets discover new resources without rebuilding for each resource.

## Requirements
- Rename/clarify existing recovery as password reset; distinguish target temporary password from administrator current password. Preserve forced password change, revocation, audit, idempotency and self/role constraints. Add confirmation and explicit success/impact guidance.
- Platform administration adds Resource publishing with Skill/MCP tabs. Admins can list, upload, validate/preview, publish, explicitly replace and withdraw managed entries. Observer is read-only. Builtin and manually deployed entries are read-only.
- Skill supports SKILL.md and ZIP/.skill containing SKILL.md plus supported UTF-8 text attachments. Reject invalid or unsupported files without silently dropping content. MCP supports standard mcp.json; reuse strict existing manifest parser.
- Content appears in existing deployment-provided markets; existing polling/refresh behavior retained. Users choose to add/configure/bind. Existing workspace copies/configuration and bindings remain unchanged on update/withdraw.
- Persist managed publications separately from readonly manual directories. Failures keep the last active revision. Detect concurrent replacement and replays. Audit metadata never includes uploaded contents or passwords/secrets.
- No arbitrary server path choice, script execution, dependency installation, automatic desktop filesystem writes or old-client auto-upgrades.

## Acceptance
Permission tests, malformed/path/symlink/archive-limit cases, known-valid publication through existing catalog readers, version/conflict/idempotency behavior, read-only/unavailable handling, no change to existing copies, password reset regression, bilingual responsive UI, Web+desktop compatibility evidence.
