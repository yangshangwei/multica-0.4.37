"""Execute one original-flow diagnostic per locale with passive telemetry."""
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import shlex
import subprocess


HERE = Path(__file__).resolve().parent
prepared = json.loads((HERE / 'preparation.json').read_text())
private = Path(prepared['private'])
runtime = Path(prepared['diagnostic_checkout'])
assert json.loads((HERE / 'live-before-fixtures.json').read_text())['test_fixture_writes_started'] is False
environment = json.loads((private / 'startup-environment-retry1.json').read_text())
for line in (private / 'source.env').read_text().splitlines():
    if line and not line.startswith('#'):
        key, value = line.split('=', 1)
        tokens = shlex.split(value)
        environment[key] = tokens[0] if tokens else ''
for key in ('DATABASE_MAX_CONNS', 'DATABASE_MIN_CONNS', 'MULTICA_RESOURCE_PUBLISH_DIR'):
    environment.pop(key, None)
environment.update(PLAYWRIGHT_BROWSERS_PATH=os.environ['PLAYWRIGHT_BROWSERS_PATH'], PYTHONDONTWRITEBYTECODE='1')
source = json.loads((HERE / 'natural-source-registration.json').read_text())
config = Path(source['config_path'])
assert hashlib.sha256(config.read_bytes()).hexdigest() == source['config_sha256']
command = ['/Users/artisan/.nvm/versions/node/v22.23.2/bin/node', str(runtime / 'node_modules/@playwright/test/cli.js'),
           'test', '--config', str(config), '--workers=1', '--retries=0', '--trace=on']
record = {'started_at': datetime.now(timezone.utc).isoformat(), 'command': command, 'cwd': str(runtime),
          'source_registration_sha256': hashlib.sha256((HERE / 'natural-source-registration.json').read_bytes()).hexdigest(),
          'live_proof_sha256': hashlib.sha256((HERE / 'live-before-fixtures.json').read_bytes()).hexdigest(),
          'scope': 'one natural original-flow execution per EN/ZH, passive focus telemetry only; no artificial focus schedule',
          'knowledge_calls': 0, 'model_keys_forwarded': False, 'retries': 0}
log = HERE / 'natural-stdout.log'
with (HERE / 'natural-start.json').open('x') as stream:
    json.dump(record, stream, indent=2)
    stream.write('\n')
with log.open('x') as stream:
    result = subprocess.run(command, cwd=runtime, env=environment, stdout=stream, stderr=subprocess.STDOUT)
report = private / 'natural-report.json'
record.update(finished_at=datetime.now(timezone.utc).isoformat(), exit_code=result.returncode,
              log_path=str(log), log_sha256=hashlib.sha256(log.read_bytes()).hexdigest(),
              report_path=str(report), report_sha256=hashlib.sha256(report.read_bytes()).hexdigest() if report.exists() else None)
with (HERE / 'natural-result.json').open('x') as stream:
    json.dump(record, stream, indent=2)
    stream.write('\n')
print(json.dumps(record))
raise SystemExit(result.returncode)
