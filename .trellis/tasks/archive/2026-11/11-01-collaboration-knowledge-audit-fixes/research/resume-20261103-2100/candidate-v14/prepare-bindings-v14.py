# Execution coordinator: 01a1a186-9a5c-7b00-96f8-0430b47edb4c; template only, no execution authorization.
from pathlib import Path
import json,subprocess,hashlib,datetime,os
ROOT=Path('/Volumes/artisan/code/2026/mc-collaboration-closure-v12-20261103')
OUT=ROOT/'.omx/private/final-check-v14'
HERE=Path(__file__).resolve().parent
OLD=Path('/Volumes/artisan/code/2026/mc-collaboration-final-owned-20261103')
NODE='/Users/artisan/.nvm/versions/node/v22.23.2/bin/node'
STANDARD=HERE.parents[4]/'10-30-collaboration-context-handoff-runtime/research/real-model-rk1/source_manifest.py'
# Use an explicit repository-relative standard path; do not infer it from a candidate.
STANDARD=Path('/Volumes/artisan/code/2026/multica-0.4.37/.trellis/tasks/10-30-collaboration-context-handoff-runtime/research/real-model-rk1/source_manifest.py')
def digest(path):
 with Path(path).open('rb') as source:return hashlib.file_digest(source,'sha256').hexdigest()
def save(path,value):
 with path.open('x') as target:json.dump(value,target,indent=2);target.write('\n')
 path.chmod(0o600)
def replace(value):
 if isinstance(value,str):return value.replace(str(OLD),str(ROOT)).replace('v11','v14')
 if isinstance(value,list):return [replace(v) for v in value]
 if isinstance(value,dict):return {k:replace(v) for k,v in value.items()}
 return value
def git(*args):return subprocess.check_output(['git',*args],cwd=ROOT,text=True).strip()
assert not git('status','--porcelain'), 'Candidate must be restored/clean after directed replay before bindings'
old=json.loads((OLD/'.omx/private/final-check-v11/owned-bindings-v11-provenance.json').read_text())
head=git('rev-parse','HEAD')
snapshot=json.loads((HERE/'candidate-setup/source-before.json').read_text())
assert snapshot['candidate_head']==head
rows=snapshot['tracked_files'];fingerprint=hashlib.sha256(json.dumps(rows,sort_keys=True,separators=(',',':')).encode()).hexdigest();assert fingerprint==snapshot['tracked_source_fingerprint']
manifest_path=OUT/'canonical-product-source-manifest-v14-provenance.json'
subprocess.run(['/opt/homebrew/opt/python@3.14/bin/python3.14',str(STANDARD),str(ROOT),str(manifest_path)],check=True)
manifest=json.loads(manifest_path.read_text());assert not manifest['dirty_product_paths']
helper=OUT/'checkout-source-id-v14.cjs'
dev_id=subprocess.check_output([NODE,str(helper),str(ROOT)],text=True).strip()
resolver=OUT/'resolve-workspaces-v14.mjs'
graph=json.loads(subprocess.check_output([NODE,str(resolver),str(ROOT)],text=True));assert not graph['errors']
packages={}
for group in ['apps','packages']:
 for package in sorted((ROOT/group).iterdir()):
  f=package/'package.json'
  if f.is_file():packages[json.loads(f.read_text())['name']]=str(package.relative_to(ROOT))
edges=[]
for group in ['apps','packages']:
 for package in sorted((ROOT/group).iterdir()):
  f=package/'package.json'
  if not f.is_file():continue
  d=json.loads(f.read_text())
  for section in ['dependencies','devDependencies','peerDependencies','optionalDependencies']:
   for name,version in d.get(section,{}).items():
    if name in packages and str(version).startswith('workspace:'):
     edge={'consumer':str(package.relative_to(ROOT)),'name':name,'expected':packages[name]}
     if edge not in edges:edges.append(edge)
expectations=OUT/'workspace-expectations-v14.json';save(expectations,{'candidate_sha':head,'edges':edges})
tools=replace(old['tools'])
for tool in tools.values():tool['sha256']=digest(tool['path'])
assert json.loads((ROOT/'.omx/private/owned-workspace-links/pnpm-10.28.2/package.json').read_text())['version']=='10.28.2'
pnpm=tools['pnpm'];pnpm['bundle_sha256']=digest(pnpm['bundle_path'])
for row in pnpm['sdk']['files']:
 assert digest(Path(pnpm['sdk']['path'])/row['path'])==row['sha256'], 'Pinned SDK bytes must remain identical'
for name,value in tools['go']['members'].items():assert digest(Path(tools['go']['goroot'])/name)==value
artifacts=json.loads((HERE/'candidate-setup/artifact-bindings-v14.json').read_text());cli_record=HERE/'candidate-setup/cli-build-v14.json';cli_build=json.loads(cli_record.read_text());desktop_record=HERE/'candidate-setup/desktop-build-v14.json'
cli={**artifacts['cli'],'version':cli_build['version'],'version_observation':cli_build['version_probe_stdout'],'build_record_sha256':digest(cli_record)}
allocation=json.loads((OUT/'build-temp-allocation-v14.json').read_text())
binding={'schema_version':1,'candidate_root':str(ROOT),'candidate_sha':head,'prepared_at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'source':{'tracked_files':rows,'tracked_fingerprint':fingerprint,'checkout_source_id':dev_id,'checkout_source_id_helper':str(helper),'checkout_source_id_helper_sha256':digest(helper),'identity_purposes':old['source']['identity_purposes'],'product_source':{'purpose':'canonical product source identity','source_id':manifest['source_id'],'file_count':manifest['file_count'],'roots':manifest['roots'],'manifest_path':str(manifest_path),'manifest_sha256':digest(manifest_path),'standard_script_path':str(STANDARD),'standard_script_sha256':digest(STANDARD)}},'workspace':{'expectations_path':str(expectations),'expectations_sha256':digest(expectations),'edges':edges,'resolver_path':str(resolver),'resolver_sha256':digest(resolver),'graph':graph},'tools':tools,'cli':cli,'desktop':artifacts['desktop'],'build_records':{'cli_sha256':digest(cli_record),'desktop_sha256':digest(desktop_record)},'allocation':allocation,'cache_paths':{'GOCACHE':str(OUT/'go-build-cache'),'GOMODCACHE':'/Users/artisan/go/pkg/mod','GOPATH':str(OUT/'go-path')},'renderer_url':'http://127.0.0.1:55132/','api_pool':{'max':10,'min':1}}
save(OUT/'owned-bindings-v14-provenance.json',binding)
print(json.dumps({'candidate_sha':head,'canonical_product_source_id':manifest['source_id'],'product_files':manifest['file_count'],'tracked_files':len(rows),'workspace_edges':len(edges),'exports':len(graph['exports']),'binding_sha256':digest(OUT/'owned-bindings-v14-provenance.json')}))
