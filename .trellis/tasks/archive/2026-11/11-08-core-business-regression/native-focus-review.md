# Native clipboard readiness review

Independent read-only review of the final focus patch in e2e/collaboration-account-desktop.spec.ts. No runtime or test was executed by this reviewer.

BeforeSHA5fb058ef8cf9af36febdd00c23b78dbbf573a78bbb2ce283b0921ccb95c23762; afterSHA7119535b04fce5185a23b4bd602b7b1165ce5cbe9b5c1ad7e6431ccc7dc1ac1a. Root and QA sync records agree.

No assertion weakening found. The existing5second expect.poll still requires native window, webContents and document focus all true. Each readiness attempt now reactivates only the test-owned application/window, restores/shows it if necessary, focuses webContents and waits two animation frames before observation. This handles another native application taking focus during asynchronous activation.

The test-owned clipboard sentinel, exactly one real Copy link click, expected absolute URL, configured localhost origin and privacy-safe clipboard classification remain unchanged and outside the focus loop. The test does not inject the expected link into the clipboard. Focus/visibility attempts are recorded on success and failure without reading arbitrary clipboard contents into evidence.

Source review PASS; the complete11case spec is running under root/author control. Final package binding must wait for its terminal result, then permit exactly this one test-only hash/mode delta against unchanged v10 build inputs. No package rebuild is required for this helper change. Original final2 failure remains retained.


Verified terminal focus-final1/results.json:11expectedPASS,0unexpected/0skipped/0flaky, duration74282.029ms. The same final file hash is now the sole declared v11 execution delta against v10 product-build inputs.
