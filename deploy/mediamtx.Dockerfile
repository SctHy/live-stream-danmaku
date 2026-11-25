FROM bluenviron/mediamtx:1.21.4 AS upstream
FROM alpine:3.23
RUN apk add --no-cache curl jq ffmpeg
COPY --from=upstream /mediamtx /usr/local/bin/mediamtx
COPY mediamtx-hook.sh /hook
COPY mediamtx.yml /mediamtx.yml
ENTRYPOINT ["/hook","start"]
