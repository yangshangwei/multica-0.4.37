"""Read-only materialized V16 admission verification; never runs launcher.main."""
from pathlib import Path
from datetime import datetime, timezone
import ast, hashlib, json, os, re, stat, subprocess, types
ROOT=Path('/Volumes/artisan/code/2026/mc-collaboration-closure-v12-20261103')
PROJECT=Path('/Volumes/artisan/code/2026/multica-0.4.37')
HERE=Path(__file__).resolve().parent
PREP=HERE.parent
OLD=PREP
OUT=ROOT/'.omx/private/final-check-v16'
HEAD='eda0d68fb07852b714f3092aa02ca0523d3ad312'
OWNER='01a1a3c9-2459-7de1-8244-55640c5f6143'
checks=[]
def check(name,value):
    if not value:raise RuntimeError(name)
    checks.append(name)
def digest(p):
    with Path(p).open('rb') as f:return hashlib.file_digest(f,'sha256').hexdigest()
def read(p):return json.loads(Path(p).read_text())
def git(*args):return subprocess.check_output(['git',*args],cwd=ROOT,text=True).strip()
check('clean exact frozen candidate',git('rev-parse','HEAD')==HEAD and not git('status','--porcelain=v1'))
check('no complete execution record/clearance published yet',all(not (OUT/n).exists() for n in ['make-check-v16.json','make-check-v16.log','check-source-v16.safe.env','independent-review-clearance-v16.json']))
launcher=OUT/'run-make-check-v16-provenance.py'
module=types.ModuleType('read_only_materialized_launcher')
module.__file__=str(launcher)
exec(compile(launcher.read_bytes(),str(launcher),'exec'),module.__dict__)
check('launcher identifies reviewed frozen SHA/root',module.EXPECTED_CANDIDATE_SHA==HEAD and module.EXPECTED_ROOT==ROOT)
values=module.parse_generated_environment((OUT/'check-source-v16.env').read_text())
check('canonical nonzero UUID is parsed as data',module.deployment_uuid(values)=='8c8cbecd-3098-4e1b-a5a5-ecf9e6cdd0db')
mat=read(PREP/'materialization.json');gold=read(OLD/'prepared-template-manifest.json');allocation=read(OUT/'build-temp-allocation-v16.json')
check('materialization SHA/root and golden manifest hash',mat['candidate_sha']==HEAD and mat['candidate_root']==str(ROOT) and mat['manifest_sha256']==digest(OLD/'prepared-template-manifest.json'))
check('24 operative rows, 26 static templates and 46 protected inputs',len(mat['rows'])==24 and len(gold['files'])==26 and len(mat['protected_inputs'])==46)
for name,expected in mat['protected_inputs'].items():check('protected donor '+name,digest(name)==expected)
replacements={'PENDING_ROOT_FINAL_SHA':HEAD,'PENDING_ROOT_RENDERER_PORT':str(mat['renderer_port']),'PENDING_ROOT_REDIS_PORT':str(mat['redis_port']),'PENDING_ROOT_PRIVATE_JWT_SECRET':values['JWT_SECRET'],'PENDING_ROOT_DATABASE_URL':values['DATABASE_URL'],'PENDING_ROOT_BUILD_TEMP_DIRECTORY':allocation['directory'],'PENDING_ROOT_POSTGRES_PASSWORD':values['POSTGRES_PASSWORD'],'PENDING_ROOT_DEPLOYMENT_UUID':mat['deployment_uuid']}
rows=[]
for row in mat['rows']:
    original=next(v for v in gold['files'] if v['prepared_template']==row['template'])
    template=Path(row['template']);final=Path(row['path'])
    check('golden template '+str(template),digest(template)==row['template_sha256']==original['prepared_sha256'])
    expected=template.read_text();occurrences={}
    for key,value in replacements.items():
        if key in expected:occurrences[key]=expected.count(key);expected=expected.replace(key,value)
    check('deterministic materialization '+str(final),final.read_text()==expected and digest(final)==row['sha256'] and row['replacement_counts']==occurrences)
    check('materialized mode/type '+str(final),not final.is_symlink() and stat.S_IMODE(final.stat().st_mode)==int(row['mode'],8))
    check('no pending executable fields '+str(final),not re.search(r'PENDING_ROOT_[A-Z_]+',expected))
    if final.suffix=='.py' or final.name in ['go','pnpm']:ast.parse(expected,filename=str(final))
    rows.append({'path':str(final),'sha256':digest(final),'mode':row['mode'],'golden_template_sha256':digest(template),'deterministic_match':True})
