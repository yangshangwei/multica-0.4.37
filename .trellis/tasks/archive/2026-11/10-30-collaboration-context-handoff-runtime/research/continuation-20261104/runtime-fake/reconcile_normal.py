#!/usr/bin/env python3
"""Recount preserved normal proof files without invoking the test process."""
from collections import Counter
import hashlib
import json
import os
from pathlib import Path

PUBLIC = Path(__file__).parent
RUN = Path('/Volumes/artisan/code/2026/mc-runtime-acceptance-20261104/.omx/private/runtime-fake-20261104/normal')


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def main():
    os.umask(0o077)
    original = json.loads((RUN / 'result.json').read_text())
    assert sha(RUN / 'tests.jsonl') == original['tests_sha256']
    assert sha(RUN / 'stderr.log') == original['stderr_sha256']
    assert sha(PUBLIC / 'run_fake.py') == original['runner_sha256']
    assert original['counts'] == {'run': 62, 'pass': 62}
    assert original['exit_code'] == 0 and original['source_stable'] is True
    assert original['invalid_json_lines'] == 0 and not original['guard_violation']
    assert len(original['top_pass']) == 9 and all(n == 1 for n in original['top_pass'].values())
    assert original['database_identity'] == original['database_identity_after']
    paths = sorted((RUN / 'proof').glob('*/RK1-*/public.json'))
    assert len(paths) == 22
    trials = []
    physical = 0
    cases = set()
    for path in paths:
        trial = json.loads(path.read_text())
        case = path.parent.parent.name
        cases.add(case)
        assert trial['label'] == path.parent.name
        assert trial['mode'] in ('fake', 'sim')
        physical += sum(run['physical_launches'] for run in trial['runs'])
        trials.append({'case': case, 'file': str(path), 'sha256': sha(path),
                       'scenario': trial['scenario'], 'trial': trial['trial'],
                       'status': trial['status'], 'verdict': trial['verdict'], 'mode': trial['mode']})
    assert len(cases) == 19 and physical == 32
    result = dict(original)
    result.update({'accepted': True, 'public_trial_count': len(trials), 'public_trials': trials,
                   'fake_proof_physical_launches': physical, 'fixture_cases': len(cases),
                   'reconciliation': {
                       'cause': 'Original collector used flat JSON paths; framework publishes case/label/public.json.',
                       'original_result_sha256': sha(RUN / 'result.json'),
                       'original_runner_sha256': sha(PUBLIC / 'run_fake.py'),
                       'script_sha256': sha(Path(__file__)),
                       'test_process_reexecuted': False,
                       'original_result_retained': True,
                   }})
    for path in [RUN / 'reconciled-result.json', PUBLIC / 'normal-reconciled-result.json']:
        with path.open('x') as out:
            json.dump(result, out, ensure_ascii=False, indent=2)
            out.write('\n')
    print(json.dumps({'normal_test_exit': 0, 'named_pass': 62, 'public_trials': 22,
                      'fixture_cases': 19, 'fake_proof_physical_launches': 32, 'accepted': True}))


if __name__ == '__main__':
    main()
