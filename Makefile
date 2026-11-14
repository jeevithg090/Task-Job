.PHONY: run build test lint fmt vuln

run:
	go run .

build:
	CGO_ENABLED=1 go build -trimpath -ldflags="-s -w" -o bin/backend-auth .

test:
	go test -race -count=1 ./...

lint:
	golangci-lint run ./...

fmt:
	golangci-lint fmt ./...

vuln:
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...
