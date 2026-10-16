import fs from "node:fs";
import path from "node:path";
const directory=process.env.PERF_RESULTS_DIR??".trellis/tasks/10-05-projects-p1-verification/performance/results";
const summary=JSON.parse(fs.readFileSync(path.join(directory,"summary.json")));
const failures=[];
if(Object.keys(summary.current??{}).length!==14)failures.push("expected 14 current endpoint/concurrency measurements");
if(Object.keys(summary.baseline??{}).length!==4)failures.push("expected 4 pre-implementation baseline measurements");
if(Object.keys(summary.full_scope_audit?.signals??{}).length!==4)failures.push("expected 4 independently audited risk scopes");
if(!summary.resources.current?.samples||!summary.resources.baseline?.samples)failures.push("missing measured process resources");
if(!summary.complete)failures.push("all active windows have not completed");
for(const [name,endpoint] of Object.entries(summary.current??{})){
  if(endpoint.samples<summary.budget.requests_per_endpoint_and_concurrency)failures.push(`${name}: insufficient samples`);
  if(endpoint.errors!==0)failures.push(`${name}: HTTP or consistency errors`);
  if(endpoint.p95_ms>summary.budget.http_p95_ms)failures.push(`${name}: P95 exceeds HTTP budget`);
}
for(let i=0;i<3;i++){
  const rate=[0,1,10][i],row=summary.active[i];
  if(!row){failures.push(`${rate}/s: no complete window`);continue;}
  if(row.elapsed_seconds<summary.budget.active_duration_seconds_each||row.samples<600)failures.push(`${rate}/s: insufficient duration or pagination operations`);
  if(row.nonempty_continuations!==undefined&&row.strictly_advancing_continuations!==row.nonempty_continuations)failures.push(`${rate}/s: nonempty continuation did not strictly advance`);
  if(row.errors||row.write_errors)failures.push(`${rate}/s: request/consistency or mutation errors`);
  const minimum=rate===0?summary.budget.idle_second_page_success_min:summary.budget.active_second_page_unusable_below;
  if(row.second_page_success_ratio===null||row.second_page_success_ratio<minimum)failures.push(`${rate}/s: second-page success ${row.second_page_success_ratio} below ${minimum}`);
}
for(const [signal,audit] of Object.entries(summary.full_scope_audit?.signals??{}))if(audit.agreement!==1||audit.expected!==audit.returned)failures.push(`${signal}: full-scope mismatch`);
const result={at:new Date().toISOString(),status:failures.length?"FAIL":"PASS",failures,scope:"API, SQL, process resources and active pagination only; cross-client convergence is assessed by the browser lane"};
fs.writeFileSync(path.join(directory,"budget-assessment.json"),JSON.stringify(result,null,2));
console.log(JSON.stringify(result));
process.exitCode=failures.length?1:0;
