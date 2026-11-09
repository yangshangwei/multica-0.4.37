# Onboarding questionnaire write ordering review

Independent read-only review of the current flow serialization and core saveQuestionnaire response fence. No blocking finding in this increment. No browser/API operation was run by this reviewer.

The server replaces the whole questionnaire object, so parallel snapshot PATCHes can commit out of order and drop newer role/use-case answers. currentAnswers derives the next complete snapshot synchronously from the prior local snapshot, and answerWrites serializes sends outside React state updater functions. Both runtime and runtime-skipped completion paths drain queued writes then await a final complete snapshot before finalizing. A failed autosave can recover via that final save; a failed final save does not complete onboarding.

Every queued send captures API scope/user and checks both before issuing its request. Core saveQuestionnaire also captures scope/user before the PATCH and rejects a late response before replacing auth-store user or emitting analytics. This prevents a queued old actor from sending via a new credential and prevents a completed old response from resurrecting the old identity. Existing positive completion paths remain intact.

The queue is local to the onboarding flow; product/API response semantics are unchanged. This review does not generalize the new fence to unrelated pre-existing bootstrap/complete workflows. Author's deterministic ordering and identity regressions remain canonical. This reviewer ran combined core auth+onboarding focused suites:4files70PASS; root owns current frontend full/type/browser verification after the performance window.
