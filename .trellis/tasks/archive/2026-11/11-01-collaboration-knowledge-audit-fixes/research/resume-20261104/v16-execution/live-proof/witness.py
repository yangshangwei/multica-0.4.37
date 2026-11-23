"""Read-only witness of the already-running, sole-owner v16 acceptance run."""
import datetime
import hashlib
import html
import json
import os
from pathlib import Path
import re
import shlex
import shutil
import subprocess
import time
from urllib.parse import unquote, urlsplit
from urllib.request import urlopen


HERE = Path(__file__).resolve().parent
ROOT = Path('/Volumes/artisan/code/2026/mc-collaboration-closure-v12-20261103')
PRIVATE = ROOT / '.omx/private/final-check-v16'
ENV_DIR = Path('/Volumes/artisan/code/2026/mc-collaboration-closure-v12-20261103/.omx/private/final-check-v16/dev-registry-v16/envs/check-20261104030822-8966')
WRAPPER_DIR = Path('/Volumes/artisan/code/2026/multica-0.4.37/.trellis/tasks/11-01-collaboration-knowledge-audit-fixes/research/resume-20261104/v16-execution')
EXPECTED = 'eda0d68fb07852b714f3092aa02ca0523d3ad312'
ENV_KEYS = (
    'APP_ENV', 'PORT', 'FRONTEND_PORT', 'MULTICA_DEPLOYMENT_ID',
    'MULTICA_AUTH_MODE', 'MULTICA_DEVICE_AUTH_ENABLED',
    'MULTICA_KNOWLEDGE_CONTEXT_ENABLED', 'MULTICA_MANAGED_INSTALLATIONS_ENABLED',
    'MULTICA_PASSWORD_LIMITER_MODE', 'NODE_ENV', 'NEXT_PUBLIC_API_URL',
    'NEXT_PUBLIC_WS_URL', 'FRONTEND_ORIGIN', 'PLAYWRIGHT_BASE_URL',
    'COLLABORATION_ELECTRON_RENDERER_URL',
)


def now():
    return datetime.datetime.now(datetime.timezone.utc).isoformat()


def digest(path):
    with Path(path).open('rb') as stream:
        return hashlib.file_digest(stream, 'sha256').hexdigest()


def load(path):
    try:
        return json.loads(Path(path).read_text())
    except (OSError, ValueError):
        return None


def save(path, value):
    with Path(path).open('x') as stream:
        json.dump(value, stream, indent=2, ensure_ascii=False)
        stream.write('\n')


def safe_text(text):
    text = re.sub(r'postgres(?:ql)?://\S+', '<private database URL>', text)
    return re.sub(r'\b(?:mcb_|mct_|mdt_|mat_)[A-Za-z0-9_.-]+', '<private token>', text)


def public_json(value):
    if isinstance(value, dict):
        return {
            key: public_json(item) for key, item in value.items()
            if not re.search(r'password|secret|token|api_key', key, re.I)
        }
    if isinstance(value, list):
        return [public_json(item) for item in value]
    return safe_text(value) if isinstance(value, str) else value


def run(command, environment=None, timeout=6):
    try:
        result = subprocess.run(command, env=environment, capture_output=True, text=True, timeout=timeout)
        return result.returncode, result.stdout
    except (OSError, subprocess.TimeoutExpired):
        return -1, ''


def parsed_env(path):
    result = {}
    for line in Path(path).read_text().splitlines():
        if not line or line.startswith('#') or '=' not in line:
            continue
        key, value = line.split('=', 1)
        if re.fullmatch(r'[A-Z0-9_]+', key):
            try:
                result[key] = ' '.join(shlex.split(value))
            except ValueError:
                pass
    return result


def http(path, timeout=4):
    try:
        with urlopen(path, timeout=timeout) as response:
            return {'reachable': True, 'status': response.status, 'final_url': response.url}, response.read()
    except Exception as error:
        return {'reachable': False, 'error_kind': type(error).__name__}, b''


