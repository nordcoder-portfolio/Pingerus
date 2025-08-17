GO            := go
GOLANGCI_LINT := golangci-lint
DOCKER        := docker
COMPOSE       := docker-compose
HELM          := helm
KUBECTL       := kubectl

DATABASE_URL  ?= postgres://postgres:example@localhost:5432/sitewatcher?sslmode=disable

SERVICES      := api-gateway scheduler ping-worker email-notifier

BIN_DIR       := bin

.PHONY: lint
lint:
	@echo ">> running golangci-lint"
	$(GOLANGCI_LINT) run ./...

.PHONY: test
test:
	@echo ">> running tests"
	$(GO) test ./... -v

.PHONY: build
build:
ifneq ($(SERVICE),)
	@echo ">> building $(SERVICE)"
	@mkdir -p $(BIN_DIR)
	$(GO) build -o $(BIN_DIR)/$(SERVICE) ./cmd/$(SERVICE)
else
	@for svc in $(SERVICES); do \
	  echo ">> building $$svc"; \
	  mkdir -p $(BIN_DIR); \
	  $(GO) build -o $(BIN_DIR)/$$svc ./cmd/$$svc; \
	done
endif


.PHONY: docker-build
docker-build:
ifneq ($(SERVICE),)
	@echo ">> building docker image for $(SERVICE)"
	$(DOCKER) build -t sitewatcher/$(SERVICE):latest cmd/$(SERVICE)
else
	@for svc in $(SERVICES); do \
	  echo ">> building docker image for $$svc"; \
	  $(DOCKER) build -t sitewatcher/$$svc:latest cmd/$$svc; \
	done
endif

.PHONY: compose-up
compose-up:
	@echo "starting local environment via docker-compose"
	$(COMPOSE) up -d --build

.PHONY: compose-down
compose-down:
	@echo "stopping local environment"
	$(COMPOSE) down


.PHONY: kind-deploy
kind-deploy:
	@echo "deploying to kind cluster via Helm"
	$(HELM) dependency update deployments/charts/sitewatcher
	$(HELM) upgrade --install sitewatcher deployments/charts/sitewatcher \
	  --set image.tag=latest

PROTO_DIR        := proto
GOOGLEAPIS_DIR   := third_party/googleapis
GO_OUT_DIR       := generated
OPENAPI_OUT_DIR  := docs/spec

.PHONY: bootstrap generate clean

bootstrap:
	@echo "Bootstrapping"
	@bash scripts/bootstrap_proto.sh

.PHONY: clean
generate: bootstrap
	@echo "Generating Protobuf"
	@protoc \
	  -I=$(PROTO_DIR) \
	  -I=$(GOOGLEAPIS_DIR) \
	  --experimental_allow_proto3_optional \
	  --go_out=$(GO_OUT_DIR)               --go_opt=paths=source_relative \
	  --go-grpc_out=$(GO_OUT_DIR)          --go-grpc_opt=paths=source_relative \
	  --grpc-gateway_out=$(GO_OUT_DIR)     --grpc-gateway_opt=paths=source_relative,generate_unbound_methods=true \
	  --openapiv2_out=$(OPENAPI_OUT_DIR)   --openapiv2_opt=generate_unbound_methods=true \
	  --validate_out=lang=go:$(GO_OUT_DIR) \
	  $(shell find $(PROTO_DIR) -name '*.proto')
	@echo "Protobuf code generated"


clean:
	@rm -rf $(GO_OUT_DIR)/* $(OPENAPI_OUT_DIR)/*
