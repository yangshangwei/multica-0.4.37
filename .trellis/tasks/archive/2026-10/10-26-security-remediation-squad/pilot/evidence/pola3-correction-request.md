Independent contract check found the handover fields are complete but three values are outside the pilot enum contract. Please add a corrected follow-up comment under this thread, preserving the original simulation note:

- exploitability: unknown
- evidence_state: unknown
- production_impact: no

Keep `severity: high`, route to `incident/release` with human Operator approval, and retain the blocked actions and next owner. Explain that “simulated” describes the synthetic exercise context, not an enum value. Do not run production commands, read credentials, probe networks, dispatch incident/release squads, or mark the issue done. Leave the issue `in_review`.
