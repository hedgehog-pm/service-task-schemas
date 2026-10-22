.PHONY: fmt lint generate generate-api generate-sql

lint:
	go tool golangci-lint config verify
	go tool golangci-lint run

fmt:
	go tool golangci-lint fmt

generate: generate-api generate-sql

generate-api:
	go tool oapi-codegen -config api/oapi-codegen.yaml api/openapi.yaml

generate-sql:
	go tool sqlc generate

.PHONY: fix
fix:
	go tool golangci-lint config verify
	go tool golangci-lint run --fix ./...
