.PHONY: cli ui test

cli:
	go build -o bin/mclauncher-cli ./cmd/mclauncher-cli

ui:
	go build -o bin/mclauncher ./cmd/mclauncher

test:
	go test ./internal/config ./internal/auth ./internal/java ./internal/launch ./internal/version ./internal/download
