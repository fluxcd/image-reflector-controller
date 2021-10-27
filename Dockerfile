ARG XX_VERSION=1.0.0-rc.2

FROM --platform=$BUILDPLATFORM tonistiigi/xx:${XX_VERSION} AS xx

# Build the manager binary
FROM --platform=$BUILDPLATFORM golang:1.16-alpine AS builder

# Copy the build utilities.
COPY --from=xx / /

ARG TARGETPLATFORM

# Configure workspace.
WORKDIR /workspace

# copy modules manifests
COPY go.mod go.mod
COPY go.sum go.sum

# Copy this, which should not change often; and, needs to be in place
# before `go mod download`.
COPY api/ api/

# cache modules
RUN go mod download

# copy source code
COPY main.go main.go
COPY controllers/ controllers/
COPY internal/ internal/

# build without giving the arch, so that it gets it from the machine
ENV CGO_ENABLED=0
RUN xx-go build -a -o image-reflector-controller main.go

FROM alpine:3.13

LABEL org.opencontainers.image.source="https://github.com/fluxcd/image-reflector-controller"

# Create minimal nsswitch.conf file to prioritize the usage of /etc/hosts over DNS queries.
# https://github.com/gliderlabs/docker-alpine/issues/367#issuecomment-354316460
RUN [ ! -e /etc/nsswitch.conf ] && echo 'hosts: files dns' > /etc/nsswitch.conf

RUN apk add --no-cache ca-certificates tini

COPY --from=builder /workspace/image-reflector-controller /usr/local/bin/

RUN addgroup -S controller && adduser -S controller -G controller

USER controller

ENTRYPOINT [ "/sbin/tini", "--", "image-reflector-controller" ]
