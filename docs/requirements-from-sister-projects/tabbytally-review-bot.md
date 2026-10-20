# tabbyTally — financial review through a meowGram polling bot

| Document field | Value |
| --- | --- |
| Requesting project | tabbyTally, Phase 2.3 review and corrections |
| Receiving project | meowGram |
| Decision date | 21 October 2026 |
| Status | Design accepted; implementation parked; not deployed |
| Working branch | `feat/tabbytally-review-integration` |
| Release dependency | UST-2.4.2 and UST-2.4.3 merged into `release/r2` |
| Story / PR | No new R2 story assigned; no PR opened for this integration |
| Decision authority | Project owner's instructions in the integration discussion |

## 1. Purpose and scope

tabbyTally needs a way to notify reviewers about items and consume their decisions
without publishing financial details in chat. It will use meowGram as a bot,
following the post-and-poll interaction pattern used by pourOver.

This document captures a sister project's requirements. It does not expand the
approved R2 scope or claim that pourOver's implementation has been inspected or
reused. meowGram supplies messaging and verified identity; tabbyTally owns financial
validation, review state and application of decisions. Actual Budget writes remain
outside this Phase 2 integration.

## 2. Confirmed requirements

| ID | Requirement from tabbyTally | Intended responsibility |
| --- | --- | --- |
| TT-01 | Post review prompts, poll sync for replies, then act locally. | tabbyTally consumes the merged meowGram send/sync APIs. |
| TT-02 | Use R2 bot Basic authentication. No new inbound callback, hostname or exposed review service. | meowGram provides the R2 bot API; tabbyTally initiates requests. |
| TT-03 | Authorize reviewers through an LLDAP reviewer group. | Integration verifies the reply author's eligibility using a trusted identity/group mechanism; exact mechanism follows R2 contract review. |
| TT-04 | Keep the individual verified subject in the audit trail only, not a per-person authorization allowlist. | tabbyTally records the human actor separately from the bot identity. |
| TT-05 | Chat content is references only: item IDs and counts. Amounts, merchants and financial facts stay in tabbyTally. | tabbyTally controls prompt, reply-processing, acknowledgment and error content. |
| TT-06 | Use placeholders only during preparation; real configuration comes from ignored host-local files. | Both projects preserve local configuration and secret boundaries. |
| TT-07 | Do not build the integration before both specified R2 stories are merged into `release/r2`. | Integration work remains parked until the dependency gate is satisfied. |

## 3. Agreed design response

The intended flow is:

1. tabbyTally records an item requiring review and posts a reference-only prompt.
2. A human replies through meowGram; tabbyTally retrieves replies through sync.
3. tabbyTally identifies the actual reply author from trusted message metadata and
   verifies that author's reviewer-group eligibility. The bot's credentials do not
   grant reviewer authority to the author of an arbitrary message.
4. tabbyTally matches the reply to an item/revision, validates the action, applies
   it locally, and records the individual subject in its audit trail.
5. Any chat acknowledgment contains references/status only, with no financial facts.

Persist the sync cursor and reply/decision receipts so restarts and repeated polls
cannot apply a reply twice. Preserve stale-revision protection and original
evidence. Do not trust subjects, group names or membership claims typed into chat.
If identity or authorization cannot be verified, leave the item unresolved.

The exact reply syntax, polling interval and trusted group-lookup contract are
implementation details to settle after inspecting the merged R2 APIs. No new
endpoint or claim format is prescribed by this document.

## 4. Work completed and its current relevance

| Completed work | Evidence | Relevance to the accepted design |
| --- | --- | --- |
| Isolated integration branch and local dependency merges | Existing bot-auth, bot-principal and shared-message-service branches integrated locally. | Retained for inspection; no owner PR merge or deployment implied. |
| Durable send and signed callback prototype | Implementation commit `cffda0d`; replay, concurrency, identity and restore tests recorded in the runbook. | Callback design is superseded. Do not deploy it as the accepted integration. |
| Review hardening | Endpoint collision validation and retryable callback throttling; targeted Go config/handler race tests and 95 processing tests passed. | Existing prototype fixes only; they do not establish polling or group-authorization correctness. |
| Design and parking record | Decision recorded in both project runbooks; meowGram parking commit `931adc6`. | Preserves the owner's selected architecture and dependency gate. |

Earlier verification also passed the full Go race suite/vet, 83 Flutter tests,
release web build, scratch restore, and a Go-to-Python callback test. These are
historical checks of existing code, **not acceptance tests of the polling design**.
No polling bot or reviewer-group integration has been implemented. The prototype's
Basic-auth gate covers sending, not bot sync.

## 5. Planned work after the dependency gate

1. Verify that both UST-2.4.2 and UST-2.4.3 are merged into `release/r2`; inspect
   their actual send, sync, pagination, identity and retry contracts.
2. Reconcile this branch with those merged contracts. Replace the superseded
   callback approach rather than enabling it alongside the polling design.
3. Define how a verified message author is checked against the LLDAP reviewer
   group, including membership changes and unavailable identity services.
4. Implement reference-only prompts and reply handling in tabbyTally with durable
   cursor storage, reply receipts, revision checks and individual audit actors.
5. Run the acceptance checks below before proposing any rollout. Live configuration,
   credentials and deployment remain separate approval steps.

## 6. Planned acceptance checks

| Requirement | Check that must pass |
| --- | --- |
| TT-01 / TT-02 | A bot posts a prompt and consumes a human reply using the merged R2 send/sync APIs without an inbound callback listener or new hostname. |
| TT-03 / TT-04 | An eligible member's action records that person's verified subject. Nonmembers, revoked members and forged author/group claims cannot apply decisions. Bot identity is never substituted for human authorship. |
| TT-05 | Prompts, acknowledgments and error paths contain references/status only; amounts, merchants and source email content are absent. |
| TT-06 | Tracked configuration contains placeholders; real settings and secrets are excluded from Git and routine logs. |
| Recovery | Duplicate replies, page replay and a crash before cursor advancement do not repeat an applied action. Cursor handling does not skip an unapplied reply. |
| Review consistency | Stale revisions and conflicting replies do not overwrite newer decisions; unavailable authorization leaves the item unresolved. |

## 7. Configuration inventory — placeholders only

```dotenv
MEOWGRAM_BASE_URL=
MEOWGRAM_SEND_ENDPOINT=
MEOWGRAM_SYNC_ENDPOINT=
MEOWGRAM_USERNAME_FILE=
MEOWGRAM_PASSWORD_FILE=
LLDAP_REVIEWER_GROUP=
```

These are design placeholders, not currently supported runtime settings. Final
names follow the merged R2 contracts. Real values belong in an ignored host-local
env file and secrets in protected local files. No callback hostname,
`OWNER_OIDC_SUB`, callback signing key or inbound port is required by this design.

## 8. Where we stopped

**Parked. Do not build or resume integration until both UST-2.4.2 and UST-2.4.3
are merged into `release/r2`.** The previous prototype remains undeployed and
superseded. This documentation reorganization does not resume implementation.

Existing tests were green at parking; this change only reorganizes documentation.
No credentials, DNS, deployment or live services have been changed. Once the gate
is satisfied, begin with the contract review in section 5, not with callback setup.

See the [meowGram runbook](../../runbook.md) for the implementation and verification
history and the [sister-project requirements register](README.md) for document scope.
