#!/usr/bin/env python3
"""Create a transparent, hash-recorded test-only backend substitution."""
import argparse
import hashlib
import json
from pathlib import Path

p=argparse.ArgumentParser(description=__doc__)
p.add_argument('--repo',required=True,type=Path)
p.add_argument('--out',required=True,type=Path)
a=p.parse_args()
repo=a.repo.resolve(strict=True);out=a.out.resolve();out.mkdir(parents=True,exist_ok=True)
if repo==out or repo in out.parents:raise SystemExit('Generated overlay must remain outside repository')
scripts=repo/'.trellis/tasks/11-18-security-remediation-v2-real-model-acceptance/scripts'
original=repo/'server/pkg/agent/builtin_runtimes.go'
text=original.read_text()
needle='func ResolveBackend(provider string, cfg Config) (Backend, error) {\n'
assert text.count(needle)==1
replacement=needle+'\tif provider == "codex" && securityDirectCLIEnabled() {\n\t\treturn newSecurityDirectCLIBackend(cfg)\n\t}\n'
modified=text.replace(needle,replacement)
copy=out/'builtin_runtimes.go';copy.write_text(modified)
files={str(original):str(copy)}
for name,virtual in [
 ('direct_cli_backend.go','server/pkg/agent/zz_secrem_direct_cli_backend.go'),
 ('direct_cli_backend_test.go','server/pkg/agent/zz_secrem_direct_cli_backend_test.go'),
 ('real_platform_harness_test.go','server/cmd/server/zz_secrem_real_harness_test.go'),
 ('direct_cli_harness_test.go','server/cmd/server/zz_secrem_direct_cli_harness_test.go'),
]:
 source=scripts/name
 if not source.is_file():raise SystemExit(f'Missing task source {name}')
 files[str(repo/virtual)]=str(source)
(out/'overlay.json').write_text(json.dumps({'Replace':files},indent=2)+'\n')
sha=lambda raw:hashlib.sha256(raw).hexdigest()
(out/'overlay-provenance.json').write_text(json.dumps({'evidence_mode':'official-cli-direct','original_path':str(original),'original_sha256':sha(original.read_bytes()),'overlay_sha256':sha(copy.read_bytes()),'change':{'before':needle,'after':replacement},'overlay_file_sha256':sha((out/'overlay.json').read_bytes()),'sources':{v:sha(Path(v).read_bytes()) for v in files.values()}},indent=2)+'\n')
print(out/'overlay.json')