def process(pid):
    if not isinstance(pid, int) or pid <= 0:
        return None
    code, raw = run(['/bin/ps', '-p', str(pid), '-o', 'pid=,ppid=,pgid=,lstart=,etime=,command='])
    if code or not raw.strip():
        return {'pid': pid, 'present': False}
    fields = raw.strip().split(None, 3)
    cwd_code, cwd_text = run(['/usr/sbin/lsof', '-a', '-p', str(pid), '-d', 'cwd', '-Fn'])
    cwd = next((line[1:] for line in cwd_text.splitlines() if line.startswith('n')), None)
    env_code, env_text = run(['/bin/ps', 'eww', '-p', str(pid), '-o', 'command='])
    nonsecret = {}
    if env_code == 0:
        for key in ENV_KEYS:
            match = re.search(r'(?:^|\s)' + re.escape(key) + r'=([^\s]*)', env_text)
            if match:
                nonsecret[key] = safe_text(match.group(1))
    return {
        'pid': int(fields[0]), 'parent_pid': int(fields[1]), 'process_group': int(fields[2]),
        'present': True, 'timing_and_command': safe_text(fields[3]), 'actual_cwd': cwd,
        'cwd_query_exit': cwd_code, 'observed_process_env_nonsecret': nonsecret,
    }


def ancestry(pid):
    result, seen = [], set()
    for _ in range(7):
        if pid in seen or not isinstance(pid, int) or pid <= 1:
            break
        seen.add(pid)
        item = process(pid)
        result.append(item)
        if not item or not item.get('present'):
            break
        pid = item['parent_pid']
    return result


def listener(port):
    code, raw = run(['/usr/sbin/lsof', '-nP', '-iTCP:' + str(port), '-sTCP:LISTEN', '-Fpcn'])
    pids = sorted({int(line[1:]) for line in raw.splitlines() if line.startswith('p') and line[1:].isdigit()})
    return {'port': port, 'query_exit': code, 'pids': pids, 'observation': safe_text(raw.strip())}


def database(values, manifest, psql, healthy):
    parsed = urlsplit(values['DATABASE_URL'])
    name = unquote(parsed.path[1:])
    if parsed.hostname not in ('127.0.0.1', 'localhost') or name != manifest['DB_NAME']:
        return {'refused': True, 'reason': 'Registered database does not match loopback allocation'}
    environment = {
        'PATH': '/usr/bin:/bin', 'HOME': '/nonexistent', 'LC_ALL': 'C',
        'PGHOST': parsed.hostname, 'PGPORT': str(parsed.port or 5432), 'PGDATABASE': name,
        'PGUSER': unquote(parsed.username or ''), 'PGPASSWORD': unquote(parsed.password or ''),
        'PGAPPNAME': 'multica-v16-live-proof', 'PGCONNECT_TIMEOUT': '3',
        'PGOPTIONS': '-c default_transaction_read_only=on -c statement_timeout=1500 -c lock_timeout=1500',
    }
    sql = "SELECT json_build_object('database',current_database(),'user',current_user,'schema_migrations_present',to_regclass('public.schema_migrations') IS NOT NULL,'server_address',inet_server_addr()::text,'server_port',inet_server_port(),'postmaster_started_at',pg_postmaster_start_time())::text"
    code, stdout = run([psql, '-X', '-A', '-t', '-v', 'ON_ERROR_STOP=1', '-c', sql], environment)
    result = {'query_exit': code, 'host': parsed.hostname, 'port': parsed.port or 5432, 'registered_database': name, 'read_only': True}
    if code == 0:
        try:
            result['actual'] = json.loads(stdout)
        except ValueError:
            result['malformed_output'] = True
    if healthy and result.get('actual', {}).get('schema_migrations_present'):
        sql = "SELECT json_build_object('latest_migration',(SELECT max(version) FROM schema_migrations),'connections',(SELECT json_agg(x) FROM (SELECT pid,application_name,client_addr::text,client_port,state FROM pg_stat_activity WHERE datname=current_database() AND application_name <> 'multica-v16-live-proof') x))::text"
        code, stdout = run([psql, '-X', '-A', '-t', '-v', 'ON_ERROR_STOP=1', '-c', sql], environment)
        result['migrated_query_exit'] = code
        if code == 0:
            try:
                result['migrated_actual'] = json.loads(stdout)
            except ValueError:
                result['migration_output_malformed'] = True
    return result


