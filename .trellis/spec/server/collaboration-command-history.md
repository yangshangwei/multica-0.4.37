# Collaboration command-history wire and execution evidence

## Scenario: nullable command-history reason and real cross-client batching

### 1. Scope / Trigger

A successfully settled command can store valid PostgreSQL text `""`. The
command-history API must remain parseable after successful execution, including
when two actual clients reopen the same retained history.

### 2. Signatures

`GET /api/collaboration/issues/{id}/commands?limit&cursor` calls
`CollaborationService.CommandHistory(ctx, issueID, query)` and returns protocol 1.
`CollaborationCommandHistoryEntry.Reason` is `*string` with JSON key `reason`;
Core's `CollaborationCommandHistoryEntrySchema` accepts nonempty text or null.

### 3. Contracts

At the service projection, invalid or empty `pgtype.Text` becomes JSON `null`.
Preserve every nonempty reason exactly, including cancellation/failure values;
do not trim, rewrite the persisted reason or change generic `util.TextToPtr`.
The history keeps original author, typed target, input event, channel, batch,
group/run and cancellation identities. A later comment edit or client switch
cannot rebuild input or trigger a new execution.

### 4. Validation & Error Matrix

| Persisted reason | Required wire |
| --- | --- |
| SQL NULL | `"reason": null` |
| Valid empty text | `"reason": null` |
| Nonempty cancellation/failure text | Exact original string |
| Empty string emitted by API | Strict client rejects; repair the service projection |
| Legacy cancellation without exact event | Unknown provenance, never inferred actor/time |

### 5. Good / Base / Bad Cases

Good: A1 stays live while B1 completes in another actual Linux namespace; A2,
A3 and A4 share one pending batch; cancelling A3 leaves only A2+A4 for one
follow-up. Both Web and Electron reload four successes and one cancellation.
Base: a command with no explanation renders without a reason label.
Bad: a successful command emits an empty non-null reason and makes the entire
history unavailable, or a fixture bypasses strict parsing to hide that failure.

### 6. Tests Required

`TestCollaborationCommandHistoryProjectsEmptyTerminalReasonAsNull` drives actual
managed submit/Resume/Claim/Permit/outcome/proof state and asserts nullable JSON
plus exact cancellation reason. Use a schema737 task DB and verify no DB skips.
The managed fixture sets password mode; a subsequent legacy JWT fixture must
restore its own test-scoped auth mode before creation.

`e2e/collaboration-cross-client-execution.spec.ts` is the complete AC33 journey:
two distinct synthetic actors in production Web and actual Electron, independent
capacity-one Linux daemons, real provider input/process/proof, B/A overlap,
coalesced follow-up, exact one-input cancellation, immutable attribution, both
client history/result reads and zero extra invocation on reload. Numeric callback
IPv4 and explicitly hashed test binaries stay task-owned; credentials pass only
via stdin and never enter evidence. Sequential namespace inode reuse is legal;
compare simultaneous namespace identities and retained run/group identities.
Run Electron serially after HTTP bytes match its known built renderer index.
Provider/account/quota, installed full Main and other OS support are separate.

### 7. Wrong vs Correct

Wrong: map an empty valid `pgtype.Text` through generic `TextToPtr`, relax the
client schema, or call a DB-backed test without DATABASE_URL and count its skips.
Correct: normalize absent/empty reason only at the history projection, preserve
all actual reason text, verify the explicit DB race result and rerun both clients
against the refreshed task API source/configuration fingerprint.
