# pourOver R2.1 handoff — documentation only

Owner-approved behavior, 2026-10-23. pourOver is a separate repository: this plan
does not change, deploy or authorize work in it. Basic scope remains GET sync and
POST send only. No bot-initiated conversation endpoint in R2.1.

## Poll, route and reply

Poll configured HTTPS sync endpoint every 3 seconds with scope=member, after and
after_id. The response is the merged readable lounge/groups stream, oldest first,
at most 500, every new row carrying channel_id. Keep one durable merged checkpoint
plus per-conversation processing/dedup state; do not substitute a per-group maximum.
Page through X-Has-More/full pages; preserve checkpoint on 429/503 and honor
Retry-After. Budget acceptance is 20 polls plus at least 10 replies per minute.

Reply in the triggering message's conversation: copy its channel_id to the POST
body alongside text_content; null or missing legacy field is lounge. Persist the
exact pending body/key before sending. A private send denied by 403 is visibly
failed/parked, never silently retried in lounge. Key uniqueness is principal-wide:
reuse with changed channel/text is 409. Same retry reauthenticates, returns exact
stored response, and produces no duplicate message/push. Preserve 168h minimum/
configured retention awareness and kind+ID self-message filtering.

A human must add the bot's existing typed principal ID to a group first. It can
resolve its principal via authorized lounge sync without posting, as in R2. It
cannot create, discover/list/manage groups or list principals. Global LDAP group
eligibility is still checked every request, including every page and retry.
Group with no human members is unreadable/unwritable even to its remaining bots.

## Trigger policy

Owner resolution, 2026-10-27: prefix always required; prefix-free replies need a
future, owner-approved bot discovery mechanism (would widen the Basic Auth scope /
AGENTS.md). This applies to lounge, groups and two-member groups alike. Ignore own
sender_kind+sender_id. Bots have no member-count discovery route, and message rows
carry no conversation metadata. Sending always checks current membership; do not
rewrite a persisted pending reply's body/key when membership changes.

## Membership backfill

Persist X-Membership-Version with merged checkpoint. New/changed revision can
make older history newly readable: replay from epoch and deduplicate already seen
UUIDs, or explicitly reset under a reviewed bot-history policy. This recovers data
without assuming global cursor alone finds old messages in a newly joined group.
Avoid a pilot flood: initialize processed baseline deliberately and keep processed
UUID state across restarts/replays. Reading history need not automatically reply
to all history; document the operator's chosen starting point.

Acceptance in the other repo: lounge and A/B routing, denied group 403, prefix
required in lounge and every group size, no-human group, identical retry,
lost-response restart, same-key cross-channel conflict, revoked membership/LDAP,
merged equal-time pagination/revision replay, and no duplicate historical replies.
This repo only supplies API fixtures and this handoff, not an external implementation.
