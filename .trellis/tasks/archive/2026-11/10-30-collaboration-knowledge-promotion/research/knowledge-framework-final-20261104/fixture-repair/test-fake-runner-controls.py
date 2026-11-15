import argparse
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import types
import unittest
from unittest.mock import patch

ROOT = Path('/Volumes/artisan/code/2026/mc-knowledge-framework-20261103')
PRIVATE = ROOT / '.omx/private/knowledge-framework-final-20261104'
RUNNER = PRIVATE / 'run-fake-knowledge-v16.py'
REAL_RUN = subprocess.run


def load():
    module = types.ModuleType('private_fake_runner_control')
    module.__file__ = str(RUNNER)
    exec(compile(RUNNER.read_bytes(), str(RUNNER), 'exec'), module.__dict__)
    return module


def save(path, value):
    descriptor = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(descriptor, 'w') as target:
        json.dump(value, target)


RECORDER = '''import json,os,sys
from pathlib import Path
names=json.loads(os.environ["CONTROL_TEST_NAMES"])
for name in names:
    print(json.dumps({"Action":"run","Test":name}))
    print(json.dumps({"Action":"pass","Test":name}))
capture={"argv":sys.argv[1:],"HOME":os.environ["HOME"],"TMPDIR":os.environ["TMPDIR"],"DATABASE_URL":os.environ["DATABASE_URL"],"RK1K_FAKE_PROOF_OUT":os.environ["RK1K_FAKE_PROOF_OUT"],"real_flags_or_keys":any(os.environ.get(k) for k in ("MULTICA_RUN_REAL_AGENT_SMOKE","MULTICA_LLM_API_KEY","ANTHROPIC_API_KEY","OPENAI_API_KEY")),"scope":"credential-free recorder; no Go/TestMain/DB/provider body"}
Path(os.environ["CONTROL_CAPTURE"]).write_text(json.dumps(capture))
out=Path(os.environ["RK1K_FAKE_PROOF_OUT"]);out.mkdir(mode=0o700)
if os.environ.get("CONTROL_BAD_SUMMARY"):
    (out/"summary.json").write_text("{invalid")
else:
    samples=[]
    for seed,count in ((False,15),(True,3)):
        for i in range(count):samples.append({"label":"recorder-only","state":"finished","verdict":"pass","launches":[{"seed":seed,"fake_starts":1,"paid_starts":0}]})
    (out/"summary.json").write_text(json.dumps({"schema_version":1,"kind":"fake_framework_proof","measured_fake_starts":15,"fake_seed_starts":3,"paid_starts":0,"knowledge_allocated":0,"real_runnable":False,"samples":samples}))
'''


def exercise(lane, bad_summary=False):
    module = load()
    data = Path(tempfile.mkdtemp(prefix='fake-runner-control-', dir=PRIVATE))
    output = data / 'output'
    temp = Path(tempfile.mkdtemp(prefix='multica-rk1k-control-', dir='/private/tmp'))
    home = temp / 'home'
    home.mkdir(mode=0o700)
    recorder = data / 'recorder.py'
    recorder.write_text(RECORDER)
    capture = data / 'child-env.json'
    permit = data / 'fake-permit.json'
    save(permit, {'scope': 'recorder-only'})
    binding = json.loads((PRIVATE / 'input-binding.json').read_text())
    env = {'PATH': '/usr/bin:/bin', 'HOME': str(home), 'TMPDIR': str(temp),
           'DATABASE_URL': 'postgres://127.0.0.1:1/credential_free_control',
           'RK1K_FAKE_PROOF_OUT': str(output / 'proof'), 'CONTROL_CAPTURE': str(capture),
           'CONTROL_TEST_NAMES': json.dumps(module.TESTS), 'MULTICA_RUN_REAL_AGENT_SMOKE': '',
           'MULTICA_LLM_API_KEY': '', 'ANTHROPIC_API_KEY': '', 'OPENAI_API_KEY': ''}
    if bad_summary:
        env['CONTROL_BAD_SUMMARY'] = '1'
    calls = []

    def record(command, **kwargs):
        calls.append(command)
        return REAL_RUN([sys.executable, str(recorder), *command], **kwargs)

    error = None
    with patch.object(module, 'verify_sources', return_value=True), \
            patch.object(module.subprocess, 'run', side_effect=record):
        try:
            result = module.execute(lane, binding, {}, env, output, permit)
        except Exception as failure:
            error = failure
            result = None
    return module, result, json.loads(capture.read_text()), calls, error, output


