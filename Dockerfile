FROM --platform=$BUILDPLATFORM node:24-alpine AS web
WORKDIR /web
ENV CI=true
ENV npm_config_verify_deps_before_run=false
COPY web/package.json web/pnpm-lock.yaml ./
RUN corepack enable && corepack prepare pnpm@11.8.0 --activate && pnpm install --frozen-lockfile --ignore-scripts
COPY web/ ./
COPY internal/openapi/openapi.yaml /internal/openapi/openapi.yaml
RUN pnpm run build

FROM --platform=$BUILDPLATFORM golang:1.26.4-alpine AS builder
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
ARG COMMIT=none
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /internal/webui/dist ./internal/webui/dist
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath \
    -ldflags="-s -w -X github.com/younsl/o/box/kubernetes/forklift/internal/version.Version=${VERSION} -X github.com/younsl/o/box/kubernetes/forklift/internal/version.Commit=${COMMIT}" \
    -o /out/forklift ./cmd/forklift

FROM alpine:3.23 AS certs
RUN apk add --no-cache ca-certificates

FROM scratch AS runtime

# Re-release 0.2.1 container image
LABEL org.opencontainers.image.title="forklift" \
      org.opencontainers.image.description="Lightweight Kubernetes-native artifact repository (Maven, npm, Cargo, Go) with proxy caching and supply-chain age policy" \
      org.opencontainers.image.version="0.2.1" \
      org.opencontainers.image.licenses="Apache-2.0" \
      org.opencontainers.image.base.name="scratch" \
      org.opencontainers.image.deprecated="false"

COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /out/forklift /app/forklift
EXPOSE 8080 8081
USER 65532:65532
ENTRYPOINT ["/app/forklift"]