check('unchanged Go admission adapter',digest(OUT/'check-bin/go')=='70ca24bb88b2a8df93ae4694e9fd9739c523e8f35755a10bb799b7561b934fce')
flag_expected={'MULTICA_AUTH_MODE':'legacy','MULTICA_DEVICE_AUTH_ENABLED':'false','MULTICA_KNOWLEDGE_CONTEXT_ENABLED':'false','MULTICA_MANAGED_INSTALLATIONS_ENABLED':'false','MULTICA_PLATFORM_ADMIN_ENABLED':'false','MULTICA_LLM_API_KEY':'','MULTICA_LLM_BASE_URL':''}
check('legacy/default closed/no external model config',all(values.get(k)==v for k,v in flag_expected.items()))
check('private source config mode0600',stat.S_IMODE((OUT/'check-source-v16.env').stat().st_mode)==0o600)
check('new seed selected consistently',values['POSTGRES_DB']==mat['database']=='multica_closure_v16_seed_01a1a3c9' and module.urlsplit(values['DATABASE_URL']).path=='/'+mat['database'])
check('matching loopback renderer URLs',module.renderer_url(values)==f'http://127.0.0.1:{mat["renderer_port"]}/' and mat['renderer_port']==58268)
check('matching dedicated Redis URL',module.urlsplit(values['REDIS_TEST_URL']).port==mat['redis_port']==58269)
for n in ['catalog','catalog/mcp','catalog/skills','catalog/published','go-build-cache','go-path','node-cache']:
    p=OUT/n;check('private directory '+n,p.is_dir() and not p.is_symlink() and p.stat().st_uid==os.geteuid() and stat.S_IMODE(p.stat().st_mode)==0o700)
for n in ['full-check-home-v16','go-packages-v16','dev-registry-v16','profiles-v16','workspaces-v16','metadata-home-not-created','missing-test-owned-dsh']:
    p=OUT/n;check('fresh execution or absent DSH '+n,not p.exists() and not p.is_symlink())
temporary=Path(allocation['directory'])
check('task-owned temporary directory is real mode0700',temporary.is_dir() and not temporary.is_symlink() and temporary.stat().st_uid==os.geteuid() and stat.S_IMODE(temporary.stat().st_mode)==0o700)
check('temporary path is outside Git',subprocess.run(['git','-C',str(temporary),'rev-parse','--show-toplevel'],capture_output=True).returncode!=0)
check('task-only dependency link',(OUT/'node_modules').is_symlink() and (OUT/'node_modules').resolve()==(ROOT/'node_modules').resolve())
clearance={'bindings_sha256':digest(OUT/'owned-bindings-v16-provenance.json'),'resource_ownership_sha256':digest(OUT/'resource-ownership-v16.json')}
binding=module.verify_owned_bindings(ROOT,OUT,clearance,values)
check('read-only actual verify_owned_bindings succeeded',binding['candidate_sha']==HEAD)
check('source counts and workspace counts',len(binding['source']['tracked_files'])==17536 and binding['source']['product_source']['file_count']==7984 and len(binding['workspace']['edges'])==22 and len(binding['workspace']['graph']['exports'])==66)
helper=Path(binding['source']['checkout_source_id_helper'])
check('development identity helper hash',digest(helper)==binding['source']['checkout_source_id_helper_sha256'])
check('actual development HEAD/overlay identity',subprocess.check_output([binding['tools']['node']['path'],str(helper),str(ROOT)],text=True).strip()==binding['source']['checkout_source_id'])
check('source snapshots before/after unchanged',read(PREP/'candidate-setup/source-before.json')['tracked_files']==read(PREP/'candidate-setup/source-after-preparation.json')['tracked_files']==binding['source']['tracked_files'])
for name in ['cli','desktop']:
    record=read(PREP/f'candidate-setup/{name}-build-v16.json')
    check('same-source successful '+name+' build',record['candidate_sha']==record['source_head_after']==HEAD and record['source_status_after']=='' and record['exit_code']==0 and record['real_agent_model_environment_forwarded'] is False)
    check(name+' builder and log/profile hashes',digest(record['launcher_path'])==record['launcher_sha256'] and digest(record['log_path'])==record['log_sha256'] and digest(record['profile_path'])==record['profile_sha256'])
    check(name+' build record binding',digest(PREP/f'candidate-setup/{name}-build-v16.json')==binding['build_records'][name+'_sha256'])
