from pathlib import Path
from datetime import datetime, timezone
import hashlib, json, types
HERE=Path(__file__).resolve().parent
PREP=HERE.parent
def sha(path):
 with Path(path).open('rb') as source:return hashlib.file_digest(source,'sha256').hexdigest()
def read(path):return json.loads(Path(path).read_text())
before=read(PREP/'cache-seeding-before.json')
after=read(PREP/'cache-seeding-result.json')
source=before['source'];destination=after['destination']
assert after['candidate_sha']=='eda0d68fb07852b714f3092aa02ca0523d3ad312'
assert after['state']=='complete' and after['successful_COW_clones']==10290
assert after['normal_copy_fallbacks']==0 and after['old_cache_written_or_redirected'] is False
assert source['entries']==after['source_after']['entries']==destination['entries']
assert source['file_identities']==after['source_after']['file_identities']
assert source['fingerprint']==after['source_before_fingerprint']==after['source_after']['fingerprint']==destination['fingerprint']
assert len(source['file_identities'])==len(destination['file_identities'])==10290
assert all(source['file_identities'][p]['inode']!=destination['file_identities'][p]['inode'] for p in source['file_identities'])
helper=PREP/'seed-warm-go-cache.py';module=types.ModuleType('read_only_cache_inventory');module.__file__=str(helper)
exec(compile(helper.read_bytes(),str(helper),'exec'),module.__dict__)
current_source=module.inventory(Path(source['path']))
current_destination=module.inventory(Path(destination['path']))
assert current_source['entries']==source['entries'] and current_source['file_identities']==source['file_identities']
assert current_source['device']==source['device'] and current_source['inode']==source['inode']
assert all(current_source['file_identities'][p]['inode']!=current_destination['file_identities'][p]['inode'] for p in source['file_identities'])
expected={r['path']:r for r in destination['entries']};actual={r['path']:r for r in current_destination['entries']}
missing=[p for p in expected if p not in actual]
assert not missing
changed=[p for p in expected if expected[p]!=actual[p]]
added=[p for p in actual if p not in expected]
record={'verified_at':datetime.now(timezone.utc).isoformat(),'probe_sha256':sha(__file__),'cache_helper_sha256':sha(helper),'source_before_path':str(PREP/'cache-seeding-before.json'),'source_before_sha256':sha(PREP/'cache-seeding-before.json'),'clone_result_path':str(PREP/'cache-seeding-result.json'),'clone_result_sha256':sha(PREP/'cache-seeding-result.json'),'clones_verified':10290,'regular_source_bytes_rehashed':sum(r.get('bytes',0) for r in source['entries']),'recorded_same_source_before_after_destination':True,'actual_old_source_content_mode_inode_mtime_unchanged':True,'all_initial_and_current_source_destination_regular_inodes_distinct':True,'source_fingerprint':source['fingerprint'],'destination_at_clone_fingerprint':destination['fingerprint'],'destination_current_after_build_fingerprint':current_destination['fingerprint'],'post_seed_task_build_cache_changed_members':changed,'post_seed_task_build_cache_added_members':added,'post_seed_task_build_cache_missing_members':missing,'current_destination_is_expected_writable_task_cache':True,'reseed_or_clone_invocations':0,'source_or_destination_writes_by_reviewer':0,'knowledge_budget_allocated':0,'knowledge_calls':0,'note':'Initial clone proof is byte/mode identical; subsequent current destination may contain task-owned Go build updates. Original source is rehashed and unchanged; no back links or lost cloned members.'}
with (HERE/'cache-verification.json').open('x') as sink:json.dump(record,sink,indent=2);sink.write('\n')
print(json.dumps({'clones':10290,'source_bytes':record['regular_source_bytes_rehashed'],'source_unchanged':True,'current_destination_changed_members':len(changed),'current_destination_added_members':len(added),'cache_mutations_or_clones_by_reviewer':0}))
