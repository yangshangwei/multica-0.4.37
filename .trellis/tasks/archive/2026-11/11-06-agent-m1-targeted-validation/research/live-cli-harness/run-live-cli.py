import json,pathlib,subprocess,time,hashlib,urllib.parse
p=pathlib.Path(__file__).parent
out=p/'cli-live-01';out.mkdir(mode=0o700)
url=(p/'rnd_m1_targeted_api01.env').read_text().strip().split('=',1)[1]
psql='/opt/homebrew/Cellar/libpq/18.6/bin/psql'
def sql(q):
 r=subprocess.run([psql,url,'-X','-A','-t','-v','ON_ERROR_STOP=1','-c',q],capture_output=True,text=True)
 if r.returncode:raise RuntimeError('SQL oracle unavailable')
 return json.loads(r.stdout.strip())
report=[]
def execute(role,name,args,want,verify=None):
 j=json.loads((p/(role+'-trial.json')).read_text());tracefile=p/'live-http-trace.jsonl';start=len(tracefile.read_text().splitlines());t=time.time()
 r=subprocess.run([j['cli'],*args],env=j['env'],cwd=j['cwd'],capture_output=True,text=True,timeout=25)
 (out/(name+'.stdout')).write_text(r.stdout);(out/(name+'.stderr')).write_text(r.stderr)
 trace=[json.loads(x) for x in tracefile.read_text().splitlines()[start:]]
 actual={'name':name,'role':role,'args':args,'exit':r.returncode,'elapsed_ms':round((time.time()-t)*1000),'requests':trace,'stdout_sha256':hashlib.sha256(r.stdout.encode()).hexdigest(),'stderr_sha256':hashlib.sha256(r.stderr.encode()).hexdigest()}
 try:
  assert (r.returncode==0)==want,(r.returncode,r.stderr)
  if want:
   parsed=json.loads(r.stdout)
   if verify:verify(parsed)
  else:assert any(x['status']==403 for x in trace),'denial did not reach actual API'
  actual['verdict']='pass'
 except Exception as e:actual['verdict']='fail';actual['assertion']=str(e)
 report.append(actual);(out/'summary.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n')
 if actual['verdict']=='fail':raise AssertionError(name+' failed; raw evidence retained')
 return parsed if want else None
for role in ['reporter','reviewer']:
 j=json.loads((p/(role+'-trial.json')).read_text());exp=j['expected'];ws=j['env']['MULTICA_WORKSPACE_ID'];issue=exp['current_issue']
 before=sql("SELECT json_build_object('issues',(SELECT json_agg(t ORDER BY id) FROM (SELECT id,title,status,description,project_id,assignee_type,assignee_id FROM issue WHERE workspace_id='"+ws+"') t),'tasks',(SELECT json_agg(t ORDER BY id) FROM (SELECT t.id,t.status FROM agent_task_queue t JOIN agent a ON a.id=t.agent_id WHERE a.workspace_id='"+ws+"') t))")
 (out/(role+'-before.json')).write_text(json.dumps(before,ensure_ascii=False,indent=2)+'\n')
 def correct_issue(x):assert x['id']==issue
 execute(role,role+'-uuid-get',['issue','get',issue,'--output','json'],True,correct_issue)
 execute(role,role+'-key-get',['issue','get',exp['issue_identifier'],'--output','json'],True,correct_issue)
 def comments_array(x):assert isinstance(x,list)
 execute(role,role+'-comments',['issue','comment','list',issue,'--output','json'],True,comments_array)
 execute(role,role+'-timeline',['issue','timeline',issue,'--output','json'],True,comments_array)
 if role=='reporter':
  expected_ids={x['id'] for x in sql("SELECT json_agg(t) FROM (SELECT id FROM issue WHERE workspace_id='"+ws+"' AND project_id='"+exp['project']+"') t")}
  def project_only(x):
   assert {i['id'] for i in x['issues']}==expected_ids
   assert x['total']==len(expected_ids) and x['has_more']==False
  execute(role,role+'-project-list',['issue','list','--project',exp['project'],'--output','json'],True,project_only)
  def page(x):assert len(x['issues'])==1 and x['issues'][0]['id'] in expected_ids and x['has_more']==True
  execute(role,role+'-project-page',['issue','list','--project',exp['project'],'--limit','1','--offset','1','--output','json'],True,page)
  execute(role,role+'-short-project-denied',['issue','list','--project',exp['project'][:8],'--output','json'],False)
 else:
  execute(role,role+'-project-list-denied',['issue','list','--project',exp['project'],'--output','json'],False)
 execute(role,role+'-outside-read-denied',['issue','get',exp['outside_issue'],'--output','json'],False)
 execute(role,role+'-write-denied',['issue','update',issue,'--title','FORBIDDEN CLI MUTATION','--output','json'],False)
 marker='Deterministic '+role+' CLI delivery '+str(time.time_ns())
 result=execute(role,role+'-comment-add',['issue','comment','add',issue,'--content',marker,'--output','json'],True)
 assert result['issue_id']==issue and result['author_id']==j['env']['MULTICA_AGENT_ID']
 persisted=sql("SELECT row_to_json(t) FROM (SELECT id,issue_id,content,author_id,author_type,source_task_id,dispatch_eligible FROM comment WHERE id='"+result['id']+"') t")
 assert persisted['content']==marker and persisted['author_type']=='agent' and persisted['author_id']==j['env']['MULTICA_AGENT_ID'] and persisted['source_task_id']==j['claim']['id'] and persisted['dispatch_eligible']==False
 def readback(x):assert len([c for c in x if c['id']==result['id'] and c['content']==marker])==1
 execute(role,role+'-comment-readback',['issue','comment','list',issue,'--output','json'],True,readback)
 after=sql("SELECT json_build_object('issues',(SELECT json_agg(t ORDER BY id) FROM (SELECT id,title,status,description,project_id,assignee_type,assignee_id FROM issue WHERE workspace_id='"+ws+"') t),'tasks',(SELECT json_agg(t ORDER BY id) FROM (SELECT t.id,t.status FROM agent_task_queue t JOIN agent a ON a.id=t.agent_id WHERE a.workspace_id='"+ws+"') t))")
 assert before==after,'CLI mutated issue or task state'
 (out/(role+'-after.json')).write_text(json.dumps({'state':after,'persisted_comment':persisted},ensure_ascii=False,indent=2)+'\n')
summary={'passed':sum(x['verdict']=='pass' for x in report),'failed':sum(x['verdict']=='fail'for x in report),'cli_sha256':hashlib.sha256((p/'bin/multica').read_bytes()).hexdigest(),'source_head':subprocess.check_output(['git','rev-parse','HEAD'],cwd='/Volumes/artisan/code/2026/mc-agent-rnd-m1-20261105',text=True).strip(),'cases':report}
(out/'final.json').write_text(json.dumps(summary,ensure_ascii=False,indent=2)+'\n');print(json.dumps({k:v for k,v in summary.items()if k!='cases'}))
