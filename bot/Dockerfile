# Multi-stage build for the FEEDBRIDGE bot (linux/arm64 or host default).
# See API/Dockerfile for the rss_detector replace-directive caveat:
# build from a context that also contains the rss_detector source.

FROM golang:1.26-alpine AS build
WORKDIR /src
COPY . .
WORKDIR /src/bot
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags "-s -w" -o /out/feedbridge-bot main.go

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/feedbridge-bot /feedbridge-bot
ENTRYPOINT ["/feedbridge-bot"]
