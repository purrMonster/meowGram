$ErrorActionPreference = 'Stop'
$root = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
docker build -f (Join-Path $root 'deploy/bot-auth-verification/Dockerfile') -t meowgram-bot-auth-verification $root
if ($LASTEXITCODE -ne 0) { throw 'Bot authentication verification image failed to build.' }
docker run --rm --network none --read-only --tmpfs /tmp:rw,noexec,nosuid,size=128m --cap-drop ALL --security-opt no-new-privileges meowgram-bot-auth-verification
if ($LASTEXITCODE -ne 0) { throw 'Bot authentication verification failed.' }
