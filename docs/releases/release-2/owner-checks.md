# R2 media owner checks

Media-specific checks from UST-2.1.0. Preserve these alongside the identity and
ingress checks arriving through the playbook PR when the epics integrate.
No live provider, account or credential was accessed for the spike.

## M1 Provider version

Not run: the deployed Immich version is not recorded in this repo and live fleet
inspection requires owner authorization. Time budget: 5 minutes.

1. On the existing authorized Immich UI, read the server version.
2. Compare it with the spike's v3.3.3 baseline; report the version, not a token.
3. Expected: exact baseline, or a recorded mismatch for a tested adapter update
   before enabling media. No upgrade is authorized by this check.

## M2 Scoped service account

Not run: creating live accounts/API keys requires owner authorization. Time
budget: 10 minutes, after backend implementation is reviewed.

1. Under an explicitly authorized provisioning change, create/select a dedicated
   non-admin meowGram Immich account, not a person's photo library.
2. Grant its API key only asset.upload/read/view/download and user.read. Store
   it only in the host-local ignored secret file; never paste it into chat.
3. Expected: version/account probe succeeds, required media calls work and admin
   operations are unavailable. TLS verification succeeds with the configured CA.

## M3 Runtime and failure acceptance

Not run in this documentation-only spike: no disposable Immich runtime was
started. The backend story must build its pinned isolated fixture; this entry
does not waive that requirement. Live pilot budget: 15 minutes after authorization.

1. Upload a synthetic image/video through meowGram and view it as a second human.
2. Confirm a bot and a human attempting another user's unattached local ID are
   denied; no raw Immich URL/key appears in network responses.
3. Exercise a provider outage in the disposable fixture, not the fleet. Expected:
   visible retry state, no phantom chat message, human text chat unaffected.
4. Review any orphan upload records before approving a separate cleanup action.

## M4 Device and codec acceptance

Not run: requires supported physical/native clients and the implemented UI.
Time budget: 15 minutes per platform.

1. On Android/iOS, capture a photo and choose a short video. On web/Windows/macOS,
   use the file picker. Confirm progress, cancellation, failure and retry.
2. Open thumbnails/lightbox and play/download the video; test a seek where Range
   playback is used and the bounded-download fallback where headers are unsupported.
3. Go offline, restart and inspect cached timeline/thumbnail behavior; attempt an
   upload, then reconnect. Expected: clear pending/failed state, no silent loss.
4. Sign out and switch accounts. Expected: account caches/object URLs cleared,
   no old protected preview visible to the new account.
