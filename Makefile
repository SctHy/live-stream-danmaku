.PHONY: up up-monitoring down run-gateway run-archiver test bench-fanout
up:
	docker compose up -d
up-monitoring:
	docker compose --profile monitoring up -d
down:
	docker compose --profile monitoring down
run-gateway:
	go run ./cmd/gateway $(ARGS)
run-archiver:
	go run ./cmd/archiver $(ARGS)
test:
	go test -race ./...
bench-fanout:
	go run ./cmd/bench fanout $(ARGS)
