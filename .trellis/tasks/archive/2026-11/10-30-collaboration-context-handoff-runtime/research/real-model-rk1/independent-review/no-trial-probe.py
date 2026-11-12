import json, tempfile, runpy, os, sys, hashlib
from pathlib import Path
from unittest.mock import patch
launcher=Path(__file__).resolve().parent.parent/'run_rk1.py'
with tempfile.TemporaryDirectory(prefix='rk1-independent-no-trial-') as temp:
 root=Path(temp);here=root/'inputs';here.mkdir();private=root/'private';private.mkdir();results=here/'results';results.mkdir()
 provider=root/'fake-provider';provider.write_text('never executed')
 (here/'runtime-addendum.json').write_text(json.dumps({'runtime_scenarios':[{'id':'RK1-E0a','trials':1,'launches':1}], 'order':['RK1-E0a'],'runtime_call_cap':25}))
 sha=lambda p:hashlib.sha256(Path(p).read_bytes()).hexdigest()
 reg={'runnable':True,'source_binding':{'source_id':'test-source'},'framework_sha256':'test-framework','launcher_sha256':sha(launcher),'provider_executable':str(provider),'provider_executable_sha256':sha(provider),'operator_configuration':{},'requested_model':'test-model'}
 (here/'framework-registration.json').write_text(json.dumps(reg))
 module=runpy.run_path(str(launcher));g=module['main'].__globals__;g.update(HERE=str(here),REPO=str(root),SERVER=str(root),RESULTS=str(results),PRIVATE=str(private),inputs_dirty=lambda:[],source_binding=lambda:{'dirty_product_paths':[],'source_id':'test-source','head':'test-head'},framework_sha256=lambda:'test-framework',operator_facts=lambda:{})
 with patch.dict(os.environ,{'DATABASE_URL':'postgres://fake@127.0.0.1/multica_rk1_test'}),patch.object(sys,'argv',[str(launcher),'RK1-E0a','1']),patch('subprocess.call',return_value=0):
  try:module['main']()
  except SystemExit as ended:code=ended.code
 invocation=json.loads(next(private.glob('*/invocation-*.json')).read_text())
 print(json.dumps({'probe':'executor exits0 without running a trial (e.g. skipped TestMain/test)','launcher_exit_code':code,'public_records':len(list(results.glob('*.json'))),'invocation':invocation},indent=2))
 assert code==0 and not list(results.glob('*.json'))
