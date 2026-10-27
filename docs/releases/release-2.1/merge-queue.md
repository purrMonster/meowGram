# R2.1 merge queue

Only the owner merges. Updated 2026-10-28 under approved fast-track sequencing.

1. [#58](https://github.com/purrMonster/meowGram/pull/58)
   feat/ust-2.1.1.3-conversations → feat/r2.1-epic-1-channels |
   stacked on none | Claude review required | unblocks scoped messaging.
2. [#59](https://github.com/purrMonster/meowGram/pull/59)
   feat/ust-2.1.1.4-message-api → feat/r2.1-epic-1-channels |
   stacked on #58 | merge after #58 and Claude review | closes epic 1 tasks.

Independent verification scaffolding: test/r2.1-verification-scaffold targets
test/r2.1-epic-5-verification, starts from its remote 1f39e07, and imports no
unmerged implementation. Its actual-tag decoder probe is preparatory only;
UST-2.1.5.1 and 5.2 remain blocked on owner-integrated epics 1–4.

Completed prerequisites:
- #51 plan → main; #52 kickoff → release/r2.1.
- #53 frozen contracts → epic 1; #55 project card → main.
- #56 schema → epic 1 at 7da19e4; #58 incorporated the parent without rebasing.
- #57 ntfy spike → epic 4 at 479b5a5; publisher still waits for epic 2's provider
  contract through release/r2.1 and the owner-merged sync.

After task merges, open the epic doc fold, epic → release and independent
release → next-epic sync PRs. No implementation commits on integration branches.
