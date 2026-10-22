$ErrorActionPreference = 'Stop'
$repo = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$scratch = Join-Path ([IO.Path]::GetTempPath()) ('meowgram-rollback-' + [guid]::NewGuid())
$compose = Join-Path $repo 'deploy/rollback.compose.yml'
New-Item -ItemType Directory -Path $scratch | Out-Null
try {
 git -C $repo archive --format=tar --output="$scratch/previous.tar" v1.1.0 server
 if ($LASTEXITCODE -ne 0) { throw 'Cannot export v1.1.0.' }
 tar -xf "$scratch/previous.tar" -C $scratch
 if ($LASTEXITCODE -ne 0) { throw 'Cannot extract previous release.' }
 docker build --build-context "previous=$scratch" -f "$repo/deploy/rollback/Dockerfile" -t meowgram-rollback:verification $repo
 if ($LASTEXITCODE -ne 0) { throw 'Rollback fixture build failed.' }
 docker compose -p meowgram-rollback -f $compose run --rm rollback
 if ($LASTEXITCODE -ne 0) { throw 'Rollback drill failed.' }
} finally {
 docker compose -p meowgram-rollback -f $compose down --volumes --remove-orphans
 # Only this invocation's generated, absolute temporary directory is removed.
 $resolved = [IO.Path]::GetFullPath($scratch)
 $prefix = [IO.Path]::GetFullPath([IO.Path]::GetTempPath()).TrimEnd('\') + '\meowgram-rollback-'
 if (-not $resolved.StartsWith($prefix, [StringComparison]::OrdinalIgnoreCase)) { throw 'Unsafe cleanup path.' }
 Remove-Item -LiteralPath $resolved -Recurse -Force
}
