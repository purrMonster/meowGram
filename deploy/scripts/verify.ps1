param([switch]$KeepCaches)
$ErrorActionPreference = 'Stop'
$compose = Join-Path $PSScriptRoot '../verify.compose.yml'
$project = 'meowgram-verify-' + [guid]::NewGuid().ToString('N').Substring(0, 10)
function Invoke-Compose {
    & docker compose -p $project -f $compose @args
    if ($LASTEXITCODE -ne 0) { throw "Scratch verification failed (exit $LASTEXITCODE)." }
}
try {
    Invoke-Compose run --rm server
    Invoke-Compose run --rm restore
    Invoke-Compose run --build --rm client
} finally {
    if ($KeepCaches) { & docker compose -p $project -f $compose down --remove-orphans }
    else { & docker compose -p $project -f $compose down --volumes --remove-orphans }
}
