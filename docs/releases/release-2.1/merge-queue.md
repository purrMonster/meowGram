# R2.1 merge queue

Only the owner merges. #51 is merged into main. P0.4 containerised baseline passed
on main 1f39e07; see log/2026-10-23-P0-kickoff.md. Host SDKs are not required.

1. #52 docs/r2.1-kickoff → release/r2.1 | merge after nothing |
   records baseline, archive restriction and branch hierarchy.

## Published integration branch map

All six refs initially published at 1f39e072211b1b441c1421e70542ee20a6370af5.
No direct integration commits or subsequent agent pushes permitted.

| Branch | Parent / PR target |
|---|---|
| release/r2.1 | main |
| feat/r2.1-epic-1-channels | release/r2.1 |
| feat/r2.1-epic-2-delivery | release/r2.1 |
| feat/r2.1-epic-3-client | release/r2.1 |
| feat/r2.1-epic-4-ntfy | release/r2.1 |
| test/r2.1-epic-5-verification | release/r2.1 |
| docs/r2.1-kickoff | release/r2.1 |
| docs/ust-2.1.1.1-contracts (next) | feat/r2.1-epic-1-channels |

The owner's kickoff naming supersedes the earlier epic-1-conversations name.
rescue/feat/tabbytally-review-integration is archive-only, NEVER a merge/PR source.
