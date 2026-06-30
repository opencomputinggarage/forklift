BINARY      := forklift
PKG         := ./cmd/$(BINARY)
VERSION     := $(shell grep -oE 'org\.opencontainers\.image\.version="[^"]+"' Dockerfile | cut -d'"' -f2)
COMMIT      := $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
LDFLAGS     := -s -w \
	-X github.com/younsl/o/box/kubernetes/forklift/internal/version.Version=$(VERSION) \
	-X github.com/younsl/o/box/kubernetes/forklift/internal/version.Commit=$(COMMIT)
ECR_REGISTRY ?= ghcr.io/younsl
IMAGE        := $(ECR_REGISTRY)/$(BINARY)
COVER_MIN   ?= 73
PLATFORMS   ?= linux/amd64,linux/arm64
DATA_DIR    ?= ./.data

.PHONY: all build run dev scan-dev web-dev test coverage fmt lint vet tidy clean web-build artifact-scan-dev artifact-scan-worker-dev artifact-scan-worker-loop docker-build docker-push helm-lint helm-template creds

all: fmt vet lint test build

## build: compile the binary into bin/
build:
	CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o bin/$(BINARY) $(PKG)

## run: build and run with local dev settings
run: build
	FORKLIFT_DATA_DIR=./.data FORKLIFT_LOG_FORMAT=text ./bin/$(BINARY)

## dev: run with debug logging
dev:
	FORKLIFT_DATA_DIR=./.data FORKLIFT_LOG_FORMAT=text FORKLIFT_LOG_LEVEL=debug go run $(PKG)

## scan-dev: run the normal dev server on 8080 with artifact scanning enabled
scan-dev:
	@echo "artifact scan dev server: http://127.0.0.1:$${FORKLIFT_SCAN_DEV_PORT:-8080}"
	@echo "data dir: $${FORKLIFT_SCAN_DEV_DATA_DIR:-./.data}"
	@echo "login: $${FORKLIFT_SCAN_DEV_ADMIN_USER:-admin} / $${FORKLIFT_SCAN_DEV_ADMIN_PASS:-adminpw}"
	@echo "worker token: $${FORKLIFT_SCAN_DEV_WORKER_TOKEN:-dev-scan-token}"
	@echo "worker run: make artifact-scan-worker-dev"
	FORKLIFT_DATA_DIR=$${FORKLIFT_SCAN_DEV_DATA_DIR:-./.data} \
	FORKLIFT_HTTP_ADDR=:$${FORKLIFT_SCAN_DEV_PORT:-8080} \
	FORKLIFT_METRICS_ADDR=:$${FORKLIFT_SCAN_DEV_METRICS_PORT:-8081} \
	FORKLIFT_LOG_FORMAT=text \
	FORKLIFT_LOG_LEVEL=debug \
	FORKLIFT_BOOTSTRAP_ADMIN_USER=$${FORKLIFT_SCAN_DEV_ADMIN_USER:-admin} \
	FORKLIFT_BOOTSTRAP_ADMIN_PASSWORD=$${FORKLIFT_SCAN_DEV_ADMIN_PASS:-adminpw} \
	go run $(PKG) \
		--osv-url= \
		--deps-dev-url= \
		--artifact-scan-enabled \
		--artifact-scan-worker-token=$${FORKLIFT_SCAN_DEV_WORKER_TOKEN:-dev-scan-token}

## web-dev: run the React UI dev server
web-dev:
	cd web && mise exec -- pnpm dev

## test: run tests with race detector
test:
	go test -race ./...

## coverage: enforce minimum line coverage
coverage:
	go test -coverprofile=cover.out ./...
	@total=$$(go tool cover -func=cover.out | awk '/^total:/ {gsub("%","",$$3); print $$3}'); \
	echo "total coverage: $$total% (min $(COVER_MIN)%)"; \
	awk "BEGIN { exit !($$total >= $(COVER_MIN)) }" || { echo "coverage below $(COVER_MIN)%"; exit 1; }

## fmt: format code
fmt:
	gofmt -w .

## lint: gofmt check + go vet
lint: vet
	@unformatted=$$(gofmt -l .); \
	if [ -n "$$unformatted" ]; then echo "gofmt needed:"; echo "$$unformatted"; exit 1; fi

## vet: run go vet
vet:
	go vet ./...

## tidy: tidy module dependencies
tidy:
	go mod tidy

## web-build: build the React UI into internal/webui/dist (embedded into the binary)
web-build:
	cd web && mise exec -- pnpm install --frozen-lockfile && mise exec -- pnpm run build

