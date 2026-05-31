#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DOCS=("storage/storage_status.md" "network/network_status.md")

failures=0

check_doc() {
  local doc_rel="$1"
  local doc_abs="$REPO_ROOT/$doc_rel"

  if [[ ! -f "$doc_abs" ]]; then
    echo "ERROR: missing status doc: $doc_rel"
    failures=$((failures + 1))
    return
  fi

  while IFS=$'\t' read -r op evidence row; do
    [[ -z "$op" ]] && continue

    local target
    target="$(printf '%s' "$evidence" | sed -nE 's/^\[[^]]+\]\(([^)]+)\)$/\1/p')"
    if [[ -z "$target" ]]; then
      echo "ERROR: $doc_rel:$row evidence cell must be a markdown link: $evidence"
      failures=$((failures + 1))
      continue
    fi

    local rel_path
    local line_no
    rel_path="$(printf '%s' "$target" | sed -nE 's/^([^#]+)#L([0-9]+)$/\1/p')"
    line_no="$(printf '%s' "$target" | sed -nE 's/^([^#]+)#L([0-9]+)$/\2/p')"
    if [[ -z "$rel_path" || -z "$line_no" ]]; then
      echo "ERROR: $doc_rel:$row evidence target must include #L<line>: $target"
      failures=$((failures + 1))
      continue
    fi

    local abs_path
    abs_path="$(cd "$(dirname "$doc_abs")" && realpath -m "$rel_path")"

    if [[ "$abs_path" != "$REPO_ROOT"/* ]]; then
      echo "ERROR: $doc_rel:$row evidence resolves outside repo: $target"
      failures=$((failures + 1))
      continue
    fi

    if [[ ! -f "$abs_path" ]]; then
      echo "ERROR: $doc_rel:$row evidence file not found: $target"
      failures=$((failures + 1))
      continue
    fi

    local code_line
    code_line="$(sed -n "${line_no}p" "$abs_path")"
    if [[ -z "$code_line" ]]; then
      echo "ERROR: $doc_rel:$row evidence line #L$line_no is empty or out of range: $target"
      failures=$((failures + 1))
      continue
    fi

    local symbol="$op"
    if [[ "$symbol" == *.* ]]; then
      symbol="${symbol##*.}"
    fi

    if [[ "$code_line" != *"$symbol"* ]]; then
      echo "ERROR: $doc_rel:$row operation '$op' does not match symbol at $target"
      echo "       code: $code_line"
      failures=$((failures + 1))
      continue
    fi

    if [[ ! "$code_line" =~ ^[[:space:]]*func[[:space:]] ]]; then
      echo "ERROR: $doc_rel:$row evidence line must point to a function definition: $target"
      echo "       code: $code_line"
      failures=$((failures + 1))
      continue
    fi
  done < <(
    awk '
      BEGIN { in_table = 0 }
      /^\| Operation \|/ { in_table = 1; next }
      in_table && /^\|[[:space:]-]+\|/ { next }
      in_table && /^\|/ {
        n = split($0, cols, "|")
        op = cols[2]
        ev = cols[n-1]
        gsub(/^[ \t]+|[ \t]+$/, "", op)
        gsub(/^[ \t]+|[ \t]+$/, "", ev)
        if (op != "" && ev ~ /^\[/) {
          printf "%s\t%s\t%d\n", op, ev, NR
        }
        next
      }
      in_table && !/^\|/ { in_table = 0 }
    ' "$doc_abs"
  )
}

for doc in "${DOCS[@]}"; do
  check_doc "$doc"
done

if [[ "$failures" -ne 0 ]]; then
  echo ""
  echo "Status-doc link check failed with $failures issue(s)."
  exit 1
fi

echo "Status-doc link check passed for ${DOCS[*]}."
