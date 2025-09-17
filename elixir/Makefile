# Makefile for Elixir Metrics Agent

.PHONY: help deps compile test run clean dev prod panic-test

help: ## Show this help message
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

deps: ## Install dependencies
	mix deps.get

compile: deps ## Compile the project
	mix compile

test: ## Run tests
	mix test

run: compile ## Run the application
	mix run --no-halt

dev: ## Run in development mode
	MIX_ENV=dev mix run --no-halt

prod: ## Run in production mode
	MIX_ENV=prod mix run --no-halt

shell: ## Start interactive shell
	iex -S mix

clean: ## Clean build artifacts
	mix clean
	rm -rf _build/
	rm -rf deps/

panic-test: ## Run panic recovery test script
	./scripts/test-panic.sh

format: ## Format code
	mix format

lint: ## Run linter
	mix credo

docs: ## Generate documentation
	mix docs

release: ## Create a release
	mix release

install: deps compile ## Install and compile everything
	@echo "Installation complete!"
