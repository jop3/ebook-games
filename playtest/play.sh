#!/usr/bin/env bash
# play.sh — run a game's headless gameplay tests under the pure-Go inkview
# emulator, on a normal PC (no device, no Docker, no cgo).
#
# It builds a temporary Go workspace (go.work) that swaps each game's cgo
# third_party/inkview for playtest/inkemu, then runs `go test`. Nothing under
# version control is modified: the workspace file is created in a temp dir and
# removed on exit.
#
# Usage:
#   playtest/play.sh <game> [test-regexp]   # one game, e.g. bullscows
#   playtest/play.sh all                    # every game that has a *play_test.go
#   playtest/play.sh <game> -v              # pass extra flags after the game
#
# Screenshots requested by a test (via PLAYTEST_SHOTS) land in playtest/_shots/.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
EMU="$ROOT/playtest/inkemu"
SHOTS="${PLAYTEST_SHOTS:-$ROOT/playtest/_shots}"

run_one() {
  local game="$1"; shift
  # An optional non-flag first argument overrides the -run regexp; anything
  # else (e.g. -v) is passed straight through to `go test`.
  local run_re="TestPlay"
  if [ $# -gt 0 ] && [ "${1#-}" = "$1" ]; then run_re="$1"; shift; fi

  if [ ! -d "$ROOT/$game" ] || [ ! -f "$ROOT/$game/go.mod" ]; then
    echo "no such game module: $game" >&2; return 2
  fi

  local work; work="$(mktemp -d)/go.work"
  cat > "$work" <<EOF
go 1.25.0

use $ROOT/$game
use $EMU
EOF

  echo ">>> playing $game (go test -run $run_re)"
  mkdir -p "$SHOTS"
  # -tags playtest gates the *play_test.go files so they compile ONLY here,
  # never during a normal go build/vet or the device Docker build.
  local rc=0
  GOWORK="$work" PLAYTEST_SHOTS="$SHOTS" \
    go test -tags playtest "$ROOT/$game/..." -run "$run_re" "$@" || rc=$?
  rm -rf "$(dirname "$work")"
  return $rc
}

main() {
  local target="${1:-all}"; shift || true
  if [ "$target" = "all" ]; then
    local failed=0 found=0
    for d in "$ROOT"/*/; do
      local g; g="$(basename "$d")"
      [ "$g" = "playtest" ] && continue
      compgen -G "$d*play_test.go" > /dev/null || continue
      found=1
      run_one "$g" TestPlay "$@" || failed=1
    done
    [ "$found" = 0 ] && { echo "no *play_test.go files found"; exit 0; }
    exit $failed
  fi
  run_one "$target" "$@"
}

main "$@"
