# syntax=docker/dockerfile:1

# --- Build stage -------------------------------------------------------
# Builds forgejo-mcp from source for the target platform. Using a
# multi-stage build (instead of copying in a prebuilt binary) means the
# image can be built and reproduced entirely by CI without any
# out-of-band build step, and BuildKit's automatic TARGETOS/TARGETARCH
# handle cross-compilation for multi-arch images.
FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS build

ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN sed -i "s/dev-test/${VERSION}/" types/version.go && \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /out/forgejo-mcp .

# --- Runtime stage -------------------------------------------------------
# Minimal, distroless-style runtime: static binary + CA certs + tzdata,
# running as a non-root user.
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -S forgejo-mcp && adduser -S forgejo-mcp -G forgejo-mcp

COPY --from=build /out/forgejo-mcp /forgejo-mcp

USER forgejo-mcp
ENTRYPOINT ["/forgejo-mcp"]
CMD ["stdio"]
