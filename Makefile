# SentinelAegis — Makefile
# Multi-agent BEC fraud detection system

APP_NAME   := sentinelaegis
GO         := go
PORT       ?= 8080
PROJECT_ID ?= $(shell gcloud config get-value project 2>/dev/null)
REGION     ?= us-central1

.PHONY: all build run test test-race bench lint fmt vet clean docker deploy help

## ── Build & Run ────────────────────────────────────────

all: fmt vet test build ## Format, vet, test, and build

build: ## Compile the binary
	$(GO) build -ldflags="-s -w" -trimpath -o $(APP_NAME) .

run: ## Run locally (requires GEMINI_API_KEY env var)
	PORT=$(PORT) $(GO) run .

## ── Testing ────────────────────────────────────────────

test: ## Run all tests with verbose output
	$(GO) test ./... -v -count=1

test-race: ## Run tests with race detector
	$(GO) test ./... -v -race -count=1

bench: ## Run benchmarks
	$(GO) test ./agents/ -bench=. -benchmem -count=3

cover: ## Generate test coverage report
	$(GO) test ./... -coverprofile=coverage.out
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

## ── Code Quality ───────────────────────────────────────

fmt: ## Format all Go source files
	$(GO) fmt ./...

vet: ## Run Go vet static analysis
	$(GO) vet ./...

lint: ## Run golangci-lint (install: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run ./...

## ── Docker ─────────────────────────────────────────────

docker: ## Build Docker image locally
	docker build -t $(APP_NAME):latest .

docker-run: docker ## Build and run Docker container
	docker run -p $(PORT):$(PORT) \
		-e GEMINI_API_KEY=$(GEMINI_API_KEY) \
		-e MODEL_NAME=gemini-1.5-pro \
		$(APP_NAME):latest

## ── Deploy ─────────────────────────────────────────────

deploy: ## Deploy to Google Cloud Run
	gcloud run deploy $(APP_NAME) \
		--source . \
		--region $(REGION) \
		--allow-unauthenticated \
		--min-instances 1 \
		--max-instances 3 \
		--memory 256Mi \
		--set-env-vars GEMINI_API_KEY=$(GEMINI_API_KEY),MODEL_NAME=gemini-1.5-pro

deploy-url: ## Get the deployed service URL
	@gcloud run services describe $(APP_NAME) --region $(REGION) --format='value(status.url)'

## ── Utilities ──────────────────────────────────────────

clean: ## Remove build artifacts
	rm -f $(APP_NAME) $(APP_NAME).exe coverage.out coverage.html

deps: ## Download and verify dependencies
	$(GO) mod download
	$(GO) mod verify

tidy: ## Tidy go.mod and go.sum
	$(GO) mod tidy

## ── Help ───────────────────────────────────────────────

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'

.DEFAULT_GOAL := help
