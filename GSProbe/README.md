# GSProbe

独立、单二进制的服务器质量检测工具，为 [GSVPS](https://www.gsvps.com) 生态提供检测内核。代码为全新实现，不包含 NodeQuality、融合怪或其他第三方检测项目源码。

## 已实现

- CPU：SHA-256 多核吞吐、gzip 压缩
- Memory：复制吞吐、随机访问延迟
- Disk：顺序读写、4K 随机读 IOPS
- System：系统、架构、虚拟化、容器、BBR/TCP 拥塞算法
- Network：IPv4/IPv6、公网 IP、ASN、GeoIP、DNS、TCP 延迟、下载采样
- Route：优先调用 `nexttrace`，其次 `tracepath`/`tracert`
- Streaming & AI：Netflix（双片源）、Disney+（bamgrid API）、YouTube、ChatGPT、Claude、Gemini 等平台专用检测
- SSE 实时日志、历史报告、Markdown/JSON 导出、暗色/亮色 UI
- Web 与静态资源通过 `embed.FS` 打包进单文件

> 流媒体检测采用 Netflix 双片源、Disney+ bamgrid API 等平台专用逻辑，可区分完整解锁 / 仅自制剧 / 部分可用等状态；仍不等同于账号登录后的完整播放验证。高级路由、fio、iperf3 等在后续版本通过适配器扩展。

## 构建

```bash
go test ./...
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o gsprobe ./cmd/gsprobe
```

## 使用

```bash
# Web 控制台
./gsprobe -addr :8899 -data ./data

# CLI 检测并输出 JSON
./gsprobe -run
```

浏览器访问 `http://服务器IP:8899`。

## API

- `POST /api/run`
- `GET /api/events`（SSE）
- `GET /api/reports`
- `GET /api/reports/latest`
- `GET /api/reports/{id}`
- `GET /api/reports/{id}/markdown`

## 安全提示

磁盘测试只在系统临时目录创建 `gsprobe-disk.tmp` 并在结束时删除。检测会产生约 256 MiB 临时写入及 10 MB 网络下载采样。

---

## 官网

**GSVPS：** https://www.gsvps.com

---

## 开源鸣谢

本项目为独立实现，以下开源项目的检测思路与方法在设计与开发中提供了重要参考：

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
