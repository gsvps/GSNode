# GSNode

> [Latest release](https://github.com/gsvps/GSNode/releases/latest) · **官网 [GSVPS](https://www.gsvps.com)**

**English**  
GSNode is a lightweight server node quality probe for the [GSVPS](https://www.gsvps.com) ecosystem. One command downloads the binary, runs a full benchmark, uploads the report to GSVPS, and prints the online report URL in your terminal.

**中文**  
GSNode 是 [GSVPS](https://www.gsvps.com) 生态下的服务器节点质量检测工具。一条命令完成下载、完整检测、报告上传，并在终端输出在线报告链接与模块评分摘要。

---

## One-line detect | 一键检测

```bash
curl -fsSL https://github.com/gsvps/GSNode/raw/main/install.sh | sh
```

备用地址（国内加速）：

```bash
curl -fsSL https://dl.gsvps.com/install.sh | sh
```

脚本会自动：

1. **临时下载**检测程序（默认不安装到系统）
2. 执行完整检测（CPU、内存、磁盘、网络、IP 质量、回程、流媒体等）
3. 将报告上传至 [GSVPS 官网](https://www.gsvps.com)
4. 在终端输出评分摘要与永久报告链接
5. **自动清理**所有本地临时文件（二进制、报告缓存），服务器不留痕迹

## Online report | 在线报告

检测完成后可在 [GSVPS](https://www.gsvps.com) 查看证书式在线报告：

![GSVPS 探针报告示例](./gsvps-probe-1FEBD3A5116D.webp)

示例：[https://www.gsvps.com/report/1febd3a5116d](https://www.gsvps.com/report/1febd3a5116d)

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
  https://www.gsvps.com/report/xxx
```

---

## Cleanup | 手动清理

若曾使用 `GSNODE_KEEP=1` 安装，或检测中断导致临时文件残留，可运行清理脚本：

```bash
curl -fsSL https://github.com/gsvps/GSNode/raw/main/cleanup.sh | sh
```

跳过确认：

```bash
curl -fsSL https://github.com/gsvps/GSNode/raw/main/cleanup.sh | sh -s -- -y
```

仅预览（不删除）：

```bash
curl -fsSL https://github.com/gsvps/GSNode/raw/main/cleanup.sh | sh -s -- -n
```

清理范围：`/usr/local/bin/gsnode`、临时目录 `/tmp/gsnode.*`、磁盘测试缓存 `gsprobe-disk.tmp`；可通过 `GSNODE_DATA`、`GSNODE_BIN` 指定额外路径。

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
| **回程** | nexttrace / tracepath / tracert，线路识别与路线图 |
| **流媒体 & AI** | Netflix（双片源）、Disney+（bamgrid API）、YouTube、ChatGPT、Claude、Gemini 等 |
| **Web 控制台** | SSE 实时日志、证书式报告 |

> 流媒体检测采用各平台专用 API 与片源探测逻辑，可区分 Netflix 完整解锁 / 仅自制剧、ChatGPT 仅网页 / 仅 APP 等状态；**不等同于**登录账号后的完整播放验证。

---

## Official site | 官网

- **GSVPS 官网：** https://www.gsvps.com
- **在线报告示例：** https://www.gsvps.com/report/1febd3a5116d

---

## Acknowledgments | 开源鸣谢

GSNode / GSProbe 为独立实现，未包含第三方检测项目源码。以下开源项目的思路与检测方法在设计与实现中提供了重要参考，特此致谢：

| 项目 | 说明 |
|------|------|
| [sjlleo/netflix-verify](https://github.com/sjlleo/netflix-verify) | Netflix 解锁检测（多片源 + 重定向地区码） |
| [xykt/RegionRestrictionCheck](https://github.com/xykt/RegionRestrictionCheck) | 流媒体解锁检测脚本（Disney+ bamgrid、YouTube、DAZN 等） |
| [lmc999/RegionRestrictionCheck](https://github.com/lmc999/RegionRestrictionCheck) | 上述脚本的原始版本 |
| [CoiaPrant/MediaUnlock_Test](https://github.com/CoiaPrant/MediaUnlock_Test) | 流媒体解锁检测早期实现 |
| [Peters-Pans/streamprobe](https://github.com/Peters-Pans/streamprobe) | Go 流媒体 / OpenAI 统一检测 CLI |
| [missuo/OpenAI-Checker](https://github.com/missuo/OpenAI-Checker) | OpenAI / ChatGPT 地区可用性检测 |
| [LemonBench/LemonBench](https://github.com/LemonBench/LemonBench) | 流媒体检测脚本演进脉络 |
| [NextTrace](https://github.com/nxtrace/Ntrace) | 回程路由探测工具（运行时按需安装） |

感谢以上作者与社区的贡献。

---

## License

MIT — see [LICENSE](LICENSE).

**Repository:** https://github.com/gsvps/GSNode
