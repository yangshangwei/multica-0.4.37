# Project description/preparation lock correction

The actual `Handler.UpdateProject` regression holds a successful description
write before commit and then calls actual knowledge preparation. Issue-number
allocation passes; description returns `503 knowledge_busy` in the retained red.
The writer borrowed `LockProjectForExecutionSquad` (`FOR UPDATE`) for every
metadata edit, conflicting with the intended parent-existence `KEY SHARE` lock.

Main owns the correction: add `LockProjectForUpdate` (`FOR NO KEY UPDATE`) in
`server/pkg/db/queries/project.sql`, generate its sqlc method, and change only
the metadata handler call. This still serializes revisions, conflicts with
project deletion and with `SHARE` readers of lead/status/association authority.
Execution-squad selection, resource writes and deletion retain their exclusive
locks. No API, retry budget, preparation policy or dependency changes are needed.

Validation: the exact real-writer regression must turn green, related project
revision/description/lead/deletion/squad suites must pass, and an independent
review must confirm that no authority fence was weakened. Final combined-source
sqlc/Go evidence follows after the remaining test-only matrix additions.
