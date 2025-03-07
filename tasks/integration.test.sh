#!/usr/bin/env bash

set -eu
set -o pipefail

APP_PATH=/app/bin/pirate
CONFIG_PATH=/app/tasks/integration.ship.yml
PORT=3939

BASE_URL="http://localhost:$PORT"

log() {
  local str
  str="$1"

  printf "[pirate | test] %s\n" "$str"
}

file_should_exist() {
  local fname
  fname="$1"

  if ! test -f "$fname"; then
    log "error: expected file ($fname) to exist"
    exit 1
  fi
}

trim_space() {
  local input
  input="$1"

  echo -n "$input" | tr -s ' '
}

file_should_eq() {
  local fname
  local contents

  fname="$1"
  contents="$(trim_space "$2")"
  trimmed="$(trim_space "$(cat "$fname")")"

  if test "$contents" != "$trimmed"; then
    log "error: file ($fname) contents differ"

    log "got : $contents"
    log "want: $trimmed"

    exit 1
  fi
}

## run the app in the background
"$APP_PATH" --config "$CONFIG_PATH" &

if ! pgrep 'pirate'; then
  log "failed to run pirate, exiting"
  cat ~/app.log
  exit 1
fi

log "app running with PID: $(pgrep 'pirate')"

log "hitting first endpoint"

curl \
  -X POST \
  -H X-Authorization:alpha \
  -H X-My-Header:foobar \
  -H "Content-Type: application/json" \
  -H "User-Agent: integration-test" \
  -d '{"data": [1, 2, 3] }' \
  "$BASE_URL/webhooks/first"

log "waiting 5s..."
sleep 5

log "step: ensuring headers.json exists..."
file_should_exist ~/headers.json
log "ok"

log "step: ensuring body.json exists..."
file_should_exist ~/body.json
log "ok"

log "step: checking headers.json matches what we expect..."
file_should_eq ~/headers.json '{"Accept":"*/*","Content-Length":"20","Content-Type":"application/json","User-Agent":"integration-test","X-Authorization":"alpha","X-My-Header":"foobar"}'
log "ok"

log "step: checking body.json matches what we expect..."
file_should_eq ~/body.json "'{\"data\": [1, 2, 3] }'"
log "ok"
