# Execution

1. Inspect suite entry points and isolation contracts; record the baseline.
2. Run all configured JavaScript/TypeScript unit suites and mobile tests.
3. Create a dedicated database, apply migrations, and run all guarded Go tests.
4. Start an isolated API/web environment and run all configured Playwright tests.
5. Run native-agent skill selection and invocation scenarios with safe fixtures.
6. Investigate failures enough to distinguish product failures, unsupported
   environments and test-harness issues; retry only with a concrete reason.
7. Summarize actual results and preserve logs; close task state and leave the
   user's desktop workspace and working branch intact.
