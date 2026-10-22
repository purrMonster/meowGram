$ErrorActionPreference = 'Stop'
$compose = Join-Path $PSScriptRoot '../api-acceptance.compose.yml'
try {
 docker compose -p meowgram-api-acceptance -f $compose run --build --rm acceptance
 if ($LASTEXITCODE -ne 0) { throw 'Combined API acceptance failed.' }
} finally {
 docker compose -p meowgram-api-acceptance -f $compose down --volumes --remove-orphans
}
