
import datetime,hashlib,json,os,stat,subprocess,sys,traceback
from pathlib import Path
now=lambda:datetime.datetime.now(datetime.timezone.utc).isoformat()
mode=os.environ["QA_MODE"]
def emit(kind,**data):
    print(json.dumps(dict(at=now(),kind=kind,**data)),flush=True)
def cmd(name,args,allowed=(0,)):
    started=now()
    p=subprocess.run(args,text=True,capture_output=True,timeout=180)
    emit("command",name=name,command=args,started_at=started,exit_code=p.returncode,stdout=p.stdout,stderr=p.stderr)
    assert p.returncode in allowed,(name,p.returncode)
    return p
def check(name,condition,**details):
    emit("assertion",name=name,passed=bool(condition),**details)
    assert condition,name
def sha(p):
    with Path(p).open("rb") as f:return hashlib.file_digest(f,"sha256").hexdigest()
def file_state(p):
    p=Path(p);s=p.stat()
    return dict(sha256=sha(p),bytes=s.st_size,mode=stat.S_IMODE(s.st_mode),uid=s.st_uid,gid=s.st_gid)
expected=json.loads(os.environ["QA_PINS"])
work=Path("/var/tmp/multica-deb-acceptance");work.mkdir()
metadata={}
sentinels={}
cli=Path("/usr/bin/multica")
def sentinel_create():
    global sentinels
    user=Path("/home/qa")
    docs={
      user/".multica/desktop.json":{"schemaVersion":1,"apiUrl":"http://192.0.2.10:18080","webUrl":"http://192.0.2.10:13000","updateUrl":"http://192.0.2.11:18080/desktop","futureSentinel":"retain-existing-configuration"},
      user/".config/Multica/Preferences":{"closeBehavior":"quit","acceptanceSentinel":"synthetic-profile"},
      user/".config/Multica/session-sentinel.json":{"authenticationMode":"password","token":"synthetic-not-a-real-session","account":"qa@example.test"},
      user/".local/state/Multica/regression.json":{"draft":"synthetic retained draft","tabs":["issues"],"serverIdentity":"synthetic-server"}
    }
    for p,data in docs.items():
        p.parent.mkdir(parents=True,exist_ok=True)
        p.write_text(json.dumps(data,sort_keys=True)+"\n")
        os.chmod(p,0o600);os.chown(p,1000,1000)
    sentinels={str(p):file_state(p) for p in docs}
    emit("synthetic_state",manifest=sentinels,scope="Package-manager retention only; no real authentication, migration or GUI session")
def retained(stage):
    observed={p:file_state(p) for p in sentinels if Path(p).exists()}
    check(stage+"-user-state-retained",observed==sentinels,observed=observed)
    check(stage+"-cli-sentinel-retained",file_state(cli)==cli_state)
def installed(label):
    data=metadata[label]
    p=cmd(label+"-package-status",["dpkg-query","-W","-f",chr(36)+"{Status}|"+chr(36)+"{Version}|"+chr(36)+"{Architecture}","multica"])
    check(label+"-configured",p.stdout=="install ok installed|"+data["version"]+"|arm64")
    listing=cmd(label+"-installed-paths",["dpkg-query","-L","multica"]).stdout.splitlines()
    check(label+"-cli-name-not-owned","/usr/bin/multica" not in listing)
    check(label+"-desktop-alternative",Path("/usr/bin/multica-desktop").is_symlink() and os.path.realpath("/usr/bin/multica-desktop")=="/opt/Multica/multica-desktop")
    cmd(label+"-alternative-query",["update-alternatives","--query","multica-desktop"])
    for rel,pin in data["pins"].items():
        check(label+"-installed-"+rel,sha("/"+rel)==pin,sha256=sha("/"+rel))
    desktop=Path("/usr/share/applications/multica-desktop.desktop").read_text()
    emit("desktop_entry",stage=label,text=desktop)
    for line in ["Name=Multica","Icon=multica-desktop","StartupWMClass=Multica","x-scheme-handler/multica"]:
        check(label+"-desktop-"+line,line in desktop)
    check(label+"-desktop-exec","Exec=/opt/Multica/multica-desktop" in desktop)
    icons=sorted(str(p) for p in Path("/usr/share/icons/hicolor").glob("*/apps/multica-desktop.png"))
    check(label+"-icons",len(icons)>=8,paths=icons)
    audit=cmd(label+"-dpkg-audit",["dpkg","--audit"])
    check(label+"-dpkg-audit-empty",not audit.stdout.strip())
    cmd(label+"-dpkg-verify",["dpkg","--verify","multica"])
    retained(label)
