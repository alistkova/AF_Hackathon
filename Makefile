SHELL := bash
.SHELLFLAGS := -eu -o pipefail -c

ifneq (,$(wildcard .env))
include .env
export
endif

COMPOSE ?= docker compose
GATEWAY_PORT ?= 8080
WEB_PORT ?= 3000
GO ?= go
PNPM ?= pnpm
UV ?= uv

.PHONY: help bootstrap check-tools infra infra-down migrate seed dev demo demo-reset up down logs psql test lint fmt gen smoke nix-check clean

.PHONY: rag-install rag-build rag-update rag-query rag-watch rag-stats rag-clean

help:
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | sed 's/:.*## / — /'

bootstrap: check-tools ## Install dependencies and create .env
	@test -f .env || cp .env.example .env
	$(GO) mod download
	$(PNPM) --dir web install
	cd services/agent && $(UV) sync
	@echo "Run: make infra && make migrate && make seed && make dev"

check-tools: ## Verify local toolchain
	@for tool in docker go golangci-lint pnpm uv ruff; do \
		command -v $$tool >/dev/null || { echo "missing: $$tool (use 'nix develop')"; exit 1; }; \
	done

infra: ## Start postgres and redis
	$(COMPOSE) -f compose.dev.yaml up -d --wait

infra-minio: ## Start optional MinIO object storage
	$(COMPOSE) -f compose.dev.yaml --profile minio up -d --wait

infra-down: ## Stop local infrastructure
	$(COMPOSE) -f compose.dev.yaml --profile minio down

migrate: ## Run all service migrations
	bash scripts/migrate.sh

seed: ## Load demo companies through the company service command
	bash scripts/seed.sh

dev: ## Run services on the host (infrastructure stays in Docker)
	bash scripts/dev.sh

demo: ## Build and run the full laptop demo
	bash scripts/demo.sh

demo-reset: ## Recreate the full demo from an empty database
	$(COMPOSE) down -v
	bash scripts/demo.sh

up: ## Start the full stack in Docker
	$(COMPOSE) up -d --build --wait

down: ## Stop the full stack
	$(COMPOSE) down

logs: ## Follow full stack logs
	$(COMPOSE) logs -f --tail=200

psql: ## Open psql on the company database
	$(COMPOSE) exec postgres psql -U company_svc company_db

test: ## Run Go race tests, Vitest and Pytest
	$(GO) test -race ./...
	$(PNPM) --dir web test
	cd services/agent && $(UV) run pytest

lint: ## Run gofmt, golangci-lint, ESLint, TypeScript and Ruff
	@test -z "$$(gofmt -l services internal)" || { gofmt -l services internal; exit 1; }
	$(GO) vet ./...
	golangci-lint run ./...
	$(PNPM) --dir web lint
	$(PNPM) --dir web typecheck
	cd services/agent && ruff format --check && ruff check

fmt: ## Format Go, Nix and Python
	gofmt -w services internal
	nix fmt flake.nix
	cd services/agent && ruff format

gen: ## Generate contract clients and types
	bash scripts/gen.sh

smoke: ## Run the end-to-end smoke test
	BASE_URL=$${BASE_URL:-http://localhost:$(GATEWAY_PORT)} bash scripts/smoke.sh

nix-check: ## Validate the flake
	nix flake check

clean: ## Remove local build artifacts
	rm -rf web/dist web/node_modules/.vite
	rm -rf services/agent/.venv services/agent/.pytest_cache services/agent/.ruff_cache
	find services/agent -type d -name __pycache__ -prune -exec rm -rf {} +

# RAG system targets
rag-install: ## Install RAG system Python dependencies
	cd services/agent/rag && pip install -r requirements.txt

rag-build: ## Build RAG index from PDFs in services/agent/rag/data/
	cd services/agent/rag && python -m rag_system.cli build --data-dir data --index-dir C:/temp/alfa_rag_index

rag-update: ## Incremental update for new PDFs in RAG system
	cd services/agent/rag && python -m rag_system.cli update --data-dir data --index-dir C:/temp/alfa_rag_index

rag-query: ## Query the RAG system (usage: make rag-query Q="your question")
	cd services/agent/rag && python -m rag_system.cli query --index-dir C:/temp/alfa_rag_index --question "$(Q)"

rag-watch: ## Watch services/agent/rag/data/ for new PDFs and auto-update
	cd services/agent/rag && python -m rag_system.cli watch --data-dir data --index-dir C:/temp/alfa_rag_index

rag-stats: ## Show RAG index statistics
	cd services/agent/rag && python -m rag_system.cli stats --index-dir C:/temp/alfa_rag_index

rag-clean: ## Remove RAG index and processed files cache
	rmdir /s /q C:\temp\alfa_rag_index 2>nul || true
	del services/agent/rag/processed_files.json 2>nul || true
