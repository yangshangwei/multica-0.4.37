"""Run the four original locale/client cases with no diagnostic controls."""
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import shlex
import subprocess


HERE = Path(__file__).resolve().parent
prepared = json.loads((HERE / 'preparation.json').read_text())
runtime = Path(prepared['runtime'])
private = Path(prepared['private'])
environment = json.loads(Path(prepared['startup_environment_path']).read_text())
for line in Path(prepared['env_path']).read_text().splitlines():
    if line and not line.startswith('#'):
        key, raw = line.split('=', 1)
        tokens = shlex.split(raw)
        environment[key] = tokens[0] if tokens else ''
for key in list(environment):
    if key.startswith('TRIAGE_') or key in ('NODE_OPTIONS', 'ELECTRON_RUN_AS_NODE', 'DATABASE_MAX_CONNS',
                                           'DATABASE_MIN_CONNS', 'MULTICA_RESOURCE_PUBLISH_DIR'):
        environment.pop(key, None)
environment.update(PLAYWRIGHT_BROWSERS_PATH=os.environ['PLAYWRIGHT_BROWSERS_PATH'],
                   MULTICA_E2E_DESKTOP_OUTPUT_DIR=prepared['desktop_output'])
source = json.loads((HERE / 'test-source-registration.json').read_text())
config = Path(source['config'])
assert hashlib.sha256(config.read_bytes()).hexdigest() == source['config_sha256']
assert hashlib.sha256(Path(source['fixture']).read_bytes()).hexdigest() == '0cbc13c518022a2af413da1f09b3801e8b0f4a6c7264bbf395544a301a69c029'
assert 'focus-observer' not in Path(source['fixture']).read_text() and 'focus-control' not in Path(source['fixture']).read_text()
command = ['/Users/artisan/.nvm/versions/node/v22.23.2/bin/node', str(runtime / 'node_modules/@playwright/test/cli.js'),
           'test', '--config', str(config), '--workers=1', '--retries=0', '--trace=on']
record = {'started_at': datetime.now(timezone.utc).isoformat(), 'command': command, 'cwd': str(runtime),
          'scope': 'four original canonical Web/native EN/ZH cases, no observer/RAF/server hooks; only two reviewed assertions',
          'fixture_sha256': source['fixture_sha256'], 'test_source_sha256': hashlib.sha256((HERE / 'test-source-registration.json').read_bytes()).hexdigest(),
          'live_proof_sha256': hashlib.sha256((HERE / 'live-before-fixtures.json').read_bytes()).hexdigest(),
          'Node_options_absent': 'NODE_OPTIONS' not in environment, 'control_environment_absent': not any(k.startswith('TRIAGE_') for k in environment),
          'original_assertions': 52, 'added_assertions': 2, 'knowledge_calls': 0, 'retries': 0}
with (HERE / 'run-start.json').open('x') as stream:
    json.dump(record, stream, indent=2)
    stream.write('\n')
log = HERE / 'raw-stdout.log'
with log.open('x') as stream:
    result = subprocess.run(command, cwd=runtime, env=environment, stdout=stream, stderr=subprocess.STDOUT)
report = private / 'report.json'
record.update(finished_at=datetime.now(timezone.utc).isoformat(), exit=result.returncode,
              stdout_sha256=hashlib.sha256(log.read_bytes()).hexdigest(), report_path=str(report),
              report_sha256=hashlib.sha256(report.read_bytes()).hexdigest() if report.is_file() else None)
with (HERE / 'run-result.json').open('x') as stream:
    json.dump(record, stream, indent=2)
    stream.write('\n')
print(json.dumps(record))
raise SystemExit(result.returncode)
