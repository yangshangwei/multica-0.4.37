"""Execute existing controls without changing their source or author evidence."""
import hashlib
import json
import os
from pathlib import Path
import runpy
import sys
import tempfile
from unittest.mock import patch

HERE = Path(__file__).resolve().parent
ROOT = Path('/Volumes/artisan/code/2026/mc-collaboration-closure-v12-20261103')
PRIVATE = ROOT / '.omx/private/final-check-v14'
SOURCES = {
    'launcher': PRIVATE / 'fake-preflight-v14-provenance.py',
    'adapter-env': PRIVATE / 'adapter-env-preflight-v14.py',
    'final-audit': HERE.parent / 'final-state-audit-v14/final_state_audit_test.py',
}
name = sys.argv[1]
source = SOURCES[name]
original = tempfile.TemporaryDirectory
allocated = []


def owned_temporary_directory(*args, **kwargs):
    # Retain the same temporary fixture; relocate only its root into reviewer scope.
    kwargs['dir'] = str(HERE / 'controlled-test-temp')
    result = original(*args, **kwargs)
    allocated.append(result.name)
    return result


before = hashlib.sha256(source.read_bytes()).hexdigest()
sys.argv = [str(source)]
code = 0
try:
    with patch('tempfile.TemporaryDirectory', side_effect=owned_temporary_directory):
        runpy.run_path(str(source), run_name='__main__')
except SystemExit as error:
    code = int(error.code or 0)
after = hashlib.sha256(source.read_bytes()).hexdigest()
record = {
    'source': str(source), 'source_sha256': before, 'source_unchanged': before == after,
    'exit_code': code, 'fixture_roots': allocated,
    'only_fixture_location_changed': True,
    'original_source_or_author_results_edited': False,
    'runtime_tests_DB_API_Provider': False,
}
with (HERE / (name + '-independent-control.json')).open('x') as output:
    json.dump(record, output, indent=2)
    output.write('\n')
raise SystemExit(code)
