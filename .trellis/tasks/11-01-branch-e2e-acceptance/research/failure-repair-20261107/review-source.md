# Full repair source review

Eight source/test files and two spec updates: **PASS**, no blocking source findings. Final execution and cleanup still pending.

The scroll fix changes only account-detail native anchoring, matching that container's existing explicit numeric memento. It introduces no global scroll behavior, retry timer or repeated forcing. The regression gates a real successful GET until cached-error rendering and620 restoration have happened, then waits for actual error removal and painted frames before checking620. Existing list, route, explicit anchor, responsive, locale and native assertions remain.

The fault response preserves real-response headers/CORS. Pending route work is released and awaited before removing the exact handler, including failure cleanup. The earlier exact-glob fixture interruption is retained separately.

The two corrected baseline global tests fail before the complete new recovery assertion (620→660/580); exact144px620→476 failure after error disappearance is established in the separate private diagnosis. Final reporting must keep these evidence scopes distinct.

Localized Skill defaults and diagnostics are approved as detailed in `review-unit.md`. Specs describe the local scroll policy's tradeoff and exact shipped-default provenance. Final acceptance requires original Web/Desktop detail journeys plus the strengthened scope journeys and relevant lint/type/unit checks. Hashes are in `review-source.json`.
