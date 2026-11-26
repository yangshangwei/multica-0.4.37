# Windows final-uninstall protocol fixture

## Confirmed setup conflict

Run45706259050 completed both full installedOFF collaboration journeys and actualreinstall. Packaged Main registers multica:// onlaunch (index.ts663), so the nextforeign-protocol preservation test cannot assume no registration. Thepreviousguard rejected this task's own expectedregistration; no productuninstaller failure was reached.

Read Electronv39.8.7 primarysources: shell/browser/browser_win.cc GetProtocolLaunchPath/SetAsDefaultProtocolClient andshell/common/command_line_util_win.cc AddQuoteForArg. Packagedcall suppliesnoextraargs, producingcurrentexecutable (quotedwhenneeded; no-spacepath maybebare) followedbyliteralquoted%1. Localinstaller.nsh onlyremovesits owncommand; unchanged.

## Scoped change

Accept existingHKCUprotocol onlyif defaultcommandexactlymatchescurrentinstallDirectory/Multica.exe plussole%1 argument. Rejectnonstring,nonabsoluteexpectedpath,quotes/newlines,extra/missingargs,otherpath ormissingcommand. Prioracceptedregistry is exported to tasktestRootbeforemutation; publicreport records onlymissing/owned_candidate andbackup/verificationbooleans.

Set task-ownedforeigncommand plusrandomMulticaAcceptanceOwner mark. Finaluninstall stillmustpreservethat exactforeigncommand. Removalrequiresbothcommandandownermatch; finally repeats onlythisguardedcleanup. Unknownchangedstateisleftalone, nobroadregistrydelete or productuninstaller change. Missingregistration retainsoriginalsetup path. Temporarybackup remainswithfailureevidenceuntilrunnerdisposal.

## Verification

ExistingPS ASTsuite nowextractsrealmatcher/cleanuphelpers. Cases coverquotedpathwithspaces,unquotedno-spacepath,wrongpath,args,%slot,newline,null; uniqueHKCU/Software randomfixturekey coversmissing,wrongowner,wrongcommand,unchangedstateandmatchedcleanup. Realregistrytestsareisolatedfrommultica:// andrequireWindows; localhosthasnopwshandtheywerenotrunhere.

ExistingNodeWindowsworkflow/candidatecontracts6PASS/0FAIL/0SKIP andgitdiff--checkpass. NewnativelifecyclemustvalidateactualElectronregistrycommand andforeignhandlerpreservation. No productchange, timeoutchange, installerrebuild,commit/pushbythisagent.

Primary references:
- https://raw.githubusercontent.com/electron/electron/v39.8.7/shell/browser/browser_win.cc
- https://raw.githubusercontent.com/electron/electron/v39.8.7/shell/common/command_line_util_win.cc
