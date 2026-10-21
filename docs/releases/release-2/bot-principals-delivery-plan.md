# UST-2.3.3: bot principals and sender identity

Updated 2026-10-17. Parent and PR target:
`feat/ust-2.3.3-bot-principals` -> `feat/r2-epic-2.3-bot-identity` -> `release/r2`.
The owner deferred merges. This independent task starts at its epic parent;
neither the directory investigation nor shared-service sibling is merged here.
The preserved investigation and R2 planning are committed on their original
`feat/ust-2.3.1-directory-identity` branch. The recovery stash remains intact.

Migration 000005 adds bot profiles, provider/namespace/UUID identity mappings
and a nullable bot sender FK on messages. Exactly one human/bot sender is required.
Existing subjects, message UUIDs, content and timestamps are unchanged. The down
migration preserves data and schema; it is not a compatibility rollback. Once
bot messages exist, old server binaries cannot read null legacy sender columns.
Disable bot access on the compatible binary; do not deploy an old binary or run
destructive cleanup as rollback. Live migration execution requires separate
deployment authorization. Constraint validation takes a table lock; measure it
against representative data before deployment.

`BotRepository.Resolve` requires an already verified directory UUID and namespace.
It serializes concurrent provisioning, preserves identity across display-name
changes, and assigns new principals for different directory UUIDs or namespaces.
It neither authenticates nor grants access. Every future bot request still needs
fresh password and group checks. No credential or membership cache is stored.
Directory revocation must not delete local profiles or messages.

Wire messages use `sender_kind: human|bot`; bot sender IDs are local UUIDs.
Legacy cached messages without kind default to human. Unknown kinds are never
treated as self. History/sync query both identity sources. Flutter retains kind
through cache serialization, labels bot bubbles, and uses kind plus ID for self
styling and foreground/background notification suppression. Update clients before
enabling bots; older clients cannot distinguish deliberately colliding IDs.

Integration note: after the owner merges the epic dependencies, update the
shared message service to propagate `SenderKind` and support the bot repository
path before wiring HTTP sends. This task deliberately adds no bot endpoint.
The existing WebSocket adapter propagates kind for live human messages and all
history records. Group-gated authentication remains UST-2.3.2.

Verification includes populated pre-migration PostgreSQL, concurrent mapping,
same-name recreation, namespace separation, human/bot ID collisions, constraints,
history retention after mapping removal and non-destructive down behavior. Client
tests cover cache round trips, peer styling and background notification collisions.
