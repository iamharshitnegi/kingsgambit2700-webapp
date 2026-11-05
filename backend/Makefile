.PHONY: generate fmt test test-race river-migrate river-validate run-api run-worker run-realtime

generate:
	go generate ./...

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './vendor/*')

test:
	go test ./...

test-race:
	go test -race ./...

river-migrate:
	go run ./cmd/river-migrate up

river-validate:
	go run ./cmd/river-migrate validate

run-api:
	go run ./cmd/api

run-worker:
	go run ./cmd/worker

run-realtime:
	go run ./cmd/realtime
