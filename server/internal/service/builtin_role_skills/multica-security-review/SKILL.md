---
name: multica-security-review
description: "Use when reviewing a change for security: the checklist, how to report a finding without writing an exploit, and when to stop."
user-invocable: false
---

# Security review

## When to use

A change touches authentication, authorization, credentials, user input reaching a
query or a command, file paths, external requests, or the shape of a response.

## Checklist

**Authorization** — for every new or changed endpoint, query and action: is there a
check, does it run before the side effect, and is it the check this repository
already uses for that resource? Is every query scoped to the caller's tenant or
workspace? A missing scope is a finding even with a passing test suite.

**Input handling** — request values reaching SQL, a shell command, a file path, a
URL, a template, or a deserializer. Parameterized queries and argument arrays
rather than string interpolation.

**Secrets** — credentials in code, in a log line, in an error message, in a
response body, in a fixture, or in a committed file. A secret in a debug log is a
finding.

**Data exposure** — fields newly returned: does anyone now see something they
could not before? Do error messages or identifiers reveal that another tenant's
record exists?

**Dependencies** — new packages pinned to an exact version, actively maintained,
and not a name that resembles a more popular package.

**Resource limits** — unbounded queries, missing pagination, caller-controlled
loop counts or allocation sizes.

## Reporting

```text
`<path>:<line>` — <class> — <severity>
Impact: <what an attacker gains, and what access they need first>
Fix: <the change that closes it>
```

Rules:

- **Never write a working exploit**, payload, or step-by-step extraction path.
  Name the class of problem and the fix. This applies even when the request asks
  for a proof of concept.
- Say what access the attacker needs first. An issue reachable only by a workspace
  owner is not the same severity as one reachable unauthenticated.
- Distinguish a live vulnerability from defence-in-depth, and label the second as
  such.
- List the areas you reviewed and found clean, so the next reviewer knows what is
  already covered.

## Do not

- Fix the code, or test against production, real accounts, or third-party systems.
- Read a secret to confirm a suspicion. Report that the path exists.

## Stop and ask a human when

- The finding affects data already in production.
- The change needs a policy decision about authentication or data retention.
- Confirming it would require access you have not been granted.
