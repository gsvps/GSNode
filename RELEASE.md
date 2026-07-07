# GSNode 最新发布

当前版本：**v0.1.14**（见 `VERSION`）

## 安装

```bash
curl -fsSL https://github.com/gsvps/GSNode/raw/main/install.sh | sh
```

备用（国内）：

```bash
curl -fsSL https://dl.gsvps.com/install.sh | sh
```

## v0.1.14 更新

- Disney+：修正 bamgrid token / GraphQL 鉴权，API 失败时回退首页检测
- YouTube Premium：CONSENT Cookie、更多解锁特征，减少误报「失败」
- 参考数据优先从 `dl.gsvps.com` 拉取，失败自动切换 GitHub `main`

历史版本见 [GitHub Releases](https://github.com/gsvps/GSNode/releases)。
