from pathlib import Path
import json,subprocess,time
HERE=Path(__file__).resolve().parent
last=None;tick=0
while True:
 result=subprocess.run(["/usr/bin/python3",str(HERE/"collect-progress.py")],capture_output=True,text=True)
 if result.returncode:
  print(json.dumps({"monitor_error":result.stderr[-1000:]}),flush=True)
  raise SystemExit(result.returncode)
 s=json.loads(result.stdout)
 key=(s.get("state"),s.get("stage"),s.get("TS_completed_package_summaries"),s.get("Go",{}).get("completed"),s.get("Go",{}).get("current_packages"))
 if key!=last or tick%2==0:
  s.pop("scope",None);s.pop("tail",None);print(json.dumps(s),flush=True);last=key
 if s["state"]=="complete":break
 tick+=1;time.sleep(30)
