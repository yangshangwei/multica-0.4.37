from pathlib import Path
import hashlib, json, os, stat, subprocess, sys, datetime

root=Path('/Volumes/artisan/code/2026/multica-0.4.37')
integration=Path('/Volumes/artisan/code/2026/mc-merge-unmerged-worktrees-20261108')
private=Path('/Volumes/artisan/code/2026/mc-merge-takeover-archives-20261108/verification')
run=json.loads((private/sys.argv[1]/'state.json').read_text())
assert run.get('exit_code')==0, 'Complete quality gate has not passed'
head=run['head']
def git(*args,cwd=root):
    return subprocess.check_output(['git',*args],cwd=cwd)
assert git('rev-parse','HEAD',cwd=integration).decode().strip()==head
assert run['ending_head']==head
assert not git('status','--porcelain=v1',cwd=integration).strip(), 'Integration tree changed'
assert git('branch','--show-current').decode().strip()=='codex/cross-workspace-collaboration'
assert subprocess.run(['git','merge-base','--is-ancestor','HEAD',head],cwd=root).returncode==0, 'Root is not a fast-forward ancestor'
changed=set(git('diff','--name-only','-z','HEAD',head).decode().strip('\0').split('\0'))-{''}
dirty=set(git('diff','--name-only','-z','HEAD').decode().strip('\0').split('\0'))-{''}
untracked=set(git('ls-files','--others','--exclude-standard','-z').decode().strip('\0').split('\0'))-{''}
for path in changed:
    assert not any(path==old or path.startswith(old+'/') or old.startswith(path+'/') for old in dirty|untracked), 'Root has overlapping work: '+path
def fingerprint(path):
    file=root/path
    if not file.exists() and not file.is_symlink(): return {'kind':'absent'}
    mode=stat.S_IMODE(file.lstat().st_mode)
    if file.is_symlink(): return {'kind':'symlink','mode':mode,'target':os.readlink(file)}
    assert file.is_file(), 'Unexpected non-file dirty entry: '+path
    return {'kind':'file','mode':mode,'sha256':hashlib.sha256(file.read_bytes()).hexdigest(),'bytes':file.stat().st_size}
before={p:fingerprint(p) for p in sorted(dirty|untracked)}
index_before=git('diff','--cached','--binary','HEAD')
record={'timestamp':datetime.datetime.now().astimezone().isoformat(),'from':git('rev-parse','HEAD').decode().strip(),'to':head,'changed_paths':sorted(changed),'preserved_paths':before,'index_diff_sha256':hashlib.sha256(index_before).hexdigest(),'strategy':'ff-only, no autostash, disjoint paths'}
(private/'root-before-landing.json').write_text(json.dumps(record,ensure_ascii=False,indent=2)+'\n')
result=subprocess.run(['git','-c','merge.autoStash=false','merge','--ff-only','--no-autostash',head],cwd=root,text=True,capture_output=True)
(private/'root-landing.log').write_text(result.stdout+result.stderr)
assert result.returncode==0, 'Fast-forward refused; original work retained'
assert git('rev-parse','HEAD').decode().strip()==head
differences=[p for p,expected in before.items() if fingerprint(p)!=expected]
assert not differences, 'Existing work changed: '+repr(differences)
assert git('diff','--cached','--binary','HEAD')==index_before, 'Existing staged changes changed'
assert not git('diff','HEAD','--',*sorted(changed)).strip(), 'Landed product differs from verified candidate'
record.update(completed_at=datetime.datetime.now().astimezone().isoformat(),preservation_pass=True,preserved_file_count=len(before),landed_source_matches_verified=True)
(private/'root-after-landing.json').write_text(json.dumps(record,ensure_ascii=False,indent=2)+'\n')
print(json.dumps({'head':head,'preserved_paths':len(before),'changed_paths':len(changed),'strategy':record['strategy'],'pass':True}))
