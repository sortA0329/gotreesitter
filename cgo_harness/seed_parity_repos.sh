#!/usr/bin/env bash
set -euo pipefail

# Seed a local grammar parity root with pinned repos from grammars/languages.lock.
#
# Default focus:
#   javascript,typescript,tsx,c,c_sharp,cobol
#   cpp stays opt-in for now: direct grammargen-vs-C generation still times out
#   beyond the bounded default container budget.
#
# Examples:
#   cgo_harness/seed_parity_repos.sh
#   cgo_harness/seed_parity_repos.sh --dest .parity_seed
#   cgo_harness/seed_parity_repos.sh --langs javascript,typescript,tsx,c,c_sharp,cobol
#   cgo_harness/seed_parity_repos.sh --langs cpp  # opt-in quarantine
#
# This script is intentionally narrow and clone-safe: it only hydrates the
# requested grammar repos at the exact pinned commit, so focused parity work
# does not require cloning the entire /tmp/grammar_parity universe.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
LOCK_FILE="$REPO_ROOT/grammars/languages.lock"

DEST_DIR="/tmp/grammar_parity"
LANGS="javascript,typescript,tsx,c,c_sharp,cobol"

usage() {
  cat <<'EOF'
Usage: seed_parity_repos.sh [options]

Options:
  --dest DIR      Destination root for seeded grammar repos
                  (default: /tmp/grammar_parity)
  --langs LIST    Comma-separated grammar names
                  (default: javascript,typescript,tsx,c,c_sharp,cobol)
  -h, --help      Show this help
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dest)
      DEST_DIR="$2"
      shift 2
      ;;
    --langs)
      LANGS="$2"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown option: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if [[ ! -f "$LOCK_FILE" ]]; then
  echo "languages.lock not found: $LOCK_FILE" >&2
  exit 1
fi

DEST_DIR="${DEST_DIR/#\~/$HOME}"
mkdir -p "$DEST_DIR"

canonical_lock_name() {
  case "$1" in
    gitcommit_gbprod) echo "gitcommit" ;;
    c_lang) echo "c" ;;
    go_lang) echo "go" ;;
    *) echo "$1" ;;
  esac
}

root_dir_for_lang() {
  case "$1" in
    gitcommit|gitcommit_gbprod) echo "gitcommit_gbprod" ;;
    tsx|typescript) echo "typescript" ;;
    markdown|markdown_inline) echo "markdown" ;;
    xml|dtd) echo "xml" ;;
    php) echo "php" ;;
    ocaml) echo "ocaml" ;;
    csv) echo "csv" ;;
    c_lang) echo "c" ;;
    go_lang) echo "go" ;;
    *) echo "$1" ;;
  esac
}

lock_repo_url() {
  local lock_name="$1"
  awk -v target="$lock_name" '$1 == target && $1 !~ /^#/ { print $2; exit }' "$LOCK_FILE"
}

lock_repo_commit() {
  local lock_name="$1"
  awk -v target="$lock_name" '$1 == target && $1 !~ /^#/ { print $3; exit }' "$LOCK_FILE"
}

ensure_repo_at_commit() {
  local lang_name="$1"
  local lock_name root_dir url commit dest current

  lock_name="$(canonical_lock_name "$lang_name")"
  root_dir="$(root_dir_for_lang "$lang_name")"
  url="$(lock_repo_url "$lock_name")"
  commit="$(lock_repo_commit "$lock_name")"
  dest="$DEST_DIR/$root_dir"

  if [[ -z "$url" || -z "$commit" ]]; then
    echo "missing languages.lock entry for $lock_name" >&2
    exit 1
  fi

  if [[ -d "$dest/.git" ]]; then
    current="$(git -C "$dest" rev-parse HEAD 2>/dev/null || true)"
    if [[ "$current" == "$commit" ]]; then
      echo "ok   $lang_name -> $dest @ ${commit:0:12}"
      return
    fi
    git -C "$dest" fetch --depth=1 origin "$commit" >/dev/null 2>&1
    git -C "$dest" checkout --detach "$commit" >/dev/null 2>&1
    echo "sync $lang_name -> $dest @ ${commit:0:12}"
    return
  fi

  rm -rf "$dest"
  git clone --depth=1 "$url" "$dest" >/dev/null 2>&1
  git -C "$dest" fetch --depth=1 origin "$commit" >/dev/null 2>&1
  git -C "$dest" checkout --detach "$commit" >/dev/null 2>&1
  echo "seed $lang_name -> $dest @ ${commit:0:12}"
}

declare -A seen_roots=()
IFS=',' read -r -a requested_langs <<< "$LANGS"
for raw_lang in "${requested_langs[@]}"; do
  lang="${raw_lang//[[:space:]]/}"
  if [[ -z "$lang" ]]; then
    continue
  fi
  root_dir="$(root_dir_for_lang "$lang")"
  if [[ -n "${seen_roots[$root_dir]:-}" ]]; then
    continue
  fi
  seen_roots["$root_dir"]=1
  ensure_repo_at_commit "$lang"
done

echo "seeded parity repos under $DEST_DIR"
