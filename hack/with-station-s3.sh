#!/usr/bin/env bash
# Usage: bash hack/with-station-s3.sh <command> [args...]
# Run inside the Linux builder. Only release artifacts persist between runs.
set -euo pipefail

fail() {
  printf 'with-station-s3: %s\n' "$*" >&2
  exit 1
}

(( $# > 0 )) || fail 'usage: bash hack/with-station-s3.sh <command> [args...]'

# SILO is the maintained MinIO fork with If-Match on single-object DELETE.
# CopyObject preconditions apply to the source, not the destination.
server_tag=RELEASE.2026-11-12T00-00-00Z
client_tag=RELEASE.2025-08-13T08-35-41Z
case $(uname -s)-$(uname -m) in
  Linux-x86_64)
    arch=amd64
    server_sha=378adce81515d31d48d8ac9b79effc62980e964c470fb06cc3ee8984786c2f4d
    client_sha=01f866e9c5f9b87c2b09116fa5d7c06695b106242d829a8bb32990c00312e891
    ;;
  Linux-aarch64)
    arch=arm64
    server_sha=ae4b1aec3481751fb50980b8074773bcc64e2ce388eb150fa10336ad808a62f3
    client_sha=14c8c9616cfce4636add161304353244e8de383b2e2752c0e9dad01d4c27c12c
    ;;
  *) fail 'requires a Linux amd64 or arm64 builder' ;;
esac
server_asset="silo_20261112000000.0.0_linux_${arch}.tar.gz"
client_asset="mc.linux-${arch}.${client_tag}"
cache="${XDG_CACHE_HOME:-$HOME/.cache}/streamplace/station-s3"
cache_stage=
server_pid=
child_pid=
launching=0
signal_status=0
scratch=$(mktemp -d "${TMPDIR:-/tmp}/streamplace-station-s3.XXXXXXXX")

cleanup() {
  local status=$? pid running deadline
  trap - EXIT
  trap '' HUP INT TERM
  for pid in "$child_pid" "$server_pid"; do
    if [[ -n $pid ]]; then kill -TERM -- "-$pid" 2>/dev/null || true; fi
  done
  deadline=$((SECONDS + 3))
  while (( SECONDS < deadline )); do
    running=0
    for pid in "$child_pid" "$server_pid"; do
      if [[ -n $pid ]] && kill -0 -- "-$pid" 2>/dev/null; then running=1; fi
    done
    (( running )) || break
    sleep 0.1
  done
  for pid in "$child_pid" "$server_pid"; do
    if [[ -n $pid ]]; then
      kill -KILL -- "-$pid" 2>/dev/null || true
      wait "$pid" 2>/dev/null || true
    fi
  done
  if [[ -n $cache_stage ]]; then rm -rf -- "$cache_stage"; fi
  rm -rf -- "$scratch"
  exit "$status"
}

interrupted() {
  signal_status=$1
  if (( ! launching )); then exit "$signal_status"; fi
}

trap cleanup EXIT
trap 'interrupted 129' HUP
trap 'interrupted 130' INT
trap 'interrupted 143' TERM
# Give each background job its own process group so teardown reaches descendants.
set -m
start_child() {
  launching=1
  "$@" <&0 &
  child_pid=$!
  launching=0
  if (( signal_status )); then exit "$signal_status"; fi
}

verify_sha() {
  local digest
  digest=$(sha256sum < "$2") && [[ ${digest%% *} == "$1" ]]
}

fetch() {
  local url=$1 sha=$2 mode=$3 path="$cache/${1##*/}"
  if [[ -f $path ]] && verify_sha "$sha" "$path"; then return; fi
  # Stage on the same filesystem; concurrent runs only publish verified bytes.
  if [[ -z $cache_stage ]]; then cache_stage=$(mktemp -d "$cache/.download.XXXXXXXX"); fi
  start_child curl -qfsSL --proto '=https' --proto-redir '=https' \
    --connect-timeout 10 --max-time 120 -o "$cache_stage/artifact" "$url"
  wait "$child_pid" || fail "download failed: $url"
  child_pid=
  verify_sha "$sha" "$cache_stage/artifact" || fail "SHA-256 mismatch: $url"
  chmod "$mode" "$cache_stage/artifact"
  mv -fT -- "$cache_stage/artifact" "$path"
}

