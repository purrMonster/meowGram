# R2.1 API contract freeze

UST-2.1.1.1 fixtures: [HTTP matrix](../../../server/contracts/r21/http.json),
[execution and seed contract](../../../server/contracts/r21/README.md).
The approved [design](channels-proposal.md) remains authoritative for later
concurrency, persistence, pagination, revocation and recovery implementation.

The 70 cases freeze method, route, raw body/query, actor and expected status.
Legacy selector omission stays lounge-only. Conflicting/repeated parameters,
literal null and malformed UUIDs reject with 400. Unknown and inaccessible groups
use identical 403 bodies. Sync fields are explicitly named:
`channel_id`, `conversation_member_count`, human-only `conversation_muted`,
`X-Membership-Version` on member pages including empty pages, and `X-Has-More`.

Default fixture integrity tests run in existing Go suites. Tagged `r21_contracts`
HTTP replay is pending implementation and expected to fail against R2.0;
it is not enabled in CI. A compile check is not an HTTP compliance pass.
See the linked README for the per-case disposable reset/actor/seed adapter.
No production endpoint or migration is added by this story.
