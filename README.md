# GSNode

> **Version 0.1.0** · [Releases](https://github.com/gsvps/GSNode/releases/tag/v0.1.0)

**English**  
GSNode is a lightweight server node quality probe for the [GSVPS](https://github.com/gsvps) ecosystem. Packaged as a single binary with an embedded Web UI, it inspects system performance, network quality, IP reputation, return routes, and streaming reachability—then outputs a certificate-style report (JSON / Markdown).

**中文**  
GSNode 是 GSVPS 生态下的服务器节点质量检测工具。以单个二进制运行，内置 Web 控制台，可对节点进行系统性能、网络质量、IP 信誉、回程路由与流媒体可达性检测，并生成证书式报告（支持 JSON / Markdown 导出）。

---

## Features | 功能

| Module | Description |
|--------|-------------|
| **System** | OS, virtualization, container, BBR / TCP tuning, sysctl |
| **CPU / Memory / Disk** | SHA-256, gzip, copy/latency, sequential & 4K random I/O |
| **Network** | IPv4/IPv6, GeoIP, ASN, DNS, latency, download sample |
| **IP Quality** | Multi-source IP type, risk score, DNSBL, mail port 25 |
| **Route** | Return path via `nexttrace` / `tracepath` / `tracert` |
| **Streaming & AI** | Netflix, Disney+, YouTube, ChatGPT, Claude, Gemini, … |
| **Web UI** | Live SSE logs, dark/light theme, certificate report |

| 模块 | 说明 |
|------|------|
| **系统** | 操作系统、虚拟化、容器、BBR/TCP、sysctl 本地策略 |
| **CPU / 内存 / 磁盘** | SHA-256、gzip、复制/延迟、顺序与 4K 随机 I/O |
| **网络** | IPv4/IPv6、GeoIP、ASN、DNS、延迟、下载采样 |
| **IP 质量** | 多源 IP 类型、风险评分、DNSBL、25 端口 |
| **回程** | `nexttrace` / `tracepath` / `tracert` |
| **流媒体 & AI** | Netflix、Disney+、YouTube、ChatGPT、Claude、Gemini 等 |
| **Web 控制台** | SSE 实时日志、暗色/亮色、证书式报告 |

---

## Quick Start | 快速开始

### One-line install (Linux / macOS) | 一键安装

```bash
curl -fsSL https://github.com/gsvps/GSNode/raw/v0.1.0/install.sh | sh
```

### Manual download | 手动下载

Pick the binary for your platform from [`bin/`](bin/):

| File | Platform |
|------|----------|
| `gsnode-linux-amd64` | Linux x86_64 |
| `gsnode-linux-arm64` | Linux ARM64 |
| `gsnode-linux-386` | Linux i386 |
| `gsnode-darwin-amd64` | macOS Intel |
| `gsnode-darwin-arm64` | macOS Apple Silicon |
| `gsnode-windows-amd64.exe` | Windows x86_64 |

```bash
chmod +x gsnode-linux-amd64
sudo mv gsnode-linux-amd64 /usr/local/bin/gsnode
```

---

## Usage | 用法

### Web console | Web 控制台

```bash
gsnode -addr :8899 -data ./data
```

Open `http://YOUR_SERVER_IP:8899` in your browser.

浏览器访问 `http://服务器IP:8899`。

### CLI quick / full check | 命令行快速 / 完整检测

```bash
# Quick mode | 快速检测
gsnode -quick

# Full mode | 完整检测
gsnode -run

# Or use helper script | 或使用脚本
./detect.sh quick
./detect.sh full
```

### Helper scripts | 辅助脚本

| Script | Description |
|--------|-------------|
| [`install.sh`](install.sh) | Download & install `gsnode` to `/usr/local/bin` |
| [`detect.sh`](detect.sh) | Run quick or full benchmark |
| [`start.sh`](start.sh) | Start Web UI server |

| 脚本 | 说明 |
|------|------|
| [`install.sh`](install.sh) | 下载并安装到 `/usr/local/bin` |
| [`detect.sh`](detect.sh) | 快速或完整检测 |
| [`start.sh`](start.sh) | 启动 Web 控制台 |

Environment variables | 环境变量:

```bash
GSNODE_BIN=/usr/local/bin/gsnode   # binary path | 二进制路径
GSNODE_ADDR=:8899                  # listen address | 监听地址
GSNODE_DATA=./data                 # report storage | 报告目录
GSNODE_BASE_URL=...                # custom download base | 自定义下载地址
```

---

## API

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/run?quick=true` | Start benchmark |
| `GET` | `/api/events` | SSE live log stream |
| `GET` | `/api/reports` | List reports |
| `GET` | `/api/reports/latest` | Latest report JSON |
| `GET` | `/api/reports/{id}` | Report by ID |
| `GET` | `/api/reports/{id}/markdown` | Export Markdown |

---

## Notes | 说明

- Streaming checks indicate **HTTP reachability only**, not full regional unlock.
- Full mode writes ~256 MiB temp disk data and ~10 MB download sample.
- Optional tools (`nexttrace`, `tracepath`) enhance route detection when installed.
- 流媒体检测仅表示 **HTTP 可达**，不代表完整解锁。
- 完整模式会产生约 256 MiB 磁盘临时写入及 10 MB 网络下载采样。
- 可选工具（`nexttrace`、`tracepath`）安装后可增强回程检测。

---

## License

MIT — see [LICENSE](LICENSE).

---

**Repository:** https://github.com/gsvps/GSNode