install -d -m 700 -- "$cache"
fetch "https://github.com/pgsty/silo/releases/download/$server_tag/$server_asset" "$server_sha" 600
fetch "https://github.com/minio/mc/releases/download/$client_tag/$client_asset" "$client_sha" 700
tar -xOzf "$cache/$server_asset" silo > "$scratch/silo"
chmod 700 "$scratch/silo"
mkdir -- "$scratch/data" "$scratch/home"

read -r access_key < /proc/sys/kernel/random/uuid
read -r secret_key < /proc/sys/kernel/random/uuid
access_key=${access_key//-/}
access_key=${access_key:0:20}
secret_key=${secret_key//-/}
bucket=station-test
region=us-east-1

# Empty environment and a private HOME keep server/client configuration local.
start_child env -i PATH="$PATH" HOME="$scratch/home" NO_COLOR=1 \
  MINIO_BROWSER=off MINIO_ROOT_USER="$access_key" MINIO_ROOT_PASSWORD="$secret_key" \
  MINIO_SITE_REGION="$region" \
  "$scratch/silo" server --address 127.0.0.1:0 --anonymous "$scratch/data" \
  > "$scratch/server.log" 2>&1
server_pid=$child_pid
child_pid=

server_fail() {
  tail -n 40 -- "$scratch/server.log" >&2
  fail "$@"
}

# Read the port selected by this server, never a guessed/preselected port.
endpoint=
ready=0
deadline=$((SECONDS + 30))
while (( SECONDS < deadline )); do
  kill -0 "$server_pid" 2>/dev/null || server_fail 'MinIO exited before becoming ready'
  if [[ -z $endpoint ]]; then
    while IFS= read -r line; do
      if [[ $line =~ ^API:[[:blank:]]+(http://127\.0\.0\.1:[1-9][0-9]{0,4})([[:blank:]]|$) ]]; then
        endpoint=${BASH_REMATCH[1]}
        break
      fi
    done < "$scratch/server.log"
  fi
  if [[ -n $endpoint ]] && curl -qfs --noproxy '*' --max-time 1 \
    "$endpoint/minio/health/ready" > /dev/null; then
    ready=1
    break
  fi
  sleep 0.1
done
(( ready )) || server_fail 'MinIO readiness timed out'

start_child timeout --foreground --kill-after=2s 10s env -i PATH="$PATH" HOME="$scratch/home" \
  MC_HOST_station="http://$access_key:$secret_key@${endpoint#http://}" \
  "$cache/$client_asset" --config-dir "$scratch/mc" mb --region "$region" "station/$bucket" \
  > "$scratch/mc.log" 2>&1
if ! wait "$child_pid"; then
  tail -n 20 -- "$scratch/mc.log" >&2
  server_fail 'could not create the local S3 bucket'
fi
child_pid=
kill -0 "$server_pid" 2>/dev/null || server_fail 'MinIO exited during bucket setup'

unset -v "${!AWS_@}" "${!S3_@}" "${!SP_S3_@}" "${!SP_TEST_S3_@}"
export AWS_CONFIG_FILE=/dev/null AWS_SHARED_CREDENTIALS_FILE=/dev/null AWS_EC2_METADATA_DISABLED=true
export AWS_REGION="$region" AWS_DEFAULT_REGION="$region"
export SP_TEST_S3_ENDPOINT="$endpoint" SP_TEST_S3_BUCKET="$bucket"
export SP_TEST_S3_ACCESS_KEY_ID="$access_key" SP_TEST_S3_SECRET_ACCESS_KEY="$secret_key"
start_child "$@"
wait "$child_pid"
