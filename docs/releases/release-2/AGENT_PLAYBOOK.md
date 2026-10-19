# meowGram Release 2: agent build playbook

Owner: Roastmaster (jyotirmoyc). Written 2026-10-18.
Audience: any coding agent (Codex/GPT) carrying R2 from its current state to a
release candidate. Read this whole file before every session. It sits on top
of `AGENTS.md`: where they conflict, `AGENTS.md` wins **except** for the
rules this file marks **[OVERRIDE]**, which the owner approved explicitly.

---

## 0. The mission in one paragraph

Deliver Release 2 as defined in `docs/releases/release-2/`: (a) the bot/human
message HTTP API (epics 2.3, 2.4), (b) Immich media sharing (epics 2.1, 2.2),
and (c) integrated verification plus release docs (epic 2.5). The finish line
is **one owner-reviewable PR `release/r2` → `main`** with every gate in §9
met, every claim backed by command output, and nothing touched outside this
repo. You stop at the gates in §8. Everything else, you decide and do.

---

## 1. Session start (every session, including after usage limits or context compaction)

Run, read, then write a 3-line orientation before touching code:

1. `git fetch origin`, `git status --short --branch`, `git branch -vv`, `git stash list`, `git log --oneline -5`.
2. Read `AGENTS.md`, this file, the newest 3 entries in `docs/releases/release-2/log/`
   (see §6), and the row for your current story in §3.
3. Write:
   - `Resuming: <story> on <branch>, last commit <sha>, tree clean|dirty.`
   - `Done when: <the pass/fail check for this story from §4>.`
   - `Next action: <one concrete step>.`

Then continue. **Never restart a story from scratch** because the context got
compacted. The repo is the memory: trust git and the log, not your recollection.

If the tree is dirty from a previous interrupted session: run the story's
tests. Green → commit as a checkpoint (§5.3). Red → fix, then commit. Never
stash to "clean up". Stashes are where work goes to die.

---

## 2. Operating rules (how to behave)

### 2.1 Define done, then stop
- Each story has a pass/fail Done check (§4). When it passes: say **DONE**,
  commit, update the log, move to the next story. No new gaps after DONE.
  Extra ideas go in the log under `Optional later` (max 3 lines).
- Vague instruction from the owner ("get it done", "make it work") means:
  continue down §3 in order. It does not mean ask what he meant.

### 2.2 Solutions, not problem lists
- Every problem you report carries the fix you chose and its status:
  `fixed` / `fixing now` / `needs owner (gate §8.x)`.
- Never end a turn on caveats alone. State a caveat once and don't repeat it.
- Don't list the remaining work every turn. The owner can read §3.

### 2.3 Decide small things, ask about gated things
- Naming, file layout, test structure, error-message wording, internal
  interfaces, library choice within the pinned stack, default values inside
  documented ranges: **decide, state it in one line, continue.**
- Ask only at the gates in §8. When asking: ask once, give 2–3 options with your
  recommendation first, and keep working on everything that doesn't
  depend on the answer.
- Before asking anything, search the repo, the docs and the log. If the
  answer is written down, it's settled. Don't ask again.

### 2.4 Use common sense, label inferences
- If context makes one answer clearly right, use it and record it
  (`decision:` line in the log). Don't block on "not explicitly specified".
- Treat something as ambiguous only when two answers are genuinely plausible
  **and** picking the wrong one is expensive to undo.

### 2.5 Evidence or it didn't happen
- "Passed" means you ran the command in this session and saw the output.
  Paste the summary line (e.g. `ok meowgram/server/internal/botauth 1.19s`,
  `All 83 tests passed!`). Never report a pass that was planned, assumed, or seen
  in an earlier session.
- A test that is **skipped** is not a pass. Report skips separately and say
  why they skipped (missing env, guarded DB name, etc.).
- If a check can't run in this environment, say `not run: <reason>` and add
  it to the integration checklist (§7.3).

### 2.6 Stop loops
- Same failure twice with no new information → stop that approach. Write:
  what's failing, two ways out, your pick. Take your pick unless it hits a gate.
- Max 3 attempts on any single flaky or environment problem before
  switching approach (e.g. different harness, build outside Docker, skip with
  a documented reason).
- Don't refactor working code mid-story to make a new test easier, unless
  the refactor is the story.

