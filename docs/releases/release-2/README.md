# Release 2: media sharing and the message API

Status: directory investigation verified; application implementation pending.
Updated: 2026-10-17. No release date is committed.

Release 2 retains the existing Immich media work in Epics 2.1 and 2.2 and adds
an authenticated text-message API in Epics 2.3–2.5. The first API version is
part of Release 2; references to “API v1” do not mean application Release 1.

## Documents

- [Directory compatibility and identity investigation](directory-identity.md):
  UST-2.3.1 fixture evidence, selected schema and remaining deployment gate.
- [API and identity design](api-design.md): authentication boundaries, message
  contract, directory integration and proposed settings.
- [Delivery and acceptance plan](delivery-plan.md): stories, dependencies,
  verification and release gates.
- [Existing roadmap](../../../roadmap.html#release-2): retained media stories.
- [Runbook](../../../runbook.md#13-release-2-planning): decision record and
  distinction between current implementation and planned behavior.

## Agreed direction

People keep Authelia OAuth/OIDC and PKCE. Human API calls use bearer access
tokens. Bots use HTTP Basic Auth with dedicated LLDAP usernames and passwords,
over HTTPS, on the message send/read endpoints only. Membership in one
configured LLDAP group gates this Basic Auth path. `meowgram-api-bots` is the
proposed group name, not an existing group or a hardcoded requirement.

Each Basic request checks credentials and current group membership through an
encrypted LDAP connection. No positive authentication or membership cache is
planned for API v1. Directory failures deny bot operations. The application
stores neither bot passwords nor locally issued authentication tokens.

The group grants both read and send access to the shared lounge. Separate
read-only/send-only roles are deferred; they were an earlier proposal, not part
of the agreed minimal group policy. Group membership is an administrator's
explicit grant of access to lounge history as well as new messages.

## Scope

| Workstream | Release 2 scope | Boundary |
|---|---|---|
| Existing media, Epics 2.1–2.2 | Server-side Immich proxy, asset references, picker and feed rendering | Existing stories retained; provider compatibility and detailed media contracts still need implementation-time review |
| Identity, Epic 2.3 | Dedicated bot accounts, configurable group, per-request validation, stable sender identity | No bot OAuth, local passwords, provisioning UI or automatic human/bot account linking |
| Message API, Epic 2.4 | HTTP text send, paginated read, persistent retry deduplication, shared message processing | One lounge; no email, direct messages, new rooms or bot media uploads |
| Verification, Epic 2.5 | Isolated integration checks, usage examples, operational instructions and pilot | Live configuration and credential work require owner authorization |

The API can be implemented and verified independently of media. Completing it
alone does not mark all of Release 2 complete. Splitting the release or moving
the media work requires an explicit scope decision.

## Design defaults and unresolved evidence

The detailed API contract and configuration names are proposed implementation
defaults. The settled product decisions are human OAuth, bot Basic Auth,
LLDAP group gating, unattended operation and chat rather than email.

Before implementing directory authentication, verify the deployed LLDAP schema,
stable account identifier, group lookup semantics and encrypted connection
support in an isolated equivalent. Before changing persistence, select an
additive identity schema compatible with the existing sender foreign key.
These are engineering gates, not reasons to return to the discarded bot OAuth
design. No live configuration was inspected or changed to produce this plan.

## Completion

Use the gates in [the delivery plan](delivery-plan.md). UST-2.3.1 now has isolated
fixture evidence; application features remain planned. This documentation does not authorize a
deployment, create accounts/groups, or change credentials.
