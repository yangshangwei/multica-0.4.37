from pathlib import Path
import datetime,json,re,os
ROOT=Path("/Volumes/artisan/code/2026/mc-collaboration-closure-v12-20261103")
OUT=ROOT/".omx/private/final-check-v14"
run=json.loads((OUT/"make-check-v14.json").read_text())
raw=(OUT/"make-check-v14.log").read_text(errors="replace")
log=re.sub(r"\x1b\[[0-9;]*[a-zA-Z]","",raw)
stages=re.findall(r"\[([1-6])/6\] ([^\n]+)",log)
summary={"observed_at":datetime.datetime.now(datetime.timezone.utc).isoformat(),"state":run["state"],"make_pid":run.get("process_id"),"launcher_pid":json.loads((Path(__file__).parent/"launch.json").read_text())["launcher_pid"],"wholecheck_accepted":False,"make_exit":run.get("exit_code"),"stage":stages[-1] if stages else None,"log_bytes":len(raw.encode()),"tail":log.splitlines()[-3:]}
if (OUT/"go-packages-v14").exists():
 packages=[]
 for p in sorted((OUT/"go-packages-v14").glob("*.json")):
  try:d=json.loads(p.read_text())
  except json.JSONDecodeError:continue
  packages.append(d)
 summary["Go"]={"allocated":len(packages),"completed":sum(d.get("state")=="complete" for d in packages),"failed_packages":[d["package"] for d in packages if d.get("state")=="complete" and d.get("exit_code")!=0],"current_packages":[d["package"] for d in packages if d.get("state")!="complete"]}
def alive(pid):
 try:os.kill(pid,0);return True
 except ProcessLookupError:return False
 except PermissionError:return None
summary["make_process_alive"]=alive(summary["make_pid"])
summary["launcher_process_alive"]=alive(summary["launcher_pid"])
ts={}
for line in log.splitlines():
 match=re.search(r"(@[^ :]+):test:\s+Tests\s+(.+)",line)
 if match:ts[match[1]]={kind:int(value) for value,kind in re.findall(r"(\d+)\s+(passed|failed|skipped|todo)",match[2])}
summary["TS_completed_package_summaries"]=ts
static=log.split("[2/6]")[0]
match=re.search(r"Tasks:\s+(\d+) successful,\s+(\d+) total",static)
summary["static_tasks"]={"passed":int(match[1]),"total":int(match[2])} if match else None
if "Go" in summary:
 summary["Go"]["completed_named_counts"]={kind:sum(d.get("counts",{}).get(kind,0) for d in packages if d.get("state")=="complete") for kind in ["pass","fail","skip"]}
if "Go" in summary:
 failures=[]
 running_counts={"pass":0,"fail":0,"skip":0}
 for d in packages:
  if d.get("state")=="complete":continue
  raw=OUT/"go-packages-v14"/(d["database"]+".jsonl")
  if not raw.exists():continue
  for line in raw.read_text(errors="replace").splitlines():
   try:event=json.loads(line)
   except json.JSONDecodeError:continue
   if event.get("Test") and event.get("Action") in running_counts:running_counts[event["Action"]]+=1
   if event.get("Action")=="fail" and event.get("Test"):
    failures.append({"package":event.get("Package"),"test":event["Test"]})
 summary["Go"]["running_named_failures"]=failures
 summary["Go"]["running_package_observed_terminal_counts"]=running_counts
if "[6/6]" in log:
 browser=log.split("[6/6]",1)[1]
 progress=re.findall(r"\[(\d+)/(\d+)\] \[",browser)
 failure_headers=re.findall(r"^\s*\d+\) \[[^\]]+\] .+$",browser,re.M)
 summary["Playwright"]={"observed_case_index":int(progress[-1][0]) if progress else None,"discovered_total":int(progress[-1][1]) if progress else None,"observed_failure_reports":failure_headers,"scope":"Reporter schedule position only; not a passed count"}
summary["scope"]="Current run only. Phase counts do not constitute wholecheck acceptance; final audit and complete inventory required."
p=Path(__file__).parent/"status.json";temp=p.with_suffix(".json.tmp");temp.write_text(json.dumps(summary,indent=2)+"\n");temp.replace(p)
print(json.dumps(summary,ensure_ascii=False))
