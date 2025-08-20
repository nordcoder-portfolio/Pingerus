# ====== TOOLS ======
GO             ?= go
GOLANGCI_LINT  ?= golangci-lint
DOCKER         ?= docker
# Пытаемся использовать docker compose v2; если нет — используем docker-compose
COMPOSE        ?= $(shell if docker compose version >/dev/null 2>&1; then echo "docker compose"; else echo "docker-compose"; fi)

BIN_DIR        := bin
SERVICES       := api-gateway scheduler ping-worker email-notifier
PROTO_DIR      := proto
GOOGLEAPIS_DIR := third_party/googleapis
PGV_DIR        := third_party/protoc-gen-validate
GO_OUT_DIR     := generated
OPENAPI_OUT_DIR:= docs/spec

# health endpoints для быстрого ожидания
HEALTH_API     := http://localhost:8080/healthz
HEALTH_SCHED   := http://localhost:8082/healthz
HEALTH_PINGW   := http://localhost:8083/healthz
HEALTH_EMAIL   := http://localhost:8084/healthz

# ====== HELPERS ======
define wait_url
	@echo "⏳ Waiting for $(1) ..."; \
	tries=60; \
	while ! curl -sf $(1) >/dev/null 2>&1; do \
		tries=$$((tries-1)); \
		if [ $$tries -le 0 ]; then \
			echo "❌ Timeout waiting for $(1)"; \
			exit 1; \
		fi; \
		sleep 2; \
	done; \
	echo "✅ $(1) OK"
endef

# ====== CHECKS ======
.PHONY: check-go
check-go:
	@command -v $(GO) >/dev/null 2>&1 || { echo "❌ Go is not installed (need Go 1.21+)."; exit 1; }
	@$(GO) version

.PHONY: check-docker
check-docker:
	@command -v $(DOCKER) >/dev/null 2>&1 || { echo "❌ docker not found"; exit 1; }
	@$(DOCKER) version >/dev/null || { echo "❌ docker daemon not running"; exit 1; }

.PHONY: check-compose
check-compose: check-docker
	@$(COMPOSE) version >/dev/null 2>&1 || { echo "❌ docker compose not available"; exit 1; }

.PHONY: check-protoc
check-protoc:
	@command -v protoc >/dev/null 2>&1 || { echo "❌ protoc not found"; exit 1; }
	@protoc --version

.PHONY: check-proto-plugins
check-proto-plugins:
	@for bin in protoc-gen-go protoc-gen-go-grpc protoc-gen-grpc-gateway protoc-gen-openapiv2 protoc-gen-validate; do \
		if ! command -v $$bin >/dev/null 2>&1; then \
			echo "❌ $$bin not found"; exit 1; \
		fi; \
	done
	@echo "✅ all protoc plugins found"

.PHONY: check-third-party
check-third-party:
	@[ -d "$(GOOGLEAPIS_DIR)" ] || { echo "❌ $(GOOGLEAPIS_DIR) is missing (run: make bootstrap-proto)"; exit 1; }
	@[ -d "$(PGV_DIR)" ]        || { echo "❌ $(PGV_DIR) is missing (run: make bootstrap-proto)"; exit 1; }
	@echo "✅ third-party protos present"

# ====== BOOTSTRAP ======
.PHONY: bootstrap-proto
bootstrap-proto:
	@echo "▶ Bootstrapping protoc & plugins & protos"
	@bash scripts/bootstrap_proto.sh

.PHONY: bootstrap-all
bootstrap-all: check-go bootstrap-proto check-protoc check-proto-plugins check-third-party
	@echo "✅ bootstrap-all done"

# ====== CODEGEN ======
.PHONY: generate
generate: bootstrap-all
	@echo "▶ Generating Protobuf"
	@mkdir -p $(GO_OUT_DIR) $(OPENAPI_OUT_DIR)
	@protoc \
	  -I=$(PROTO_DIR) \
	  -I=$(GOOGLEAPIS_DIR) \
	  -I=$(PGV_DIR) \
	  --experimental_allow_proto3_optional \
	  --go_out=$(GO_OUT_DIR)               --go_opt=paths=source_relative \
	  --go-grpc_out=$(GO_OUT_DIR)          --go-grpc_opt=paths=source_relative \
	  --grpc-gateway_out=$(GO_OUT_DIR)     --grpc-gateway_opt=paths=source_relative,generate_unbound_methods=true \
	  --openapiv2_out=$(OPENAPI_OUT_DIR)   --openapiv2_opt=generate_unbound_methods=true \
	  --validate_out=lang=go,paths=source_relative:$(GO_OUT_DIR) \
	  $(shell find $(PROTO_DIR) -name '*.proto')
	@echo "✅ Protobuf generated"

# ====== LINT/TEST/BUILD ======
.PHONY: lint
lint:
	@command -v $(GOLANGCI_LINT) >/dev/null 2>&1 || { echo "❌ golangci-lint not found. Install: https://golangci-lint.run/usage/install/"; exit 1; }
	@echo "▶ running golangci-lint"
	@$(GOLANGCI_LINT) run ./...

.PHONY: test
test:
	@echo "▶ running unit tests"
	@$(GO) test ./... -race -v

.PHONY: build
build:
ifdef SERVICE
	@echo "▶ building $(SERVICE)"
	@mkdir -p $(BIN_DIR)
	@$(GO) build -o $(BIN_DIR)/$(SERVICE) ./cmd/$(SERVICE)
else
	@for svc in $(SERVICES); do \
	  echo "▶ building $$svc"; \
	  mkdir -p $(BIN_DIR); \
	  $(GO) build -o $(BIN_DIR)/$$svc ./cmd/$$svc; \
	done
endif

# ====== DOCKER COMPOSE ======
.PHONY: compose-up
compose-up: check-compose
	@echo "▶ starting local environment via docker compose"
	@$(COMPOSE) up -d --build
	@$(COMPOSE) ps

.PHONY: compose-down
compose-down: check-compose
	@echo "▶ stopping environment"
	@$(COMPOSE) down -v

.PHONY: wait-health
wait-health:
	$(call wait_url,$(HEALTH_API))
	$(call wait_url,$(HEALTH_SCHED))
	$(call wait_url,$(HEALTH_PINGW))
	$(call wait_url,$(HEALTH_EMAIL))

# ====== E2E ======
.PHONY: e2e-up e2e-down e2e-test e2e
e2e-up: compose-up wait-health
	@echo "✅ stack is healthy"

e2e-down: compose-down

e2e-test:
	@echo "▶ running E2E tests"
	E2E_API_BASE=http://localhost:8080 \
	E2E_MAILHOG_BASE=http://localhost:8025 \
	go test -tags=e2e ./test/e2e -v -timeout=120s

e2e: e2e-up e2e-test

.PHONY: up
up: bootstrap-all generate build compose-up wait-health
	@echo "🚀 Ready at:"
	@echo "  - API:      $(HEALTH_API)"
	@echo "  - SCHED:    $(HEALTH_SCHED)"
	@echo "  - PING-W:   $(HEALTH_PINGW)"
	@echo "  - EMAIL:    $(HEALTH_EMAIL)"
	@echo "  - Mailhog:  http://localhost:8025"
	@echo "  - Metrics:  /metrics on each service"

.PHONY: down
down: compose-down

.PHONY: clean
clean:
	@rm -rf $(GO_OUT_DIR)/* $(OPENAPI_OUT_DIR)/*
	@echo "🧹 cleaned generated artifacts"
