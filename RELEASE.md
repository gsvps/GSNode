# GSNode 最新发布

当前版本：**v0.1.19**（见 `VERSION`）

## 安装

> **请务必使用最新版本。** 当前版本：**v0.1.19**。运行脚本后终端会输出实际版本号。

```bash
curl -fsSL https://dl.gsvps.com/install.sh?v=0.1.19 | sh
```

备用地址：

```bash
# GitHub
curl -fsSL https://github.com/gsvps/GSNode/raw/v0.1.19/install.sh | sh

# jsDelivr CDN
curl -fsSL https://cdn.jsdelivr.net/gh/gsvps/GSNode@v0.1.19/install.sh | sh
```

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
