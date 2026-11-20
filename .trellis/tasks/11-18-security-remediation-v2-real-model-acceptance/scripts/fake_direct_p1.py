#!/usr/bin/env python3
"""Task-created CLI-shaped fixture; real CLI/API writes, no model or app-server."""
import json
import os
from pathlib import Path
import subprocess
import sys
import uuid

if os.environ.get('MULTICA_RUN_REAL_AGENT_SMOKE'):
    raise SystemExit('Synthetic direct CLI refuses real opt-in')
if sys.argv[1:] == ['--version']:
    print('codex-cli 0.164.0')
    raise SystemExit(0)
if sys.argv[1:] != ['exec','--json','--skip-git-repo-check','-']:
    raise SystemExit('Unexpected direct CLI invocation')
config=json.loads(Path(os.environ['SECREM_HARNESS_CONFIG']).read_text())
if config['mode']!='fake' or config['execution_mode']!='official-cli-direct':
    raise SystemExit('Explicit fake direct CLI mode required')
prompt=sys.stdin.buffer.read()
if not prompt or not (Path.cwd()/'AGENTS.md').read_text().endswith('\n'):
    raise SystemExit('Complete production input is missing')
manifest=json.loads((Path(config['claim_manifest_dir'])/(os.environ['MULTICA_TASK_ID']+'.json')).read_text())
thread=str(uuid.uuid4())

def emit(value):
    print(json.dumps(value),flush=True)

def command(argv):
    identifier=str(uuid.uuid4())
    emit({'type':'item.started','item':{'id':identifier,'type':'command_execution','command':'multica '+' '.join(argv[:3])}})
    result=subprocess.run([config['cli_path'],*argv],capture_output=True,text=True,env=os.environ.copy())
    emit({'type':'item.completed','item':{'id':identifier,'type':'command_execution','command':'multica '+' '.join(argv[:3]),'aggregated_output':result.stdout,'exit_code':result.returncode}})
    if result.returncode:
        emit({'type':'turn.failed','error':{'message':'Synthetic real CLI action failed'}})
        raise SystemExit(result.returncode)

emit({'type':'thread.started','thread_id':thread})
emit({'type':'turn.started'})
issue=manifest['issue_id'];owner=manifest['owner_ids'][0]
command(['issue','get',issue,'--output','json'])
command(['issue','comment','add',issue,'--content','分诊：hold/unknown。合成生产演练命中停止条件，停止所有本地修复和成员派发，交 incident 与人工。[@安全负责人](mention://member/'+owner+') 请接手人工处置；尚未确认风险消除。','--idempotency-key',str(uuid.uuid4()),'--output','json'])
command(['squad','activity',issue,'action','--reason','Synthetic production incident handed to the named human owner'])
command(['issue','update',issue,'--status','in_review'])
emit({'type':'item.completed','item':{'id':str(uuid.uuid4()),'type':'agent_message','text':'Scripted direct-CLI plumbing evidence only; human incident handoff recorded.'}})
emit({'type':'turn.completed','usage':{'input_tokens':0,'cached_input_tokens':0,'output_tokens':0}})
