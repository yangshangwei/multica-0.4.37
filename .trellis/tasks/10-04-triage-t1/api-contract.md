# Triage T1 API contract

Implementation owners must coordinate changes to this contract before changing either side. Dates use YYYY-MM-DD; snooze/time fields use ISO8601 instants. All new collections are arrays, never null. Existing IssueResponse is reused and gains admission_status.

## DTOs

`Settings`: `{supported:true, enabled:boolean, acceptance_status:string, require_priority:boolean, responsibility_mode:"none"|"notify"|"assign", responsibility_member_id:string|null, revision:number}`.

All actor/reviewer/responsibility IDs are **user UUIDs**, not member-row UUIDs. Server validates their active membership in the selected workspace. The existing responsibility_member_id wire name denotes the member's user identity.

`Fields`: `{title?, description?, status?, priority?, project_id?:string|null, assignee_type?:string|null, assignee_id?:string|null, label_ids?:string[], start_date?:string|null, due_date?:string|null}`. Acceptance may change all listed fields except title/description if existing edit is preferred; those fields remain editable through issue detail. Null explicitly clears; absent retains existing/candidate value. status must be backlog/todo category. Candidate assignee supports member/agent/squad as existing rules allow.

`Item`: `{issue:IssueResponse, candidate_project_id:string|null, candidate_assignee_type:string|null, candidate_assignee_id:string|null, reviewer_id:string|null, reviewer_valid:boolean, round:number, first_entered_at:string, entered_at:string, snoozed_until:string|null, duplicate_issue_id:string|null, duplicate_identifier:string|null, source:"manual"|"csv", source_url:string|null, external_id:string|null, batch_id:string|null, filename:string|null, row_number:number|null}`. Concurrency token is item.issue.revision. Historical deleted targets keep duplicate_identifier; never disclose foreign target information.

`Action`: `{id,issue_id,actor_id,action,round,reason:string|null,before:object,after:object,created_at,execution_status:"not_requested"|"pending"|"queued"|"failed",task_id:string|null,execution_error:string|null}`.

`ActionResult`: `{item:Item,action:Action}`.

`ActionInput`: `{request_id:string,expected_revision:number,action:"accept"|"accept_and_execute"|"reject"|"duplicate"|"snooze"|"unsnooze"|"reopen"|"assign_reviewer",reason?:string,duplicate_issue_id?:string,snoozed_until?:string,reviewer_id?:string|null,fields?:Fields}`. Reasons 1–2000 nonblank characters for reject/reopen; snooze future <=90 days. Request IDs are UUIDs. Duplicate may receive a resolved issue UUID/identifier; pasted links are parsed by the client and scoped resolution happens server-side.

## Endpoints

- GET `/triage/settings` -> Settings; PUT same with settings fields and expected_revision -> Settings (human owner/admin).
- GET `/triage/items` -> `{items:Item[],total:number,counts:{pending:number,ready:number,snoozed:number},limit:number,offset:number}`. Query view=ready|all|snoozed|history (default ready), q, source, priority, project_id, label_id, reviewer_id, creator_id, entered_after, entered_before, result, processed_by, processed_after, processed_before, sort=oldest|newest|priority, limit (default50,max100), offset. Counts ignore user filters.
- POST `/triage/items` with `{request_id,title,description?,priority?,candidate_project_id?,candidate_assignee_type?,candidate_assignee_id?,label_ids?,start_date?,due_date?,attachment_ids?,source_url?}` -> Item. Source is server-derived manual; intake fields cannot set formal state.
- GET `/triage/items/{id}` -> Item; GET `/triage/items/{id}/history` -> `{events:Action[]}`.
- GET `/triage/history` -> `{entries:HistoryEntry[],total:number,limit:number,offset:number}`. Filters q, result, processed_by, processed_after, processed_before, source and pagination. Includes old rounds after reopen and CSV batch summaries even when no item is currently finalized. A history query evaluates immutable action/source snapshots rather than only the current Issue state.
- POST `/triage/items/{id}/actions` with ActionInput -> ActionResult.
- POST `/triage/actions/{actionId}/retry-execution` with `{}` -> ActionResult. Same action/task identity for every retry; only original human actor with current permission may resume.
- POST `/triage/batch/preview` with `{items:[{issue_id,expected_revision}],action,reason?,snoozed_until?,reviewer_id?,fields?}` -> `{items:[{issue_id,expected_revision,valid:boolean,error:string|null}],valid_count:number}`. Allowed action accept/reject/snooze/assign_reviewer.
- POST `/triage/batch` with `{items:[{issue_id,...ActionInput}]}` -> `{results:[{issue_id,status:"success"|"conflict"|"invalid"|"forbidden"|"failed",result?:ActionResult,error?:string}],success_count:number}`. Preserve each row request_id for retry. Server rejects any action outside accept/reject/snooze/assign_reviewer here as well as in preview.
- POST `/triage/imports/preview` with `{request_id,filename,csv:string,mapping?:Record<string,string>}` -> ImportPreview. Mapping uses header names -> canonical fields (`title`, `description`, `priority`, `labels`, `project`, `assignee`, `start_date`, `due_date`, `source_url`, `external_id`, `ignore`). Unmapped state/iteration columns warn explicitly. New mapping uses a new request_id. Strict server UTF8 validation also rejects replacement-character client decoding; browser uses TextDecoder(fatal:true).
- GET `/triage/imports/{id}` -> ImportPreview/current row outcomes.
- POST `/triage/imports/{id}/commit` with `{rows:[{row_number:number,import_duplicate:boolean}]}` -> ImportResult. Valid selected warning rows are confirmed by inclusion; invalid rows never create. Unselected rows are skipped for this request, not irreversibly erased.
- GET `/triage/imports/{id}/failures` -> text/csv attachment.

`ImportPreview`: `{batch_id,filename,headers:string[],mapping:Record<string,string>,rows:ImportRow[],counts:{valid:number,warning:number,error:number,duplicate:number},limits:{max_rows:number,max_bytes:number}}`.

`ImportRow`: `{row_number:number,values:Record<string,string>,warnings:string[],errors:string[],duplicate:boolean,duplicate_issue_id:string|null,similar_issue_ids:string[],status:"ready"|"created"|"skipped"|"failed",issue_id:string|null,error:string|null}`. Row number is data row index starting1. CSV provenance may additionally expose physical source line if parser needs it.

`ImportResult`: `{batch_id,results:ImportRow[],created:number,skipped:number,failed:number}`. Retry returns previously created rows as created, never duplicates them; UI may refresh full batch after partial response.

`HistoryEntry`: `{id:string,kind:"action"|"import",issue_id:string|null,identifier:string|null,title:string,action:string,actor_id:string,created_at:string,reason:string|null,before:object,after:object,batch_id:string|null,filename:string|null,counts:{created:number,skipped:number,failed:number}|null}`. Titles/identifiers are decision-time snapshots; links resolve live scoped resources. Batch entries use durable batch identity and current cumulative result counts; action entries remain immutable.

Realtime: `triage:updated` payload `{workspace_id,issue_id?:string,batch_id?:string,settings_changed?:boolean}` after committed writes. Ordinary accepted Issue DTO changes also need safe cache invalidation, without execution-triggering plugin events. Existing direct IssueResponse + websocket mappers must both expose admission_status.

Deleted-result replay: consumed manual/action request keys and successful CSV row keys survive issue deletion. A replay returns HTTP409 with a clear deleted-result message and stable issue ID; it never recreates that issue. Import results may retain issue_id for a deleted task and must show it as unavailable if opened.
