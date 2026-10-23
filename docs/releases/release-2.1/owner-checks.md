# R2.1 owner checks — revised draft

O1 container baseline **PASS** on 2026-10-23; all live checks remain **PLANNED**. Product/process decisions in README/ADRs
are already APPROVED and must not be asked again. No live changes this session.
Each check <=15 minutes hands-on; restore/CI may need unattended time. Keep actual
domains/subjects/topics/tokens and sensitive screenshots private.

| Check | Procedure / expected result | Time | Status |
|---|---|---|---|
| O1 Environment | Approved Docker toolchain: Go 1.27.1 and Flutter 3.44.4 containers; host SDKs not required. Both timezones and all required suites passed; commands/digests in [kickoff log](log/2026-10-23-P0-kickoff.md). | 10 min | PASS 2026-10-23 |
| O2 ntfy inventory | Supply exact server/image digest and iOS app version for first contract spike; review upstream version/config. Fixture pinned, no assumption of latest. | 5 min | PLANNED |
| O3 ntfy protection | Owner sets publisher write-only and each user's topic read-only; deny anonymous. Synthetic own-topic read succeeds, other-user read/write fails. Inspect generic alert and relay metadata exposure; credentials stay host-local. | 15 min | PLANNED |
| O4 iPhone | Configure separate ntfy iOS app, lock phone, send authorized lounge/group messages with meowGram stopped. Generic alert arrives; ntfy mode registers no FCM or duplicate local alert. Test network outage/recovery, record latency without SLA claim. | 15 min | PLANNED |
| O5 Notification control | Mute/unmute each conversation, remove member, then remove private ntfy mapping. No new dispatch to muted/removed user; sender excluded; other group unaffected; already handed-off alerts may finish. meowGram logout does not log out ntfy. | 10 min | PLANNED |
| O6 Firebase inventory | Google Cloud Console → APIs & Services → Credentials: inventory inherited client keys privately; review required API and platform application restrictions. No code/key rotation; no key values in evidence. | 10 min | PLANNED |
| O7 Firebase validation | Test restrictions with test app/key; retain actually needed APIs. Apply reviewed settings and smoke Android/macOS registration/notifications and relevant web startup; no API_KEY_SERVICE_BLOCKED/403. Revert failing restriction only. | 15 min | PLANNED |
| O8 LLDAP reader | Before bot pilot, verify reader sees uid/entryUUID and allowed bot group/memberOf; current direct group removal denies next request. Owner handles live directory/credential changes. | 10 min | PLANNED |
| O9 Ingress | Verify Traefik/Authelia forward Authorization to /api/* unchanged for bearer/Basic, no SSO redirect/interception, overwrite X-Forwarded-Proto; backend inaccessible to untrusted peers. | 15 min | PLANNED |
| O10 Trust/budgets | Trusted CIDRs include actual proxy peers only; spoofed forwarding denied. Bot budgets sustain 20 scope=member polls plus >=10 replies/min. Verified LDAPS CA/SAN and private readable mounts retained. | 10 min | PLANNED |
| O11 Backup | Quiesce writers, pin actual prior server/web images, complete real pre-R2.1 dump, copy off-node/check checksum. Record exact identity privately. | 15 min | PLANNED |
| O12 Restore drill | Restore that exact dump into isolated database, compare counts/history; scratch synthetic CI alone is insufficient. | 15 min + unattended | PLANNED |
| O13 Lounge-first upgrade | Upgrade only after backup/restore proof; smoke old/new clients on lounge before any group message. Capture clean schema and image digests; zero-group rollback window remains open. | 15 min | PLANNED |
| O14 Groups/mixed clients | Then start using groups: unnamed reuse/named separate, human member rename/add/remove/leave/mute; A/B canaries absent on v2.0.0 paths, present for authorized new clients. Last-human leave denies remaining bots. | 15 min | PLANNED |
| O15 pourOver pilot | Human adds existing typed bot ID first; poll scope=member; replies copy triggering channel_id; prefix-free only two-member group with bot; larger groups/lounge use prefix. Retry/restart once, no fallback on 403, LDAP revoke denies. Use deliberate history baseline. | 15 min | PLANNED |
| O16 Recovery | Before any image revert assert zero non-null messages AND zero channel-specific replay rows under tested guarded procedure. After first group message fix forward or explicitly authorize verified restore losing ALL post-backup changes. | 10 min | PLANNED |

Source references reviewed during original planning: [Firebase API-key guidance](https://firebase.google.com/docs/projects/api-keys)
and [ntfy iOS configuration](https://docs.ntfy.sh/config/#ios-instant-notifications).
These are guidance, not inspection of the owner's systems. All live operations,
credentials, external configuration and publication remain separately owner-gated.
The docs-only evidence exception applies to #51 alone, not these checks or future
implementation. No pending R2 ingress/directory prerequisite is waived by this plan.
