$ErrorActionPreference = 'Stop'
$root = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$source = (& git -C $root rev-parse 'v2.0.0^{}').Trim()
if ($LASTEXITCODE -ne 0 -or $source -ne 'be15cec6ceed2cb4a21b69cfe5e4548d2bfd818c') { throw 'Expected actual v2.0.0 tag is unavailable or changed.' }
$suffix = [guid]::NewGuid().ToString('N')
$tempRoot = [IO.Path]::GetFullPath([IO.Path]::GetTempPath())
$scratch = Join-Path $tempRoot "meowgram-legacy-decoder-$suffix"
$image = "meowgram-legacy-decoder:$suffix"
New-Item -ItemType Directory -Path $scratch | Out-Null
try {
 $archive = Join-Path $scratch 'source.tar'
 & git -C $root archive --format=tar "--output=$archive" $source
 if ($LASTEXITCODE -ne 0) { throw 'Tag archive failed.' }
 & tar -xf $archive -C $scratch
 if ($LASTEXITCODE -ne 0) { throw 'Tag extraction failed.' }
 Write-Host "Actual legacy source: $source"
 & docker build --build-context "legacy=$scratch" --build-arg "LEGACY_COMMIT=$source" -f (Join-Path $root 'deploy/legacy-decoder/Dockerfile') -t $image $root
 if ($LASTEXITCODE -ne 0) { throw 'Legacy decoder build failed.' }
 foreach ($zone in @('UTC','Asia/Kolkata')) {
  Write-Host "Legacy decoder TZ=$zone"
  & docker run --rm --network none --read-only --tmpfs /tmp:rw,noexec,nosuid,size=16m --cap-drop ALL --security-opt no-new-privileges -e "TZ=$zone" $image
  if ($LASTEXITCODE -ne 0) { throw "Legacy decoder failed in $zone." }
 }
 & docker image inspect $image --format '{{.Id}}'
} finally {
 & docker image rm $image 2>$null | Out-Null
 $resolved = [IO.Path]::GetFullPath($scratch)
 if ([IO.Path]::GetDirectoryName($resolved) -ne $tempRoot.TrimEnd([IO.Path]::DirectorySeparatorChar) -or [IO.Path]::GetFileName($resolved) -ne "meowgram-legacy-decoder-$suffix") { throw 'Refusing cleanup outside generated scratch path.' }
 Remove-Item -LiteralPath $resolved -Recurse -Force
}

