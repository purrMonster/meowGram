---
project: meowGram
status: active
status_reason: v2.0.0 (bot message API) is tagged and its images are published; R2.1 (lounge + groups) is planned and its kickoff/contract PRs are open
resume_trigger: n/a
last_activity: 2026-10-23
last_reviewed: 2026-10-23
repo: https://github.com/purrMonster/meowGram
related: ["[[purrbrews]]", "[[pourover]]"]
tags: [project]
---

# meowGram

## What it is
A private cat-lounge chat for the purrBrews household: one shared lounge today,
plus an authenticated HTTP message API so bots (first: pourOver) can read and post.
Stack: Flutter client (web, Android, iOS, Windows, macOS) · Go WebSocket/API server ·
PostgreSQL · Authelia OIDC (bots: LLDAP-checked Basic Auth) · Docker Compose behind Traefik.

## Current state
| Area | State | Evidence |
|---|---|---|
| Code | Built: R1 chat + R2 bot message API on main; R2.1 not started (planning only) | [runbook §13.9](runbook.md#139-integrated-message-api-epic-24-2026-10-21), [R2.1 README](docs/releases/release-2.1/README.md) |
| Release | v2.0.0: tag at `be15cec`; GHCR `meowgram-server:2.0.0` + `meowgram-web:2.0.0` published (amd64/arm64); GitHub Release v2.0.0 is still a **draft** (installers + checksums uploaded) | [runbook §14.2](runbook.md#142-r2-publication-verified-2026-10-23-ist), [release run 39834547253](https://github.com/purrMonster/meowGram/actions/runs/39834547253) |
| Deployment | v2.0.0 **not deployed**: rollout record reads NOT DEPLOYED / NOT PULLED; `BOT_BASIC_AUTH_ENABLED=false`. What currently runs, and where, is unknown (see Questions) | [rollout.md deployment record](docs/releases/release-2/rollout.md), [runbook §14.2](runbook.md#142-r2-publication-verified-2026-10-23-ist) |
| Tests / CI | Passing on main `1f39e07`: Scratch verification + CodeQL (2026-10-23) | [Scratch verification run](https://github.com/purrMonster/meowGram/actions/runs/39950662187), [CodeQL run](https://github.com/purrMonster/meowGram/actions/runs/39950662033) |

## Open points
1. **R2 rollout (owner):** cellar package access → real dump, off-node copy, isolated
   restore check → keep pre-r2 image IDs → pull 2.0.0 → smoke human messaging → record
   digests. [rollout.md](docs/releases/release-2/rollout.md)
2. **Publish the draft GitHub Release v2.0.0** (owner action; v1.1.0 is also still draft). [runbook §14.2](runbook.md#142-r2-publication-verified-2026-10-23-ist)
3. **Bot pilot:** R2 owner checks, then enable Basic Auth and run the pourOver pilot.
   [owner-checks.md](docs/releases/release-2/owner-checks.md)
4. **Review/merge open R2.1 PRs:** #52 `docs/r2.1-kickoff` → `release/r2.1`; #53
   `docs/ust-2.1.1.1-contracts` → `feat/r2.1-epic-1-channels` (unblocks UST-2.1.1.2).
   [R2.1 merge queue on #52's branch](https://github.com/purrMonster/meowGram/blob/docs/r2.1-kickoff/docs/releases/release-2.1/merge-queue.md)
5. **R2.1 owner checks O1–O16** all PLANNED (O1 marked verified on #52's branch, not on main).
   [owner-checks.md](docs/releases/release-2.1/owner-checks.md)
6. **Stale docs:** AGENTS.md §3 says it runs on roastery and §4 "Known gaps" date from
   1.0.1; runbook header says v1.21.0 / "await owner merges", now superseded by §14.2.
   [AGENTS.md](AGENTS.md), [runbook header](runbook.md)
7. **Recovery limit:** once any bot message exists, old-image rollback is forbidden;
   only fix-forward or full restore. [runbook §14](runbook.md#14-r2-release-readiness-and-ghcr-rollout-2026-10-22)

## Branches
- `release/r2.1`, `feat/r2.1-epic-1-channels`, `feat/r2.1-epic-2-delivery`,
  `feat/r2.1-epic-3-client`, `feat/r2.1-epic-4-ntfy`, `test/r2.1-epic-5-verification`:
  R2.1 integration layers, all still at main `1f39e07`. Active, empty by design.
- `docs/r2.1-kickoff`: PR #52, records baseline and branch map. Active.
- `docs/ust-2.1.1.1-contracts`: PR #53, frozen HTTP fixtures. Active.
- `rescue/docs/ust-2.1.0-immich-contract`: 2 docs commits (Immich media contract, 2026-10-18), no PR. Looks like a preserved copy for R3; stale until R3.
- `rescue/feat/tabbytally-review-integration`: 6 commits (pourOver/tabbyTally integration notes plus code, 2026-10-21), no PR. Looks stale; its migrations appear superseded by main.
- Fully merged into main, safe-looking to delete (owner decides): `chore/r2-verification-parent-sync`,
  `docs/r2-release-publication`, `feat/ust-2.4.2-http-send-idempotency`, `feat/ust-2.4.3-bot-sync-example`.

## Key decisions
- 2026-10-07 · Claude review → 1.0.1 hardening (mandatory `aud`/`exp`, no real domains, loopback ports) · [runbook §12.9](runbook.md#129-release-101-resolution)
- 2026-10-17 · Release → epic → task branch layers, owner merges every layer · [runbook §13.2](runbook.md#132-release-branch-hierarchy-2026-10-17), [AGENTS.md §1](AGENTS.md)
- 2026-10-18 · Bot accounts may use Basic Auth on send/sync only, validated against LLDAP · [runbook §13.4](runbook.md#134-bot-authentication-gate-ust-232)
- 2026-10-21 · Scope split: R2 = API only, R2.1 = channels, R3 = Immich media · [runbook §13.7](runbook.md#137-other-completed-work-and-approved-release-scope-2026-10-21)
- 2026-10-22 · Production pulls versioned GHCR images (no `latest`); builds move to a dev override · [ghcr-rollout-decision.md](docs/releases/release-2/ghcr-rollout-decision.md)
- 2026-10-22 · Disabling bots + old image is not a rollback; guarded Recovery A/B only · [runbook §14](runbook.md#14-r2-release-readiness-and-ghcr-rollout-2026-10-22)
- 2026-10-23 · R2.1 = lounge (`channel_id` null) + member groups, 1:1 is a 2-member group · [ADR 002](docs/releases/release-2.1/adr-002-membership.md)
- 2026-10-23 · Fail-closed startup when DB schema is ahead of the binary · [ADR 001](docs/releases/release-2.1/adr-001-schema-compatibility.md)

## How to resume
1. Orient: AGENTS.md → [R2 AGENT_PLAYBOOK](docs/releases/release-2/AGENT_PLAYBOOK.md) → runbook §14 → `git log -20` → open PRs.
2. Owner: run the R2 rollout on cellar per [rollout.md](docs/releases/release-2/rollout.md) and fill its deployment record.
3. Owner: review and merge #52, then #53.
4. Agent: start UST-2.1.1.2 (`feat/ust-2.1.1.2-schema` from `feat/r2.1-epic-1-channels`) per the [R2.1 delivery plan](docs/releases/release-2.1/delivery-plan.md); needs Docker, Go 1.27.1 and Flutter (O1).

## Decisions to remake
- **Deploy R2 before building R2.1?** R2.1 is estimated at 26.5–40 days, while v2.0.0
  and the pourOver pilot are not live yet. Shipping R2 first gives real feedback and
  a clean backup baseline (R2.1 O11–O13 assume it). [R2.1 README](docs/releases/release-2.1/README.md)
- **Where meowGram runs:** AGENTS.md §3 (roastery, local builds) and the R2 rollout
  (cellar, GHCR pulls) disagree; settle it and update AGENTS.md §3 before the next rollout.
- **Rescue branches:** keep, PR or delete; they predate the current scope split.

## Questions for Roastmaster
- What runs in production right now (v1.1.0? something else?) and on which host, roastery or cellar? No dated runbook entry confirms either.
- Should the draft GitHub Releases (v1.1.0, v2.0.0) be published, or are they intentionally kept as drafts?
- Are the two `rescue/*` branches still needed? Was the Immich contract merged anywhere that still exists?
- Can the four fully merged branches be deleted?
- Is O1 (environment) verified on main, or only on #52's branch?
- Is the R2 bot pilot (pourOver) still planned before R2.1 work starts?

## Card history
- 2026-10-23 · created by Claude (Opus 5.5)
