$ErrorActionPreference = 'Stop'
$root = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
docker build -f (Join-Path $root 'deploy/ntfy/Dockerfile') -t meowgram-ntfy-contract:local $root
if ($LASTEXITCODE -ne 0) { throw 'ntfy fixture build failed.' }
foreach ($zone in @('UTC', 'Asia/Kolkata')) {
    docker run --rm --network none --read-only --tmpfs /tmp -e "TZ=$zone" meowgram-ntfy-contract:local
    if ($LASTEXITCODE -ne 0) { throw "ntfy contract failed in $zone." }
}
