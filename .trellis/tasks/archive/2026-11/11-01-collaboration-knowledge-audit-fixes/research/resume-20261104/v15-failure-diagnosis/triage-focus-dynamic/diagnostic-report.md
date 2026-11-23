# Triage initial-focus diagnostic

The installed Base UI / Playwright sequence supports the exact observed failure:
Reason receives focus and selection, a previously queued real first-control
focus callback moves focus to Duplicate target task, then Playwright's actual
server Keyboard.insertText writes the reason into that combobox. One bounded
scheduled control reproduced the original blank-target assertion failure.

The separate positive control waited for intended initial target focus and
immediately checked Reason after filling. The real initial callback completed
before Reason focus, no callback remained at insertText, and all 52 original
pagination, keyboard, selection, API and history assertions passed. The private
control adds only those two proposed boundary assertions; it clears no values
and changes no product or installed SDK file.

Natural EN/ZH runs both passed once with no retries. EN retained a correct focus
timeline; ZH retained its complete trace but its final window collector was
unavailable. No natural stealing event was captured, so this establishes a
high-confidence reachable mechanism, while attribution of the historical v15
event remains medium confidence. It is not an original-run reproduction claim.

The proposed owning change is limited to
`e2e/fixtures/triage-duplicate-picker.ts`: await intended target initial focus
before existing Reason.fill, and immediately assert its original reason value.
Keep every existing subsequent action and assertion. Root independent review
and separate implementation/directed-green authorization remain required.

The new dedicated DB and production API/Web remain in the recorded diagnostic
window for Root review. All v15 final references, old .next files/modes,
installed desktop files/modes and candidate source status were rechecked
unchanged. Knowledge allocation/calls remain 0/0 and production injection off.

`diagnostic-report.json` contains exact source, trace, event, control and artifact
hashes. The initial exit127 startup and successful private PATH-corrected retry
remain separate setup evidence and are not focus reproduction samples.
