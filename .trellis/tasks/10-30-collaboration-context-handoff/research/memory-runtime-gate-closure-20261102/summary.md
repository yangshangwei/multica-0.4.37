# Nested Host gate cleanup closure

Code `5dec88f9809b4a703a0e82d65796876dfd31ffaf` changes only `e2e/collaboration-memory-runtime.spec.ts`. Product source remains `639bf6ef8`; the committed test bytes/diff match the actual green run.

The old fixture closes the outer runtime before its nested successor. Because each close restores captured flags, an initially closed Host gate becomes open again. The new canonical red completed the actual two-runtime memory handoff through v3, then failed only the new gate assertion: enabled/creation remained true, Host false→true, epoch3→6. The RED database and old final3 Leave409 failure are retained.

The fix closes the second fixture first, then the first through nested finally, followed by context cleanup and read-only state evidence. GREEN memory passed once: original flags true/true/false were restored, epoch3→5. Leave then passed once on the same GREEN database, API pid and configuration, without any coordinator gate reset or service restart. Worker1/retry0 and original timeouts remain unchanged; Provider calls0.

The test now validates the explicit configured Host environment before connecting, verifies actual `current_database()`, and asserts the three original booleans after cleanup while recording epochs separately. A pre-existing unused, never-invoked `read` closure was removed; no executed request or business assertion was deleted. Strict TypeScript, unchanged repository base ESLint (0 errors/warnings) and collection checks pass. Initial static diagnostics and metadata-reader failure remain separately recorded.

No product, dependency, admission default, shared fixture API or timeout changed. This lane created/migrated/dropped no database and performed no manual gate or service/container operation. Root supplied separate fresh RED/GREEN task environments; raw artifacts stay private. Final independent review, integration and parent/global acceptance remain open.
