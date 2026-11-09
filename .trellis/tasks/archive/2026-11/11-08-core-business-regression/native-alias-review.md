# Native API alias review

Read-only review of `packaged/native-diagnostic/alias-run.py`, `alias.cjs`, the byte-forwarding canary, diagnostic recorder, production profile derivation and actual completed run records. No API call, runtime test, application or process control was performed by this reviewer.

The wrapper affects only native packaged Playwright invocation. An owned TCP listener forwards localhost19080 to the same owned API18953 without rewriting HTTP method/path/body/headers or WebSocket bytes. Child API/TestApiClient/native WebSocket origins change together; production Web remains on its original API. Its real password cookies remain localhost-scoped across ports. Original Web/renderer Origin values remain allowed, and native same-origin WebSocket validation sees the preserved alias Host. The production profile formula derives19594 for `desktop-localhost-19080`, avoiding the independently observed foreign19602 listener without stopping it.

The author-owned canary covers HTTP POST method/query/body and exact upgraded WebSocket payload. The proxy itself has no application response logic, records its exclusive loopback listener, owns accepted/upstream sockets, and closes them on SIGTERM. Wrapper cleanup stops only the spawned proxy process group after the child has settled; it preserves any primary failure. The underlying reviewed packaged controller still owns exact source/payload guards, private credentials/profile, provider restrictions, protocol restoration and original API restoration.

Actual `native-alias-1` evidence now records identical `/health` PID41583/commit6201a5810/starttime through direct and alias origins, matching public configuration/deployment, proxyPID41682, unchanged Web13873 and hashes for wrapper/proxy/recorder/controller. Playwright's actual full Main case passed in28.4s, exit0, zero retries; proxy cleanup reports terminated=true/errors=[]/exit0; package cleanup records exit0/no original failure/no cleanup errors. This review does not replace root's final daemon/port/source audit.

Reviewed hashes:

- Wrapper: `b9d7ecc14b5ef5e3930d1cb1a358ab6811086c455f76fc1e91aa7e3101e82625`.
- TCP proxy: `4efcd0b1ecea89559346d2d142510aa9cd29668ac5b43d57cc21719b8e0a59f1`.
- Recorder: `49781a1533a5240928034b9b707b1580463c6a6f7f099c89f77b2a36d59597bb`.
- Controller: `e99509cfbb0b740e14675d8f34cf94cfee93788240d67f62a61f4d53e1137722`.

No blocking issue found. The original collision failure remains failed; this separately numbered result proves the isolated alias configuration. It is not a product change to daemon port allocation or a claim that arbitrary shared-machine health-port collisions are solved globally.
