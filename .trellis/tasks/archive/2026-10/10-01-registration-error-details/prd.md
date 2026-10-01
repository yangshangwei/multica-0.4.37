# Specific registration errors

## Requirements and acceptance

- Explain username, name and password failures with the violated rule and actual character count where useful.
- Recognize known validation messages from older backends that return `invalid_request`; never show raw English server errors.
- Preserve numeric username support, leading zeros, rate-limit countdowns, and uncertain-registration handling.
- Verify and update the checkout-owned API on port 18573, which is still running the old leading-letter username rule.
- Cover modern and legacy responses, Chinese rendering, English translations, malformed bodies and unknown errors.

## Implementation plan

1. Capture the live old-backend response and add failing tests.
2. Move the existing error-message selection into a small presentation helper with explicit API-boundary compatibility mapping. Use the submitted field values only to explain the server validation failure.
3. Add detailed bilingual messages and connect the helper to the shared form without altering registration flow or layout.
4. Run focused tests, translation parity, typecheck and lint; restart only the verified checkout-owned API and probe invalid requests without creating accounts.
