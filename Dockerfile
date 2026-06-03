# syntax=docker/dockerfile:1.7

FROM node:22-alpine AS frontend-builder
WORKDIR /src/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

FROM golang:1.26-bookworm AS backend-builder
WORKDIR /src/backend
RUN apt-get update \
  && apt-get install -y --no-install-recommends gcc libc6-dev \
  && rm -rf /var/lib/apt/lists/*
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=1 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/ludex ./cmd/server

FROM debian:bookworm-slim AS runtime
RUN apt-get update \
  && apt-get install -y --no-install-recommends ca-certificates tzdata \
  && rm -rf /var/lib/apt/lists/* \
  && useradd --system --uid 10001 --gid nogroup --home-dir /app --shell /usr/sbin/nologin ludex
WORKDIR /app
COPY --from=backend-builder /out/ludex /app/ludex
COPY --from=frontend-builder /src/frontend/dist /app/public
RUN mkdir -p /data \
  && chown -R ludex:nogroup /app /data
USER ludex
ENV LUDEX_ADDR=0.0.0.0:8787 \
  LUDEX_DATA_DIR=/data \
  LUDEX_PUBLIC_DIR=/app/public
EXPOSE 8787
VOLUME ["/data"]
ENTRYPOINT ["/app/ludex"]
