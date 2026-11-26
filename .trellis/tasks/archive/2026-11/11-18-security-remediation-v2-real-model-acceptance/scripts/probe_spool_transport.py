#!/usr/bin/env python3
"""No model turn: exercise the actual wrapper, native sandbox and CLI spool."""
import argparse
import base64
import hashlib
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import os
from pathlib import Path
import queue
import select
import subprocess
import threading
import time
import uuid

HERE=Path(__file__).resolve().parent
IMAGE='sha256:f3ba3b59ec5b66ed823735c612885ef11479914091e976c7df3410a17525608f'


def main(root):
    root.mkdir(parents=True,exist_ok=False);root.chmod(0o700)
    work=root/'role';work.mkdir(mode=0o755)
    home=root/'codex';home.mkdir(mode=0o755)
    claims=root/'claims';claims.mkdir(mode=0o700)
    peer=root/'peer-role';peer.mkdir(mode=0o700);(peer/'canary').write_text('synthetic-unmounted-peer')
    task,agent,workspace,issue,squad=[str(uuid.uuid4()) for _ in range(5)]
    (work/'AGENTS.md').write_text('# Synthetic transport probe\nNo model turn is allowed.\n')
    (work/'code.py').write_text('synthetic = True\n')
    manifest=dict(task_id=task,agent_id=agent,workspace_id=workspace,issue_id=issue,squad_id=squad,role='security-reviewer',restricted=True,is_leader_task=False,read_only_code=True)
    (claims/(task+'.json')).write_text(json.dumps(manifest))
    calls=[]
    class API(BaseHTTPRequestHandler):
        def log_message(self,*a):pass
        def do_GET(self):
            calls.append({'path':self.path,'authorization_correct':self.headers.get('Authorization')=='Bearer mat_synthetic_transport','workspace_correct':self.headers.get('X-Workspace-ID')==workspace})
            body=json.dumps({'id':issue,'actor':'synthetic-agent','original_task_identity':all(calls[-1][x] for x in ['authorization_correct','workspace_correct'])}).encode();self.send_response(200);self.send_header('Content-Length',str(len(body)));self.end_headers();self.wfile.write(body)
    api=ThreadingHTTPServer(('127.0.0.1',0),API);threading.Thread(target=api.serve_forever,daemon=True).start()
    fake_cli=root/'fake-multica'
    fake_cli.write_text('#!/usr/bin/env python3\nimport urllib.request,json,os,sys\nu=os.environ["MULTICA_SERVER_URL"]+"/api/issues/"+sys.argv[3]\nr=urllib.request.Request(u,headers={"Authorization":"Bearer "+os.environ["MULTICA_TOKEN"],"X-Workspace-ID":os.environ["MULTICA_WORKSPACE_ID"]})\nprint(urllib.request.urlopen(r).read().decode())\n')
    fake_cli.chmod(0o700)
    token=root/'proxy.token';token.write_text('synthetic-proxy-only');token.chmod(0o600)
    wrapper=HERE/'codex_container.py'
    registration={'schema_version':1,'mode':'fake','enabled':False,'model':'synthetic-model','max_starts':1,'max_model_requests':1,'max_seconds':180,'per_start_seconds':150,'expires_at':time.time()+300,'ledger':str(root/'ledger.json'),'argv':[str(wrapper),'app-server','--listen','stdio://'],'executable_sha256':hashlib.sha256(wrapper.read_bytes()).hexdigest(),'cwd':str(work),'source_hashes':{},'environment':{}}
    reg=root/'registration.json';reg.write_text(json.dumps(registration));reg.chmod(0o600)
    config={'schema_version':1,'image':IMAGE,'evidence_root':str(root),'allowed_workspaces_root':str(work),'claim_manifest_dir':str(claims),'cli_path':str(fake_cli),'cli_sha256':hashlib.sha256(fake_cli.read_bytes()).hexdigest(),'model':'synthetic-model','model_gateway_url':'http://127.0.0.1:1/model','model_proxy_token_file':str(token),'network':'none','registration_file':str(reg),'registration_sha256':hashlib.sha256(reg.read_bytes()).hexdigest()}
    conf=root/'transport.json';conf.write_text(json.dumps(config));conf.chmod(0o600)
    environment=os.environ.copy();environment.pop('MULTICA_RUN_REAL_AGENT_SMOKE',None)
    environment.update(SECREM_TRANSPORT_CONFIG=str(conf),CODEX_HOME=str(home),MULTICA_TASK_ID=task,MULTICA_AGENT_ID=agent,MULTICA_WORKSPACE_ID=workspace,MULTICA_AGENT_NAME='Synthetic reviewer',MULTICA_TOKEN='mat_synthetic_transport',MULTICA_SERVER_URL=f'http://127.0.0.1:{api.server_port}')
    result={'evidence_type':'native-no-model-spool','model_turns':0,'provider_requests':0,'source_hashes':{p.name:hashlib.sha256(p.read_bytes()).hexdigest() for p in [wrapper,HERE/'multica_spool.py',Path(__file__)]}}
    with (root/'wrapper.stderr').open('wb') as err:
        proc=subprocess.Popen([str(wrapper),'app-server','--listen','stdio://'],cwd=work,env=environment,stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=err)
        messages=queue.Queue()
        def read():
            for line in proc.stdout:
                try:messages.put(json.loads(line))
                except ValueError:messages.put({'invalid':True})
        threading.Thread(target=read,daemon=True).start()
        counter=0
        def rpc(method,params):
            nonlocal counter
            counter+=1;identifier=counter
            proc.stdin.write((json.dumps({'id':identifier,'method':method,'params':params})+'\n').encode());proc.stdin.flush()
            deadline=time.monotonic()+35
            while time.monotonic()<deadline:
                try:msg=messages.get(timeout=1)
                except queue.Empty:
                    if proc.poll() is not None:raise RuntimeError('wrapper exited')
                    continue
                if msg.get('id')==identifier:return msg
            raise TimeoutError(method)
        try:
            result['initialize']=rpc('initialize',{'clientInfo':{'name':'spool_probe','version':'1'},'capabilities':{'experimentalApi':True}})
            proc.stdin.write(b'{"method":"initialized"}\n');proc.stdin.flush()
            result['requirements']=rpc('configRequirements/read',{})
            def command(argv,extra=None):
                params={'command':argv,'cwd':str(work),'timeoutMs':12000};params.update(extra or {});return rpc('command/exec',params)
            transport_dir=next((root/'transport').iterdir())
            container=json.loads((transport_dir/'metadata.json').read_text())['container']
            subprocess.run(['docker','exec','-d',container,'python3','-m','http.server','39871','--bind','127.0.0.1','--directory','/tmp'],check=True,capture_output=True,timeout=15)
            positive=subprocess.run(['docker','exec',container,'python3','-c','import urllib.request,time\nfor _ in range(20):\n try: print(urllib.request.urlopen("http://127.0.0.1:39871").status);break\n except OSError:time.sleep(.05)'],capture_output=True,text=True,timeout=15)
            result['live_peer_control']={'exit':positive.returncode,'stdout':positive.stdout}
            result['allowed_cli']=command(['multica','issue','get',issue,'--output','json'])
            result['outside_cli']=command(['multica','issue','get',str(uuid.uuid4())])
            result['status_cli']=command(['multica','issue','status',issue,'done'])
            response_dir=next(work.glob('.secrem-responses-*'))
            request_dir=next(work.glob('.secrem-requests-*'))
            fileprobe='import pathlib,json,os\npaths='+repr([str(work/'code.py'),str(home/'config.toml'),'/etc/codex/requirements.toml',str(response_dir/'forged.json')])+'\nout={}\nfor p in paths:\n try:\n  pathlib.Path(p).write_text("MUTATED");out[p]="BYPASS"\n except OSError:out[p]="DENIED"\ntry:\n pathlib.Path('+repr(str(peer/'canary'))+').read_text();out["peer_read"]="BYPASS"\nexcept OSError:out["peer_read"]="DENIED"\nfor p in '+repr([str(response_dir),str(request_dir)])+':\n try:os.rename(p,p+".moved");out[p+"/rename"]="BYPASS"\n except OSError:out[p+"/rename"]="DENIED"\nprint(json.dumps(out))'
            result['file_write_denials']=command(['python3','-c',fileprobe])
            result['dangerous']=command(['true'],{'sandboxPolicy':{'type':'dangerFullAccess'}})
            networkprobe='import socket\ns=socket.socket();s.settimeout(2)\ntry:s.connect(("127.0.0.1",39871));print("BYPASS")\nexcept OSError as e:print("DENIED:"+type(e).__name__)'
            result['network_ordinary']=command(['python3','-c',networkprobe])
            result['network_widened']=command(['python3','-c',networkprobe],{'sandboxPolicy':{'type':'workspaceWrite','networkAccess':True}})
            proc.stdin.close();proc.wait(timeout=25)
            result['wrapper_exit']=proc.returncode
        except Exception as error:
            result['error']=type(error).__name__+': '+str(error)
            proc.terminate()
            try:proc.wait(timeout=30)
            except subprocess.TimeoutExpired:proc.kill();proc.wait()
    api.shutdown()
    result['api_calls']=calls
    result['source_unchanged']=(work/'code.py').read_text()=='synthetic = True\n'
    allowed=result.get('allowed_cli',{}).get('result',{})
    denials=result.get('file_write_denials',{}).get('result',{})
    try:
        denial_map=json.loads(denials.get('stdout',''))
    except ValueError:
        denial_map={}
    ledger=json.loads((root/'ledger.json').read_text())
    terminal=bool(ledger['starts']) and ledger['starts'][-1].get('outcome')=='completed'
    result['passed']=allowed.get('exitCode')==0 and len(calls)==1 and all(calls[0][x] for x in ['authorization_correct','workspace_correct']) and result.get('outside_cli',{}).get('result',{}).get('exitCode')==126 and result.get('status_cli',{}).get('result',{}).get('exitCode')==126 and denials.get('exitCode')==0 and len(denial_map)==7 and all(v=='DENIED' for v in denial_map.values()) and bool(result.get('dangerous',{}).get('error')) and result['source_unchanged'] and all(result.get(k,{}).get('result',{}).get('stdout','').startswith('DENIED:') for k in ['network_ordinary','network_widened']) and result.get('live_peer_control',{}).get('stdout','').strip()=='200' and result.get('wrapper_exit')==0 and terminal
    (root/'result.json').write_text(json.dumps(result,indent=2)+'\n')
    print(json.dumps({'path':str(root/'result.json'),'passed':result['passed'],'api_calls':len(calls),'error':result.get('error')}))
    return 0 if result['passed'] else 1


if __name__=='__main__':
    parser=argparse.ArgumentParser();parser.add_argument('--out',required=True);args=parser.parse_args();raise SystemExit(main(Path(args.out).resolve()))
