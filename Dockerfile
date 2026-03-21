# Build stage for frontend
FROM node:22-slim AS frontend-builder

WORKDIR /app/internal/frontend

# Install pnpm
RUN corepack enable && corepack prepare pnpm@10.28.0 --activate

# Copy frontend package files
COPY internal/frontend/package.json internal/frontend/pnpm-lock.yaml ./

# Install dependencies
RUN pnpm install --frozen-lockfile

# Copy frontend source
COPY internal/frontend/ ./

# Build frontend
RUN pnpm run build

# Build stage for Go binary
FROM golang:1.26-alpine AS builder

# Build arguments for version info
ARG VERSION=0.0.0
ARG REVISION=HEAD

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Copy built frontend from previous stage
COPY --from=frontend-builder /app/internal/static/dist ./internal/static/dist

# Build the binary with version info
RUN go build -ldflags="-s -w -X moted/version.Version=${VERSION} -X moted/version.Revision=${REVISION}" -trimpath -o moted .

# Final stage
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/moted /usr/local/bin/moted

# Use nonroot user (65532:65532 in distroless)
USER nonroot:nonroot

# Expose default port
EXPOSE 8080

# Run in foreground mode for container
ENTRYPOINT ["moted", "--foreground", "--bind", "0.0.0.0", "--server"]
CMD ["--port", "8080"]
