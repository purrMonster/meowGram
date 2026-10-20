# tabbyTally review bot — decision and parking note

21 October 2026. Branch: `feat/tabbytally-review-integration`.

## Accepted design

- tabbyTally posts review prompts as a meowGram bot, polls message sync for replies,
  then applies authorized decisions locally. Follow the same interaction pattern
  as pourOver, using R2 bot Basic authentication.
- No inbound callback, new hostname or exposed review service. The existing
  callback prototype on this branch is superseded and must not be deployed.
- Authorization comes from an LLDAP reviewer group. Keep the individual verified
  subject only as the audit actor; do not use a per-person subject allowlist as the
  authorization policy. Bot credentials authenticate the polling process, not the
  human who authored a reply. Verify the reply author's group eligibility before
  applying a decision; never trust a subject/group typed into message text.
- Chat contains references only: item IDs and counts. Amounts, merchants and other
  financial facts remain in tabbyTally. No financial source content in prompts,
  replies, errors or acknowledgments.
- Persist the sync cursor and reply/decision receipts so restarts and repeated
  polling cannot apply an action twice. Preserve revision checks and audit history.

## Placeholders only

```dotenv
MEOWGRAM_BASE_URL=
MEOWGRAM_SEND_ENDPOINT=
MEOWGRAM_SYNC_ENDPOINT=
MEOWGRAM_USERNAME_FILE=
MEOWGRAM_PASSWORD_FILE=
LLDAP_REVIEWER_GROUP=
```

These are design placeholders, not a supported runtime configuration. Final names
and contracts must follow the merged R2 implementation. Real values belong in an
ignored host-local env file; secrets stay in host-local files. No callback hostname,
OWNER_OIDC_SUB, signing key or inbound port is required by the accepted design.

## Where we stopped

The branch contains the earlier durable-send/callback prototype and dependency
merges. Existing code is retained for traceability, not approved for deployment.
The prototype's Basic-auth gate currently covers sending, not bot sync; it is not
the accepted polling implementation. No polling bot or reviewer-group integration
has been built. The prior callback tests verify the prototype only.

Review fixes reject endpoint collisions and return retryable callback throttling;
the corresponding Go config/handler race tests and all 95 tabbyTally tests passed.
The earlier complete Go/vet, 83 Flutter tests/web build and scratch restore results
are recorded in the runbook. No credentials, DNS or live services were changed.

**Parked. Do not build or resume integration until both meowGram UST-2.4.2 and
UST-2.4.3 are merged into `release/r2`.** Then inspect the merged send/sync and
identity contracts, replace the superseded callback approach with the polling
design, and define tests for group revocation, forged reply identities, duplicate
replies, stale revisions and references-only content before rollout.
