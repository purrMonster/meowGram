# R2.1 merge queue

UST-2.1.1.4 published 2026-10-28:
- [#59](https://github.com/purrMonster/meowGram/pull/59)
  feat/ust-2.1.1.4-message-api → feat/r2.1-epic-1-channels | stacked on #58 |
  merge after #58 and Claude review; completes epic1 task work, then doc-fold.
  UTC533 / IST533, contracts78+actual-tag replay per timezone; see October28 log.

Only the owner merges. Updated 2026-10-27 for approved fast-track sequencing.

1. [#58](https://github.com/purrMonster/meowGram/pull/58)
   feat/ust-2.1.1.3-conversations → feat/r2.1-epic-1-channels | stacked on none |
   merge after Claude review; unblocks UST-2.1.1.4 (already stacked locally).

Completed prerequisites, verified before branching:
- #51 approved plan → main.
- #52 kickoff → release/r2.1 (d881890); its release-level branch map remains authoritative.
- #53 frozen contracts → feat/r2.1-epic-1-channels (3c03772).
- #55 PROJECT.md → main (76f722a); no main/sibling commits imported into this task.
- #56 schema → feat/r2.1-epic-1-channels (7da19e4); merged parent incorporated
  into #58 without rebasing at 9e80d67, with identical source tree.
- #57 ntfy spike → feat/r2.1-epic-4-ntfy (479b5a5); publisher still waits for
  epic 2's provider interface through release/r2.1 and the owner-merged sync.

UST-2.1.1.3 starts from #56 at 84493e4 under owner-approved same-epic stacking;
its PR targets feat/r2.1-epic-1-channels and requires Claude review before merge.
The #56 parent sync is complete; fresh verification is recorded in the
[2026-10-27 log](log/2026-10-27-UST-2.1.1.3-parent-sync.md).
No implementation commits on integration branches.
