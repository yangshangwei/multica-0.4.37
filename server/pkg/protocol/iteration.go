package protocol

// EventIterationUpdated invalidates authorized iteration/issue projections.
// It contains identities only; consumers refetch through workspace-scoped APIs.
const EventIterationUpdated = "iteration:updated"
