#!/usr/bin/env python3
"""Synthetic code-flow fixture; never used for real model acceptance."""
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import uuid

if os.environ.get('MULTICA_RUN_REAL_AGENT_SMOKE'):raise SystemExit('Fake refuses real opt-in')
if sys.argv[1:]==['--version']:
    print('codex-cli 0.164.0');raise SystemExit(0)
cfg=json.loads(Path(os.environ['SECREM_HARNESS_CONFIG']).read_text())
if cfg['mode']!='fake' or cfg['scenario'] not in {'C1','D1'} or cfg.get('execution_mode')!='official-cli-direct':raise SystemExit(3)
args=sys.argv[1:]
if args!=['exec','--json','--skip-git-repo-check','-'] and not(len(args)==6 and args[:4]==['exec','resume','--json','--skip-git-repo-check'] and args[-1]=='-'):raise SystemExit(4)
if not sys.stdin.buffer.read():raise SystemExit(5)
m=json.loads((Path(cfg['claim_manifest_dir'])/(os.environ['MULTICA_TASK_ID']+'.json')).read_text())
issue=m['issue_id'];phase=m['phase'];scenario=m['scenario'];thread=args[4] if args[1]=='resume' else str(uuid.uuid4())
def emit(obj):print(json.dumps(obj),flush=True)
def cli(argv):
    ident=str(uuid.uuid4());emit({'type':'item.started','item':{'id':ident,'type':'command_execution','command':'multica '+' '.join(argv[:3])}})
    p=subprocess.run([cfg['cli_path'],*argv],env=os.environ.copy(),capture_output=True,text=True)
    emit({'type':'item.completed','item':{'id':ident,'type':'command_execution','command':'multica '+' '.join(argv[:3]),'aggregated_output':p.stdout,'exit_code':p.returncode}})
    if p.returncode:raise RuntimeError('Synthetic real CLI operation failed: '+p.stderr)
    return json.loads(p.stdout) if p.stdout.strip().startswith(('{','[')) else p.stdout

def shell(argv,expect=0):
    ident=str(uuid.uuid4());emit({'type':'item.started','item':{'id':ident,'type':'command_execution','command':' '.join(argv)}})
    p=subprocess.run(argv,capture_output=True,text=True)
    emit({'type':'item.completed','item':{'id':ident,'type':'command_execution','command':' '.join(argv),'aggregated_output':p.stdout+p.stderr,'exit_code':p.returncode}})
    if p.returncode!=expect:raise RuntimeError('Synthetic command outcome differs')
    return p.stdout+p.stderr

def comment(text):
    argv=['issue','comment','add',issue,'--content',text,'--idempotency-key',str(uuid.uuid4()),'--output','json']
    if m.get('trigger_comment_id'):argv+=['--parent',m['trigger_comment_id']]
    return cli(argv)

def roster():
    text=(Path.cwd()/'AGENTS.md').read_text()
    rows=re.findall(r'\[@([^]]+)\]\(mention://agent/([a-f0-9-]+)\)',text)
    names={'security-reviewer':['安全审查','Security Reviewer'],'migration-reviewer':['迁移审查','Migration Reviewer'],'code-reviewer':['代码审查','Code Reviewer'],'implementer':['实现工程师','Implementation Engineer'],'qa-engineer':['测试工程师','QA Engineer'],'vulnerability-verifier':['漏洞验证','verification']}
    found={}
    for role,labels in names.items():
        found[role]=next((aid for name,aid in rows if any(label.lower() in name.lower() for label in labels)),None)
    return found

def delegate(roles,text):
    people=roster()
    tags=' '.join('[@'+role+'](mention://agent/'+people[role]+')' for role in roles)
    comment(tags+' '+text)
    cli(['squad','activity',issue,'action','--reason','Synthetic delegation recorded with actual receipt'])