def source(run_record):
    code, head = run(['/usr/bin/git', '-C', str(ROOT), 'rev-parse', 'HEAD'])
    status_code, status = run(['/usr/bin/git', '-C', str(ROOT), 'status', '--porcelain', '--untracked-files=all'])
    selected = [
        'server/cmd/server/main.go', 'server/internal/database/database.go',
        'apps/web/app/layout.tsx', 'scripts/dev-env.sh', 'scripts/check.sh',
        'apps/desktop/out/renderer/index.html',
    ]
    return {
        'actual_root': str(ROOT), 'head_query_exit': code, 'actual_head': head.strip(),
        'expected_head_matches': code == 0 and head.strip() == EXPECTED,
        'status_query_exit': status_code, 'dirty_paths': status.splitlines(),
        'selected_current_file_hashes': {name: digest(ROOT / name) for name in selected if (ROOT / name).is_file()},
        'canonical_product_source_id_declared_in_run': run_record.get('canonical_product_source_id'),
        'full_source_binding_limit': 'Selected files/HEAD are observed here; registered detached final auditor owns all-file/tool/workspace/CLI/Desktop final verification.',
    }


def redis(proof):
    expected = proof['redis']
    executable = shutil.which('docker')
    result = {'registered': public_json(expected), 'inspection_available': executable is not None}
    if executable:
        code, stdout = run([executable, 'inspect', expected['container_id']], timeout=5)
        result['inspect_exit'] = code
        if code == 0:
            try:
                item = json.loads(stdout)[0]
                actual = {'id': item['Id'], 'image': item['Image'], 'running': item['State']['Running'], 'labels': item['Config']['Labels'], 'ports': item['NetworkSettings']['Ports']}
                result['actual'] = public_json(actual)
                result['container_image_labels_match'] = item['Id'] == expected['container_id'] and item['Image'] == expected['image_id'] and all(item['Config']['Labels'].get(k) == v for k, v in expected['labels'].items())
            except (ValueError, KeyError, IndexError):
                result['inspection_malformed'] = True
    return result


