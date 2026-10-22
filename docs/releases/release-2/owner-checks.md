# Release 2 owner checks

All live checks below are **NOT RUN** by the agent. Scratch evidence is separate
in verification-matrix.md and story logs. Run the pre-bot upgrade with
BOT_BASIC_AUTH_ENABLED=false. Bot activation needs separate owner approval.
Exact commands and recovery limits: [runbook rollout procedure](rollout.md).

| Order / check | Time | Expected result |
|---|---|---|
| 1. GHCR access | 5–10 min | After reviewing the image-inspection evidence, make meowgram-server and meowgram-web public OR provision a private read:packages token for cellar. Pull both exact 2.0.0 tags; record digests. No token in logs, Git or chat. Local candidate inspection passed: see image-inspection.md; recheck if published inputs/artifacts differ. |
| 2. Backup and previous image identity | 15–30 min | Completed real dump copied off-node with matching checksum, restored in isolated PG with matching message count; actual previous release and both image IDs recorded/tagged pre-r2. v1.1.0 compatibility proof does not identify the running image. |
| 3. Pre-bot human smoke | 15 min | Health 200, OIDC login, old history visible, phone and web sends persist/reconnect once, no Bot label on human messages. Basic remains false. |
| 4. Least-privilege LLDAP reader | 10 min | Reader can see uid, entryUUID and direct memberOf for the allowed bot group under the people base; configured namespace stays stable. Valid member passes; nonmember gets 403; removal appears on next request. Reader secret remains in a private host-local mounted file. |
| 5. LDAPS certificate and hostname | 5–10 min | Verified CA chain and SAN match BOT_LDAP_URL; wrong CA/hostname fail closed. Server UID 10001 can read the CA and password files; they are not baked into images. |
| 6. Traefik/Authelia boundary | 10–15 min | Authorization reaches /api/* unchanged for Basic and human bearer; no Basic-to-SSO interception/redirect. Proxy overwrites X-Forwarded-Proto and backend remains unreachable from untrusted peers. Use sanitized status evidence, never log raw Authorization. External config changes require owner action. |
| 7. Trusted CIDRs and budgets | 5 min | BOT_TRUSTED_PROXY_CIDRS lists actual proxy peers only; spoofed forwarding from other peers cannot admit Basic. API budget >=30/min per bot and shared ingress >=60/min allow one 3-second poller plus replies. |
| 8. Dedicated bot account | 5–10 min | Owner creates LLDAP login uid and direct allowed-group membership; password stored privately. Renaming display name leaves username stable; record meowGram principal sender_id from a controlled send, not LDAP UUID. Human/bot usernames need not be unique across kinds. |
| 9. pourOver pilot | 15–20 min | One instance, persistent private cursor/state, HTTPS URLs, MEOWGRAM_CHANNEL empty. New human message produces one plain @username reply, Bot label and no self collision. Replay does not duplicate; changed body 409; own messages ignored by kind+ID; 429/503 back off. Group removal denies the next request. |
| 10. Devices and stop drill | 15 min per platform | Reconnect preserves order/no duplicates, existing push reaches an authorized physical device. Stop bot, retain state, disable Basic and recreate current server; human login/send/sync/WS still work. No image rollback needed. |

Do not point v1.1.0 at version 6 directly. Keep-data rollback requires the guarded
version reset AND zero bot messages; after bot traffic use current image with
Basic disabled or explicitly authorized full restore (losing post-backup data).
Native install/provider delivery are owner checks, not inferred from Flutter tests.

Pilot placeholder values, reviewed starting cursor (avoid replying to all old
history), smoke invocation and stop procedure are in rollout.md. R3 media M1–M4
will be combined here only when the media epic reaches its future release.
