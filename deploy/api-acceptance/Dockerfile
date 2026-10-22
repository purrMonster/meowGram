FROM golang:1.27.1 AS build
WORKDIR /src
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ ./
RUN CGO_ENABLED=0 go build -o /meowgram-server ./cmd/server && CGO_ENABLED=0 go test -c -o /api-acceptance.test ./integration
FROM lldap/lldap:v0.6.3@sha256:2a8454b668c1aba7157e832eab0e242e1e7eb5fb7591d7e7774ba05286511ca8
COPY --from=build /meowgram-server /api-acceptance.test /
COPY server/migrations /migrations
USER 1000:1000
ENV MEOWGRAM_DISPOSABLE_API=1
ENTRYPOINT ["/api-acceptance.test", "-test.v", "-test.run=^TestAPIEndToEnd$", "-test.timeout=180s"]
