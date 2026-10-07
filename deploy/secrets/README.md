# deploy/secrets

Host-local secrets, mounted read-only into the server container at `/run/secrets`.
Everything here except this README is gitignored.

- `fcm-service-account.json`: Firebase service account for push notifications.
  Set `GOOGLE_APPLICATION_CREDENTIALS=/run/secrets/fcm-service-account.json` in
  `deploy/.env`. Leave it unset to disable push.
