from pathlib import Path
import datetime, json, os, shlex, subprocess, sys

worktree=Path('/Volumes/artisan/code/2026/mc-merge-unmerged-worktrees-20261108')
private=Path('/Volumes/artisan/code/2026/mc-merge-takeover-archives-20261108/verification')
run_id=sys.argv[1]
directory=private/run_id
directory.mkdir(exist_ok=False)
head=subprocess.check_output(['git','rev-parse','HEAD'],cwd=worktree,text=True).strip()
status=subprocess.check_output(['git','status','--porcelain=v1'],cwd=worktree,text=True).strip()
assert not status, 'Full verification requires a clean integration tree'
env={k:v for k,v in os.environ.items() if not k.startswith(('MULTICA_','E2E_','DATABASE_','POSTGRES_','REDIS_','RESEND_','SMTP_','RK1','PLAYWRIGHT_','COLLABORATION_','NEXT_PUBLIC_','PG'))}
env.update(PATH='/opt/homebrew/opt/libpq/bin:'+env['PATH'], ENV_FILE=str(private/'check-source.env'), MULTICA_DEV_HOME=str(private/'dev-registry'), MULTICA_RUN_REAL_AGENT_SMOKE='0')
state={'run_id':run_id,'head':head,'worktree':str(worktree),'command':['make','check'],'started_at':datetime.datetime.now().astimezone().isoformat(),'log':str(directory/'make-check.log'),'registry':env['MULTICA_DEV_HOME']}
(directory/'state.json').write_text(json.dumps(state,indent=2)+'\n')
with (directory/'make-check.log').open('w') as log:
    process=subprocess.Popen(['make','check'],cwd=worktree,env=env,stdout=log,stderr=subprocess.STDOUT,start_new_session=True)
    state['pid']=process.pid
    (directory/'state.json').write_text(json.dumps(state,indent=2)+'\n')
    print(json.dumps(state),flush=True)
    code=process.wait()
state.update(exit_code=code,completed_at=datetime.datetime.now().astimezone().isoformat(),ending_head=subprocess.check_output(['git','rev-parse','HEAD'],cwd=worktree,text=True).strip())
(directory/'state.json').write_text(json.dumps(state,indent=2)+'\n')
print(json.dumps(state),flush=True)
sys.exit(code)
