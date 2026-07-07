#!/usr/bin/env sh
# GSNode 纯净一键检测：临时下载 → 完整检测 → 上传 GSVPS → 终端输出 → 自动清理
# Usage: curl -fsSL https://github.com/gsvps/GSNode/raw/v0.1.14/install.sh | sh
set -eu

VERSION="${GSNODE_VERSION:-0.1.14}"
REPO="${GSNODE_REPO:-https://github.com/gsvps/GSNode}"
BASE_URL="${GSNODE_BASE_URL:-$REPO/raw/v${VERSION}/bin}"
INSTALL_DIR="${GSNODE_INSTALL_DIR:-/usr/local/bin}"
GSVPS_UPLOAD_URL="${GSVPS_UPLOAD_URL:-https://www.gsvps.com/api/reports/upload}"
GSVPS_SITE_URL="${GSVPS_SITE_URL:-https://www.gsvps.com}"

WORK_DIR=""
DATA_DIR=""
TARGET_BIN=""
DOWNLOADED_BIN=""
INSTALLED_THIS_RUN=0
PURE_MODE=1

if [ "${GSNODE_KEEP:-0}" = "1" ] || [ "${GSNODE_INSTALL_ONLY:-0}" = "1" ]; then
  PURE_MODE=0
fi

if [ "${1:-}" = "--help" ] || [ "${1:-}" = "-h" ]; then
  cat <<EOF
GSNode 纯净一键检测

用法:
  curl -fsSL https://github.com/gsvps/GSNode/raw/v${VERSION}/install.sh | sh

默认行为（纯净模式）:
  临时下载检测程序 → 完整检测 → 上传 GSVPS → 终端显示结果 → 自动清理所有本地文件
  不会在系统中永久安装 gsnode，也不会留下报告缓存。

环境变量:
  GSNODE_VERSION        版本号 (默认 ${VERSION})
  GSVPS_UPLOAD_URL      上传 API (默认 ${GSVPS_UPLOAD_URL})
  GSVPS_SITE_URL        站点 URL (默认 ${GSVPS_SITE_URL})
  GSVPS_UPLOAD=0        禁用上传
  GSNODE_KEEP=1         检测后保留安装到 ${INSTALL_DIR}/gsnode
  GSNODE_INSTALL_ONLY=1 仅安装，不检测（需配合 GSNODE_KEEP=1）
  GSNODE_BIN            使用已有二进制路径
  GSNODE_DATA           自定义报告缓存目录

手动清理:
  curl -fsSL https://github.com/gsvps/GSNode/raw/v${VERSION}/cleanup.sh | sh
EOF
  exit 0
fi

cleanup() {
  code=$?

  if [ -n "${DATA_DIR:-}" ] && [ -d "$DATA_DIR" ]; then
    rm -rf "$DATA_DIR"
  fi

  if [ "$PURE_MODE" = "1" ]; then
    if [ -n "${DOWNLOADED_BIN:-}" ] && [ -f "$DOWNLOADED_BIN" ]; then
      rm -f "$DOWNLOADED_BIN"
    fi
    if [ -n "${WORK_DIR:-}" ] && [ -d "$WORK_DIR" ]; then
      rm -rf "$WORK_DIR"
    fi
    rm -f "${TMPDIR:-/tmp}/gsprobe-disk.tmp" 2>/dev/null || true
    echo ""
    echo "→ 已清理本地检测临时文件（未在系统中安装 gsnode）"
  elif [ "$INSTALLED_THIS_RUN" = "0" ] && [ -n "${DATA_DIR:-}" ]; then
    :
  fi

  exit "$code"
}

trap cleanup EXIT INT HUP TERM

if ! command -v curl >/dev/null 2>&1; then
  echo "错误: 需要 curl" >&2
  exit 1
fi

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$OS" in
  linux) ;;
  darwin) OS=darwin ;;
  *)
    echo "错误: 暂不支持的操作系统 $OS" >&2
    exit 1
    ;;
esac

case "$ARCH" in
  x86_64|amd64) ARCH=amd64 ;;
  aarch64|arm64) ARCH=arm64 ;;
  i686|i386|x86) ARCH=386 ;;
  *)
    echo "错误: 暂不支持架构 $ARCH" >&2
    exit 1
    ;;
esac

BIN_NAME="gsnode-${OS}-${ARCH}"
DOWNLOAD_URL="$BASE_URL/$BIN_NAME"

download_to() {
  dest="$1"
  echo "→ 正在下载 GSNode ..."
  curl -fL --retry 3 --connect-timeout 30 --progress-bar "$DOWNLOAD_URL" -o "$dest"
  chmod +x "$dest"
}

prepare_binary() {
  if [ -n "${GSNODE_BIN:-}" ] && [ -x "$GSNODE_BIN" ]; then
    TARGET_BIN="$GSNODE_BIN"
    DATA_DIR="${GSNODE_DATA:-${TMPDIR:-/tmp}/gsnode-$$}"
    mkdir -p "$DATA_DIR"
    echo "→ 使用已有二进制: $TARGET_BIN"
    return
  fi

  if [ "$PURE_MODE" = "0" ]; then
    TARGET_BIN="${GSNODE_BIN:-$INSTALL_DIR/gsnode}"
    if [ -x "$TARGET_BIN" ]; then
      echo "→ 使用已安装: $TARGET_BIN"
    else
      mkdir -p "$(dirname "$TARGET_BIN")"
      TMP_DL="$(mktemp)"
      download_to "$TMP_DL"
      mv "$TMP_DL" "$TARGET_BIN"
      INSTALLED_THIS_RUN=1
      echo "→ 已安装: $TARGET_BIN"
    fi
    DATA_DIR="${GSNODE_DATA:-${TMPDIR:-/tmp}/gsnode-$$}"
    mkdir -p "$DATA_DIR"
    return
  fi

  WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/gsnode.XXXXXX")"
  DOWNLOADED_BIN="$WORK_DIR/gsnode"
  DATA_DIR="$WORK_DIR/data"
  TARGET_BIN="$DOWNLOADED_BIN"
  mkdir -p "$DATA_DIR"

  download_to "$DOWNLOADED_BIN"
  echo "→ 已就绪: 临时二进制（检测后自动删除）"
  echo ""
}

echo "===================================="
echo " GSNode 一键检测"
echo "===================================="
echo "版本     : v${VERSION}"
echo "平台     : ${OS}/${ARCH}"
if [ "$PURE_MODE" = "1" ] && [ -z "${GSNODE_BIN:-}" ]; then
  echo "模式     : 纯净检测（完成后自动清理）"
elif [ "$PURE_MODE" = "0" ]; then
  echo "模式     : 保留安装"
fi
echo ""

prepare_binary

if [ "${GSNODE_INSTALL_ONLY:-0}" = "1" ]; then
  echo "安装完成。运行完整检测:"
  echo "  $TARGET_BIN -run"
  exit 0
fi

export GSVPS_UPLOAD_URL
export GSVPS_SITE_URL
export GSPROBE_PING_TARGETS_URL="${GSPROBE_PING_TARGETS_URL:-$REPO/raw/v${VERSION}/ping_targets.json}"

echo "→ 开始完整检测（约 3–8 分钟，进度见下方日志）"
if [ "${GSVPS_UPLOAD:-1}" != "0" ]; then
  echo "→ 检测完成后将上传至 GSVPS 并显示在线报告链接"
fi
if [ "$PURE_MODE" = "1" ]; then
  echo "→ 检测结束后将自动清理所有本地文件"
fi
echo ""

"$TARGET_BIN" -run -data "$DATA_DIR"
