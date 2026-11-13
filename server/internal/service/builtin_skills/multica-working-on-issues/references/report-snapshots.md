# Frozen report snapshots

`multica issue report-snapshot show <report-uuid> --output json` reads the current
sealed snapshot; `--snapshot <snapshot-uuid>` rereads one immutable identity.
Use complete UUIDs: this command never lists projects or resolves an issue prefix.
Reading does not capture, refresh, publish a comment, or change issue status.
Reporter task credentials may read only their exact current report and its frozen
project under current authority; a missing snapshot is not permission to collect.
The response is a sealed public projection with `public_projection_version` and
`projection_digest`. It omits private collector identities, request keys, read
cuts and directory facts. Do not fetch or invent those fields to complete a report.

Only a currently authorized human may explicitly run `issue report-snapshot
capture <report-uuid>` with `--request-key`, `--expected-revision`, `--timezone`,
`--start`, `--end`, and `--period-mode`. Times are UTC with six fractional digits
and `Z`; the interval includes start and excludes end. `--correction-of` is an
explicit new capture intent, never an automatic retry. Retain the original key
and exact flags after an uncertain response. The CLI does not generate a key,
retry a write, or follow a redirect; replay with the original intent is distinct
from selecting another key or correction.

Human capture confirms the original key and exact typed request through a separate
`capture_receipt`; this receipt is absent from GET and is not a capability. Missing,
mismatched or malformed receipts leave the capture result unconfirmed, even when
the public snapshot itself validates.

A currently authorized human may explicitly withdraw **all** snapshot content of
one report with `issue report-snapshot withdraw <report-uuid> --request-key <uuid>
--expected-revision <n> --current-snapshot <snapshot-uuid> --reason <text> --accept`.
The report can no longer be captured or corrected, and both current and old snapshot
IDs stop returning content. There is no restore command. This stops application
reads; it does not delete the issue or claim physical backup erasure. Retain the
original key and exact flags after an uncertain response; an explicit repeat only
recovers the original `withdrawal_receipt`, never retries automatically. Reporter
credentials cannot capture or withdraw, and must not create another report or use
a new key to bypass withdrawal.

Interpret the returned scope, window, `as_of`, versions, coverage, and gaps.
`formal_stock` is the captured formal-task cohort, not proven business throughput.
Unknown classification or history stays unknown; `null` is not zero, and a
successful sealed-snapshot read does not mean every metric is complete. Do not
replace a frozen metric with current rows or infer deployment/product results
from task counts. See the source map's report-snapshot contract for the authority
and integrity boundaries.
