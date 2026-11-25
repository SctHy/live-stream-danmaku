.PHONY: up up-monitoring up-cluster down run-gateway run-archiver test bench-fanout
up:
	docker compose up -d
up-monitoring:
	docker compose --profile monitoring up -d
up-cluster:
	docker compose --profile cluster up -d --build
down:
	docker compose --profile monitoring --profile cluster down
run-gateway:
	go run ./cmd/gateway $(ARGS)
run-archiver:
	go run ./cmd/archiver $(ARGS)
test:
	go test -race ./...
bench-fanout:
	go run ./cmd/bench fanout $(ARGS)

.PHONY: eval-p0 eval-p1 eval-all eval-quick eval-report eval-diff
LEDGER ?= evaluation/baseline.json
BASELINE ?= evaluation/baseline.json
CANDIDATE ?= evaluation/candidate.json
eval-p0 eval-p1:
	go run ./cmd/bench eval $(patsubst eval-%,%,$@) $(ARGS)
eval-all:
	go run ./cmd/bench eval all -profile full -out $(LEDGER) $(ARGS)
eval-quick:
	go run ./cmd/bench eval all -profile quick -out $(CANDIDATE) $(ARGS)
eval-report:
	go run ./cmd/bench eval report -in $(LEDGER) -md evaluation/report.md -html evaluation/report.html $(ARGS)
eval-diff:
	go run ./cmd/bench eval diff -baseline $(BASELINE) -candidate $(CANDIDATE) $(ARGS)
