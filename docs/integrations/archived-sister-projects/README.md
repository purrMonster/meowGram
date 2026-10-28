> Historical archive copied for owner task A.2 on 2026-10-28. These notes are not current release requirements or implementation evidence. Source: rescue/feat/tabbytally-review-integration (archive only; never merge). No code or migrations were copied. The tabbyTally repository was inaccessible, so the owner-approved neutral documentation fallback is used. R2.1 requires trigger prefixes everywhere; historical prefix-free proposals below are superseded. Any historical relative runbook/release links refer to the source branch.
# Requirements from sister projects

This folder records requirements that other projects place on meowGram, the
agreed design response, work already completed, and work planned against it.
These documents are integration handoffs, not automatic additions to a release.
Each requirement must retain its originating project, decision date, ownership,
dependencies and acceptance criteria.

## Requirements register

| Requesting project | Requirement | Status | Dependency |
| --- | --- | --- | --- |
| tabbyTally | [Financial review through a polling bot](tabbytally-review-bot.md) | Design accepted; implementation parked | UST-2.4.2 and UST-2.4.3 merged into `release/r2` |
| pourOver | [Authenticated message API and polling replies](pourover-message-api.md) | Consumer adapter reported merged; provider contract pending verification | R2 send/idempotency and bot sync; channels deferred to R2.1 proposal |

## Document convention

For each project, distinguish:

1. **Requested requirements:** outcomes and constraints confirmed by the owner.
2. **Design response:** how responsibilities are divided between the projects.
3. **Completed work:** actual changes and the limits of their verification.
4. **Planned work:** work that is not implemented, with dependencies and checks.
5. **Where we stopped:** the exact condition for resuming and actions prohibited
   while parked.

Use placeholders for infrastructure and identities. Real settings and credentials
belong in ignored host-local files. Do not copy private business data into a
requirements document. Update the project runbook when a decision changes.
