#!/usr/bin/env bash
set -euo pipefail

# Scoped, Docker-gated canopy runner.
#
# Canopy is tree-sitter based and can chew through this repo if pointed at
# the whole tree. This wrapper:
#   - mounts the host's canopy binary into the harness container (no rebuild),
#   - pins memory/cpu/pids so a runaway parse can't kill WSL,
#   - wraps `docker run` in host-side `timeout` so hangs can't stall forever,
#   - applies a default exclude set (generated grammars, blobs, worktrees),
#   - scopes each invocation to one package at a time.
#
# Usage:
#   run_canopy_scoped.sh --path ./grammargen --mode summary
#   run_canopy_scoped.sh --path .            --mode review --base main
#   run_canopy_scoped.sh --path ./grammargen --mode all
#
# Modes:
#   summary     — canopy analyze summary
#   smells      — canopy analyze smells
#   complexity  — canopy analyze complexity
#   coupling    — canopy analyze coupling
#   hotspot     — canopy analyze hotspot
#   dead        — canopy graph dead
#   impact      — canopy graph impact (requires --symbols)
#   review      — canopy analyze review --base REF (requires --base)
#   index       — canopy index build (populates cache, no analysis)
#   all         — summary + smells + complexity + coupling (sequential, reusing index)

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
HOST_CANOPY="${HOST_CANOPY:-$HOME/go/bin/canopy}"
IMAGE_TAG="gotreesitter/cgo-harness:go1.24-local"

PATH_ARG="."
MODE="summary"
BASE_REF=""
SYMBOLS=""
MEMORY_LIMIT="4g"
CPUS_LIMIT="4"
PIDS_LIMIT="1024"
WALL_TIMEOUT="10m"       # host-side SIGTERM deadline
KILL_GRACE="30s"         # SIGKILL after SIGTERM
GOMEMLIMIT_VAL=""        # auto-derive from MEMORY_LIMIT if empty
INDEX_GC_EVERY_VAL="${CANOPY_INDEX_GC_EVERY:-}"
OUT_ROOT="$REPO_ROOT/harness_out/canopy"
EXTRA_EXCLUDES=()
VERBOSE=0
NO_DEFAULT_EXCLUDES=0

# Default excludes: things canopy should never chew on in this repo.
# Generated code, binary blobs, worktrees, CGO harness, test data.
DEFAULT_EXCLUDES=(
  "grammars/grammar_blobs/**"
  "grammars/*_register.go"
  "grammars/embedded_grammars_gen.go"
  "grammars/zzz_scanner_attachments.go"
  "cgo_harness/**"
  ".claude/**"
  "harness_out/**"
  "parity_out/**"
  ".golden/**"
  "testdata/**"
  "**/testdata/**"
  "benchgate/**"
  "bench_out/**"
  "grammar_seed/**"
)

# Root sweeps should stay lightweight. These packages are still indexable by
# passing --path for the package itself; they are just kept out of the repo-root
# sweep where cumulative parser memory is highest.
ROOT_SWEEP_EXCLUDES=(
  "grammargen/**"
)

usage() {
  cat <<'EOF'
Usage: run_canopy_scoped.sh [options]

Options:
  --path <dir>           Package path under repo (default: .)
  --mode <mode>          summary|smells|complexity|coupling|hotspot|dead|
                         impact|review|index|all (default: summary)
  --base <ref>           Base git ref for `review` mode
  --symbols <list>       Comma-separated symbols for `impact` mode
  --memory <limit>       Container memory cap (default: 4g)
  --cpus <n>             Container CPU cap (default: 4)
  --pids <n>             Container PID cap (default: 1024)
  --timeout <dur>        Host-side wall-clock deadline (default: 10m)
  --kill-grace <dur>     SIGKILL grace after SIGTERM (default: 30s)
  --gomemlimit <size>    Override GOMEMLIMIT inside container
  --out-root <dir>       Artifact root (default: harness_out/canopy)
  --exclude <glob>       Extra exclude pattern (repeatable)
  --no-default-excludes  Skip the built-in exclude list (not recommended)
  -v, --verbose          Echo the docker command before running
  -h, --help             Show this help

Environment:
  HOST_CANOPY            Path to canopy binary on host (default: ~/go/bin/canopy)

Output per run (under <out-root>/<stamp>-<mode>-<path-slug>/):
  canopy.stdout, canopy.stderr, time.txt, inspect.json, metadata.txt
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --path)           PATH_ARG="$2"; shift 2 ;;
    --mode)           MODE="$2"; shift 2 ;;
    --base)           BASE_REF="$2"; shift 2 ;;
    --symbols)        SYMBOLS="$2"; shift 2 ;;
    --memory)         MEMORY_LIMIT="$2"; shift 2 ;;
    --cpus)           CPUS_LIMIT="$2"; shift 2 ;;
    --pids)           PIDS_LIMIT="$2"; shift 2 ;;
    --timeout)        WALL_TIMEOUT="$2"; shift 2 ;;
    --kill-grace)     KILL_GRACE="$2"; shift 2 ;;
    --gomemlimit)     GOMEMLIMIT_VAL="$2"; shift 2 ;;
    --out-root)       OUT_ROOT="$2"; shift 2 ;;
    --exclude)        EXTRA_EXCLUDES+=("$2"); shift 2 ;;
    --no-default-excludes) NO_DEFAULT_EXCLUDES=1; shift ;;
    -v|--verbose)     VERBOSE=1; shift ;;
    -h|--help)        usage; exit 0 ;;
    *) echo "unknown option: $1" >&2; usage >&2; exit 2 ;;
  esac
