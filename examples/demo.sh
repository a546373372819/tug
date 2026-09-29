#!/usr/bin/env bash
# Demo: live vertical scaling of a container with tug.
# Requires: a running Docker daemon, Go 1.22+.
set -euo pipefail

cd "$(dirname "$0")/.."

NAME="tug-demo"

cleanup() {
  docker rm -f "$NAME" >/dev/null 2>&1 || true
}
trap cleanup EXIT

echo "==> building tug"
go build -o tug ./cmd/tug

echo "==> starting container '$NAME' with initial limits (cpu=1, memory=256m)"
docker rm -f "$NAME" >/dev/null 2>&1 || true
docker run -d --name "$NAME" --cpus 1 --memory 256m nginx:alpine >/dev/null

show() {
  local nano mem started
  nano=$(docker inspect -f '{{.HostConfig.NanoCpus}}' "$NAME")
  mem=$(docker inspect -f '{{.HostConfig.Memory}}' "$NAME")
  started=$(docker inspect -f '{{.State.StartedAt}}' "$NAME")
  printf '    docker sees: NanoCpus=%s Memory=%s  (StartedAt=%s)\n' "$nano" "$mem" "$started"
}

echo "==> BEFORE"
./tug inspect --container "$NAME"
show

echo "==> tug set --cpu 2 --memory 512MiB"
./tug set --container "$NAME" --cpu 2 --memory 512MiB

echo "==> tug add --cpu 0.5"
./tug add --container "$NAME" --cpu 0.5

echo "==> AFTER"
./tug inspect --container "$NAME"
show

echo
echo "==> Note: StartedAt is unchanged across the scaling — the container was"
echo "    resized live, without a restart."
