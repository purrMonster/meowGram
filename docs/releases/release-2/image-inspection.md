# R2 local release-image inspection — 2026-10-22

Built the production Dockerfiles with their domain-free defaults, without local
Compose build arguments. Inspected the final images, not only their source.
No publication, package-visibility change or deployment occurred.

| Image / local tag | `docker inspect .Id` | Saved layers / regular file entries |
|---|---|---|
| meowgram-server:readiness-inspect | sha256:7102aaae708a335c6c3ff1da57b0f79892f6dcdd3235f71c4a5803c4d9858957 | 6 / 580 |
| meowgram-web:readiness-inspect | sha256:c40ebcf02b6caae2c3d45d0ed811ced3a797cbf2cf7c5d8b170743d3dd5e7755 | 10 / 1,052 |

Commands executed from the repository:

```sh
docker build -t meowgram-server:readiness-inspect -f server/Dockerfile server
docker build -t meowgram-web:readiness-inspect -f client/Dockerfile.web client
# For each image:
docker history --no-trunc <image>
docker inspect <image>
docker save --output <component>.tar <image>
```

Read the manifest, enumerated every member of every saved layer (including files
hidden/deleted by later layers), and scanned regular-file bytes. Reviewed full
history, Env, Labels, file listings, URL-host inventory, private-key PEM markers,
token-shaped strings, credential/keystore/.env filenames, user-home paths and
deployment-name markers. Inspected production build inputs and COPY boundaries.
Local logs, JSON configurations, file lists and image archives are under
Temp/meowgram-image-audit; source build logs are Temp/meowgram-image-*-build.log.

Results:
- No `.env`, credential files, private-key PEMs, application keystores, host
  personal paths or deployment-specific endpoint hostnames found in either
  final image's saved layers. Public CA certificates and package signing public
  keys are expected and are not private keys.
- Server Env contains only PATH, PORT=8080, APP_DOMAIN=localhost,
  ENVIRONMENT=production, LOG_LEVEL=info, MIGRATIONS_PATH=/migrations; no Labels.
  Final app content is the binary and migrations, plus base packages/user files.
- Web Env contains PATH and Nginx/package version values. Its only inherited
  label is the public Nginx maintainer contact. App content is built static web
  output and nginx configuration; no project source, local configuration or
  credentials are copied into the final stage. Deployment-specific Firebase
  identifiers are absent from main.dart.js.
- URL inventory contains upstream library/documentation/CDN/provider endpoints,
  localhost/placeholders and false fragments from binary string tables; these
  are not household deployment hostnames. The web bundle's two purrBrews matches
  are the existing login-button/loading text, not addresses or personal data.
- One token-shaped match was a false positive in libunistring's concatenated
  Unicode name table. The file SHA256
  `019724bf2df8b72edb83669318f1857bee8fb5753bcf176c9ca487e24b91bff6`
  matches the upstream nginx:1.29-alpine file exactly. No token value was logged.
- History records ordinary build/copy/package commands and public upstream
  URLs, with no supplied secrets or private build arguments.

**Recommendation:** public packages are reasonable for these inspected image
contents; private packages with a read:packages token remain an owner option.
This is a bounded inspection, not proof against every possible secret encoding.
The local images are linux/amd64 candidates, not pulled GHCR release digests.
The owner must record the actual published version/digests and recheck if build
inputs or artifacts differ; do not substitute these IDs for rollout digests.
CI adds public source/revision/version/license labels and builds amd64/arm64.
