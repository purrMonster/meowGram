# ADR 002 — explicit memberships managed by human channel owners

Status: PROPOSED, not an authorization to implement or provision accounts.

| Option | Benefits | Trade-offs |
|---|---|---|
| A. Any authenticated human creates; channel owners manage (recommended) | Small household workflow; no new household-admin identity; matches existing proposal | Every human can create private channels; owner transfer and audit needed |
| B. Household-admin-only API or CLI | Central control and minimal end-user management UI | Need configured immutable OIDC admin subjects, bootstrap/recovery policy and operator work for every change |
| C. Channel membership from LLDAP groups | Reuses existing directory administration | Human OIDC subjects need trustworthy directory mapping; per-request directory dependencies, sync/revocation rules and external configuration expand scope |

Choose A unless the owner wants centralized control. Basic bots can neither
create/list/manage channels nor discover the household principal directory.
Bot send/sync remains the only Basic surface, with the existing global LDAP group
checked on every request and channel membership checked separately. A channel
owner cannot grant LDAP eligibility or create credentials.

## Proposed rules

- Creator becomes a human owner atomically with channel creation. Members can
  read all retained channel history, including before joining, and send. Removing
  then re-adding restores history access; no time-limited membership semantics.
- Owners add/remove existing typed principals, promote/demote human owners,
  rename and archive their channel. Bots are members only. No implicit username
  grants: human FK targets `users.authelia_sub` (text); bot FK targets
  `bot_principals.id` (UUID). Equal strings/names across kinds stay distinct.
- Protect the last human owner transactionally, including concurrent demotions,
  leave and principal deletion. A deleted directory identity is not automatically
  transferable to a recreated account. Lost-owner recovery is an explicit
  operator procedure, never a public self-escalation endpoint.
- Archive preserves rows and member reads, disables sends/invites, and permits
  existing owners to remove members. Hard delete and unarchive UI are out of scope.
- Lounge is implicit null: no membership row, no archive/delete/ownership, and
  authenticated humans/eligible bots retain access. Removing a private-channel
  membership does not remove lounge access.
- Unknown well-formed IDs and nonmembership both return 403; malformed IDs 400.
  Missing authentication remains 401. No empty-list substitute for a forbidden
  specific channel. Owners are members too; every page/replay/send is authorized.
- Membership audit is append-only application behavior: actor, typed target,
  channel, operation and UTC time, without message bodies or credentials.
  Preserve audit/membership data through down/up; last-owner constraints must not
  be silently bypassed through FK cascades.

## Minimal management surface

Proposed configurable route family: channel list/create; per-channel rename/archive;
membership list/add/remove/role update; bounded human-only principal lookup for
channel owners to select already provisioned IDs. List only the caller's channels;
lookup returns only minimal typed identity/label and is rate-limited/audited.
The owner-only lookup is a product choice in A, not a new anonymous directory.
Reject Basic everywhere in this family, including alternate methods/aliases.
Reuse OIDC and API budgets; never add local passwords/tokens.

Bots can be resolved by a successful authenticated lounge sync without posting
or by their existing principal record, then a human owner adds that ID. No bot
provisioning UI or new live LLDAP operation is needed in the application.

The minimal Flutter controls are create, member picker, add/remove/transfer and
archive confirmation, visible only to eligible humans. Full administrative
dashboards, invitations to unprovisioned identities and group mapping are deferred.
