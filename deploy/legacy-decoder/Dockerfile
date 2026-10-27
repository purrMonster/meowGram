# syntax=docker/dockerfile:1
FROM golang:1.27.1@sha256:162be5298a40ed317005c8339c6de4d10d3eef336d66dc8e9259b03ab9d3a6d2
WORKDIR /src
COPY --from=legacy server/ ./
COPY deploy/legacy-decoder/main.go deploy/legacy-decoder/main_test.go ./cmd/legacy-decoder/
ARG LEGACY_COMMIT
RUN test "$LEGACY_COMMIT" = be15cec6ceed2cb4a21b69cfe5e4548d2bfd818c && \
    CGO_ENABLED=0 go build -trimpath -ldflags "-X main.sourceCommit=$LEGACY_COMMIT" -o /legacy-decoder ./cmd/legacy-decoder && \
    go test -race -c -o /decoder.test ./cmd/legacy-decoder && \
    go vet ./cmd/legacy-decoder
USER 65532:65532
ENTRYPOINT ["/decoder.test", "-test.v", "-test.count=1", "-test.timeout=30s"]

