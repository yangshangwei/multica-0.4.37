# Workspace deletion manifest correction

Full handler schema coverage exposed three unclassified knowledge tables. Actual DeleteWorkspace already refuses source workspaces with collaboration records and calls DeleteKnowledgeTargetsInTx under the parent delete fence before ordinary cleanup. The manifest must keep immutable candidate history and both pending/delivered invalidation history, and delete only independent target counters after retaining final hints.

Own edits: server/internal/handler/workspace_delete_manifest_test.go and .trellis/spec/server/collaboration-knowledge.md. Add a focused actual-handler retention regression in the existing manifest file. No production handler, query, schema, sqlc, cascade, source-history deletion, or unrelated test edits. Main confirmed these exact paths. Use only guarded9eee DB.
