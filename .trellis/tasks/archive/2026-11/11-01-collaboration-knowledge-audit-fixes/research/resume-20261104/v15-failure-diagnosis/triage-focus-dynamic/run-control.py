"""Run one explicitly scheduled mechanism control, never an acceptance rerun."""
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import shlex
import subprocess
import sys


HERE = Path(__file__).resolve().parent
private = Path('/Volumes/artisan/code/2026/mc-collaboration-closure-v12-20261103/.omx/private/triage-focus-diagnostic-v15')
runtime = private / 'runtime'
mode = sys.argv[1]
assert mode in ('redirect', 'await-initial')
registration = json.loads((HERE / 'control-source-registration.json').read_text())
selected = next(row for row in registration['controls'] if row['mode'] == mode)
config = Path(selected['config'])
assert hashlib.sha256(config.read_bytes()).hexdigest() == selected['config_sha256']
assert hashlib.sha256(Path(registration['hook_path']).read_bytes()).hexdigest() == registration['hook_sha256']
environment = json.loads((private / 'startup-environment-retry1.json').read_text())
for line in (private / 'source.env').read_text().splitlines():
    if line and not line.startswith('#'):
        key, value = line.split('=', 1)
        tokens = shlex.split(value)
        environment[key] = tokens[0] if tokens else ''
for key in ('DATABASE_MAX_CONNS', 'DATABASE_MIN_CONNS', 'MULTICA_RESOURCE_PUBLISH_DIR'):
    environment.pop(key, None)
environment.update(PLAYWRIGHT_BROWSERS_PATH=os.environ['PLAYWRIGHT_BROWSERS_PATH'],
                   TRIAGE_FOCUS_CONTROL_MODE=mode, TRIAGE_PLAYWRIGHT_SERVER_INPUT=registration['server_input_module'],
                   NODE_OPTIONS='--require ' + registration['hook_path'])
command = ['/Users/artisan/.nvm/versions/node/v22.23.2/bin/node', str(runtime / 'node_modules/@playwright/test/cli.js'),
           'test', '--config', str(config), '--workers=1', '--retries=0', '--trace=on']
record = {'started_at': datetime.now(timezone.utc).isoformat(), 'mode': mode, 'command': command,
          'scope': 'explicit forced-schedule mechanism proof, separate from original V15 and natural runs',
          'production_or_installed_SDK_files_changed': False, 'knowledge_calls': 0, 'retries': 0}
with (HERE / (mode + '-start.json')).open('x') as stream:
    json.dump(record, stream, indent=2)
    stream.write('\n')
log = HERE / (mode + '-stdout.log')
with log.open('x') as stream:
    result = subprocess.run(command, cwd=runtime, env=environment, stdout=stream, stderr=subprocess.STDOUT)
report = private / (mode + '-report.json')
record.update(finished_at=datetime.now(timezone.utc).isoformat(), exit_code=result.returncode,
              log_path=str(log), log_sha256=hashlib.sha256(log.read_bytes()).hexdigest(),
              report_path=str(report), report_sha256=hashlib.sha256(report.read_bytes()).hexdigest() if report.exists() else None)
with (HERE / (mode + '-result.json')).open('x') as stream:
    json.dump(record, stream, indent=2)
    stream.write('\n')
print(json.dumps(record))
raise SystemExit(result.returncode)
