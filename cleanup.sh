#!/usr/bin/env sh
# GSNode 清理脚本：移除本地 gsnode 二进制、报告缓存与检测临时文件
# Usage: curl -fsSL https://github.com/gsvps/GSNode/raw/main/cleanup.sh | sh
set -eu

VERSION="${GSNODE_VERSION:-0.1.26}"
INSTALL_DIR="${GSNODE_INSTALL_DIR:-/usr/local/bin}"
DRY_RUN=0
ASSUME_YES=0

usage() {
  cat <<EOF
GSNode 清理脚本

用法:
  curl -fsSL https://github.com/gsvps/GSNode/raw/main/cleanup.sh | sh
  curl -fsSL https://dl.gsvps.com/cleanup.sh | sh
  curl -fsSL https://github.com/gsvps/GSNode/raw/main/cleanup.sh | sh -s -- -y

清理内容:
  - 已安装的 gsnode 二进制 (${INSTALL_DIR}/gsnode)
  - /tmp 下 gsnode 临时目录 (gsnode.* / gsnode-*)
  - 磁盘测试临时文件 (gsprobe-disk.tmp)
  - 可选: GSNODE_DATA 指定的报告目录
  - 可选: GSNODE_CLEAN_LOCAL=1 时清理当前目录 ./data

选项:
  -y, --yes       跳过确认
  -n, --dry-run   仅显示将删除的内容，不实际删除
  -h, --help      显示帮助

环境变量:
  GSNODE_INSTALL_DIR  安装目录 (默认 ${INSTALL_DIR})
  GSNODE_BIN          额外指定要删除的二进制路径
  GSNODE_DATA         指定要删除的报告缓存目录
  GSNODE_CLEAN_LOCAL=1  同时清理 ./data
  GSNODE_YES=1        等同 -y
EOF
}

for arg in "$@"; do
  case "$arg" in
    -y|--yes) ASSUME_YES=1 ;;
    -n|--dry-run) DRY_RUN=1 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "未知选项: $arg" >&2; usage >&2; exit 1 ;;
  esac
done

if [ "${GSNODE_YES:-0}" = "1" ]; then
  ASSUME_YES=1
fi

# shellcheck disable=SC2034
REMOVED=0

log_action() {
  printf '%s\n' "$1"
}

do_remove() {
  target="$1"
  [ -e "$target" ] || return 0
  if [ "$DRY_RUN" = "1" ]; then
    log_action "  [dry-run] 将删除: $target"
  else
    rm -rf "$target"
    log_action "  ✓ 已删除: $target"
  fi
  REMOVED=1
}

collect_temp_dirs() {
  tmp_root="${TMPDIR:-/tmp}"
  for base in "$tmp_root" /tmp /var/tmp; do
    [ -d "$base" ] || continue
    for entry in "$base"/gsnode.* "$base"/gsnode-*; do
      [ -e "$entry" ] || continue
      case "$entry" in
        *'*'*) continue ;;
      esac
      printf '%s\n' "$entry"
    done
  done
}

if command -v pgrep >/dev/null 2>&1; then
  if pgrep -x gsnode >/dev/null 2>&1; then
    echo "错误: 检测到 gsnode 正在运行，请先停止后再清理。" >&2
    echo "提示: pkill gsnode  或  kill \$(pgrep -x gsnode)" >&2
    exit 1
  fi
fi

echo "===================================="
echo " GSNode 清理"
echo "===================================="
echo "版本 : v${VERSION}"
echo ""

TARGETS=""
DEFAULT_BIN="${GSNODE_BIN:-$INSTALL_DIR/gsnode}"
TARGETS="$TARGETS
$DEFAULT_BIN"

if [ -n "${GSNODE_BIN:-}" ] && [ "$GSNODE_BIN" != "$DEFAULT_BIN" ]; then
  TARGETS="$TARGETS
$GSNODE_BIN"
fi

for extra in /usr/bin/gsnode /usr/local/bin/gsnode "$HOME/.local/bin/gsnode"; do
  if [ "$extra" != "$DEFAULT_BIN" ]; then
    TARGETS="$TARGETS
$extra"
  fi
done

if [ -n "${GSNODE_DATA:-}" ]; then
  TARGETS="$TARGETS
$GSNODE_DATA"
fi

if [ "${GSNODE_CLEAN_LOCAL:-0}" = "1" ] && [ -d "./data" ]; then
  TARGETS="$TARGETS
$(cd . && pwd)/data"
fi

TEMP_DIRS="$(collect_temp_dirs || true)"
DISK_TMP="${TMPDIR:-/tmp}/gsprobe-disk.tmp"
if [ ! -f "$DISK_TMP" ] && [ -f "/tmp/gsprobe-disk.tmp" ]; then
  DISK_TMP="/tmp/gsprobe-disk.tmp"
fi

echo "将清理以下内容:"
echo ""

FOUND=0

for target in $TARGETS; do
  [ -n "$target" ] || continue
  if [ -e "$target" ]; then
    echo "  · $target"
    FOUND=1
  fi
done

for dir in $TEMP_DIRS; do
  [ -n "$dir" ] || continue
  echo "  · $dir"
  FOUND=1
done

if [ -f "$DISK_TMP" ]; then
  echo "  · $DISK_TMP"
  FOUND=1
fi

if [ "$FOUND" = "0" ]; then
  echo "  （未发现 GSNode 相关文件，系统已是干净状态）"
  exit 0
fi

echo ""

if [ "$DRY_RUN" = "1" ]; then
  echo "→ dry-run 模式，未执行删除"
  exit 0
fi

if [ "$ASSUME_YES" != "1" ]; then
  printf '确认删除以上文件? [y/N] '
  read -r ans || ans=""
  case "$ans" in
    y|Y|yes|YES) ;;
    *) echo "已取消"; exit 0 ;;
  esac
fi

echo ""
echo "→ 开始清理 ..."
echo ""

for target in $TARGETS; do
  [ -n "$target" ] || continue
  do_remove "$target"
done

for dir in $TEMP_DIRS; do
  [ -n "$dir" ] || continue
  do_remove "$dir"
done

do_remove "$DISK_TMP"

echo ""
if [ "$REMOVED" = "1" ]; then
  echo "→ 清理完成"
else
  echo "→ 未发现可删除的文件"
fi
