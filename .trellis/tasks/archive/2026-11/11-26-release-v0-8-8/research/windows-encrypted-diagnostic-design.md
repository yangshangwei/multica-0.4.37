# Encrypted Windows startup failure evidence

## Why needed

Run45694785915 confirms a nonempty bounded daemon.err.log but no allowlisted prefix/IO classification. Existing classification cannot determine the cause; avoid another speculative code change. Private keys remain only on the parent-owned local filesystem (0600), never read by this implementer or passed to CI. Only the public key enters workflow input.

## Minimum collection

No controller or installed flow change is required: after failure, the Windows driver can read the existing daemon.err.log below validated plan.home/.multica/profiles/<derived desktop host profile>. Require exact realpath/ordinary file within the owned phase directory; read at most the final64KiB through a bounded read. Do not follow symbolic/reparse links or accept paths from report attachments.

Add at most16KiB of parsed Playwright failed-result error.message/stack plus16KiB of subprocess stderr. Do not include full stdout JSON, request/response records, source environments, attachments, screenshots, config, keys or arbitrary home files. These error fields may contain private values; encrypt the entire payload in memory before inserting anything into the public report. The original local runner log remains private.

## Envelope

Optional workflow_dispatch input diagnostic_public_key, passed as a structured job environment value. Accept base64DER SPKI PUBLIC KEY only, reject private-key formats and RSA modulus <3072. Use Node built-in crypto: fresh32-byte AES key,12-byte nonce, AES-256-GCM; wrap key with RSA-OAEP-SHA256. Bind envelope schema version, full source SHA and phase via authenticated additional data. Public envelope contains fixed algorithm/version metadata, publickeySHA256 fingerprint, AAD, wrappedkey, IV, tag and ciphertext. Never put raw bytes/messages/errors outside the envelope. No dependency added.

Only on failure and with a valid supplied public key. Absent key keeps current behavior; invalid key, unexpected file or encryption error produces fixed failure/unavailable status, never plaintext fallback. The existing public report/asset glob carries encryptedDiagnostic; no expanded filesystem upload.

## Files and validation

- .github/workflows/desktop-smoke.yml: one optional string input and jobenv.
- apps/desktop/scripts/windows-collaboration.mjs: failure-only collection/sealing integration.
- one Node crypto helper plus existing windows-collaboration-config.node-test.mjs behavioral tests.
- Tests: decrypt exact bytes with ephemeral test key; wrong key and tampered ciphertext/tag/AAD rejected; malformed/absent/weak keys; bounded read and owned-path refusal; no plaintext in public JSON. Fixture-generated test private keys stay in test memory.

Parent approved the design before implementation; independent implementation review is required before commit. No product changes, startup retry, identity writes, timeout increase or skipped native acceptance.

## Implementation and local verification

The driver reads the existing owned daemon.err.log directly after failure; no extra controller/rawattachment path is needed. lstat/realpath enforce the exact ownedhome and fixed profile filename, file identity is rechecked after open, and only64KiBtail is read. Endpoint must be the validated loopbackAPI shape. Missing/rejected capture is recorded only inside the encrypted payload. No key returnsundefined without touching paths; badkey/context returns fixedstatus with no fallback.

Payload contains only rawerrlog(base64), failed-result message/stack bounded16KiB and stderr bounded16KiB. Full stdoutreport/attachments/config/environment are excluded. Envelope uses fresh AES256GCM key/nonce and RSAOAEP-SHA256(publicRSA>=3072), with schema/sourceSHA/phaseAAD and recipientSPKIsha256. The test keypair is generated in memory; no local productionprivatekey or publickey was read by this implementer.

Tests use real temporary files, owned/outside/symlinkpaths, exacttailcomparison, privatekey/wrongrecipient/weakkey/invalidkey, ciphertext/tag/source/phase tampering, independent random envelopes, absentkey, and no plaintext in publicJSON. Initial targeted encrypted test passed; all related config/workflow/version/CI-contract suites31PASS/0FAIL/0SKIP. Syntax/diffchecks passed. Windows realpath/junction/encryption execution still requires nativeCI; no native or release success claim. No dependency, product, restart, timeout or uploadglob change. No commit/push by this agent.

## Independent review adjustment

The16KiB string-tail helper now advances past UTF8 continuation bytes when trimming, preventing replacement-character expansion beyond the byte cap. Decrypting Chinese/emoji cases checks both controllerfailure and stderr are <=16384UTF8bytes and contain no injected replacement character. DER-SPKI-only remains the supportedkeyformat; noPEMcompatibility. Narrow encrypted test rerun passes; no other behavior expansion.
