#!/usr/bin/env python3
"""Review and conditionally apply Chinese agent and squad instructions.

Uses only the standard library and the normal authenticated API. Creating a plan
and applying it are separate operations; writes require --write and a new journal.
"""

import argparse
from collections import Counter
from contextlib import nullcontext
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import sys
from urllib.error import HTTPError
from urllib.parse import urlsplit
from urllib.request import HTTPRedirectHandler, Request, build_opener
from uuid import UUID


CAPABILITY = "X-Multica-Instructions-Precondition"
MAX_JSON_BYTES = 64 * 1024 * 1024


class APIError(Exception):
    def __init__(self, status):
        self.status = status
        super().__init__(f"API returned HTTP {status}")


class NoRedirects(HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        # A profile's credential must never follow an API redirect elsewhere.
        return None


class API:
    def __init__(self, config, workspace_id):
        self.server_url = config.get("server_url", "").rstrip("/")
        url = urlsplit(self.server_url)
        if (url.scheme not in ("http", "https") or not url.hostname or
                url.username or url.password or url.query or url.fragment):
            raise ValueError("config must contain an HTTP(S) server_url without credentials or query")
        self.token = config.get("token")
        if not isinstance(self.token, str) or not self.token:
            raise ValueError("config does not contain an authenticated token")
        self.workspace_id = valid_uuid(workspace_id)
        self.opener = build_opener(NoRedirects)

    def request(self, method, path, body=None):
        data = None if body is None else json.dumps(body, ensure_ascii=False).encode("utf-8")
        request = Request(self.server_url + path, data=data, method=method, headers={
            "Authorization": f"Bearer {self.token}",
            "X-Workspace-ID": self.workspace_id,
            "Content-Type": "application/json; charset=utf-8",
        })
        try:
            with self.opener.open(request, timeout=30) as response:
                raw = response.read(MAX_JSON_BYTES + 1)
                if len(raw) > MAX_JSON_BYTES:
                    raise ValueError("API response exceeds the supported size")
                return json.loads(raw), dict(response.headers)
        except HTTPError as error:
            # Do not echo response bodies, saved instructions, or credentials.
            raise APIError(error.code) from None


def digest(value):
    return hashlib.sha256(value.encode("utf-8")).hexdigest()


def valid_uuid(value):
    if not isinstance(value, str):
        raise ValueError("workspace and resource IDs must be UUID strings")
    return str(UUID(value))


def valid_timestamp(value):
    if not isinstance(value, str):
        raise ValueError("updated_at must be an RFC3339 timestamp")
    parsed = datetime.fromisoformat(value.replace("Z", "+00:00"))
    if parsed.tzinfo is None:
        raise ValueError("updated_at must include its timezone")
    # Validate without reformatting: PostgreSQL microseconds must survive.
    return value


def validate_row(record, workspace_id, resource_id=None):
    if not isinstance(record, dict):
        raise ValueError("API resource must be an object")
    if record.get("workspace_id") != workspace_id:
        raise ValueError("API resource belongs to a different workspace")
    valid_uuid(record.get("id"))
    if resource_id is not None and record["id"] != resource_id:
        raise ValueError("API returned a different resource")
    if not isinstance(record.get("instructions"), str):
        raise ValueError("API resource instructions must be a string")
    valid_timestamp(record.get("updated_at"))


def build_plan(api, workspace_id, catalog):
    workspace_id = valid_uuid(workspace_id)
    workspace, _ = api.request("GET", f"/api/workspaces/{workspace_id}")
    if not isinstance(workspace, dict) or workspace.get("id") != workspace_id:
        raise ValueError("workspace response does not match the requested workspace")
    entries = []
    for kind in ("agent", "squad"):
        records, _ = api.request("GET", f"/api/{kind}s")
        if not isinstance(records, list):
            raise ValueError("expected the complete resource list, not a paginated or malformed response")
        for record in records:
            validate_row(record, workspace_id)
            before = record["instructions"]
            key, version = record.get("template_key"), record.get("template_version")
            template = catalog.get((kind, key))
            after, reason = None, "custom_or_unknown"
            if record.get("system_key") == "mika":
                reason = "mika_workspace_notes"
            elif template:
                if before == template["target"]:
                    reason = "already_chinese_default"
                elif (type(version) is int and version == template["source_version"] ==
                      template["target_version"] and before == template["source"]):
                    after, reason = template["target"], "known_unchanged_default"
            entries.append({
                "kind": kind, "id": record["id"], "name": record.get("name", ""),
                "template_key": key, "template_version": version,
                "before": before, "before_sha256": digest(before),
                "before_updated_at": record["updated_at"], "after": after,
                "selected": after is not None and after != before, "reason": reason,
            })
    return {
        "version": 1, "server_url": api.server_url, "workspace_id": workspace_id,
        "workspace_name": workspace.get("name", ""),
        "created_at": datetime.now(timezone.utc).isoformat(), "entries": entries,
    }


def validate_plan(plan, server_url):
    if not isinstance(plan, dict) or plan.get("version") != 1:
        raise ValueError("unsupported plan version")
    if plan.get("server_url") != server_url:
        raise ValueError("plan belongs to a different server")
    valid_uuid(plan.get("workspace_id"))
    if not isinstance(plan.get("entries"), list):
        raise ValueError("plan entries must be a list")
    seen = set()
    for entry in plan["entries"]:
        if not isinstance(entry, dict) or entry.get("kind") not in ("agent", "squad"):
            raise ValueError("plan contains an unsupported resource kind")
        resource = (entry["kind"], valid_uuid(entry.get("id")))
        if resource in seen:
            raise ValueError("plan contains a duplicate resource")
        seen.add(resource)
        if not isinstance(entry.get("before"), str) or digest(entry["before"]) != entry.get("before_sha256"):
            raise ValueError("original instruction backup does not match its hash")
        valid_timestamp(entry.get("before_updated_at"))
        if type(entry.get("selected")) is not bool:
            raise ValueError("selected must explicitly be true or false")
        if entry["selected"] and not isinstance(entry.get("after"), str):
            raise ValueError("a selected entry must contain a reviewed after string")


def new_private_file(path):
    path = Path(path)
    path.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    return os.fdopen(fd, "w", encoding="utf-8")


def record_journal(journal, record):
    if journal is not None:
        journal.write(json.dumps(record, ensure_ascii=False) + "\n")
        journal.flush()
        os.fsync(journal.fileno())


def execute_plan(api, plan, journal_path=None):
    """No journal means dry-run. Every real write records its backup first."""
    validate_plan(plan, api.server_url)
    if getattr(api, "workspace_id", plan["workspace_id"]) != plan["workspace_id"]:
        raise ValueError("configured workspace does not match the plan")
    summary = Counter()
    context = new_private_file(journal_path) if journal_path is not None else nullcontext(None)
    with context as journal:
        record_journal(journal, {
            "type": "header", "version": 1, "server_url": api.server_url,
            "workspace_id": plan["workspace_id"],
        })
        for entry in plan["entries"]:
            if not entry["selected"]:
                summary["preserved"] += 1
                continue
            path = f"/api/{entry['kind']}s/{entry['id']}"
            try:
                current, headers = api.request("GET", path)
            except APIError as error:
                if error.status == 404:
                    summary["conflict"] += 1
                    record_journal(journal, {
                        "type": "conflict", "kind": entry["kind"], "id": entry["id"],
                        "reason": "resource_not_found",
                    })
                    continue
                raise
            validate_row(current, plan["workspace_id"], entry["id"])
            if {k.lower(): v for k, v in headers.items()}.get(CAPABILITY.lower()) != "1":
                raise ValueError("server lacks atomic instruction preconditions; upgrade it before applying")
            if current["instructions"] == entry["after"]:
                summary["already_matches"] += 1
                continue
            if (current["instructions"] != entry["before"] or
                    current["updated_at"] != entry["before_updated_at"]):
                summary["conflict"] += 1
                record_journal(journal, {"type": "conflict", "kind": entry["kind"], "id": entry["id"]})
                continue
            if journal is None:
                summary["ready"] += 1
                continue
            record_journal(journal, {"type": "attempt", "entry": entry})
            body = {
                "instructions": entry["after"], "expected_instructions": entry["before"],
                "expected_updated_at": entry["before_updated_at"],
            }
            try:
                updated, _ = api.request("PUT", path, body)
                validate_row(updated, plan["workspace_id"], entry["id"])
                if updated["instructions"] != entry["after"]:
                    raise ValueError("write response does not contain the reviewed instructions")
                valid_timestamp(updated["updated_at"])
            except APIError as error:
                if error.status == 409:
                    summary["conflict"] += 1
                    record_journal(journal, {"type": "conflict", "kind": entry["kind"], "id": entry["id"]})
                    continue
                record_journal(journal, {"type": "uncertain", "kind": entry["kind"], "id": entry["id"]})
                raise
            except Exception:
                record_journal(journal, {"type": "uncertain", "kind": entry["kind"], "id": entry["id"]})
                raise
            record_journal(journal, {
                "type": "applied", "entry": entry, "after_updated_at": updated["updated_at"],
            })
            summary["applied"] += 1
    return summary


def rollback_plan(journal_path):
    records = [json.loads(line) for line in Path(journal_path).read_text().splitlines() if line.strip()]
    if not records or records[0].get("type") != "header" or records[0].get("version") != 1:
        raise ValueError("invalid journal header")
    header, pending, entries = records[0], {}, []
    for record in records[1:]:
        kind = record.get("type")
        if kind in ("attempt", "applied"):
            entry = record["entry"]
            validate_plan({
                "version": 1, "server_url": header["server_url"],
                "workspace_id": header["workspace_id"], "entries": [entry],
            }, header["server_url"])
            if not entry["selected"]:
                raise ValueError("journal contains an unselected write")
            identity = (entry["kind"], entry["id"])
            if kind == "attempt":
                pending[identity] = entry
            else:
                if pending.pop(identity, None) != entry:
                    raise ValueError("write receipt does not match the recorded backup")
                valid_timestamp(record.get("after_updated_at"))
                entries.append({
                    **entry, "before": entry["after"], "before_sha256": digest(entry["after"]),
                    "before_updated_at": record["after_updated_at"], "after": entry["before"],
                    "selected": True, "reason": "restore_recorded_write",
                })
        elif kind == "conflict":
            pending.pop((record["kind"], record["id"]), None)
        elif kind == "uncertain":
            pending.setdefault((record["kind"], record["id"]), None)
        else:
            raise ValueError("unrecognized journal record")
    if pending:
        raise ValueError("journal contains an interrupted or uncertain write; reconcile that record before rollback")
    return {
        "version": 1, "server_url": header["server_url"], "workspace_id": header["workspace_id"],
        "entries": list(reversed(entries)),
    }


def git_read(repo, revision, path):
    result = subprocess.run(["git", "show", f"{revision}:{path}"], cwd=repo,
                            capture_output=True, text=True, check=False)
    if result.returncode:
        raise ValueError(f"baseline revision does not contain {path}")
    return result.stdout


def catalog_from_git(repo, baseline_ref):
    revision = subprocess.run(
        ["git", "rev-parse", "--verify", "--end-of-options", baseline_ref + "^{commit}"],
        cwd=repo, capture_output=True, text=True, check=True,
    ).stdout.strip()
    if not re.fullmatch(r"[0-9a-f]{40,64}", revision):
        raise ValueError("baseline must resolve to a full commit ID")
    catalog = {}
    base = Path("server/internal/service")
    for kind, directory, roster in (
        ("agent", "builtin_agent_templates", "builtin_agent_templates_roster.go"),
        ("squad", "builtin_squad_templates", "builtin_squad_templates.go"),
    ):
        old_roster = git_read(repo, revision, (base / roster).as_posix())
        new_roster = (repo / base / roster).read_text()
        for path in sorted((repo / base / directory).glob("*/INSTRUCTIONS.md")):
            key = path.parent.name
            pattern = r'Key:\s*"' + re.escape(key) + r'",\s*Version:\s*(\d+)'
            old_match, new_match = re.search(pattern, old_roster), re.search(pattern, new_roster)
            if not old_match or not new_match:
                continue
            source = git_read(repo, revision, path.relative_to(repo).as_posix()).rstrip("\n")
            target = path.read_text().rstrip("\n")
            if not re.search(r"[\u3400-\u9fff]", target):
                raise ValueError(f"Chinese target is not present for {kind}/{key}")
            catalog[(kind, key)] = {
                "source": source, "target": target,
                "source_version": int(old_match[1]), "target_version": int(new_match[1]),
            }
    if not catalog:
        raise ValueError("no templates were found in the chosen baseline")
    return catalog


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("operation", choices=("plan", "apply", "rollback"))
    parser.add_argument("--config", type=Path, required=True, help="authenticated Multica CLI profile JSON")
    parser.add_argument("--workspace", required=True, help="explicit workspace UUID")
    parser.add_argument("--file", type=Path, required=True, help="plan output / plan input / rollback journal input")
    parser.add_argument("--repo", type=Path, default=Path(__file__).resolve().parents[1])
    parser.add_argument("--baseline-ref", help="commit containing the English defaults being translated")
    parser.add_argument("--write", action="store_true", help="perform conditional writes; omitted means dry-run")
    parser.add_argument("--journal", type=Path, help="new private journal required with --write")
    args = parser.parse_args(argv)
    workspace_id = valid_uuid(args.workspace)
    api = API(json.loads(args.config.read_text()), workspace_id)
    if args.operation == "plan":
        if not args.baseline_ref or args.write or args.journal:
            parser.error("plan requires --baseline-ref and never accepts --write or --journal")
        plan = build_plan(api, workspace_id, catalog_from_git(args.repo.resolve(), args.baseline_ref))
        validate_plan(plan, api.server_url)
        with new_private_file(args.file) as output:
            json.dump(plan, output, ensure_ascii=False, indent=2)
            output.write("\n")
            output.flush()
            os.fsync(output.fileno())
        print(json.dumps(dict(Counter(entry["reason"] for entry in plan["entries"])), ensure_ascii=False))
        return 0
    if args.write != (args.journal is not None):
        parser.error("--write and --journal must be supplied together")
    plan = (rollback_plan(args.file) if args.operation == "rollback"
            else json.loads(args.file.read_text()))
    if plan.get("workspace_id") != workspace_id:
        raise ValueError("explicit workspace does not match the saved plan or journal")
    summary = execute_plan(api, plan, args.journal)
    print(json.dumps(dict(summary), ensure_ascii=False))
    return 1 if summary["conflict"] else 0


if __name__ == "__main__":
    try:
        sys.exit(main())
    except (ValueError, OSError, APIError, subprocess.CalledProcessError) as error:
        print(f"Instruction localization stopped: {error}", file=sys.stderr)
        sys.exit(2)
