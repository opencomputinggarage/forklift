# Build the React UI first; its output is embedded into the Go binary.
FROM --platform=$BUILDPLATFORM node:24-alpine AS web
WORKDIR /web
# Run pnpm non-interactively and don't let `pnpm run` reinstall before scripts:
# the deps were installed with --ignore-scripts (esbuild's build script is
# skipped; its binary ships via a platform optional dependency), and a pre-run
# reinstall would re-trip that gate and abort without a TTY.
ENV CI=true
ENV npm_config_verify_deps_before_run=false
COPY web/package.json web/pnpm-lock.yaml ./
# --ignore-scripts: pnpm 11 fails a fresh non-interactive install when a
# dependency (esbuild) has an unapproved build script. esbuild ships its binary
# via a platform-specific optional dependency, so skipping install scripts is
# safe and the later `pnpm run build` (vite) still works.
RUN corepack enable && corepack prepare pnpm@11.8.0 --activate && pnpm install --frozen-lockfile --ignore-scripts
COPY web/ ./
# `pnpm run build` runs generate:queries, which reads the OpenAPI spec from
# outside web/ (../internal/openapi/openapi.yaml -> /internal/...). Make it
# available in this stage.
COPY internal/openapi/openapi.yaml /internal/openapi/openapi.yaml
RUN pnpm run build

# Statically linked Go binary on scratch. Cross-compilation happens inside the
# builder stage (CGO disabled), so buildx needs no QEMU for the compile step.
# The React UI (internal/webui/dist) is embedded into the binary via go:embed.
FROM --platform=$BUILDPLATFORM golang:1.26.4-alpine AS builder
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
ARG COMMIT=none
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Overlay the compiled UI on top of the committed placeholder.
COPY --from=web /internal/webui/dist ./internal/webui/dist
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath \
    -ldflags="-s -w -X github.com/younsl/o/box/kubernetes/forklift/internal/version.Version=${VERSION} -X github.com/younsl/o/box/kubernetes/forklift/internal/version.Commit=${COMMIT}" \
    -o /out/forklift ./cmd/forklift

FROM alpine:3.23 AS certs
RUN apk add --no-cache ca-certificates

FROM scratch AS runtime

# Rebuilt 0.2.1 to embed the upstream-status badge fix (reachable transition).
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
