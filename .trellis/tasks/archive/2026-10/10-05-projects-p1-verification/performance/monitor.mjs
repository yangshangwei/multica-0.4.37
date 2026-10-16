import fs from "node:fs";
import { execFileSync } from "node:child_process";
const label = process.env.PERF_LABEL ?? "current";
const mode = process.env.PERF_MODE ?? "full";
const api = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:19073";
const directory = process.env.PERF_DIRECTORY ?? ".omx/projects-p1-performance";
const output = `${directory}/${label}-${mode}-resource-samples.jsonl`;
const deadline = Date.now() + 70 * 60_000;
while (Date.now() < deadline) {
  if (fs.existsSync(`${directory}/${label}-${mode}-complete.json`)) break;
  try {
    const health = await (await fetch(`${api}/health`, { signal: AbortSignal.timeout(1000) })).json();
    const data = execFileSync("ps", ["-p", String(health.pid), "-o", "pid=,%cpu=,rss=,etime="], { encoding: "utf8" }).trim().split(/\s+/);
    fs.appendFileSync(output, JSON.stringify({ at: new Date().toISOString(), pid: health.pid, commit: health.commit, cpu_percent: Number(data[1]), rss_kib: Number(data[2]), elapsed: data[3] }) + "\n");
  } catch (error) { fs.appendFileSync(output, JSON.stringify({ at: new Date().toISOString(), error: String(error) }) + "\n"); }
  await new Promise((done) => setTimeout(done, 1000));
}
