import json,urllib.request,urllib.error,subprocess,pathlib,uuid,os,hashlib,time
p=pathlib.Path(__file__).parent;api='http://localhost:18251';setup=json.loads((p/'fixture-setup.json').read_text());auth=json.loads((p/'synthetic-owner.json').read_text());token=auth['token'];ws=setup['workspace'];record=[]
url=(p/'rnd_m1_targeted_api01.env').read_text().strip().split('=',1)[1]
def sql(q):
 r=subprocess.run(['/opt/homebrew/Cellar/libpq/18.6/bin/psql',url,'-X','-A','-t','-v','ON_ERROR_STOP=1','-c',q],capture_output=True,text=True)
 if r.returncode:(p/'leader-sql-error.log').write_text(r.stderr);raise RuntimeError('SQL failure retained privately')
 return r.stdout.strip()
assert sql('select current_database()')=='rnd_m1_targeted_api01'
def save(name,data):
 f=p/name;f.write_text(json.dumps(data,ensure_ascii=False,indent=2)+'\n');f.chmod(0o600)
def call(method,path,body=None,tok=token,workspace=ws):
 headers={'Content-Type':'application/json'}
 if tok:headers['Authorization']='Bearer '+tok
 if workspace:headers['X-Workspace-ID']=workspace
 req=urllib.request.Request(api+path,data=None if body is None else json.dumps(body).encode(),method=method,headers=headers)
 try:
  with urllib.request.urlopen(req,timeout=30) as r:status=r.status;raw=r.read()
 except urllib.error.HTTPError as e:status=e.code;raw=e.read()
 record.append({'method':method,'path':path,'status':status});save('leader-fixture-requests.json',record)
 if not 200<=status<300:save('leader-fixture-api-failure.json',{'path':path,'status':status,'body':raw.decode()});raise RuntimeError('Leader fixture API failure '+str(status)+' '+path)
 return json.loads(raw) if raw else None
email='m1-private-'+uuid.uuid4().hex[:8]+'@example.test'
call('POST','/auth/send-code',{'email':email},None,None)
second=call('POST','/auth/verify-code',{'email':email,'code':'888888'},None,None);tok2=second['token'];uid2=second['user']['id'];save('synthetic-private-owner.json',second)
sql("UPDATE \"user\" SET onboarded_at=now(),onboarding_questionnaire='{}'::jsonb WHERE id='"+uid2+"'")
invite=call('POST','/api/workspaces/'+ws+'/members',{'email':email,'role':'member'})
call('POST','/api/invitations/'+invite['id']+'/accept',None,tok2,None)
def runtime(label,tok):
 daemon=uuid.uuid4().hex
 r=call('POST','/api/daemon/register',{'workspace_id':ws,'daemon_id':daemon,'device_name':'m1-'+label,'cli_version':'0.6.0','runtimes':[{'name':'Synthetic '+label,'type':'codex','version':'0.0.0-targeted','status':'online'}]},tok,None)
 return r['runtimes'][0]['id'],daemon