cli=read(PREP/'candidate-setup/cli-build-v16.json')
check('CLI actual SHA/version/commit',digest(binding['cli']['path'])==cli['binary_sha256']==binding['cli']['sha256'] and cli['commit']==HEAD[:9] and HEAD[:9] in cli['version_probe_stdout'])
desktop_counts={}
for domain,artifact in binding['desktop'].items():
    check(domain+' staged/installed byte/mode metadata equal',artifact['installed']['files']==artifact['staged']['files'])
    for label in ['installed','staged']:
        directory=Path(artifact[label]['path']);rows_artifact=artifact[label]['files']
        for row in rows_artifact:
            file=directory/row['path'];check(label+' artifact mode/bytes '+domain+'/'+row['path'],stat.S_IMODE(file.stat().st_mode)==row['mode'] and file.stat().st_size==row['bytes'] and not file.is_symlink())
        desktop_counts[domain+'_'+label]=len(rows_artifact)
check('495 current installed/staged artifacts',sum(desktop_counts[k+'_installed'] for k in binding['desktop'])==495)
old_artifacts=read(PREP/'desktop-install-before.json');retained=Path(old_artifacts['retention_destination']);old_rows=old_artifacts['retired_installed']['files']
check('495 original artifact files retained',len(old_rows)==495 and sorted(str(p.relative_to(retained)) for p in retained.rglob('*') if p.is_file())==[r['path'] for r in old_rows])
for row in old_rows:
    file=retained/row['path'];check('retained original artifact '+row['path'],digest(file)==row['sha256'] and file.stat().st_size==row['bytes'] and stat.S_IMODE(file.stat().st_mode)==row['mode'] and not file.is_symlink())
inv=read(PREP/'private-control-inventory.json');counts={};names=[]
for suite in inv['suites']:
    record=read(suite['report_path']);log=Path(record['log_path']);raw=log.read_text()
    check('private control actual report/log/source '+suite['suite'],digest(suite['report_path'])==suite['report_sha256'] and digest(log)==record['log_sha256']==suite['raw_log_sha256'] and record['candidate_sha']==HEAD and record['exit_code']==0 and record['network']=='denied')
    check('actual private test source hash '+suite['suite'],digest(record['command'][-1])==record['source_sha256'])
    terminals=re.findall(r'^(test_\S+) \(([^)]+)\) \.\.\. ok$',raw,re.M)
    reconstructed=[identity+'.'+name for name,identity in terminals]
    check('actual complete private named inventory '+suite['suite'],len(terminals)==suite['count'] and reconstructed==[t['name'] for t in suite['tests']] and all(t['status']=='pass' for t in suite['tests']) and f'Ran {suite["count"]} tests' in raw and re.search(r'^OK$',raw,re.M) is not None)
    counts[suite['suite']]=suite['count'];names.extend(reconstructed)
