$ErrorActionPreference = 'Stop'
$repo = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$tempRoot = [IO.Path]::GetFullPath([IO.Path]::GetTempPath())
$project = 'meowgram-rollback-' + [guid]::NewGuid().ToString('N')
$scratch = Join-Path $tempRoot $project
$compose = Join-Path $repo 'deploy/rollback.compose.yml'
New-Item -ItemType Directory -Path $scratch | Out-Null
try {
 foreach ($release in @(@{Context='previous'; Tag='v1.1.0'}, @{Context='r2'; Tag='v2.0.0'})) {
  $context = Join-Path $scratch $release.Context
  $archive = Join-Path $scratch ($release.Context + '.tar')
  New-Item -ItemType Directory -Path $context | Out-Null
  git -C $repo archive --format=tar --output=$archive $release.Tag server
  if ($LASTEXITCODE -ne 0) { throw "Cannot export $($release.Tag)." }
  tar -xf $archive -C $context
  if ($LASTEXITCODE -ne 0) { throw "Cannot extract $($release.Tag)." }
 }
 docker build --build-context "previous=$(Join-Path $scratch 'previous')" --build-context "r2=$(Join-Path $scratch 'r2')" -f "$repo/deploy/rollback/Dockerfile" -t meowgram-rollback:verification $repo
 if ($LASTEXITCODE -ne 0) { throw 'Rollback fixture build failed.' }
 docker compose -p $project -f $compose run --rm rollback
 if ($LASTEXITCODE -ne 0) { throw 'Rollback drill failed.' }
} finally {
 try {
  docker compose -p $project -f $compose down --volumes --remove-orphans
  if ($LASTEXITCODE -ne 0) { throw 'Rollback scratch cleanup failed.' }
 } finally {
  # Check the final absolute path is one generated child of this temp root.
  # GetRelativePath handles Windows and POSIX separators in PowerShell Core.
  $resolved = [IO.Path]::GetFullPath($scratch)
  $relative = [IO.Path]::GetRelativePath($tempRoot, $resolved)
  if ($relative -notmatch '^meowgram-rollback-[0-9a-f]{32}$') { throw 'Unsafe cleanup path.' }
  Remove-Item -LiteralPath $resolved -Recurse -Force
 }
}
