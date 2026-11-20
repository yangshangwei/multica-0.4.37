# Synthetic workspace read fixture

This repository contains only local, fictional data. There is no server, network
client, credential, customer data, real advisory or deployment.

`workspace_reader.read_issue` accepts a caller workspace and in-memory rows. A
read belonging to another workspace must return `None` before adding an audit
entry. The supplied baseline tests intentionally omit that missing boundary
case. An independent vulnerability verifier must author the regression test
after G1; the patch author must leave that test unchanged.

Run the local baseline with `python3 -m unittest discover -s tests -v`.
