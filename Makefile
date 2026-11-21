.PHONY: up down run-gateway run-archiver test bench-fanout
up:
	docker compose up -d
down:
	docker compose down
run-gateway:
	go run ./cmd/gateway $(ARGS)
run-archiver:
	go run ./cmd/archiver $(ARGS)
test:
	go test -race ./...
bench-fanout:
	go run ./cmd/bench fanout $(ARGS)
