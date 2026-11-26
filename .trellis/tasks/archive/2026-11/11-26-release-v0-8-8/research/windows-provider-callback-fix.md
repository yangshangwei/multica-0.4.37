# Windows provider callback Stop boundary

## Observed failure and evidence limit

Run45702305203 completes daemonstart, Stop, pausedrestart andResume, receives the thirdprovider firstframe, butwaitForVisit3 still sees the cumulative generic Provider callback socket failed. The oldhandler discardederrorcode andsocketidentity. Thus historical evidence does not prove whichsocket/code failed; it must not be retroactively labeled ECONNRESET. ProviderGo holds a TCPcallback whileawaitingrelease, andforcedWindows terminationcanresetit. A realNode FIN/RST control reproduces cleanclose versusECONNRESET; codealone cannot distinguish expectedStop fromtransportfailure.

## Exact qualifying boundary

The spec registers intent for the acceptedsecondvisit immediatelybeforeits unchangedrealStopcontrolcall. Registration requires anownedopen callback, noexistingerrors, norelease, nopendingaction. Every errorisretained withits exactvisit reference, fixedcode, after_stop_intent andidle_at_error snapshots. Nothing isremovedfromtheeventrecord orordinaryprotocolerrors.

Aftertheexisting Stopreceipt assertion, same-runsettlement andexistingclosedpoll, confirmation requiresstop_requested receiptwithsametarget, samevisitrunID, forced_dispositionstopped, progresssettled, host_exit_observed, closed/destroyedsocket, norelease/pendingaction. OnlyWindowsECONNRESET observedafterintentwhileidlethenconfirmed isexcludedfromtheunqualifiederrorview. SameIDondifferentsocket, preintentreset, releasedprovider, pendingaction-at-error, unknownproof, wrongrun/target, nonreset andnonWindows errorsremainfailures. Finalsummary retainscallback_errors andmusthavezero unqualifiederrors.

No timeout/waitbudget, realcontrolrequest, originalreceipt/state/history/identity assertion, providercodeorproductcode changed. The same30s waitForVisit andoriginalscenario timeoutremain. OrdinaryFINdoesnotgeneratefailure. Finalsummary.errors is anadditionalguardagainsterrorsafterthelastrelease. Thewrapperpreserves thedynamicerrors getter acrossobjectspread.

## Verification

- Actualhelpersemantic Node tests plusrealTCPcontrols:12PASS/0FAIL/0SKIP. Real transport casescovergracefulFIN, unexpectedRST, controlledRST. Negativecontrolscoverbeforeintent, pendingaterror(latercleared), wrongrun/target/proof, notclosed, differentsocketwithsameID, alreadyreleased/closed/actionbusy, nonreset/nonWindows.
- StrictTypeScript programfortwochangedTSroots andtheirdependencies:0targetdiagnostics/0dependencydiagnostics. OptionsESNext/bundler/strict/allowJs/skipLibCheck, existingcorepaths.
- gitdiff--checkpass. NewfullWindowsrunstillneeded to observe exactcallback_errors. Do notclaimhistoricalresetcausalityorthisrun'sacceptancebeforethat evidence.

Only e2e/fixtures/collaboration-host-runtime.ts, its existingnode-test, andinstalledspec passiveobservationhooks changed; no commit/push/build by thisagent.
