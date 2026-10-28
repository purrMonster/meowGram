# Actual v2.0.0 decoder probe

Independent scaffolding for UST-2.1.5.1. This does not implement that story or
claim R2.1 privacy, Flutter cache compatibility, server routing or rollback safety.

Run `pwsh -File deploy/scripts/verify-legacy-decoder.ps1` or
`sh deploy/scripts/verify-legacy-decoder.sh`. Both export the actual v2.0.0 tag
at be15cec6ceed2cb4a21b69cfe5e4548d2bfd818c, add only a probe command to the
temporary module, and compile against its unchanged internal/model types.
The pinned Go 1.27.1 image runs vet and race-enabled binary assertions in UTC
and Asia/Kolkata, without networking, credentials, published ports or a database.

The probe accepts one JSON value on stdin, using modes `message`, `history`
(array) and `live`. It emits the old Go message representation as JSON.
`source` prints the compiled tag commit. The wrapper's temporary image is removed
after verification; future acceptance harnesses can reuse the Dockerfile and
`/legacy-decoder` binary against captured HTTP/WebSocket payloads.

Positive controls establish that lounge rows survive decoding, but a synthetic
private row also survives while its channel_id is silently discarded. This is
the danger the future real-server privacy matrix must falsify: absence of a
channel field in old output is **not** proof that private content was withheld.
Malformed/trailing JSON rejection only protects this probe's input framing.

No old source/model is copied into the current server module, and no current
server feature or fixture from another epic is imported. The actual Flutter
decoder/cache and every transport/provider path remain future UST-2.1.5.1 work.

