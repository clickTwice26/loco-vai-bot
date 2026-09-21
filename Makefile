.PHONY: all build run test clean tidy lint docker-build docker-run docker-stop redis-up redis-down redis-logs

APP_NAME = bot
BUILD_DIR = bin
CMD_DIR = ./cmd/bot

all: test build

build:
	@echo "Building binary..."
	@mkdir -p $(BUILD_DIR)
	go build -ldflags="-s -w" -o $(BUILD_DIR)/$(APP_NAME) $(CMD_DIR)
	@echo "Build complete: $(BUILD_DIR)/$(APP_NAME)"

run:
	@echo "Running bot..."
	go run $(CMD_DIR)

test:
	@echo "Running tests..."
	go test -v -race -cover ./...

clean:
	@echo "Cleaning artifacts..."
	@rm -rf $(BUILD_DIR) coverage.out

tidy:
	@echo "Tidying go modules..."
	go mod tidy

docker-build:
	@echo "Building Docker image..."
	docker build -t localoy-bot:latest .

docker-run:
	@echo "Running Docker containers..."
	docker compose up --build -d

docker-stop:
	@echo "Stopping Docker containers..."
	docker compose down

redis-up:
	@echo "Starting Redis container..."
	docker compose up -d redis

redis-down:
	@echo "Stopping Redis container..."
	docker compose stop redis

redis-logs:
	docker compose logs -f redis
