# Independent review: stored resource owner identity

Decision: **approve the bounded authority fix and regression pair for an isolated
Lore commit**. No blocking correctness, scope, privilege or verification finding.
This is independent of the content implementation lane and does not promote
AC40/49/96 or claim full provider/client acceptance.

Reviewed source is exactly `collaboration_authority.go`
SHA11842f553fe4ae1aefc0b44e3381fb86b28b7f5369545d0dd68aa30f433d493b and
`collaboration_resource_owner_identity_test.go`
SHA0150c893b834cde047a48a01ef97a8595cadb8c343e44aedf1ad5b8b5e6ea6fc.
The production delta is confined to the managed branch of
`collaborationIdentityCurrent`: validate the already-matched stored resource
owner/binding in a child context, without comparing that owner's password
identity to a different authenticated request caller.

`auth.WithPasswordSession` is `context.WithValue`, preserving parent cancellation,
deadline and all other values. `auth.CheckPasswordVersion` still validates current
user, password version and forced-change state, then invokes the unchanged
`CheckManagedBinding` with the child's explicit owner tuple. Its existing checks
retain binding generation/state/principal/workspace/daemon, installation lifecycle,
deployment/organization and membership. The caller's original context is never
replaced or returned; generic HTTP/WS/daemon authorization is unchanged.
The enrolled identity branch and existing initial runtime/binding equality checks
are untouched. The duplicate direct `CheckManagedBinding` call is removed because
`CheckPasswordVersion` already owns it for a binding-tagged session.

The test independently authenticates another valid daemon before evaluating the
resource. Its valid branch proves that the owner's resource identity remains
current for same-owner daemon, different valid daemon and human caller. Claim
against the other owner's run still rejects with an identical retained control
snapshot; stale caller binding and human List still reject. Thus owner identity
validation is a resource fact, not caller privilege transfer.

Thirteen final scenarios each cover all three caller modes: valid, unknown owner,
missing/stale owner password, missing/stale/revoked binding, lost owner membership,
disabled owner, retired installation, database failure, pre-cancellation and
cancellation during owner password query. The query observer verifies exact owner
tuple and inherited deadline; the after-call assertion preserves caller session
and deadline. Database errors/cancellation remain errors, rather than converting
into valid identity or silently creating a grant. No writes or new dependency,
fallback, background context, logging or API/schema changes are introduced.

## Verified evidence

The retained original red run fails only the named valid
`different_valid_daemon` expectation (`current=false want=true err=nil`) while
the same-owner/human controls pass. Its source and raw result are
`owner-switch-resource-identity-red-current-20261022*`.
The intermediate expanded run failed during an illegal fixture deletion of the
existing owner user under a legacy `agent_owner_id_fkey`; it did not establish a
product defect. Final unknown-owner uses a mismatched unknown UUID and separately
tests missing password, avoiding deletion of retained foreign references.

Coordinator exact HEAD829815775 plus only the two reviewed files passes serial
race **1 top-level /53 including subtests**, no fail/skip, **20.456 seconds**, on
schema737 `service_s6_commit`. `owner-identity-isolated-head-source.json` records
no other dirty files copied; command/result/JSONL hashes match current source.
The reviewer parsed all53 actual PASS events, including all39 leaf combinations,
and checked final source hashes, empty gofmt diagnostics and `git diff --check`.
Coordinator `go vet -p 1 ./internal/service ./internal/auth` also passes with exact
same source and empty diagnostics. Current handoff/runtime/resource adjacency
has112 top-level/406 total passes, no skips; it is separate broad regression
evidence and not a substitute for the exact isolated fix proof.

Record the owner-versus-caller distinction in the coordinator-owned authority
spec so future modifiers do not reintroduce `context.Background` or accidentally
tag the caller's session as the owner. This recommendation is documentation,
not an unapproved product scope change. The implementation already has a concise
English contract comment. Review made no product/spec/ledger/commit edits.

The fixture uses real database authorization and service-managed Claim/Permit;
it does not launch a provider process, prove kernel containment or validate full
installed Main/old clients. Task and parent remain in progress, production gates
stay closed and provider account/quota authorization remains separate.
