#!/usr/bin/env python3
"""Verify the standalone design artifact with the existing gstack browser."""

import argparse
import hashlib
import json
import subprocess
import threading
from datetime import datetime, timezone
from functools import partial
from http.server import SimpleHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path


class QuietHandler(SimpleHTTPRequestHandler):
    def log_message(self, *_args):
        pass


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--browser", type=Path,
        default=Path.home() / ".codex/skills/gstack/browse/dist/browse",
    )
    args = parser.parse_args()
    if not args.browser.is_file():
        parser.error("Pass --browser with the path to the installed gstack browse binary.")

    root = Path(__file__).resolve().parent
    server = ThreadingHTTPServer(("127.0.0.1", 0), partial(QuietHandler, directory=str(root)))
    threading.Thread(target=server.serve_forever, daemon=True).start()
    url = f"http://127.0.0.1:{server.server_port}/preview.html"
    commands = [["newtab", url], ["viewport", "1440x1000"]]
    expected = []

    def cmd(*parts):
        commands.append(list(parts))
        if parts == ("click", "a.back") or parts == ("back",):
            commands.append(["wait", "#screen .grid"])
        elif (parts[0] == "click" and parts[1].startswith("a.integration")) or parts in [("forward",), ("reload",)]:
            commands.append(["wait", "#screen .detail"])

    def check(name, expression):
        expected.append(name)
        cmd("js", "(() => {"
            f"const passed = Boolean({expression});"
            f"return JSON.stringify({{check:{json.dumps(name)},passed}});"
            "})()")

    def screenshot(filename):
        cmd("js", "document.querySelector('#toast').hidden=true")
        cmd("screenshot", str(root / filename))

    def arm_errors():
        cmd("js", "window.__previewErrors=[];"
            "window.addEventListener('error',e=>window.__previewErrors.push(e.message));"
            "window.addEventListener('unhandledrejection',e=>window.__previewErrors.push(String(e.reason)));"
            "true")

    arm_errors()
    check("catalog-renders", "document.querySelector('h1')?.textContent === '集成' && document.querySelectorAll('.integration').length === 6")
    check("catalog-order-and-planned-fuxin", "[...document.querySelectorAll('.section h2')].map(e=>e.textContent).join('|')==='代码托管|任务管理|沟通与协作' && document.querySelectorAll('.section')[2].querySelectorAll('.planned').length===1 && document.querySelectorAll('.section')[2].querySelector('.card-name').textContent==='孚信' && document.querySelectorAll('.section')[2].querySelector('.badge').textContent==='待规划' && !document.querySelector('#screen').textContent.includes('Composio')")
    check("future-items-are-inert", "document.querySelectorAll('.planned').length === 4 && [...document.querySelectorAll('.section')[1].querySelectorAll('.card-name')].map(e=>e.textContent).join('|')==='ONES|Plane|Kaneo' && [...document.querySelectorAll('.planned')].every(e=>e.getAttribute('aria-disabled')==='true' && !e.hasAttribute('href') && e.tabIndex === -1 && !e.querySelector('a,button,.chevron'))")
    check("github-brand-mark-embedded", "['github'].every(id=>document.querySelector(`[data-brand=\"${id}\"] svg path`))")
    check("no-detail-mounted-in-catalog", "!document.querySelector('[data-toggle]')")
    cmd("js", "document.querySelector('#demo-state').focus()")
    cmd("press", "Tab")
    check("keyboard-reaches-navigation", "document.activeElement.matches('aside a.active')")
    cmd("press", "Tab")
    check("keyboard-reaches-github-card", "document.activeElement.matches('a.integration[href=\"#github\"]')")
    check("keyboard-focus-visible", "getComputedStyle(document.activeElement).outlineStyle === 'solid' && parseFloat(getComputedStyle(document.activeElement).outlineWidth) >= 2")
    cmd("press", "Enter")
    cmd("wait", "#screen .detail")
    check("keyboard-opens-github-detail", "location.hash === '#github' && document.querySelector('h1')?.textContent === 'GitHub' && document.activeElement.matches('h1')")
    cmd("click", "a.back")
    check("back-restores-card-focus", "location.hash === '#catalog' && document.activeElement.matches('a.integration[href=\"#github\"]')")
    cmd("click", "a.integration[href='#github']")
    cmd("back")
    check("browser-back-shows-catalog", "document.querySelectorAll('.integration').length === 6")
    cmd("forward")
    check("browser-forward-shows-detail", "document.querySelector('h1')?.textContent === 'GitHub'")
    check("no-runtime-errors-before-refresh", "window.__previewErrors.length === 0")
    cmd("reload")
    check("refresh-preserves-detail-route", "location.hash === '#github' && document.querySelectorAll('[data-toggle]').length === 4")
    arm_errors()
    cmd("select", "#demo-state", "disabled")
    check("master-off-disables-and-clears-visible-child-switches", "[...document.querySelectorAll('.feature-row [role=switch]')].every(e=>e.disabled && e.getAttribute('aria-checked')==='false')")
    check("master-off-preserves-connection", "document.querySelector('.detail-heading .badge').textContent === '已连接' && document.querySelector('#connection-action').textContent === '断开'")
    screenshot("github-disabled.png")
    cmd("click", "[data-toggle='master']")
    cmd("click", "[data-toggle='pr']")
    cmd("click", "[data-toggle='master']")
    cmd("click", "[data-toggle='master']")
    check("master-restores-individual-preferences", "document.querySelector('[data-toggle=pr]').getAttribute('aria-checked')==='false' && document.querySelector('[data-toggle=coauthor]').getAttribute('aria-checked')==='true' && !document.querySelector('[data-toggle=pr]').disabled")
    cmd("click", "[data-toggle='pr']")
    cmd("click", "#connection-action")
    check("disconnect-dialog-has-name-and-safe-focus", "document.querySelector('#confirm').open && document.querySelector('#confirm').getAttribute('aria-labelledby')==='confirm-title' && document.activeElement.id==='cancel-disconnect'")
    screenshot("github-disconnect-confirmation.png")
    cmd("click", "#cancel-disconnect")
    check("cancel-preserves-connection-and-focus", "!document.querySelector('#confirm').open && document.querySelector('#connection-action').textContent==='断开' && document.activeElement.id==='connection-action'")
    cmd("click", "#connection-action")
    cmd("press", "Escape")
    check("escape-cancels-disconnect", "!document.querySelector('#confirm').open && document.querySelector('#connection-action').textContent==='断开'")
    cmd("click", "#connection-action")
    cmd("click", "#confirm-disconnect")
    check("confirm-disconnect-updates-demo-and-focus", "!document.querySelector('#confirm').open && document.querySelector('.detail-heading .badge').textContent==='未连接' && document.activeElement.id==='connection-action'")
    cmd("click", "a.back")
    check("catalog-reflects-disconnection", "document.querySelector('a[href=\"#github\"] .badge').textContent==='未连接'")
    cmd("click", "a.integration[href='#github']")
    cmd("click", "#connection-action")
    check("connect-is-an-explicit-preview", "location.hash==='#github' && document.querySelector('#toast').textContent.includes('正式页面将在此打开 GitHub 授权') && document.querySelector('.badge').textContent==='未连接'")
    cmd("click", "#repository-action")
    check("repository-link-is-an-explicit-preview", "location.hash==='#github' && document.querySelector('#toast').textContent.includes('设置 → 代码仓库')")
    cmd("click", "a.back")
    for provider, name in [("vcs", "自托管 Git")]:
        cmd("click", f"a.integration[href='#{provider}']")
        check(f"{provider}-detail-and-preview-boundary", f"document.querySelector('h1')?.textContent==={json.dumps(name)} && document.querySelector('.demo-hint').textContent.includes('本次可操作预览集中在 GitHub')")
        cmd("click", "a.back")
    cmd("click", "a.integration[href='#github']")
    cmd("js", "location.hash='composio'")
    cmd("wait", "#screen .grid")
    check("removed-provider-recovers-to-catalog", "document.querySelectorAll('.integration').length===6 && !document.querySelector('[data-toggle]')")
    cmd("js", "location.hash='fuxin'")
    check("planned-fuxin-hash-does-not-open-detail", "document.querySelectorAll('.integration').length===6 && !document.querySelector('[data-toggle]')")
    cmd("js", "location.hash='catalog'")
    cmd("select", "#demo-state", "connected")
    for width, height in [(320, 740), (390, 844), (768, 1024), (900, 900), (1024, 768), (1440, 1000)]:
        cmd("viewport", f"{width}x{height}")
        check(f"catalog-{width}-no-horizontal-overflow", "document.documentElement.scrollWidth <= innerWidth && [...document.querySelectorAll('.integration')].every(e=>e.scrollWidth<=e.clientWidth+1)")
        check(f"catalog-{width}-column-count", f"getComputedStyle(document.querySelector('.grid')).gridTemplateColumns.split(' ').length === {1 if width <= 800 else 2}")
        if width in [390, 1440]:
            cmd("js", "document.activeElement.blur(); window.scrollTo(0,0)")
            screenshot("mobile-catalog.png" if width == 390 else "catalog.png")
        cmd("click", "a.integration[href='#github']")
        check(f"github-{width}-no-horizontal-overflow", "document.documentElement.scrollWidth <= innerWidth && [...document.querySelectorAll('.setting-row')].every(e=>e.scrollWidth<=e.clientWidth+1)")
        check(f"github-{width}-control-targets", "[...document.querySelectorAll('#screen button,#screen a')].every(e=>e.getBoundingClientRect().width>=24 && e.getBoundingClientRect().height>=24)")
        if width in [390, 1440]:
            cmd("js", "document.activeElement.blur(); document.querySelector('#toast').hidden=true; window.scrollTo(0,0)")
            screenshot("mobile-github-detail.png" if width == 390 else "github-detail.png")
        cmd("click", "a.back")
    check("no-runtime-errors-during-interactions", "window.__previewErrors.length===0")
    check("no-api-or-external-resource-requests", "performance.getEntriesByType('resource').every(e=>e.name.startsWith('data:') || e.name.startsWith(location.origin)) && !performance.getEntriesByType('resource').some(e=>['fetch','xmlhttprequest'].includes(e.initiatorType))")
    cmd("js", "JSON.stringify({browser:navigator.userAgent,resources:performance.getEntriesByType('resource').map(e=>({name:e.name,initiatorType:e.initiatorType}))})")
    cmd("console", "--errors")
    try:
        result = subprocess.run([str(args.browser), "chain"], input=json.dumps(commands), text=True, capture_output=True, timeout=120)
    finally:
        server.shutdown()
        server.server_close()
    log = result.stdout + result.stderr
    (root / "verification-browser.log").write_text(log)
    checks = []
    browser = None
    for line in log.splitlines():
        if line.startswith("[js] {"):
            item = json.loads(line.removeprefix("[js] "))
            if "check" in item:
                checks.append(item)
            if "browser" in item:
                browser = item
    seen = {item["check"] for item in checks}
    missing = [name for name in expected if name not in seen]
    passed = result.returncode == 0 and not missing and all(item["passed"] for item in checks)
    report = {
        "scope": "Standalone Chinese design preview; no production application changes",
        "verifiedAt": datetime.now(timezone.utc).isoformat(),
        "status": "passed" if passed else "failed",
        "artifactSha256": hashlib.sha256((root / "preview.html").read_bytes()).hexdigest(),
        "tool": "gstack browse / Chromium",
        "command": "python3 .trellis/tasks/10-01-settings-integration-catalog/verify-preview.py",
        "checksPassed": sum(item["passed"] for item in checks),
        "checksTotal": len(expected),
        "checks": checks,
        "missingChecks": missing,
        "browser": browser,
        "screenshots": ["catalog.png", "github-detail.png", "mobile-catalog.png", "mobile-github-detail.png", "github-disabled.png", "github-disconnect-confirmation.png"],
        "notVerified": ["Production Web/Electron integration", "Real OAuth, API writes and permissions", "Deployment visibility filtering and API loading/error states", "English localization and screen-reader software", "Pixel comparison with the original user screenshot (unavailable in resumed session)", "Direct file:// opening (browse permits only HTTP/HTTPS); the self-contained page was tested through a local HTTP server with zero resource requests"],
    }
    (root / "verification.json").write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n")
    print(json.dumps({"status": report["status"], "passed": report["checksPassed"], "total": len(expected), "failed": [item["check"] for item in checks if not item["passed"]], "missing": missing}, ensure_ascii=False))
    raise SystemExit(0 if passed else 1)


if __name__ == "__main__":
    main()
