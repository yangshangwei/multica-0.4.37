from pathlib import Path
import json,re,csv,hashlib,collections
root=Path.cwd(); task=root/'.trellis/tasks/11-04-core-business-e2e-acceptance'; ev=task/'evidence/inventory'
data=json.loads((ev/'playwright-list.json').read_text()); candidate=json.loads((task/'evidence/candidate.json').read_text())['sha']
files={p.name:p for p in (root/'e2e').glob('*.spec.ts')}
def dependencies(p,seen=None):
 seen=set() if seen is None else seen
 if p in seen:return seen
 seen.add(p)
 for m in re.finditer(r'(?:from\s+|import\s*)[\"\'](\.[^\"\']+)[\"\']',p.read_text()):
  q=p.parent/m[1]
  for z in [q,q.with_suffix('.ts'),Path(str(q)+'.ts'),Path(str(q)+'.mjs'),Path(str(q)+'.cjs'),q/'index.ts']:
   if z.is_file() and z.suffix in ('.ts','.mjs','.cjs','.js'):
    dependencies(z.resolve(),seen);break
 return seen

def journeys(file,title):
 s=(file+' '+title).lower(); out=[]
 rules=[('E01',r'auth|password|onboard|missing-user|login|session'),('E02',r'workspace|member|invit|role change|remov'),('E03',r'issue|quick|bulk|board|task|property|search|navigation'),('E04',r'triage|duplicate-picker'),('E05',r'agent|runtime|execution|progress-report|lifecycle'),('E06',r'stop|resume|retry|fail|recovery|cancel|lifecycle|terminal'),('E07',r'comment|inbox|notif|mention|unread|realtime'),('E08',r'collaboration'),('E09',r'collaboration.*(control|runtime|host|squad|switch|execution|command|resource|preview|lane|lifecycle|leave|revoke|terminal|installed)'),('E10',r'account|navigation|switch|boundar|selection|security|session|retained-language'),('E11',r'project|goal|risk|health'),('E12',r'iteration'),('E13',r'chat|conversation'),('E14',r'autopilot|automation'),('E15',r'skill|mcp|market|catalog|resource-publish|plugin|template'),('E16',r'knowledge|memory|handoff|preparation-record'),('E17',r'platform-admin|admin-collaboration|admin-resource|collaboration-administration')]
 for code,pat in rules:
  if re.search(pat,s):out.append(code)
 return out or ['E10']

def classify(file,title,src,deps):
 s=(file+' '+title).lower(); text='\n'.join(p.read_text() for p in deps)
 native=bool(re.search(r'desktop|electron|native',s)) or '_electron' in src or 'project-p1-desktop' in src
 # Guard predicates and lifecycle prerequisites, not inferred observed results.
 if file=='collaboration-detail-journey.spec.ts':lane='legacy-manual-detail-permit'
 elif file=='platform-admin-initialization.spec.ts':lane='password-admin-initialization'
 elif file=='auth-device.spec.ts':lane='legacy-device-auth'
 elif file=='collaboration-knowledge-deployed.spec.ts':lane='password-host-knowledge-on-off'
 elif file=='collaboration-memory-runtime.spec.ts':lane='password-host-memory-provider'
 elif file=='collaboration-host-installed.spec.ts':lane='password-packaged-host-main'
 elif file=='collaboration-installed-main.spec.ts':lane='password-packaged-linux-main'
 elif file=='collaboration-installed-clients.spec.ts':lane='password-installed-legacy-cli'
 elif 'crossClientEnvironmentReady' in src:lane='password-linux-namespace-native'
 elif 'hostRuntimeConfigured' in src:lane='password-native-host'
 elif 'controlEnvironmentReady' in src:lane='password-managed-control'
 elif file=='admin-resource-publishing.spec.ts':lane='password-resource-publishing'
 elif 'platform-admin' in file or file in ['admin-collaboration-switches.spec.ts','collaboration-administration-web.spec.ts','collaboration-administration-desktop.spec.ts']:lane='password-admin'
 elif file=='password-session-persistence.spec.ts':lane='native-synthetic-password-api'
 elif file.startswith('password-') or file=='mcp-desktop.spec.ts':lane='password-native' if native else 'password'
 elif 'process.env.E2E_PASSWORD_AUTH !== "1"' in src:lane='password-collaboration-native' if native else 'password-collaboration'
 elif file=='issue-assist-flow.spec.ts':lane='legacy-assist-provider'
 elif file=='quick-create-actor-picker-phase2.spec.ts':lane='legacy-recommendation-provider'
 elif file in ['upstream-backports-cli.spec.ts','cli-forbidden-reason.spec.ts']:lane='legacy-current-cli'
 elif file=='changelog.spec.ts' or file=='changelog-desktop.spec.ts':lane='legacy-changelog-feed'
 elif file=='mcp-market.spec.ts':lane='legacy-managed-mcp-catalog'
 elif file.startswith('collaboration-') or file=='global-scope-navigation.spec.ts':lane='legacy-collaboration-native' if native else 'legacy-collaboration'
 elif native:lane='legacy-native-renderer'
 else:lane='legacy-business'
 kinds=['browser-real-api']
 if native:kinds.append('native-renderer-harness')
 if 'SimulatedDaemon' in text:kinds.append('daemon-http-protocol-simulation')
 if 'hostRuntimeConfigured' in src or 'seedHostRuntime' in src:kinds.append('real-daemon-host-controlled-provider')
 if 'crossClientEnvironmentReady' in src:kinds.append('real-linux-namespace-controlled-provider')
 if 'packaged' in lane:kinds.append('packaged-main')
 if file=='password-session-persistence.spec.ts':kinds=['native-renderer-synthetic-api']
 if 'page.route(' in text or 'route.fulfill(' in text:kinds.append('contains-http-fault-or-route-fixtures')
 return lane,kinds,native

