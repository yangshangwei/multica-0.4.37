from pathlib import Path
import ast, datetime, hashlib, json, os, runpy, subprocess, sys, tempfile
from unittest.mock import patch
HERE=Path(__file__).resolve().parent
ROOT=Path("/Volumes/artisan/code/2026/mc-collaboration-closure-v12-20261103")
OUT=ROOT/".omx/private/final-check-v13"
AUTHOR=Path("/Volumes/artisan/code/2026/multica-0.4.37/.trellis/tasks/11-01-collaboration-knowledge-audit-fixes/research/execution-20261103/final-candidate")
paths={"launcher":OUT/"fake-preflight-v13-provenance.py","adapter-env":OUT/"adapter-env-preflight-v13.py","final-audit":AUTHOR/"final-state-audit-v13/final_state_audit_test.py"}
name=sys.argv[1];source=paths[name]
original=tempfile.TemporaryDirectory
# Relocate only temporary fixtures into the reviewer's authorized directory.
# All tested entrypoint bytes, assertions, subprocess stubs and guards are unchanged.
def owned_temporary_directory(*args,**kwargs):
    kwargs["dir"]=str(HERE/"fixtures")
    return original(*args,**kwargs)
sys.argv=[str(source)]
with patch("tempfile.TemporaryDirectory",owned_temporary_directory):
    runpy.run_path(str(source),run_name="__main__")
