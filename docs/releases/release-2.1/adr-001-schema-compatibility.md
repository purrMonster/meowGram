# ADR 001 — fail closed on unknown database versions

Status: PROPOSED; owner decision required before migration 000007.

## Evidence and options

`server/internal/database/db.go:runMigrations` calls golang-migrate v4.18.2
`Up()` and accepts only nil/ErrNoChange. R2's actual v1.1.0 rollback harness
proved version 6 is rejected when its files are absent. A binary being newer
than another installed binary does not mean it understands a schema newer than
its own bundled migrations. Dirty state must always fail closed.

| Option | Benefit | Cost / risk |
|---|---|---|
| A. Reject ahead/dirty/unknown schemas (recommended) | No claim of compatibility without evidence; retains current safety | Recovery needs a tested version-specific procedure or restore |
| B. Allow only an explicit tested schema range/compatibility manifest | Can support selected adjacent versions | Every read/write/auth path needs compatibility proof; old v2.0.0 cannot acquire this policy retroactively |
| C. Environment switch to skip migrations when ahead | Fast operator escape hatch | Unknown constraints and unfiltered old readers can expose data; reject |

Recommendation A: retain rejection, improve sanitized version diagnostics if needed.
No generic force/reset/ignore-version setting. New migrations stay additive,
partial/repeated-up safe and non-destructive down; those properties alone do not
prove semantic compatibility. Pin the actual candidate and v2.0.0 source/image
identities in the future harness, not a mock of migration failure.

## R2.1 → v2.0.0 recovery boundary

The old `GetRecent` and `GetMessagesAfter` select all messages, without channel
authorization. Old push selects all active device tokens. Even one non-null
channel message makes old reads unsafe, regardless of archived channels, disabled
bots, absent memberships or whether a client currently displays that channel.
Bot rows themselves are compatible with v2.0.0; R2's zero-bot rule was for v1.1.0.

Before any channel message, a **candidate** keep-data recovery can retain additive
objects, stop all writers/providers, assert clean exact R2.1 schema and zero
non-null channel rows, then reset migration metadata to 6 and run v2.0.0.
This is not authorized SQL or a proven recovery procedure today. It requires:

1. Owner-approved procedure and a verified pre-upgrade backup/off-node restore.
2. Actual-binary tests of old human/bot sends, replay and all read paths against
   the new constraints/defaults, including retained memberships/audits/ntfy config.
3. Quiesced atomic preconditions; no mixed old/new servers or background workers.
4. Preserve R2-compatible lounge fingerprints for new null-channel sends;
   retain existing response bytes. No deletion or rewriting to meet the guard.
5. Forward reapply to the candidate's exact clean version preserving all rows.
   Any channel-specific idempotency row is another refusal condition.

After channel traffic, prefer roll-forward repair on R2.1 with new writes/providers
disabled. To revert to v2.0.0, restore the verified pre-upgrade backup only under
explicit owner authorization; **all post-backup changes are lost**, lounge included.
Never null out channel IDs, delete private rows or bypass the guard. Non-destructive
down migrations retain private data and therefore do not make old reads safe.
Client rollback is separate: old clients can use R2.1 lounge-only routes; private
cache storage must never be written into the legacy v1 box.

## Required proof before release

Extend `server/integration/rollback_test.go` and the rollback scripts to build
tag v2.0.0 alongside the candidate. Prove startup rejection at the new version,
safe pre-channel recovery/forward reapply, refusal with one private human or bot
message, and isolated demonstration that forced old reads would leak that fixture.
Prove restore removes post-backup changes, then old login/history/send still work.
Test dirty, partially applied, unknown and missing migration states in both timezones.
Write exact guarded owner steps only after these tests pass; no recovery claim now.
