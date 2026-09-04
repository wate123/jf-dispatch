# syntax=docker/dockerfile:1
FROM --platform=$BUILDPLATFORM golang:1.24-bookworm AS build
ARG TARGETOS TARGETARCH
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /out/jf-scheduler ./cmd/jf-scheduler && \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /out/jf-worker ./cmd/jf-worker && \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /out/jf-ffmpeg-wrapper ./cmd/jf-ffmpeg-wrapper && \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /out/jf-dispatch ./cmd/jf-dispatch

FROM debian:bookworm-slim
ARG TARGETARCH
RUN apt-get update && \
    apt-get install -y --no-install-recommends ffmpeg ca-certificates vainfo mesa-va-drivers && \
    if [ "$TARGETARCH" = "amd64" ]; then \
      apt-get install -y --no-install-recommends intel-media-va-driver; \
    fi && \
    rm -rf /var/lib/apt/lists/*
COPY --from=build /out/* /usr/local/bin/
ENTRYPOINT ["/usr/local/bin/jf-dispatch"]
CMD ["worker"]
