# Native acceptance evidence

The root of this directory contains the **final fixed build** evidence.
`pre-fix/` contains the initial fresh build and its layout matrix; the separate
focus diagnostic JSON/logs explain why that build was not accepted.

`build-provenance.json` records the base revision, uncommitted source patch,
14 source hashes, exact build command and generated artifact hashes.
`verified-source-patch.json` contains that narrowly scoped patch. The runtime files
record which built artifacts were actually served. Temporary builds/profiles
were removed after their hashes were checked.

The audit scripts use existing repository dependencies:

- `electron-main.cjs`: isolated native main-process fixture. Loads the actual
  built preload, blocks non-read requests and disables agent/update actions.
- `launch.mjs`: serves built assets, copies a local development profile into a
  disposable profile, and starts the actual Electron renderer. Credentials stay
  in memory/the disposable profile and are never written to evidence.
- `probe.mjs`: connects to that owned native instance and executes the supplied
  audit script. Locale fixtures affect only the copied renderer session.
- `matrix.js`: captures Chinese/English, both themes, wide/compact/narrow and
  real 200% native zoom; measures actual geometry and stable sRGB contrast.
- `interactions.js`: checks both return origins, settled connected-node focus,
  other focus owners, refetch and actual synthetic touch.
- `capture-touch.js`: recaptures the four dialog images after opening animations
  finish. `interactions.js` includes the same settled-capture condition.

For a repeat run, regenerate a fresh build/provenance for the intended source
before using the fixture. The recorded run used the existing local API at
18572, allowed renderer origin 13492, and owned control/CDP ports 9478/9477.
With those prerequisites verified, the invocation shape is:

```sh
node <evidence-dir>/launch.mjs
node <evidence-dir>/probe.mjs < <evidence-dir>/interactions.js
node <evidence-dir>/probe.mjs < <evidence-dir>/matrix.js
curl -fsS http://127.0.0.1:9478/close -o /dev/null
```

These are bounded acceptance fixtures, not application entry points. The
canonical behavioral regression remains beside the production template page.