def collect(stage, run_record, baseline=False):
    started = now()
    manifest = parsed_env(ENV_DIR / 'manifest.env')
    values = parsed_env(ENV_DIR / 'check.env')
    psql = run_record['tools']['psql']
    if digest(psql['path']) != psql['sha256']:
        raise RuntimeError('Pinned psql bytes changed')
    api_url = 'http://127.0.0.1:' + manifest['BACKEND_PORT']
    api_http, raw_health = http(api_url + '/health', timeout=3) if stage >= 4 else ({'reachable': False, 'not_started_at_stage3': True}, b'')
    try:
        health = json.loads(raw_health) if raw_health else None
    except ValueError:
        health = None
    api_pid = health.get('pid') if isinstance(health, dict) else None
    api_listener = listener(int(manifest['BACKEND_PORT']))
    web_listener = listener(int(manifest['FRONTEND_PORT']))
    healthy = bool(api_http.get('reachable') and isinstance(api_pid, int) and api_pid > 0)
    observation = {
        'observation_started_at': started, 'reviewer': '/root/shared_executor', 'stage': stage,
        'run_state': run_record.get('state'), 'make_pid': run_record.get('process_id'),
        'candidate_sha': EXPECTED, 'retained_execution_claim': load(PRIVATE / 'execution-claim.json'),
        'manifest': {'path': str(ENV_DIR / 'manifest.env'), 'sha256': digest(ENV_DIR / 'manifest.env'), 'declared': {k: manifest.get(k) for k in ('NAME', 'DIR', 'OWNER', 'WEB_MODE', 'BACKEND_PORT', 'FRONTEND_PORT', 'DB_NAME', 'ENV_FILE', 'CREATED_AT')}},
        'task_environment': {'path': str(ENV_DIR / 'check.env'), 'sha256': digest(ENV_DIR / 'check.env'), 'declared_nonsecret': {k: safe_text(values[k]) for k in ENV_KEYS if k in values}},
        'pinned_psql': {**psql, 'actual_sha256': digest(psql['path']), 'matches': True},
        'source': source(run_record), 'api': {'url': api_url, 'http': api_http, 'actual_health_nonsecret': public_json(health), 'listener': api_listener, 'actual_process_ancestry': ancestry(api_pid), 'health_pid_is_actual_listener': api_pid in api_listener['pids'] if api_pid else False, 'recorded_provenance': public_json(load(ENV_DIR / 'api.running.json'))},
        'web': {'url': 'http://127.0.0.1:' + manifest['FRONTEND_PORT'], 'listener': web_listener, 'recorded_provenance': public_json(load(ENV_DIR / 'web.running.json'))},
        'knowledge_budget_allocated': 0, 'knowledge_real_calls': 0, 'production_knowledge_default': False,
        'commands_started': 'Read-only ps/lsof/git/HTTP/explicit own-DB psql and optional own-Redis docker inspect only; no tests/build/service/DB write/Provider commands.',
    }
    if baseline or healthy:
        observation['database'] = database(values, manifest, psql['path'], healthy)
    if healthy:
        config_http, config_body = http(api_url + '/api/config', timeout=3)
        try:
            config = json.loads(config_body) if config_body else {}
        except ValueError:
            config = {}
        observation['api']['public_config_http'] = config_http
        observation['api']['public_config_nonsecret'] = {key: config.get(key) for key in ('deployment_id', 'auth_mode', 'managed_installations_enabled', 'device_auth_enabled')}
        env_code, env_text = run(['/bin/ps', 'eww', '-p', str(api_pid), '-o', 'command='])
        match = re.search(r'(?:^|\s)DATABASE_URL=([^\s]*)', env_text) if env_code == 0 else None
        observation['api']['actual_database_url_matches_registered'] = bool(match and match.group(1) == values['DATABASE_URL'])
        observation['api']['actual_deployment_uuid_matches_registered'] = config.get('deployment_id') == values.get('MULTICA_DEPLOYMENT_ID')

    if healthy:
        observation['api']['actual_config_http'], config = http(api_url + '/api/config')
        try:
            observation['api']['actual_config_nonsecret'] = public_json(json.loads(config))
        except ValueError:
            pass
        if api_pid:
            code, sockets = run(['/usr/sbin/lsof', '-nP', '-a', '-p', str(api_pid), '-iTCP', '-Fpn'])
            observation['api']['owned_tcp_connections'] = {'query_exit': code, 'observation': safe_text(sockets.strip())}
    if stage >= 5 and web_listener['pids']:
        web_http, body = http(observation['web']['url'] + '/', timeout=5)
        text = html.unescape(body.decode(errors='replace')).replace('\\"', '"')
        configuration = {key: value for key, value in re.findall(r'"(apiBaseUrl|wsUrl)"\s*:\s*"([^\"]+)"', text)}
        observation['web'].update(http=web_http, html_sha256=hashlib.sha256(body).hexdigest() if body else None, html_bytes=len(body), actual_rendered_public_configuration=configuration,
                                  rendered_API_WS_match_declared=configuration.get('apiBaseUrl') == values.get('NEXT_PUBLIC_API_URL') and configuration.get('wsUrl') == values.get('NEXT_PUBLIC_WS_URL'),
                                  actual_process_ancestry=ancestry(web_listener['pids'][0]))
        build = ROOT / 'apps/web/.next/BUILD_ID'
        if build.exists():
            observation['web']['actual_build_id'] = build.read_text().strip()
        build_log = ENV_DIR / 'logs/web-build.log'
        if build_log.exists():
            observation['web']['actual_build_log'] = {'path': str(build_log), 'sha256': digest(build_log), 'bytes': build_log.stat().st_size}
        if stage >= 6 and body:
            artifact = HERE / ('web-public-' + datetime.datetime.now(datetime.timezone.utc).strftime('%H%M%S%f') + '.html')
            with artifact.open('xb') as sink:
                sink.write(body)
            observation['web']['unauthenticated_public_HTML_artifact'] = str(artifact)
    if baseline or healthy or stage >= 5:
        ownership = load(PRIVATE / 'resource-ownership-v16.json')
        if ownership:
            renderer_http, renderer_body = http(ownership['renderer']['url'])
            renderer_listener = listener(urlsplit(ownership['renderer']['url']).port)
            observation['renderer'] = {'registered': public_json(ownership['renderer']), 'http': renderer_http, 'actual_index_sha256': hashlib.sha256(renderer_body).hexdigest() if renderer_body else None,
                                       'current_artifact_sha256': digest(ROOT / 'apps/desktop/out/renderer/index.html'), 'listener': renderer_listener,
                                       'actual_process_ancestry': ancestry(ownership['renderer']['pid']),
                                       'registered_pid_is_actual_listener': ownership['renderer']['pid'] in renderer_listener['pids'],
                                       'HTTP_bytes_match_current_and_registered': renderer_body == (ROOT / 'apps/desktop/out/renderer/index.html').read_bytes() and hashlib.sha256(renderer_body).hexdigest() == ownership['renderer']['index_sha256']}
            observation['redis'] = redis(ownership)
    observation['observation_finished_at'] = now()
    return observation


