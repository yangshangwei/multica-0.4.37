#!/usr/bin/env python3
"""Actual official codex exec against a network-none scripted Responses peer."""
import argparse
import hashlib
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import os
from pathlib import Path
import subprocess
import threading
import time
import uuid

SCRIPTS=Path(__file__).resolve().parent
IMAGE="sha256:f3ba3b59ec5b66ed823735c612885ef11479914091e976c7df3410a17525608f"


def run_role(root, role_mode):
    scratch=False
    root.mkdir(mode=0o700,parents=True,exist_ok=False)
    work=root/"role";work.mkdir(mode=0o755)
    home=root/"codex";home.mkdir(mode=0o755)
    claims=root/"claims";claims.mkdir(mode=0o700)
    task,agent,workspace,issue,squad=[str(uuid.uuid4()) for _ in range(5)]
    agents="# Synthetic direct CLI proof\nUse the registered local tools.\n"
    (work/"AGENTS.md").write_text(agents)
    (work/"app.py").write_text("synthetic = 1\n")
    (work/"tests").mkdir();(work/"tests/test_base.py").write_text("def test_base(): pass\n")
    for args in [["init","-q"],["config","user.name","Synthetic "+role_mode],["config","user.email",role_mode+"@example.invalid"],["add","app.py","tests"],["commit","-qm","Synthetic baseline"]]:
        subprocess.run(["git","-C",str(work),*args],check=True,capture_output=True)
    baseline=subprocess.check_output(["git","-C",str(work),"rev-parse","HEAD"],text=True).strip()
    if scratch:
        (work/'.agent_context').mkdir();(work/'.agent_context/issue_context.md').write_text('Synthetic immutable task context')
        (work/'.multica').mkdir();(work/'.multica/daemon_task_context.json').write_text('{}')
        (work/'drill.json').write_text('{"synthetic":true,"deployment_exists":false}')
    else:
        (work/"code.py").write_text("synthetic = True\n")
    manifest={"task_id":task,"agent_id":agent,"workspace_id":workspace,"issue_id":issue,"squad_id":squad,"role":"security-reviewer","restricted":True,"is_leader_task":False,"read_only_code":True}
    if scratch:
        manifest.update(role='security-remediation-lead',restricted=False,is_leader_task=True,no_repository=True,repo_count=0,project_resource_count=0,immutable_files={'drill.json':hashlib.sha256((work/'drill.json').read_bytes()).hexdigest()})
    role = {"implementer":"implementer", "verifier":"vulnerability-verifier", "verifier_recheck":"vulnerability-verifier", "qa":"qa-engineer", "reviewer":"security-reviewer", "analysis":"product-analyst"}[role_mode]
    manifest.update(role=role,restricted=role_mode=="reviewer",no_repository=False,repo_root=str(work),baseline_commit=baseline,role_mode=role_mode,read_only_code=role_mode!="implementer",protected_test_dir="tests",protected_test_sha256={"tests/test_base.py":hashlib.sha256((work/"tests/test_base.py").read_bytes()).hexdigest()})
    (claims/(task+".json")).write_text(json.dumps(manifest))
    calls=[]
    class API(BaseHTTPRequestHandler):
        def log_message(self,*args):pass
        def do_GET(self):
            self.reply()
        def do_POST(self):
            self.reply()
        def reply(self):
            body=self.rfile.read(int(self.headers.get('Content-Length','0'))).decode()
            calls.append({"method":self.command,"path":self.path,"body":body,"task_identity":self.headers.get("Authorization")=="Bearer mat_synthetic_transport","workspace":self.headers.get("X-Workspace-ID")==workspace})
            self.send_response(200);self.send_header("Content-Type","application/json");self.end_headers();self.wfile.write(json.dumps({"id":issue,"synthetic":True}).encode())
    api=ThreadingHTTPServer(("127.0.0.1",0),API);threading.Thread(target=api.serve_forever,daemon=True).start()
    cli=root/"fake-multica";cli.write_text('#!/usr/bin/env python3\nimport urllib.request,os,sys\nr=urllib.request.Request(os.environ["MULTICA_SERVER_URL"]+"/api/issues/"+sys.argv[3],headers={"Authorization":"Bearer "+os.environ["MULTICA_TOKEN"],"X-Workspace-ID":os.environ["MULTICA_WORKSPACE_ID"]})\nprint(urllib.request.urlopen(r).read().decode())\n');cli.chmod(0o700)
    if scratch:
        cli.write_text('''#!/usr/bin/env python3
import urllib.request,os,sys
args=sys.argv[1:]
if args[-1]=='--help':print('Synthetic declared command help');raise SystemExit(0)
path='/api/squads' if args[:2]==['squad','list'] else '/api/issues/'+(args[3] if args[:3]==['issue','comment','add'] else args[2])
data=sys.stdin.buffer.read() if '--content-stdin' in args else None
if data is not None:path+='/comments'
r=urllib.request.Request(os.environ['MULTICA_SERVER_URL']+path,data=data,headers={'Authorization':'Bearer '+os.environ['MULTICA_TOKEN'],'X-Workspace-ID':os.environ['MULTICA_WORKSPACE_ID']})
print(urllib.request.urlopen(r).read().decode())
''')
    token=root/"synthetic-proxy-token";token.write_text("synthetic-only");token.chmod(0o600)
    wrapper=SCRIPTS/"codex_exec_code_container.py"
    reg={"schema_version":1,"mode":"fake","enabled":False,"transport_mode":"official_cli_exec_code","model":"synthetic-model","max_starts":1,"max_model_requests":10,"max_seconds":180,"per_start_seconds":150,"expires_at":time.time()+300,"ledger":str(root/"ledger.json"),"argv":[str(wrapper),"exec","--json","--skip-git-repo-check","-"],"executable_sha256":hashlib.sha256(wrapper.read_bytes()).hexdigest(),"cwd":str(work),"source_hashes":{},"environment":{}}
    p=root/"registration.json";p.write_text(json.dumps(reg));p.chmod(0o600)
    endpoint=root/"fake_code_exec_responses.py"
    endpoint.write_bytes((SCRIPTS/"fake_code_exec_responses.py").read_bytes());endpoint.chmod(0o600)
    cfg={"schema_version":1,"transport_mode":"official_cli_exec_code","image":IMAGE,"evidence_root":str(root),"allowed_workspaces_root":str(work),"claim_manifest_dir":str(claims),"cli_path":str(cli),"cli_sha256":hashlib.sha256(cli.read_bytes()).hexdigest(),"model":"synthetic-model","model_gateway_url":"http://127.0.0.1:39091/model","model_proxy_token_file":str(token),"network":"none","registration_file":str(p),"registration_sha256":hashlib.sha256(p.read_bytes()).hexdigest(),"fake_endpoint_script":str(endpoint),"fake_endpoint_sha256":hashlib.sha256(endpoint.read_bytes()).hexdigest()}
    conf=root/"transport.json";conf.write_text(json.dumps(cfg));conf.chmod(0o600)
    env=os.environ.copy();env.pop("MULTICA_RUN_REAL_AGENT_SMOKE",None);env.pop("SECREM_REAL_EXECUTION_AUTHORIZED",None)
    env.update(SECREM_TRANSPORT_CONFIG=str(conf),CODEX_HOME=str(home),MULTICA_TASK_ID=task,MULTICA_AGENT_ID=agent,MULTICA_WORKSPACE_ID=workspace,MULTICA_AGENT_NAME="Synthetic reviewer",MULTICA_TOKEN="mat_synthetic_transport",MULTICA_SERVER_URL=f"http://127.0.0.1:{api.server_port}")
    prompt="Synthetic fixture input retained verbatim. No real provider is called.\n"
    try:
        result=subprocess.run(reg["argv"],input=prompt.encode(),cwd=work,env=env,capture_output=True,timeout=175)
        (root/"stdout.jsonl").write_bytes(result.stdout);(root/"stderr.txt").write_bytes(result.stderr)
        run=next((root/"cli-transport").iterdir())
        metadata=json.loads((run/"metadata.json").read_text())
        responses=json.loads((run/"codex-home/fake-responses.json").read_text())
        control=json.loads((run/"codex-home/fake-peer-control.json").read_text())
        tool_outputs=[v for request in responses for v in request["body"].get("input",[]) if v.get("type")=="function_call_output"]
        native_ua=responses[0]["headers"].get("User-Agent",responses[0]["headers"].get("user-agent"))
        out={"evidence_type":"scripted-official-cli-not-model","provider_requests":0,"exit":result.returncode,"metadata":metadata,"synthetic_request_count":len(responses),"native_user_agent":native_ua,"api_calls":calls,"live_peer_control":control,"tool_outputs":tool_outputs,"source_hashes":{str(p):hashlib.sha256(p.read_bytes()).hexdigest() for p in [wrapper,endpoint,Path(__file__),SCRIPTS/"codex_container.py",SCRIPTS/"multica_spool.py"]}}
        outputs={x.get("call_id"):x.get("output","") for x in tool_outputs}
        common_pass=result.returncode==0 and metadata["outcome"]=="completed" and metadata["agents_unchanged"] and metadata.get("protected_tests_unchanged") and (run/"prompt.txt").read_text()==prompt and len(calls)==1 and all(row['task_identity'] and row['workspace'] for row in calls) and control["status"]==200 and "codex_exec" in str(native_ua)
        raw_write=outputs.get('call_synthetic_2','')
        json_start=raw_write.find('{')
        writes=json.loads(raw_write[json_start:]) if json_start>=0 else {}
        expected={"code":"WRITABLE" if role_mode=="implementer" else "DENIED", "new_test":"WRITABLE" if role_mode=="verifier" else "DENIED", "baseline_test":"DENIED", "agents":"DENIED"}
        out["write_matrix"]=writes;out["expected_write_matrix"]=expected
        head=subprocess.check_output(['git','-C',str(work),'rev-parse','HEAD'],text=True).strip()
        out["git_changed"]=head!=baseline
        expected_git_change=role_mode in {"implementer","verifier"}
        out["passed"]=common_pass and writes==expected and out["git_changed"]==expected_git_change and 'DENIED' in outputs.get('call_synthetic_4','') and 'ESCALATED_BYPASS' not in outputs.get('call_synthetic_5','ESCALATED_BYPASS') and bool(outputs.get('call_synthetic_5'))
    except Exception as error:
        out={"passed":False,"provider_requests":0,"error":type(error).__name__+": "+str(error)}
    finally:
        api.shutdown();api.server_close()
    (root/"result.json").write_text(json.dumps(out,indent=2)+"\n")
    print(json.dumps({"passed":out["passed"],"result":str(root/"result.json"),"error":out.get("error")}))
    return 0 if out["passed"] else 1


if __name__=="__main__":
    parser=argparse.ArgumentParser();parser.add_argument('root',type=Path);parser.add_argument('--role',choices=['implementer','verifier','verifier_recheck','qa','reviewer','analysis']);args=parser.parse_args()
    root=args.root.resolve();root.mkdir(mode=0o700,parents=True,exist_ok=False)
    roles=[args.role] if args.role else ['implementer','verifier','verifier_recheck','qa','reviewer']
    codes=[run_role(root/role,role) for role in roles]
    summary={'roles':[{"role":role,"passed":code==0} for role,code in zip(roles,codes)],'provider_requests':0}
    (root/'summary.json').write_text(json.dumps(summary,indent=2)+'\n');print(json.dumps(summary))
    raise SystemExit(0 if all(code==0 for code in codes) else 1)
