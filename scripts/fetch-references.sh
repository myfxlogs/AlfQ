#!/usr/bin/env bash
# fetch-references.sh — 克隆开源参考项目到 references/
# 用法: bash scripts/fetch-references.sh [project-name]
#       不带参数列出所有可用项目
set -uo pipefail

REFS_DIR="$(cd "$(dirname "$0")/.." && pwd)/references"
mkdir -p "$REFS_DIR"

declare -A PROJECTS=(
  ["bbgo"]="https://github.com/c9s/bbgo.git"
  ["nautilus_trader"]="https://github.com/nautechsystems/nautilus_trader.git"
  ["qlib"]="https://github.com/microsoft/qlib.git"
  ["gocryptotrader"]="https://github.com/thrasher-corp/gocryptotrader.git"
  ["Lean"]="https://github.com/QuantConnect/Lean.git"
  ["StockSharp"]="https://github.com/StockSharp/StockSharp.git"
  ["hftbacktest"]="https://github.com/nkaz001/hftbacktest.git"
  ["vectorbt"]="https://github.com/polakowo/vectorbt.git"
  ["quantstats"]="https://github.com/ranaroussi/quantstats.git"
  ["jesse"]="https://github.com/jesse-ai/jesse.git"
  ["barter"]="https://github.com/barter-rs/barter-rs.git"
  ["zipline"]="https://github.com/quantopian/zipline.git"
  ["alphalens"]="https://github.com/quantopian/alphalens.git"
  ["pyfolio"]="https://github.com/quantopian/pyfolio.git"
  ["QuantDinger"]="https://github.com/brokermr810/QuantDinger.git"
)

list_projects() {
  echo "可用参考项目:"
  for name in "${!PROJECTS[@]}"; do
    printf "  %-20s  %s\n" "$name" "${PROJECTS[$name]}"
  done
}

clone_project() {
  local name="$1"
  local url="${PROJECTS[$name]:-}"

  if [ -z "$url" ]; then
    echo "错误: 未知项目 '$name'"
    echo ""
    list_projects
    exit 1
  fi

  local target="$REFS_DIR/$name"
  if [ -d "$target" ]; then
    echo "已存在 $target，执行 git pull..."
    git -C "$target" pull --ff-only || echo "警告: pull 失败，使用现有副本"
  else
    echo "克隆 $name → $target ..."
    git clone --depth 1 "$url" "$target"
    echo "完成: $target"
  fi
}

case "${1:-}" in
  ""|-h|--help)
    echo "用法: bash scripts/fetch-references.sh [project-name]"
    echo "       bash scripts/fetch-references.sh --all"
    echo ""
    list_projects
    ;;
  --all)
    echo "批量克隆/更新全部 ${#PROJECTS[@]} 个参考项目 ..."
    for name in "${!PROJECTS[@]}"; do
      clone_project "$name"
    done
    echo "全部完成"
    ;;
  *)
    clone_project "$1"
    ;;
esac