### 2.7 Reply format (every turn that ends work)
```
Status: DONE | IN PROGRESS | BLOCKED (gate §8.x)
Story: UST-x.y.z on <branch> @ <sha>
Changed: ≤5 bullets with numbers (files, tests, pass counts)
Decisions: one line each, or "none"
Needs owner: ≤1 decision (multiple choice), plus "Merge next: #N, #M, #K" from the queue
Next: one line
```
Short is better than complete. No tables of remaining gaps unless asked.

---

## 3. Build order and current state

State verified independently on 2026-10-18 (Go suites with real PostgreSQL 16
and LLDAP 0.6.3; Flutter not re-verified).

| # | Story | Branch | Parent | State 2026-10-18 |
|---|---|---|---|---|
| 0 | Housekeeping (§3.1) | n/a | n/a | **Do first** |
| 1 | UST-2.3.1 directory identity | `feat/ust-2.3.1-directory-identity` | epic 2.3 | Committed `a21ef10`; 73 Go pass |
| 2 | UST-2.3.3 bot principals | `feat/ust-2.3.3-bot-principals` | epic 2.3 | Committed `beba79a`; **1 test fails off-UTC** (§3.1.2) |
| 3 | UST-2.3.2 bot auth (Basic + LDAP group) | `feat/ust-2.3.2-bot-auth` | epic 2.3 | **Uncommitted**, no upstream set; 119 Go pass, real-LLDAP test pass |
| 4 | UST-2.4.1 shared message service | `refactor/ust-2.4.1-shared-message-service` | epic 2.4 | Committed `83f570d`; 81 Go pass |
| 5 | UST-2.4.2 HTTP send + idempotency | new from epic 2.4 | epic 2.4 | Not started; needs 1–4 integrated |
| 6 | UST-2.4.3 bot sync + examples | new from epic 2.4 | epic 2.4 | Not started |
| 7 | UST-2.1.0 Immich contract spike | new from epic 2.1 | epic 2.1 | Not started (timeboxed, §4) |
| 8 | UST-2.1.1–2.1.2 media backend | from epic 2.1 | epic 2.1 | Not started |
| 9 | UST-2.2.1–2.2.2 Flutter media | from epic 2.2 | epic 2.2 | Not started; needs 7–8 contract |
| 10 | UST-2.5.1 integrated regression/security | from epic 2.5 | epic 2.5 | Not started |
| 11 | UST-2.5.2 docs, rollout, pilot prep | from epic 2.5 | epic 2.5 | Not started |
| 12 | Release candidate PR `release/r2` → `main` | `release/r2` | `main` | Final |

Work strictly in this order unless blocked at a gate. While blocked, move to
the next story that doesn't depend on the blocked one (media 7–9 doesn't depend
on API 5–6, so it's the default parallel lane).

### 3.1 Housekeeping (story 0): do this before anything else

1. **Commit UST-2.3.2.** On `feat/ust-2.3.2-bot-auth`: run the Go suite,
   commit as `feat(auth): gate bot Basic Auth on LDAP group membership`, then
   `git branch --set-upstream-to=feat/r2-epic-2.3-bot-identity`.