def current_stage():
    status = load(WRAPPER_DIR / 'status.json') or {}
    try:
        stage = int(status.get('stage', ['3'])[0])
    except (TypeError, ValueError, IndexError):
        stage = 3
    with (PRIVATE / 'make-check-v16.log').open('rb') as stream:
        stream.seek(max(0, (PRIVATE / 'make-check-v16.log').stat().st_size - 200000))
        tail = re.sub(r'\x1b\[[0-9;]*m', '', stream.read().decode(errors='replace'))
    markers = re.findall(r'\[(\d)/6\]\s+([^\r\n]+)', tail)
    return int(markers[-1][0]) if markers else stage


def main():
    save(HERE / 'witness-start.json', {'started_at': now(), 'pid': os.getpid(), 'script_sha256': digest(__file__), 'mode': 'readonly_existing_single_v16_witness', 'new_tests': 0, 'writes_scope': str(HERE)})
    initial = True
    previous = None
    last_saved = 0.0
    with (HERE / 'observations.jsonl').open('x') as summary:
        while True:
            run_record = load(PRIVATE / 'make-check-v16.json')
            if run_record is None:
                raise RuntimeError('Existing run record is unavailable')
            stage = current_stage()
            observation = collect(stage, run_record, baseline=initial)
            signature = (stage, run_record.get('state'), observation['api']['actual_health_nonsecret'].get('pid') if isinstance(observation['api']['actual_health_nonsecret'], dict) else None,
                         bool(observation['web'].get('http', {}).get('reachable')), bool(observation['web'].get('rendered_API_WS_match_declared')))
            changed = signature != previous
            if initial or changed or time.monotonic() - last_saved >= 300:
                label = datetime.datetime.now(datetime.timezone.utc).strftime('%H%M%S%f')
                snapshot = HERE / ('snapshot-' + label + '.json')
                save(snapshot, observation)
                event = {'observed_at': now(), 'stage': stage, 'run_state': run_record.get('state'), 'api_live': bool(signature[2]), 'web_live': signature[3], 'rendered_config_matches': signature[4], 'snapshot': str(snapshot), 'snapshot_sha256': digest(snapshot), 'phase_or_live_state_changed': changed}
                summary.write(json.dumps(event) + '\n')
                summary.flush()
                last_saved = time.monotonic()
                if changed or initial:
                    print(json.dumps(event), flush=True)
            initial, previous = False, signature
            finished = load(WRAPPER_DIR / 'execution-finished.json')
            if finished:
                audit_path = Path(finished['final_audit_path']) if finished.get('final_audit_path') else None
                audit = load(audit_path) if audit_path else None
                final = {'observed_at': now(), 'original_wrapper_result': finished, 'original_wrapper_path': str(WRAPPER_DIR / 'execute-once.py'), 'original_wrapper_sha256': digest(WRAPPER_DIR / 'execute-once.py'),
                         'wrapper_result_sha256': digest(WRAPPER_DIR / 'execution-finished.json'), 'actual_run_path': str(PRIVATE / 'make-check-v16.json'), 'actual_run_sha256': digest(PRIVATE / 'make-check-v16.json'),
                         'actual_raw_log_path': str(PRIVATE / 'make-check-v16.log'), 'actual_raw_log_sha256': digest(PRIVATE / 'make-check-v16.log'),
                         'registered_final_audit': audit, 'registered_final_audit_path': str(audit_path) if audit_path else None, 'actual_final_audit_hash_matches_wrapper': bool(audit_path and audit_path.exists() and digest(audit_path) == finished.get('final_audit_sha256')),
                         'make_exit': finished.get('make_exit_code'), 'final_audit_exit': finished.get('final_audit_exit_code'), 'wholecheck_acceptance': 'not decided by witness; full named/skip inventory and root acceptance remain required',
                         'named_evidence_source_paths': [str(PRIVATE / 'go-packages-v16'), str(ROOT / 'test-results'), str(ROOT / 'playwright-report'), str(WRAPPER_DIR / 'status.json')],
                         'second_auditor_started': False, 'tests_started': 0, 'resource_or_DB_mutations': 0, 'Provider_calls': 0}
                save(HERE / 'completion-witness.json', final)
                print(json.dumps({'completion_observed': True, 'make_exit': final['make_exit'], 'final_audit_exit': final['final_audit_exit'], 'record': str(HERE / 'completion-witness.json')}), flush=True)
                return
            time.sleep(30)


if __name__ == '__main__':
    main()
