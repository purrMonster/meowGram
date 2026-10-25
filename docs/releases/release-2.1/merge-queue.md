# R2.1 merge queue

Only the owner merges. Product/process decisions from the review are APPROVED;
this revision still needs review and does not authorize implementation.

1. #51 docs/r2.1-plan → main | after merged #50 | revised lounge + groups plan,
   exact API/reuse/member-sync/mute/pourOver contract and verification gates.
   Draft: https://github.com/purrMonster/meowGram/pull/51

STOP after pushing this revision and updating #51. Create release/r2.1 and declared
epic/task layers only after explicit implementation authorization. No new PR.
R2 publication queue entry is complete (#50); media remains outside R2.1.

## Fast-track implementation — 2026-10-25 (epic 4)

The owner authorized implementation after #51 and explicitly allowed the ntfy
spike with a recommended pinned upstream version while O2 remains pending; the
planning-only STOP above is historical and superseded by that authorization.

- #57 docs/ust-2.1.4.1-ntfy-contract → feat/r2.1-epic-4-ntfy | merge after nothing |
  independent UST-2.1.4.1 spike, unblocks 4.2 once epic 2 provider contract is
  integrated and synced. https://github.com/purrMonster/meowGram/pull/57
  UTC/IST: Go 166 passes + 3 existing fixture skips each; ntfy 13 checks each;
  vet and restore pass. O2 inventory/O3 real setup/O4 phone/O5 controls pending.
  No mandatory Claude gate for this task. Owner merges only.
