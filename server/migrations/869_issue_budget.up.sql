-- F3-S1 parent-task budget root (observe only).
--
-- Six tables. None has a foreign key: each row keeps ID copies of the
-- workspace, issue, task, runtime and agent it describes, so deleting an issue
-- (which physically removes its tasks, usage rows and tokens) and tearing down
-- a workspace leave the budget records in place. Relationships are resolved in
-- application code.
--
-- Every index, including the keys that make rows identifiable, is built by its
-- own concurrent migration (870-884). 885 verifies them.
--
-- A CHECK that spans columns, or ties a stored form to its digest, has an
-- explicit name so a later migration can refer to it.
--
-- Amounts are ticks (1 tick = 1e-10 USD) in signed 64-bit columns that refuse
-- negatives. State columns are text with a CHECK. Identifiers a caller
-- supplies are 1 to 200 bytes: the metering package puts no limit on them.

-- One budget root per ordinary parent issue, for life.
CREATE TABLE issue_budget_root (
    id uuid NOT NULL,
    workspace_id uuid NOT NULL,
    root_issue_id uuid NOT NULL,
    -- Admission and settlement are separate axes. A deleted root issue is
    -- neither: it is read from the root issue's own member row, and nothing
    -- copies it onto this row.
    admission_state text NOT NULL DEFAULT 'open' CHECK (admission_state IN ('open', 'closed')),
    -- NULL while unsettled; otherwise the ledger revision the settlement was
    -- made at.
    settled_as_of_revision bigint,
    -- The observation switch. This slice cannot record enforcement.
    mode text NOT NULL DEFAULT 'observe' CHECK (mode IN ('off', 'observe')),
    policy_revision bigint NOT NULL DEFAULT 1 CHECK (policy_revision > 0),
    -- Never reused, also not when the mode is switched off and on again.
    policy_epoch bigint NOT NULL DEFAULT 1 CHECK (policy_epoch > 0),
    -- The seq of the last ledger entry; 0 for an empty ledger.
    ledger_revision bigint NOT NULL DEFAULT 0 CHECK (ledger_revision >= 0),
    -- NULL means no limit is configured.
    limit_ticks bigint CHECK (limit_ticks >= 0),
    -- What a manager declared each attempt reserves: a positive amount. NULL
    -- means nothing was declared and every reservation is unsized.
    attempt_reserve_ticks bigint CHECK (attempt_reserve_ticks > 0),
    -- The net projection of the ledger as of ledger_revision.
    actual_known_ticks bigint NOT NULL DEFAULT 0 CHECK (actual_known_ticks >= 0),
    estimated_ticks bigint NOT NULL DEFAULT 0 CHECK (estimated_ticks >= 0),
    reserved_ticks bigint NOT NULL DEFAULT 0 CHECK (reserved_ticks >= 0),
    unknown_hold_ticks bigint NOT NULL DEFAULT 0 CHECK (unknown_hold_ticks >= 0),
    overrun_ticks bigint NOT NULL DEFAULT 0 CHECK (overrun_ticks >= 0),
    unknown_count bigint NOT NULL DEFAULT 0 CHECK (unknown_count >= 0),
    -- Costs are attributed exactly from this point on.
    activated_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT issue_budget_root_settlement_check
        CHECK (settled_as_of_revision IS NULL OR settled_as_of_revision BETWEEN 0 AND ledger_revision)
);

-- Which root an issue belongs to, for life. The root issue has a row too.
CREATE TABLE issue_budget_member (
    workspace_id uuid NOT NULL,
    issue_id uuid NOT NULL,
    budget_root_id uuid NOT NULL,
    -- The parent the binding was inherited through, kept after the issue is
    -- moved or the parent is deleted. NULL only on the root issue's own row.
    bound_parent_issue_id uuid,
    bind_source text NOT NULL CHECK (bind_source IN ('root', 'activation', 'create', 'reparent')),
    -- Version of the binding rule that wrote the row.
    binding_version integer NOT NULL DEFAULT 1 CHECK (binding_version > 0),
    bound_at timestamptz NOT NULL DEFAULT now(),
    -- Set when the issue is deleted. The row stays. On the root issue's own
    -- row this is what marks the whole root as deleted.
    tombstoned_at timestamptz,
    CONSTRAINT issue_budget_member_root_parent_check
        CHECK ((bind_source = 'root') = (bound_parent_issue_id IS NULL))
);

