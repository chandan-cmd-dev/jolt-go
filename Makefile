.PHONY: all test bench vet ci

all: test

test:
	go mod tidy
	go vet ./...
	go test ./... -bench=. -benchmem

bench:
	go test ./... -bench=. -benchmem

vet:
	go vet ./...

ci: vet test

run-api:
	go run ./cmd/restapi -keyfile /tmp/jolt.key
