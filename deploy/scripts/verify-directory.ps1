$ErrorActionPreference = 'Stop'
$context = Join-Path $PSScriptRoot '../directory-verification'
$image = 'meowgram-directory-verification'
& docker build -t $image $context
if ($LASTEXITCODE -ne 0) { throw 'Directory verification image build failed.' }
& docker run --rm --network none --read-only --tmpfs /tmp:rw,noexec,nosuid,size=128m --cap-drop ALL --security-opt no-new-privileges $image
if ($LASTEXITCODE -ne 0) { throw 'Directory compatibility probe failed.' }