try:
    emit("environment",mode=mode,uname=list(os.uname()),os_release=Path("/etc/os-release").read_text())
    check("architecture",cmd("architecture",["dpkg","--print-architecture"]).stdout.strip()=="arm64")
    absent=cmd("package-absent",["dpkg-query","-W","-f",chr(36)+"{Status}","multica"],allowed=(1,))
    check("package-absent",absent.returncode==1)
    check("cli-name-initially-absent",not cli.exists())
    cli.write_text("#!/bin/sh\nprintf 'synthetic-cli-sentinel\\n'\n");cli.chmod(0o755)
    cli_state=file_state(cli)
    for label in ["baseline","candidate"]:
        deb=Path("/packages")/(label+".deb")
        check(label+"-archive-hash",sha(deb)==expected[label],sha256=sha(deb))
        control=cmd(label+"-control",["dpkg-deb","--info",str(deb)])
        version=cmd(label+"-version",["dpkg-deb","-f",str(deb),"Version"]).stdout.strip()
        check(label+"-architecture",cmd(label+"-arch",["dpkg-deb","-f",str(deb),"Architecture"]).stdout.strip()=="arm64")
        check(label+"-package",cmd(label+"-name",["dpkg-deb","-f",str(deb),"Package"]).stdout.strip()=="multica")
        extracted=work/label;control_dir=work/(label+"-control")
        cmd(label+"-extract-control",["dpkg-deb","-e",str(deb),str(control_dir)])
        scripts={p.name:{"sha256":sha(p),"text":p.read_text()} for p in control_dir.iterdir() if p.name in ["preinst","postinst","prerm","postrm"]}
        emit("maintainer_scripts",stage=label,scripts=scripts)
        cmd(label+"-extract-data",["dpkg-deb","-x",str(deb),str(extracted)])
        relative=["opt/Multica/multica-desktop","opt/Multica/resources/app.asar","opt/Multica/resources/app.asar.unpacked/resources/bin/multica"]
        pins={r:sha(extracted/r) for r in relative}
        metadata[label]=dict(version=version,pins=pins)
        emit("payload",stage=label,**metadata[label])
        if label=="candidate":
            check("candidate-payload-pins",pins==expected["payload"],observed=pins)
    cmd("candidate-version-newer",["dpkg","--compare-versions",metadata["candidate"]["version"],"gt",metadata["baseline"]["version"]])
    sentinel_create()
    if mode=="upgrade":
        cmd("baseline-install",["dpkg","-i","/packages/baseline.deb"])
        installed("baseline")
        cmd("candidate-upgrade",["dpkg","-i","/packages/candidate.deb"])
        installed("candidate")
        emit("result",status="PASS",mode=mode,versions={k:v["version"] for k,v in metadata.items()},package_manager_only=True)
    else:
        cmd("candidate-fresh-install",["dpkg","-i","/packages/candidate.deb"])
        installed("candidate")
        cmd("candidate-remove",["dpkg","--remove","multica"])
        retained("remove")
        check("remove-payload-absent",not Path("/opt/Multica/multica-desktop").exists())
        check("remove-launcher-absent",not os.path.lexists("/usr/bin/multica-desktop"))
        cmd("candidate-reinstall",["dpkg","-i","/packages/candidate.deb"])
        installed("candidate")
        cmd("candidate-purge",["dpkg","--purge","multica"])
        retained("purge")
        check("purge-launcher-absent",not os.path.lexists("/usr/bin/multica-desktop"))
        emit("result",status="PASS",mode=mode,versions={k:v["version"] for k,v in metadata.items()},package_manager_only=True)
except Exception as e:
    emit("result",status="FAIL",mode=mode,error_type=type(e).__name__,error=str(e),traceback=traceback.format_exc())
    sys.exit(1)
