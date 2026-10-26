# R2.1 merge queue

Only the owner merges. Updated 2026-10-26 for approved fast-track sequencing.

1. [#56](https://github.com/purrMonster/meowGram/pull/56)
   feat/ust-2.1.1.2-schema → feat/r2.1-epic-1-channels | merge after nothing |
   unblocks UST-2.1.1.3; additive schema and passing migration/regression evidence.
2. [#58](https://github.com/purrMonster/meowGram/pull/58)
   feat/ust-2.1.1.3-conversations → feat/r2.1-epic-1-channels | stacked on #56 |
   merge after #56 and Claude review; unblocks UST-2.1.1.4 (may stack immediately).
3. [#57](https://github.com/purrMonster/meowGram/pull/57)
   docs/ust-2.1.4.1-ntfy-contract → feat/r2.1-epic-4-ntfy | merge after nothing |
   independent pinned provider spike; publisher still waits for epic 2 via release.

Completed prerequisites, verified before branching:
- #51 approved plan → main.
- #52 kickoff → release/r2.1 (d881890); its release-level branch map remains authoritative.
- #53 frozen contracts → feat/r2.1-epic-1-channels (3c03772).
- #55 PROJECT.md → main (76f722a); no main/sibling commits imported into this task.

UST-2.1.1.3 starts from #56 at 84493e4 under owner-approved same-epic stacking;
its PR targets feat/r2.1-epic-1-channels and requires Claude review before merge.
After #56 merges, merge the updated epic into the task (no rebase/force), verify,
then push. No implementation commits on integration branches.
