#!/usr/bin/env python3
"""Probe the selected pure parser against a test-owned Make include/export."""
import ast
import hashlib
import json
from pathlib import Path
import re
import shlex
import subprocess
import tempfile

LAUNCHER = Path('/Volumes/artisan/code/2026/mc-collaboration-final4-20261102/.omx/private/final-candidate/run-make-check-v6.py')


def main():
    raw = LAUNCHER.read_bytes()
    tree = ast.parse(raw)
    scope = {'re': re, 'shlex': shlex}
    functions = [node for node in tree.body if isinstance(node, ast.FunctionDef)
                 and node.name in ('require', 'parse_generated_environment')]
    exec(compile(ast.Module(body=functions, type_ignores=[]), str(LAUNCHER), 'exec'), scope)
    with tempfile.TemporaryDirectory(prefix='multica-selected-v6-Make-proof-') as temporary:
        root = Path(temporary)
        marker = root / 'only-test-marker'
        fixture = 'JWT_SECRET=' + shlex.quote('${shell printf touched > ' + str(marker) + '}') + '\n'
        accepted = True
        try:
            values = scope['parse_generated_environment'](fixture)
        except RuntimeError:
            accepted = False
            values = {}
        canonical = ''.join(key + '=' + shlex.quote(value) + '\n' for key, value in values.items())
        environment = root / 'canonical.env'
        environment.write_text(canonical)
        makefile = root / 'ProbeMakefile'
        makefile.write_text('include ' + str(environment) + '\nexport\n.PHONY: probe\nprobe:\n\t@/usr/bin/true\n')
        result = subprocess.run(['/usr/bin/make', '--no-print-directory', '-f', str(makefile), 'probe'],
                                cwd=root, env={'PATH': '/usr/bin:/bin', 'HOME': temporary},
                                capture_output=True, text=True)
        print(json.dumps({'launcher_sha256': hashlib.sha256(raw).hexdigest(),
                          'brace_shell_value_accepted': accepted, 'make_exit_code': result.returncode,
                          'owned_marker_created': marker.exists(), 'actual_launcher_executed': False,
                          'actual_source_executed': False, 'database_operations': 0, 'network_operations': 0}))
        return 1 if marker.exists() else 0


if __name__ == '__main__':
    raise SystemExit(main())