-- One claim of one task. A re-claim is a new attempt.
CREATE TABLE issue_budget_attempt (
    -- Issued by the server and sent to the daemon with the launch payload.
    id uuid NOT NULL,
    workspace_id uuid NOT NULL,
    budget_root_id uuid NOT NULL,
    task_id uuid NOT NULL,
    claim_generation bigint NOT NULL CHECK (claim_generation >= 0),
    runtime_id uuid NOT NULL,
    agent_id uuid NOT NULL,
    -- A quick-create task has no issue; it is attributed through the parent
    -- issue saved in its context.
    issue_id uuid,
    context_parent_issue_id uuid,
    -- open, one of the hold reasons of the metering package, or settled.
    state text NOT NULL DEFAULT 'open' CHECK (state IN ('open', 'cancelled', 'timeout', 'process_exit', 'lost_contact', 'superseded_unproven', 'settled')),
    -- The original shadow reservation. The unspent part is in the ledger.
    -- NULL is an unsized reservation: the root declared no reserve when the
    -- attempt was claimed. 0 is a sized reservation of nothing.
    reserved_ticks bigint CHECK (reserved_ticks >= 0),
    -- Budget protocol version the claiming daemon spoke; NULL for a daemon
    -- without the protocol.
    protocol_version integer CHECK (protocol_version > 0),
    -- The root's policy revision and epoch the attempt was judged under.
    -- Both or neither. What the judgement found is in observation rows.
    policy_revision bigint CHECK (policy_revision > 0),
    policy_epoch bigint CHECK (policy_epoch > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT issue_budget_attempt_attribution_check
        CHECK (num_nonnulls(issue_id, context_parent_issue_id) >= 1),
    CONSTRAINT issue_budget_attempt_policy_pair_check
        CHECK ((policy_revision IS NULL) = (policy_epoch IS NULL))
);

-- The immutable ledger of a root. Rows are appended in seq order under the
-- root row lock and never updated; the projection on the root is the replay
-- of these rows.
CREATE TABLE issue_budget_entry (
    workspace_id uuid NOT NULL,
    budget_root_id uuid NOT NULL,
    seq bigint NOT NULL CHECK (seq > 0),
    entry_id text NOT NULL CHECK (octet_length(entry_id) BETWEEN 1 AND 200),
    kind text NOT NULL CHECK (kind IN ('reserve', 'usage', 'hold', 'settle', 'gap', 'exclusion')),
    -- The stored form only knows a reservation ID; the attempt is kept here.
    attempt_id uuid,
    invocation_id text CHECK (octet_length(invocation_id) BETWEEN 1 AND 200),
    scope_id text CHECK (octet_length(scope_id) BETWEEN 1 AND 200),
    -- The canonical stored form: the exact bytes the metering package
    -- marshals for the entry. entry_digest is its digest over those bytes
    -- (domain "budget-entry-v1\n"), so a stored form cannot drift from it.
    entry_json bytea NOT NULL CHECK (octet_length(entry_json) BETWEEN 2 AND 65536),
    entry_digest text NOT NULL,
    -- The committed outcome, returned again when the entry is resent.
    receipt jsonb NOT NULL CHECK (jsonb_typeof(receipt) = 'object'),
    -- The price snapshot an estimated usage entry was priced with, frozen as
    -- the exact bytes the metering package marshals, and the digest it
    -- computes over them (domain "budget-price-snapshot-v1\n"). One attempt
    -- can use several models, so the snapshot belongs to the entry it priced.
    -- NULL on every entry that was not estimated.
    price_snapshot bytea CHECK (octet_length(price_snapshot) BETWEEN 2 AND 65536),
    price_digest text,
    -- The envelope: how the entry arrived. None of it enters the replay.
    -- Every entry names who produced it and the version of its stored form.
    -- The rest is what a reporting daemon supplies and is NULL on an entry
    -- the server writes itself.
    source text NOT NULL CHECK (source ~ '^[a-z][a-z0-9_]{0,62}$'),
    -- Version of the stored entry form in entry_json.
    schema_version integer NOT NULL CHECK (schema_version > 0),
    adapter_version text CHECK (octet_length(adapter_version) BETWEEN 1 AND 200),
    normalizer_version text CHECK (octet_length(normalizer_version) BETWEEN 1 AND 200),
    producer_epoch text CHECK (octet_length(producer_epoch) BETWEEN 1 AND 200),
    -- SHA-256 of the reported payload the entry was normalized from.
    payload_digest text CHECK (payload_digest ~ '^[0-9a-f]{64}$'),
    observed_at timestamptz,
    reported_at timestamptz,
    -- When the reporting request arrived at the server. created_at is when
    -- the row was committed.
    received_at timestamptz,
    completeness text CHECK (completeness IN ('partial', 'final')),
    -- The committed operation that appended the entry (for reported usage,
    -- the batch): issue_budget_operation.id.
    operation_id uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT issue_budget_entry_digest_check
        CHECK (entry_digest = encode(sha256(decode('6275646765742d656e7472792d76310a', 'hex') || entry_json), 'hex')),
    CONSTRAINT issue_budget_entry_kind_attempt_check
        CHECK (kind NOT IN ('reserve', 'hold', 'settle') OR attempt_id IS NOT NULL),
    CONSTRAINT issue_budget_entry_usage_invocation_check
        CHECK ((kind = 'usage') = (invocation_id IS NOT NULL)),
    CONSTRAINT issue_budget_entry_usage_scope_check
        CHECK ((kind = 'usage') = (scope_id IS NOT NULL)),
    CONSTRAINT issue_budget_entry_price_pair_check
        CHECK ((price_snapshot IS NULL) = (price_digest IS NULL)),
    CONSTRAINT issue_budget_entry_price_digest_check
        CHECK (price_digest = encode(sha256(decode('6275646765742d70726963652d736e617073686f742d76310a', 'hex') || price_snapshot), 'hex')),
    CONSTRAINT issue_budget_entry_price_usage_check
        CHECK (kind = 'usage' OR price_snapshot IS NULL)
);

COMMENT ON COLUMN issue_budget_entry.schema_version IS
    'Version of the stored entry form in entry_json.';
COMMENT ON COLUMN issue_budget_entry.received_at IS
    'When the reporting request arrived at the server; NULL on an entry the server wrote itself. created_at is the commit time of the row.';

-- Committed operations by their original key: the same key with the same
-- intent returns result, with another intent it is a conflict.
CREATE TABLE issue_budget_operation (
    -- What an entry cites as the operation that appended it.
    id uuid NOT NULL,
    workspace_id uuid NOT NULL,
    budget_root_id uuid NOT NULL,
    -- What the key is scoped to: the issue a root is activated on, an
    -- existing root, or the attempt a usage batch reports for.
    scope_kind text NOT NULL CHECK (scope_kind IN ('issue', 'root', 'attempt')),
    scope_id uuid NOT NULL,
    operation_key text NOT NULL CHECK (octet_length(operation_key) BETWEEN 1 AND 200),
    operation_kind text NOT NULL CHECK (operation_kind ~ '^[a-z][a-z0-9_]{0,62}$'),
    intent_digest text NOT NULL CHECK (intent_digest ~ '^[0-9a-f]{64}$'),
    actor_type text NOT NULL CHECK (actor_type IN ('member', 'runtime')),
    actor_id uuid NOT NULL,
    result jsonb NOT NULL CHECK (jsonb_typeof(result) = 'object'),
    created_at timestamptz NOT NULL DEFAULT now()
);

-- Append-only facts that carry no amount: a would-refuse, an issue created
-- somewhere other than the attempt's context parent, an in-flight or
-- old-protocol gap. This is the only place a would-refuse is recorded. One
-- attempt may have any number of rows, so nothing here is keyed by attempt.
CREATE TABLE issue_budget_observation (
    id uuid NOT NULL,
    workspace_id uuid NOT NULL,
    budget_root_id uuid NOT NULL,
    kind text NOT NULL CHECK (kind IN ('would_refuse', 'attribution_difference', 'coverage_gap')),
    reason text NOT NULL CHECK (reason ~ '^[a-z][a-z0-9_]{0,62}$'),
    issue_id uuid,
    task_id uuid,
    -- With task_id, the claim an in-flight gap could not attribute; matched
    -- against issue_budget_attempt (task_id, claim_generation) when read.
    claim_generation bigint CHECK (claim_generation >= 0),
    attempt_id uuid,
    detail jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(detail) = 'object'),
    created_at timestamptz NOT NULL DEFAULT now()
);