emit({'type':'thread.started','thread_id':thread});emit({'type':'turn.started'})
cli(['issue','get',issue,'--output','json'])
try:
    if m['is_leader_task']:
        if phase=='intake':
            cli(['issue','update',issue,'--status','in_progress'])
            roles=['security-reviewer']+(['migration-reviewer'] if scenario=='D1' else [])
            delegate(roles,'G1分诊 type='+('code' if scenario=='C1' else 'dependency')+'，hold/unknown。[@安全负责人](mention://member/'+m['owner_ids'][0]+') 请读本轮结果后确认定级、处置、路由、期限、复现和知情范围。只用合成数据，不出站；不放宽停止规则。')
        elif phase=='G1-release-leader':
            cli(['issue','comment','list',issue,'--output','json'])
            delegate(['vulnerability-verifier'] if scenario=='C1' else ['implementer'],'负责人本次放行已通过校验。基线提交'+m['baseline_commit']+'，只交付本角色工作；复现只写本地tests，补丁不得修改复现测试。')
        elif phase=='reproduction-return-leader':delegate(['implementer'],'独立失败测试已交付；在提交'+m['baseline_commit']+'上修复read_issue，不改tests，不修read_summary变体。')
        elif phase=='repair-return-leader':delegate(['vulnerability-verifier'] if scenario=='C1' else ['qa-engineer'],'在修复提交'+m['baseline_commit']+'重跑同一测试和回归，不修改代码或测试。')
        elif phase=='verification-return-leader':delegate(['qa-engineer'],'独立重跑后做回归，修复提交'+m['baseline_commit']+'，保持安全停止规则。')
        elif phase=='regression-return-leader':
            roles=['code-reviewer','security-reviewer']+(['migration-reviewer'] if scenario=='D1' else [])
            delegate(roles,'G2请独立复审提交'+m['baseline_commit']+'的补丁、测试、变体与回移。[@安全负责人](mention://member/'+m['owner_ids'][0]+') 请读全部新结果后放行，不能跳过审查。')
        elif phase=='closing':
            cli(['issue','comment','list',issue,'--output','json'])
            if scenario=='C1':cli(['issue','create','--title','SEC-F1-V1','--description','Synthetic follow-up variant linked to '+issue,'--status','backlog','--output','json'])
            comment('本次G2放行已通过校验。收口仅限合成提交'+m['baseline_commit']+'，按原评论、作者、提交与真实输出对账；不作生产安全结论。变体已按负责人决定处理，回移none。')
            cli(['squad','activity',issue,'action','--reason','Synthetic evidence and owner release reconciled']);cli(['issue','update',issue,'--status','in_review'])
        else:raise RuntimeError('Unknown synthetic leader phase')
    elif m['role']=='vulnerability-verifier' and m['role_mode']=='verifier':
        (Path.cwd()/'tests/test_security_property.py').write_text('import unittest\nfrom workspace_reader import IssueRow,read_issue\nclass IndependentProperty(unittest.TestCase):\n def test_cross_workspace_denied_before_audit(self):\n  audit=[]\n  self.assertIsNone(read_issue({"x":IssueRow("one","synthetic")},"x","two",audit))\n  self.assertEqual(audit,[])\n')
        output=shell(['python3','-m','unittest','discover','-s','tests','-v'],1)
        shell(['git','add','tests/test_security_property.py']);shell(['git','commit','-m','SEC-F1 reproduction'])
        head=shell(['git','rev-parse','HEAD']).strip()
        comment('独立复现：tests/test_security_property.py，原基线'+m['baseline_commit']+'，测试提交'+head+'。实际命令python3 -m unittest discover -s tests -v退出1：\n'+output+'\n检索read_读法：read_issue可跨workspace读取并写audit；read_summary同样缺少边界，作为独立变体。没有补丁、没有出站。')
    elif m['role']=='implementer':
        if scenario=='C1':
            p=Path('workspace_reader.py');text=p.read_text();text=text.replace('if row is None:\n        return None','if row is None or row.workspace_id != workspace_id:\n        return None',1);p.write_text(text)
            shell(['git','add','workspace_reader.py'])
        else:
            lock=Path('dependency.lock.json');data=json.loads(lock.read_text());data['version']='2.0.0';data['source']='vendor/labelkit_v2.py';data['sha256']=hashlib.sha256(Path(data['source']).read_bytes()).hexdigest();lock.write_text(json.dumps(data,indent=2)+'\n')
            p=Path('application.py');text=p.read_text().replace('return dependency.format_label(value).label','return dependency.format_label(value).text');p.write_text(text)
            shell(['git','add','application.py','dependency.lock.json'])
        output=shell(['python3','-m','unittest','discover','-s','tests','-v'])
        shell(['git','commit','-m','SEC-F1 minimal repair']);head=shell(['git','rev-parse','HEAD']).strip()
        comment('最小修复提交'+head+'；复现测试未修改。实际回归退出0：\n'+output+'\n未处理独立变体，未执行生产操作。')
    elif m['role'] in {'qa-engineer','vulnerability-verifier'}:
        output=shell(['python3','-m','unittest','discover','-s','tests','-v']);comment('独立验证提交'+m['baseline_commit']+'；同一测试原字节不变，真实命令退出0：\n'+output+'\n没有修改补丁或测试。')
    else:
        text='本轮独立审查提交'+m['baseline_commit']+'。只读合成仓库，审查来源与边界，未写补丁、不触发其他智能体。'
        if scenario=='D1':text+='本地版本/锁文件和依赖路径已核对；返回字段兼容变化需保持application.render_label返回str。扫描输出：'+shell(['python3','synthetic_scan.py'])
        else:text+='read_issue边界需先于audit；read_summary独立变体，复现者和修改者不同。'
        comment(text)
    emit({'type':'item.completed','item':{'id':str(uuid.uuid4()),'type':'agent_message','text':'Scripted code-route result; not model evidence.'}})
    emit({'type':'turn.completed','usage':{'input_tokens':0,'cached_input_tokens':0,'output_tokens':0}})
except Exception as error:
    emit({'type':'turn.failed','error':{'message':type(error).__name__+': '+str(error)}})
    raise SystemExit(7)
