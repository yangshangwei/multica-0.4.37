import fs from "node:fs";
import pg from "pg";
const directory = process.env.PERF_DIRECTORY ?? ".omx/projects-p1-performance";
const session = JSON.parse(fs.readFileSync(`${directory}/current-session.json`));
const base = process.env.NEXT_PUBLIC_API_URL;
const headers = { Authorization: `Bearer ${session.token}`, "X-Workspace-ID": session.workspace.id };
const db = new pg.Client(process.env.DATABASE_URL);
await db.connect();
const source = (await db.query("SELECT id,status,due_date::text,assignee_id FROM issue WHERE workspace_id=$1 AND project_id=$2 AND admission_status IN ('not_required','accepted') ORDER BY id", [session.workspace.id, session.project])).rows;
const overview = await (await fetch(`${base}/api/projects/${session.project}/overview`, { headers })).json();
const day = overview.statistics.reference_date;
const predicates = {
  blocked: (row) => row.status === "blocked",
  overdue: (row) => !["done", "cancelled"].includes(row.status) && row.due_date !== null && row.due_date < day,
  unassigned: (row) => !["done", "cancelled"].includes(row.status) && row.assignee_id === null,
  in_review: (row) => row.status === "in_review",
};
const report = { started_at: new Date().toISOString(), snapshot_version: overview.statistics.snapshot_version, reference_date: day, signals: {} };
for (const [signal, predicate] of Object.entries(predicates)) {
  const expected = source.filter(predicate).map((row) => row.id);
  const seen = []; let cursor = null; let pages = 0;
  do {
    const params = new URLSearchParams({ signal, limit: "100", snapshot_version: report.snapshot_version });
    if (cursor) params.set("cursor", cursor);
    const response = await fetch(`${base}/api/projects/${session.project}/health/issues?${params}`, { headers });
    const result = await response.json();
    if (!response.ok || result.refreshed || result.snapshot_version !== report.snapshot_version || result.total !== expected.length || result.overview.statistics.counts[signal] !== expected.length) throw new Error(`Unstable/incorrect full scope ${signal}: ${response.status}`);
    seen.push(...result.items.map((item) => item.id)); cursor = result.next_cursor; pages++;
  } while (cursor);
  if (JSON.stringify(seen) !== JSON.stringify(expected)) throw new Error(`Full scope differs from independent SQL: ${signal}`);
  report.signals[signal] = { expected: expected.length, returned: seen.length, pages, agreement: 1, ids: seen };
}
report.ended_at = new Date().toISOString();
fs.writeFileSync(`${directory}/current-full-scope-audit.json`, JSON.stringify(report, null, 2));
console.log(JSON.stringify({ ...report, signals: Object.fromEntries(Object.entries(report.signals).map(([key, value]) => [key, { ...value, ids: undefined }])) }));
await db.end();
