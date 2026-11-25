.PHONY: up up-monitoring up-cluster up-media demo-obs-stream down run-gateway run-archiver test bench-fanout
up:
	docker compose up -d
up-monitoring:
	docker compose --profile monitoring up -d
up-cluster:
	docker compose --profile cluster up -d --build
up-media:
	docker compose --profile media --profile cluster up -d --build

RTMP_URL ?= rtmp://127.0.0.1:1935/live
SECONDS ?= 30
demo-obs-stream: export DEMO_SECONDS := $(SECONDS)
demo-obs-stream: export RTMP_URL := $(RTMP_URL)
demo-obs-stream:
	@set -eu; : "$${ROOM:?set ROOM}" "$${STREAM_KEY:?set STREAM_KEY}"; \
	key=$${STREAM_KEY#"$$ROOM?key="}; \
	ffmpeg -hide_banner -loglevel warning -nostdin -re -f lavfi -i 'testsrc=size=640x360:rate=25:decimals=2' \
		-f lavfi -i 'sine=frequency=440:sample_rate=44100' -t "$$DEMO_SECONDS" \
		-c:v libx264 -preset veryfast -pix_fmt yuv420p -g 50 -c:a aac -f flv \
		"$${RTMP_URL%/}/$$ROOM?key=$$key"

down:
	docker compose --profile monitoring --profile cluster --profile media down
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

.PHONY: bench-capacity bench-soak
bench-capacity bench-soak:
	go run ./cmd/bench $(patsubst bench-%,%,$@) $(ARGS)