2. **Fix the timezone test bug on UST-2.3.3.** In
   `server/internal/repository/bot_repository_integration_test.go` the legacy
   timestamp check uses `at.Format(time.RFC3339) != oldAt`. Under a non-UTC `TZ`
   (owner's machine is IST) pgx returns local time and the test fails. Change it
   to `at.UTC().Format(time.RFC3339)` (or compare with `time.Time.Equal`).
   Commit `fix(test): compare migration timestamps in UTC`. Then grep the whole
   repo for the same pattern (`.Format(` on DB-read times in tests) and fix
   every instance in that commit.
3. **Drop the redundant stash.** `stash@{0}` ("R2 planning and UST-2.3.1
   investigation…") was verified byte-identical to commit `a21ef10`. Confirm
   with `git diff stash@{0} a21ef10 --stat` (expect only paths that came from the
   stash's untracked part) and then `git stash drop`. **Ask the owner first (§8.1).**
4. **Empty planning branch.** `docs/release-2-planning` has no commits; its
   content landed in `a21ef10`. Delete the local branch after asking the owner
   (§8.1) and record in the log that release planning travels with 2.3.1.
5. **Publish per §5.4.** Push `release/r2` and the five epic branches. Push
   the four task branches. Open PRs 2.3.1, 2.3.3, 2.3.2 → epic 2.3 and
   2.4.1 → epic 2.4, each body following §5.5. Write the merge queue (§5.6) in
   this order: 2.3.1, 2.3.3, 2.3.2, epic 2.3 → `release/r2`, 2.4.1, then the
   sync PR `release/r2` → epic 2.4.
6. Start story 7 (media spike) while the owner merges.

Done when: `git status` is clean on every branch, no stash, all four task PRs
are open with evidence, every task branch passes the full Go suite under both
`TZ=UTC` and `TZ=Asia/Kolkata`, the Flutter suite passes, and the merge queue
is written.

---

## 4. Per-story Done checks

Acceptance criteria live in `delivery-plan.md` (work breakdown and "Required
verification"). These are the pass/fail checks that close each story.

- **UST-2.4.2 HTTP send + idempotency.** `POST /api/messages` (path
  configurable) accepts human bearer and bot Basic auth. Sender comes only from
  verified auth. Same key and same body → same response and one row. Same key and
  different body → 409. Concurrent duplicates (≥20 goroutines) → exactly one row
  and one broadcast and at most one push. DB failure → nothing broadcast. The
  idempotency table is additive with a documented retention/expiry; the expiry
  boundary is tested. CORS allows the idempotency header on web. Integration
  tests run against real Postgres, not mocks, for persistence claims.
- **UST-2.4.3 bot sync + examples.** `GET /api/messages/sync` accepts bot
  Basic auth and checks access on **every page**. Existing timestamp/UUID
  pagination is unchanged: >500 messages, equal timestamps, empty pages and
  restarts are tested. `examples/` has one bot client (Python or Go, stdlib-first)
  that persists its cursor, uses idempotency keys, honours 429 `Retry-After`
  and backs off on 503. The example runs against the scratch stack in a test.
- **UST-2.1.0 Immich contract spike (timebox: 1 session).** Output:
  `docs/releases/release-2/media-contract.md` with the Immich version
  assumption, the API calls used, the auth model (server-held API key, never
  sent to clients), upload/stream/proxy flow, size and type limits, the
  authorization rule (who may see which asset), the failure modes, and an estimate
  for 2.1.1–2.2.2. If the spike shows media is ≥2× the API effort, raise gate
  §8.4 (narrower release) with a recommendation and continue the API lane.
- **UST-2.1.1–2.1.2 media backend.** Streaming upload proxy (no full-file
  buffering in memory), asset rows persisted with a message link, and the
  authorization check on every fetch. Basic-auth bots get **no** media access
  (route-isolation test). The provider is faked in unit tests, and there is one
  integration test against a disposable Immich container if one can be pinned
  and started offline; otherwise `not run` plus an entry in §7.3.
- **UST-2.2.1–2.2.2 Flutter media.** Camera/gallery pick, upload progress,
  retry and failure states, feed rendering, and offline behaviour (queued or
  clearly failed, never silent loss). Widget tests for each state. `flutter test`
  and `flutter analyze` are clean and the web release build passes.
- **UST-2.5.1 integrated checks.** Every row of the "Required verification"
  table in `delivery-plan.md` maps to at least one named test, recorded in
  `docs/releases/release-2/verification-matrix.md` (row → test name →
  last-run result with date). No row may say "planned".
- **UST-2.5.2 docs + rollout prep.** `runbook.md` config tables are complete,
  `.env.example` has every new variable with placeholder values, the API usage
  doc has curl and example-client snippets, and the rollout/rollback procedure
  is written as numbered owner steps (Basic disabled by default, pilot group,
  disable path). The pilot checklist is ready for the owner to run.
- **Release candidate.** §9 is fully green.

---

## 5. Git rules

### 5.1 Branch layers (from AGENTS.md, unchanged)
`main` ← `release/r2` ← epic branches ← task branches. A new task branch starts
from its epic's **current** tip. Fetch first. Set upstream to the parent
immediately: `git branch --set-upstream-to=<parent>`.

### 5.2 Commits
- Conventional Commits, `type(scope): what and why`. Author per AGENTS.md:
  `git -c user.name=jyotirmoyc -c user.email=jyotirmoy.github@jyotirmoy.cc commit …`.
- One story per task branch. A story can have several commits.

### 5.3 Checkpoint commits [OVERRIDE]
Usage limits will interrupt you. **Commit every time the story's tests are
green**, at least once per session, and before any long-running harness run.
Prefix unfinished-but-green checkpoints with `wip:` in the subject only on task
branches (e.g. `wip(api): idempotency table and repository`). Before the
branch's first push (§5.4), squash `wip:` commits into clean commits. After it
has been pushed, leave history alone and add new commits.
Never leave a session with uncommitted work and never use `git stash` for
work in progress.

### 5.4 Push, PR, merge: who does what [OVERRIDE]
Owner decision, 2026-10-18: **the agent pushes and opens PRs. Only the owner
merges, at every layer.**

- **Agent may:** push task branches (`git push -u origin <branch>`), push the
  integration branches once so PRs have a base (`release/r2` and the five
  epics, currently identical to `main`), and open PRs with `gh pr create`
  targeting the correct parent (task → epic, epic → `release/r2`,
  `release/r2` → `main`).
- **Agent must never:** merge any PR, push to `main`, `release/r2` or an
  epic after its first creation, force-push, or rebase a pushed branch.
- **When a story is DONE:** clean up history before the first push (squashing
  `wip:` commits is allowed only before the first push), push, open the PR
  using the template in §5.5, add it to the merge queue (§5.6), and move to
  the next story without waiting.
- **After the owner merges:** `git fetch origin` and start new task branches
  from the **remote** epic tip (`origin/<epic>`), never from a stale local copy.

### 5.5 PR template
Title: Conventional Commit style (`feat(auth): …`). Body:
```
Story: UST-x.y.z   Base: <parent branch>   Stacked on: #N | none
What: 2–4 lines
Evidence: harness summary lines, Go pass count under UTC and IST, Flutter count
Migrations: none | additive (legacy-data test: <name>)
Config: new env vars, or none
Owner checks: link to owner-checks.md entries, or none
Merge after: #N | nothing
```

### 5.6 Waiting on merges without going idle
- **Merge queue:** keep `docs/releases/release-2/merge-queue.md` as an ordered
  list of open PRs, one line each: `#N <branch> → <base> | merge after #M |
  unblocks <story>`. Put the queue's top 3 in every reply under "Needs owner".
- **Same-epic dependency, PR still open:** stack. Branch the next task from
  the dependency's task branch, open its PR against the **epic**, and write
  `Stacked on: #N`. Once #N is merged, `git merge origin/<epic>` into the
  stacked branch (merge, not rebase) and push. GitHub then shows only the new diff.
- **Cross-epic dependency** (2.4.2 needs epic 2.3 and 2.4.1): this needs
  owner merges in this order: 2.3.1, 2.3.3, 2.3.2 → epic 2.3; epic 2.3 →
  `release/r2`; 2.4.1 → epic 2.4. Then the agent opens a sync PR
  `release/r2` → epic 2.4, and the owner merges it. 2.4.2 starts only after
  that. **Don't** stack across epics or cherry-pick sibling work.
- While blocked, work the next unblocked lane (media spike 2.1.0 is always available).

### 5.7 Conflicts
- `runbook.md` is the main conflict source (every task appended to its
  end). From now on, task branches **do not edit `runbook.md`**. They write a
  log fragment instead (§6). Once an epic's last task PR is merged, the agent
  opens one `docs/r2-epic-2.x-runbook` PR against that epic that folds the
  fragments into the runbook: `docs(runbook): fold epic 2.x decisions`.
  The existing task branches already edit `runbook.md`, so expect conflicts
  when the owner merges them. The agent resolves each one on its task branch
  by merging `origin/<epic>` into it (keeping both entries), then pushes. The
  owner should never have to resolve a conflict.
- Resolve code conflicts by keeping both behaviours and re-running the full
  suite. If the merge changes behaviour beyond a mechanical resolution, it's a
  separate `fix:` commit with its own test.

### 5.8 Never
Merge any PR; force-push; rewrite pushed history; commit or push to `main`,
`release/r2` or an epic (beyond their one-time creation); `git add -A` without
checking `git status` for secrets and env files; delete branches without
asking (§8.1).

---

## 6. Logging and docs

- **Log fragments:** `docs/releases/release-2/log/YYYY-MM-DD-UST-x.y.z.md`, one
  per story per day, appended during work. Content: decisions (`decision:`),
  owner answers verbatim (`owner:`), commands run with summary output,
  skips and why, `Where we stopped / next step:` as the final line.
- Owner answers are **settled facts**. Record them the moment they're given.
  Never re-ask a recorded answer.
- `delivery-plan.md` status column: update at epic integration, not per commit.
- `verification-matrix.md`: maintained from UST-2.4.2 onward, not left to the end.
- New env variable → `config.go` + `.env.example` + runbook table (at epic
  fold) in the same epic. Zero hardcoding per AGENTS.md §2.

---

## 7. Testing rules

### 7.1 What to run
- Per commit: the affected Go packages with `-race` and `go vet` on them.
- Per story Done: the full harness `./deploy/scripts/verify.ps1` (Go race + vet,
  Postgres migrations/restore, Flutter analyze/test, web build, web smoke) plus
  any story-specific harness (e.g. `verify-bot-auth.ps1`).
- Per epic integration: the full harness on the epic branch.

### 7.2 Test quality rules
- **Timezones:** run the Go suite under `TZ=UTC` **and** `TZ=Asia/Kolkata`
  at least once per story. Compare times with `.Equal` or after `.UTC()`, never
  as formatted local strings.
- **Real dependencies for real claims:** persistence, idempotency, migrations
  and concurrency use real Postgres (`TEST_DATABASE_URL`). Guarded tests need the
  DB path `/verification`; set that up instead of letting them skip. LDAP claims
  use the pinned disposable LLDAP image. Mocks are only for unit-level logic.
- **No green by skipping:** a test you added must run in the harness. If it
  needs an env var, the harness must set it.
- **Concurrency tests** use ≥20 goroutines and run under `-race`.
- **Additive migrations only.** Every new migration has a test that seeds legacy
  rows, migrates, and proves they're unchanged. Down migrations must not drop
  user or message data.
- **Synthetic data only:** no real names, domains, emails or credentials in
  fixtures. Use `example.home.arpa` and generated secrets.
- Don't weaken an existing assertion to make a new change pass. If an
  assertion is wrong, fix it in its own `fix(test):` commit and say why.

### 7.3 Can't run it here?
Keep `docs/releases/release-2/owner-checks.md`: anything that needs the real
fleet, real Immich, real LLDAP, a phone or a desktop build, written as numbered
steps the owner can do in ≤15 minutes, each with the expected result. This is
the only acceptable home for "not verified".

---

## 8. Gates: stop and ask the owner

Ask once, with options and a recommendation, then continue on unblocked work.

1. **Destructive git:** dropping stashes, deleting branches, any history rewrite.
2. **Anything outside this repo:** roastery or fleet deployment, the
   `purrbrews-containers` repo, Authelia config, live LLDAP
   (accounts, groups, reader credential), DNS, Traefik, Cloudflare.
3. **Credentials:** creating, rotating, printing or committing any secret.
   Generated test secrets inside disposable containers are fine.
4. **Scope:** narrowing R2 (e.g. ship API without media), adding features not in
   the plan, changing a "non-negotiable" from AGENTS.md §2, or a migration that
   rewrites or drops data.
5. **Merging** is always the owner's job. Pushing task branches, the one-time
   creation of integration branches, and opening PRs are pre-approved (§5.4).
   Anything else that writes to GitHub (deleting remote branches, editing repo
   settings, releases, tags) needs approval.
6. **Spending money** or adding a paid or external service.

Everything not on this list is the agent's call.

---

## 9. Release-candidate checklist (story 12)

All must be true, each with evidence in the log:

- [ ] Every story in §3 is DONE or explicitly descoped by the owner (recorded `owner:` line).
- [ ] All epic PRs merged into `release/r2` by the owner; full harness green on `origin/release/r2`.
- [ ] Go suite green under UTC and IST; Flutter analyze + test green; web build + smoke green.
- [ ] `verification-matrix.md`: every required-verification row → named test → passing result with date.
- [ ] No skipped tests without a recorded reason; `owner-checks.md` complete.
- [ ] Migrations additive; legacy-data test present for each.
- [ ] Basic Auth disabled by default in config; disable path documented and tested.
- [ ] `.env.example`, `config.go`, runbook tables consistent (scripted check or listed diff).
- [ ] No secrets, real domains or personal data in tracked files (grep evidence).
- [ ] `runbook.md` folded from all log fragments; delivery-plan statuses current.
- [ ] PR `release/r2` → `main` opened: summary, stories, test evidence, migrations, config, rollout and rollback, owner pilot steps.

Then stop, with: `Status: DONE. R2 release candidate PR #N ready for owner review and merge.`

---

## 10. Anti-patterns seen in earlier sessions (don't repeat)

- Reporting "coverage" or "agreement with labels I generated" as accuracy.
- Ending turns with "remaining gaps" tables instead of fixes.
- Asking questions answerable from the repo or already answered.
- Leaving a whole story uncommitted when a usage limit hit.
- Stashing work "to keep tasks separate" and leaving the stash behind.
- Creating integration branches and then never integrating into them.
- Every branch appending to the same runbook section, which guarantees conflicts.
- Tests that only pass in UTC containers.
- Claiming a suite passed that ran in a previous session or not at all.
