# syntax=docker/dockerfile:1

# =============================================================================
# Stage 1: Build React SPA Frontend
# =============================================================================
FROM --platform=$BUILDPLATFORM node:22-alpine AS frontend-builder
WORKDIR /workspace/frontend

COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci

COPY frontend/ ./
# Vite outputs directly to ../backend/internal/webui/dist
RUN npm run build

# =============================================================================
# Stage 2: Build Go Hexagonal Gateway Binary (with embedded SPA)
# =============================================================================
FROM --platform=$BUILDPLATFORM golang:1.23-alpine AS backend-builder
ARG TARGETOS=linux
ARG TARGETARCH=amd64
ARG VERSION=dev

WORKDIR /workspace/backend

COPY backend/go.mod ./
RUN go mod download

COPY backend/ ./
COPY --from=frontend-builder /workspace/backend/internal/webui/dist ./internal/webui/dist

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /out/dvr-gateway ./cmd/dvr-gateway

# =============================================================================
# Stage 3: Minimal Runtime Image with FFmpeg
# =============================================================================
FROM alpine:3.21 AS runtime

RUN apk add --no-cache \
      ca-certificates \
      ffmpeg \
      tzdata \
      wget \
    && adduser -D -H -u 10001 dvr

COPY --from=backend-builder /out/dvr-gateway /usr/local/bin/dvr-gateway

USER dvr
EXPOSE 8080

ENV LISTEN_ADDR=":8080" \
    DVR_HOST="192.168.1.64" \
    DVR_HTTP_PORT="80" \
    DVR_RTSP_PORT="554"

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8080/api/health || exit 1

ENTRYPOINT ["/usr/local/bin/dvr-gateway"]
