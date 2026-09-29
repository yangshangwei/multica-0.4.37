# Persist the desktop intranet update URL

The user confirmed that entering a business server must also configure the same hostname at `http://<host>:18080/desktop`.

Acceptance:
- Saving a server persists the derived update URL, including HTTPS API addresses, custom API ports/paths and IPv6 hosts.
- Existing packaged configs missing the field are backfilled atomically; preserve unknown fields and explicit update URLs. Invalid configs still fail closed. Development mode never reads/writes packaged configuration.
- Custom update sources survive server changes; the standard generated source follows the new hostname.
- A newly saved server is used on the next update check without an app restart. Before configuration exists, the installed client makes no GitHub update request.
- Existing URL validation, architecture channels, update preferences and explicit configuration tooling remain covered.
- No new dependencies, UI changes, deployment, or modification of actual user configuration.

Implementation and verification: `docs/plans/2026-09-29-desktop-update-url.md`.
