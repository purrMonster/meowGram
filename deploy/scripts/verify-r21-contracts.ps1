$ErrorActionPreference = 'Stop'
$compose = Join-Path $PSScriptRoot '../r21-contracts.compose.yml'
try {
    docker compose -p meowgram-r21-contract -f $compose build contracts
    if ($LASTEXITCODE -ne 0) { throw 'R2.1 contract fixture build failed.' }
    foreach ($zone in @('UTC', 'Asia/Kolkata')) {
        Write-Host "R2.1 contract TZ=$zone"
        docker compose -p meowgram-r21-contract -f $compose run --rm -e "TZ=$zone" contracts
        if ($LASTEXITCODE -ne 0) { throw "R2.1 HTTP contracts failed in $zone." }
    }
} finally {
    docker compose -p meowgram-r21-contract -f $compose down --volumes --remove-orphans
}
