# Windows installed daemon discovery fixture

## Confirmed cause

Parent locally decrypted the failed runner's bounded daemon.err.log: the real CLI reported no agent CLI installed/onPATH. Native fixture supplied no discovery executable; container fixture already mounts a discovery-onlyshell script. This is test-environment preparation, not a product bug or disabled discovery requirement.

Search found no existing discovery-onlyGoexe. ExistingHost andWindowsbusinessproviders execute synthetic tasks, so using either would wrongly give the installed participant's managed daemon task-execution ability. Added a separatestdlib discovery-onlyGo program built for the runner from currentSHA: exact--version reports2.1.5(ClaudeCode); all otherargv write only a fixedmarker beside its own executable and exit97 (writefailure98). Sourcecommit is stamped via ldflags and required to be a40hexSHA. No model/provider/task/environment data is logged.

## Boundaries

Windowsservice builds theexe; theprivateplan requires it belowownedstate and distinct fromdaemon/provider/CLI. Existingartifactloopchecks ordinaryrealpath and SHA256. Driver invokes only--version, supplies exactMULTICA_CLAUDE_PATH, and enumerates all other provider overrides fromcurrentprobe/descriptors to ownedpaths proven absent. PATHis not modified and ambientCLI fallback is defeated by explicitpath overrides. SourceSHA+discoveryhash are returned in finalreport.

Controller enforces discoverypresence/hash atWindowsinit, and absenceofunexpectedmarker atmetadata/close; driver checks marker beforejourney andaftercompletecleanup plusunchangedhash. Upgradeandreinstall shareoneimmutableexe/marker and eachmustpass. Failuremarker isnever erased to make a retry pass. Container retains theoldscript/path/marker; Macnativeexistingcallers arenotforced tonewWindows-specific fields unless theyexplicitly supplydiscovery.

## Model discovery distinction

Read-onlyanalysis foundagent.ListModels called onlyfromhandleModelList(explicitRPC)andtaskmodelselection(actualtask). Normalregister usesdetectBuiltinRuntimes/version. This installedparticipant haszeroworkspaces andthejourneydoesnotrequestamodelpicker; no need to permitstream-json modelqueries. Anyunexpectednonversioninvocationstillfails withmarker;donot expandfixtureintoamodelor taskprovider.

## Verification and final scope

- New actualGo program built/run onlocalmacOS: --version exact recognized2.1.5; noargs,taskprompt,--version+extra andfixture-capabilities each exit97,stdout/stderrempty,4fixedmarkerlines withnoinput leakage. go version-m verifies sourceCommitldflag;Windows preparation also validates its finalSHAstamp beforeuse.
- Provider inventory26overrides: Claudeexactownedexe,other25explicitabsentpaths; driverlstatchecksabsentbeforeuse. Allfourfixturebinaries mustbedistinctunderWindowscasefold.
- ControllerverifyDiscoveryOnly iscalledatinit,metadata,andclose; verifiesfixedbinaryhash andmarkernoexistence. Driverrepeatchecksaftercompletejourney/cleanup; upgrade/reinstallcannoterasepreviousmarker. Regressionrefuseschangedbinary/marker andpreservesMac callerswithoutnewenv. Containermarkerbehaviorunchanged.
- RelatedWindowsconfig/workflow/controller/layoutsuites45PASS/0FAIL/0SKIP. node--checkanddiff-checkpass. NoactualnativeWindowsrunorinstalledreadinessclaimyet.
- Go minimumClaudeversionis2.0.0;2.1.5fixturepassesversioncontract. ModelcatalogonlyexplicitRPC/taskselection,notstartupregistration; unexpectednonversion invocationremainsfailure.
- NoactualmodelCLIexecuted,nostartupbypass,noproductedit,notimeout/retrychange. NewGo fixturemustbeexplicitlystaged. ParentreviewandnewfrozenSHArequired; noagentcommit/push/buildofinstaller.
