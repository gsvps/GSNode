# GSNode

> **Version 0.1.2** · [Releases](https://github.com/gsvps/GSNode/releases)

**English**  
GSNode is a lightweight server node quality probe for the [GSVPS](https://www.gsvps.com) ecosystem. One command installs the binary, runs a full benchmark, uploads the report to GSVPS, and prints the online report URL in your terminal.

**中文**  
GSNode 是 GSVPS 生态下的服务器节点质量检测工具。一条命令完成安装、完整检测、报告上传 GSVPS，并在终端显示在线报告链接与模块评分摘要。

---

## One-line detect | 一键检测

```bash
curl -fsSL https://github.com/gsvps/GSNode/raw/v0.1.2/install.sh | sh
```

脚本会自动：

1. **临时下载**检测程序（默认不安装到系统）
2. 执行完整检测（CPU、内存、磁盘、网络、IP 质量、回程、流媒体等）
3. 将报告上传至 [GSVPS](https://www.gsvps.com)
4. 在终端输出评分摘要与永久报告链接
5. **自动清理**所有本地临时文件（二进制、报告缓存），服务器不留痕迹

终端示例：

```text
====================================
 GSNode 检测完成
====================================
报告编号 : 4E101587B749
综合评分 : 6463 / 10000  ★★★★☆

模块明细
------------------------------------
 System          7000  passed   Debian GNU/Linux ...
 CPU             ...
 ...

上传状态 : 成功

在线报告 :
  https://www.gsvps.com/report/4e101587b749
```

---

## Environment | 环境变量

| Variable | Default | Description |
|----------|---------|-------------|
| `GSVPS_UPLOAD_URL` | `https://www.gsvps.com/api/reports/upload` | Upload API |
| `GSVPS_SITE_URL` | `https://www.gsvps.com` | Site base URL |
| `GSVPS_UPLOAD=0` | — | Disable upload |
| `GSNODE_KEEP=1` | — | Keep binary installed to `/usr/local/bin` after run |
| `GSNODE_INSTALL_ONLY=1` | — | Install only, skip benchmark (use with `GSNODE_KEEP=1`) |
| `GSNODE_BIN` | — | Use existing binary path |
| `GSNODE_DATA` | temp dir | Local report cache dir (auto-cleaned in pure mode) |

---

## Manual usage | 手动用法

```bash
# Full benchmark + upload + terminal summary
gsnode -run

# Full benchmark, JSON output (for scripting)
gsnode -run -json

# Web console with live SSE logs
gsnode -addr :8899 -data ./data
```

Open `http://YOUR_SERVER_IP:8899` for the certificate-style Web UI.

---

## Binaries | 预编译二进制

See [`bin/`](bin/):

| File | Platform |
|------|----------|
| `gsnode-linux-amd64` | Linux x86_64 |
| `gsnode-linux-arm64` | Linux ARM64 |
| `gsnode-linux-386` | Linux i386 |
| `gsnode-darwin-amd64` | macOS Intel |
| `gsnode-darwin-arm64` | macOS Apple Silicon |
| `gsnode-windows-amd64.exe` | Windows x86_64 |

---

## Features | 功能

| 模块 | 说明 |
|------|------|
| **系统** | OS、虚拟化、容器、BBR/TCP、sysctl |
| **CPU / 内存 / 磁盘** | 算力、带宽、I/O 基准 |
| **网络** | GeoIP、ASN、DNS、延迟、上下行采样 |
| **IP 质量** | 多源 IP 类型、风险评分、DNSBL |
| **回程** | nexttrace / tracepath / tracert |
| **流媒体 & AI** | Netflix、YouTube、ChatGPT、Claude 等 |
| **Web 控制台** | SSE 实时日志、证书式报告 |

---

## License

MIT — see [LICENSE](LICENSE).

**Repository:** https://github.com/gsvps/GSNode
