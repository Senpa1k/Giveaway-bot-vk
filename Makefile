# Команды разработки бота по образцу основного backend-проекта.
GO ?= go
ARGS ?=
BINARY ?= bin/bot
MIGRATIONS_DIR ?= migrations
GOLANGCI_LINT ?= golangci-lint
GOOSE_BIN ?= $(shell command -v goose 2>/dev/null || echo "go run github.com/pressly/goose/v3/cmd/goose@v3.24.0")

# Настройки локального запуска и миграций загружаются из .env.
-include .env
export

GOOSE = $(GOOSE_BIN) -dir $(MIGRATIONS_DIR)

.DEFAULT_GOAL := help
.PHONY: help install build run format lint tidy-check test migrate migrate-down migrate-status migrate-create

help: ## Показать список команд
	@grep -hE '^[a-zA-Z_-]+:.*## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*## "}; {printf "  %-16s %s\n", $$1, $$2}'

install: ## Скачать зависимости приложения
	$(GO) mod download

build: ## Собрать исполняемый файл
	$(GO) build -buildvcs=false -trimpath -o $(BINARY) ./cmd/bot

run: ## Запустить бота с настройками из .env и параметрами ARGS
	$(GO) run ./cmd/bot $(ARGS)

format: ## Применить gofumpt и goimports
	$(GOLANGCI_LINT) fmt --config .golangci.yml

lint: ## Проверить модули, форматирование и код
	$(GO) mod verify
	$(GOLANGCI_LINT) config verify --config .golangci.yml
	$(GOLANGCI_LINT) fmt --config .golangci.yml --diff
	$(GOLANGCI_LINT) run --config .golangci.yml

tidy-check: ## Проверить go.mod и go.sum (нужен доступ к зависимостям)
	$(GO) mod tidy -diff

test: ## Запустить тесты с race detector и создать отчёты покрытия
	$(GO) test -race -coverprofile=coverage.out ./...
	$(GO) tool cover -func=coverage.out
	$(GO) tool cover -html=coverage.out -o coverage.html

migrate: ## Применить миграции PostgreSQL к DATABASE_URL
	$(GOOSE) postgres "$(DATABASE_URL)" up

migrate-down: ## Откатить последнюю миграцию
	$(GOOSE) postgres "$(DATABASE_URL)" down

migrate-status: ## Показать состояние миграций
	$(GOOSE) postgres "$(DATABASE_URL)" status

migrate-create: ## Создать SQL-миграцию: make migrate-create NAME=create_tasks
	@test -n "$(NAME)" || { echo "Укажите имя: make migrate-create NAME=create_tasks"; exit 1; }
	$(GOOSE) create $(NAME) sql
