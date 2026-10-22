FROM golang:1.27.1 AS build
WORKDIR /src
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ ./
RUN CGO_ENABLED=0 go test -c -o /bot-auth.test ./internal/botauth

FROM lldap/lldap:v0.6.3@sha256:2a8454b668c1aba7157e832eab0e242e1e7eb5fb7591d7e7774ba05286511ca8
COPY --from=build /bot-auth.test /bot-auth.test
USER 1000:1000
ENV MEOWGRAM_DISPOSABLE_LLDAP=1
ENTRYPOINT ["/bot-auth.test", "-test.v", "-test.run=^TestLLDAP$", "-test.timeout=120s"]
