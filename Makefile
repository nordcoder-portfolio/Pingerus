GO       ?= go
LINTER   ?= golangci-lint
COMPOSE  ?= $(shell if docker compose version >/dev/null 2>&1; then echo "docker compose"; else echo "docker-compose"; fi)

BIN_DIR  := bin
SERVICES := api-gateway scheduler ping-worker email-notifier

PROTO_DIR       := proto
GOOGLEAPIS_DIR  := third_party/googleapis
PGV_DIR         := third_party/protoc-gen-validate
GO_OUT_DIR      := generated
OPENAPI_OUT_DIR := docs/spec

HEALTH_API   := http://localhost:8080/healthz
HEALTH_SCHED := http://localhost:8082/healthz
HEALTH_PINGW := http://localhost:8083/healthz
HEALTH_EMAIL := http://localhost:8084/healthz

# utils
define wait_url
	@echo "⏳ Waiting for $(1) ..."; \
	tries=60; \
	while ! curl -sf $(1) >/dev/null 2>&1; do \
		tries=$$((tries-1)); \
		if [ $$tries -le 0 ]; then \
			echo "Timeout waiting for $(1)"; \
			exit 1; \
		fi; \
		sleep 2; \
	done; \
	echo "$(1) OK"
endef

# bootstrap
.PHONY: bootstrap
bootstrap:
	@echo "Installing proto deps & plugins..."
	@bash scripts/bootstrap_proto.sh
	@go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	@go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	@go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@latest
	@go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2@latest
	@go install github.com/envoyproxy/protoc-gen-validate@latest

.PHONY: generate
generate: bootstrap
	@mkdir -p $(GO_OUT_DIR) $(OPENAPI_OUT_DIR)
	@protoc \
	  -I=$(PROTO_DIR) -I=$(GOOGLEAPIS_DIR) -I=$(PGV_DIR) \
	  --experimental_allow_proto3_optional \
	  --go_out=$(GO_OUT_DIR) --go_opt=paths=source_relative \
	  --go-grpc_out=$(GO_OUT_DIR) --go-grpc_opt=paths=source_relative \
	  --grpc-gateway_out=$(GO_OUT_DIR) --grpc-gateway_opt=paths=source_relative,generate_unbound_methods=true \
	  --openapiv2_out=$(OPENAPI_OUT_DIR) --openapiv2_opt=generate_unbound_methods=true \
	  --validate_out=lang=go,paths=source_relative:$(GO_OUT_DIR) \
	  $(shell find $(PROTO_DIR) -name '*.proto')
	@echo "✅ Protobuf generated"

.PHONY: lint
lint:
	@$(LINTER) run ./...

.PHONY: test
test:
	@$(GO) test ./... -race -v

# build
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

# composes
.PHONY: up down clean
up:
	@echo "▶ starting local environment"
	$(COMPOSE) up -d --build
	@$(COMPOSE) ps

down:
	@echo "▶ stopping environment"
	$(COMPOSE) down -v

clean:
	@rm -rf $(GO_OUT_DIR)/* $(OPENAPI_OUT_DIR)/* $(BIN_DIR)/*
	@echo "🧹 cleaned generated artifacts"

# e2e
.PHONY: wait-health
wait-health:
	$(call wait_url,$(HEALTH_API))
	$(call wait_url,$(HEALTH_SCHED))
	$(call wait_url,$(HEALTH_PINGW))
	$(call wait_url,$(HEALTH_EMAIL))

.PHONY: e2e
e2e: up wait-health
	@echo "▶ running E2E tests"
	E2E_API_BASE=http://localhost:8080 \
	E2E_MAILHOG_BASE=http://localhost:8025 \
	go test -tags=e2e ./test/e2e -v -timeout=120s

# it
.PHONY: it-up it-down it-restart
it-up:
	$(COMPOSE) -f docker-compose.it.yml up -d --build
	@echo "waiting for core services..."
	@for i in $$(seq 1 60); do \
  	if curl -sf --max-time 2 http://127.0.0.1:8084/healthz >/dev/null 2>&1 && \
       curl -sf --max-time 2 http://127.0.0.1:8083/healthz >/dev/null 2>&1; then \
       echo "services are up"; exit 0; fi; \
    echo "retry $$i..."; sleep 2; \
   done; \
   echo "services failed to become healthy"; \
   docker compose -f docker-compose.it.yml ps; \
   exit 1

it-down:
	$(COMPOSE) -f docker-compose.it.yml down -v

it-restart: it-down it-up

define run_it_test
	set -euo pipefail; \
	make it-up; \
	trap 'docker compose -f docker-compose.it.yml logs > it_logs.txt || true; docker compose -f docker-compose.it.yml down -v || true' EXIT; \
	$(1)
endef

.PHONY: it-test-en
it-test-en:
	$(call run_it_test, IT_BOOTSTRAP=127.0.0.1:19092 \
	  IT_DB_DSN=postgres://postgres:secret@127.0.0.1:55432/pingerus?sslmode=disable \
	  IT_MAILHOG_API=http://127.0.0.1:18025 \
	  go test -v ./test/integration -tags=integration -run ^TestEmailNotifier_)

.PHONY: it-test-pw
it-test-pw:
	$(call run_it_test, IT_BOOTSTRAP=127.0.0.1:19092 \
	  IT_DB_DSN=postgres://postgres:secret@127.0.0.1:55432/pingerus?sslmode=disable \
	  go test -v ./test/integration -tags=integration -run ^TestPingWorker_)

.PHONY: it-test-ag
it-test-ag:
	$(call run_it_test, IT_BOOTSTRAP=127.0.0.1:19092 \
	  IT_DB_DSN=postgres://postgres:secret@127.0.0.1:55432/pingerus?sslmode=disable \
	  go test -v ./test/integration -tags=integration -run ^TestAPIGateway_)
