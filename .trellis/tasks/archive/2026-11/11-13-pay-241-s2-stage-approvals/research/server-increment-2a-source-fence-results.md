# S2 A5 source-member fence follow-up

The A5 revocation race is fixed and passes targeted normal/race verification. The actual decision-write hook previously allowed a second connection to delete source membership and then allowed A5 to commit. `stageKindConfirmScope` now uses the existing workflow NOWAIT member-row fence. The source-first transaction blocks DELETE, a committed deletion rejects cached nonempty scope selection, and empty scope confirmation remains available to the external designated approver.

The adjacent JSON binds the two implementation/test files, exact frozen copies, original red, green and race logs, direct exits and named tests. The C1 comment fence was a passing 2b control even before this fix; it is now in the separate 2b report test file. No 2b completion or full acceptance claim follows. Prior reports and the earlier manifest reconstruction failure remain historical.
