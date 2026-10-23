# R2 publication verification — 2026-10-23 IST

owner: "verify the release and upgrade the runbook?"
decision: standalone documentation branch docs/r2-release-publication from
origin/main be15cec after the owner merged the full release in #49; PR to main.
No runtime, workflow, migration, AGENTS or playbook changes.

Release run 39834547253: all eight jobs completed successfully, verified using
GitHub job metadata and verify/image job logs. gh release view v2.0.0 confirms
draft=true, five uploaded assets (four installers and SHA256SUMS.txt).
gh release download followed by Get-FileHash SHA256 comparison: all four match.
docker buildx imagetools inspect for both :2.0.0 images confirms linux/amd64 and
linux/arm64 plus attestation manifests; index digests match CI and runbook §14.2.
This does not claim physical-device installation or cellar pull/deployment.

Local deploy/scripts/verify.ps1 attempted but stopped before tests: Docker
Desktop Linux engine pipe is absent. docker version confirms daemon unavailable.
No local Go/Flutter passes or timezone counts claimed. Release CI's verify job
is successful; documentation PR CI will independently run both suites.
One gh run API request timed out; connector read succeeded. Release metadata and
asset download via gh succeeded. No secrets, package visibility, live services,
deployment or draft-publication changes made. Untracked requirement list/ kept.

Where we stopped / next step: record documentation checks, push the branch and
open its owner PR; release draft publication and cellar rollout remain owner gates.
