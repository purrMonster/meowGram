# R2 media contract — UST-2.1.0

Decision date: 2026-10-18. One-session contract spike, not a deployed feature.
Branch: `docs/ust-2.1.0-immich-contract`, parent/PR target:
`feat/r2-epic-2.1-media-backend` at `0e41333`.

## Provider baseline and evidence

**Assumption:** implement against Immich **v3.3.3**, not a floating `release`
image. The official release API reported that stable tag, published 2026-10-14.
Its source commit is `d2dac658fe1c67e8bbd08aa18200a32e5d45b9e4`.
This is an implementation baseline, not a claim about the household deployment.
Deployment version confirmation is [owner check M1](owner-checks.md#m1-provider-version).
Do not silently use this adapter against a different major version; compare the
actual provider contract and add a tested adapter if required.

Inspected primary sources:

- [Release v3.3.3](https://github.com/immich-app/immich/releases/tag/v3.3.3).
- [Pinned OpenAPI schema](https://github.com/immich-app/immich/blob/d2dac658fe1c67e8bbd08aa18200a32e5d45b9e4/open-api/immich-openapi-specs.json).
- [Media controller](https://github.com/immich-app/immich/blob/d2dac658fe1c67e8bbd08aa18200a32e5d45b9e4/server/src/controllers/asset-media.controller.ts),
  [media service](https://github.com/immich-app/immich/blob/d2dac658fe1c67e8bbd08aa18200a32e5d45b9e4/server/src/services/asset-media.service.ts),
  [upload DTO](https://github.com/immich-app/immich/blob/d2dac658fe1c67e8bbd08aa18200a32e5d45b9e4/server/src/dtos/asset-media.dto.ts).
- [Pinned Compose source](https://github.com/immich-app/immich/blob/d2dac658fe1c67e8bbd08aa18200a32e5d45b9e4/docker/docker-compose.yml).

Executed schema check: **PASS 7 pinned Immich operations and 3 required upload
fields verified against v3.3.3 OpenAPI.** This verifies the static provider
contract only. Runtime-provider checks belong to M1–M3 and the backend harness.

## Calls and credentials

Paths below are relative to configured `IMMICH_API_URL`, which includes `/api`.
They are adapter defaults; an operator-supplied base URL is never taken from a
request. The old roadmap `/server-info/version` path is replaced by `/server/version`.

| Method and path | Operation | Key permission / use |
|---|---|---|
| GET `/server/version` | `getServerVersion` | Public version check; does not prove key validity |
| GET `/users/me` | `getMyUser` | `user.read`; validate the dedicated service account |
| POST `/assets` | `uploadAsset` | `asset.upload`; multipart stream |
| GET `/assets/{id}` | `getAssetInfo` | `asset.read`; narrowly map type/dimensions/status |
| GET `/assets/{id}/thumbnail?size=thumbnail` | `viewAsset` | `asset.view`; preview uses `size=preview` |
| GET `/assets/{id}/original` | `downloadAsset` | `asset.download`; original image/download |
| GET `/assets/{id}/video/playback` | `playAssetVideo` | `asset.view`; video stream, including byte ranges |

Use one dedicated, non-admin Immich account for meowGram uploads. Read its
scoped API key from a host-local secret file and send only `x-api-key` upstream.
Required grants: `asset.upload`, `asset.read`, `asset.view`, `asset.download`,
`user.read`. No admin, shared-link, album or delete permission is required.
Never forward a client's Authorization, cookies, query credentials or arbitrary
headers to Immich. Never expose its key, internal URL, filesystem paths, people
metadata or raw metadata response to Flutter. Provisioning is owner-only (M2).

TLS verification is mandatory for non-loopback provider URLs; no skip-verify
switch. The development fixture may use isolated loopback/container HTTP only.
Redirects are not followed by the generic credential-bearing HTTP client.
Thumbnail redirects may be resolved explicitly, at most twice, only within the
configured origin/base and the same validated asset's thumbnail/original paths;
credentials and Location headers never reach another origin or the client.

## meowGram authorization and local IDs

All media routes use existing human OIDC middleware and budgets. Basic-auth bots
cannot upload, inspect metadata, retrieve thumbnails, download, or play media.
Check this before any provider call; add a route-isolation test for every route.

The lounge is shared: every currently authenticated human allowed into the
lounge may fetch media linked to a committed lounge message. An unattached
upload is visible only to its uploader. Unknown, unlinked-to-another-user,
expired or inaccessible local IDs return the same 404. UUID knowledge is not
authorization. No endpoint accepts an arbitrary Immich ID or external URL.

Use additive local `media_assets` rows: UUID, uploader OIDC subject, provider
namespace/asset UUID, MIME, size, state, timestamps and nullable message FK.
Wire messages expose local media UUIDs, type and dimensions, not provider IDs.
An attachment can be linked to one message only, with uploader and state checked
under a row lock in the same transaction as message persistence. Commit before
broadcast. Keep existing message IDs, sender FKs, UTC ordering and UUID dedup.
Missing attachment fields remain compatible with old text messages/caches.

Multiple local uploads may refer to one deduplicated provider asset. Do not
automatically delete provider assets on failure/expiry: another message may
reference the same bytes. Retain orphan records for owner reconciliation (M3).
No destructive migration or provider-delete credential is part of this contract.

## Upload, attach and fetch flow

1. Flutter selects/captures one image or video, validates the advertised limits,
   compresses supported images, and shows upload progress/cancel/retry states.
   It retains the selected source until success or explicit cancellation.
2. Human-only `POST /api/media` takes one multipart `file` and bounded timestamps.
   A client UUID operation key scopes retry state to the authenticated uploader.
   Reserve a local upload row first; reject concurrent attempts for the same key
   with 409/Retry-After, and return the stored result once completed. A changed
   payload for a completed key must not overwrite the original upload.
3. Validate the file signature and allowlist with a bounded prefix buffer, then
   stream through a multipart writer and `io.Pipe` to Immich. Do not use
   `ReadAll` or whole-file `ParseMultipartForm`. Count and hash bytes while
   streaming, enforce the limit even without Content-Length, and cancel both
   directions on disconnect/error. Await the copy result before recording success.
4. Upstream multipart uses `assetData`, `fileCreatedAt`, `fileModifiedAt`.
   Timestamps are validated UTC instants; missing local timestamps use upload
   time. Generate a safe filename from the local UUID and detected type.
   Device IDs are not required by this pinned schema. Do not forward sidecars,
   owner/album IDs, visibility settings or arbitrary metadata from the caller.
5. Accept Immich 201/`created` or 200/`duplicate` with a valid UUID. Persist the
   local result before returning its local ID. Provider deduplication is not
   message idempotency. A timeout after upstream acceptance can leave an orphan;
   a retry reuses the local operation record and provider byte deduplication.
   Never guess success or publish a message after an ambiguous upstream failure.
6. Attach the local ID through the existing WebSocket message path, with optional
   caption; a media-only message is valid. At most one attachment per message in
   the first implementation. The owner/state/one-message checks happen in the
   message transaction. A retry of an already-linked upload can return its
   message UUID so the client reconciles through normal sync without sending twice.
   Future HTTP send uses the same attachment checks after epic integration; the
   media backend does not depend on unmerged HTTP API task code.
7. `GET /api/media/{localId}` supplies safe status and attachment/message ID.
   `GET /api/media/{localId}/{thumbnail|preview|original|video}` resolves the local
   authorization first, then streams only the mapped provider asset. Support one
   byte Range for original/video; preserve 206, Content-Range, Accept-Ranges and
   validated Content-Type/Length. Reject multipart ranges and invalid ranges.
   Use bounded copy buffers; abort on downstream cancellation or provider truncation.

Immich thumbnails/transcoding can lag upload acceptance. Render a processing
placeholder and retry with bounded backoff; do not treat a missing thumbnail as
a failed original upload. Playback can return the original before a transcode
exists, so codec errors need a visible retry/download fallback. Avoid the pinned
API's alpha HLS routes in the first adapter; byte-range playback retains video
support without introducing streaming-session credentials.

## Limits, browser transport and offline behavior

Initial configurable limits: images **20 MiB**, videos **100 MiB**, multipart
metadata **16 KiB**, image dimensions **64 megapixels**, four concurrent uploads
per server, two per human, and **120s** total upload deadline. Admit JPEG, PNG,
WebP, HEIC/HEIF, MP4 and QuickTime/MOV after signature/type validation. Return
415 for unsupported types; no SVG, HTML or arbitrary executable attachments.
Client MIME/extensions are hints, not authority. Provider-derived previews are
used for image types a platform cannot decode. These are product defaults,
not claims that all codec combinations have been exercised.

Continue human API budgets and apply separate bounded media concurrency before
expensive work. Media deadlines must be route-specific: the current server's
15-second HTTP write timeout cannot silently remain the upload/download limit.
Slow transfers must not relax WebSocket/ticket or global request protections.

Every network fetch carries an OIDC bearer header and reauthorizes the local
media ID. Never put bearer tokens in image/video URLs. Use an authenticated image
loader; web fetches image bytes into revocable object URLs. Native video loaders
may use authenticated Range requests where supported. A platform that cannot
attach headers downloads through the authenticated loader to a bounded local
file (native) or size-capped Blob (web), then plays that local object. Show progress
before playback; do not promise immediate streaming on that fallback. Dispose
objects and clear account-scoped caches on sign-out/account change.

Responses use `Cache-Control: private, no-store` and `X-Content-Type-Options:
nosniff`; browser/service-worker shared caches must not store protected media.
An explicit account-scoped Flutter thumbnail cache may retain already-authorized
bytes for offline viewing. Revocation cannot erase bytes a client has downloaded.
Cached timeline text/metadata still renders before network. Offline upload/send
is visibly pending or failed with a retry action; no silent loss. Image lightbox,
pinch-to-zoom and progressive thumbnails remain in scope. Photo and video
ingestion are retained; platform/codec acceptance is M4.

## Failure mapping

| Condition | meowGram result / recovery |
|---|---|
| Missing/invalid human token or Basic | 401; no provider call |
| Unknown/inaccessible local asset | 404; no provider existence disclosure |
| Malformed multipart/metadata | 400; cancel upstream and retain safe failed state |
| Too large / unsupported type | 413 / 415; stop streaming, no message |
| Budget exhausted / operation in progress | 429 / 409 with Retry-After |
| Disabled, provider unavailable, key rejected, TLS failure | Sanitized 503; human text chat remains available |
| Provider deadline | 504; retain indeterminate upload for retry/reconciliation |
| Derivative not yet available | 503 with Retry-After; show processing placeholder |
| Confirmed provider asset removal | Mark unavailable; retain message/history |
| DB failure after provider upload | 503; no broadcast; retain/reconcile orphan, never auto-delete |

No raw provider error, key, Authorization header or multipart body is logged.
Before response headers, return structured JSON errors; after a partial stream,
terminate it so the client treats the transfer as incomplete.

## Implementation settings and estimate

Proposed settings (not enabled by this documentation PR): `MEDIA_ENABLED=false`,
`MEDIA_ENDPOINT=/api/media`, `IMMICH_API_URL`, `IMMICH_API_KEY_FILE`,
`IMMICH_CA_FILE`, `IMMICH_EXPECTED_VERSION=3.3.3`, plus `MEDIA_IMAGE_MAX_BYTES`,
`MEDIA_VIDEO_MAX_BYTES`, `MEDIA_METADATA_MAX_BYTES`, `MEDIA_IMAGE_MAX_PIXELS`,
`MEDIA_UPLOAD_TIMEOUT`, `MEDIA_MAX_CONCURRENT_UPLOADS`,
`MEDIA_MAX_UPLOADS_PER_USER`, `MEDIA_UNATTACHED_TTL=24h`.
Backend implementation must add config validation, `.env.example`, Compose and
an epic log table together. Keep all settings and credentials out of client
defines; advertise only safe limits and local routes through discovery.

Estimate, including tests: 2.1.1 adapter/auth/config **1–2 days**; 2.1.2 streaming,
local rows/message linking and failures **3–4 days**; 2.2.1 picker/progress/retry
**2–3 days**; 2.2.2 feed/cache/viewer/playback **2–3 days**. Total **8–12 focused
engineering days**, central estimate 10, versus the existing API estimate 6–13
(central 9.5). This spike does not establish a >=2x media/API effort ratio, so
no scope reduction is proposed. A provider-major mismatch or platform blocker
that changes that estimate triggers gate 8.4 before narrowing the release.

Backend verification must fake each upstream failure and demonstrate bounded
memory, cancellation, byte ranges, human ownership and bot rejection. Its real
fixture should pin the server, Valkey and vector-enabled PostgreSQL images from
the selected release, disable unrelated ML downloads, use synthetic files and
an isolated network with no published ports. That fixture is backend-story work;
this spike's outstanding runtime checks are recorded in owner-checks.md.
