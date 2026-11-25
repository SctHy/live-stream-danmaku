#!/bin/sh
set -eu
if [ "$1" = start ]; then
    : "${STREAM_WEBHOOK_SECRET:?set STREAM_WEBHOOK_SECRET for the media profile}"
    mkdir -p /replays/videos /capture/live /capture/sessions
    chown 10001:10001 /replays/videos /capture/live /capture/sessions
    # This instance exclusively owns the recording tree. A restart closes all publishers.
    find /capture/live -maxdepth 1 -type l -delete
    for dir in /capture/sessions/*; do
        [ -f "$dir/room" ] || continue
        set -- "$dir"/*.mp4
        if [ ! -f "$1" ] && [ ! -f "$dir/ready" ] && [ ! -f "$dir/ready.tmp" ]; then
            rm -r "$dir"
            continue
        fi
        touch "$dir/disconnected"
        MTX_SOURCE_ID=${dir##*/} MTX_PATH="live/$(cat "$dir/room")" /hook offline &
    done
    exec mediamtx /mediamtx.yml
fi
post() {
    curl --silent --show-error --fail --retry 5 --retry-all-errors --max-time 5 \
        -H "X-Webhook-Secret: $STREAM_WEBHOOK_SECRET" -H 'Content-Type: application/json' \
        --data "$2" "http://gateway-a:8080/internal/stream/$1"
}
if [ "$1" = disconnect ]; then
    # A connection can pass authentication and disconnect before sending any media.
    dir=/capture/sessions/$MTX_CONN_ID
    [ -f "$dir/room" ] || exit 0
    room=$(cat "$dir/room")
    if [ "$(readlink "/capture/live/$room")" = "../sessions/$MTX_CONN_ID" ]; then
        rm "/capture/live/$room"
    fi
    touch "$dir/disconnected"
    exit 0
fi
room=${MTX_PATH#live/}
source=$MTX_SOURCE_ID
dir=/capture/sessions/$source
payload=$(jq -nc --arg room "$room" --arg source "$source" '{room_slug:$room,source_id:$source}')
case "$1" in
online)
    post ready "$payload" > "$dir/ready.tmp"
    mv "$dir/ready.tmp" "$dir/ready"
    ;;
offline)
    # A very short stream can interrupt the online hook before it saves the reply.
    if [ ! -s "$dir/ready" ]; then post ready "$payload" > "$dir/ready"; fi
    id=$(jq -er '.replay_id' "$dir/ready")
    # RTMP disconnect runs after MediaMTX has removed the publisher and
    # closed its native recorder. New publishers already have their own tree.
    for i in $(seq 1 100); do
        [ -f "$dir/disconnected" ] && break
        sleep 0.1
    done
    [ -f "$dir/disconnected" ] || exit 1
    video=
    for segment in "$dir"/*.mp4; do
        if [ -f "$segment" ]; then printf "file '%s'\n" "$segment"; fi
    done > "$dir/concat"
    if [ -s "$dir/concat" ] && ffmpeg -nostdin -loglevel error -y -f concat -safe 0 -i "$dir/concat" -c copy -movflags +faststart "/replays/videos/$id.tmp.mp4"; then
        video=videos/$id.mp4
        mv "/replays/videos/$id.tmp.mp4" "/replays/$video"
    fi
    duration=null
    if [ -n "$video" ]; then
        duration=$(ffprobe -v error -show_entries format=duration -of csv=p=0 "/replays/$video" | jq -e 'tonumber * 1000 | round')
    fi
    post ended "$(jq -nc --arg room "$room" --argjson id "$id" --arg file "$video" --argjson duration "$duration" '{room_slug:$room,replay_id:$id,video_file:$file,duration_ms:$duration}')"
    if [ -n "$video" ]; then rm -r "$dir"; fi
    ;;
esac