check('69 real private-control named passes',counts=={'launcher':46,'adapter-env':7,'final-audit':16} and len(names)==len(set(names))==69 and inv['total']==inv['pass']==69 and inv['fail']==inv['skip']==0)
root_confirmation=PREP.parent/'root-live-ownership-before-full-v16.json';live=read(root_confirmation);proof=read(OUT/'resource-ownership-v16.json')
check('root actual live-resource confirmation hash',digest(root_confirmation)=='2fb9b5b656032e3000991cfb144050eefcf19e84f57f7999f61794c9fd3b5b6e')
check('root live proof binds actual proof/config/source',live['candidate_sha']==HEAD and live['source_env_sha256']==digest(OUT/'check-source-v16.env') and live['resource_proof_sha256']==digest(OUT/'resource-ownership-v16.json'))
check('root renderer identity consistent',live['renderer']['ownership_verified'] and live['renderer']['pid']==proof['renderer']['pid']==5896 and live['renderer']['url']==binding['renderer_url'] and live['renderer']['index_sha256']==digest(ROOT/'apps/desktop/out/renderer/index.html'))
check('root Redis identity consistent',live['redis']['ownership_verified'] and live['redis']['PONG'] is True and live['redis']['port']==58269 and live['redis']['container_id']==proof['redis']['container_id'] and live['redis']['labels']['multica.candidate']==HEAD and live['redis']['labels']['multica.task.owner']==OWNER)
check('root isolated PG latest764/752ledger',live['postgres']['database']==mat['database'] and live['postgres']['latest']=='764_collaboration_ai_policy' and live['postgres']['ledger']==752 and int(live['postgres']['connections'])<int(live['postgres']['max_connections']))
check('root knowledge gate retained',live['knowledge']=={'allocated':0,'consumed':0,'enabled':False,'real_allowed':False})
audit=PREP/'final-state-audit-v16/final_state_audit.py';audit_module=types.ModuleType('read_only_audit_defaults');audit_module.__file__=str(audit);exec(compile(audit.read_bytes(),str(audit),'exec'),audit_module.__dict__)
check('final audit default review path is current owned directory',audit_module.REVIEW==HERE/'review.json' and audit_module.ROOT==ROOT)
check('clean candidate after read-only checks',git('rev-parse','HEAD')==HEAD and not git('status','--porcelain=v1') and not (OUT/'metadata-home-not-created').exists())
summary={'verified_at':datetime.now(timezone.utc).isoformat(),'probe_script_sha256':digest(__file__),'candidate_sha':HEAD,'candidate_root':str(ROOT),'checks_passed':len(checks),'checks':checks,'deterministic_materialization':rows,'actual_binding_verification':'pass; verify_owned_bindings called with in-memory hashes only; no real clearance file or launch','source_identities':{'canonical_product':{'source_id':binding['source']['product_source']['source_id'],'file_count':7984,'purpose':'Tracked production roots only; no E2E/task docs'},'development_HEAD_overlay':{'source_id':binding['source']['checkout_source_id'],'purpose':'Actual HEAD plus normalized developer overlay identity for runtime'},'all_tracked_snapshot':{'sha256':binding['source']['tracked_fingerprint'],'file_count':17536,'purpose':'Every tracked content/mode, including tests/docs'}},'desktop_counts':desktop_counts,'original_desktop_files_preserved':495,'private_control_counts':counts,'private_control_names':names,'root_resource_confirmation':{'path':str(root_confirmation),'sha256':digest(root_confirmation),'actual_root_live_recheck':True,'reviewer_live_operations_repeated':False},'launcher_sha256':digest(launcher),'adapter_sha256':{n:digest(OUT/'check-bin'/n)for n in ['go','pnpm']},'source_env_sha256':digest(OUT/'check-source-v16.env'),'bindings_sha256':digest(OUT/'owned-bindings-v16-provenance.json'),'resource_ownership_sha256':digest(OUT/'resource-ownership-v16.json'),'final_auditor_sha256':digest(audit),'full_check_or_final_audit_executed':False,'knowledge_budget_allocated':0,'knowledge_calls':0}
with (HERE/'verification.json').open('x') as f:json.dump(summary,f,indent=2);f.write('\n')
print(json.dumps({k:summary[k]for k in ['candidate_sha','checks_passed','actual_binding_verification','desktop_counts','private_control_counts','full_check_or_final_audit_executed']}))
