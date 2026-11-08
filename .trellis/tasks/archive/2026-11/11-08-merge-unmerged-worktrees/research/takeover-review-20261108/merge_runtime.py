from pathlib import Path
import subprocess, os, tempfile

integration = Path('/Volumes/artisan/code/2026/mc-merge-unmerged-worktrees-20261108')
root = Path('/Volumes/artisan/code/2026/multica-0.4.37')
sources = {
    'handoff-launch': (Path('/Volumes/artisan/code/2026/mc-handoff-launch-20261031'), '4d0d1a76ddcbbf2ab985550d694a38263aaff25f'),
    'handoff-cursor': (Path('/Volumes/artisan/code/2026/mc-handoff-cursor-20261101'), 'e3620721efacb820b89709c8db9d326d9cf2b00b'),
    'recording-recovery': (Path('/Volumes/artisan/code/2026/mc-runtime-recording-recovery-20261101'), 'e3620721efacb820b89709c8db9d326d9cf2b00b'),
}

def run(args, cwd):
    return subprocess.run(args, cwd=cwd, stdout=subprocess.PIPE, stderr=subprocess.PIPE)

def status_paths(wt):
    out = run(['git','status','--porcelain=v1'], wt).stdout.decode().splitlines()
    ret = {}
    for line in out:
        if len(line) < 4:
            continue
        code = line[:2]
        p = line[3:]
        if ' -> ' in p:
            p = p.split(' -> ')[-1]
        ret[p] = code
    return ret

def is_allowed(p):
    if not (p.startswith('server/') or p.startswith('packages/') or p.startswith('e2e/') or p.startswith('scripts/')):
        return False
    if p.startswith('server/internal/service/builtin_skills/'):
        return False
    if p.endswith(('.md','.mdx','.json','.jsonl','.log','.png','.txt')):
        return False
    if p.startswith('.trellis/') or p.startswith('.omx/'):
        return False
    return True

def read_file(wt, p):
    fp = wt / p
    if fp.exists() and fp.is_file():
        return fp.read_bytes()
    return None

def git_blob(commit, p):
    r=run(['git','show',f'{commit}:{p}'], root)
    if r.returncode != 0:
        return None
    return r.stdout

def merge_bytes(ours, base, theirs, labels, binary=False):
    if ours == theirs:
        return ours, 0, 'identical'
    if ours == base:
        return theirs, 0, 'source-only'
    if theirs == base:
        return ours, 0, 'current-only'
    if binary or any(b'\x00' in (x or b'') for x in (ours,base,theirs)):
        return theirs, 1, 'binary/source-preferred'
    with tempfile.TemporaryDirectory(prefix='runtime-merge-') as td:
        d=Path(td)
        op=d/'ours'; bp=d/'base'; tp=d/'theirs'
        op.write_bytes(ours or b''); bp.write_bytes(base or b''); tp.write_bytes(theirs or b'')
        r=run(['git','merge-file','-p','--diff3',str(op),str(bp),str(tp)], integration)
        return r.stdout, (1 if r.returncode==1 else r.returncode), ('conflict' if r.returncode==1 else 'merged')

states={name:status_paths(wt) for name,(wt,_) in sources.items()}
all_paths=set().union(*(s.keys() for s in states.values()))
allowed=sorted(p for p in all_paths if is_allowed(p))
ledger=[]; conflicts=[]; changed=[]
for p in allowed:
    candidates=[]
    for name,(wt,base_commit) in sources.items():
        if p not in states[name]:
            continue
        code=states[name][p]
        candidates.append((name,base_commit,read_file(wt,p),code))
    # First reconcile source snapshots. Prefer the two common-base sources over stale launch when both exist.
    candidate=None; candidate_base=None; candidate_labels=[]
    # group cursor/recording
    cr=[x for x in candidates if x[0] in ('handoff-cursor','recording-recovery')]
    launch=[x for x in candidates if x[0]=='handoff-launch']
    if cr:
        if len(cr)==1:
            candidate, candidate_base = cr[0][2], cr[0][1]
            candidate_labels=[cr[0][0]]
            decision='single-common-base-source'
        else:
            a,b=cr[0],cr[1]
            base=git_blob(a[1],p)
            candidate, rc, decision=merge_bytes(a[2],base,b[2],(a[0],b[0]),binary=False)
            candidate_base=a[1]
            candidate_labels=[a[0],b[0]]
            if rc:
                conflicts.append((p,','.join(candidate_labels),decision))
        # If launch has a path absent from common sources, ignore; if it differs, record as stale overlap.
        if launch:
            launch_blob=launch[0][2]
            if candidate != launch_blob:
                ledger.append((p,'handoff-launch','stale-overlap-ignored','common-base cursor/recording snapshot selected'))
    elif launch:
        candidate, candidate_base = launch[0][2], launch[0][1]
        candidate_labels=['handoff-launch']
        decision='launch-only-source'
    else:
        continue

    current=read_file(integration,p)
    base=git_blob(candidate_base,p)
    result, rc, merge_decision=merge_bytes(current,base,candidate,( 'integration', '+'.join(candidate_labels) ), binary=False)
    if rc:
        conflicts.append((p,'+'.join(candidate_labels),merge_decision))
    # result empty means delete; preserve mode as regular file.
    fp=integration/p
    if result in (b'', None):
        # Only delete when source explicitly deleted/empty; do not remove a current file for an unresolved conflict.
        if rc==0 and candidate in (None,b''):
            if fp.exists(): fp.unlink()
    else:
        fp.parent.mkdir(parents=True,exist_ok=True)
        fp.write_bytes(result)
    if result != current:
        changed.append(p)
    ledger.append((p,'+'.join(candidate_labels),merge_decision, 'conflict' if rc else 'applied'))

ledger_path=root/'.trellis/tasks/11-08-merge-unmerged-worktrees/research/runtime-handoff-conflicts-20261108.md'
with ledger_path.open('w') as f:
    f.write('# Runtime handoff merge ledger — 2026-11-08\n\n')
    f.write('Base integration HEAD: `e541cfb43`. Source worktrees were read only.\n\n')
    f.write('## Decisions\n\n| Path | Source snapshot | Source decision | Result |\n| --- | --- | --- | --- |\n')
    for p,s,d,r in ledger:
        f.write(f'| `{p}` | {s} | {d} | {r} |\n')
    f.write('\n## Conflicts requiring review\n\n')
    if not conflicts:
        f.write('None.\n')
    else:
        for p,s,d in conflicts:
            f.write(f'- `{p}` from `{s}`: {d}\n')
    f.write(f'\nChanged paths: {len(changed)}\nConflicts: {len(conflicts)}\n')
print(f'allowed_paths={len(allowed)} changed_paths={len(changed)} conflicts={len(conflicts)} ledger={ledger_path}')
for x in conflicts[:100]: print('CONFLICT', *x, sep=' | ')
