import json,re
from pathlib import Path
root=Path(__file__).resolve().parents[6]
a=(root/'server/cmd/server/rk1_scenarios_handoff_test.go').read_text()
def regex(name):
 text=re.search(r'var '+name+r' = regexp.MustCompile\(`(.*?)`\)',a,re.S).group(1)
 return re.compile(text)
id='11111111-1111-4111-8111-111111111111'
answer='EXTRA UNREGISTERED ANSWER\nPRIOR_RUN '+id+' RECORD INCOMPLETE RESULT ABSENT\nRESULT_TEXT NONE RECORDED\nEXTRA TRAILING TEXT'
one=regex('rk1PriorLine').search(answer);two=regex('rk1PriorText').search(answer)
squad='EXTRA UNREGISTERED ANSWER\nMEMBER_RUN '+id+'\nMEMBER_CODE MEMBER-ABCDEF\nWOKEN_BY proof '+id
ok1=bool(one and two and one[1]==id and one[2]=='INCOMPLETE' and one[3]=='ABSENT' and two[1].strip()=='NONE RECORDED')
ok2=bool(regex('rk1SquadAnswer').search(squad))
print(json.dumps({'H1_extra_lines_pass_format':ok1,'H2_extra_leading_line_pass_format':ok2,'H3_scope_note':'registered code-set scoring may deliberately ignore prose; only H1/H2 exact-line claim is demonstrated'},indent=2))
assert ok1 and ok2
