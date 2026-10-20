# 2026-10-20 owner-approved release split

owner: "R2 = authenticated message API only: epics 2.3 (bot identity), 2.4 (message API) and 2.5 (verification/release, scoped to the API). Goal: pourOver live in the shared lounge."
owner: "R2.1 = channels (new). Planning only for now."
owner: "R3 = Immich media: epics 2.1 and 2.2 move here, unchanged. #32's contract stands."
owner: "Don't edit AGENT_PLAYBOOK.md or AGENTS.md. If either names media as R2 scope, list the exact proposed wording in the PR body for my approval."
owner: "Credit rule: don't start a task you can't finish or cleanly park this session (tests green, branch pushed, story log 'Where we stopped')."

decision: Step 1 only. Branch docs/r2-scope-split from fetched origin/release/r2
30fa40c. The base lacks README, delivery-plan and api-design because they have
not yet arrived through the identity epic. Establish the updated release-wide
documents here; do not import sibling implementation. Future epic integration
must preserve this approved scope and contract while retaining earlier evidence.

- Scope commit aa3a66e: API-only R2, stable-username task before identity epic
  integration, pourOver handler-level acceptance, R2.1 proposal and R3 deferrals.
- Published media branches are unchanged; no release/r3 created and no media
  epic merged into release/r2. #32 contract remains at its historical path.
- Existing ephemeral-media roadmap remains as unassigned historical backlog;
  it is not included in the newly approved R3 Immich scope. Media story IDs and
  endpoint/placeholder-host corrections are preserved.
- R2 estimate: base 6.5–12 eight-hour days; 25% contingency rounded up to half-days
  gives 8.5–15 days. Separate owner pilot/config budget 1–2 hours. Channels is a
  proposal only: 7–12 days base / 9–15 with contingency. No next task started.
- Proposed playbook text is in the PR body for approval, not applied. AGENTS.md
  assigns no media release scope in this base, so no edit proposed there.

## Verification in this session

- Relative Markdown links and roadmap IDs/anchors passed; no duplicate IDs.
- git diff --check passed. AGENTS.md and AGENT_PLAYBOOK.md unchanged against
  origin/release/r2; only documentation/roadmap files changed.
- Existing scratch harness: meowgram-r2-checks.ps1 -Story scope-split -Project
  meowgram-r2-scope-split; deploy/verify.compose.yml with real PostgreSQL.
- Go -race -count=1: UTC 73 test/subtest passes, IST 73 passes; zero skips;
  go vet ./... passed in both. These are the release base counts, not claims
  that the separate epic implementations are already integrated.
- Backup/restore drill passed (probe and message count match).
- Flutter: No issues found! (ran in 9.9s); 00:05 +81: All tests passed!;
  Built build/web. Web runtime passed: non-root, SPA fallback, no-store HTML
  and bootstrap asset. Logs: local Temp/meowgram-scope-split-*.log.
- Live reader/proxy/pilot checks not run; explicitly listed in owner-checks.md.
  No code, migrations, live configuration or credentials changed. Unrelated
  untracked requirement list/ left untouched.

Where we stopped / next step: step 1 DONE; push and open the scope PR into release/r2, record its queue number, then park. Next implementation session starts UST-2.3.4 on the fetched bot-identity epic; instruction-file proposals await owner approval.
