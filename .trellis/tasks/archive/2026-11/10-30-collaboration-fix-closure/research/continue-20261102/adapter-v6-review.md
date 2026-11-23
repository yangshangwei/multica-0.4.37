# v6 adapter / launcher independent re-review

Verdict: **changes-required**. The original three findings are substantially corrected and all 29 supplied fake cases pass, but independent consumer/recursive probes found two additional blocking contracts. Do not generate v6 clearance.

## Findings (corrected by implementation and independently verified)

- AV5-P1: ordinary and recursive Go test branches remove DATABASE_MAX_CONNS / DATABASE_MIN_CONNS; non-test and compile paths retain API settings. V5 independently fails the two test-child cases while the non-test case passes; v6 passes all three.
- AV5-P2: original dotenv is parsed as data; unsupported raw substitution/backticks reject. Canonical settings remove inherited context and preserve forced API10/1 values, use O_EXCL mode0600, and remain literal in a real temporary bash consumer. The Make consumer below still prevents complete closure.
- AV5-P3: candidate, actual Go/pnpm, self launcher and source configuration hashes are checked before safe-file/test/service creation. All mutation/missing/blocking review fake cases reject before fake make.

## Findings (not fixed)

### AV6-M1 — Make consumes the canonical file as code (high, blocking)

The v6 parser's raw check rejects `$(...)`, but escaped raw `$`/parentheses pass `shlex.split` and produce literal `$(shell touch <task-owned-marker>)`. `shlex.quote` protects the shell consumer; it does not protect Make expression evaluation. The actual Makefile includes ENV_FILE at line8 and exports all variables at line29.

An independent pure-temp Make include/export fixture with an actual external recipe created its harmless marker. Explicit variable use and eager PORT-style assignment also did. No repository target, DB, Provider or network was executed. The initial @: probe produced no marker and is retained as insufficient coverage, rather than treated as contrary proof.

Reject Make-active dollar symbols in parsed values before writing canonical input, or provide a separate proven safe Make boundary. Require a real temporary Make consumption regression, not only bash loading. Root/implementer owns this correction.

### AV6-G1 — recursive mode lacks a package permit (high, blocking)

The v6 recursive branch validates the clone name and allocation's template/state/database, but not allocation.package. With a record naming `example/permitted`, the fake `go test example/different` entrypoint returns0 and uses the same clone. This is inconsistent with the package-owned clone and exact child package permit contract.

Resolve requested recursive packages under the actual -C/build context and compare with the allocation permit before test execution; refuse missing/mismatched package records, preserving same-package recursion and compile-only handling.

### AV6-N1 — mode1 REAL_GO fallback (scope note)

A fully mocked entrypoint confirms ordinary mode1 tests lacking REAL_GO fall back to `/opt/homebrew/bin/go`. Template validation, clone creation and evidence still happen. This is weaker explicit tool selection, not observed default-DB access or a clone-isolation bypass. The selected final launcher already provides REAL_GO. A narrow explicit-tool guard is reasonable; it should be reported accurately.

## Verification

- Current v6 Go fake suite: **14/14 pass**, exit0.
- Current v6 launcher suite: **15/15 pass**, exit0, including canonical values and real temporary bash literal/mode0600 checks.
- Independent v5 red: pool3 cases → 2 expected failures /1 pass; launcher7 cases → 7 expected failures.
- Five Python source AST checks pass; final green-test source/config hashes stayed unchanged.
- Lint/TypeCheck not run: no JS/TS source change, this dispatch is fake/AST only.
- Make execution was limited to disposable harmless fixtures. No real database, service, browser, Provider, account, network or complete check was run.
- No implementation source or clearance file was modified; only this report and its JSON were written.

## Reviewed bindings

| Binding | Value |
|---|---|
| candidate | `64a9994ec549e8dff73ab14456dc9a9f4b63f7c7` |
| Go adapter | `b8c5d5adbece4f781a00a230b4f6ffbbfed195f4e44826d53997d72b3f160079` |
| pnpm adapter | `7a416eb60f1bdbe6bdf7421d2f61346d74d0e2575ae8a5236e450627db63ff48` |
| launcher | `7e2592d1b6e0a795ce9078f4c97d04255f635d392ff7e16a154309aa87695516` |
| source env | `99475d2e36917761e88bab9687073ddb182c131241d8d815b7eb18850572dfe2` |
| renderer | `http://127.0.0.1:19845/` |

Candidate status was clean and source env mode0600. Machine record retains raw red/green output, source/test hashes and all independent probes. Current bindings accompany a changes-required review and are not a launcher authorization.

