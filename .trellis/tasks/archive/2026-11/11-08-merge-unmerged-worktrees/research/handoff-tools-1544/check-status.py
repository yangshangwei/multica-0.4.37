from pathlib import Path
import json, re, datetime, sys, subprocess
p=Path('/Volumes/artisan/code/2026/mc-merge-takeover-archives-20261108/verification')/sys.argv[1]
s=json.loads((p/'state.json').read_text());lines=(p/'make-check.log').read_text(errors='replace').splitlines()
fail=[(i+1,l) for i,l in enumerate(lines) if re.search(r"^(--- FAIL:|FAIL(?:\s|$)|WARNING: DATA RACE|panic:|fatal error:|make: \*\*\*)",l) or re.search(r"Tests\s+\d+ failed|Error:.*(?:failed|timeout)|^\s*[✘×]\s+\d+|^\s*\d+ failed|^\s*\d+\) \[chromium\]",l)]
stages=[l for l in lines if '/6]' in l]; passed=[l for l in lines if re.search(r'Tests\s+.*passed|Test Files\s+.*passed|^ok\s',l)]
r={'time':datetime.datetime.now().astimezone().isoformat(),'run':s['run_id'],'exit_code':s.get('exit_code'),'stage':stages[-1] if stages else None,'log_lines':len(lines),'failure_markers':len(fail),'latest_failures':fail[-3:],'last_pass':passed[-2:],'tail':lines[-3:]}
print(json.dumps(r,ensure_ascii=False))
