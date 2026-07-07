#!/usr/bin/env sh
# GSNode 一键检测：安装客户端 → 完整检测 → 上传 GSVPS → 终端输出报告链接
# Usage: curl -fsSL https://github.com/gsvps/GSNode/raw/v0.1.1/install.sh | sh
set -eu

VERSION="${GSNODE_VERSION:-0.1.1}"
INSTALL_DIR="${GSNODE_INSTALL_DIR:-/usr/local/bin}"
REPO="${GSNODE_REPO:-https://github.com/gsvps/GSNode}"
BASE_URL="${GSNODE_BASE_URL:-$REPO/raw/v${VERSION}/bin}"
DATA_DIR="${GSNODE_DATA:-/tmp/gsnode-$$}"
GSVPS_UPLOAD_URL="${GSVPS_UPLOAD_URL:-https://www.gsvps.com/api/reports/upload}"
GSVPS_SITE_URL="${GSVPS_SITE_URL:-https://www.gsvps.com}"

if [ "${1:-}" = "--help" ] || [ "${1:-}" = "-h" ]; then
  cat <<EOF
GSNode 一键检测脚本

用法:
  curl -fsSL https://github.com/gsvps/GSNode/raw/v${VERSION}/install.sh | sh

环境变量:
  GSNODE_VERSION        版本号 (默认 ${VERSION})
  GSNODE_INSTALL_DIR    安装目录 (默认 /usr/local/bin)
  GSNODE_DATA           报告缓存目录
  GSVPS_UPLOAD_URL      上传 API (默认 ${GSVPS_UPLOAD_URL})
  GSVPS_SITE_URL        站点 URL (默认 ${GSVPS_SITE_URL})
  GSVPS_UPLOAD=0        禁用上传

仅安装不检测:
  GSNODE_INSTALL_ONLY=1 curl ... | sh
EOF
  exit 0
fi

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
TARGET_BIN="${GSNODE_BIN:-$INSTALL_DIR/gsnode}"

install_binary() {
  TMP="$(mktemp)"
  trap 'rm -f "$TMP"' EXIT INT HUP

  echo "===================================="
  echo " GSNode 一键检测"
  echo "===================================="
  echo "版本     : v${VERSION}"
  echo "平台     : ${OS}/${ARCH}"
  echo "下载地址 : ${DOWNLOAD_URL}"
  echo ""

  echo "→ 正在下载 GSNode ..."
  curl -fL --retry 3 --connect-timeout 30 --progress-bar "$DOWNLOAD_URL" -o "$TMP"
  chmod +x "$TMP"
  mkdir -p "$(dirname "$TARGET_BIN")"
  mv "$TMP" "$TARGET_BIN"
  trap - EXIT INT HUP
  echo "→ 已安装: $TARGET_BIN"
  echo ""
}

if [ -n "${GSNODE_BIN:-}" ]; then
  TARGET_BIN="$GSNODE_BIN"
elif [ -x "$INSTALL_DIR/gsnode" ]; then
  TARGET_BIN="$INSTALL_DIR/gsnode"
  echo "→ 使用已安装的 gsnode: $TARGET_BIN"
  echo ""
else
  install_binary
fi

if [ "${GSNODE_INSTALL_ONLY:-0}" = "1" ]; then
  echo "安装完成。运行完整检测:"
  echo "  $TARGET_BIN -run"
  exit 0
fi

mkdir -p "$DATA_DIR"
export GSVPS_UPLOAD_URL
export GSVPS_SITE_URL

echo "→ 开始完整检测（约 3–8 分钟，进度见下方日志）"
echo "→ 检测完成后将自动上传至 GSVPS 并显示在线报告链接"
echo ""

exec "$TARGET_BIN" -run -data "$DATA_DIR"
