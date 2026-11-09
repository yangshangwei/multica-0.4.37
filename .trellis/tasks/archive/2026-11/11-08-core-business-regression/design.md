# Verification design

Use task-owned databases and production Web plus built desktop renderer. API and package test databases are separate; each DB-backed Go package gets a fresh migrated database. Use existing Playwright fixtures and synthetic actors. Keep private configuration and traces outside Git. Bind results to Git HEAD plus dirty file hashes. Diagnose both prior timing failures before modifying product or test code. Separate main browser runs from password/provider runs.
