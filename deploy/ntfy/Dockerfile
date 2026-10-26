FROM golang:1.27.1 AS build
WORKDIR /src
COPY deploy/ntfy/probe.go .
RUN CGO_ENABLED=0 go build -o /probe probe.go
FROM binwiederhier/ntfy:v2.34.0@sha256:901e14503d3f605f445be0e68f2ac71f0aa73391897ce6ceb599f8c48d8b3792
COPY --from=build /probe /probe
ENTRYPOINT ["/probe"]
