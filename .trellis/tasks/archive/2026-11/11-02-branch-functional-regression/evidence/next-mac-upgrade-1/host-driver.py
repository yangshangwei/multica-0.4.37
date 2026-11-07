import datetime
import hashlib
import json
import os
from pathlib import Path
import plistlib
import shutil
import stat
import subprocess

ROOT=Path("/Volumes/artisan/code/2026/multica-0.4.37")
TASK=ROOT/".trellis/tasks/11-02-branch-functional-regression"
PRIVATE=Path(__file__).resolve().parent
RUN=TASK/"evidence/next-mac-upgrade-1"
record=json.loads((RUN/"run.json").read_text())
now=lambda:datetime.datetime.now(datetime.timezone.utc).isoformat()
def sha(p):
    with Path(p).open("rb") as f:
        return hashlib.file_digest(f,"sha256").hexdigest()
def persist():
    (RUN/"run.json").write_text(json.dumps(record,indent=2)+"\n")
def invoke(name,command,allowed=(0,)):
    step=dict(name=name,command=command,cwd=str(ROOT),started_at=now())
    record["commands"].append(step);persist()
    with (RUN/(name+".stdout")).open("w") as out,(RUN/(name+".stderr")).open("w") as err:
        p=subprocess.run(command,cwd=ROOT,stdout=out,stderr=err,text=True,timeout=180)
    step.update(exit_code=p.returncode,finished_at=now(),stdout_sha256=sha(RUN/(name+".stdout")),stderr_sha256=sha(RUN/(name+".stderr")))
    persist()
    assert p.returncode in allowed,(name,p.returncode)
    return (RUN/(name+".stdout")).read_text()
def check(name,value,**details):
    row=dict(name=name,passed=bool(value),**details)
    record.setdefault("assertions",[]).append(row);persist()
    assert value,name
def manifest(directory):
    result={}
    for p in sorted(directory.rglob("*")):
        s=p.lstat()
        row={"mode":stat.S_IMODE(s.st_mode)}
        if p.is_symlink():
            row.update(kind="symlink",target=os.readlink(p))
        elif p.is_file():
            row.update(kind="file",bytes=s.st_size,sha256=sha(p))
        elif p.is_dir():
            row.update(kind="directory")
        result[str(p.relative_to(directory))]=row
    return result
def file_state(p):
    s=p.stat()
    return dict(sha256=sha(p),mode=stat.S_IMODE(s.st_mode),uid=s.st_uid,gid=s.st_gid,bytes=s.st_size)

record.update(state="file-lifecycle-running",runner_sha256=sha(__file__),runtime_launches=0,
              default_protocol_registration_calls=0,external_requests=0,subgates={})
