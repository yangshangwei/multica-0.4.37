import argparse
import base64
import copy
import hashlib
import io
import json
import os
from pathlib import Path
import shlex
import subprocess
import sys
import tempfile
import types
import unittest
from unittest.mock import patch


ROOT = Path('/Volumes/artisan/code/2026/mc-collaboration-closure-v12-20261103')
PRIVATE = ROOT / '.omx/private/knowledge-final-gates-v16'
CACHE = '/Users/artisan/Library/Caches/ms-playwright'
BINDING = PRIVATE / 'browser-cache-binding-v16.json'
REAL_RUN = subprocess.run


def digest(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def save(path, value):
    descriptor = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(descriptor, 'w') as target:
        json.dump(value, target, indent=2)
        target.write('\n')


class Response(io.BytesIO):
    def __init__(self, body, origin=None):
        super().__init__(body)
        self.headers = {'Access-Control-Allow-Origin': origin}


RECORDER = '''import base64,json,os,sys
from pathlib import Path
def write(path,value):
    fd=os.open(path,os.O_WRONLY|os.O_CREAT|os.O_EXCL,0o600)
    with os.fdopen(fd,"w") as f:json.dump(value,f)
write(os.environ["CONTROL_CAPTURE"],{"argv":sys.argv[1:],"browser_path":os.environ.get("PLAYWRIGHT_BROWSERS_PATH"),"home":os.environ["HOME"],"temp":os.environ["TMPDIR"],"database":os.environ["DATABASE_URL"],"model_credentials_present":any(os.environ.get(k) for k in ("ANTHROPIC_API_KEY","OPENAI_API_KEY","MULTICA_LLM_API_KEY")),"scope":"fake argv/env recorder; no browser/API/DB/provider execution"})
evidence={"real_provider_calls":0,"expected_knowledge_enabled":False,"actual_input":{"section_present":False},"knowledge_events":{"knowledge_claim":0,"knowledge_preparation":0,"knowledge_authorization":0}}
attachment={"name":"deployed-knowledge-gate-evidence","body":base64.b64encode(json.dumps(evidence).encode()).decode()}
write(os.environ["PLAYWRIGHT_JSON_OUTPUT_NAME"],{"stats":{"expected":1,"unexpected":0,"flaky":0,"skipped":0},"errors":[],"suites":[{"specs":[{"tests":[{"results":[{"attachments":[attachment]}]}]}]}]})
'''


def exercise(runner_path, mutation=None, missing_binding=False):
    fixture = Path(tempfile.mkdtemp(prefix='browser-env-control-', dir=PRIVATE))
    output = fixture / 'output'
    output.mkdir(mode=0o700)
    home = output / 'home'
    home.mkdir(mode=0o700)
    temp = Path(tempfile.mkdtemp(prefix='multica-ccg-v16-browser-control-', dir='/private/tmp'))
    recorder = fixture / 'recorder.py'
    recorder.write_text(RECORDER)
    recorder.chmod(0o600)
    capture = fixture / 'child-env.json'
    inputs = json.loads((PRIVATE / 'source-inputs.json').read_text())
    envvalues = {
        'MULTICA_KNOWLEDGE_CONTEXT_ENABLED': 'false', 'E2E_PASSWORD_AUTH': '1',
        'MULTICA_AUTH_MODE': 'password', 'MULTICA_MANAGED_INSTALLATIONS_ENABLED': 'true',
        'DATABASE_URL': 'postgres://127.0.0.1:1/credential_free_fixture',
        'MULTICA_LLM_API_KEY': '', 'MULTICA_LLM_BASE_URL': '',
        'COLLABORATION_E2E_API_URL': 'http://127.0.0.1:29310',
        'NEXT_PUBLIC_API_URL': 'http://127.0.0.1:29310',
        'PLAYWRIGHT_BASE_URL': 'http://127.0.0.1:29311',
        'COLLABORATION_ELECTRON_RENDERER_URL': 'http://127.0.0.1:29312',
        'NEXT_PUBLIC_WS_URL': 'ws://127.0.0.1:29310/ws',
        'MULTICA_DEPLOYMENT_ID': '12345678-1234-4234-9234-123456789012',
    }
    envfile = fixture / 'source.env'
    envfile.write_text(''.join(key + '=' + shlex.quote(value) + '\n' for key, value in envvalues.items()))
    envfile.chmod(0o600)
    renderer = (ROOT / 'apps/desktop/out/renderer/index.html').read_bytes()
    proof = {
        'candidate_sha': inputs['candidate_sha'], 'database': 'credential_free_fixture',
        'source_environment_sha256': digest(envfile), 'distinct_from_complete_check_resources': True,
        'api': {'origin': envvalues['NEXT_PUBLIC_API_URL'], 'pid': 4242,
                'actual_startup_knowledge_flag': 'false', 'flag_observed_from_owned_process': True},
        'web': {'origin': envvalues['PLAYWRIGHT_BASE_URL'], 'mode': 'production',
                'rendered_api_base_url': envvalues['NEXT_PUBLIC_API_URL'],
                'rendered_ws_url': envvalues['NEXT_PUBLIC_WS_URL'], 'rendered_runtime_config_verified': True},
        'renderer': {'origin': envvalues['COLLABORATION_ELECTRON_RENDERER_URL'],
                     'ownership_verified': True, 'index_sha256': hashlib.sha256(renderer).hexdigest()},
    }
    prooffile = fixture / 'proof.json'
    save(prooffile, proof)
    binding_path = BINDING
    if mutation:
        changed = copy.deepcopy(json.loads(BINDING.read_text()))
        mutation(changed, fixture)
        binding_path = fixture / 'changed-browser-binding.json'
        save(binding_path, changed)
    permission = {
        'source_environment': str(envfile), 'source_environment_sha256': digest(envfile),
        'deployment_proof': str(prooffile), 'deployment_proof_sha256': digest(prooffile),
        'binary_binding': str(PRIVATE / 'binary-binding.json'),
        'binary_binding_sha256': digest(PRIVATE / 'binary-binding.json'),
        'DATABASE_URL': envvalues['DATABASE_URL'], 'database': 'credential_free_fixture',
        'browser_binding': {'path': str(binding_path), 'sha256': digest(binding_path)},
    }
    if missing_binding:
        permission.pop('browser_binding')
    save(fixture / 'fake-permit.json', permission)
    initial_env = {'PATH': '/usr/bin:/bin', 'HOME': str(home), 'TMPDIR': str(temp),
                   'DATABASE_URL': envvalues['DATABASE_URL'], 'CONTROL_CAPTURE': str(capture)}
    module = types.ModuleType('browser_env_recorder_control')
    module.__file__ = str(runner_path)
    exec(compile(Path(runner_path).read_bytes(), str(runner_path), 'exec'), module.__dict__)
    calls = []

    def response(request, timeout=None):
        url = request.full_url if hasattr(request, 'full_url') else request
        if hasattr(request, 'get_method') and request.get_method() == 'OPTIONS':
            return Response(b'', request.get_header('Origin'))
        if url.endswith('/health'):
            return Response(json.dumps({'commit': inputs['candidate_sha'][:9], 'pid': 4242}).encode())
        if url.endswith('/api/config'):
            return Response(json.dumps({'auth_mode': 'password', 'managed_installation_supported': True,
                                        'password_signup_available': True}).encode())
        return Response(renderer)

    def child(command, **kwargs):
        calls.append(command)
        assert command[:8] == ['bash', 'scripts/go-test-with-agent-cli-guard.sh', '--',
                               inputs['tools']['pnpm']['path'], 'exec', 'playwright', 'test',
                               'e2e/collaboration-knowledge-deployed.spec.ts']
        assert command[8:11] == ['--workers=1', '--retries=0', '--reporter=line,json']
        assert kwargs['env']['DATABASE_URL'] == envvalues['DATABASE_URL']
        return REAL_RUN([sys.executable, str(recorder), *command], **kwargs)

    argv = [str(runner_path), 'off', '--permit', str(fixture / 'fake-permit.json'), '--label', 'recorder-control']
    fake_audit = {'scope': 'upstream admission stubbed only for credential-free env/argv control'}
    error = None
    with patch.object(module, 'admission', return_value=(permission, inputs, fake_audit, output, initial_env)), \
            patch.object(module, 'urlopen', side_effect=response), \
            patch.object(module.subprocess, 'run', side_effect=child), \
            patch.object(sys, 'argv', argv), \
            patch.dict(os.environ, {'PLAYWRIGHT_BROWSERS_PATH': '/ambient-cache-must-not-be-used'}):
        try:
            code = module.main()
        except (RuntimeError, KeyError, OSError, ValueError) as failure:
            error = failure
            code = None
    captured = json.loads(capture.read_text()) if capture.exists() else None
    return code, captured, calls, error, str(fixture)


class BrowserChildEnvironment(unittest.TestCase):
    def test_reviewed_cache_reaches_real_recorder_child_without_ambient_home_or_credentials(self):
        code, captured, calls, error, fixture = exercise(ARGS.runner)
        self.assertIsNone(error)
        self.assertEqual(len(calls), 1)
        self.assertEqual(code, 0)
        self.assertEqual(captured['browser_path'], CACHE,
                         'The actual child process did not receive the reviewed browser cache')
        self.assertTrue(captured['home'].startswith(fixture))
        self.assertEqual(Path(captured['home']).stat().st_mode & 0o777, 0o700)
        self.assertTrue(captured['temp'].startswith('/private/tmp/multica-ccg-v16-browser-control-'))
        self.assertFalse(captured['model_credentials_present'])

    def test_missing_binding_refuses_before_child(self):
        _, captured, calls, error, _ = exercise(ARGS.runner, missing_binding=True)
        self.assertIsNotNone(error)
        self.assertIsNone(captured)
        self.assertEqual(calls, [])

    def test_changed_version_revision_owner_marker_or_executable_refuses_before_child(self):
        mutations = [
            lambda b, _: b.update(playwright_version='1.58.3'),
            lambda b, _: b.update(revision='1209'),
            lambda b, _: b['cache_root'].update(uid=-1),
            lambda b, _: b['installations']['chromium-headless-shell']['markers'].pop('INSTALLATION_COMPLETE'),
            lambda b, _: b['installations']['chromium-headless-shell']['executable'].update(sha256='0' * 64),
        ]
        for mutation in mutations:
            with self.subTest(mutation=mutations.index(mutation)):
                _, captured, calls, error, _ = exercise(ARGS.runner, mutation=mutation)
                self.assertIsNotNone(error)
                self.assertIsNone(captured)
                self.assertEqual(calls, [])


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('--runner', type=Path, required=True)
    parser.add_argument('--phase', choices=('red', 'green'), required=True)
    ARGS = parser.parse_args()
    names = ['test_reviewed_cache_reaches_real_recorder_child_without_ambient_home_or_credentials']
    if ARGS.phase == 'green':
        names += ['test_missing_binding_refuses_before_child',
                  'test_changed_version_revision_owner_marker_or_executable_refuses_before_child']
    suite = unittest.TestSuite(BrowserChildEnvironment(name) for name in names)
    result = unittest.TextTestRunner(verbosity=2).run(suite)
    raise SystemExit(0 if result.wasSuccessful() else 1)
