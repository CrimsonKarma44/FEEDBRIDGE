# Multi-stage build for the FEEDBRIDGE API (linux/arm64 or host default).
#
# IMPORTANT: both modules use local `replace` directives that point at the
# sibling rss_detector checkout, which lives OUTSIDE this repository.
# Build from the repo root with rss_detector copied next to it:
#
#   cp -r ../rss_detector /tmp/build-ctx/rss_detector
#   cp -r . /tmp/build-ctx/API
#   docker build -f deploy/../API/Dockerfile ... (see README Deployment > Docker)
#
# Simpler alternative until rss_detector is vendored/published:
#   cd repo-root && go mod vendor in BOTH modules, then COPY vendor dirs.

FROM golang:1.26-alpine AS build
WORKDIR /src
COPY . .
WORKDIR /src/API
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags "-s -w" -o /out/feedbridge main.go

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/feedbridge /feedbridge
EXPOSE 50051
ENTRYPOINT ["/feedbridge"]