filemeta={}
for name,p in files.items():
 src=p.read_text(); deps=dependencies(p.resolve()); text='\n'.join(q.read_text() for q in deps)
 skips=[]
 for m in re.finditer(r'test\.skip\(',src):
  s=src[m.start():]; end=s.find(';');skips.append({'line':src.count('\n',0,m.start())+1,'expression':s[:end+1]})
 filemeta[name]={'file':'e2e/'+name,'sha256':hashlib.sha256(p.read_bytes()).hexdigest(),'local_dependencies':sorted(str(q.relative_to(root)) for q in deps),'direct_skip_guards':skips,'env_keys_in_local_dependency_graph':sorted(set(re.findall(r'process\.env\.([A-Z][A-Z0-9_]*)',text))),'deps':deps,'source':src}
rows=[]
def visit(suite,parents=[]):
 trail=parents+[suite['title']]
 for spec in suite.get('specs',[]):
  name=Path(spec['file']).name;m=filemeta[name]
  for test in spec['tests']:
   lane,kinds,native=classify(name,spec['title'],m['source'],m['deps'])
   rows.append({'id':spec['id']+':'+test['projectName'],'project':test['projectName'],'file':'e2e/'+name,'line':spec['line'],'title':' > '.join(trail[1:]+[spec['title']]),'journeys':journeys(name,spec['title']),'lane':lane,'evidence_scope':kinds,'native_required':native,'status':'BLOCKED' if lane=='legacy-manual-detail-permit' else 'NOT_RUN','result_evidence':'research/inventory-execution-lanes.md#new-fixture-blocker-to-resolve-before-final-acceptance' if lane=='legacy-manual-detail-permit' else '','candidate':candidate})
 for child in suite.get('suites',[]):visit(child,trail)
for suite in data['suites']:visit(suite)
assert len(rows)==441,len(rows)
assert len({r['id'] for r in rows})==441
assert len({r['file'] for r in rows})==149
for m in filemeta.values():del m['deps'];del m['source']
(ev/'cases.json').write_text(json.dumps({'candidate':candidate,'mode':'inventory-only','case_count':len(rows),'file_count':len(files),'mapping_method':'Static file/title/readiness-predicate mapping; overlapping journey labels identify related evidence, not completeness of every behavior. Playwright list-only status fields are NOT execution results.','cases':rows},indent=2)+'\n')
(ev/'spec-prerequisites.json').write_text(json.dumps(filemeta,indent=2)+'\n')
with (task/'coverage.csv').open('w',newline='') as f:
 fields=['id','project','file','line','title','journeys','lane','evidence_scope','native_required','status','result_evidence','candidate'];w=csv.DictWriter(f,fieldnames=fields);w.writeheader()
 for row in rows:w.writerow({k:'|'.join(v) if isinstance(v,list) else v for k,v in row.items()})
lanes={}
for row in rows:
 lane=lanes.setdefault(row['lane'],{'case_ids':[],'spec_files':[],'native_required':False})
 lane['case_ids'].append(row['id']);lane['spec_files'].append(row['file']);lane['native_required']|=row['native_required']
for lane in lanes.values():lane['spec_files']=sorted(set(lane['spec_files']));lane['case_count']=len(lane['case_ids'])
(ev/'lanes.json').write_text(json.dumps(lanes,indent=2)+'\n')
summary={'candidate':candidate,'cases':441,'files':149,'files_with_direct_skip':sum(bool(m['direct_skip_guards']) for m in filemeta.values()),'journey_case_counts':dict(sorted(collections.Counter(j for r in rows for j in r['journeys']).items())),'lane_case_counts':{k:v['case_count'] for k,v in lanes.items()},'no_tests_executed':True}
(ev/'summary.json').write_text(json.dumps(summary,indent=2)+'\n')
print(json.dumps(summary,indent=2))