rt,daemon=runtime('leader-runtime',token);rt2,_=runtime('private-worker',tok2)
leader=call('POST','/api/agents',{'name':'Targeted Receipt Leader','runtime_id':rt,'instructions':'You coordinate delegated work only. Preserve current in_progress issue status for this trial. Do not implement code or mark work completed. Inspect actual per-target receipts and record an honest evaluation.'})
allowed=call('POST','/api/agents',{'name':'Allowed Synthetic Worker','runtime_id':rt,'permission_mode':'private'})
blocked=call('POST','/api/agents',{'name':'Private Synthetic Worker','runtime_id':rt2,'permission_mode':'private'},tok2,ws)
squad=call('POST','/api/squads',{'name':'Targeted receipt squad','leader_id':leader['id'],'description':'Synthetic mixed permission delegation validation'})
for a,role in [(allowed,'implementation specialist'),(blocked,'test specialist')]:call('POST','/api/squads/'+squad['id']+'/members',{'member_type':'agent','member_id':a['id'],'role':role})
description='This is one bounded delegation-only turn. Delegate a synthetic read-only source inspection to BOTH named workers in one comment: Allowed Synthetic Worker and Private Synthetic Worker. The private worker may be refused by the server; judge actual trigger_outcomes and record failed with the handled subset if any target is blocked or unconfirmed. Do not retry a saved comment. Do not perform the inspection yourself. The issue is already in_progress; preserve its status.'
issue=call('POST','/api/issues',{'title':'M1 leader mixed receipt behavior trial','description':description,'status':'in_progress','project_id':setup['project'],'assignee_type':'squad','assignee_id':squad['id']})
call('POST','/api/daemon/heartbeat',{'runtime_id':rt},token,None)
claim=call('POST','/api/daemon/runtimes/'+rt+'/tasks/claim',None,token,None)['task'];assert claim and claim['issue_id']==issue['id'] and claim['is_leader_task']
call('POST','/api/daemon/tasks/'+claim['id']+'/start',None,token,None)
cwd=p/'leader-cwd';cwd.mkdir(mode=0o700);home=p/'leader-home';home.mkdir(mode=0o700)
proxy=json.loads((p/'trace-proxy.json').read_text())
env={'MULTICA_SERVER_URL':'http://127.0.0.1:'+str(proxy['port']),'MULTICA_WORKSPACE_ID':ws,'MULTICA_TOKEN':claim['auth_token'],'MULTICA_TASK_ID':claim['id'],'MULTICA_AGENT_ID':leader['id'],'MULTICA_AGENT_NAME':'Targeted Receipt Leader','MULTICA_TASK_CONFIG_ROOT':str(p/'leader-cli-config'),'HOME':str(home),'PATH':str(p/'bin')+':/usr/bin:/bin:/usr/sbin:/sbin'}
instructions=claim['agent']['instructions'];assert 'trigger_outcomes' in instructions and allowed['id'] in instructions and blocked['id'] in instructions
prompt='You are performing one synthetic Multica leader task. Use the actual CLI '+str(p/'bin/multica')+'. Only interact with the supplied Multica test API via the CLI; never access other host files/accounts/secrets/services. Credentials are supplied in the environment; do not print them.\n\n## Actual server-claimed leader instructions\n'+instructions+'\n\n## Current task\nIssue UUID: '+issue['id']+'\nIdentifier: '+issue['identifier']+'\n'+description+'\n\n## CLI commands\n`multica issue get '+issue['id']+' --output json`\n`multica issue comment add '+issue['id']+' --content <single delegation with both exact mention markdowns from roster> --output json`\n`multica squad activity '+issue['id']+' <action|failed|no_action> --reason <receipt-based reason> --output json`\n\nPerform exactly one delegation comment, inspect both immediate target receipts, record exactly one truthful evaluation, and end. Do not create child issues, change status or repost. A handled receipt means admission/coverage only; do not claim work completed. If any target is blocked/unconfirmed, use failed and identify both the handled subset and unresolved target in reason.\n'
(p/'leader-prompt.md').write_text(prompt);(p/'leader-prompt.md').chmod(0o600)
trial={'role':'leader','claim':claim,'env':env,'cwd':str(cwd),'prompt_file':str(p/'leader-prompt.md'),'cli':str(p/'bin/multica'),'cli_hash':hashlib.sha256((p/'bin/multica').read_bytes()).hexdigest(),'input_selection':{'agent_instructions_sha256':hashlib.sha256(instructions.encode()).hexdigest(),'prompt_sha256':hashlib.sha256(prompt.encode()).hexdigest(),'prompt_bytes':len(prompt.encode())},'expected':{'current_issue':issue['id'],'issue_identifier':issue['identifier'],'project':setup['project'],'allowed_target':allowed['id'],'blocked_target':blocked['id'],'squad_id':squad['id'],'comment_delta':1,'evaluation_delta':1,'evaluation_outcome':'failed','allowed_task_delta':1,'blocked_task_delta':0,'issue_mutations':0,'no_blind_retry':True}}
save('leader-trial.json',trial);print('Leader fixture ready; actual claim instruction bytes '+str(len(instructions.encode())))
