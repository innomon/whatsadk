# Track: Verification Database Auto-Whitelisting & Blacklist Explicit Rejection

- **Track ID**: `verification_whitelist_and_blacklist_rejection_20261104`
- **Specification**: [spec.md](spec.md)
- **Plan**: [plan.md](plan.md)
- **Metadata**: [metadata.json](metadata.json)

## Scope
1. Introduce persistent `whitelisted_numbers` store support across SQLite-P2P, PostgreSQL, and SurrealDB.
2. Auto-whitelist successfully verified phone numbers in the database.
3. Check dynamic database whitelisting in `isUserAllowed`.
4. Send an explicit rejection message (`config.messages.blacklisted`) to blacklisted users attempting verification instead of silently dropping the message.
