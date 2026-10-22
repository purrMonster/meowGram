# Epic 2.5 parent sync — 2026-10-22

Verified owner merges #42 c41a1d5 and #43 e74a1de. Created
chore/r2-verification-parent-sync from origin/test/r2-epic-2.5-verification
0e41333, then merged origin/release/r2 e74a1de without rebase or conflicts.
Merge checkpoint f3b3112. Application tree is unchanged from the release parent.

owner: cellar will run GHCR images instead of building locally; UST-2.5.2 must
use pinned required MEOWGRAM_VERSION, explicit local build override, unchanged
verify.compose.yml, .env.example 2.0.0 placeholder and release pubspec 2.0.0+4.
Full owner decision captured in ../ghcr-rollout-decision.md, including backup,
off-node restore validation, pre-r2 image tags, pull/up, smoke, digests, rollback
only before any bot message with Basic disabled, and public/private package owner
check gated on actual image layers/environment inspection. No image publication,
package visibility change, credential or live operation performed.

Inspection: release.yml already compares tag v<semantic pubspec version>; current
pubspec is 1.1.0+3. verify_restore.sh tests only its synthetic /verification DB;
UST-2.5.2 must not label that a production-dump restore verification. db_backup.sh
runs continuously, so document completed-dump selection. These are implementation
requirements for 2.5.2, not extra live actions.
Where we stopped / next step: parent-sync full scratch harness running; publish owner PR after green, then STOP before 2.5 implementation.
