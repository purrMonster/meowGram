# syntax=docker/dockerfile:1
FROM golang:1.27.1 AS previous-build
WORKDIR /src
COPY --from=previous server/ ./
RUN CGO_ENABLED=0 go build -trimpath -o /previous-server ./cmd/server

FROM golang:1.27.1 AS current-build
WORKDIR /src
COPY --from=r2 server/ ./
RUN CGO_ENABLED=0 go build -trimpath -o /current-server ./cmd/server

FROM golang:1.27.1 AS test-build
WORKDIR /src
COPY server/ ./
RUN CGO_ENABLED=0 go test -c -o /rollback.test ./integration

FROM postgres:16-alpine
COPY --from=previous-build /previous-server /previous-server
COPY --from=previous-build /src/migrations /previous-migrations
COPY --from=current-build /current-server /current-server
COPY --from=test-build /rollback.test /rollback.test
# R2 boundary only: both binaries and their complete migrations come from tags.
# The current checkout supplies assertions, never a substitute v2.0.0 binary.
# The v2.0.0 <-> R2.1 boundary belongs to UST-2.1.5.2.
COPY --from=current-build /src/migrations /current-migrations
USER postgres
ENV MEOWGRAM_DISPOSABLE_ROLLBACK=1
ENTRYPOINT ["/rollback.test", "-test.v", "-test.run=^TestReleaseRollback$", "-test.timeout=120s"]
