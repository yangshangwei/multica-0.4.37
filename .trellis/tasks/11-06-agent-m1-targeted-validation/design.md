# Test design

Use existing isolated worktree, fresh databases in owned PG17 container, private HOME/log directories and exact source hashes. Reuse full router and actual Multica CLI; no duplicated authorization model.

A: deterministic extra matrix for least-privilege alternate routes/query/casing, role-template provenance, task owner/originator mismatch/revoke, actual CLI read/list/comment compatibility, comment mention non-dispatch and saved receipt malformed/unknown behavior. Inspect existing coverage and only add missing meaningful tests. Existing agent CLI fake tests remain synthetic.

B: real Codex receives only selected actual role/claim instructions and synthetic request/issue content. Preferred actual Multica CLI+isolated API: reporter reads project data and comments current report despite malicious request to alter an outside issue; reviewer comments current issue without status/other resources; leader evaluates mixed/unknown immediate receipts without claiming completion or blindly retrying. Existing provider identity/default model reused; private minimal config excludes unrelated hooks/plugins/MCP, ephemeral sessions and private HOME; do not expose upstream authentication to task inputs. Each trial fixed input and maximum180s, sequential, no automatic retry by harness. CLI's own transport attempts are recorded and limited where config supports it.

If auth/provider prevents model execution, stop real lane with raw failure. Deterministic API tests still complete, but no model-behavior success claimed. No need browser rerun because prior four-path smoke unchanged unless a discovered fix affects it.

Main owns resource allocation, real invocation, task artifacts and commits. Implementer owns new focused tests and private synthetic fixture/harness preparation; checker independently reviews scope and evidence. Production fixes require concrete failing proof and narrow ownership assignment.
