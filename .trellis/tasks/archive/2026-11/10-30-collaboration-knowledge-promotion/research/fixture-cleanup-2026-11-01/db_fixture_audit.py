#!/usr/bin/env python3
"""Audit and clean exact, retained test-source IDs in the exclusive C2 DB."""

from datetime import datetime, timezone
import json
import os
from pathlib import Path
import subprocess
import sys
from urllib.parse import urlparse
from uuid import UUID


out = Path(__file__).resolve().parent
database = "multica_knowledge_c2_20261101_7d95f46f"
database_url = os.environ["DATABASE_URL"]
assert urlparse(database_url).path.removeprefix("/") == database


def query(sql):
    result = subprocess.run(
        ["/opt/homebrew/opt/libpq/bin/psql", "--dbname", database_url, "-X", "-A", "-t", "-v", "ON_ERROR_STOP=1", "-c", sql],
        capture_output=True, text=True, check=True,
    )
    return [json.loads(line) for line in result.stdout.splitlines() if line.startswith("{")]


def snapshot():
    return query("""
SELECT jsonb_build_object(
  'database',current_database(),
  'all_pending',(SELECT count(*) FROM collaboration_event WHERE outbox_state='pending'),
  'orphan_pending',(SELECT count(*) FROM collaboration_event e WHERE e.outbox_state='pending' AND NOT EXISTS(SELECT 1 FROM issue i WHERE i.id=e.issue_id)),
  'candidate_count',(SELECT count(*) FROM collaboration_knowledge_candidate),
  'candidate_hash',(SELECT md5(string_agg(to_jsonb(k)::text,E'\\n' ORDER BY id)) FROM collaboration_knowledge_candidate k),
  'invalidation_count',(SELECT count(*) FROM collaboration_knowledge_invalidation),
  'invalidation_hash',(SELECT md5(string_agg(to_jsonb(k)::text,E'\\n' ORDER BY id)) FROM collaboration_knowledge_invalidation k),
  'owned_sources',COALESCE((SELECT jsonb_agg(jsonb_build_object(
    'source_issue_id',k.source_issue_id,'source_workspace_id',k.source_workspace_id,
    'candidate_id',k.id,'family',CASE WHEN k.content='EXTERNAL_PRIVATE_DISCLOSURE' THEN 'preparation' ELSE 'server_logging' END,
    'issue_exists',EXISTS(SELECT 1 FROM issue i WHERE i.id=k.source_issue_id),
    'pending_events',(SELECT count(*) FROM collaboration_event e WHERE e.issue_id=k.source_issue_id AND e.outbox_state='pending'),
    'events',(SELECT count(*) FROM collaboration_event e WHERE e.issue_id=k.source_issue_id),
    'collaboration_rows',(SELECT count(*) FROM issue_collaboration c WHERE c.issue_id=k.source_issue_id),
    'sessions',(SELECT count(*) FROM collaboration_session c WHERE c.issue_id=k.source_issue_id),
    'participants',(SELECT count(*) FROM issue_participant c WHERE c.issue_id=k.source_issue_id),
    'handoffs',(SELECT count(*) FROM issue_handoff c WHERE c.issue_id=k.source_issue_id),
    'notifications',(SELECT count(*) FROM collaboration_notification c WHERE c.issue_id=k.source_issue_id),
    'memory_versions',(SELECT count(*) FROM collaboration_memory_version c WHERE c.issue_id=k.source_issue_id),
    'issue_receipts',(SELECT count(*) FROM collaboration_operation c WHERE c.scope_kind='issue' AND c.scope_id=k.source_issue_id),
    'create_receipts',(SELECT count(*) FROM collaboration_operation c WHERE c.scope_kind='create' AND c.scope_id=k.source_workspace_id)
  ) ORDER BY k.proposed_at) FROM collaboration_knowledge_candidate k
  WHERE k.content IN ('EXTERNAL_PRIVATE_DISCLOSURE','RR2_PRIVATE_APPROVED_STDIN_ECHOED_TO_STDERR')),'[]'::jsonb)
)
""")[0]


action, label = sys.argv[1:]
record = {"started_at": datetime.now(timezone.utc).isoformat(), "database": database, "action": action, "before": snapshot()}
if action == "clean":
    # Candidate sentinels identify these two test families; UUIDs are frozen in
    # the evidence before deleting only their already-orphaned transient rows.
    rows = [r for r in record["before"]["owned_sources"] if not r["issue_exists"] and (r["events"] or r["collaboration_rows"])]
    ids = sorted({str(UUID(r["source_issue_id"])) for r in rows})
    workspaces = sorted({str(UUID(r["source_workspace_id"])) for r in rows})
    record["source_ids"] = ids
    record["workspace_ids"] = workspaces
    if ids:
        quoted_ids = ",".join("'" + value + "'" for value in ids)
        quoted_workspaces = ",".join("'" + value + "'" for value in workspaces)
        statements = ["BEGIN"]
        for table in ("collaboration_event", "collaboration_notification", "issue_handoff", "collaboration_memory_version", "collaboration_session", "issue_participant", "issue_collaboration"):
            statements.append(f"DELETE FROM {table} WHERE issue_id IN ({quoted_ids})")
        statements.append(f"DELETE FROM collaboration_operation WHERE (scope_kind='issue' AND scope_id IN ({quoted_ids})) OR (scope_kind='create' AND scope_id IN ({quoted_workspaces}))")
        statements.append("COMMIT")
        record["sql"] = ";\n".join(statements)
        query(record["sql"])
    record["after"] = snapshot()
    for field in ("candidate_count", "candidate_hash", "invalidation_count", "invalidation_hash"):
        assert record["before"][field] == record["after"][field], field
else:
    assert action == "audit"
record["finished_at"] = datetime.now(timezone.utc).isoformat()
(out / f"{label}.json").write_text(json.dumps(record, indent=2) + "\n")
last = record.get("after", record["before"])
print(json.dumps({"artifact": label + ".json", "pending": last["all_pending"], "orphan_pending": last["orphan_pending"], "owned_sources": len(last["owned_sources"])}))
