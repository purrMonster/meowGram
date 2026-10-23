# R2.1 owner checks — draft, all NOT RUN

Separate from scratch verification. No live service, credential, Firebase console,
ntfy, LLDAP, Authelia or deployment change is authorized by this planning session.
Perform checks after implementation and a separately authorized rollout window.
Each step is <=15 minutes hands-on; large restores/CI may need unattended time.
Keep actual identities, hostnames, keys, topics and screenshots with sensitive
values in private records, never in the public repository.

1. **Plan decisions (10 min).** Review README and both ADRs. Record accepted
   membership/startup options, replay exception, cache attribution and playbook
   translation. Expected: explicit authorization before 000007 or release branches.
2. **ntfy inventory (5 min).** Record installed server version/image digest, iOS
   app version and enabled upstream privately; provide sanitized version information
   for the contract spike. Expected: tested fixture version matches deployment;
   no assumed latest version. External configuration remains owner-operated.
3. **ntfy ACL/relay (15 min).** Provision publisher write-only and each user
   read-only to their unique topic; deny anonymous access. With synthetic alerts,
   verify another user cannot read/publish that topic and own user can fetch it.
   Enable/review upstream configuration. Expected: only generic New message body,
   no message text/sender/channel data; understand relay timing/routing metadata.
4. **iPhone delivery (15 min).** Install/configure ntfy iOS app, subscribe with
   private credentials, lock phone and send synthetic authorized-channel traffic.
   Repeat with meowGram stopped, then with ntfy temporarily unreachable and recovery.
   Expected: generic alert arrives via ntfy, no dependency on meowGram background
   polling/APNs; app sync catches up after recovery. Record latency, not a universal SLA.
5. **Revocation/notification control (10 min).** Remove channel membership, send
   again after successful removal; check HTTP 403 and no new channel alert. Remove
   user's private ntfy mapping and verify dispatch stops. Expected: prior authorized
   in-flight alerts may arrive, future dispatch denied. meowGram logout alone does
   not sign out the separate ntfy subscription; confirm this is understood.
6. **Firebase restrictions inventory (10 min, owner only).** In Google Cloud
   Console, APIs & Services → Credentials, identify client keys by platform without
   copying values into evidence. Review API allowlists and application restrictions
   supported by each client (web referrers, Android package/certificate, Apple bundle
   identifiers as applicable). Expected: only required APIs allowed; no unrelated
   billable API access. Do not remove an API merely because its name seems unfamiliar.
7. **Firebase safe validation (15 min).** Test proposed restrictions with a test
   key/app first; retain necessary Firebase Installations/other actually used APIs.
   Apply reviewed restrictions and smoke existing Android/macOS notification/token
   registration and relevant web startup. Expected: no API_KEY_SERVICE_BLOCKED/403;
   revert only the failing restriction if needed, record without key values. No
   source edit, key rotation or secret-handling task is part of R2.1 scope D.
8. **Backup identity (15 min).** Before upgrade, quiesce writers, save exact previous
   server/web image identities and a completed real dump; copy off-node and compare
   checksum. Expected: known pre-R2.1 recovery inputs, not assumed tag identity.
9. **Actual restore drill (15 min plus unattended restore).** Restore that dump
   into an isolated database and compare counts. Expected: readable matching data;
   synthetic CI restore alone is not proof of the owner's dump.
10. **Mixed clients (15 min).** With v2.0.0 and new app signed in, send lounge/A/B
    messages as separate test principals. Expected: old app sees lounge only; new
    members see their channels; nonmember gets 403; old background notification/cache
    never contains private test text. Check new phone offline/reconnect and account switch.
11. **pourOver (15 min).** Add its existing typed bot ID to one channel, set its
    private channel parameter/state, test polling and idempotent reply/restart.
    Expected: correct channel_id each time, inaccessible channel 403, lounge still
    works, revoked LDAP eligibility blocks next request, no silent lounge retry.
12. **Recovery boundary and release record (10 min).** Read tested final rollback
    procedure and record whether any private messages exist. Expected: no bare image
    downgrade after private traffic; prefer roll-forward, or explicitly approve
    restore with loss of all post-backup changes. Record pulled image digests and
    separate approval for enabling ntfy/bots/new channel writes.

Sources checked 2026-10-23: [Firebase API-key guidance](https://firebase.google.com/docs/projects/api-keys)
explains that client keys identify the project rather than authorize backend access,
and recommends restrictions with testing; [ntfy configuration](https://docs.ntfy.sh/config/#ios-instant-notifications)
describes self-hosted iOS relay delivery. These are guidance, not inspection of the
owner's console or ntfy installation.

Local test-environment check: start an authorized disposable Docker environment or
use PR CI to run the unchanged baseline suites; this session's Docker engine is
unavailable. New R2.1 behavior remains unimplemented and unverified regardless of
baseline CI outcome. See verification-matrix.md and the planning log.
