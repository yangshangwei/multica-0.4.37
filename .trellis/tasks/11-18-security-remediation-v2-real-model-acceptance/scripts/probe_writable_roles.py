#!/usr/bin/env python3
"""Native app-server command-only tests for candidate C1/D1 role mounts.

This is a new deployment-policy probe, not the frozen P1 wrapper, and never
starts a model turn or reads an account. Every input and output is synthetic.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import queue
import shutil
import subprocess
import threading
import time
import uuid

HERE=Path(__file__).resolve().parent
IMAGE='sha256:f3ba3b59ec5b66ed823735c612885ef11479914091e976c7df3410a17525608f'
LABEL='multica.task=secrem-v2-real-20261120'


def git(repo,*args):
    return subprocess.check_output(['git','-c','core.hooksPath=/dev/null','-C',str(repo),*args],text=True).strip()


def run_role(root,role):
    trial=root/role;trial.mkdir();repo=trial/'repo';repo.mkdir();(repo/'tests').mkdir()
    (repo/'AGENTS.md').write_text('# Synthetic native role isolation probe\nNo model turn.\n')
    (repo/'app.py').write_text('answer = 1\n')
    (repo/'tests/test_reproduction.py').write_text('def test_synthetic():\n    assert True\n')
    for name in ['.agents','.codex','.aws','.azure','.config/gcloud','.kube','.ssh','.gnupg']:(repo/name).mkdir(parents=True,exist_ok=True)
    git(repo,'init','-q');git(repo,'config','user.name','Synthetic '+role);git(repo,'config','user.email',role+'@example.invalid');git(repo,'add','AGENTS.md','app.py','tests/test_reproduction.py');git(repo,'commit','-qm','Synthetic baseline')
    baseline=git(repo,'rev-parse','HEAD')
    home=trial/'home';home.mkdir();(home/'config.toml').touch()
    cfg=trial/'config.toml';cfg.write_text('model="synthetic-model"\napproval_policy="never"\nsandbox_mode="workspace-write"\nproject_doc_max_bytes=131072\n[sandbox_workspace_write]\nnetwork_access=false\nwritable_roots=["/role/.git"]\n[shell_environment_policy]\ninherit="none"\n[shell_environment_policy.set]\nPATH="/usr/local/bin:/usr/bin:/bin"\nHOME="/home/node"\nCODEX_HOME="/home/node/.codex"\n');
    if role == 'reviewer':
        cfg.write_text(cfg.read_text().replace('writable_roots=["/role/.git"]\n',''))
    cfg.chmod(0o444)
    requirements=HERE.parent/'fixtures/native-requirements.toml'
    container='secrem-v2-writable-'+uuid.uuid4().hex[:12]
    mounts=[(repo,'/role',role in {'verifier','reviewer'}),(home,'/home/node/.codex',False),(cfg,'/home/node/.codex/config.toml',True),(requirements,'/etc/codex/requirements.toml',True),(repo/'AGENTS.md','/role/AGENTS.md',True)]
    if role=='implementer':mounts.append((repo/'tests','/role/tests',True))
    if role=='verifier':mounts.extend([(repo/'tests','/role/tests',False),(repo/'.git','/role/.git',False)])
    args=['docker','run','-i','--rm','--pull=never','--name',container,'--label',LABEL,'--network=none','--read-only','--cap-drop=ALL','--security-opt=no-new-privileges','--security-opt=seccomp=unconfined',f'--user={os.getuid()}:{os.getgid()}','--pids-limit=128','--memory=1g','--tmpfs=/tmp:rw,nosuid,size=64m',f'--tmpfs=/home/node:rw,nosuid,size=32m,uid={os.getuid()},gid={os.getgid()}','--workdir=/role','-e','HOME=/home/node','-e','CODEX_HOME=/home/node/.codex']
    for source,target,ro in mounts:args.extend(['--mount',f'type=bind,src={source},dst={target}'+(',readonly' if ro else '')])
    args+=['--entrypoint','codex',IMAGE,'app-server','--listen','stdio://']
    result={'role':role,'evidence_type':'native-no-model-role-mount','model_turns':0,'baseline':baseline,'image':IMAGE,'config_sha256':hashlib.sha256(cfg.read_bytes()).hexdigest(),'requirements_sha256':hashlib.sha256(requirements.read_bytes()).hexdigest(),'mounts':[{'source':str(a),'target':b,'readonly':c} for a,b,c in mounts]}
    with (trial/'stderr.txt').open('wb') as errors:
        process=subprocess.Popen(args,stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=errors)
        messages=queue.Queue()
        def read():
            for line in process.stdout:
                try:messages.put(json.loads(line))
                except ValueError:messages.put({'invalid_json':True})
        threading.Thread(target=read,daemon=True).start();counter=0
        def rpc(method,params):
            nonlocal counter
            counter+=1;identifier=counter
            process.stdin.write((json.dumps({'id':identifier,'method':method,'params':params})+'\n').encode());process.stdin.flush();deadline=time.monotonic()+25
            while time.monotonic()<deadline:
                try:item=messages.get(timeout=.25)
                except queue.Empty:
                    if process.poll() is not None:raise RuntimeError('native process exited')
                    continue
                if item.get('id')==identifier:return item
            raise TimeoutError(method)
        def command(argv,policy=None):
            params={'command':argv,'cwd':'/role','timeoutMs':8000}
            if policy:params['sandboxPolicy']=policy
            return rpc('command/exec',params)
        try:
            result['initialize']=rpc('initialize',{'clientInfo':{'name':'role_probe','version':'1'},'capabilities':{'experimentalApi':True}})
            process.stdin.write(b'{"method":"initialized"}\n');process.stdin.flush()
            subprocess.run(['docker','exec','-d',container,'python3','-m','http.server','39872','--bind','127.0.0.1','--directory','/tmp'],check=True,capture_output=True,timeout=10)
            positive=subprocess.run(['docker','exec',container,'python3','-c','import urllib.request,time\nfor _ in range(20):\n try:print(urllib.request.urlopen("http://127.0.0.1:39872").status);break\n except OSError:time.sleep(.05)'],capture_output=True,text=True,timeout=10)
            result['live_peer']=positive.stdout.strip()
            test='import pathlib,json,os\np={"code":"/role/app.py","tests":"/role/tests/new_reproduction.py","instruction":"/role/AGENTS.md","config":"/home/node/.codex/config.toml","requirements":"/etc/codex/requirements.toml","peer":"/peer-role/secret","response":"/etc/passwd"}\nr={}\nfor k,v in p.items():\n try:pathlib.Path(v).write_text("synthetic = 2\\n");r[k]="writable"\n except OSError:r[k]="denied"\nprint(json.dumps(r))'
            result['writes']=command(['python3','-c',test])
            target='tests/new_reproduction.py' if role=='verifier' else 'app.py'
            result['git_commit']=command(['sh','-c',f'git -c core.hooksPath=/dev/null add -- {target} && git -c core.hooksPath=/dev/null commit -m "Synthetic native transport output" && git rev-parse HEAD'])
            network='import socket\ns=socket.socket();s.settimeout(1)\ntry:s.connect(("127.0.0.1",39872));print("BYPASS")\nexcept OSError as e:print("DENIED:"+type(e).__name__)'
            result['network']=command(['python3','-c',network])
            result['network_widened']=command(['python3','-c',network],{'type':'workspaceWrite','networkAccess':True})
            result['dangerous']=command(['true'],{'type':'dangerFullAccess'})
            process.stdin.close();process.wait(timeout=15);result['exit']=process.returncode
        except Exception as e:result['error']=type(e).__name__+': '+str(e)
        finally:
            subprocess.run(['docker','rm','-f',container],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL,timeout=15)
            if process.poll() is None:process.kill();process.wait()
    result['final_commit']=git(repo,'rev-parse','HEAD')
    try:write_results=json.loads(result.get('writes',{}).get('result',{}).get('stdout',''))
    except ValueError:write_results={}
    expected={'contributor':{'code':'writable','tests':'writable'},'implementer':{'code':'writable','tests':'denied'},'verifier':{'code':'denied','tests':'writable'},'reviewer':{'code':'denied','tests':'denied'}}[role]
    expected.update({k:'denied' for k in ['instruction','config','requirements','peer','response']})
    result['filesystem_passed']=write_results==expected
    result['network_passed']=result.get('live_peer')=='200' and all(result.get(k,{}).get('result',{}).get('stdout','').startswith('DENIED:') for k in ['network','network_widened']) and bool(result.get('dangerous',{}).get('error'))
    result['git_commit_passed']=(result.get('git_commit',{}).get('result',{}).get('exitCode')==0) if role!='reviewer' else (result.get('git_commit',{}).get('result',{}).get('exitCode')!=0)
    result['passed']=result['filesystem_passed'] and result['network_passed'] and result['git_commit_passed'] and result.get('exit')==0
    (trial/'result.json').write_text(json.dumps(result,indent=2)+'\n')
    return result


def main():
    parser=argparse.ArgumentParser();parser.add_argument('--out',required=True);parser.add_argument('--role',choices=['contributor','implementer','verifier','reviewer']);args=parser.parse_args()
    root=Path(args.out).resolve();root.mkdir(parents=True,exist_ok=False);root.chmod(0o700)
    roles=[args.role] if args.role else ['contributor','implementer','verifier','reviewer']
    results=[run_role(root,role) for role in roles]
    summary={'source_sha256':hashlib.sha256(Path(__file__).read_bytes()).hexdigest(),'roles':[{key:row[key] for key in ['role','passed','filesystem_passed','network_passed','git_commit_passed']} for row in results],'model_turns':0}
    (root/'summary.json').write_text(json.dumps(summary,indent=2)+'\n');print(json.dumps(summary));return 0 if all(x['passed'] for x in results) else 1

if __name__=='__main__':raise SystemExit(main())
