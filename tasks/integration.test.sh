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

assert() {
  local what
  local want
  local got

  what="$1"
  want="$2"
  got="$3"

  if test "$want" != "$got"; then
    log "assert '$what': want '$want', got '$got'"
    exit 1
  fi
}

file_should_eq() {
  local fname
  local contents

  fname="$1"
  contents="$(trim_space "$2")"
  trimmed="$(trim_space "$(cat "$fname")")"

  assert "file content" "$contents" "$trimmed"
}

## run the app in the background
"$APP_PATH" --config "$CONFIG_PATH" &

if ! pgrep 'pirate'; then
  log "failed to run pirate, exiting"
  exit 1
fi

log "app running with PID: $(pgrep 'pirate')"

##
## Test: hit endpoint and ensure it returns a 200.
##
log "============================================"
log "Test: it should execute a handler correctly"
log "============================================"

CODE=$(curl \
  -o /dev/null \
  -w "%{http_code}" \
  -s \
  -X POST \
  -H X-Authorization:alpha \
  -H X-My-Header:foobar \
  -H "Content-Type: application/json" \
  -H "User-Agent: integration-test" \
  -d '{"data": [1, 2, 3] }' \
  "$BASE_URL/webhooks/simple")

assert "status code" "200" "$CODE"

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

##
## Test: ensure invalid enpoints return a 404
##

log "=========================================="
log "test: ensure invalid enpoints return a 404"
log "=========================================="

CODE=$(curl \
  -o /dev/null \
  -w "%{http_code}" \
  -s \
  -X POST \
  -H X-Authorization:alpha \
  "$BASE_URL/missing")

log "got status code: $CODE"
assert "status code" "404" "$CODE"

##
## Test: ensure invalid auth returns a 404
##

log "======================================="
log "test: ensure invalid auth returns a 404"
log "======================================="

CODE=$(curl \
  -o /dev/null \
  -w "%{http_code}" \
  -s \
  -X POST \
  -H X-Authorization:invalid \
  "$BASE_URL/webhooks/simple")

log "got status code: $CODE"
assert "status code" "404" "$CODE"

##
## Test: ensure invalid method returns a 405
##

log "========================================="
log "test: ensure invalid method returns a 405"
log "========================================="

CODE=$(curl \
  -o /dev/null \
  -w "%{http_code}" \
  -s \
  -X GET \
  -H X-Authorization:invalid \
  "$BASE_URL/webhooks/simple")

log "got status code: $CODE"
assert "status code" "405" "$CODE"

##
## Test: ensure validation fails if command validator fails
##

log "========================================================"
log "test: ensure validation fails if command validator fails"
log "========================================================"

CODE=$(curl \
  -o /dev/unll \
  -w "%{http_code}" \
  -s \
  -X POST \
  -H X-Authorization:ok \
  "$BASE_URL/command/should-fail")

log "got status code: $CODE"
assert "status code" "404" "$CODE"

##
## Test: ensure validation succeeds if command validator succeeds
##

log "=============================================================="
log "test: ensure validation succeeds if command validator succeeds"
log "=============================================================="

CODE=$(curl \
  -o /dev/unll \
  -w "%{http_code}" \
  -s \
  -X POST \
  -H X-Authorization:ok \
  "$BASE_URL/command/should-succeed")

log "got status code: $CODE"
assert "status code" "200" "$CODE"

sleep 5 

log "step: ensuring ~/command-succeeded.txt exists..."
file_should_exist ~/command-succeeded.txt
log "ok"

log "step: checking ~/command-succeeded.txt matches what we expect..."
file_should_eq ~/command-succeeded.txt "command succeeded"
log "ok"
