# 2026-10-21 queue refresh

#33 is merged at release/r2 0364c75. Published #34 (UST-2.3.4) and #35
(stacked identity runbook fold), both targeting epic 2.3. This release-wide docs
branch starts from origin/release/r2 and changes only the queue and this log.

No code imported across epics. Preserve the SenderKind sync regression and R3
owner-check/roadmap merge requirements. Application test evidence belongs to
#34/#35; no new application verification claimed for this documentation-only
release branch. git diff --check passed. AGENTS/playbook untouched.

Where we stopped / next step: DONE; owner merges #34 then #35, then integrated identity epic verification/release PR and message API sync can proceed.
