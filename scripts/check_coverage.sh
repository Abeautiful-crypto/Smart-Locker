#!/usr/bin/env bash
# 覆盖率门禁：CI 与本地同口径
# 用法: bash scripts/check_coverage.sh [阈值，默认85]
set -euo pipefail
cd "$(dirname "$0")/.."

THRESHOLD="${1:-85}"

echo "== go vet =="
go vet ./...

echo "== go test -race =="
go test -race -count=1 ./...

echo "== 合并覆盖率 =="
rm -f coverage_merged.out
for pkg in ./internal/biz ./internal/store ./internal/httpapi; do
  go test -coverprofile=cover_tmp.out "$pkg" > /dev/null
  if [ -f cover_tmp.out ]; then
    if [ -f coverage_merged.out ]; then
      tail -n +2 cover_tmp.out >> coverage_merged.out
    else
      cp cover_tmp.out coverage_merged.out
    fi
  fi
  rm -f cover_tmp.out
done

TOTAL=$(go tool cover -func=coverage_merged.out | awk '/^total:/ {print substr($3, 1, length($3)-1)}')
echo "整体语句覆盖率: ${TOTAL}%"
awk -v t="$TOTAL" -v th="$THRESHOLD" 'BEGIN { exit (t+0 >= th+0) ? 0 : 1 }' \
  && echo "✅ 覆盖率达标 (≥${THRESHOLD}%)" \
  || { echo "❌ 覆盖率不达标 (<${THRESHOLD}%)"; exit 1; }
