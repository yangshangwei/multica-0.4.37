import argparse
import datetime
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import types
from urllib.parse import urlsplit

ROOT = Path('/Volumes/artisan/code/2026/mc-collaboration-closure-v12-20261103')
PRIVATE = ROOT / '.omx/private/knowledge-final-gates-v13'
EXPECTED = 'd0b132fff0c477eb2d744933e13db730240231aa'

def require(condition, message):
    if not condition:
        raise RuntimeError(message)

def digest(path):
    with Path(path).open('rb') as source:
        return hashlib.file_digest(source, 'sha256').hexdigest()

def write_json(path, value):
    descriptor = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(descriptor, 'w') as target:
        json.dump(value, target, indent=2)
        target.write('\n')

def admission(gate, permit_path, label):
    require(re.fullmatch(r'[a-z0-9][a-z0-9._-]*', label), 'Invalid private result label')
    require(not permit_path.is_symlink(), 'Permit must not be redirected')
    permit_path = permit_path.resolve()
    require(permit_path.parent == PRIVATE / 'permissions', 'Permit outside owned directory')
    permission = json.loads(permit_path.read_text())
    require(permission.get('ready_to_execute') is True and permission.get('gate') == gate, 'Later root execution permission is required')
    require(permission.get('root_released_complete_check_resources') is True and permission.get('root_verified_complete_named_inventory') is True,
            'Complete-check resources and inventory are not cleared by root')
    require(permission.get('root_verified_fresh_database') is True and permission.get('real_provider_allowed') is False, 'A fresh fake-only database permission is required')
    require(permission.get('candidate_sha') == EXPECTED and permission.get('runner_sha256') == digest(__file__), 'Runner or candidate differs from root permission')
    inputs_path = PRIVATE / 'source-inputs.json'
    require(permission.get('source_inputs_sha256') == digest(inputs_path), 'Source inputs differ from root permission')
    inputs = json.loads(inputs_path.read_text())
    require(inputs['candidate_sha'] == EXPECTED and inputs['candidate_root'] == str(ROOT), 'Wrong source candidate')
    for name, item in inputs['tools'].items():
        require(digest(item['path']) == item['sha256'], 'Pinned tool changed: ' + name)
    for name, sha in inputs['source_sha256'].items():
        require(digest(ROOT / name) == sha, 'Gate source changed: ' + name)
    audit_spec, review_spec = inputs['audit_module'], inputs['audit_review']
    require(digest(audit_spec['path']) == audit_spec['sha256'] and digest(review_spec['path']) == review_spec['sha256'], 'Reviewed final source verifier changed')
    module = types.ModuleType('registered_ccg_final_source_auditor')
    module.__file__ = audit_spec['path']
    exec(compile(Path(audit_spec['path']).read_bytes(), audit_spec['path'], 'exec'), module.__dict__)
    audited = module.audit(ROOT, json.loads(Path(review_spec['path']).read_text()))
    require(audited['status'] == 'pass' and audited['wholecheck_passed'] is True, 'Actual successful complete check and final source identities are required')
    raw_temp = Path(permission['temp_directory'])
    require(not raw_temp.is_symlink(), 'Build temp must not be redirected')
    temp = raw_temp.resolve()
    require(str(temp).startswith('/private/tmp/multica-ccg-v14-') and temp.is_dir() and not temp.is_symlink()
            and temp.stat().st_uid == os.geteuid() and temp.stat().st_mode & 0o777 == 0o700, 'Missing root-allocated isolated build temp')
    require(subprocess.run(['git', '-C', str(temp), 'rev-parse', '--show-toplevel'], stdout=subprocess.PIPE, stderr=subprocess.PIPE).returncode != 0,
            'CCG build temp must be outside every Git checkout')
    database = permission['database']
    parsed = urlsplit(permission['DATABASE_URL'])
    require(re.fullmatch(r'[a-z0-9_]+', database) and database not in ('multica', 'postgres', 'template0', 'template1')
            and parsed.hostname in ('127.0.0.1', 'localhost') and parsed.path == '/' + database, 'Database does not match root allocation')
    identity = subprocess.run([inputs['tools']['psql']['path'], permission['DATABASE_URL'], '-X', '-Atqc',
                               'SELECT current_database(),max(version) FROM schema_migrations'], capture_output=True, text=True)
    require(identity.returncode == 0 and identity.stdout.strip() == permission['identity_verified']
            and permission['identity_verified'].startswith(database + '|764_'), 'Database identity/migration 764 does not match root proof')
    output = PRIVATE / 'runs' / label
    output.parent.mkdir(mode=0o700, exist_ok=True)
    output.mkdir(mode=0o700)
    cache = permission['cache_paths']
    require(cache['GOCACHE'] == str(PRIVATE / 'go-build-cache') and cache['GOPATH'] == str(PRIVATE / 'go-path')
            and cache['GOMODCACHE'] == '/Users/artisan/go/pkg/mod', 'Unreviewed cache paths')
    environment = {'PATH': str(Path(inputs['tools']['node']['path']).parent) + ':' + str(Path(inputs['tools']['go']['path']).parent)
                   + ':/opt/homebrew/opt/libpq/bin:/usr/bin:/bin:/usr/sbin:/sbin',
                   'HOME': str(output / 'home'), 'TMPDIR': str(temp), 'TMP': str(temp), 'TEMP': str(temp),
                   'GOROOT': inputs['goroot'], 'GOCACHE': cache['GOCACHE'], 'GOMODCACHE': cache['GOMODCACHE'], 'GOPATH': cache['GOPATH'],
                   'GOPROXY': 'file:///Users/artisan/go/pkg/mod/cache/download', 'GOSUMDB': 'off', 'GOTOOLCHAIN': 'local',
                   'GOENV': 'off', 'GOWORK': 'off', 'GOTELEMETRY': 'off', 'GOMAXPROCS': '2', 'LC_ALL': 'C',
                   'DATABASE_URL': permission['DATABASE_URL'], 'NEXT_TELEMETRY_DISABLED': '1', 'COREPACK_ENABLE_NETWORK': '0',
                   'PNPM_DISABLE_SELF_UPDATE_CHECK': '1', 'NPM_CONFIG_USERCONFIG': '/dev/null', 'NPM_CONFIG_GLOBALCONFIG': '/dev/null'}
    (output / 'home').mkdir(mode=0o700)
    return permission, inputs, audited, output, environment

