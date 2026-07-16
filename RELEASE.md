# GSNode 最新发布

当前版本：**v0.1.23**（见 `VERSION`）

## 安装

> **请务必使用最新版本。** 当前版本：**v0.1.23**。运行脚本后终端会输出实际版本号。

```bash
curl -fsSL https://dl.gsvps.com/install.sh?v=0.1.23 | sh
```

备用地址：

```bash
# GitHub
curl -fsSL https://github.com/gsvps/GSNode/raw/v0.1.23/install.sh | sh

# jsDelivr CDN
curl -fsSL https://cdn.jsdelivr.net/gh/gsvps/GSNode@v0.1.23/install.sh | sh
```

## v0.1.23 更新

- IP 类型：原生/广播改为与 xykt IPQuality 一致（使用地国家码 == 注册地国家码）
- IP 纯净度：多源加权（Scamalytics/IPQS/AbuseIPDB/ipapi）+ DNSBL + 代理/VPN/Tor 等因子，避免单源掩盖

## v0.1.22 更新

- IP 质量 / 网络：公网 IP 强制 IPv4 探测（避免双栈误报 IPv6）
- IP 类型改为「原生IP / 广播IP」（出口是否在本机网卡）；网页报告保持脱敏

## v0.1.21 更新

- 修复 Scamalytics 风险分解析：旧正则误匹配 `</h1>`，导致 IP 纯净度几乎全是 99%
- 改为解析 `Fraud Score: N` / JSON `"score"`，与 xykt IPQuality 等工具一致

## v0.1.20 更新

- IP 质量：基础信息新增「IP纯净度」百分比（越高越纯净，红/黄/绿区分）
- 优先 Scamalytics，失败时按多源风险分与 DNSBL 合成

## v0.1.19 更新

- 终端输出：公网 IP、回程路由跳点等不再脱敏，完整显示检测结果
- 网页报告仍保持 IP 脱敏展示

## v0.1.18 更新

- 参考数据：ping_targets / dnsbl 下载失败时新增 jsDelivr CDN 第三备用源
- 终端输出：检测完成后的在线报告地址支持可点击超链接（Windows Terminal / iTerm 等）

## v0.1.17 更新

- 系统报告：CPU 偷取时间（steal time）采样与宿主机超售判断
- 系统报告：cgroup v2/v1 CPU、内存、I/O 限额检测与缩水提示
- 系统报告：文件系统、块设备、I/O 调度器、inode 使用率、fsync 延迟
- 系统报告：展示内存气球/KSM、BBR/TCP 调优、时区

## v0.1.16 更新

- 全国延迟：34 省含港澳台（ping 目标库校验与采集同步）
- 国际互连：香港节点改为韩国（sel.speedtest.sggs.network）
- 参考数据：ping_targets / intl_latency_nodes 同步更新

## v0.1.15 更新

- 网络质量：新增 UDP 检测（DNS 出站、STUN NAT 类型、QUIC UDP 443）
- 报告展示：网络质量板块显示 UDP 出站 / NAT / QUIC 详情
- 安装脚本：下载增加重试机制与进度条（v0.1.14）

## v0.1.14 更新

- Disney+：修正 bamgrid token / GraphQL 鉴权，API 失败时回退首页检测
- YouTube Premium：CONSENT Cookie、更多解锁特征，减少误报「失败」
- 参考数据优先从 `dl.gsvps.com` 拉取，失败自动切换 GitHub `main`

历史版本见 [GitHub Releases](https://github.com/gsvps/GSNode/releases)。
