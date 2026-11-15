from pathlib import Path
import json,os,subprocess,sys,datetime
root=Path('/Volumes/artisan/code/2026/mc-merge-unmerged-worktrees-20261108')
private=Path('/Volumes/artisan/code/2026/mc-merge-takeover-archives-20261108/verification')
directory=private/sys.argv[1];directory.mkdir(exist_ok=False)
command=["bash","-c",r'''set -euo pipefail
source scripts/check.sh
source_env="$ENV_FILE"
load_env_file "$source_env"
prepare_check_environment "$source_env"
trap check_cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
info "Focused verification environment: $NAME; provenance: $STATE_DIR"
mkdir -p "$DEV_TMPDIR"
export TMPDIR="$DEV_TMPDIR" TMP="$DEV_TMPDIR" TEMP="$DEV_TMPDIR"
ensure_database
migrate_database
CHECK_STARTED_API=true
start_api
CHECK_STARTED_RENDERER=true
start_check_desktop_renderer
CHECK_STARTED_WEB=true
start_web
print_status_json > "$STATE_DIR/verification.running.json"
pnpm exec playwright test --workers=1 --retries=0 "$@"
CHECK_COMPLETE=true
''',"focused-e2e",*sys.argv[2:]]
env={k:v for k,v in os.environ.items() if not k.startswith(('MULTICA_','E2E_','DATABASE_','POSTGRES_','REDIS_','RESEND_','SMTP_','RK1','PLAYWRIGHT_','COLLABORATION_','NEXT_PUBLIC_','PG'))}
env.update(PATH='/opt/homebrew/opt/libpq/bin:'+env['PATH'],ENV_FILE=str(private/'check-source.env'),MULTICA_DEV_HOME=str(private/'dev-registry'),MULTICA_RUN_REAL_AGENT_SMOKE='0')
state={'started_at':datetime.datetime.now().astimezone().isoformat(),'head':subprocess.check_output(['git','rev-parse','HEAD'],cwd=root,text=True).strip(),'command':command}
(directory/'source.patch').write_bytes(subprocess.check_output(['git','diff','--binary','HEAD'],cwd=root))
with (directory/'run.log').open('w') as log:
 p=subprocess.Popen(command,cwd=root,env=env,stdout=log,stderr=subprocess.STDOUT,start_new_session=True);state['pid']=p.pid;(directory/'state.json').write_text(json.dumps(state,indent=2)+'\n');print(json.dumps({'run':sys.argv[1],'pid':p.pid}),flush=True);code=p.wait()
state.update(exit_code=code,completed_at=datetime.datetime.now().astimezone().isoformat());(directory/'state.json').write_text(json.dumps(state,indent=2)+'\n');print(json.dumps({'run':sys.argv[1],'exit_code':code}),flush=True);sys.exit(code)