from urllib.request import Request, urlopen
import base64

def checked_file(spec):
    path = Path(spec['path']).resolve()
    require(path.is_relative_to(PRIVATE) and digest(path) == spec['sha256'], 'CCG-owned input differs from root binding')
    return path

def loopback_origin(value):
    parsed = urlsplit(value)
    require(parsed.scheme in ('http', 'https') and parsed.hostname in ('127.0.0.1', 'localhost') and parsed.username is None
            and parsed.password is None and parsed.port is not None, 'Expected explicit credential-free loopback origin')
    return value.rstrip('/')

def main():
    parser = argparse.ArgumentParser(description='Current CCG2 deployment probe; later root allocation/permission only')
    parser.add_argument('lane', choices=('off', 'on'))
    parser.add_argument('--permit', type=Path, required=True)
    parser.add_argument('--label', required=True)
    args = parser.parse_args()
    permission, inputs, audited, output, environment = admission('ccg2-' + args.lane, args.permit, args.label)
    expected = 'true' if args.lane == 'on' else 'false'
    envfile = checked_file({'path': permission['source_environment'], 'sha256': permission['source_environment_sha256']})
    proof_file = checked_file({'path': permission['deployment_proof'], 'sha256': permission['deployment_proof_sha256']})
    binaries_file = checked_file({'path': permission['binary_binding'], 'sha256': permission['binary_binding_sha256']})
    proof = json.loads(proof_file.read_text())
    binaries = json.loads(binaries_file.read_text())
    parser_source = inputs['environment_parser']
    require(digest(parser_source['path']) == parser_source['sha256'], 'Reviewed environment data parser changed')
    loader = types.ModuleType('registered_ccg_environment_parser')
    loader.__file__ = parser_source['path']
    exec(compile(Path(parser_source['path']).read_bytes(), parser_source['path'], 'exec'), loader.__dict__)
    loaded = loader.parse_generated_environment(envfile.read_text())
    require(loaded.get('MULTICA_KNOWLEDGE_CONTEXT_ENABLED') == expected and loaded.get('E2E_PASSWORD_AUTH') == '1'
            and loaded.get('MULTICA_AUTH_MODE') == 'password' and loaded.get('MULTICA_MANAGED_INSTALLATIONS_ENABLED') == 'true', 'Deployment flags differ from this explicit probe')
    require(loaded.get('DATABASE_URL') == permission['DATABASE_URL'], 'Browser environment database differs from its allocation')
    require(not loaded.get('MULTICA_LLM_API_KEY') and not loaded.get('MULTICA_LLM_BASE_URL'), 'Real model configuration is forbidden')
    api = loopback_origin(loaded['COLLABORATION_E2E_API_URL'])
    web = loopback_origin(loaded['PLAYWRIGHT_BASE_URL'])
    renderer = loopback_origin(loaded['COLLABORATION_ELECTRON_RENDERER_URL'])
    require(loopback_origin(loaded['NEXT_PUBLIC_API_URL']) == api and renderer not in inputs['forbidden_renderer_origins'], 'Client origin differs or reuses the full-check renderer')
    require(proof.get('candidate_sha') == EXPECTED and proof.get('database') == permission['database']
            and proof.get('source_environment_sha256') == digest(envfile) and proof.get('distinct_from_complete_check_resources') is True,
            'Deployment proof is not current/independent')
    api_proof, web_proof, renderer_proof = proof['api'], proof['web'], proof['renderer']
    require(api_proof.get('origin') == api and api_proof.get('actual_startup_knowledge_flag') == expected
            and api_proof.get('flag_observed_from_owned_process') is True, 'Actual owned API startup flag is not proved')
    require(web_proof.get('origin') == web and web_proof.get('mode') == 'production'
            and web_proof.get('rendered_api_base_url') == api and web_proof.get('rendered_runtime_config_verified') is True
            and web_proof.get('rendered_ws_url') == loaded.get('NEXT_PUBLIC_WS_URL'),
            'Production browser runtime configuration not verified')
    health = json.load(urlopen(api + '/health', timeout=10))
    require(health.get('commit') in (EXPECTED, EXPECTED[:9]) and health.get('pid') == api_proof['pid'] and health['pid'] > 0,
            'Actual API listener commit/PID differs from root proof')
    config = json.load(urlopen(api + '/api/config', timeout=10))
    require(config.get('auth_mode') == 'password' and config.get('managed_installation_supported') is True
            and config.get('password_signup_available') is True, 'Configured password/managed admission is unavailable')
    for origin in (web, renderer):
        request = Request(api + '/api/config', method='OPTIONS', headers={'Origin': origin, 'Access-Control-Request-Method': 'GET', 'Access-Control-Request-Headers': 'content-type'})
        with urlopen(request, timeout=10) as response:
            require(response.headers.get('Access-Control-Allow-Origin') == origin, 'Actual API CORS does not admit the bound origin')
    rendered = urlopen(renderer + '/', timeout=10).read()
    require(renderer_proof.get('origin') == renderer and renderer_proof.get('ownership_verified') is True
            and hashlib.sha256(rendered).hexdigest() == renderer_proof['index_sha256']
            and rendered == (ROOT / 'apps/desktop/out/renderer/index.html').read_bytes(), 'Owned renderer bytes differ from current artifact')
    require(binaries.get('candidate_sha') == EXPECTED and binaries.get('exit_code') == 0, 'Current fake binaries were not built successfully')
    require(set(binaries['source_sha256']) == {'server/internal/daemon/collaboration_host_router_test.go', 'e2e/fixtures/collaboration-host-provider.go'},
            'Fake daemon/provider source inventory is incomplete')
    for path, sha in binaries['source_sha256'].items():
        require(inputs['source_sha256'].get(path) == sha and digest(ROOT / path) == sha, 'Fake binary source differs')
    for key in ('COLLABORATION_HOST_DAEMON', 'COLLABORATION_HOST_PROVIDER', 'COLLABORATION_HOST_CLI'):
        executable = checked_file(binaries['binaries'][key])
        require(os.access(executable, os.X_OK), 'Explicit fake/CLI binary is not executable')
        environment[key] = str(executable)
    environment.update({key: loaded[key] for key in ('DATABASE_URL', 'COLLABORATION_E2E_API_URL', 'NEXT_PUBLIC_API_URL',
                        'PLAYWRIGHT_BASE_URL', 'MULTICA_DEPLOYMENT_ID', 'E2E_PASSWORD_AUTH', 'COLLABORATION_ELECTRON_RENDERER_URL')})
    environment.update(COLLABORATION_KNOWLEDGE_EXPECTED=expected, COLLABORATION_SOURCE_CLI=environment['COLLABORATION_HOST_CLI'],
                       COLLABORATION_ELECTRON_SOURCE_ROOT=str(ROOT), PLAYWRIGHT_JSON_OUTPUT_NAME=str(output / 'results.json'))
    command = ['bash', 'scripts/go-test-with-agent-cli-guard.sh', '--', inputs['tools']['pnpm']['path'], 'exec', 'playwright', 'test',
               'e2e/collaboration-knowledge-deployed.spec.ts', '--workers=1', '--retries=0', '--reporter=line,json', '--output=' + str(output / 'artifacts')]
    record = {'candidate_sha': EXPECTED, 'lane': args.lane, 'expected_knowledge': expected, 'command': command,
              'permit_sha256': digest(args.permit), 'source_env_sha256': digest(envfile), 'deployment_proof_sha256': digest(proof_file),
              'binary_binding_sha256': digest(binaries_file), 'database': permission['database'], 'health': health,
              'root_fullcheck_identity_audit': audited, 'state': 'running',
              'started_at': datetime.datetime.now(datetime.timezone.utc).isoformat(), 'real_Provider_calls': 0}
    write_json(output / 'started.json', record)
    log_path = output / 'output.log'
    descriptor = os.open(log_path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(descriptor, 'w') as log:
        result = subprocess.run(command, cwd=ROOT, env=environment, stdout=log, stderr=subprocess.STDOUT)
    report_path = output / 'results.json'
    stats, observations = {}, []
    if report_path.exists():
        report_path.chmod(0o600)
        report = json.loads(report_path.read_text())
        stats = report.get('stats', {})
        def visit(suites):
            for suite in suites:
                yield from visit(suite.get('suites', []))
                for spec in suite.get('specs', []):
                    for test in spec.get('tests', []):
                        for attempt in test.get('results', []):
                            for attachment in attempt.get('attachments', []):
                                if attachment.get('name') == 'deployed-knowledge-gate-evidence' and attachment.get('body'):
                                    yield json.loads(base64.b64decode(attachment['body']))
        observations = list(visit(report.get('suites', [])))
        record.update(results_sha256=digest(report_path), report_errors=report.get('errors', []))
    accepted = result.returncode == 0 and stats.get('expected') == 1 and all(stats.get(key) == 0 for key in ('unexpected', 'flaky', 'skipped')) and not record.get('report_errors') and len(observations) == 1
    if observations:
        observation = observations[0]
        knowledge = expected == 'true'
        events = {name: (1 if knowledge else 0) for name in ('knowledge_claim', 'knowledge_preparation', 'knowledge_authorization')}
        accepted = accepted and observation.get('real_provider_calls') == 0 and observation.get('expected_knowledge_enabled') is knowledge
        accepted = accepted and observation.get('actual_input', {}).get('section_present') is knowledge and observation.get('knowledge_events') == events
        record['observation_sha256'] = hashlib.sha256(json.dumps(observation, sort_keys=True, separators=(',', ':')).encode()).hexdigest()
    record.update(state='complete', exit_code=result.returncode, stats=stats, accepted=bool(accepted),
                  finished_at=datetime.datetime.now(datetime.timezone.utc).isoformat(), log_sha256=digest(log_path))
    write_json(output / 'result.json', record)
    print(json.dumps({'gate': 'CCG2', 'lane': args.lane, 'exit_code': result.returncode, 'stats': stats, 'accepted': bool(accepted), 'record': str(output / 'result.json')}))
    return result.returncode or (0 if accepted else 1)

if __name__ == '__main__':
    try:
        raise SystemExit(main())
    except (RuntimeError, OSError, ValueError, KeyError, TypeError) as error:
        print('REFUSED: ' + (str(error) if isinstance(error, RuntimeError) else 'Required private deployed input unavailable'))
        raise SystemExit(2)
