# Ordinary comment upload decision

The user explicitly selected “保留成员可上传，上传者独立归属” in the current chat.

Current same-workspace member append semantics are intentional. Generic upload does not inherit body-edit/delete author/admin restrictions, does not change the original comment author/body, and retains independent uploader provenance. Membership, workspace association, private collaboration-flow gates and failed-upload no-side-effect guarantees remain.

No permission implementation change is warranted. Add a focused handler contract regression for another member appending, unchanged author/body, actual uploader and denied foreign/nonmember cases, using a fresh owned DB; retain earlier observation as a now-decided product boundary.