done

if [[ ! -x "$HOST_CANOPY" ]]; then
  echo "canopy binary not found at $HOST_CANOPY (override with HOST_CANOPY=…)" >&2
  exit 2
fi

if [[ "$MODE" == "review" && -z "$BASE_REF" ]]; then
  echo "--mode review requires --base <ref>" >&2
  exit 2
fi
if [[ "$MODE" == "impact" && -z "$SYMBOLS" ]]; then
  echo "--mode impact requires --symbols <sym1,sym2,...>" >&2
  exit 2
fi

# Derive a conservative GOMEMLIMIT from --memory if not explicitly set.
derive_gomemlimit() {
  local raw="$1" lower
  raw="${raw//[[:space:]]/}"
  lower="$(printf '%s' "$raw" | tr '[:upper:]' '[:lower:]')"
  local num unit
  if [[ "$lower" =~ ^([0-9]+)(g|gi|gb|gib)$ ]]; then
    num="${BASH_REMATCH[1]}"
    # Leave broad headroom so Go GC triggers well before cgroup OOM.
    echo "$(( num * 512 ))MiB"
  elif [[ "$lower" =~ ^([0-9]+)(m|mi|mb|mib)$ ]]; then
    num="${BASH_REMATCH[1]}"
    echo "$(( num * 50 / 100 ))MiB"
  else
    echo ""
  fi
}
if [[ -z "$GOMEMLIMIT_VAL" ]]; then
  GOMEMLIMIT_VAL="$(derive_gomemlimit "$MEMORY_LIMIT")"
