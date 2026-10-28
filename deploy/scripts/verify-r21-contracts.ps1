$ErrorActionPreference = 'Stop'
$repo = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$tempRoot = [IO.Path]::GetFullPath([IO.Path]::GetTempPath())
$project = 'meowgram-r21-contract-' + [guid]::NewGuid().ToString('N')
$scratch = Join-Path $tempRoot $project
$compose = Join-Path $repo 'deploy/r21-contracts.compose.yml'
New-Item -ItemType Directory -Path (Join-Path $scratch 'r2') -Force | Out-Null
try {
    $archive = Join-Path $scratch 'r2.tar'
    git -C $repo archive --format=tar --output=$archive v2.0.0 server
    if ($LASTEXITCODE -ne 0) { throw 'Cannot export v2.0.0.' }
    tar -xf $archive -C (Join-Path $scratch 'r2')
    if ($LASTEXITCODE -ne 0) { throw 'Cannot extract v2.0.0.' }
    docker build --build-context "r2=$(Join-Path $scratch 'r2')" -f "$repo/deploy/r21-contracts/Dockerfile" -t "$project-contracts" $repo
    if ($LASTEXITCODE -ne 0) { throw 'R2.1 contract fixture build failed.' }
    foreach ($zone in @('UTC', 'Asia/Kolkata')) {
        Write-Host "R2.1 contract TZ=$zone"
        docker compose -p $project -f $compose run --rm -e "TZ=$zone" contracts
        if ($LASTEXITCODE -ne 0) { throw "R2.1 HTTP contracts failed in $zone." }
        # Start each timezone with the actual v2.0.0 schema.
        docker compose -p $project -f $compose down --volumes --remove-orphans
        if ($LASTEXITCODE -ne 0) { throw 'R2.1 scratch cleanup failed.' }
    }
} finally {
    try {
        docker compose -p $project -f $compose down --volumes --remove-orphans
        if ($LASTEXITCODE -ne 0) { throw 'R2.1 scratch cleanup failed.' }
    } finally {
        $resolved = [IO.Path]::GetFullPath($scratch)
        $relative = [IO.Path]::GetRelativePath($tempRoot, $resolved)
        if ($relative -notmatch '^meowgram-r21-contract-[0-9a-f]{32}$') { throw 'Unsafe cleanup path.' }
        Remove-Item -LiteralPath $resolved -Recurse -Force
    }
}
