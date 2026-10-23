# Release 2.1 plan — channels and iPhone notifications

Planning date: 2026-10-23 (Asia/Calcutta). **Proposed, not implementation approval.**
This session creates only `docs/r2.1-plan` → `main`, as explicitly requested by
the owner. No release/epic branches, runtime changes, migrations or live changes.

## Budget and risks

Base: **20–31 engineering days (160–248 hours)**. With 25% contingency:
**25–39 days (200–312 hours)**, upper bound rounded up to a half-day.
Eight-hour focused days include tests, review fixes, integration and documentation;
exclude owner/CI waiting and live administration. See [story estimates](delivery-plan.md).
Reserve 2–3 owner hours separately, spread over timed checks and physical devices.

1. Privacy: every existing delivery path is lounge-global. A missed history,
   queued socket frame or push recipient filter defeats channel authorization.
2. Upgrade/recovery: v2.0.0 has unfiltered reads. Forcing its migration version
   after channel messages exist would expose private history, not merely fail.
3. Client/provider lifecycle: one Hive cache and background cursor must become
   account/channel-scoped; ntfy delivery needs a real locked iPhone test.

## Verified starting point

`git fetch origin` on this date resolved `origin/main` to `e094dfa` (#50).
Annotated tag `v2.0.0` peels to `be15cec6ceed2cb4a21b69cfe5e4548d2bfd818c`,
the #49 release merge, preceded by #48 epic integration and #47 doc fold.
Tracked source is unchanged between that tag and the planning base; later
changes record publication. The current source implements authenticated message
send/sync, per-request bot LDAP checks, durable idempotency, sender kinds and
migrations 000005/000006. Basic defaults off. Immich remains deferred.

The merged runbook §14.2 records successful GHCR publication; §14.1 and the R2
matrix record actual-binary rollback tests. These are historical evidence,
not new test runs or proof of live deployment. Older runbook headers, R2 delivery
status and merge-queue text lag the verified merges; use git and the latest entries.

## Scope

- Channels with nullable UUID `channel_id`; null is the existing lounge. Existing
  history stays there. Humans use OIDC; bots retain per-request global LLDAP
  eligibility plus explicit channel membership. Bots may still post in lounge.
- Channel list, selection, minimal membership controls, scoped history/live/sync,
  unread state and offline cache. No general provisioning console.
- Content-free iPhone alerts through a per-user self-hosted ntfy topic and the
  ntfy iOS app/upstream relay. Android/macOS FCM remains supported.
- Decide schema-ahead startup policy before writing 000007; test recovery with
  the actual v2.0.0 binary. Firebase key restrictions are an owner check only.
- No Immich/media, threads, structured mentions, local human authentication,
  new paid service, live provisioning or deployment in this planning session.

## Decisions for owner review

| Decision | Options | Recommendation |
|---|---|---|
| Startup with schema ahead | Fail closed; explicit tested compatibility manifest; unrestricted bypass | Fail closed; [ADR 001](adr-001-schema-compatibility.md) |
| Channel administration | Any human creates/owners manage; household-admin-only API/CLI; LLDAP group mapping | Any human creates, human channel owners manage typed memberships; [ADR 002](adr-002-membership.md) |
| Pre-upgrade replay bytes | Preserve exact R2 stored responses as a narrow missing-channel-field exception; add null and relax byte equality | Preserve exact bytes until each key expires; all new responses carry channel_id |
| Legacy unowned cache | Import only with attributable account/server, otherwise quarantine; assume current account owns all legacy bytes | Safe attribution/quarantine; preserve data, never silently attach it to a different account |
| Playbook translation | Use the R2.1 stories/gates below; literal stale R2 story order | Approve replacement story map and planning-only evidence exception below |

These are proposals, not recorded owner answers. The replay exception needs explicit
approval because the requested universal field and R2 exact stored response guarantee
cannot both hold for already-stored bytes. No rewriting of those rows is proposed.

## Playbook translation needing confirmation

Apply AGENTS.md and the unchanged R2 playbook's overrides, owner merges, checkpoint
rules, evidence standards and §2.7 reply format. Translate log/matrix/queue paths to
`release-2.1`. The following do not translate by simple text substitution:

- §§0/3/4 name obsolete R2 stories and media lanes. Replace them with this plan's
  stories and checks; media is not an alternate work lane.
- This planning PR directly targets main by the owner's explicit instruction.
  Later work must follow release → epic → task ancestry; this exception does not
  authorize implementation on main or creation of release/r2.1 today.
- §4/§9's no-planned-rows rule applies at release candidate completion. This
  planning matrix must truthfully say PLANNED. Request accepting static doc
  checks plus CI for this docs PR; local full harness is unavailable, as logged.
- AGENTS.md requires same-branch runbook updates; playbook §5.7 instead prescribes
  epic log folds without an [OVERRIDE] label. Request the established R2 log/fold
  convention for R2.1. No runtime decision is implemented in this docs PR.

Approval of the plan should settle these translations before implementation;
otherwise leave the affected gate closed. Do not edit AGENTS.md/playbook.

## Reading map

[Code and compatibility analysis](channels-proposal.md) ·
[Delivery plan and branch map](delivery-plan.md) ·
[Verification matrix](verification-matrix.md) ·
[Owner checks](owner-checks.md) · [Merge queue](merge-queue.md).

Implementation starts only after explicit owner approval, then creation of
`release/r2.1` from fetched main. All recommendations remain reviewable here.
