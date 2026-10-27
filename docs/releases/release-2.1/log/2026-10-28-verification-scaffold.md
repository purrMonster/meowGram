# Independent legacy decoder scaffolding — 2026-10-28 IST

owner: Fast-track explicitly permits independent harness scaffolding for 5.1/5.2
while dependency merges are pending.

Branch test/r2.1-verification-scaffold starts from and targets
test/r2.1-epic-5-verification at fetched 1f39e07. No sibling implementation was
merged/cherry-picked. This completes only a reusable probe, not UST-2.1.5.1/5.2.
The verification matrix remains PLANNED for those stories.

Decision: export actual tag v2.0.0 with git archive, compile a small command in
the exported module against its unchanged internal/model.Message and WSMessage,
then execute that command from race-enabled assertions. No old model is copied
into the current server. PowerShell and POSIX wrappers verify the expected tag
commit; an independent CI job fetches tags and runs both timezones.

Source identity:
- v2.0.0 commit: be15cec6ceed2cb4a21b69cfe5e4548d2bfd818c.
- server/internal/model/message.go blob: d04bd31e06b12c28245e8105e5ed95677c2c71fd.

The actual old model decoder retains the lounge positive control, but also
retains a deliberately supplied private canary while discarding channel_id.
This proves the future privacy harness cannot treat missing output channel_id
as evidence of non-delivery. It does not prove any current server leak.
The probe handles message, history-array and live frames; malformed/trailing
input is rejected. Actual Flutter/cache/background, authorization, provider,
startup and recovery assertions remain future integrated work.

Commands, run from repository root:
```powershell
pwsh -NoProfile -File deploy/scripts/verify-legacy-decoder.ps1
$project='meowgram-r21-verification-scaffold-20261028'
foreach ($zone in @('UTC','Asia/Kolkata')) {
 docker compose -p $project -f deploy/verify.compose.yml run --rm -e "TZ=$zone" server sh -ec 'go version && go test -json -race -count=1 ./... && go vet ./...'
}
foreach ($service in @('restore','client','web-smoke')) {
 docker compose -p $project -f deploy/verify.compose.yml run --build --rm $service
}
docker compose -p $project -f deploy/verify.compose.yml down --volumes --remove-orphans
git diff --check
```

Fresh results:
- Actual decoder UTC: 7 subchecks + parent, zero skips;
  `--- PASS: TestActualV200Decoder (0.07s)`.
- Actual decoder Asia/Kolkata: 7 subchecks + parent, zero skips;
  `--- PASS: TestActualV200Decoder (0.10s)`.
- Probe vet passes in Docker build. First build caught an unused test import;
  removed it, then rebuilt/reran all probe checks successfully.
- Full Go UTC and Asia/Kolkata: **166 passes each**, zero failures, vet clean.
  Three existing generic-suite skips per timezone: TestAPIEndToEnd,
  TestReleaseRollback, TestLLDAP require dedicated environments. Those old dedicated
  harnesses were not rerun for this scaffold-only change; no fresh pass claimed.
- Flutter analyze: `No issues found! (ran in 10.9s)`.
  `00:05 +83: All tests passed!`; `✓ Built build/web` (40.4s).
- `Backup/restore drill passed (probe and message count match).`
- `Web runtime passed: non-root, SPA fallback, no-store HTML and bootstrap asset.`
- Overall wrapper exit 0: `Scaffold full regression stages passed.`
- PowerShell parser clean; POSIX `sh -n` passed inside the Go container.
  Full POSIX wrapper awaits CI; PowerShell exercised archive/build/runs/cleanup.
  Local MSYS syntax-only attempt was sandbox-blocked, resolved with container sh.
- Optional Android packaging not run. No client or production source changed.
- Scratch project/resources removed; decoder wrapper removes its unique image
  and generated export. No live services, credentials or deployment changes.

Image identities:
- Go1.27.1 pinned digest: sha256:162be5298a40ed317005c8339c6de4d10d3eef336d66dc8e9259b03ab9d3a6d2.
- PostgreSQL16: sha256:f4610795587492cd4fb887c2dc3de301b813278bb6f616992061141b5f15ac36.
- Decoder fixture: sha256:353bfe1e23c57368a2f9b2232f20df66bd3640529797ede29efa6c7d5dee6e74.
- Client verification: sha256:bf8de36fbddfbe1ab07922658355551d13855134e780230f951623bb89d8eab4.
- Web smoke: sha256:40e8e88871f6c42908e529abca1f6a61392a48a3efd9c6488c0aa8a0ae82fdca.

Raw logs retained locally in ../verification-scaffold-evidence/.
Local image identities are not published release or deployment claims.

Published #60 into test/r2.1-epic-5-verification; no integration branch push/merge.
Where we stopped / next step: scaffold verified, pushed and queued; full privacy
work remains blocked on owner-integrated epics 1–4.

