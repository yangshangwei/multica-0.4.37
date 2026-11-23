import fs from "node:fs";
import pg from "pg";
const label = process.env.PERF_LABEL ?? "current";
const directory = process.env.PERF_DIRECTORY ?? ".omx/projects-p1-performance";
const session = JSON.parse(fs.readFileSync(`${directory}/${label}-session.json`));
const db = new pg.Client(process.env.DATABASE_URL);
await db.connect();
const cases = {
  project_health_inputs: ["SELECT id,status,due_date,assignee_type,assignee_id FROM issue WHERE workspace_id=$1::uuid AND project_id=$2::uuid AND admission_status IN ('not_required','accepted') ORDER BY id ASC", [session.workspace.id,session.project]],
  legacy_project_stats: ["SELECT project_id,count(*)::bigint AS total_count,count(*) FILTER(WHERE status=ANY($3::text[]))::bigint AS done_count FROM issue WHERE admission_status IN ('not_required','accepted') AND workspace_id=$1::uuid AND project_id=ANY($2::uuid[]) GROUP BY project_id", [session.workspace.id,[session.project],["done","cancelled"]]],
  legacy_500_project_stats: ["SELECT project_id,count(*)::bigint AS total_count,count(*) FILTER(WHERE status=ANY($3::text[]))::bigint AS done_count FROM issue WHERE admission_status IN ('not_required','accepted') AND workspace_id=$1::uuid AND project_id=ANY($2::uuid[]) GROUP BY project_id", [session.workspace.id,session.projects,["done","cancelled"]]],
  new_500_project_status_counts: ["SELECT project_id,status,count(*)::bigint AS issue_count FROM issue WHERE workspace_id=$1::uuid AND project_id=ANY($2::uuid[]) AND admission_status IN ('not_required','accepted') GROUP BY project_id,status", [session.workspace.id,session.projects]],
  risk_page: ["SELECT * FROM issue WHERE workspace_id=$1::uuid AND project_id=$2::uuid AND admission_status IN ('not_required','accepted') AND id=ANY($3::uuid[]) ORDER BY id ASC", [session.workspace.id,session.project,session.targetIDs.slice(0,50)]],
};
const report = { measured_at: new Date().toISOString(), label, queries: {} };
for(const [name,[sql,args]] of Object.entries(cases)) {
  const result = await db.query(`EXPLAIN (ANALYZE,BUFFERS,FORMAT JSON) ${sql}`,args);
  report.queries[name]={sql,explain:result.rows[0]["QUERY PLAN"]};
}
fs.writeFileSync(`${directory}/${label}-exact-query-plans.json`,JSON.stringify(report,null,2));
console.log(JSON.stringify({label,queries:Object.fromEntries(Object.entries(report.queries).map(([key,v])=>[key,{planning_ms:v.explain[0]["Planning Time"],execution_ms:v.explain[0]["Execution Time"],rows:v.explain[0].Plan["Actual Rows"]}]))}));
await db.end();