## artifact-scan-dev: run an isolated artifact scan demo server on port 18080
artifact-scan-dev:
	@echo "artifact scan dev server: http://127.0.0.1:$${FORKLIFT_SCAN_DEV_PORT:-18080}"
	@echo "login: $${FORKLIFT_SCAN_DEV_ADMIN_USER:-admin} / $${FORKLIFT_SCAN_DEV_ADMIN_PASS:-adminpw}"
	@echo "worker token: $${FORKLIFT_SCAN_DEV_WORKER_TOKEN:-dev-scan-token}"
	@echo "worker run, no image build: FORKLIFT_SCAN_DEV_PORT=$${FORKLIFT_SCAN_DEV_PORT:-18080} make artifact-scan-worker-dev"
	@echo "worker run, container: docker run --rm forklift-scanner:dev --server=http://host.docker.internal:$${FORKLIFT_SCAN_DEV_PORT:-18080} --worker-id=local-worker --worker-token=$${FORKLIFT_SCAN_DEV_WORKER_TOKEN:-dev-scan-token} --once"
	FORKLIFT_DATA_DIR=$${FORKLIFT_SCAN_DEV_DATA_DIR:-/tmp/forklift-scan-dev} \
	FORKLIFT_HTTP_ADDR=:$${FORKLIFT_SCAN_DEV_PORT:-18080} \
	FORKLIFT_METRICS_ADDR=:$${FORKLIFT_SCAN_DEV_METRICS_PORT:-18081} \
	FORKLIFT_LOG_FORMAT=text \
	FORKLIFT_LOG_LEVEL=debug \
	FORKLIFT_BOOTSTRAP_ADMIN_USER=$${FORKLIFT_SCAN_DEV_ADMIN_USER:-admin} \
	FORKLIFT_BOOTSTRAP_ADMIN_PASSWORD=$${FORKLIFT_SCAN_DEV_ADMIN_PASS:-adminpw} \
	go run $(PKG) \
		--osv-url= \
		--deps-dev-url= \
		--artifact-scan-enabled \
		--artifact-scan-worker-token=$${FORKLIFT_SCAN_DEV_WORKER_TOKEN:-dev-scan-token}

## artifact-scan-worker-dev: run one local scanner worker with go run (requires grype on PATH)
artifact-scan-worker-dev:
	@command -v grype >/dev/null || { echo "grype is required for no-image worker dev. Install it or use the scanner-runtime Docker image."; exit 1; }
	@grype db status 2>&1 | grep -Eq "Status:[[:space:]]+valid$$" || { echo "updating local grype vulnerability DB..."; grype db update; }
	@server="$${FORKLIFT_SCAN_SERVER:-http://127.0.0.1:$${FORKLIFT_SCAN_DEV_PORT:-8080}}"; \
	echo "scanner worker target: $$server"; \
	echo "worker mode: $${FORKLIFT_SCAN_WORKER_LOOP:-false} (set FORKLIFT_SCAN_WORKER_LOOP=true to keep polling)"
	@once_flag="--once"; \
	if [ "$${FORKLIFT_SCAN_WORKER_LOOP:-false}" = "true" ]; then once_flag=""; fi; \
	server="$${FORKLIFT_SCAN_SERVER:-http://127.0.0.1:$${FORKLIFT_SCAN_DEV_PORT:-8080}}"; \
	FORKLIFT_ARTIFACT_SCAN_WORKER_TOKEN=$${FORKLIFT_SCAN_DEV_WORKER_TOKEN:-dev-scan-token} \
	go run ./cmd/forklift-scanner \
		--server=$$server \
		--worker-id=$${FORKLIFT_SCAN_WORKER_ID:-local-go-worker} \
		--worker-token=$${FORKLIFT_SCAN_DEV_WORKER_TOKEN:-dev-scan-token} \
		--work-dir=$${FORKLIFT_SCAN_WORKER_DIR:-/tmp/forklift-scan-worker} \
		$$once_flag

## artifact-scan-worker-loop: run the local scanner worker continuously
artifact-scan-worker-loop:
	FORKLIFT_SCAN_WORKER_LOOP=true $(MAKE) artifact-scan-worker-dev

## creds: list local users and password hashes from the local DB (plaintext is bcrypt-hashed and unrecoverable; the generated admin password is only printed once in bootstrap logs)
creds:
	@db="$(DATA_DIR)/forklift.db"; \
	if [ ! -f "$$db" ]; then echo "no database at $$db (run 'make run' first)"; exit 1; fi; \
	command -v sqlite3 >/dev/null || { echo "sqlite3 not installed"; exit 1; }; \
	sqlite3 -header -column "$$db" \
		"SELECT id, username, source, disabled, password_hash FROM users ORDER BY id;"

## clean: remove build and coverage artifacts
clean:
	rm -rf bin cover.out .data

## docker-build: build multi-arch image (requires buildx)
docker-build:
	docker buildx build --platform $(PLATFORMS) \
		--build-arg VERSION=$(VERSION) --build-arg COMMIT=$(COMMIT) \
		-t $(IMAGE):$(VERSION) -t $(IMAGE):latest .

## docker-push: build and push multi-arch image
docker-push:
	docker buildx build --push --platform $(PLATFORMS) \
		--build-arg VERSION=$(VERSION) --build-arg COMMIT=$(COMMIT) \
		-t $(IMAGE):$(VERSION) -t $(IMAGE):latest .

## helm-lint: lint the Helm chart
helm-lint:
	helm lint charts/$(BINARY)

## helm-template: render Helm templates locally
helm-template:
	helm template $(BINARY) charts/$(BINARY)

## helm-package: package the chart as a tgz
helm-package:
	helm package charts/$(BINARY)

## helm-push: push the packaged chart to the OCI registry (GHCR)
helm-push: helm-package
	helm push $(BINARY)-$(shell grep '^version:' charts/$(BINARY)/Chart.yaml | awk '{print $$2}').tgz oci://$(ECR_REGISTRY)/charts
