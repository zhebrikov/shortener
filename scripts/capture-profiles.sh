#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
mkdir -p profiles

backup() {
  for f in "$@"; do
    cp "$f" "${f}.profilebak"
  done
}

restore() {
  for f in "$@"; do
    mv "${f}.profilebak" "$f"
  done
}

FILES=(
  internal/storage/memory.go
  internal/service/shortener.go
  internal/handler/create_link.go
  internal/handler/create_link_json.go
)

backup "${FILES[@]}"
trap 'restore "${FILES[@]}"' EXIT

cp tools/profilebaseline/storage/memory.go.baseline internal/storage/memory.go
cp tools/profilebaseline/service/shortener.go.baseline internal/service/shortener.go
cp tools/profilebaseline/handler/create_link.go.baseline internal/handler/create_link.go
cp tools/profilebaseline/handler/create_link_json.go.baseline internal/handler/create_link_json.go

PROFILE_LINKS=3000
PROFILE_OPS=20000

go run ./cmd/memprofile -output profiles/base.pprof -links "$PROFILE_LINKS" -ops "$PROFILE_OPS"

restore "${FILES[@]}"
trap - EXIT

go run ./cmd/memprofile -output profiles/result.pprof -links "$PROFILE_LINKS" -ops "$PROFILE_OPS"

echo "Profiles: profiles/base.pprof profiles/result.pprof"
