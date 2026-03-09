.PHONY: help test lint coverage build docker clean frontend-build frontend-clean frontend-install frontend clone-example build-example start-example start-example-stand stop-example-stand download-jacococli screenshot-example

# Variables
BINARY_NAME=aggregator
DOCKER_IMAGE=jacoco-aggregator
COVERAGE_THRESHOLD=80
VERSION=0.1.0
JACOCO_VERSION=0.8.14
JACOCO_DIST_URL=https://github.com/jacoco/jacoco/releases/download/v$(JACOCO_VERSION)/jacoco-$(JACOCO_VERSION).zip
TOOLS_DIR=tools
JACOCO_ZIP=$(TOOLS_DIR)/jacoco-$(JACOCO_VERSION).zip
JACOCO_CLI_JAR=$(TOOLS_DIR)/jacococli.jar

help: ## Show this help
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

test: ## Run unit and integration tests
	go test ./... -v

test-race: ## Run tests with race detector
	go test ./... -race

lint: ## Run golangci-lint
	golangci-lint run ./...

coverage: ## Generate coverage report and check threshold
	@echo "Running tests with coverage..."
	@go test ./... -coverprofile=coverage.out
	@go tool cover -func=coverage.out
	@echo "Checking coverage threshold..."
	@go tool cover -func=coverage.out | grep total | awk '{print substr($$3, 1, length($$3)-1)}' | awk -v threshold=$(COVERAGE_THRESHOLD) '{if ($$1 < threshold) {print "Coverage is below " threshold "%"; exit 1}}'

build: frontend-build ## Build the binary
	go build -ldflags "-s -w -X main.version=$(VERSION)" -o bin/$(BINARY_NAME) ./cmd/aggregator

docker: ## Build Docker image from Dockerfile.base
	docker build -t $(DOCKER_IMAGE) -f Dockerfile.base .

clean: ## Clean up generated files
	rm -rf bin/
	rm -f coverage.out

frontend-install: ## Install frontend dependencies
	cd frontend && npm ci

frontend-build: frontend-install ## Build frontend static files
	cd frontend && npm run build
	rm -rf internal/publisher/frontend/assets
	cp -r frontend/build internal/publisher/frontend/assets

frontend-clean: ## Remove frontend build artifacts
	rm -rf frontend/build

frontend: frontend-build ## Alias for frontend-build

clone-example: ## Clone example repository (demo-service-flyway-pg) into example/
	@if [ -d "example" ]; then \
		echo "Directory 'example' already exists, skipping clone."; \
	else \
		echo "Cloning https://github.com/abatalev/demo-service-flyway-pg.git into example/..."; \
		git clone https://github.com/abatalev/demo-service-flyway-pg.git example; \
		cd example && git checkout prepare_jacoco; \
		echo "Building example services..."; \
		cd things && ./mk.sh; \
	fi

build-example: docker ## Build example services and create Docker image with example data
	@echo "Building example services..."
	cd example/things && ./mk.sh
	@echo "Building Docker image with example data..."
	docker build -f Dockerfile.example -t jacoco-aggregator-example .

download-jacococli: ## Download JaCoCo CLI jar from distribution
	@mkdir -p $(TOOLS_DIR)
	@if [ ! -f "$(JACOCO_CLI_JAR)" ]; then \
		echo "Downloading JaCoCo distribution version $(JACOCO_VERSION) from $(JACOCO_DIST_URL)"; \
		curl -L -o "$(JACOCO_ZIP)" "$(JACOCO_DIST_URL)"; \
		unzip -j "$(JACOCO_ZIP)" "lib/jacococli.jar" -d "$(TOOLS_DIR)"; \
		rm "$(JACOCO_ZIP)"; \
		echo "jacococli.jar extracted to $(TOOLS_DIR)"; \
	else \
		echo "jacococli.jar already exists in $(TOOLS_DIR)"; \
	fi

start-example: build download-jacococli ## Start the aggregator with example/things config
	@echo "Starting aggregator with config.example.local.yaml..."
	./bin/aggregator --config config.example.local.yaml --storage ./data

start-example-stand: clone-example build-example ## Start example services with docker compose
	@echo "Starting docker compose..."
	cd example/things && docker compose up -d

stop-example-stand: ## Stop example services
	@echo "Stopping docker compose..."
	cd example/things && docker compose down

screenshot-example: ## Take screenshot of example app and save to docs/screenshot.png
	-pkill -f aggregator 2>/dev/null || true
	sleep 1 || true
	echo "Starting aggregator..."
	./bin/aggregator --config config.example.local.yaml --storage ./data > /tmp/aggregator.log 2>&1 &
	sleep 10 || true
	echo "Taking screenshot..."
	chromium --headless --disable-gpu --screenshot --window-size=1400,900 http://localhost:8080 2>/dev/null || true
	cp screenshot.png docs/screenshot.png 2>/dev/null || echo "Failed to copy screenshot"
	rm -f screenshot.png 2>/dev/null || true
	-pkill -f aggregator 2>/dev/null || true
	echo "Screenshot saved to docs/screenshot.png"