persist()
try:
    invoke("host",["sw_vers"])
    check("host-arm64",invoke("architecture",["uname","-m"]).strip()=="arm64")
    baseline=record["artifacts"]["baseline"]
    candidate=record["artifacts"]["candidate"]
    check("same-bundle-identity",baseline["bundle"]["CFBundleIdentifier"]==candidate["bundle"]["CFBundleIdentifier"]=="ai.multica.desktop")
    check("candidate-version-greater",tuple(int(i) for i in candidate["bundle"]["CFBundleShortVersionString"].split("-")[0].split("."))>tuple(int(i) for i in baseline["bundle"]["CFBundleShortVersionString"].split("-")[0].split(".")))
    check("baseline-is-dirty-artifact","-dirty" in baseline["bundle"]["CFBundleShortVersionString"])
    for label,artifact in record["artifacts"].items():
        archive=Path(artifact["zip"])
        check(label+"-zip-pin",sha(archive)==artifact["zip_sha256"])
        invoke(label+"-zip-integrity",["unzip","-tq",str(archive)])
        stage=PRIVATE/(label+"-extracted")
        stage.mkdir(exist_ok=False)
        invoke(label+"-extract",["ditto","-x","-k",str(archive),str(stage)])
        app=stage/"Multica.app"
        metadata=plistlib.loads((app/"Contents/Info.plist").read_bytes())
        check(label+"-extracted-version",metadata["CFBundleShortVersionString"]==artifact["bundle"]["CFBundleShortVersionString"])
        check(label+"-extracted-asar",sha(app/"Contents/Resources/app.asar")==artifact["asar_sha256"])
        invoke(label+"-arch",["lipo","-archs",str(app/"Contents/MacOS/Multica")])
        invoke(label+"-signature-structure",["codesign","--verify","--deep","--strict","--verbose=2",str(app)])
        invoke(label+"-signature-display",["codesign","--display","--verbose=4",str(app)])
        artifact["extracted_app"]=str(app)
        artifact["payload_sha256"]={rel:sha(app/"Contents"/rel) for rel in [
            "MacOS/Multica","Resources/app.asar","Resources/app.asar.unpacked/resources/bin/multica"]}
        source_manifest=manifest(app)
        (RUN/(label+"-app-manifest.json")).write_text(json.dumps(source_manifest,indent=2)+"\n")
        artifact["manifest_sha256"]=sha(RUN/(label+"-app-manifest.json"))
        persist()
    pins=json.loads((TASK/"research/reusable-package-pins.json").read_text())["mac"]
    for rel,pin in pins.items():
        check("candidate-original-pin-"+rel,sha(Path(candidate["extracted_app"])/"Contents"/rel)==pin["sha256"])
    install=PRIVATE/"installed"
    install.mkdir(exist_ok=False)
    installed=install/"Multica.app"
    home=PRIVATE/"acceptance-home"
    sentinels={
        home/".multica/desktop.json":{"schemaVersion":1,"apiUrl":"http://127.0.0.1:19341","appUrl":"http://127.0.0.1:19342","updateUrl":"http://127.0.0.1:19343/desktop","futureSentinel":"preserve-existing-config"},
        home/".multica/desktop_prefs.json":{"autoStart":False,"autoStop":False},
        home/".config/Multica/updater-preferences.json":{"automaticUpdates":False},
        home/".config/Multica/device-identity.json":{"synthetic":True,"deviceId":"11111111-1111-4111-8111-111111111111","note":"retention sentinel; no real identity or account"},
        home/".config/Multica/acceptance-state.json":{"authenticationMode":"password","draft":"synthetic retained draft","tabs":["issues"],"note":"server auth mode marker only; desktop configuration cannot select auth mode"}
    }
    for p,data in sentinels.items():
        p.parent.mkdir(parents=True,exist_ok=True)
        p.write_text(json.dumps(data,sort_keys=True)+"\n");p.chmod(0o600)
    before={str(p.relative_to(home)):file_state(p) for p in sentinels}
    (RUN/"synthetic-profile-before.json").write_text(json.dumps(before,indent=2)+"\n")
    invoke("baseline-manual-install",["ditto",baseline["extracted_app"],str(installed)])
    check("baseline-installed-manifest",manifest(installed)==json.loads((RUN/"baseline-app-manifest.json").read_text()))
    check("baseline-installed-identity",plistlib.loads((installed/"Contents/Info.plist").read_bytes())["CFBundleShortVersionString"]==baseline["bundle"]["CFBundleShortVersionString"])
    retained=PRIVATE/"retained-baseline.app"
    invoke("retain-installed-baseline",["mv",str(installed),str(retained)])
    invoke("candidate-manual-upgrade",["ditto",candidate["extracted_app"],str(installed)])
    check("candidate-installed-manifest",manifest(installed)==json.loads((RUN/"candidate-app-manifest.json").read_text()))
    check("candidate-installed-identity",plistlib.loads((installed/"Contents/Info.plist").read_bytes())["CFBundleShortVersionString"]==candidate["bundle"]["CFBundleShortVersionString"])
    invoke("candidate-installed-signature",["codesign","--verify","--deep","--strict","--verbose=2",str(installed)])
    after={str(p.relative_to(home)):file_state(p) for p in sentinels}
    (RUN/"synthetic-profile-after.json").write_text(json.dumps(after,indent=2)+"\n")
    check("synthetic-filesystem-profile-retained",after==before)
    check("retained-baseline-unchanged",manifest(retained)==json.loads((RUN/"baseline-app-manifest.json").read_text()))
    for label,artifact in record["artifacts"].items():
        check(label+"-distribution-unchanged",sha(artifact["zip"])==artifact["zip_sha256"])
    static=json.loads((RUN/"static-main-inspection.json").read_text())
    check("both-explicit-home-overrides-present",all(x["matches"]["MULTICA_PACKAGED_ACCEPTANCE_HOME"]["present"] for x in static["artifacts"].values()))
    check("both-register-host-protocol",all(x["matches"]["setAsDefaultProtocolClient"]["present"] for x in static["artifacts"].values()))
    record.update(installed_app=str(installed),acceptance_home=str(home),
        state="complete",finished_at=now(),
        subgates={"distribution_integrity":"PASS","manual_file_install_upgrade":"PASS","synthetic_filesystem_state_retention":"PASS","actual_old_to_new_main_restart":"BLOCKED","actual_runtime_retained_state":"NOT_RUN","business_login_ipc_auto_update":"NOT_RUN","developer_id_notarized_delivery":"BLOCKED"},
        runtime_blocker="Both immutable bundled Main programs call app.setAsDefaultProtocolClient unconditionally before the single-instance lock. Acceptance HOME redirects file paths but does not isolate host LaunchServices registration. No safe OS/protocol boundary was proven; assigned scope forbids native host registration, so neither application was launched.",
        baseline_provenance_limit="Historical0.6.5-185-g61ff581d2-dirty exact ZIP bytes validated; source commit and dirty source diff are not reconstructed. Result applies only to these exact artifact bytes.",
        cleanup={"app_processes_started":0,"daemon_processes_started":0,"mounts_created":0,"external_requests":0,"host_applications_modified":False,"user_profile_modified":False,"task_files_retained":True},
        product_lint_typecheck="Not repeated: no product code or dependency changes; actual distribution/copy/signature/hash checks executed.")
    persist()
    print(json.dumps({"state":record["state"],"subgates":record["subgates"],"assertions":len(record["assertions"]),"runtime_blocker":record["runtime_blocker"]},indent=2))
except Exception as error:
    record.update(state="failed",finished_at=now(),error_type=type(error).__name__,error=str(error))
    persist()
    raise