class FakeRunnerControls(unittest.TestCase):
    def test_normal_and_race_use_exact_command_and_private_credential_free_child(self):
        for lane in ('normal', 'race'):
            with self.subTest(lane=lane):
                module, result, captured, calls, error, _ = exercise(lane)
                self.assertIsNone(error)
                self.assertTrue(result['accepted'])
                self.assertEqual(len(calls), 1)
                self.assertEqual(captured['argv'], calls[0])
                self.assertEqual(captured['argv'][4:8], ['-C', 'server', 'test', './cmd/server'])
                self.assertIn(module.FILTER, captured['argv'])
                for flag in ('-tags=agentintegration', '-p=1', '-parallel=1', '-count=1', '-json', '-timeout=10m'):
                    self.assertIn(flag, captured['argv'])
                self.assertEqual('-race' in captured['argv'], lane == 'race')
                self.assertTrue(captured['HOME'].startswith('/private/tmp/'))
                self.assertFalse(captured['real_flags_or_keys'])
                self.assertEqual(result['physical_totals'], {'measured_fake_starts': 15, 'fake_seed_starts': 3, 'paid_starts': 0})

    def test_false_permission_or_changed_binding_refuses_before_go_or_sql(self):
        for ready in (False, True):
            with self.subTest(ready=ready):
                module = load()
                private = Path(tempfile.mkdtemp(prefix='admission-control-', dir=PRIVATE))
                (private / 'permissions').mkdir(mode=0o700)
                permit = private / 'permissions/rk1k-normal-v16-01.json'
                record = private / 'root-record.json'
                save(record, {'scope': 'fake-only control'})
                (private / 'input-binding.json').write_text('{}')
                save(permit, {'ready_to_execute': ready, 'root_authorized_fake_suite': ready,
                             'real_provider_allowed': False, 'lane': 'normal', 'runner_sha256': module.digest(RUNNER),
                             'root_permission_record': {'path': str(record), 'sha256': module.digest(record)},
                             'input_binding_sha256': '0' * 64})
                with patch.object(module, 'PRIVATE', private), \
                        patch.object(module, 'verify_sources', side_effect=AssertionError('No source discovery expected')), \
                        patch.object(module.subprocess, 'run', side_effect=AssertionError('No SQL or Go command expected')):
                    with self.assertRaises(RuntimeError):
                        module.admission('normal', permit)

    def test_invalid_durable_summary_preserves_failed_terminal_result(self):
        _, result, _, calls, error, output = exercise('normal', bad_summary=True)
        self.assertIsNone(error, 'Malformed summary escaped without a terminal result')
        self.assertEqual(len(calls), 1)
        self.assertFalse(result['accepted'])
        self.assertFalse(result['summary_complete'])
        self.assertTrue((output / 'result.json').is_file())


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('--phase', choices=('red', 'green'), required=True)
    args = parser.parse_args()
    if args.phase == 'red':
        suite = unittest.TestSuite([FakeRunnerControls('test_invalid_durable_summary_preserves_failed_terminal_result')])
    else:
        suite = unittest.defaultTestLoader.loadTestsFromTestCase(FakeRunnerControls)
    result = unittest.TextTestRunner(verbosity=2).run(suite)
    raise SystemExit(0 if result.wasSuccessful() else 1)
