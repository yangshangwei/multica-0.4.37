import json, os, runpy, tempfile
from pathlib import Path
from unittest.mock import patch
launcher=Path(__file__).resolve().parent.parent/'run_rk1.py'
fn=runpy.run_path(str(launcher))['operator_facts']
with tempfile.TemporaryDirectory(prefix='rk1-independent-config-') as temp:
 home=Path(temp); (home/'.claude').mkdir(); (home/'.claude/settings.json').write_text('{}')
 override=home/'alternate-config'; override.mkdir(); (override/'settings.json').write_text('{"model":"different-model","bashOutputMaxChars":1000}')
 with patch.dict(os.environ,{'HOME':str(home)},clear=True):
  before=fn()
  os.environ.update(CLAUDE_CONFIG_DIR=str(override),ANTHROPIC_BASE_URL='https://different-provider.invalid',CLAUDE_CODE_USE_BEDROCK='1',AWS_PROFILE='different-profile')
  after=fn()
 print(json.dumps({'fingerprint_unchanged':before==after,'config_dir_contents_fingerprinted':any('alternate-config' in key for key in after),'before':before,'after':after},indent=2))
 assert before==after