fi
if [[ -z "$INDEX_GC_EVERY_VAL" ]]; then
  case "${PATH_ARG#./}" in
    grammargen|grammargen/*) INDEX_GC_EVERY_VAL="1" ;;
  esac
fi

# Build excludes.
EXCLUDES=()
if [[ "$NO_DEFAULT_EXCLUDES" != "1" ]]; then
  EXCLUDES+=("${DEFAULT_EXCLUDES[@]}")
  if [[ "$PATH_ARG" == "." || "$PATH_ARG" == "./" ]]; then
    EXCLUDES+=("${ROOT_SWEEP_EXCLUDES[@]}")
  fi
fi
if [[ ${#EXTRA_EXCLUDES[@]} -gt 0 ]]; then
  EXCLUDES+=("${EXTRA_EXCLUDES[@]}")
fi

EXCLUDE_ARGS=()
for e in "${EXCLUDES[@]}"; do
  EXCLUDE_ARGS+=("-X" "$e")
done

# Normalize path slug for artifact dir.
path_slug() {
  local in="$1"
  in="${in#./}"
  in="${in%/}"
  if [[ -z "$in" || "$in" == "." ]]; then
    echo "root"
    return
  fi
  echo "$in" | tr '/' '-'
}

STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
SLUG="$(path_slug "$PATH_ARG")"
OUT_DIR="$OUT_ROOT/${STAMP}-${MODE}-${SLUG}"
mkdir -p "$OUT_DIR"

# Build the inner canopy command. Path inside container is relative to /workspace.
build_inner_cmd() {
  local mode="$1"
  local p="$PATH_ARG"
  # Index cache lives in tmpfs inside the container, not in the mounted repo.
  local cache_dir="/tmp/canopy-cache"
  local cache_file="$cache_dir/index.json"

  local excludes_str=""
  for e in "${EXCLUDES[@]}"; do
    excludes_str+=" -X \"$e\""
  done

  cat <<INNER
set -eo pipefail
mkdir -p $cache_dir
export PATH=/usr/local/bin:/usr/local/go/bin:\$PATH
if [[ -n "$GOMEMLIMIT_VAL" ]]; then
  export GOMEMLIMIT="$GOMEMLIMIT_VAL"
fi
if [[ -n "$INDEX_GC_EVERY_VAL" ]]; then
  export CANOPY_INDEX_GC_EVERY="$INDEX_GC_EVERY_VAL"
fi
export GOMAXPROCS="${CPUS_LIMIT%%.*}"
if [[ -z "\$GOMAXPROCS" || "\$GOMAXPROCS" == "0" ]]; then
  export GOMAXPROCS=1
fi
export GTS_MAX_CONCURRENT="\$GOMAXPROCS"
echo "--- canopy version ---"
canopy --version
echo "--- canopy index build ---"
/usr/bin/time -v canopy$excludes_str index build "$p" --out "$cache_file" 2>&1
echo "--- canopy $mode ---"
case "$mode" in
  summary)
    /usr/bin/time -v canopy$excludes_str analyze summary "$p" --cache "$cache_file" 2>&1 ;;
  smells)
    /usr/bin/time -v canopy$excludes_str analyze smells "$p" --cache "$cache_file" 2>&1 ;;
  complexity)
    /usr/bin/time -v canopy$excludes_str analyze complexity "$p" --cache "$cache_file" 2>&1 ;;
  coupling)
    /usr/bin/time -v canopy$excludes_str analyze coupling "$p" --cache "$cache_file" 2>&1 ;;
  hotspot)
    /usr/bin/time -v canopy$excludes_str analyze hotspot "$p" --cache "$cache_file" 2>&1 ;;
  dead)
    /usr/bin/time -v canopy$excludes_str graph dead "$p" --cache "$cache_file" 2>&1 ;;
  impact)
    /usr/bin/time -v canopy$excludes_str graph impact --symbols "$SYMBOLS" "$p" 2>&1 ;;
  review)
    /usr/bin/time -v canopy$excludes_str analyze review "$p" --base "$BASE_REF" --cache "$cache_file" 2>&1 ;;
  index)
    echo "(index already built above; nothing to do)" ;;
  all)
    for sub in summary smells complexity coupling; do
      echo "--- canopy analyze \$sub ---"
      /usr/bin/time -v canopy$excludes_str analyze \$sub "$p" --cache "$cache_file" 2>&1
    done ;;
  *)
    echo "unknown mode: $mode" >&2; exit 2 ;;
esac
INNER
}

INNER_CMD="$(build_inner_cmd "$MODE")"

CONTAINER_NAME="gts-canopy-${STAMP,,}-${SLUG//[^a-z0-9]/-}-${MODE}"
CID=""
cleanup() {
  if [[ -n "$CID" ]]; then
    docker rm -f "$CID" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

CID="$(docker create \
  --name "$CONTAINER_NAME" \
  --init \
  --memory "$MEMORY_LIMIT" \
  --memory-swap "$MEMORY_LIMIT" \
  --cpus "$CPUS_LIMIT" \
  --pids-limit "$PIDS_LIMIT" \
  --read-only \
  --tmpfs /tmp:size=512m,exec \
  --mount "type=bind,src=$REPO_ROOT,dst=/workspace,readonly" \
  --mount "type=bind,src=$HOST_CANOPY,dst=/usr/local/bin/canopy,readonly" \
  -w /workspace \
  "$IMAGE_TAG" \
  bash -c "$INNER_CMD")"

if [[ "$VERBOSE" == "1" ]]; then
  echo "--- inner command ---"
  echo "$INNER_CMD"
  echo "---------------------"
fi

echo "canopy run: mode=$MODE path=$PATH_ARG memory=$MEMORY_LIMIT cpus=$CPUS_LIMIT pids=$PIDS_LIMIT timeout=$WALL_TIMEOUT"
echo "container: $CONTAINER_NAME"
echo "artifacts: $OUT_DIR"

START_EPOCH="$(date +%s)"
docker start "$CID" >/dev/null

# Host-side deadline: if the run doesn't finish in $WALL_TIMEOUT, SIGTERM the
# container; if still alive after $KILL_GRACE, SIGKILL. This catches hangs
# that never trip the memory cap (infinite LALR loop, busy-wait, etc).
(
  sleep "$(numfmt --from=iec <(echo 0) 2>/dev/null || echo 0)" # no-op, placeholder for shellcheck
  : # placeholder
) &
WATCHDOG_PID=$!
kill "$WATCHDOG_PID" 2>/dev/null || true

(
  # Convert durations like 10m / 30s / 1h to seconds.
  dur_to_sec() {
    local d="$1"
    case "$d" in
      *h) echo $(( ${d%h} * 3600 )) ;;
      *m) echo $(( ${d%m} * 60 )) ;;
      *s) echo "${d%s}" ;;
      *)  echo "$d" ;;
    esac
  }
  term_after="$(dur_to_sec "$WALL_TIMEOUT")"
  kill_after="$(dur_to_sec "$KILL_GRACE")"
  sleep "$term_after"
  if docker inspect -f '{{.State.Running}}' "$CID" 2>/dev/null | grep -q true; then
    echo "[watchdog] wall timeout after ${WALL_TIMEOUT}, sending SIGTERM to $CONTAINER_NAME" >&2
    docker kill --signal=SIGTERM "$CID" >/dev/null 2>&1 || true
    sleep "$kill_after"
    if docker inspect -f '{{.State.Running}}' "$CID" 2>/dev/null | grep -q true; then
      echo "[watchdog] still running after ${KILL_GRACE}, sending SIGKILL" >&2
      docker kill --signal=SIGKILL "$CID" >/dev/null 2>&1 || true
    fi
  fi
) &
WATCHDOG_PID=$!

EXIT_CODE=""
while [[ -z "$EXIT_CODE" ]]; do
  RUNNING="$(docker inspect -f '{{.State.Running}}' "$CID" 2>/dev/null || true)"
  if [[ -z "$RUNNING" ]]; then
    EXIT_CODE="125"
    break
  fi
  if [[ "$RUNNING" != "true" ]]; then
    EXIT_CODE="$(docker inspect -f '{{.State.ExitCode}}' "$CID" 2>/dev/null || true)"
    if [[ -z "$EXIT_CODE" ]]; then
      EXIT_CODE="125"
    fi
    break
  fi
  sleep 0.5
done
kill "$WATCHDOG_PID" 2>/dev/null || true

docker logs "$CID" \
  > >(tee "$OUT_DIR/canopy.stdout") \
  2> >(tee "$OUT_DIR/canopy.stderr" >&2) || true

END_EPOCH="$(date +%s)"
WALL_SEC=$(( END_EPOCH - START_EPOCH ))

docker inspect "$CID" >"$OUT_DIR/inspect.json"
OOM_KILLED="$(docker inspect -f '{{.State.OOMKilled}}' "$CID")"
STATE_ERROR="$(docker inspect -f '{{.State.Error}}' "$CID")"
FINISHED_AT="$(docker inspect -f '{{.State.FinishedAt}}' "$CID")"

# Try to extract peak RSS from time -v output (maxresident in KB).
PEAK_RSS_KB="$(grep -Eo 'Maximum resident set size \(kbytes\): [0-9]+' "$OUT_DIR/canopy.stdout" 2>/dev/null | tail -1 | awk '{print $NF}')"
PEAK_RSS_KB="${PEAK_RSS_KB:-unknown}"

{
  echo "container_name=$CONTAINER_NAME"
  echo "image=$IMAGE_TAG"
  echo "mode=$MODE"
  echo "path=$PATH_ARG"
  echo "base_ref=$BASE_REF"
  echo "symbols=$SYMBOLS"
  echo "memory=$MEMORY_LIMIT"
  echo "cpus=$CPUS_LIMIT"
  echo "pids=$PIDS_LIMIT"
  echo "wall_timeout=$WALL_TIMEOUT"
  echo "kill_grace=$KILL_GRACE"
  echo "gomemlimit=$GOMEMLIMIT_VAL"
  echo "canopy_index_gc_every=$INDEX_GC_EVERY_VAL"
  echo "exit_code=$EXIT_CODE"
  echo "oom_killed=$OOM_KILLED"
  echo "state_error=$STATE_ERROR"
  echo "wall_seconds=$WALL_SEC"
  echo "peak_rss_kb=$PEAK_RSS_KB"
  echo "finished_at=$FINISHED_AT"
  echo "excludes=${EXCLUDES[*]:-}"
} >"$OUT_DIR/metadata.txt"

echo "exit=$EXIT_CODE oom=$OOM_KILLED wall=${WALL_SEC}s peak_rss_kb=$PEAK_RSS_KB"
if [[ "$EXIT_CODE" != "0" ]]; then
  exit "$EXIT_CODE"
fi
