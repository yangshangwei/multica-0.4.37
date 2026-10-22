# Full Main package prerequisites

Three host package bind paths (/tmp, /private/tmp and task-owned user cache)
were rejected by Docker Desktop before app launch. No sharing settings changed.
The exact package was copied into a named task-only Docker volume; repo read-only
bind separately succeeds for the existing Playwright pure-JS dependency.

The first Xvfb smoke container did not use an init process and remained in its
Xvfb readiness wait before Node or app launch. Process inspection found only
xvfb-run and Xvfb, with no runtime config file or Electron/Node. This is no
product evidence. The actual fixture uses docker --init and the same package
volume; full production Main assertion is required before acceptance. Only the
named task container is stopped/removed. No user process/config/Home is changed.
