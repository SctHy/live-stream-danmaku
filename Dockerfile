FROM golang:1.27.4-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/maku-gateway ./cmd/gateway && \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/maku-archiver ./cmd/archiver

FROM alpine:3.21 AS gateway
RUN apk add --no-cache ca-certificates && adduser -D -H -u 10001 maku
COPY --from=builder /out/maku-gateway /usr/local/bin/maku-gateway
USER maku
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/maku-gateway"]

FROM alpine:3.21 AS archiver
RUN apk add --no-cache ca-certificates && adduser -D -H -u 10001 maku
COPY --from=builder /out/maku-archiver /usr/local/bin/maku-archiver
USER maku
ENTRYPOINT ["/usr/local/bin/maku-archiver"]
