# Collaboration task boundaries

Creation and client execution remain disabled. The v4 `execution` projection
retains channel order and original group-input history; an unknown or missing
protocol must never fall back to ordinary execution.

On `unsupported_collaboration_action`, do not retry ordinary issue/comment writes,
delete, batch or rerun elsewhere. Mixed edits refuse atomically. Machine credentials
cannot borrow human/source membership for shared content or control. Ordinary
execution also rejects captured/merged collaboration sources; an ordinary child's
parent reference grants no access to private collaboration content.

Use dedicated controls: target Resume is atomic, Resume All reports channel/hold
results separately, and consent never releases a hold. Recover the original key
at `control/operations/{requestId}` without automatic retries using newer versions.
Resource withdrawal or rebinding never removes the old run's proof requirement.
Held input cannot block valid input; only complete group nonlaunch proof permits
return to waiting. Pending configuration still requires consent and Resume.
Configuration writers persist invalidation; display renames alone do not revoke.

Human source members may reorder without advancing collaboration revision. Other
edits/handoffs retain original versions and keys; stale status confirmation needs
an explicit human decision. Full contracts and source anchors are in
[working-on-issues-source-map.md](working-on-issues-source-map.md); ordinary rules below are not shared APIs.

