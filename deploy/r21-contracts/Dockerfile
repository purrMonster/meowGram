FROM golang:1.27.1 AS tests
WORKDIR /src
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ ./
RUN CGO_ENABLED=0 go test -tags=r21_contracts -c -o /r21-fixture.test ./integration && CGO_ENABLED=0 go test -tags=r21_contracts -c -o /r21-contracts.test ./contracts/r21
FROM tests AS build
RUN CGO_ENABLED=0 go build -o /meowgram-server ./cmd/server
FROM lldap/lldap:v0.6.3@sha256:2a8454b668c1aba7157e832eab0e242e1e7eb5fb7591d7e7774ba05286511ca8
COPY --from=build /meowgram-server /r21-fixture.test /r21-contracts.test /
COPY server/migrations /migrations
COPY server/contracts/r21/http.json /contracts/http.json
USER 1000:1000
ENV MEOWGRAM_DISPOSABLE_R21_CONTRACT=1
ENTRYPOINT ["/r21-fixture.test", "-test.v", "-test.run=^TestR21HTTPContract$", "-test.timeout=300s"]

