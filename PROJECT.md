---
project: meowGram
status: active
status_reason: R2 is on main and v2.0.0 images are published; R2.1 kickoff and contract fixtures await owner merges
resume_trigger: n/a
last_activity: 2026-10-23
last_reviewed: 2026-10-25
repo: https://github.com/purrMonster/meowGram
related: ["[[purrbrews]]", "[[pourover]]"]
tags: [project]
---

# meowGram

## What it is

A private household chat with an offline-first lounge and authenticated bot messaging.
Flutter on web, Android, iOS, Windows and macOS; Go API/WebSockets; PostgreSQL;
Authelia OIDC for people, LLDAP-checked Basic Auth for bots; Docker deployment.
See the [overview](README.md) and [current design](runbook.md).

## Current state

| Area | State | Evidence |
|---|---|---|
| Code | Built: R2 chat/bot API on main. R2.1 lounge + groups approved; contract fixtures in review, runtime implementation pending | [runbook §13.9](runbook.md#139-integrated-message-api-epic-24-2026-10-21), [plan](docs/releases/release-2.1/README.md), [#53](https://github.com/purrMonster/meowGram/pull/53) |
| Release | v2.0.0 tag at `be15cec`; server/web GHCR images published for amd64/arm64. GitHub Release remains draft (checked 2026-10-25) | [publication record and digests](runbook.md#142-r2-publication-verified-2026-10-23-ist), [release workflow](https://github.com/purrMonster/meowGram/actions/runs/39834547253) |
| Deployment | Current deployment unknown; the dated runbook leaves R2 rollout and bot pilot unverified | [runbook §14.2](runbook.md#142-r2-publication-verified-2026-10-23-ist), [unfilled rollout record](docs/releases/release-2/rollout.md) |
| Tests / CI | Passing main `f008947`: Scratch verification and CodeQL. Contract-branch local regression is recorded separately; fixture integrity is not runtime compliance | [Scratch CI](https://github.com/purrMonster/meowGram/actions/runs/39989607036), [CodeQL](https://github.com/purrMonster/meowGram/actions/runs/39989606862), [#53](https://github.com/purrMonster/meowGram/pull/53) |

## Open points

- Triage 5 open dependency alerts (2 critical, 2 high, 1 low; checked 2026-10-25). [Dependabot](https://github.com/purrMonster/meowGram/security/dependabot)
- Owner merge [#52](https://github.com/purrMonster/meowGram/pull/52) (kickoff), then review amended [#53](https://github.com/purrMonster/meowGram/pull/53) (78 contract fixtures); schema story UST-2.1.1.2 follows. [Queue](https://github.com/purrMonster/meowGram/blob/62926eae11cb4099fd7d08da05918a1d4a6e38d9/docs/releases/release-2.1/merge-queue.md)
- Implement the remaining [R2.1 stories](docs/releases/release-2.1/delivery-plan.md); tagged HTTP replay needs the disposable seed adapter and server implementation before it can prove compliance.
- R2 rollout, real backup/restore, deployment digests and bot pilot need owner evidence; publishing the draft release is separate. [Rollout](docs/releases/release-2/rollout.md), [owner checks](docs/releases/release-2/owner-checks.md)
- O1 container baseline is recorded in #52; remaining live/device checks stay pending. [Kickoff evidence](https://github.com/purrMonster/meowGram/blob/62926eae11cb4099fd7d08da05918a1d4a6e38d9/docs/releases/release-2.1/log/2026-10-23-P0-kickoff.md), [R2.1 checks](docs/releases/release-2.1/owner-checks.md)
- Reconcile older deployment descriptions with a dated owner record; AGENTS §3 and the newer rollout name different hosts. [AGENTS](AGENTS.md), [rollout](docs/releases/release-2/rollout.md)

## Branches

Remote inventory checked 2026-10-25; merged means ancestor of main, not deletion approval.

- `release/r2.1`: active release integration at `1f39e07`; awaits #52, no direct commits.
- `feat/r2.1-epic-1-channels`: active epic 1 integration at `1f39e07`; #53 targets it.
- `feat/r2.1-epic-2-delivery`: active, empty integration at `1f39e07`; depends on epic 1.
- `feat/r2.1-epic-3-client`: active, empty integration at `1f39e07`; dependencies in plan.
- `feat/r2.1-epic-4-ntfy`: active, empty integration at `1f39e07`; version inventory needed.
- `test/r2.1-epic-5-verification`: active, empty integration at `1f39e07`; final verification.
- `docs/r2.1-kickoff`: active #52; baseline, branch map and rescue decisions.
- `docs/ust-2.1.1.1-contracts`: active #53; amended contract fixtures, no runtime changes.
- `docs/project-card`: active card update; earlier card merged as #54, this revision targets main.
- `chore/r2-verification-parent-sync`: merged into main; stale R2 integration work.
- `docs/r2-release-publication`: merged into main; stale publication-evidence branch.
- `feat/ust-2.4.2-http-send-idempotency`: merged into main; stale completed R2 story.
- `feat/ust-2.4.3-bot-sync-example`: merged into main; stale completed R2 story.
- `rescue/docs/ust-2.1.0-immich-contract`: retained duplicate of merged #32; owner says leave it, no action.
- `rescue/feat/tabbytally-review-integration`: ARCHIVE ONLY; never merge/PR into meowGram (000006 migration collision).

Rescue restrictions: [owner decision log](https://github.com/purrMonster/meowGram/blob/62926eae11cb4099fd7d08da05918a1d4a6e38d9/docs/releases/release-2.1/log/2026-10-23-P0-kickoff.md). `origin/docs/r2.1-plan` is only a stale local tracking ref; absent remotely.

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

1. Read [AGENTS](AGENTS.md), [playbook §1](docs/releases/release-2/AGENT_PLAYBOOK.md), [R2.1 translations](docs/releases/release-2.1/README.md) and newest logs; fetch, inspect status/branches and open PRs.
2. Owner merges #52 and corrected #53. Fetch again, then branch `feat/ust-2.1.1.2-schema` from `origin/feat/r2.1-epic-1-channels`; PR back to that epic. Follow the [delivery plan](docs/releases/release-2.1/delivery-plan.md) and actual [queue](https://github.com/purrMonster/meowGram/blob/62926eae11cb4099fd7d08da05918a1d4a6e38d9/docs/releases/release-2.1/merge-queue.md).
3. Use Docker: Go `golang:1.27.1`, pinned Flutter `3.44.4`; host SDKs unnecessary. Run verify scripts, both Go timezones and dedicated harnesses; record counts/skips/digests. [Approved environment](https://github.com/purrMonster/meowGram/blob/62926eae11cb4099fd7d08da05918a1d4a6e38d9/docs/releases/release-2.1/log/2026-10-23-P0-kickoff.md)

## Decisions to remake

- Revisit the earlier pourOver prefix assumption before its handoff: message rows no longer carry member count, and bots still cannot list conversations. [Owner-corrected contract in #53](https://github.com/purrMonster/meowGram/blob/docs/ust-2.1.1.1-contracts/docs/releases/release-2.1/api-contract.md)

## Questions for Roastmaster

- Which version and host currently run production? The dated rollout record is unfilled.
- Should the draft v2.0.0 GitHub Release be published now or remain draft?
- Which ntfy server/iOS versions should the contract spike pin? [O2](docs/releases/release-2.1/owner-checks.md)
- How should pourOver decide its two-member prefix behavior without message-row counts or bot conversation-list access?

## Card history

- 2026-10-23 · created by Claude (Opus 5.5)
- 2026-10-25 · updated by Codex
